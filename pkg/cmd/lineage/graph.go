package lineage

import (
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

type sortableStringSlice []string

func (s sortableStringSlice) Len() int           { return len(s) }
func (s sortableStringSlice) Less(i, j int) bool { return s[i] < s[j] }
func (s sortableStringSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// Relationship represents a relationship type.
type Relationship string

// RelationshipSet contains a set of relationships.
type RelationshipSet map[Relationship]struct{}

// List returns the contents as a sorted string slice.
func (s RelationshipSet) List() []string {
	res := make(sortableStringSlice, 0, len(s))
	for key := range s {
		res = append(res, string(key))
	}
	sort.Sort(res)
	return []string(res)
}

// Node represents a Kubernetes object in an relationship tree.
type Node struct {
	*unstructuredv1.Unstructured
	UID             types.UID
	Name            string
	Namespace       string
	Group           string
	Kind            string
	OwnerReferences []metav1.OwnerReference
	Dependents      map[types.UID]RelationshipSet
}

// NodeMap contains a relationship tree stored as a map of nodes.
type NodeMap map[types.UID]*Node

const (
	RelationshipOwnerRef Relationship = "OwnerReference"
)

// resolveDependents resolves all dependents of the provided root object and
// returns a relationship tree.
//nolint:funlen
func resolveDependents(objects []unstructuredv1.Unstructured, rootUID types.UID) NodeMap {
	// Create global node map of all objects
	globalMap := NodeMap{}
	for ix, o := range objects {
		node := Node{
			Unstructured:    &objects[ix],
			UID:             o.GetUID(),
			OwnerReferences: o.GetOwnerReferences(),
			Dependents:      map[types.UID]RelationshipSet{},
		}
		globalMap[node.UID] = &node
	}

	// Populate dependents data for every node
	for _, node := range globalMap {
		uid, ownerRefs := node.UID, node.OwnerReferences
		for _, ref := range ownerRefs {
			if owner, ok := globalMap[ref.UID]; ok {
				if _, ok := owner.Dependents[uid]; !ok {
					owner.Dependents[uid] = RelationshipSet{}
				}
				owner.Dependents[uid][RelationshipOwnerRef] = struct{}{}
			}
		}
	}

	// Create submap of the root node & its dependents from the global map
	nodeMap, uidQueue, uidSet := NodeMap{}, []types.UID{}, map[types.UID]struct{}{}
	if node := globalMap[rootUID]; node != nil {
		nodeMap[rootUID] = node
		uidQueue = append(uidQueue, rootUID)
	}
	for {
		if len(uidQueue) == 0 {
			break
		}
		uid := uidQueue[0]

		// Guard against possible cyclic dependency
		if _, ok := uidSet[uid]; ok {
			uidQueue = uidQueue[1:]
			continue
		} else {
			uidSet[uid] = struct{}{}
		}

		if node := nodeMap[uid]; node != nil {
			dependents, ix := make([]types.UID, len(node.Dependents)), 0
			for dUID := range node.Dependents {
				nodeMap[dUID] = globalMap[dUID]
				dependents[ix] = dUID
				ix++
			}
			uidQueue = append(uidQueue[1:], dependents...)
		}
	}

	// Populate remaining data for submap nodes
	for _, o := range nodeMap {
		gvk := o.GroupVersionKind()
		o.Group = gvk.Group
		o.Kind = gvk.Kind
		o.Name = o.GetName()
		o.Namespace = o.GetNamespace()
	}

	klog.V(4).Infof("Resolved %d dependents for root object (uid: %s)", len(nodeMap)-1, rootUID)
	return nodeMap
}

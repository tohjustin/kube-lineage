package lineage

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// NodeMap represents an owner-dependent relationship tree stored as flat list
// of nodes.
type NodeMap map[types.UID]*Node

// Node represents a Kubernetes object.
type Node struct {
	*unstructuredv1.Unstructured
	UID             types.UID
	Name            string
	Namespace       string
	Group           string
	Kind            string
	OwnerReferences []metav1.OwnerReference
	Dependents      []types.UID
}

// resolveDependents resolves all dependents of the provided root object and
// returns an owner-dependent relationship tree.
func resolveDependents(objects []unstructuredv1.Unstructured, rootUID types.UID) (NodeMap, error) {
	// Create global node map of all objects
	globalMap := NodeMap{}
	for ix, o := range objects {
		node := Node{
			Unstructured:    &objects[ix],
			UID:             o.GetUID(),
			OwnerReferences: o.GetOwnerReferences(),
		}
		globalMap[node.UID] = &node
	}

	// Populate dependents data for every node
	for _, node := range globalMap {
		uid, ownerRefs := node.UID, node.OwnerReferences
		for _, ref := range ownerRefs {
			if owner, ok := globalMap[ref.UID]; ok {
				owner.Dependents = append(owner.Dependents, uid)
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
			for _, dUID := range node.Dependents {
				nodeMap[dUID] = globalMap[dUID]
			}
			uidQueue = append(uidQueue[1:], node.Dependents...)
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

	return nodeMap, nil
}

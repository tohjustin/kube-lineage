package lineage

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type NodeMap map[types.UID]*Node

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

func buildRelationshipNodeMap(objects []unstructuredv1.Unstructured, root unstructuredv1.Unstructured) (NodeMap, error) {
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
	rootUID := root.GetUID()
	uidSet := map[types.UID]struct{}{}
	uidQueue := []types.UID{rootUID}
	nodeMap := NodeMap{rootUID: globalMap[rootUID]}
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
			uidQueue = append(uidQueue[1:], nodeMap[uid].Dependents...)
			uidSet[uid] = struct{}{}
		}

		for _, dUID := range nodeMap[uid].Dependents {
			nodeMap[dUID] = globalMap[dUID]
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

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

	// Populate dependents for every node
	for _, node := range globalMap {
		uid, ownerRefs := node.UID, node.OwnerReferences
		for _, ref := range ownerRefs {
			if owner, ok := globalMap[ref.UID]; ok {
				owner.Dependents = append(owner.Dependents, uid)
			}
		}
	}

	// Create submap of the root node & its dependents
	rootUID := root.GetUID()
	nodeMap, queue := NodeMap{rootUID: globalMap[rootUID]}, []types.UID{rootUID}
	for {
		if len(queue) == 0 {
			break
		}
		uid := queue[0]
		queue = append(queue[1:], nodeMap[uid].Dependents...)
		for _, dUID := range nodeMap[uid].Dependents {
			nodeMap[dUID] = globalMap[dUID]
		}
	}

	// Populate field data for submap nodes
	for _, o := range nodeMap {
		gvk := o.GroupVersionKind()
		o.Group = gvk.Group
		o.Kind = gvk.Kind
		o.Name = o.GetName()
		o.Namespace = o.GetNamespace()
	}

	return nodeMap, nil
}

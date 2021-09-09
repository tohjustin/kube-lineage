package lineage

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type NodeMap map[types.UID]*Node

type Node struct {
	*unstructuredv1.Unstructured
	Dependents      []types.UID
	OwnerReferences []metav1.OwnerReference
	UID             types.UID
}

func newNode(u unstructuredv1.Unstructured) *Node {
	return &Node{
		Unstructured:    &u,
		UID:             u.GetUID(),
		OwnerReferences: u.GetOwnerReferences(),
	}
}

func buildRelationshipNodeMap(objects []unstructuredv1.Unstructured, root unstructuredv1.Unstructured) (NodeMap, error) {
	// Create global node map of all objects
	globalMap := NodeMap{}
	for _, o := range objects {
		node := newNode(o)
		globalMap[node.UID] = node
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

	return nodeMap, nil
}

package lineage

import (
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type Graph map[types.UID]*Node

type Node struct {
	*unstructuredv1.Unstructured
	Dependents []types.UID
}

func newNode(u unstructuredv1.Unstructured) *Node {
	return &Node{Unstructured: &u}
}

func buildDependencyGraph(objects []unstructuredv1.Unstructured, root unstructuredv1.Unstructured) Graph {
	graph := Graph{}
	for _, o := range objects {
		node := newNode(o)
		graph[node.GetUID()] = node
	}

	// TODO: Resolve dependencies using a queue + root nodes
	for _, node := range graph {
		uid, ownerRefs := node.GetUID(), node.GetOwnerReferences()
		for _, ref := range ownerRefs {
			if owner, ok := graph[ref.UID]; ok {
				owner.Dependents = append(owner.Dependents, uid)
			}
		}
	}

	return graph
}

package lineage

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

type sortableStringSlice []string

func (s sortableStringSlice) Len() int           { return len(s) }
func (s sortableStringSlice) Less(i, j int) bool { return s[i] < s[j] }
func (s sortableStringSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// Relationship represents a relationship type between two Kubernetes objects.
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

func (n *Node) AddDependent(uid types.UID, r Relationship) {
	if _, ok := n.Dependents[uid]; !ok {
		n.Dependents[uid] = RelationshipSet{}
	}
	n.Dependents[uid][r] = struct{}{}
}

// NodeMap contains a relationship tree stored as a map of nodes.
type NodeMap map[types.UID]*Node

const (
	// Kubernetes Event relationships.
	RelationshipEventRegarding Relationship = "EventRegarding"
	RelationshipEventRelated   Relationship = "EventRelated"

	// Kubernetes Owner-Dependent relationships.
	RelationshipControllerRef Relationship = "ControllerReference"
	RelationshipOwnerRef      Relationship = "OwnerReference"
)

// resolveDependents resolves all dependents of the provided root object and
// returns a relationship tree.
//nolint:funlen,gocognit
func resolveDependents(objects []unstructuredv1.Unstructured, rootUID types.UID) NodeMap {
	// Create global node map of all objects
	globalMap := NodeMap{}
	for ix, o := range objects {
		gvk := o.GroupVersionKind()
		node := Node{
			Unstructured:    &objects[ix],
			UID:             o.GetUID(),
			Name:            o.GetName(),
			Namespace:       o.GetNamespace(),
			Group:           gvk.Group,
			Kind:            gvk.Kind,
			OwnerReferences: o.GetOwnerReferences(),
			Dependents:      map[types.UID]RelationshipSet{},
		}
		globalMap[node.UID] = &node

		if node.Group == "" && node.Kind == "Node" {
			// Node events sent by the Kubelet uses the node's name as the
			// ObjectReference UID, so we include them as keys in our global map to
			// support lookup by nodename
			globalMap[types.UID(node.Name)] = &node
			// Node events sent by the kube-proxy uses the node's hostname as the
			// ObjectReference UID, so we include them as keys in our global map to
			// support lookup by hostname
			if hostname, ok := o.GetLabels()["kubernetes.io/hostname"]; ok {
				globalMap[types.UID(hostname)] = &node
			}
		}
	}

	// Populate dependents based on ControllerRef & OwnerRef relationships
	for _, node := range globalMap {
		uid, refs := node.UID, node.OwnerReferences
		for _, ref := range refs {
			if n, ok := globalMap[ref.UID]; ok {
				if ref.Controller != nil && *ref.Controller {
					n.AddDependent(uid, RelationshipControllerRef)
				}
				n.AddDependent(uid, RelationshipOwnerRef)
			}
		}
	}

	// Populate dependents based on EventRegarding & EventRelated relationships
	// TODO: It's possible to have events to be in a different namespace from the
	//       its referenced object, so update the resource fetching logic to
	//       always try to fetch events at the cluster scope for event resources
	var evUID, regUID, relUID types.UID
	var err error
	for _, node := range globalMap {
		switch {
		case node.Group == "" && node.Kind == "Event":
			evUID = node.UID
			regUID, err = getEventCoreReferenceUID(node.Unstructured)
			if err != nil || len(regUID) == 0 {
				klog.V(4).Infof("Failed to get object reference for event named \"%s\" in namespace \"%s\"", node.Name, node.Namespace)
				continue
			}
			if n, ok := globalMap[regUID]; ok {
				n.AddDependent(evUID, RelationshipEventRegarding)
			}
		case node.Group == "events.k8s.io" && node.Kind == "Event":
			evUID = node.UID
			regUID, relUID, err = getEventReferenceUIDs(node.Unstructured)
			if err != nil || len(regUID) == 0 {
				klog.V(4).Infof("Failed to get object reference for event.events.k8s.io named \"%s\" in namespace \"%s\"", node.Name, node.Namespace)
				continue
			}
			if n, ok := globalMap[regUID]; ok {
				n.AddDependent(evUID, RelationshipEventRegarding)
			}
			if len(relUID) > 0 {
				if n, ok := globalMap[relUID]; ok {
					n.AddDependent(evUID, RelationshipEventRelated)
				}
			}
		default:
			continue
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

	klog.V(4).Infof("Resolved %d dependents for root object (uid: %s)", len(nodeMap)-1, rootUID)
	return nodeMap
}

// getEventCoreReferenceUID returns the UID of the object this Event is about.
// The UID will be an empty string if the reference doesn't exist.
func getEventCoreReferenceUID(u *unstructuredv1.Unstructured) (types.UID, error) {
	var regUID types.UID
	var ev corev1.Event
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &ev)
	if err != nil {
		return "", err
	}
	regUID = ev.InvolvedObject.UID

	return regUID, nil
}

// getEventReferenceUIDs returns the UID of the object this Event.events.k8s.io
// is about & the UID of a secondary object if it exist. The UID will be an
// empty string if the reference doesn't exist.
func getEventReferenceUIDs(u *unstructuredv1.Unstructured) (types.UID, types.UID, error) {
	var regUID, relUID types.UID
	var ev eventsv1.Event
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &ev)
	if err != nil {
		return "", "", err
	}
	regUID = ev.Regarding.UID
	if ev.Related != nil {
		relUID = ev.Related.UID
	}

	return regUID, relUID, nil
}

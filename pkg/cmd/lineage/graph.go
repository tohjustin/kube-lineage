package lineage

import (
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
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

func (n *Node) GetNestedString(fields ...string) string {
	val, found, err := unstructuredv1.NestedString(n.UnstructuredContent(), fields...)
	if !found || err != nil {
		return ""
	}
	return val
}

// Key returns a key composed of the node's group, kind, namespace & name which
// can be used to identify or reference a node.
func (n *Node) Key() string {
	return fmt.Sprintf("%s/%s/%s/%s", n.Group, n.Kind, n.Namespace, n.Name)
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

	// Kubernetes Pod relationships.
	RelationshipPodContainerEnv    Relationship = "PodContainerEnvironment"
	RelationshipPodImagePullSecret Relationship = "PodImagePullSecret" //nolint:gosec
	RelationshipPodNode            Relationship = "PodNode"
	RelationshipPodPriorityClass   Relationship = "PodPriorityClass"
	RelationshipPodRuntimeClass    Relationship = "PodRuntimeClass"
	RelationshipPodServiceAccount  Relationship = "PodServiceAccount"
	RelationshipPodVolume          Relationship = "PodVolume"
)

// resolveDependents resolves all dependents of the provided root object and
// returns a relationship tree.
//nolint:funlen,gocognit
func resolveDependents(objects []unstructuredv1.Unstructured, rootUID types.UID) NodeMap {
	// Create global node maps of all objects, one mapped by node UIDs & the other
	// mapped by node keys
	globalMap := NodeMap{}
	globalMapByKey := map[string]*Node{}
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
		globalMapByKey[node.Key()] = &node

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

	// Populate dependents based on Owner-Dependent relationships
	for _, node := range globalMap {
		for _, ref := range node.OwnerReferences {
			if n, ok := globalMap[ref.UID]; ok {
				if ref.Controller != nil && *ref.Controller {
					n.AddDependent(node.UID, RelationshipControllerRef)
				}
				n.AddDependent(node.UID, RelationshipOwnerRef)
			}
		}
	}

	// Populate dependents based on Event relationships
	// TODO: It's possible to have events to be in a different namespace from the
	//       its referenced object, so update the resource fetching logic to
	//       always try to fetch events at the cluster scope for event resources
	for _, node := range globalMap {
		if (node.Group == "" || node.Group == "events.k8s.io") && node.Kind == "Event" {
			regUID, relUID := getEventReferenceUIDs(node)
			if len(regUID) == 0 {
				klog.V(4).Infof("Failed to get object reference for event named \"%s\" in namespace \"%s\"", node.Name, node.Namespace)
				continue
			}
			if n, ok := globalMap[regUID]; ok {
				n.AddDependent(node.UID, RelationshipEventRegarding)
			}
			if n, ok := globalMap[relUID]; ok {
				n.AddDependent(node.UID, RelationshipEventRelated)
			}
		}
	}

	// Populate dependents based on Pod relationships
	for _, node := range globalMap {
		if node.Group == "" && node.Kind == "Pod" {
			keyToRsetMap, err := getPodRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get object references for pod named \"%s\" in namespace \"%s\": %s", node.Name, node.Namespace, err)
				continue
			}
			for k, rset := range keyToRsetMap {
				if n, ok := globalMapByKey[k]; ok {
					for r := range rset {
						n.AddDependent(node.UID, r)
					}
				}
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

	klog.V(4).Infof("Resolved %d dependents for root object (uid: %s)", len(nodeMap)-1, rootUID)
	return nodeMap
}

// getEventReferenceUIDs returns the UID of the object this Event is about & the
// UID of a secondary object if it exist. The returned UID will be an empty
// string if the reference doesn't exist.
func getEventReferenceUIDs(n *Node) (types.UID, types.UID) {
	var regUID, relUID string
	switch n.Group {
	case "":
		regUID = n.GetNestedString("involvedobject", "uid")
	case "events.k8s.io":
		regUID = n.GetNestedString("regarding", "uid")
		relUID = n.GetNestedString("related", "uid")
	}
	return types.UID(regUID), types.UID(relUID)
}

// getPodRelationships returns returns a map of relationships (keyed by the
// nodes representing each object reference) that this Pod has.
//nolint:funlen,gocognit
func getPodRelationships(n *Node) (map[string]RelationshipSet, error) {
	var pod corev1.Pod
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &pod)
	if err != nil {
		return nil, err
	}

	var node Node
	ns := pod.Namespace
	result := map[string]RelationshipSet{}
	addRelationship := func(r Relationship, n Node) {
		k := n.Key()
		if _, ok := result[k]; !ok {
			result[k] = RelationshipSet{r: {}}
		}
		result[k][r] = struct{}{}
	}

	// RelationshipPodContainerEnv
	var cList []corev1.Container
	cList = append(cList, pod.Spec.InitContainers...)
	cList = append(cList, pod.Spec.Containers...)
	for _, c := range cList {
		for _, env := range c.EnvFrom {
			switch {
			case env.ConfigMapRef != nil:
				node = Node{Kind: "ConfigMap", Name: env.ConfigMapRef.Name, Namespace: ns}
				addRelationship(RelationshipPodContainerEnv, node)
			case env.SecretRef != nil:
				node = Node{Kind: "Secret", Name: env.SecretRef.Name, Namespace: ns}
				addRelationship(RelationshipPodContainerEnv, node)
			}
		}
		for _, env := range c.Env {
			if env.ValueFrom == nil {
				continue
			}
			switch {
			case env.ValueFrom.ConfigMapKeyRef != nil:
				node = Node{Kind: "ConfigMap", Name: env.ValueFrom.ConfigMapKeyRef.Name, Namespace: ns}
				addRelationship(RelationshipPodContainerEnv, node)
			case env.ValueFrom.SecretKeyRef != nil:
				node = Node{Kind: "Secret", Name: env.ValueFrom.SecretKeyRef.Name, Namespace: ns}
				addRelationship(RelationshipPodContainerEnv, node)
			}
		}
	}

	// RelationshipPodImagePullSecret
	for _, ips := range pod.Spec.ImagePullSecrets {
		node = Node{Kind: "Secret", Name: ips.Name, Namespace: ns}
		addRelationship(RelationshipPodImagePullSecret, node)
	}

	// RelationshipPodNode
	node = Node{Kind: "Node", Name: pod.Spec.NodeName}
	addRelationship(RelationshipPodNode, node)

	// RelationshipPodPriorityClass
	if pc := pod.Spec.PriorityClassName; len(pc) != 0 {
		node = Node{Group: "scheduling.k8s.io", Kind: "PriorityClass", Name: pc}
		addRelationship(RelationshipPodPriorityClass, node)
	}

	// RelationshipPodRuntimeClass
	if rc := pod.Spec.RuntimeClassName; rc != nil && len(*rc) != 0 {
		node = Node{Group: "node.k8s.io", Kind: "RuntimeClass", Name: *rc}
		addRelationship(RelationshipPodRuntimeClass, node)
	}

	// RelationshipPodServiceAccount
	node = Node{Kind: "ServiceAccount", Name: pod.Spec.ServiceAccountName, Namespace: ns}
	addRelationship(RelationshipPodServiceAccount, node)

	// RelationshipPodVolume
	for _, v := range pod.Spec.Volumes {
		switch {
		case v.ConfigMap != nil:
			node = Node{Kind: "ConfigMap", Name: v.ConfigMap.Name, Namespace: ns}
			addRelationship(RelationshipPodVolume, node)
		case v.PersistentVolumeClaim != nil:
			node = Node{Kind: "PersistentVolumeClaim", Name: v.PersistentVolumeClaim.ClaimName, Namespace: ns}
			addRelationship(RelationshipPodVolume, node)
		case v.Secret != nil:
			node = Node{Kind: "Secret", Name: v.Secret.SecretName, Namespace: ns}
			addRelationship(RelationshipPodVolume, node)
		case v.Projected != nil:
			for _, src := range v.Projected.Sources {
				switch {
				case src.ConfigMap != nil:
					node = Node{Kind: "ConfigMap", Name: src.ConfigMap.Name, Namespace: ns}
					addRelationship(RelationshipPodVolume, node)
				case src.Secret != nil:
					node = Node{Kind: "Secret", Name: src.Secret.Name, Namespace: ns}
					addRelationship(RelationshipPodVolume, node)
				}
			}
		}
	}

	return result, nil
}

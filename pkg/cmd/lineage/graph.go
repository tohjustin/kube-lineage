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

// ObjectReference is a reference to a Kubernetes object.
type ObjectReference struct {
	Group     string
	Kind      string
	Namespace string
	Name      string
}

// ObjectReferenceKey is a compact representation of an ObjectReference.
// Typically used as key types for in maps.
type ObjectReferenceKey string

// Key converts the ObjectReference into a ObjectReferenceKey.
func (o *ObjectReference) Key() ObjectReferenceKey {
	k := fmt.Sprintf("%s/%s/%s/%s", o.Group, o.Kind, o.Namespace, o.Name)
	return ObjectReferenceKey(k)
}

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

// RelationshipMap contains a map of relationships a Kubernetes object has with
// other objects in the cluster.
type RelationshipMap struct {
	DependenciesByRef map[ObjectReferenceKey]RelationshipSet
	DependenciesByUID map[types.UID]RelationshipSet
	DependentsByRef   map[ObjectReferenceKey]RelationshipSet
	DependentsByUID   map[types.UID]RelationshipSet
}

func newRelationshipMap() RelationshipMap {
	return RelationshipMap{
		DependenciesByRef: map[ObjectReferenceKey]RelationshipSet{},
		DependenciesByUID: map[types.UID]RelationshipSet{},
		DependentsByRef:   map[ObjectReferenceKey]RelationshipSet{},
		DependentsByUID:   map[types.UID]RelationshipSet{},
	}
}

func (m *RelationshipMap) AddDependencyByKey(k ObjectReferenceKey, r Relationship) {
	if _, ok := m.DependenciesByRef[k]; !ok {
		m.DependenciesByRef[k] = RelationshipSet{}
	}
	m.DependenciesByRef[k][r] = struct{}{}
}

func (m *RelationshipMap) AddDependencyByUID(uid types.UID, r Relationship) {
	if _, ok := m.DependenciesByUID[uid]; !ok {
		m.DependenciesByUID[uid] = RelationshipSet{}
	}
	m.DependenciesByUID[uid][r] = struct{}{}
}

func (m *RelationshipMap) AddDependentByKey(k ObjectReferenceKey, r Relationship) {
	if _, ok := m.DependentsByRef[k]; !ok {
		m.DependentsByRef[k] = RelationshipSet{}
	}
	m.DependentsByRef[k][r] = struct{}{}
}

func (m *RelationshipMap) AddDependentByUID(uid types.UID, r Relationship) {
	if _, ok := m.DependentsByUID[uid]; !ok {
		m.DependentsByUID[uid] = RelationshipSet{}
	}
	m.DependentsByUID[uid][r] = struct{}{}
}

// Node represents a Kubernetes object in an relationship tree.
type Node struct {
	*unstructuredv1.Unstructured
	UID             types.UID
	Group           string
	Kind            string
	Namespace       string
	Name            string
	OwnerReferences []metav1.OwnerReference
	Dependents      map[types.UID]RelationshipSet
}

func (n *Node) AddDependent(uid types.UID, r Relationship) {
	if _, ok := n.Dependents[uid]; !ok {
		n.Dependents[uid] = RelationshipSet{}
	}
	n.Dependents[uid][r] = struct{}{}
}

func (n *Node) GetObjectReferenceKey() ObjectReferenceKey {
	ref := ObjectReference{
		Group:     n.Group,
		Kind:      n.Kind,
		Name:      n.Name,
		Namespace: n.Namespace,
	}
	return ref.Key()
}

func (n *Node) GetNestedString(fields ...string) string {
	val, found, err := unstructuredv1.NestedString(n.UnstructuredContent(), fields...)
	if !found || err != nil {
		return ""
	}
	return val
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

	// Kubernetes PersistentVolume & PersistentVolumeClaim relationships.
	RelationshipPersistentVolumeClaim        Relationship = "PersistentVolumeClaim"
	RelationshipPersistentVolumeStorageClass Relationship = "PersistentVolumeStorageClass"

	// Kubernetes Pod relationships.
	RelationshipPodContainerEnv    Relationship = "PodContainerEnvironment"
	RelationshipPodImagePullSecret Relationship = "PodImagePullSecret" //nolint:gosec
	RelationshipPodNode            Relationship = "PodNode"
	RelationshipPodPriorityClass   Relationship = "PodPriorityClass"
	RelationshipPodRuntimeClass    Relationship = "PodRuntimeClass"
	RelationshipPodVolume          Relationship = "PodVolume"

	// Kubernetes ServiceAccount relationships.
	RelationshipServiceAccountSecret          Relationship = "ServiceAccountSecret"
	RelationshipServiceAccountImagePullSecret Relationship = "ServiceAccountImagePullSecret"
)

// resolveDependents resolves all dependents of the provided root object and
// returns a relationship tree.
//nolint:funlen,gocognit,gocyclo
func resolveDependents(objects []unstructuredv1.Unstructured, rootUID types.UID) NodeMap {
	// Create global node maps of all objects, one mapped by node UIDs & the other
	// mapped by node keys
	globalMapByUID := map[types.UID]*Node{}
	globalMapByKey := map[ObjectReferenceKey]*Node{}
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
		uid, key := node.UID, node.GetObjectReferenceKey()
		globalMapByUID[uid] = &node
		globalMapByKey[key] = &node

		if node.Group == "" && node.Kind == "Node" {
			// Node events sent by the Kubelet uses the node's name as the
			// ObjectReference UID, so we include them as keys in our global map to
			// support lookup by nodename
			globalMapByUID[types.UID(node.Name)] = &node
			// Node events sent by the kube-proxy uses the node's hostname as the
			// ObjectReference UID, so we include them as keys in our global map to
			// support lookup by hostname
			if hostname, ok := o.GetLabels()["kubernetes.io/hostname"]; ok {
				globalMapByUID[types.UID(hostname)] = &node
			}
		}
	}

	updateRelationships := func(node *Node, rmap *RelationshipMap) {
		for k, rset := range rmap.DependenciesByRef {
			if n, ok := globalMapByKey[k]; ok {
				for r := range rset {
					n.AddDependent(node.UID, r)
				}
			}
		}
		for uid, rset := range rmap.DependenciesByUID {
			if n, ok := globalMapByUID[uid]; ok {
				for r := range rset {
					n.AddDependent(node.UID, r)
				}
			}
		}
		for k, rset := range rmap.DependentsByRef {
			if n, ok := globalMapByKey[k]; ok {
				for r := range rset {
					node.AddDependent(n.UID, r)
				}
			}
		}
		for uid, rset := range rmap.DependentsByUID {
			if n, ok := globalMapByUID[uid]; ok {
				for r := range rset {
					node.AddDependent(n.UID, r)
				}
			}
		}
	}

	// Populate dependents based on Owner-Dependent relationships
	for _, node := range globalMapByUID {
		for _, ref := range node.OwnerReferences {
			if n, ok := globalMapByUID[ref.UID]; ok {
				if ref.Controller != nil && *ref.Controller {
					n.AddDependent(node.UID, RelationshipControllerRef)
				}
				n.AddDependent(node.UID, RelationshipOwnerRef)
			}
		}
	}

	var rmap *RelationshipMap
	var err error
	for _, node := range globalMapByUID {
		switch {
		// Populate dependents based on Event relationships
		// TODO: It's possible to have events to be in a different namespace from the
		//       its referenced object, so update the resource fetching logic to
		//       always try to fetch events at the cluster scope for event resources
		case (node.Group == "" || node.Group == "events.k8s.io") && node.Kind == "Event":
			rmap, err = getEventRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for event named \"%s\" in namespace \"%s\": %s", node.Name, node.Namespace, err)
				continue
			}
		// Populate dependents based on PersistentVolume relationships
		case node.Group == "" && node.Kind == "PersistentVolume":
			rmap, err = getPersistentVolumeRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for persistentvolume named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependents based on PersistentVolumeClaim relationships
		case node.Group == "" && node.Kind == "PersistentVolumeClaim":
			rmap, err = getPersistentVolumeClaimRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for persistentvolumeclaim named \"%s\" in namespace \"%s\": %s", node.Name, node.Namespace, err)
				continue
			}
		// Populate dependents based on Pod relationships
		case node.Group == "" && node.Kind == "Pod":
			rmap, err = getPodRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for pod named \"%s\" in namespace \"%s\": %s", node.Name, node.Namespace, err)
				continue
			}
		// Populate dependents based on ServiceAccount relationships
		case node.Group == "" && node.Kind == "ServiceAccount":
			rmap, err = getServiceAccountRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for serviceaccount named \"%s\" in namespace \"%s\": %s", node.Name, node.Namespace, err)
				continue
			}
		default:
			continue
		}
		updateRelationships(node, rmap)
	}

	// Create submap of the root node & its dependents from the global map
	nodeMap, uidQueue, uidSet := NodeMap{}, []types.UID{}, map[types.UID]struct{}{}
	if node := globalMapByUID[rootUID]; node != nil {
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
				nodeMap[dUID] = globalMapByUID[dUID]
				dependents[ix] = dUID
				ix++
			}
			uidQueue = append(uidQueue[1:], dependents...)
		}
	}

	klog.V(4).Infof("Resolved %d dependents for root object (uid: %s)", len(nodeMap)-1, rootUID)
	return nodeMap
}

// getEventRelationships returns a map of relationships that this Event has with
// other objects, based on what was referenced in its manifest.
//nolint:unparam
func getEventRelationships(n *Node) (*RelationshipMap, error) {
	result := newRelationshipMap()
	switch n.Group {
	case "":
		// RelationshipEventRegarding
		regUID := types.UID(n.GetNestedString("involvedobject", "uid"))
		result.AddDependencyByUID(regUID, RelationshipEventRegarding)
	case "events.k8s.io":
		// RelationshipEventRegarding
		regUID := types.UID(n.GetNestedString("regarding", "uid"))
		result.AddDependencyByUID(regUID, RelationshipEventRegarding)
		// RelationshipEventRelated
		relUID := types.UID(n.GetNestedString("related", "uid"))
		result.AddDependencyByUID(relUID, RelationshipEventRelated)
	}

	return &result, nil
}

// getPersistentVolumeRelationships returns a map of relationships that this
// PersistentVolume has with other objects, based on what was referenced in its
// manifest.
func getPersistentVolumeRelationships(n *Node) (*RelationshipMap, error) {
	var pv corev1.PersistentVolume
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &pv)
	if err != nil {
		return nil, err
	}

	var ref ObjectReference
	ns := pv.Namespace
	result := newRelationshipMap()

	// RelationshipPersistentVolumeClaim
	if pvcRef := pv.Spec.ClaimRef; pvcRef != nil {
		ref = ObjectReference{Kind: "PersistentVolumeClaim", Name: pvcRef.Name, Namespace: ns}
		result.AddDependentByKey(ref.Key(), RelationshipPersistentVolumeClaim)
	}

	// RelationshipPersistentVolumeStorageClass
	if sc := pv.Spec.StorageClassName; len(sc) > 0 {
		ref = ObjectReference{Group: "storage.k8s.io", Kind: "StorageClass", Name: sc}
		result.AddDependencyByKey(ref.Key(), RelationshipPersistentVolumeStorageClass)
	}

	return &result, nil
}

// getPersistentVolumeClaimRelationships returns a map of relationships that
// this PersistentVolumeClaim has with other objects, based on what was
// referenced in its manifest.
func getPersistentVolumeClaimRelationships(n *Node) (*RelationshipMap, error) {
	var pvc corev1.PersistentVolumeClaim
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &pvc)
	if err != nil {
		return nil, err
	}

	var ref ObjectReference
	result := newRelationshipMap()

	// RelationshipPersistentVolumeClaim
	if pv := pvc.Spec.VolumeName; len(pv) > 0 {
		ref = ObjectReference{Kind: "PersistentVolume", Name: pv}
		result.AddDependencyByKey(ref.Key(), RelationshipPersistentVolumeClaim)
	}

	return &result, nil
}

// getPodRelationships returns a map of relationships that this Pod has with
// other objects, based on what was referenced in its manifest.
//nolint:funlen,gocognit
func getPodRelationships(n *Node) (*RelationshipMap, error) {
	var pod corev1.Pod
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &pod)
	if err != nil {
		return nil, err
	}

	var ref ObjectReference
	ns := pod.Namespace
	result := newRelationshipMap()

	// RelationshipPodContainerEnv
	var cList []corev1.Container
	cList = append(cList, pod.Spec.InitContainers...)
	cList = append(cList, pod.Spec.Containers...)
	for _, c := range cList {
		for _, env := range c.EnvFrom {
			switch {
			case env.ConfigMapRef != nil:
				ref = ObjectReference{Kind: "ConfigMap", Name: env.ConfigMapRef.Name, Namespace: ns}
				result.AddDependencyByKey(ref.Key(), RelationshipPodContainerEnv)
			case env.SecretRef != nil:
				ref = ObjectReference{Kind: "Secret", Name: env.SecretRef.Name, Namespace: ns}
				result.AddDependencyByKey(ref.Key(), RelationshipPodContainerEnv)
			}
		}
		for _, env := range c.Env {
			if env.ValueFrom == nil {
				continue
			}
			switch {
			case env.ValueFrom.ConfigMapKeyRef != nil:
				ref = ObjectReference{Kind: "ConfigMap", Name: env.ValueFrom.ConfigMapKeyRef.Name, Namespace: ns}
				result.AddDependencyByKey(ref.Key(), RelationshipPodContainerEnv)
			case env.ValueFrom.SecretKeyRef != nil:
				ref = ObjectReference{Kind: "Secret", Name: env.ValueFrom.SecretKeyRef.Name, Namespace: ns}
				result.AddDependencyByKey(ref.Key(), RelationshipPodContainerEnv)
			}
		}
	}

	// RelationshipPodImagePullSecret
	for _, ips := range pod.Spec.ImagePullSecrets {
		ref = ObjectReference{Kind: "Secret", Name: ips.Name, Namespace: ns}
		result.AddDependencyByKey(ref.Key(), RelationshipPodImagePullSecret)
	}

	// RelationshipPodNode
	ref = ObjectReference{Kind: "Node", Name: pod.Spec.NodeName}
	result.AddDependencyByKey(ref.Key(), RelationshipPodNode)

	// RelationshipPodPriorityClass
	if pc := pod.Spec.PriorityClassName; len(pc) != 0 {
		ref = ObjectReference{Group: "scheduling.k8s.io", Kind: "PriorityClass", Name: pc}
		result.AddDependencyByKey(ref.Key(), RelationshipPodPriorityClass)
	}

	// RelationshipPodRuntimeClass
	if rc := pod.Spec.RuntimeClassName; rc != nil && len(*rc) != 0 {
		ref = ObjectReference{Group: "node.k8s.io", Kind: "RuntimeClass", Name: *rc}
		result.AddDependencyByKey(ref.Key(), RelationshipPodRuntimeClass)
	}

	// RelationshipPodVolume
	for _, v := range pod.Spec.Volumes {
		switch {
		case v.ConfigMap != nil:
			ref = ObjectReference{Kind: "ConfigMap", Name: v.ConfigMap.Name, Namespace: ns}
			result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
		case v.PersistentVolumeClaim != nil:
			ref = ObjectReference{Kind: "PersistentVolumeClaim", Name: v.PersistentVolumeClaim.ClaimName, Namespace: ns}
			result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
		case v.Secret != nil:
			ref = ObjectReference{Kind: "Secret", Name: v.Secret.SecretName, Namespace: ns}
			result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
		case v.Projected != nil:
			for _, src := range v.Projected.Sources {
				switch {
				case src.ConfigMap != nil:
					ref = ObjectReference{Kind: "ConfigMap", Name: src.ConfigMap.Name, Namespace: ns}
					result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
				case src.Secret != nil:
					ref = ObjectReference{Kind: "Secret", Name: src.Secret.Name, Namespace: ns}
					result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
				}
			}
		}
	}

	return &result, nil
}

// getServiceAccountRelationships returns a map of relationships that this
// ServiceAccount has with other objects, based on what was referenced in its
// manifest.
func getServiceAccountRelationships(n *Node) (*RelationshipMap, error) {
	var sa corev1.ServiceAccount
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &sa)
	if err != nil {
		return nil, err
	}

	var ref ObjectReference
	ns := sa.Namespace
	result := newRelationshipMap()

	// RelationshipServiceAccountImagePullSecret
	for _, s := range sa.ImagePullSecrets {
		ref = ObjectReference{Kind: "Secret", Name: s.Name, Namespace: ns}
		result.AddDependencyByKey(ref.Key(), RelationshipServiceAccountImagePullSecret)
	}

	// RelationshipServiceAccountSecret
	for _, s := range sa.Secrets {
		ref = ObjectReference{Kind: "Secret", Name: s.Name, Namespace: ns}
		result.AddDependentByKey(ref.Key(), RelationshipServiceAccountSecret)
	}

	return &result, nil
}

package graph

import (
	"fmt"
	"sort"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	nodev1 "k8s.io/api/node/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

// ObjectLabelSelectorKey is a compact representation of an ObjectLabelSelector.
// Typically used as key types for maps.
type ObjectLabelSelectorKey string

// ObjectLabelSelector is a reference to a collection of Kubernetes objects.
type ObjectLabelSelector struct {
	Group     string
	Kind      string
	Namespace string
	Selector  labels.Selector
}

// Key converts the ObjectLabelSelector into a ObjectLabelSelectorKey.
func (o *ObjectLabelSelector) Key() ObjectLabelSelectorKey {
	k := fmt.Sprintf("%s\\%s\\%s\\%s", o.Group, o.Kind, o.Namespace, o.Selector)
	return ObjectLabelSelectorKey(k)
}

// ObjectSelectorKey is a compact representation of an ObjectSelector.
// Typically used as key types for maps.
type ObjectSelectorKey string

// ObjectSelector is a reference to a collection of Kubernetes objects.
type ObjectSelector struct {
	Group      string
	Kind       string
	Namespaces sets.String
}

// Key converts the ObjectSelector into a ObjectSelectorKey.
func (o *ObjectSelector) Key() ObjectSelectorKey {
	k := fmt.Sprintf("%s\\%s\\%s", o.Group, o.Kind, o.Namespaces)
	return ObjectSelectorKey(k)
}

// ObjectReferenceKey is a compact representation of an ObjectReference.
// Typically used as key types for maps.
type ObjectReferenceKey string

// ObjectReference is a reference to a Kubernetes object.
type ObjectReference struct {
	Group     string
	Kind      string
	Namespace string
	Name      string
}

// Key converts the ObjectReference into a ObjectReferenceKey.
func (o *ObjectReference) Key() ObjectReferenceKey {
	k := fmt.Sprintf("%s\\%s\\%s\\%s", o.Group, o.Kind, o.Namespace, o.Name)
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
	DependenciesByLabelSelector map[ObjectLabelSelectorKey]RelationshipSet
	DependenciesByRef           map[ObjectReferenceKey]RelationshipSet
	DependenciesBySelector      map[ObjectSelectorKey]RelationshipSet
	DependenciesByUID           map[types.UID]RelationshipSet
	DependentsByLabelSelector   map[ObjectLabelSelectorKey]RelationshipSet
	DependentsByRef             map[ObjectReferenceKey]RelationshipSet
	DependentsBySelector        map[ObjectSelectorKey]RelationshipSet
	DependentsByUID             map[types.UID]RelationshipSet
	ObjectLabelSelectors        map[ObjectLabelSelectorKey]ObjectLabelSelector
	ObjectSelectors             map[ObjectSelectorKey]ObjectSelector
}

func newRelationshipMap() RelationshipMap {
	return RelationshipMap{
		DependenciesByLabelSelector: map[ObjectLabelSelectorKey]RelationshipSet{},
		DependenciesByRef:           map[ObjectReferenceKey]RelationshipSet{},
		DependenciesBySelector:      map[ObjectSelectorKey]RelationshipSet{},
		DependenciesByUID:           map[types.UID]RelationshipSet{},
		DependentsByLabelSelector:   map[ObjectLabelSelectorKey]RelationshipSet{},
		DependentsByRef:             map[ObjectReferenceKey]RelationshipSet{},
		DependentsBySelector:        map[ObjectSelectorKey]RelationshipSet{},
		DependentsByUID:             map[types.UID]RelationshipSet{},
		ObjectLabelSelectors:        map[ObjectLabelSelectorKey]ObjectLabelSelector{},
		ObjectSelectors:             map[ObjectSelectorKey]ObjectSelector{},
	}
}

func (m *RelationshipMap) AddDependencyByKey(k ObjectReferenceKey, r Relationship) {
	if _, ok := m.DependenciesByRef[k]; !ok {
		m.DependenciesByRef[k] = RelationshipSet{}
	}
	m.DependenciesByRef[k][r] = struct{}{}
}

func (m *RelationshipMap) AddDependencyByLabelSelector(o ObjectLabelSelector, r Relationship) {
	k := o.Key()
	if _, ok := m.DependenciesByLabelSelector[k]; !ok {
		m.DependenciesByLabelSelector[k] = RelationshipSet{}
	}
	m.DependenciesByLabelSelector[k][r] = struct{}{}
	m.ObjectLabelSelectors[k] = o
}

func (m *RelationshipMap) AddDependencyBySelector(o ObjectSelector, r Relationship) {
	k := o.Key()
	if _, ok := m.DependenciesBySelector[k]; !ok {
		m.DependenciesBySelector[k] = RelationshipSet{}
	}
	m.DependenciesBySelector[k][r] = struct{}{}
	m.ObjectSelectors[k] = o
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

func (m *RelationshipMap) AddDependentByLabelSelector(o ObjectLabelSelector, r Relationship) {
	k := o.Key()
	if _, ok := m.DependentsByLabelSelector[k]; !ok {
		m.DependentsByLabelSelector[k] = RelationshipSet{}
	}
	m.DependentsByLabelSelector[k][r] = struct{}{}
	m.ObjectLabelSelectors[k] = o
}

func (m *RelationshipMap) AddDependentBySelector(o ObjectSelector, r Relationship) {
	k := o.Key()
	if _, ok := m.DependentsBySelector[k]; !ok {
		m.DependentsBySelector[k] = RelationshipSet{}
	}
	m.DependentsBySelector[k][r] = struct{}{}
	m.ObjectSelectors[k] = o
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
	Version         string
	Kind            string
	Resource        string
	Namespaced      bool
	Namespace       string
	Name            string
	OwnerReferences []metav1.OwnerReference
	Dependencies    map[types.UID]RelationshipSet
	Dependents      map[types.UID]RelationshipSet
	Depth           uint
}

func (n *Node) AddDependency(uid types.UID, r Relationship) {
	if _, ok := n.Dependencies[uid]; !ok {
		n.Dependencies[uid] = RelationshipSet{}
	}
	n.Dependencies[uid][r] = struct{}{}
}

func (n *Node) AddDependent(uid types.UID, r Relationship) {
	if _, ok := n.Dependents[uid]; !ok {
		n.Dependents[uid] = RelationshipSet{}
	}
	n.Dependents[uid][r] = struct{}{}
}

func (n *Node) GetDeps(depsIsDependencies bool) map[types.UID]RelationshipSet {
	if depsIsDependencies {
		return n.Dependencies
	}
	return n.Dependents
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

func (n *Node) GetAPIResource() metav1.APIResource {
	// NOTE: This is a rather incomplete APIResource object, but it has enough
	//       information inside for our use case, which is to fetch API objects
	return metav1.APIResource{
		Group:      n.Group,
		Version:    n.Version,
		Kind:       n.Kind,
		Name:       n.Resource,
		Namespaced: n.Namespaced,
	}
}

// NodeList contains a list of nodes.
type NodeList []*Node

func (n NodeList) Len() int {
	return len(n)
}

func (n NodeList) Less(i, j int) bool {
	// Sort nodes in following order: Namespace, Kind, Group, Name
	a, b := n[i], n[j]
	if a.Namespace != b.Namespace {
		return a.Namespace < b.Namespace
	}
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	if a.Group != b.Group {
		return a.Group < b.Group
	}
	return a.Name < b.Name
}

func (n NodeList) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// NodeMap contains a relationship tree stored as a map of nodes.
type NodeMap map[types.UID]*Node

// ResolveDependencies resolves all dependencies of the provided objects and
// returns a relationship tree.
func ResolveDependencies(m meta.RESTMapper, objects []unstructuredv1.Unstructured, uids []types.UID) (NodeMap, error) {
	return resolveDeps(m, objects, uids, true)
}

// ResolveDependents resolves all dependents of the provided objects and returns
// a relationship tree.
func ResolveDependents(m meta.RESTMapper, objects []unstructuredv1.Unstructured, uids []types.UID) (NodeMap, error) {
	return resolveDeps(m, objects, uids, false)
}

// resolveDeps resolves all dependencies or dependents of the provided objects
// and returns a relationship tree.
//nolint:funlen,gocognit,gocyclo
func resolveDeps(m meta.RESTMapper, objects []unstructuredv1.Unstructured, uids []types.UID, depsIsDependencies bool) (NodeMap, error) {
	if len(uids) == 0 {
		return NodeMap{}, nil
	}
	// Create global node maps of all objects, one mapped by node UIDs & the other
	// mapped by node keys. This step also helps deduplicate the list of provided
	// objects
	globalMapByUID := map[types.UID]*Node{}
	globalMapByKey := map[ObjectReferenceKey]*Node{}
	for ix, o := range objects {
		gvk := o.GroupVersionKind()
		m, err := m.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			klog.V(4).Infof("Failed to map resource \"%s\" to GVR", gvk)
			return nil, err
		}
		ns := o.GetNamespace()
		node := Node{
			Unstructured:    &objects[ix],
			UID:             o.GetUID(),
			Name:            o.GetName(),
			Namespace:       ns,
			Namespaced:      ns != "",
			Group:           m.Resource.Group,
			Version:         m.Resource.Version,
			Kind:            m.GroupVersionKind.Kind,
			Resource:        m.Resource.Resource,
			OwnerReferences: o.GetOwnerReferences(),
			Dependencies:    map[types.UID]RelationshipSet{},
			Dependents:      map[types.UID]RelationshipSet{},
		}
		uid, key := node.UID, node.GetObjectReferenceKey()
		if n, ok := globalMapByUID[uid]; ok {
			klog.V(4).Infof("Duplicated %s.%s resource \"%s\" in namespace \"%s\"", n.Kind, n.Group, n.Name, n.Namespace)
		}
		globalMapByUID[uid] = &node
		globalMapByKey[key] = &node

		if node.Group == corev1.GroupName && node.Kind == "Node" {
			// Node events sent by the Kubelet uses the node's name as the
			// ObjectReference UID, so we include them as keys in our global map to
			// support lookup by nodename
			globalMapByUID[types.UID(node.Name)] = &node
			// Node events sent by the kube-proxy uses the node's hostname as the
			// ObjectReference UID, so we include them as keys in our global map to
			// support lookup by hostname
			if hostname, ok := o.GetLabels()[corev1.LabelHostname]; ok {
				globalMapByUID[types.UID(hostname)] = &node
			}
		}
	}

	resolveLabelSelectorToNodes := func(o ObjectLabelSelector) []*Node {
		var result []*Node
		for _, n := range globalMapByUID {
			if n.Group == o.Group && n.Kind == o.Kind && n.Namespace == o.Namespace {
				if ok := o.Selector.Matches(labels.Set(n.GetLabels())); ok {
					result = append(result, n)
				}
			}
		}
		return result
	}
	resolveSelectorToNodes := func(o ObjectSelector) []*Node {
		var result []*Node
		for _, n := range globalMapByUID {
			if n.Group == o.Group && n.Kind == o.Kind {
				if len(o.Namespaces) == 0 || o.Namespaces.Has(n.Namespace) {
					result = append(result, n)
				}
			}
		}
		return result
	}
	updateRelationships := func(node *Node, rmap *RelationshipMap) {
		for k, rset := range rmap.DependenciesByRef {
			if n, ok := globalMapByKey[k]; ok {
				for r := range rset {
					node.AddDependency(n.UID, r)
					n.AddDependent(node.UID, r)
				}
			}
		}
		for k, rset := range rmap.DependentsByRef {
			if n, ok := globalMapByKey[k]; ok {
				for r := range rset {
					n.AddDependency(node.UID, r)
					node.AddDependent(n.UID, r)
				}
			}
		}
		for k, rset := range rmap.DependenciesByLabelSelector {
			if ols, ok := rmap.ObjectLabelSelectors[k]; ok {
				for _, n := range resolveLabelSelectorToNodes(ols) {
					for r := range rset {
						node.AddDependency(n.UID, r)
						n.AddDependent(node.UID, r)
					}
				}
			}
		}
		for k, rset := range rmap.DependentsByLabelSelector {
			if ols, ok := rmap.ObjectLabelSelectors[k]; ok {
				for _, n := range resolveLabelSelectorToNodes(ols) {
					for r := range rset {
						n.AddDependency(node.UID, r)
						node.AddDependent(n.UID, r)
					}
				}
			}
		}
		for k, rset := range rmap.DependenciesBySelector {
			if os, ok := rmap.ObjectSelectors[k]; ok {
				for _, n := range resolveSelectorToNodes(os) {
					for r := range rset {
						node.AddDependency(n.UID, r)
						n.AddDependent(node.UID, r)
					}
				}
			}
		}
		for k, rset := range rmap.DependentsBySelector {
			if os, ok := rmap.ObjectSelectors[k]; ok {
				for _, n := range resolveSelectorToNodes(os) {
					for r := range rset {
						n.AddDependency(node.UID, r)
						node.AddDependent(n.UID, r)
					}
				}
			}
		}
		for uid, rset := range rmap.DependenciesByUID {
			if n, ok := globalMapByUID[uid]; ok {
				for r := range rset {
					node.AddDependency(n.UID, r)
					n.AddDependent(node.UID, r)
				}
			}
		}
		for uid, rset := range rmap.DependentsByUID {
			if n, ok := globalMapByUID[uid]; ok {
				for r := range rset {
					n.AddDependency(node.UID, r)
					node.AddDependent(n.UID, r)
				}
			}
		}
	}

	// Populate dependencies & dependents based on Owner-Dependent relationships
	for _, node := range globalMapByUID {
		for _, ref := range node.OwnerReferences {
			if n, ok := globalMapByUID[ref.UID]; ok {
				if ref.Controller != nil && *ref.Controller {
					node.AddDependency(n.UID, RelationshipControllerRef)
					n.AddDependent(node.UID, RelationshipControllerRef)
				}
				node.AddDependency(n.UID, RelationshipOwnerRef)
				n.AddDependent(node.UID, RelationshipOwnerRef)
			}
		}
	}

	var rmap *RelationshipMap
	var err error
	for _, node := range globalMapByUID {
		switch {
		// Populate dependencies & dependents based on PersistentVolume relationships
		case node.Group == corev1.GroupName && node.Kind == "PersistentVolume":
			rmap, err = getPersistentVolumeRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for persistentvolume named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependencies & dependents based on PersistentVolumeClaim relationships
		case node.Group == corev1.GroupName && node.Kind == "PersistentVolumeClaim":
			rmap, err = getPersistentVolumeClaimRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for persistentvolumeclaim named \"%s\" in namespace \"%s\": %s", node.Name, node.Namespace, err)
				continue
			}
		// Populate dependencies & dependents based on Pod relationships
		case node.Group == corev1.GroupName && node.Kind == "Pod":
			rmap, err = getPodRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for pod named \"%s\" in namespace \"%s\": %s", node.Name, node.Namespace, err)
				continue
			}
		// Populate dependencies & dependents based on Service relationships
		case node.Group == corev1.GroupName && node.Kind == "Service":
			rmap, err = getServiceRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for service named \"%s\" in namespace \"%s\": %s", node.Name, node.Namespace, err)
				continue
			}
		// Populate dependencies & dependents based on ServiceAccount relationships
		case node.Group == corev1.GroupName && node.Kind == "ServiceAccount":
			rmap, err = getServiceAccountRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for serviceaccount named \"%s\" in namespace \"%s\": %s", node.Name, node.Namespace, err)
				continue
			}
		// Populate dependencies & dependents based on PodSecurityPolicy relationships
		case node.Group == policyv1beta1.GroupName && node.Kind == "PodSecurityPolicy":
			rmap, err = getPodSecurityPolicyRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for podsecuritypolicy named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependencies & dependents based on PodDisruptionBudget relationships
		case node.Group == policyv1.GroupName && node.Kind == "PodDisruptionBudget":
			rmap, err = getPodDisruptionBudgetRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for poddisruptionbudget named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependencies & dependents based on MutatingWebhookConfiguration relationships
		case node.Group == admissionregistrationv1.GroupName && node.Kind == "MutatingWebhookConfiguration":
			rmap, err = getMutatingWebhookConfigurationRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for mutatingwebhookconfiguration named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependencies & dependents based on ValidatingWebhookConfiguration relationships
		case node.Group == admissionregistrationv1.GroupName && node.Kind == "ValidatingWebhookConfiguration":
			rmap, err = getValidatingWebhookConfigurationRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for validatingwebhookconfiguration named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependencies & dependents based on APIService relationships
		case node.Group == apiregistrationv1.GroupName && node.Kind == "APIService":
			rmap, err = getAPIServiceRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for apiservice named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependencies & dependents based on Event relationships
		case (node.Group == eventsv1.GroupName || node.Group == corev1.GroupName) && node.Kind == "Event":
			rmap, err = getEventRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for event named \"%s\" in namespace \"%s\": %s", node.Name, node.Namespace, err)
				continue
			}
		// Populate dependencies & dependents based on Ingress relationships
		case (node.Group == networkingv1.GroupName || node.Group == extensionsv1beta1.GroupName) && node.Kind == "Ingress":
			rmap, err = getIngressRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for ingress named \"%s\" in namespace \"%s\": %s", node.Name, node.Namespace, err)
				continue
			}
		// Populate dependencies & dependents based on IngressClass relationships
		case node.Group == networkingv1.GroupName && node.Kind == "IngressClass":
			rmap, err = getIngressClassRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for ingressclass named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependencies & dependents based on NetworkPolicy relationships
		case node.Group == networkingv1.GroupName && node.Kind == "NetworkPolicy":
			rmap, err = getNetworkPolicyRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for networkpolicy named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependencies & dependents based on RuntimeClass relationships
		case node.Group == nodev1.GroupName && node.Kind == "RuntimeClass":
			rmap, err = getRuntimeClassRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for runtimeclass named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependencies & dependents based on ClusterRole relationships
		case node.Group == rbacv1.GroupName && node.Kind == "ClusterRole":
			rmap, err = getClusterRoleRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for clusterrole named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependencies & dependents based on ClusterRoleBinding relationships
		case node.Group == rbacv1.GroupName && node.Kind == "ClusterRoleBinding":
			rmap, err = getClusterRoleBindingRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for clusterrolebinding named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependencies & dependents based on Role relationships
		case node.Group == rbacv1.GroupName && node.Kind == "Role":
			rmap, err = getRoleRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for role named \"%s\" in namespace \"%s\": %s: %s", node.Name, node.Namespace, err)
				continue
			}
		// Populate dependencies & dependents based on RoleBinding relationships
		case node.Group == rbacv1.GroupName && node.Kind == "RoleBinding":
			rmap, err = getRoleBindingRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for rolebinding named \"%s\" in namespace \"%s\": %s: %s", node.Name, node.Namespace, err)
				continue
			}
		// Populate dependencies & dependents based on CSIStorageCapacity relationships
		case node.Group == storagev1beta1.GroupName && node.Kind == "CSIStorageCapacity":
			rmap, err = getCSIStorageCapacityRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for csistoragecapacity named \"%s\": %s: %s", node.Name, err)
				continue
			}
		// Populate dependencies & dependents based on CSINode relationships
		case node.Group == storagev1.GroupName && node.Kind == "CSINode":
			rmap, err = getCSINodeRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for csinode named \"%s\": %s: %s", node.Name, err)
				continue
			}
		// Populate dependencies & dependents based on StorageClass relationships
		case node.Group == storagev1.GroupName && node.Kind == "StorageClass":
			rmap, err = getStorageClassRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for storageclass named \"%s\": %s: %s", node.Name, err)
				continue
			}
		// Populate dependencies & dependents based on VolumeAttachment relationships
		case node.Group == storagev1.GroupName && node.Kind == "VolumeAttachment":
			rmap, err = getVolumeAttachmentRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for volumeattachment named \"%s\": %s: %s", node.Name, err)
				continue
			}
		default:
			continue
		}
		updateRelationships(node, rmap)
	}

	// Create submap containing the provided objects & either their dependencies
	// or dependents from the global map
	var depth uint
	nodeMap, uidQueue, uidSet := NodeMap{}, []types.UID{}, map[types.UID]struct{}{}
	for _, uid := range uids {
		if node := globalMapByUID[uid]; node != nil {
			nodeMap[uid] = node
			uidQueue = append(uidQueue, uid)
		}
	}
	depth, uidQueue = 0, append(uidQueue, "")
	for {
		if len(uidQueue) <= 1 {
			break
		}
		uid := uidQueue[0]
		if uid == "" {
			depth, uidQueue = depth+1, append(uidQueue[1:], "")
			continue
		}

		// Guard against possible cycles
		if _, ok := uidSet[uid]; ok {
			uidQueue = uidQueue[1:]
			continue
		} else {
			uidSet[uid] = struct{}{}
		}

		if node := nodeMap[uid]; node != nil {
			// Allow nodes to keep the smallest depth. For example, if a node has a
			// depth of 1 & 7 in the relationship tree, we keep 1 so that when
			// printing the tree with a depth of 2, the node will still be printed
			if node.Depth == 0 || depth < node.Depth {
				node.Depth = depth
			}
			deps := node.GetDeps(depsIsDependencies)
			depUIDs, ix := make([]types.UID, len(deps)), 0
			for depUID := range deps {
				nodeMap[depUID] = globalMapByUID[depUID]
				depUIDs[ix] = depUID
				ix++
			}
			uidQueue = append(uidQueue[1:], depUIDs...)
		}
	}

	klog.V(4).Infof("Resolved %d deps for %d objects", len(nodeMap)-1, len(uids))
	return nodeMap, nil
}

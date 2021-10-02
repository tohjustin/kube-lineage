package lineage

import (
	"fmt"
	"sort"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
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
	DependenciesByUID           map[types.UID]RelationshipSet
	DependentsByLabelSelector   map[ObjectLabelSelectorKey]RelationshipSet
	DependentsByRef             map[ObjectReferenceKey]RelationshipSet
	DependentsByUID             map[types.UID]RelationshipSet
	ObjectLabelSelectors        map[ObjectLabelSelectorKey]ObjectLabelSelector
}

func newRelationshipMap() RelationshipMap {
	return RelationshipMap{
		DependenciesByLabelSelector: map[ObjectLabelSelectorKey]RelationshipSet{},
		DependenciesByRef:           map[ObjectReferenceKey]RelationshipSet{},
		DependenciesByUID:           map[types.UID]RelationshipSet{},
		DependentsByLabelSelector:   map[ObjectLabelSelectorKey]RelationshipSet{},
		DependentsByRef:             map[ObjectReferenceKey]RelationshipSet{},
		DependentsByUID:             map[types.UID]RelationshipSet{},
		ObjectLabelSelectors:        map[ObjectLabelSelectorKey]ObjectLabelSelector{},
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
	// Kubernetes ClusterRole, ClusterRoleBinding, RoleBinding relationships.
	RelationshipClusterRoleAggregationRule Relationship = "ClusterRoleAggregationRule"
	RelationshipClusterRoleBindingSubject  Relationship = "ClusterRoleBindingSubject"
	RelationshipClusterRoleBindingRole     Relationship = "ClusterRoleBindingRole"
	RelationshipRoleBindingSubject         Relationship = "RoleBindingSubject"
	RelationshipRoleBindingRole            Relationship = "RoleBindingRole"

	// Kubernetes Event relationships.
	RelationshipEventRegarding Relationship = "EventRegarding"
	RelationshipEventRelated   Relationship = "EventRelated"

	// Kubernetes Ingress & IngressClass relationships.
	RelationshipIngressClass           Relationship = "IngressClass"
	RelationshipIngressClassParameters Relationship = "IngressClassParameters"
	RelationshipIngressResource        Relationship = "IngressResource"
	RelationshipIngressService         Relationship = "IngressService"
	RelationshipIngressTLSSecret       Relationship = "IngressTLSSecret"

	// Kubernetes MutatingWebhookConfiguration & ValidatingWebhookConfiguration relationships.
	RelationshipWebhookConfigurationService Relationship = "WebhookConfigurationService"

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

	// Kubernetes Service relationships.
	RelationshipService Relationship = "Service"

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

	resolveSelectorToNodes := func(o ObjectLabelSelector) []*Node {
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
	updateRelationships := func(node *Node, rmap *RelationshipMap) {
		for k, rset := range rmap.DependenciesByRef {
			if n, ok := globalMapByKey[k]; ok {
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
		for k, rset := range rmap.DependenciesByLabelSelector {
			if ols, ok := rmap.ObjectLabelSelectors[k]; ok {
				for _, n := range resolveSelectorToNodes(ols) {
					for r := range rset {
						n.AddDependent(node.UID, r)
					}
				}
			}
		}
		for k, rset := range rmap.DependentsByLabelSelector {
			if ols, ok := rmap.ObjectLabelSelectors[k]; ok {
				for _, n := range resolveSelectorToNodes(ols) {
					for r := range rset {
						node.AddDependent(n.UID, r)
					}
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
		// Populate dependents based on Service relationships
		case node.Group == "" && node.Kind == "Service":
			rmap, err = getServiceRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for service named \"%s\" in namespace \"%s\": %s", node.Name, node.Namespace, err)
				continue
			}
		// Populate dependents based on ServiceAccount relationships
		case node.Group == "" && node.Kind == "ServiceAccount":
			rmap, err = getServiceAccountRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for serviceaccount named \"%s\" in namespace \"%s\": %s", node.Name, node.Namespace, err)
				continue
			}
		// Populate dependents based on MutatingWebhookConfiguration relationships
		case node.Group == "admissionregistration.k8s.io" && node.Kind == "MutatingWebhookConfiguration":
			rmap, err = getMutatingWebhookConfigurationRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for mutatingwebhookconfiguration named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependents based on ValidatingWebhookConfiguration relationships
		case node.Group == "admissionregistration.k8s.io" && node.Kind == "ValidatingWebhookConfiguration":
			rmap, err = getValidatingWebhookConfigurationRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for validatingwebhookconfiguration named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependents based on Event relationships
		// TODO: It's possible to have events to be in a different namespace from the
		//       its referenced object, so update the resource fetching logic to
		//       always try to fetch events at the cluster scope for event resources
		case (node.Group == "events.k8s.io" || node.Group == "") && node.Kind == "Event":
			rmap, err = getEventRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for event named \"%s\" in namespace \"%s\": %s", node.Name, node.Namespace, err)
				continue
			}
		// Populate dependents based on Ingress relationships
		case (node.Group == "networking.k8s.io" || node.Group == "extensions") && node.Kind == "Ingress":
			rmap, err = getIngressRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for ingress named \"%s\" in namespace \"%s\": %s", node.Name, node.Namespace, err)
				continue
			}
		// Populate dependents based on IngressClass relationships
		case node.Group == "networking.k8s.io" && node.Kind == "IngressClass":
			rmap, err = getIngressClassRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for ingressclass named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependents based on ClusterRole relationships
		case node.Group == "rbac.authorization.k8s.io" && node.Kind == "ClusterRole":
			rmap, err = getClusterRoleRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for clusterrole named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependents based on ClusterRoleBinding relationships
		case node.Group == "rbac.authorization.k8s.io" && node.Kind == "ClusterRoleBinding":
			rmap, err = getClusterRoleBindingRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for clusterrolebinding named \"%s\": %s", node.Name, err)
				continue
			}
		// Populate dependents based on RoleBinding relationships
		// TODO: It's possible to have rolebinding to reference clusterrole(s), so
		//       update the resource fetching logic to always try to fetch
		//       clusterroles
		case node.Group == "rbac.authorization.k8s.io" && node.Kind == "RoleBinding":
			rmap, err = getRoleBindingRelationships(node)
			if err != nil {
				klog.V(4).Infof("Failed to get relationships for rolebinding named \"%s\" in namespace \"%s\": %s: %s", node.Name, err)
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

// getClusterRoleRelationships returns a map of relationships that this
// ClusterRole has with other objects, based on what was referenced in
// its manifest.
func getClusterRoleRelationships(n *Node) (*RelationshipMap, error) {
	var cr rbacv1.ClusterRole
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &cr)
	if err != nil {
		return nil, err
	}

	var ols ObjectLabelSelector
	result := newRelationshipMap()

	// RelationshipClusterRoleAggregationRule
	if ar := cr.AggregationRule; ar != nil {
		for ix := range ar.ClusterRoleSelectors {
			selector, err := metav1.LabelSelectorAsSelector(&ar.ClusterRoleSelectors[ix])
			if err != nil {
				return nil, err
			}
			ols = ObjectLabelSelector{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole", Selector: selector}
			result.AddDependencyByLabelSelector(ols, RelationshipClusterRoleAggregationRule)
		}
	}

	return &result, nil
}

// getClusterRoleBindingRelationships returns a map of relationships that this
// ClusterRoleBinding has with other objects, based on what was referenced in
// its manifest.
func getClusterRoleBindingRelationships(n *Node) (*RelationshipMap, error) {
	var crb rbacv1.ClusterRoleBinding
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &crb)
	if err != nil {
		return nil, err
	}

	var ref ObjectReference
	result := newRelationshipMap()

	// RelationshipClusterRoleBindingSubject
	// TODO: Handle non-object type subjects such as user & group names
	for _, s := range crb.Subjects {
		ref = ObjectReference{Group: s.APIGroup, Kind: s.Kind, Namespace: s.Namespace, Name: s.Name}
		result.AddDependentByKey(ref.Key(), RelationshipClusterRoleBindingSubject)
	}

	// RelationshipClusterRoleBindingRole
	r := crb.RoleRef
	ref = ObjectReference{Group: r.APIGroup, Kind: r.Kind, Name: r.Name}
	result.AddDependencyByKey(ref.Key(), RelationshipClusterRoleBindingRole)

	return &result, nil
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

// getIngressRelationships returns a map of relationships that this Ingress has
// with other objects, based on what was referenced in its manifest.
//nolint:funlen,gocognit
func getIngressRelationships(n *Node) (*RelationshipMap, error) {
	var ref ObjectReference
	ns := n.Namespace
	result := newRelationshipMap()
	switch n.Group {
	case "extensions":
		var ing extensionsv1beta1.Ingress
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &ing)
		if err != nil {
			return nil, err
		}

		// RelationshipIngressClass
		if ingc := ing.Spec.IngressClassName; ingc != nil && len(*ingc) > 0 {
			ref = ObjectReference{Group: "networking.k8s.io", Kind: "IngressClass", Name: *ingc}
			result.AddDependencyByKey(ref.Key(), RelationshipIngressClass)
		}

		// RelationshipIngressResource
		// RelationshipIngressService
		var backends []extensionsv1beta1.IngressBackend
		if ing.Spec.Backend != nil {
			backends = append(backends, *ing.Spec.Backend)
		}
		for _, rule := range ing.Spec.Rules {
			if rule.HTTP != nil {
				for _, path := range rule.HTTP.Paths {
					backends = append(backends, path.Backend)
				}
			}
		}
		for _, b := range backends {
			switch {
			case b.Resource != nil:
				group := ""
				if b.Resource.APIGroup != nil {
					group = *b.Resource.APIGroup
				}
				ref = ObjectReference{Group: group, Kind: b.Resource.Kind, Name: b.Resource.Name, Namespace: ns}
				result.AddDependencyByKey(ref.Key(), RelationshipIngressResource)
			case b.ServiceName != "":
				ref = ObjectReference{Kind: "Service", Name: b.ServiceName, Namespace: ns}
				result.AddDependencyByKey(ref.Key(), RelationshipIngressService)
			}
		}

		// RelationshipIngressTLSSecret
		for _, tls := range ing.Spec.TLS {
			ref = ObjectReference{Kind: "Secret", Name: tls.SecretName, Namespace: ns}
			result.AddDependencyByKey(ref.Key(), RelationshipIngressClass)
		}
	case "networking.k8s.io":
		var ing networkingv1.Ingress
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &ing)
		if err != nil {
			return nil, err
		}

		// RelationshipIngressClass
		if ingc := ing.Spec.IngressClassName; ingc != nil && len(*ingc) > 0 {
			ref = ObjectReference{Group: "networking.k8s.io", Kind: "IngressClass", Name: *ingc}
			result.AddDependencyByKey(ref.Key(), RelationshipIngressClass)
		}

		// RelationshipIngressResource
		// RelationshipIngressService
		var backends []networkingv1.IngressBackend
		if ing.Spec.DefaultBackend != nil {
			backends = append(backends, *ing.Spec.DefaultBackend)
		}
		for _, rule := range ing.Spec.Rules {
			if rule.HTTP != nil {
				for _, path := range rule.HTTP.Paths {
					backends = append(backends, path.Backend)
				}
			}
		}
		for _, b := range backends {
			switch {
			case b.Resource != nil:
				group := ""
				if b.Resource.APIGroup != nil {
					group = *b.Resource.APIGroup
				}
				ref = ObjectReference{Group: group, Kind: b.Resource.Kind, Name: b.Resource.Name, Namespace: ns}
				result.AddDependencyByKey(ref.Key(), RelationshipIngressResource)
			case b.Service != nil:
				ref = ObjectReference{Kind: "Service", Name: b.Service.Name, Namespace: ns}
				result.AddDependencyByKey(ref.Key(), RelationshipIngressService)
			}
		}

		// RelationshipIngressTLSSecret
		for _, tls := range ing.Spec.TLS {
			ref = ObjectReference{Kind: "Secret", Name: tls.SecretName, Namespace: ns}
			result.AddDependencyByKey(ref.Key(), RelationshipIngressClass)
		}
	}

	return &result, nil
}

// getIngressClassRelationships returns a map of relationships that this
// IngressClass has with other objects, based on what was referenced in its
// manifest.
func getIngressClassRelationships(n *Node) (*RelationshipMap, error) {
	var ingc networkingv1.IngressClass
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &ingc)
	if err != nil {
		return nil, err
	}

	var ref ObjectReference
	result := newRelationshipMap()

	// RelationshipIngressClassParameters
	if p := ingc.Spec.Parameters; p != nil {
		group := ""
		if p.APIGroup != nil {
			group = *p.APIGroup
		}
		ns := ""
		if p.Namespace != nil {
			ns = *p.Namespace
		}
		ref = ObjectReference{Group: group, Kind: p.Kind, Namespace: ns, Name: p.Name}
		result.AddDependencyByKey(ref.Key(), RelationshipIngressClassParameters)
	}

	return &result, nil
}

// getMutatingWebhookConfigurationRelationships returns a map of relationships
// that this MutatingWebhookConfiguration has with other objects, based on what
// was referenced in its manifest.
func getMutatingWebhookConfigurationRelationships(n *Node) (*RelationshipMap, error) {
	var mwc admissionregistrationv1.MutatingWebhookConfiguration
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &mwc)
	if err != nil {
		return nil, err
	}

	var ref ObjectReference
	result := newRelationshipMap()

	// RelationshipWebhookConfigurationService
	for _, wh := range mwc.Webhooks {
		if svc := wh.ClientConfig.Service; svc != nil {
			ref = ObjectReference{Kind: "Service", Namespace: svc.Namespace, Name: svc.Name}
			result.AddDependencyByKey(ref.Key(), RelationshipWebhookConfigurationService)
		}
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

// getRoleBindingRelationships returns a map of relationships that this
// RoleBinding has with other objects, based on what was referenced in its
// manifest.
func getRoleBindingRelationships(n *Node) (*RelationshipMap, error) {
	var rb rbacv1.RoleBinding
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &rb)
	if err != nil {
		return nil, err
	}

	var ref ObjectReference
	ns := rb.Namespace
	result := newRelationshipMap()

	// RelationshipRoleBindingSubject
	// TODO: Handle non-object type subjects such as user & group names
	for _, s := range rb.Subjects {
		ref = ObjectReference{Group: s.APIGroup, Kind: s.Kind, Namespace: s.Namespace, Name: s.Name}
		result.AddDependentByKey(ref.Key(), RelationshipRoleBindingSubject)
	}

	// RelationshipRoleBindingRole
	r := rb.RoleRef
	ref = ObjectReference{Group: r.APIGroup, Kind: r.Kind, Namespace: ns, Name: r.Name}
	result.AddDependencyByKey(ref.Key(), RelationshipRoleBindingRole)

	return &result, nil
}

// getServiceRelationships returns a map of relationships that this
// Service has with other objects, based on what was referenced in its
// manifest.
func getServiceRelationships(n *Node) (*RelationshipMap, error) {
	var svc corev1.Service
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &svc)
	if err != nil {
		return nil, err
	}

	var ols ObjectLabelSelector
	ns := svc.Namespace
	result := newRelationshipMap()

	// RelationshipServiceSelector
	selector, err := labels.ValidatedSelectorFromSet(labels.Set(svc.Spec.Selector))
	if err != nil {
		return nil, err
	}
	ols = ObjectLabelSelector{Kind: "Pod", Namespace: ns, Selector: selector}
	result.AddDependencyByLabelSelector(ols, RelationshipService)

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

// getValidatingWebhookConfigurationRelationships returns a map of relationships
// that this ValidatingWebhookConfiguration has with other objects, based on
// what was referenced in its manifest.
func getValidatingWebhookConfigurationRelationships(n *Node) (*RelationshipMap, error) {
	var vwc admissionregistrationv1.ValidatingWebhookConfiguration
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &vwc)
	if err != nil {
		return nil, err
	}

	var ref ObjectReference
	result := newRelationshipMap()

	// RelationshipWebhookConfigurationService
	for _, wh := range vwc.Webhooks {
		if svc := wh.ClientConfig.Service; svc != nil {
			ref = ObjectReference{Kind: "Service", Namespace: svc.Namespace, Name: svc.Name}
			result.AddDependencyByKey(ref.Key(), RelationshipWebhookConfigurationService)
		}
	}

	return &result, nil
}

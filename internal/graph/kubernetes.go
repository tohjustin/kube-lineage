package graph

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// Kubernetes ClusterRole, ClusterRoleBinding, RoleBinding relationships.
	RelationshipClusterRoleAggregationRule Relationship = "ClusterRoleAggregationRule"
	RelationshipClusterRoleBindingSubject  Relationship = "ClusterRoleBindingSubject"
	RelationshipClusterRoleBindingRole     Relationship = "ClusterRoleBindingRole"
	RelationshipRoleBindingSubject         Relationship = "RoleBindingSubject"
	RelationshipRoleBindingRole            Relationship = "RoleBindingRole"

	// Kubernetes CSINode relationships.
	RelationshipCSINodeDriver Relationship = "CSINodeDriver"

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

// getCSINodeRelationships returns a map of relationships that this CSINode has
// with other objects, based on what was referenced in its manifest.
func getCSINodeRelationships(n *Node) (*RelationshipMap, error) {
	var csin storagev1.CSINode
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &csin)
	if err != nil {
		return nil, err
	}

	var ref ObjectReference
	result := newRelationshipMap()

	// RelationshipCSINodeDriver
	for _, d := range csin.Spec.Drivers {
		ref = ObjectReference{Group: "storage.k8s.io", Kind: "CSIDriver", Name: d.Name}
		result.AddDependentByKey(ref.Key(), RelationshipCSINodeDriver)
	}

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

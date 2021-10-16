package graph

import (
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	nodev1 "k8s.io/api/node/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

const (
	// Kubernetes APIService relationships.
	RelationshipAPIService Relationship = "APIService"

	// Kubernetes ClusterRole, ClusterRoleBinding, RoleBinding relationships.
	RelationshipClusterRoleAggregationRule Relationship = "ClusterRoleAggregationRule"
	RelationshipClusterRolePolicyRule      Relationship = "ClusterRolePolicyRule"
	RelationshipClusterRoleBindingSubject  Relationship = "ClusterRoleBindingSubject"
	RelationshipClusterRoleBindingRole     Relationship = "ClusterRoleBindingRole"
	RelationshipRoleBindingSubject         Relationship = "RoleBindingSubject"
	RelationshipRoleBindingRole            Relationship = "RoleBindingRole"
	RelationshipRolePolicyRule             Relationship = "RolePolicyRule"

	// Kubernetes CSINode relationships.
	RelationshipCSINodeDriver Relationship = "CSINodeDriver"

	// Kubernetes CSIStorageCapacity relationships.
	RelationshipCSIStorageCapacityStorageClass Relationship = "CSIStorageCapacityStorageClass"

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

	// Kubernetes RelationshipNetworkPolicy relationships.
	RelationshipNetworkPolicy Relationship = "NetworkPolicy"

	// Kubernetes Owner-Dependent relationships.
	RelationshipControllerRef Relationship = "ControllerReference"
	RelationshipOwnerRef      Relationship = "OwnerReference"

	// Kubernetes PersistentVolume & PersistentVolumeClaim relationships.
	RelationshipPersistentVolumeClaim           Relationship = "PersistentVolumeClaim"
	RelationshipPersistentVolumeCSIDriver       Relationship = "PersistentVolumeCSIDriver"
	RelationshipPersistentVolumeCSIDriverSecret Relationship = "PersistentVolumeCSIDriverSecret"
	RelationshipPersistentVolumeStorageClass    Relationship = "PersistentVolumeStorageClass"

	// Kubernetes Pod relationships.
	RelationshipPodContainerEnv          Relationship = "PodContainerEnvironment"
	RelationshipPodImagePullSecret       Relationship = "PodImagePullSecret" //nolint:gosec
	RelationshipPodNode                  Relationship = "PodNode"
	RelationshipPodPriorityClass         Relationship = "PodPriorityClass"
	RelationshipPodRuntimeClass          Relationship = "PodRuntimeClass"
	RelationshipPodSecurityPolicy        Relationship = "PodSecurityPolicy"
	RelationshipPodServiceAccount        Relationship = "PodServiceAccount"
	RelationshipPodVolume                Relationship = "PodVolume"
	RelationshipPodVolumeCSIDriver       Relationship = "PodVolumeCSIDriver"
	RelationshipPodVolumeCSIDriverSecret Relationship = "PodVolumeCSIDriverSecret" //nolint:gosec

	// Kubernetes PodDisruptionBudget relationships.
	RelationshipPodDisruptionBudget Relationship = "PodDisruptionBudget"

	// Kubernetes PodSecurityPolicy relationships.
	RelationshipPodSecurityPolicyAllowedCSIDriver    Relationship = "PodSecurityPolicyAllowedCSIDriver"
	RelationshipPodSecurityPolicyAllowedRuntimeClass Relationship = "PodSecurityPolicyAllowedRuntimeClass"
	RelationshipPodSecurityPolicyDefaultRuntimeClass Relationship = "PodSecurityPolicyDefaultRuntimeClass"

	// Kubernetes RuntimeClass relationships.
	RelationshipRuntimeClass Relationship = "RuntimeClass"

	// Kubernetes Service relationships.
	RelationshipService Relationship = "Service"

	// Kubernetes ServiceAccount relationships.
	RelationshipServiceAccountImagePullSecret Relationship = "ServiceAccountImagePullSecret"
	RelationshipServiceAccountSecret          Relationship = "ServiceAccountSecret"

	// Kubernetes StorageClass relationships.
	RelationshipStorageClassProvisioner Relationship = "StorageClassProvisioner"

	// Kubernetes VolumeAttachment relationships.
	RelationshipVolumeAttachmentAttacher                    Relationship = "VolumeAttachmentAttacher"
	RelationshipVolumeAttachmentNode                        Relationship = "VolumeAttachmentNode"
	RelationshipVolumeAttachmentSourceVolume                Relationship = "VolumeAttachmentSourceVolume"
	RelationshipVolumeAttachmentSourceVolumeClaim           Relationship = "VolumeAttachmentSourceVolumeClaim"
	RelationshipVolumeAttachmentSourceVolumeCSIDriver       Relationship = "VolumeAttachmentSourceVolumeCSIDriver"
	RelationshipVolumeAttachmentSourceVolumeCSIDriverSecret Relationship = "VolumeAttachmentSourceVolumeCSIDriverSecret"
	RelationshipVolumeAttachmentSourceVolumeStorageClass    Relationship = "VolumeAttachmentSourceVolumeStorageClass"
)

// getAPIServiceRelationships returns a map of relationships that this
// APIService has with other objects, based on what was referenced in its
// manifest.
func getAPIServiceRelationships(n *Node) (*RelationshipMap, error) {
	var apisvc apiregistrationv1.APIService
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &apisvc)
	if err != nil {
		return nil, err
	}

	// var os ObjectSelector
	var ref ObjectReference
	result := newRelationshipMap()

	// RelationshipAPIService
	if svc := apisvc.Spec.Service; svc != nil {
		ref = ObjectReference{Kind: "Service", Namespace: svc.Namespace, Name: svc.Name}
		result.AddDependencyByKey(ref.Key(), RelationshipAPIService)
	}

	return &result, nil
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

	var os ObjectSelector
	var ols ObjectLabelSelector
	var ref ObjectReference
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

	// RelationshipClusterRolePolicyRule
	for _, r := range cr.Rules {
		if podSecurityPolicyMatches(r) {
			switch len(r.ResourceNames) {
			case 0:
				os = ObjectSelector{Group: "policy", Kind: "PodSecurityPolicy"}
				result.AddDependencyBySelector(os, RelationshipClusterRolePolicyRule)
			default:
				for _, n := range r.ResourceNames {
					ref = ObjectReference{Group: "policy", Kind: "PodSecurityPolicy", Name: n}
					result.AddDependencyByKey(ref.Key(), RelationshipClusterRolePolicyRule)
				}
			}
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

// getCSIStorageCapacityRelationships returns a map of relationships that this
// CSIStorageCapacity has with other objects, based on what was referenced in
// its manifest.
func getCSIStorageCapacityRelationships(n *Node) (*RelationshipMap, error) {
	var csisc storagev1beta1.CSIStorageCapacity
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &csisc)
	if err != nil {
		return nil, err
	}

	var ref ObjectReference
	result := newRelationshipMap()

	// RelationshipCSIStorageCapacityStorageClass
	if sc := csisc.StorageClassName; len(sc) > 0 {
		ref = ObjectReference{Group: "storage.k8s.io", Kind: "StorageClass", Name: sc}
		result.AddDependencyByKey(ref.Key(), RelationshipCSIStorageCapacityStorageClass)
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

// getNetworkPolicyRelationships returns a map of relationships that this
// NetworkPolicy has with other objects, based on what was referenced in its
// manifest.
func getNetworkPolicyRelationships(n *Node) (*RelationshipMap, error) {
	var netpol networkingv1.NetworkPolicy
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &netpol)
	if err != nil {
		return nil, err
	}

	var ols ObjectLabelSelector
	ns := netpol.Namespace
	result := newRelationshipMap()

	// RelationshipNetworkPolicy
	selector, err := metav1.LabelSelectorAsSelector(&netpol.Spec.PodSelector)
	if err != nil {
		return nil, err
	}
	ols = ObjectLabelSelector{Kind: "Pod", Namespace: ns, Selector: selector}
	result.AddDependencyByLabelSelector(ols, RelationshipNetworkPolicy)

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

	// RelationshipPersistentVolumeCSIDriver
	// RelationshipPersistentVolumeCSIDriverSecret
	//nolint:gocritic
	switch {
	case pv.Spec.PersistentVolumeSource.CSI != nil:
		csi := pv.Spec.PersistentVolumeSource.CSI
		if d := csi.Driver; len(d) > 0 {
			ref = ObjectReference{Group: "storage.k8s.io", Kind: "CSIDriver", Name: d}
			result.AddDependencyByKey(ref.Key(), RelationshipPersistentVolumeCSIDriver)
		}
		if ces := csi.ControllerExpandSecretRef; ces != nil {
			ref = ObjectReference{Kind: "Secret", Name: ces.Name, Namespace: ces.Namespace}
			result.AddDependentByKey(ref.Key(), RelationshipPersistentVolumeCSIDriverSecret)
		}
		if cps := csi.ControllerPublishSecretRef; cps != nil {
			ref = ObjectReference{Kind: "Secret", Name: cps.Name, Namespace: cps.Namespace}
			result.AddDependentByKey(ref.Key(), RelationshipPersistentVolumeCSIDriverSecret)
		}
		if nps := csi.NodePublishSecretRef; nps != nil {
			ref = ObjectReference{Kind: "Secret", Name: nps.Name, Namespace: nps.Namespace}
			result.AddDependentByKey(ref.Key(), RelationshipPersistentVolumeCSIDriverSecret)
		}
		if nss := csi.NodeStageSecretRef; nss != nil {
			ref = ObjectReference{Kind: "Secret", Name: nss.Name, Namespace: nss.Namespace}
			result.AddDependentByKey(ref.Key(), RelationshipPersistentVolumeCSIDriverSecret)
		}
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

	// RelationshipPodSecurityPolicy
	// Hardcode "k8s.io/kubernetes/pkg/security/podsecuritypolicy/util.ValidatedPSPAnnotation"
	// as "kubernetes.io/psp" so we don't need import the entire k8s.io/kubernetes
	// package
	if psp, ok := pod.Annotations["kubernetes.io/psp"]; ok {
		ref = ObjectReference{Group: "policy", Kind: "PodSecurityPolicy", Name: psp}
		result.AddDependencyByKey(ref.Key(), RelationshipPodSecurityPolicy)
	}

	// RelationshipPodServiceAccount
	if sa := pod.Spec.ServiceAccountName; len(sa) != 0 {
		ref = ObjectReference{Kind: "ServiceAccount", Name: sa, Namespace: ns}
		result.AddDependencyByKey(ref.Key(), RelationshipPodServiceAccount)
	}

	// RelationshipPodVolume
	// RelationshipPodVolumeCSIDriver
	// RelationshipPodVolumeCSIDriverSecret
	for _, v := range pod.Spec.Volumes {
		vs := v.VolumeSource
		switch {
		case vs.ConfigMap != nil:
			ref = ObjectReference{Kind: "ConfigMap", Name: vs.ConfigMap.Name, Namespace: ns}
			result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
		case vs.CSI != nil:
			csi := vs.CSI
			ref = ObjectReference{Group: "storage.k8s.io", Kind: "CSIDriver", Name: csi.Driver}
			result.AddDependencyByKey(ref.Key(), RelationshipPodVolumeCSIDriver)
			if nps := csi.NodePublishSecretRef; nps != nil {
				ref = ObjectReference{Kind: "Secret", Name: nps.Name, Namespace: ns}
				result.AddDependencyByKey(ref.Key(), RelationshipPodVolumeCSIDriverSecret)
			}
		case vs.PersistentVolumeClaim != nil:
			ref = ObjectReference{Kind: "PersistentVolumeClaim", Name: vs.PersistentVolumeClaim.ClaimName, Namespace: ns}
			result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
		case vs.Projected != nil:
			for _, src := range vs.Projected.Sources {
				switch {
				case src.ConfigMap != nil:
					ref = ObjectReference{Kind: "ConfigMap", Name: src.ConfigMap.Name, Namespace: ns}
					result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
				case src.Secret != nil:
					ref = ObjectReference{Kind: "Secret", Name: src.Secret.Name, Namespace: ns}
					result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
				}
			}
		case vs.Secret != nil:
			ref = ObjectReference{Kind: "Secret", Name: vs.Secret.SecretName, Namespace: ns}
			result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
		}
	}

	return &result, nil
}

// getPodDisruptionBudgetRelationships returns a map of relationships that this
// PodDisruptionBudget has with other objects, based on what was referenced in its
// manifest.
func getPodDisruptionBudgetRelationships(n *Node) (*RelationshipMap, error) {
	var pdb policyv1.PodDisruptionBudget
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &pdb)
	if err != nil {
		return nil, err
	}

	var ols ObjectLabelSelector
	ns := pdb.Namespace
	result := newRelationshipMap()

	// RelationshipPodDisruptionBudget
	if s := pdb.Spec.Selector; s != nil {
		selector, err := metav1.LabelSelectorAsSelector(s)
		if err != nil {
			return nil, err
		}
		ols = ObjectLabelSelector{Kind: "Pod", Namespace: ns, Selector: selector}
		result.AddDependencyByLabelSelector(ols, RelationshipPodDisruptionBudget)
	}

	return &result, nil
}

// getPodSecurityPolicyRelationships returns a map of relationships that this
// PodSecurityPolicy has with other objects, based on what was referenced in its
// manifest.
func getPodSecurityPolicyRelationships(n *Node) (*RelationshipMap, error) {
	var psp policyv1beta1.PodSecurityPolicy
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &psp)
	if err != nil {
		return nil, err
	}

	var ref ObjectReference
	result := newRelationshipMap()

	// RelationshipPodSecurityPolicyAllowedCSIDriver
	for _, csi := range psp.Spec.AllowedCSIDrivers {
		ref = ObjectReference{Group: "storage.k8s.io", Kind: "CSIDriver", Name: csi.Name}
		result.AddDependencyByKey(ref.Key(), RelationshipPodSecurityPolicyAllowedCSIDriver)
	}
	if rc := psp.Spec.RuntimeClass; rc != nil {
		// RelationshipPodSecurityPolicyAllowedRuntimeClass
		for _, n := range psp.Spec.RuntimeClass.AllowedRuntimeClassNames {
			ref = ObjectReference{Group: "node.k8s.io", Kind: "RuntimeClass", Name: n}
			result.AddDependencyByKey(ref.Key(), RelationshipPodSecurityPolicyAllowedRuntimeClass)
		}

		// RelationshipPodSecurityPolicyDefaultRuntimeClass
		if n := psp.Spec.RuntimeClass.DefaultRuntimeClassName; n != nil {
			ref = ObjectReference{Group: "node.k8s.io", Kind: "RuntimeClass", Name: *n}
			result.AddDependencyByKey(ref.Key(), RelationshipPodSecurityPolicyDefaultRuntimeClass)
		}
	}

	return &result, nil
}

// getRoleRelationships returns a map of relationships that this Role has with
// other objects, based on what was referenced in its manifest.
func getRoleRelationships(n *Node) (*RelationshipMap, error) {
	var ro rbacv1.Role
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &ro)
	if err != nil {
		return nil, err
	}
	var os ObjectSelector

	var ref ObjectReference
	result := newRelationshipMap()

	// RelationshipRolePolicyRule
	for _, r := range ro.Rules {
		if podSecurityPolicyMatches(r) {
			switch len(r.ResourceNames) {
			case 0:
				os = ObjectSelector{Group: "policy", Kind: "PodSecurityPolicy"}
				result.AddDependencyBySelector(os, RelationshipRolePolicyRule)
			default:
				for _, n := range r.ResourceNames {
					ref = ObjectReference{Group: "policy", Kind: "PodSecurityPolicy", Name: n}
					result.AddDependencyByKey(ref.Key(), RelationshipRolePolicyRule)
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

// getRuntimeClassRelationships returns a map of relationships that this
// RuntimeClass has with other objects, based on what was referenced in its
// manifest.
func getRuntimeClassRelationships(n *Node) (*RelationshipMap, error) {
	var rc nodev1.RuntimeClass
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &rc)
	if err != nil {
		return nil, err
	}

	var ols ObjectLabelSelector
	result := newRelationshipMap()

	// RelationshipRuntimeClass
	if s := rc.Scheduling; s != nil {
		selector, err := labels.ValidatedSelectorFromSet(labels.Set(s.NodeSelector))
		if err != nil {
			return nil, err
		}
		ols = ObjectLabelSelector{Kind: "Node", Selector: selector}
		result.AddDependencyByLabelSelector(ols, RelationshipRuntimeClass)
	}

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

	// RelationshipService
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

// getStorageClassRelationships returns a map of relationships that this
// StorageClass has with other objects, based on what was referenced in its
// manifest.
func getStorageClassRelationships(n *Node) (*RelationshipMap, error) {
	var sc storagev1.StorageClass
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &sc)
	if err != nil {
		return nil, err
	}

	var ref ObjectReference
	result := newRelationshipMap()

	// RelationshipStorageClassProvisioner (external provisioners only)
	if p := sc.Provisioner; len(p) > 0 && !strings.HasPrefix(p, "kubernetes.io/") {
		ref = ObjectReference{Group: "storage.k8s.io", Kind: "CSIDriver", Name: p}
		result.AddDependencyByKey(ref.Key(), RelationshipStorageClassProvisioner)
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

// getVolumeAttachmentRelationships returns a map of relationships that this
// VolumeAttachment has with other objects, based on what was referenced in its
// manifest.
//nolint:funlen,nestif
func getVolumeAttachmentRelationships(n *Node) (*RelationshipMap, error) {
	var va storagev1.VolumeAttachment
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &va)
	if err != nil {
		return nil, err
	}

	var ref ObjectReference
	result := newRelationshipMap()

	// RelationshipVolumeAttachmentAttacher
	if a := va.Spec.Attacher; len(a) > 0 {
		ref = ObjectReference{Group: "storage.k8s.io", Kind: "CSIDriver", Name: a}
		result.AddDependencyByKey(ref.Key(), RelationshipVolumeAttachmentAttacher)
	}

	// RelationshipVolumeAttachmentNode
	if n := va.Spec.NodeName; len(n) > 0 {
		ref = ObjectReference{Kind: "Node", Name: n}
		result.AddDependencyByKey(ref.Key(), RelationshipVolumeAttachmentNode)
	}

	// RelationshipVolumeAttachmentSourceVolume
	if pvName := va.Spec.Source.PersistentVolumeName; pvName != nil && len(*pvName) > 0 {
		ref = ObjectReference{Kind: "PersistentVolume", Name: *pvName}
		result.AddDependentByKey(ref.Key(), RelationshipVolumeAttachmentSourceVolume)
	}

	if iv := va.Spec.Source.InlineVolumeSpec; iv != nil {
		// RelationshipVolumeAttachmentSourceVolumeClaim
		if iv.ClaimRef != nil {
			ref = ObjectReference{Kind: "PersistentVolumeClaim", Name: iv.ClaimRef.Name, Namespace: iv.ClaimRef.Namespace}
			result.AddDependentByKey(ref.Key(), RelationshipVolumeAttachmentSourceVolumeClaim)
		}

		// RelationshipVolumeAttachmentSourceVolumeStorageClass
		ref = ObjectReference{Group: "storage.k8s.io", Kind: "StorageClass", Name: iv.StorageClassName}
		result.AddDependentByKey(ref.Key(), RelationshipVolumeAttachmentSourceVolumeStorageClass)

		// RelationshipVolumeAttachmentSourceVolumeCSIDriver
		// RelationshipVolumeAttachmentSourceVolumeCSIDriverSecret
		//nolint:gocritic
		switch {
		case iv.PersistentVolumeSource.CSI != nil:
			csi := iv.PersistentVolumeSource.CSI
			if d := csi.Driver; len(d) > 0 {
				ref = ObjectReference{Group: "storage.k8s.io", Kind: "CSIDriver", Name: csi.Driver}
				result.AddDependentByKey(ref.Key(), RelationshipVolumeAttachmentSourceVolumeCSIDriver)
			}
			if ces := csi.ControllerExpandSecretRef; ces != nil {
				ref = ObjectReference{Kind: "Secret", Name: ces.Name, Namespace: ces.Namespace}
				result.AddDependentByKey(ref.Key(), RelationshipVolumeAttachmentSourceVolumeCSIDriverSecret)
			}
			if cps := csi.ControllerPublishSecretRef; cps != nil {
				ref = ObjectReference{Kind: "Secret", Name: cps.Name, Namespace: cps.Namespace}
				result.AddDependentByKey(ref.Key(), RelationshipVolumeAttachmentSourceVolumeCSIDriverSecret)
			}
			if nps := csi.NodePublishSecretRef; nps != nil {
				ref = ObjectReference{Kind: "Secret", Name: nps.Name, Namespace: nps.Namespace}
				result.AddDependentByKey(ref.Key(), RelationshipVolumeAttachmentSourceVolumeCSIDriverSecret)
			}
			if nss := csi.NodeStageSecretRef; nss != nil {
				ref = ObjectReference{Kind: "Secret", Name: nss.Name, Namespace: nss.Namespace}
				result.AddDependentByKey(ref.Key(), RelationshipVolumeAttachmentSourceVolumeCSIDriverSecret)
			}
		}
	}

	return &result, nil
}

// podSecurityPolicyMatches returns true if PolicyRule matches "policy" APIGroup,
// "podsecuritypolicies" resource & "use" verb.
func podSecurityPolicyMatches(r rbacv1.PolicyRule) bool {
	// NOTE: As of Kubernetes v1.22.1, the PodSecurityPolicy admission controller
	// 	     still checks against extensions API group for backward compatibility
	//       so we're going to do the same over here.
	//       See https://github.com/kubernetes/kubernetes/blob/v1.22.1/plugin/pkg/admission/security/podsecuritypolicy/admission.go#L346
	if sets.NewString(r.APIGroups...).HasAny(rbacv1.APIGroupAll, extensionsv1beta1.GroupName, policyv1beta1.GroupName) {
		if sets.NewString(r.Resources...).HasAny(rbacv1.ResourceAll, "podsecuritypolicies") {
			if sets.NewString(r.Verbs...).HasAny(rbacv1.VerbAll, "use") {
				return true
			}
		}
	}
	return false
}

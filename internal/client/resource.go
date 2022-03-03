package client

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// APIResource represents a Kubernetes API resource.
type APIResource metav1.APIResource

func (r APIResource) GroupKind() schema.GroupKind {
	return schema.GroupKind{
		Group: r.Group,
		Kind:  r.Kind,
	}
}

func (r APIResource) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   r.Group,
		Version: r.Version,
		Kind:    r.Kind,
	}
}

func (r APIResource) GroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    r.Group,
		Version:  r.Version,
		Resource: r.Name,
	}
}

func (r APIResource) String() string {
	if len(r.Group) == 0 {
		return fmt.Sprintf("%s.%s", r.Name, r.Version)
	}
	return fmt.Sprintf("%s.%s.%s", r.Name, r.Version, r.Group)
}

func (r APIResource) WithGroupString() string {
	if len(r.Group) == 0 {
		return r.Name
	}
	return r.Name + "." + r.Group
}

func ResourcesToGroupKindSet(apis []APIResource) map[schema.GroupKind]struct{} {
	gkSet := map[schema.GroupKind]struct{}{}
	for _, api := range apis {
		gk := api.GroupKind()
		// Account for resources that migrated API groups (for Kubernetes v1.18 & above)
		switch {
		// migrated from "events.v1" to "events.v1.events.k8s.io"
		case gk.Kind == "Event" && (gk.Group == "" || gk.Group == "events.k8s.io"):
			gkSet[schema.GroupKind{Kind: gk.Kind, Group: ""}] = struct{}{}
			gkSet[schema.GroupKind{Kind: gk.Kind, Group: "events.k8s.io"}] = struct{}{}
		// migrated from "ingresses.v1.extensions" to "ingresses.v1.networking.k8s.io"
		case gk.Kind == "Ingress" && (gk.Group == "extensions" || gk.Group == "networking.k8s.io"):
			gkSet[schema.GroupKind{Kind: gk.Kind, Group: "extensions"}] = struct{}{}
			gkSet[schema.GroupKind{Kind: gk.Kind, Group: "networking.k8s.io"}] = struct{}{}
		default:
			gkSet[gk] = struct{}{}
		}
	}
	return gkSet
}

// ObjectMeta contains the metadata for identifying a Kubernetes object.
type ObjectMeta struct {
	APIResource
	Name      string
	Namespace string
}

func (o ObjectMeta) String() string {
	return fmt.Sprintf("%s/%s", o.APIResource, o.Name)
}

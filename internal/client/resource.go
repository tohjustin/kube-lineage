package client

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// APIResource represents a Kubernetes API resource.
type APIResource metav1.APIResource

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

// ObjectMeta contains the metadata for identifying a Kubernetes object.
type ObjectMeta struct {
	APIResource
	Name      string
	Namespace string
}

func (o ObjectMeta) String() string {
	return fmt.Sprintf("%s/%s", o.APIResource, o.Name)
}

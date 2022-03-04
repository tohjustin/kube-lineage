package client

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	_ "k8s.io/client-go/plugin/pkg/client/auth" //nolint:gci
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	clientQPS   = 200
	clientBurst = 400
)

type GetOptions struct {
	APIResource APIResource
	Namespace   string
}

type GetTableOptions struct {
	APIResource APIResource
	Namespace   string
	Names       []string
}

type ListOptions struct {
	APIResourcesToExclude []APIResource
	APIResourcesToInclude []APIResource
	Namespaces            []string
}

type Interface interface {
	GetMapper() meta.RESTMapper
	IsReachable() error
	ResolveAPIResource(s string) (*APIResource, error)

	Get(ctx context.Context, name string, opts GetOptions) (*unstructuredv1.Unstructured, error)
	GetAPIResources(ctx context.Context) ([]APIResource, error)
	GetTable(ctx context.Context, opts GetTableOptions) (*metav1.Table, error)
	List(ctx context.Context, opts ListOptions) (*unstructuredv1.UnstructuredList, error)
}

type client struct {
	configFlags *Flags

	discoveryClient discovery.DiscoveryInterface
	dynamicClient   dynamic.Interface
	mapper          meta.RESTMapper
}

func (c *client) GetMapper() meta.RESTMapper {
	return c.mapper
}

// IsReachable tests connectivity to the cluster.
func (c *client) IsReachable() error {
	_, err := c.discoveryClient.ServerVersion()
	return err
}

func (c *client) ResolveAPIResource(s string) (*APIResource, error) {
	var gvr schema.GroupVersionResource
	var gvk schema.GroupVersionKind
	var err error

	// Resolve type string into GVR
	fullySpecifiedGVR, gr := schema.ParseResourceArg(strings.ToLower(s))
	if fullySpecifiedGVR != nil {
		gvr, _ = c.mapper.ResourceFor(*fullySpecifiedGVR)
	}
	if gvr.Empty() {
		gvr, err = c.mapper.ResourceFor(gr.WithVersion(""))
		if err != nil {
			if len(gr.Group) == 0 {
				err = fmt.Errorf("the server doesn't have a resource type \"%s\"", gr.Resource)
			} else {
				err = fmt.Errorf("the server doesn't have a resource type \"%s\" in group \"%s\"", gr.Resource, gr.Group)
			}
			return nil, err
		}
	}
	// Obtain Kind from GVR
	gvk, err = c.mapper.KindFor(gvr)
	if gvk.Empty() {
		if err != nil {
			if len(gvr.Group) == 0 {
				err = fmt.Errorf("the server couldn't identify a kind for resource type \"%s\"", gvr.Resource)
			} else {
				err = fmt.Errorf("the server couldn't identify a kind for resource type \"%s\" in group \"%s\"", gvr.Resource, gvr.Group)
			}
			return nil, err
		}
	}
	// Determine scope of resource
	mapping, err := c.mapper.RESTMapping(gvk.GroupKind())
	if err != nil {
		if len(gvk.Group) == 0 {
			err = fmt.Errorf("the server couldn't identify a group kind for resource type \"%s\"", gvk.Kind)
		} else {
			err = fmt.Errorf("the server couldn't identify a group kind for resource type \"%s\" in group \"%s\"", gvk.Kind, gvk.Group)
		}
		return nil, err
	}
	// NOTE: This is a rather incomplete APIResource object, but it has enough
	//       information inside for our use case, which is to fetch API objects
	res := &APIResource{
		Name:       gvr.Resource,
		Namespaced: mapping.Scope.Name() == meta.RESTScopeNameNamespace,
		Group:      gvk.Group,
		Version:    gvk.Version,
		Kind:       gvk.Kind,
	}

	return res, nil
}

// Get returns an object that matches the provided name & options on the server.
func (c *client) Get(ctx context.Context, name string, opts GetOptions) (*unstructuredv1.Unstructured, error) {
	klog.V(4).Infof("Get \"%s\" with options: %+v", name, opts)
	gvr := opts.APIResource.GroupVersionResource()
	var ri dynamic.ResourceInterface
	if opts.APIResource.Namespaced {
		ri = c.dynamicClient.Resource(gvr).Namespace(opts.Namespace)
	} else {
		ri = c.dynamicClient.Resource(gvr)
	}
	return ri.Get(ctx, name, metav1.GetOptions{})
}

// GetTable returns a table output from the server which contains data of the
// list of objects that matches the provided options. This is similar to an API
// request made by `kubectl get TYPE NAME... [-n NAMESPACE]`.
func (c *client) GetTable(ctx context.Context, opts GetTableOptions) (*metav1.Table, error) {
	klog.V(4).Infof("GetTable with options: %+v", opts)
	gk := opts.APIResource.GroupVersionKind().GroupKind()
	r := resource.NewBuilder(c.configFlags).
		Unstructured().
		NamespaceParam(opts.Namespace).
		ResourceNames(gk.String(), opts.Names...).
		ContinueOnError().
		Latest().
		TransformRequests(func(req *rest.Request) {
			req.SetHeader("Accept", strings.Join([]string{
				fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1.SchemeGroupVersion.Version, metav1.GroupName),
				fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1beta1.SchemeGroupVersion.Version, metav1beta1.GroupName),
				"application/json",
			}, ","))
			req.Param("includeObject", string(metav1.IncludeMetadata))
		}).
		Do()
	r.IgnoreErrors(apierrors.IsNotFound)
	if err := r.Err(); err != nil {
		return nil, err
	}

	infos, err := r.Infos()
	if err != nil || infos == nil {
		return nil, err
	}
	var table *metav1.Table
	for ix := range infos {
		t, err := decodeIntoTable(infos[ix].Object)
		if err != nil {
			return nil, err
		}
		if table == nil {
			table = t
			continue
		}
		table.Rows = append(table.Rows, t.Rows...)
	}
	return table, nil
}

func decodeIntoTable(obj runtime.Object) (*metav1.Table, error) {
	u, ok := obj.(*unstructuredv1.Unstructured)
	if !ok {
		return nil, fmt.Errorf("attempt to decode non-Unstructured object")
	}
	table := &metav1.Table{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, table); err != nil {
		return nil, err
	}

	for i := range table.Rows {
		row := &table.Rows[i]
		if row.Object.Raw == nil || row.Object.Object != nil {
			continue
		}
		converted, err := runtime.Decode(unstructuredv1.UnstructuredJSONScheme, row.Object.Raw)
		if err != nil {
			return nil, err
		}
		row.Object.Object = converted
	}

	return table, nil
}

// List returns a list of objects that matches the provided options on the
// server.
//nolint:funlen,gocognit
func (c *client) List(ctx context.Context, opts ListOptions) (*unstructuredv1.UnstructuredList, error) {
	klog.V(4).Infof("List with options: %+v", opts)
	apis, err := c.GetAPIResources(ctx)
	if err != nil {
		return nil, err
	}

	// Filter APIs
	if len(opts.APIResourcesToInclude) > 0 {
		includeGKSet := ResourcesToGroupKindSet(opts.APIResourcesToInclude)
		newAPIs := []APIResource{}
		for _, api := range apis {
			if _, ok := includeGKSet[api.GroupKind()]; ok {
				newAPIs = append(newAPIs, api)
			}
		}
		apis = newAPIs
	}
	if len(opts.APIResourcesToExclude) > 0 {
		excludeGKSet := ResourcesToGroupKindSet(opts.APIResourcesToExclude)
		newAPIs := []APIResource{}
		for _, api := range apis {
			if _, ok := excludeGKSet[api.GroupKind()]; !ok {
				newAPIs = append(newAPIs, api)
			}
		}
		apis = newAPIs
	}

	// Deduplicate list of namespaces & determine the scope for listing objects
	isClusterScopeRequest, nsSet := false, make(map[string]struct{})
	if len(opts.Namespaces) == 0 {
		isClusterScopeRequest = true
	}
	for _, ns := range opts.Namespaces {
		if ns != "" {
			nsSet[ns] = struct{}{}
		} else {
			isClusterScopeRequest = true
		}
	}

	var mu sync.Mutex
	var items []unstructuredv1.Unstructured
	createListFn := func(ctx context.Context, api APIResource, ns string) func() error {
		return func() error {
			objs, err := c.listByAPI(ctx, api, ns)
			if err != nil {
				return err
			}
			mu.Lock()
			items = append(items, objs.Items...)
			mu.Unlock()
			return nil
		}
	}
	eg, ctx := errgroup.WithContext(ctx)
	for i := range apis {
		api := apis[i]
		clusterScopeListFn := func() error {
			return createListFn(ctx, api, "")()
		}
		namespaceScopeListFn := func() error {
			egInner, ctxInner := errgroup.WithContext(ctx)
			for ns := range nsSet {
				listFn := createListFn(ctxInner, api, ns)
				egInner.Go(func() error {
					err = listFn()
					// If no permissions to list the resource at the namespace scope,
					// suppress the error to allow other goroutines to continue listing
					if apierrors.IsForbidden(err) {
						err = nil
					}
					return err
				})
			}
			return egInner.Wait()
		}
		eg.Go(func() error {
			var err error
			if isClusterScopeRequest {
				err = clusterScopeListFn()
				// If no permissions to list the cluster-scoped resource,
				// suppress the error to allow other goroutines to continue listing
				if !api.Namespaced && apierrors.IsForbidden(err) {
					err = nil
				}
				// If no permissions to list the namespaced resource at the cluster
				// scope, don't return the error yet & reattempt to list the resource
				// in other namespace(s)
				if !api.Namespaced || !apierrors.IsForbidden(err) {
					return err
				}
			}
			return namespaceScopeListFn()
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	klog.V(4).Infof("Got %4d objects from %d API resources", len(items), len(apis))
	return &unstructuredv1.UnstructuredList{Items: items}, nil
}

// GetAPIResources returns all API resource registered on the server.
func (c *client) GetAPIResources(_ context.Context) ([]APIResource, error) {
	rls, err := c.discoveryClient.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	apis := []APIResource{}
	for _, rl := range rls {
		if len(rl.APIResources) == 0 {
			continue
		}
		gv, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil {
			klog.V(4).Infof("Ignoring invalid discovered resource %q: %v", rl.GroupVersion, err)
			continue
		}
		for _, r := range rl.APIResources {
			// Filter resources that can be watched, listed & get
			if len(r.Verbs) == 0 || !sets.NewString(r.Verbs...).HasAll("watch", "list", "get") {
				continue
			}
			api := APIResource{
				Group:      gv.Group,
				Version:    gv.Version,
				Kind:       r.Kind,
				Name:       r.Name,
				Namespaced: r.Namespaced,
			}
			// Exclude duplicated resources (for Kubernetes v1.18 & above)
			switch {
			// migrated to "events.v1.events.k8s.io"
			case api.Group == "" && api.Kind == "Event":
				klog.V(4).Infof("Exclude duplicated discovered resource: %s", api)
				continue
			// migrated to "ingresses.v1.networking.k8s.io"
			case api.Group == "extensions" && api.Kind == "Ingress":
				klog.V(4).Infof("Exclude duplicated discovered resource: %s", api)
				continue
			}
			apis = append(apis, api)
		}
	}

	klog.V(4).Infof("Discovered %d available API resources to list", len(apis))
	return apis, nil
}

// listByAPI list all objects of the provided API & namespace. If listing the
// API at the cluster scope, set the namespace argument as an empty string.
func (c *client) listByAPI(ctx context.Context, api APIResource, ns string) (*unstructuredv1.UnstructuredList, error) {
	var ri dynamic.ResourceInterface
	var items []unstructuredv1.Unstructured
	var next string

	isClusterScopeRequest := !api.Namespaced || ns == ""
	if isClusterScopeRequest {
		ri = c.dynamicClient.Resource(api.GroupVersionResource())
	} else {
		ri = c.dynamicClient.Resource(api.GroupVersionResource()).Namespace(ns)
	}
	for {
		objectList, err := ri.List(ctx, metav1.ListOptions{
			Limit:    250,
			Continue: next,
		})
		if err != nil {
			switch {
			case apierrors.IsForbidden(err):
				if isClusterScopeRequest {
					klog.V(4).Infof("No access to list at cluster scope for resource: %s", api)
				} else {
					klog.V(4).Infof("No access to list in the namespace \"%s\" for resource: %s", ns, api)
				}
				return nil, err
			case apierrors.IsNotFound(err):
				break
			default:
				if isClusterScopeRequest {
					err = fmt.Errorf("failed to list resource type \"%s\" in API group \"%s\" at the cluster scope: %w", api.Name, api.Group, err)
				} else {
					err = fmt.Errorf("failed to list resource type \"%s\" in API group \"%s\" in the namespace \"%s\": %w", api.Name, api.Group, ns, err)
				}
				return nil, err
			}
		}
		if objectList == nil {
			break
		}
		items = append(items, objectList.Items...)
		next = objectList.GetContinue()
		if len(next) == 0 {
			break
		}
	}

	if isClusterScopeRequest {
		klog.V(4).Infof("Got %4d objects from resource at the cluster scope: %s", len(items), api)
	} else {
		klog.V(4).Infof("Got %4d objects from resource in the namespace \"%s\": %s", len(items), ns, api)
	}
	return &unstructuredv1.UnstructuredList{Items: items}, nil
}

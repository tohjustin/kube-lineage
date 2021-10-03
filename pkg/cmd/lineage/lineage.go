package lineage

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/tohjustin/kube-lineage/internal/graph"
	"github.com/tohjustin/kube-lineage/internal/log"
	lineageprinters "github.com/tohjustin/kube-lineage/internal/printers"
)

var (
	cmdPath    string
	cmdName    = "lineage"
	cmdUse     = "%CMD% (TYPE[.VERSION][.GROUP] [NAME] | TYPE[.VERSION][.GROUP]/NAME) [flags]"
	cmdExample = templates.Examples(`
		# List all dependents of the deployment named "bar" in the current namespace
		%CMD_PATH% deployments bar

		# List all dependents of the cronjob named "bar" in namespace "foo"
		%CMD_PATH% cronjobs.batch/bar -n foo

		# List all dependents of the node named "k3d-dev-server" & the corresponding relationship type(s)
		%CMD_PATH% node/k3d-dev-server -o wide`)
	cmdShort = "Display all dependents of a Kubernetes object"
	cmdLong  = templates.LongDesc(`
		Display all dependents of a Kubernetes object.

		TYPE is a Kubernetes resource. Shortcuts and groups will be resolved.
		NAME is the name of a particular Kubernetes resource.`)
)

// CmdOptions contains all the options for running the lineage command.
type CmdOptions struct {
	// RequestObject represents the requested object.
	RequestObject Object
	// RequestScope represents the scope of requested object.
	RequestScope meta.RESTScopeName

	ConfigFlags     *ConfigFlags
	ClientConfig    *rest.Config
	DynamicClient   dynamic.Interface
	DiscoveryClient discovery.DiscoveryInterface
	Namespace       string

	PrintFlags *lineageprinters.PrintFlags
	ToPrinter  func(withGroup bool, withNamespace bool) (printers.ResourcePrinterFunc, error)

	genericclioptions.IOStreams
}

// Object represents a Kubernetes object.
type Object struct {
	Name string
	Resource
}

func (o Object) String() string {
	return fmt.Sprintf("%s/%s", o.Resource, o.Name)
}

// Resource represents a Kubernetes resource.
type Resource struct {
	// Name is the plural name of the resource.
	Name string
	// Namespaced indicates if a resource is namespaced or not.
	Namespaced bool
	// Group is the preferred group of the resource.
	Group string
	// Version is the preferred version of the resource.
	Version string
	// Kind is the kind for the resource (e.g. 'Foo' is the kind for a resource 'foo').
	Kind string
}

func (r Resource) String() string {
	if len(r.Group) == 0 {
		return fmt.Sprintf("%s.%s", r.Name, r.Version)
	}
	return fmt.Sprintf("%s.%s.%s", r.Name, r.Version, r.Group)
}

func (r Resource) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   r.Group,
		Version: r.Version,
		Kind:    r.Kind,
	}
}

func (r Resource) GroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    r.Group,
		Version:  r.Version,
		Resource: r.Name,
	}
}

// NewCmd returns an initialized Command for the lineage command.
func NewCmd(streams genericclioptions.IOStreams, name, parentCmdPath string) *cobra.Command {
	o := &CmdOptions{
		ConfigFlags: NewConfigFlags(),
		PrintFlags:  lineageprinters.NewFlags(),
		IOStreams:   streams,
	}

	if len(name) > 0 {
		cmdName = name
	}
	cmdPath = cmdName
	if len(parentCmdPath) > 0 {
		cmdPath = parentCmdPath + " " + cmdName
	}
	cmd := &cobra.Command{
		Use:                   strings.ReplaceAll(cmdUse, "%CMD%", cmdName),
		Example:               strings.ReplaceAll(cmdExample, "%CMD_PATH%", cmdName),
		Short:                 cmdShort,
		Long:                  cmdLong,
		Args:                  cobra.MaximumNArgs(2),
		DisableFlagsInUseLine: true,
		DisableSuggestions:    true,
		SilenceUsage:          true,
		Run: func(c *cobra.Command, args []string) {
			klog.V(4).Infof("Version: %s", c.Version)
			cmdutil.CheckErr(o.Complete(c, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.ConfigFlags.AddFlags(cmd.Flags())
	o.PrintFlags.AddFlags(cmd.Flags())
	log.AddFlags(cmd.Flags())

	return cmd
}

// Complete completes all the required options for command.
//nolint:funlen
func (o *CmdOptions) Complete(cmd *cobra.Command, args []string) error {
	var resourceType, resourceName string
	switch len(args) {
	case 1:
		resourceTokens := strings.SplitN(args[0], "/", 2)
		if len(resourceTokens) != 2 {
			return fmt.Errorf("arguments in <resource>/<name> form must have a single resource and name\nSee '%s -h' for help and examples", cmdPath)
		}
		resourceType = resourceTokens[0]
		resourceName = resourceTokens[1]
	case 2:
		resourceType = args[0]
		resourceName = args[1]
	default:
		return fmt.Errorf("resource must be specified as <resource> <name> or <resource>/<name>\nSee '%s -h' for help and examples", cmdPath)
	}
	restMapper, err := o.ConfigFlags.ToRESTMapper()
	if err != nil {
		return err
	}
	name := resourceName
	gvr, gvk, err := resourceFor(restMapper, resourceType)
	if err != nil {
		return err
	}
	scope, err := resourceScopeFor(restMapper, *gvk)
	if err != nil {
		return err
	}
	o.RequestObject = Object{
		Name: name,
		Resource: Resource{
			Name:       gvr.Resource,
			Namespaced: *scope == meta.RESTScopeNameNamespace,
			Group:      gvk.Group,
			Version:    gvk.Version,
			Kind:       gvk.Kind,
		},
	}
	o.RequestScope = *scope

	o.ClientConfig, err = o.ConfigFlags.ToRESTConfig()
	if err != nil {
		return err
	}
	o.ClientConfig.QPS = 1000
	o.ClientConfig.Burst = 1000
	o.ClientConfig.WarningHandler = rest.NoWarnings{}
	o.DynamicClient, err = dynamic.NewForConfig(o.ClientConfig)
	if err != nil {
		return err
	}
	o.DiscoveryClient, err = o.ConfigFlags.ToDiscoveryClient()
	if err != nil {
		return err
	}
	o.Namespace, _, err = o.ConfigFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.ToPrinter = func(withGroup bool, withNamespace bool) (printers.ResourcePrinterFunc, error) {
		printFlags := o.PrintFlags.Copy()
		if withGroup {
			if err := printFlags.EnsureWithGroup(); err != nil {
				return nil, err
			}
		}
		if withNamespace {
			if err := printFlags.EnsureWithNamespace(); err != nil {
				return nil, err
			}
		}
		printer, err := printFlags.ToPrinter()
		if err != nil {
			return nil, err
		}

		return printer.PrintObj, nil
	}

	return nil
}

// Validate validates all the required options for the lineage command.
func (o *CmdOptions) Validate() error {
	if len(o.RequestObject.Name) == 0 ||
		o.RequestObject.GroupVersionResource().Empty() ||
		o.RequestObject.GroupVersionKind().Empty() {
		return fmt.Errorf("resource TYPE/NAME must be specified")
	}

	if o.ClientConfig == nil || o.DynamicClient == nil {
		return fmt.Errorf("client config, client must be provided")
	}

	klog.V(4).Infof("Namespace: %s", o.Namespace)
	klog.V(4).Infof("RequestObject: %v", o.RequestObject)
	klog.V(4).Infof("RequestScope: %v", o.RequestScope)
	klog.V(4).Infof("ConfigFlags.Context: %s", *o.ConfigFlags.Context)
	klog.V(4).Infof("ConfigFlags.Namespace: %s", *o.ConfigFlags.Namespace)
	klog.V(4).Infof("PrintFlags.OutputFormat: %s", *o.PrintFlags.OutputFormat)
	klog.V(4).Infof("PrintFlags.NoHeaders: %t", *o.PrintFlags.HumanReadableFlags.NoHeaders)
	klog.V(4).Infof("PrintFlags.ShowGroup: %t", *o.PrintFlags.HumanReadableFlags.ShowGroup)
	klog.V(4).Infof("PrintFlags.ShowLabels: %t", *o.PrintFlags.HumanReadableFlags.ShowLabels)
	klog.V(4).Infof("PrintFlags.WithNamespace: %t", o.PrintFlags.HumanReadableFlags.WithNamespace)

	return nil
}

// Run implements all the necessary functionality for command.
func (o *CmdOptions) Run() error {
	var rootObject *unstructuredv1.Unstructured
	var rootUID types.UID
	var err error
	ctx := context.Background()

	// Fetch the root object to ensure it exists before proceeding
	var ri dynamic.ResourceInterface
	gvr := o.RequestObject.GroupVersionResource()
	if o.RequestObject.Namespaced {
		ri = o.DynamicClient.Resource(gvr).Namespace(o.Namespace)
	} else {
		ri = o.DynamicClient.Resource(gvr)
	}
	rootObject, err = ri.Get(ctx, o.RequestObject.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	rootUID = rootObject.GetUID()

	// Fetch all resources in the cluster
	resources, err := o.getAPIResources(ctx)
	if err != nil {
		return err
	}
	objects, err := o.getObjectsByResources(ctx, resources)
	if err != nil {
		return err
	}

	// Include root object into objects to handle cases where user has access
	// to get the root object but unable to list it resource type
	objects = append(objects, *rootObject)

	// Find all dependents of the root object
	nodeMap := graph.ResolveDependents(objects, rootUID)

	// Print output
	return o.printObj(nodeMap, rootUID)
}

// getAPIResources fetches & returns all API resources that exists on the
// cluster.
func (o *CmdOptions) getAPIResources(_ context.Context) ([]Resource, error) {
	lists, err := o.DiscoveryClient.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	resources := []Resource{}
	for _, list := range lists {
		if len(list.APIResources) == 0 {
			continue
		}
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		for _, resource := range list.APIResources {
			// Filter resources that can be watched, listed & get
			if len(resource.Verbs) == 0 || !sets.NewString(resource.Verbs...).HasAll("watch", "list", "get") {
				continue
			}

			api := Resource{
				Name:       resource.Name,
				Namespaced: resource.Namespaced,
				Group:      gv.Group,
				Version:    gv.Version,
				Kind:       resource.Kind,
			}

			// Exclude duplicated resources (for Kubernetes v1.18 & above)
			switch {
			// migrated to "events.v1.events.k8s.io"
			case api.Group == "" && api.Name == "events":
				fallthrough
			// migrated to "ingresses.v1.networking.k8s.io"
			case api.Group == "extensions" && api.Name == "ingresses":
				klog.V(4).Infof("Exclude duplicated resource from discovered API resources: %s", api)
				continue
			}
			resources = append(resources, api)
		}
	}

	klog.V(4).Infof("Discovered %d available API resources to list", len(resources))
	return resources, nil
}

// getObjectsByResources fetches & returns all objects of the provided list of
// resources.
func (o *CmdOptions) getObjectsByResources(ctx context.Context, apis []Resource) ([]unstructuredv1.Unstructured, error) {
	var mu sync.Mutex
	var result []unstructuredv1.Unstructured
	g, ctx := errgroup.WithContext(ctx)
	for _, api := range apis {
		// Avoid getting cluster-scoped objects if the root object is a namespaced
		// resource since cluster-scoped objects cannot have namespaced resources as
		// an owner reference
		if o.RequestScope == meta.RESTScopeNameNamespace && !api.Namespaced {
			klog.V(4).Infof("Skip getting objects for resource: %s", api)
			continue
		}

		resource := api
		g.Go(func() error {
			objects, err := o.getObjectsByResource(ctx, resource)
			if err != nil {
				return err
			}
			mu.Lock()
			result = append(result, objects...)
			mu.Unlock()

			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	klog.V(4).Infof("Got %4d objects from %d API resources", len(result), len(apis))
	return result, nil
}

// getObjectsByResource fetches & returns all objects of the provided resource.
func (o *CmdOptions) getObjectsByResource(ctx context.Context, api Resource) ([]unstructuredv1.Unstructured, error) {
	gvr := api.GroupVersionResource()
	// If the root object is a namespaced resource, fetch all objects only from
	// the root object's namespace since its dependents cannot exist in other
	// namespaces
	scope := o.RequestScope

list_objects:
	var ri dynamic.ResourceInterface
	if scope == meta.RESTScopeNameRoot {
		ri = o.DynamicClient.Resource(gvr)
	} else {
		ri = o.DynamicClient.Resource(gvr).Namespace(o.Namespace)
	}

	var result []unstructuredv1.Unstructured
	var next string
	for {
		objectList, err := ri.List(ctx, metav1.ListOptions{
			Limit:    250,
			Continue: next,
		})
		if err != nil {
			switch {
			case apierrors.IsForbidden(err):
				// If the user doesn't have access to list the resource at the cluster
				// scope, attempt to list the resource in the root object's namespace
				if scope == meta.RESTScopeNameRoot {
					klog.V(4).Infof("No access to list at cluster scope for resource: %s", api)
					scope = meta.RESTScopeNameNamespace
					goto list_objects
				}
				// If the user doesn't have access to list the resource in the
				// namespace, we abort listing the resource
				klog.V(4).Infof("No access to list in the namespace \"%s\" for resource: %s", o.Namespace, api)
				return nil, nil
			default:
				if scope == meta.RESTScopeNameRoot {
					err = fmt.Errorf("failed to list resource type \"%s\" in API group \"%s\" at the cluster scope: %w", api.Name, api.Group, err)
				} else {
					err = fmt.Errorf("failed to list resource type \"%s\" in API group \"%s\" in the namespace \"%s\": %w", api.Name, api.Group, o.Namespace, err)
				}
				return nil, err
			}
		}
		result = append(result, objectList.Items...)

		next = objectList.GetContinue()
		if len(next) == 0 {
			break
		}
	}

	if scope == meta.RESTScopeNameRoot {
		klog.V(4).Infof("Got %4d objects from resource at the cluster scope: %s", len(result), api)
	} else {
		klog.V(4).Infof("Got %4d objects from resource in the namespace \"%s\": %s", len(result), api, o.Namespace)
	}
	return result, nil
}

// printObj prints the root object & its dependents in table format.
func (o *CmdOptions) printObj(nodeMap graph.NodeMap, rootUID types.UID) error {
	root, ok := nodeMap[rootUID]
	if !ok {
		return fmt.Errorf("requested object (uid: %s) not found in list of fetched objects", rootUID)
	}

	// Setup Table Printer
	withGroup := false
	if o.PrintFlags.HumanReadableFlags.ShowGroup != nil {
		withGroup = *o.PrintFlags.HumanReadableFlags.ShowGroup
	}
	// Display namespace column only if objects are in different namespaces
	withNamespace := false
	if o.RequestScope != meta.RESTScopeNameNamespace {
		for _, node := range nodeMap {
			if root.Namespace != node.Namespace {
				withNamespace = true
				break
			}
		}
	}
	printer, err := o.ToPrinter(withGroup, withNamespace)
	if err != nil {
		return err
	}

	// Generate Table Rows for printing
	table, err := lineageprinters.PrintNode(nodeMap, root, withGroup)
	if err != nil {
		return err
	}

	return printer.PrintObj(table, o.Out)
}

// resourceFor returns the GroupVersionResource & GroupVersionKind that matches
// provided resource argument string.
func resourceFor(mapper meta.RESTMapper, resourceArg string) (*schema.GroupVersionResource, *schema.GroupVersionKind, error) {
	var gvr schema.GroupVersionResource
	var gvk schema.GroupVersionKind
	var err error

	fullySpecifiedGVR, Resource := schema.ParseResourceArg(strings.ToLower(resourceArg))
	if fullySpecifiedGVR != nil {
		gvr, _ = mapper.ResourceFor(*fullySpecifiedGVR)
	}
	if gvr.Empty() {
		gvr, err = mapper.ResourceFor(Resource.WithVersion(""))
		if err != nil {
			if len(Resource.Group) == 0 {
				err = fmt.Errorf("the server doesn't have a resource type \"%s\"", Resource.Resource)
			} else {
				err = fmt.Errorf("the server doesn't have a resource type \"%s\" in group \"%s\"", Resource.Resource, Resource.Group)
			}
			return nil, nil, err
		}
	}

	gvk, err = mapper.KindFor(gvr)
	if gvk.Empty() {
		if err != nil {
			if len(gvr.Group) == 0 {
				err = fmt.Errorf("the server couldn't identify a kind for resource type \"%s\"", gvr.Resource)
			} else {
				err = fmt.Errorf("the server couldn't identify a kind for resource type \"%s\" in group \"%s\"", gvr.Resource, gvr.Group)
			}
			return nil, nil, err
		}
	}

	return &gvr, &gvk, nil
}

// resourceScopeFor returns the scope of the provided GroupVersionKind.
func resourceScopeFor(mapper meta.RESTMapper, gvk schema.GroupVersionKind) (*meta.RESTScopeName, error) {
	ret := meta.RESTScopeNameNamespace
	mapping, err := mapper.RESTMapping(gvk.GroupKind())
	if err != nil {
		if len(gvk.Group) == 0 {
			err = fmt.Errorf("the server couldn't identify a group kind for resource type \"%s\"", gvk.Kind)
		} else {
			err = fmt.Errorf("the server couldn't identify a group kind for resource type \"%s\" in group \"%s\"", gvk.Kind, gvk.Group)
		}
		return nil, err
	}
	ret = mapping.Scope.Name()

	return &ret, nil
}

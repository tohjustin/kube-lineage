package lineage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
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
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	cmdLong = templates.LongDesc(`
		Display all dependents of a Kubernetes object.

		TYPE is a Kubernetes resource. Shortcuts and groups will be resolved.
		NAME is the name of a particular Kubernetes resource.`)

	cmdExample = templates.Examples(`
		# List all dependents of the deployment named "bar" in the current namespace
		kubectl lineage deployments bar

		# List all dependents of the cronjob named "bar" in namespace "foo"
		kubectl lineage cronjobs.batch/bar -n foo`)
)

// CmdOptions contains all the options for running the lineage command.
type CmdOptions struct {
	ConfigFlags *genericclioptions.ConfigFlags
	PrintFlags  *PrintFlags
	ToPrinter   func(withGroup bool, withNamespace bool) (printers.ResourcePrinterFunc, error)

	ClientConfig    *rest.Config
	DynamicClient   dynamic.Interface
	DiscoveryClient discovery.DiscoveryInterface
	Namespace       string

	Resource      schema.GroupVersionResource
	ResourceName  string
	ResourceScope meta.RESTScopeName

	genericclioptions.IOStreams
}

// Resource contains the GroupVersionResource and APIResource for a resource
type Resource struct {
	// GroupVersionResource unambiguously identifies a resource.
	APIGroupVersionResource schema.GroupVersionResource
	// APIResource specifies the name of a resource and whether it is namespaced.
	APIResource metav1.APIResource
}

// New returns an initialized Command for the lineage command.
func New(streams genericclioptions.IOStreams) *cobra.Command {
	o := &CmdOptions{
		ConfigFlags: genericclioptions.NewConfigFlags(true),
		PrintFlags:  NewLineagePrintFlags(),
		IOStreams:   streams,
	}

	cmd := &cobra.Command{
		Use:                   "lineage (TYPE[.VERSION][.GROUP] [NAME] | TYPE[.VERSION][.GROUP]/NAME) [flags]",
		DisableFlagsInUseLine: true,
		Short:                 "Display all dependents of a Kubernetes object",
		Long:                  cmdLong,
		Example:               cmdExample,
		SilenceUsage:          true,
		Run: func(c *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(c, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.ConfigFlags.AddFlags(cmd.Flags())
	o.PrintFlags.AddFlags(cmd)

	return cmd
}

// Complete completes all the required options for command.
func (o *CmdOptions) Complete(cmd *cobra.Command, args []string) error {
	var resourceType, resourceName string
	switch len(args) {
	case 1:
		resourceTokens := strings.SplitN(args[0], "/", 2)
		if len(resourceTokens) != 2 {
			return errors.New("you must specify one or two arguments: resource or resource & resourceName")
		}
		resourceType = resourceTokens[0]
		resourceName = resourceTokens[1]
	case 2:
		resourceType = args[0]
		resourceName = args[1]
	default:
		return errors.New("you must specify one or two arguments: resource or resource & resourceName")
	}
	restMapper, err := o.ConfigFlags.ToRESTMapper()
	if err != nil {
		return err
	}
	o.Resource, err = resourceFor(restMapper, resourceType)
	if err != nil {
		return err
	}
	o.ResourceName = resourceName
	o.ResourceScope, err = resourceScopeFor(restMapper, o.Resource)
	if err != nil {
		return err
	}

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
	if o.Resource.Empty() || len(o.ResourceName) == 0 {
		return errors.New("resource TYPE/NAME must be specified")
	}

	if o.ClientConfig == nil || o.DynamicClient == nil {
		return errors.New("client config, client must be provided")
	}

	return nil
}

// Run implements all the necessary functionality for command.
func (o *CmdOptions) Run() error {
	// Fetch the root object to ensure it exists before proceeding
	var ri dynamic.ResourceInterface
	if o.ResourceScope == meta.RESTScopeNameRoot {
		ri = o.DynamicClient.Resource(o.Resource)
	} else {
		ri = o.DynamicClient.Resource(o.Resource).Namespace(o.Namespace)
	}
	rootObject, err := ri.Get(context.Background(), o.ResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	rootUID := rootObject.GetUID()

	// Fetch all resources in the cluster
	resources, err := o.getAPIResources()
	if err != nil {
		return err
	}
	objects, err := o.getObjectsByResources(resources)
	if err != nil {
		return err
	}

	// Include root object into objects to handle cases where user has access
	// to get the root object but unable to list it resource type
	objects = append(objects, *rootObject)

	// Find all dependents of the root object
	nodeMap, err := buildRelationshipNodeMap(objects, rootUID)
	if err != nil {
		return err
	}

	// Print output
	err = o.print(nodeMap, rootUID)
	if err != nil {
		return err
	}

	return nil
}

func (o *CmdOptions) getAPIResources() ([]Resource, error) {
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
			// Exclude duplicated resources (for Kubernetes v1.18 & above)
			switch {
			// migrated to "events.v1.events.k8s.io"
			case gv.Group == "" && resource.Name == "events":
				continue
			// migrated to "ingresses.v1.networking.k8s.io"
			case gv.Group == "extensions" && resource.Name == "ingresses":
				continue
			}
			resources = append(resources, Resource{
				APIGroupVersionResource: schema.GroupVersionResource{
					Group:    gv.Group,
					Version:  gv.Version,
					Resource: resource.Name,
				},
				APIResource: resource,
			})
		}
	}

	return resources, nil
}

func (o *CmdOptions) getObjectsByResources(apis []Resource) ([]unstructuredv1.Unstructured, error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var result []unstructuredv1.Unstructured
	var errResult *multierror.Error

	errors := make(chan error, len(apis))
	for _, api := range apis {
		// Avoid getting cluster-scoped objects if the root object is a namespaced
		// resource since cluster-scoped objects cannot have namespaced resources as
		// an owner reference
		if o.ResourceScope == meta.RESTScopeNameNamespace && !api.APIResource.Namespaced {
			continue
		}

		wg.Add(1)
		go func(r Resource) {
			defer wg.Done()
			objects, err := o.getObjectsByResource(r)
			if err != nil {
				errors <- err
				return
			}
			mu.Lock()
			result = append(result, objects...)
			mu.Unlock()
		}(api)
	}
	wg.Wait()
	close(errors)

	for err := range errors {
		errResult = multierror.Append(errResult, err)
	}

	return result, errResult.ErrorOrNil()
}

func (o *CmdOptions) getObjectsByResource(api Resource) ([]unstructuredv1.Unstructured, error) {
	gvr := api.APIGroupVersionResource
	resourceScope := o.ResourceScope

list_objects:
	// If the root object is a namespaced resource, fetch all objects only from
	// the root object's namespace since its dependents cannot exist in other
	// namespaces
	var ri dynamic.ResourceInterface
	if resourceScope == meta.RESTScopeNameRoot {
		ri = o.DynamicClient.Resource(gvr)
	} else {
		ri = o.DynamicClient.Resource(gvr).Namespace(o.Namespace)
	}

	var result []unstructuredv1.Unstructured
	var next string
	for {
		objectList, err := ri.List(context.Background(), metav1.ListOptions{
			Limit:    250,
			Continue: next,
		})
		if err != nil {
			switch {
			case apierrors.IsForbidden(err):
				// If the user doesn't have access to list the resource at the cluster
				// scope, attempt to list the resource in the root object's namespace
				if resourceScope == meta.RESTScopeNameRoot {
					resourceScope = meta.RESTScopeNameNamespace
					goto list_objects
				}
				// If the user doesn't have access to list the resource in the
				// namespace, we abort listing the resource
				return nil, nil
			default:
				if resourceScope == meta.RESTScopeNameRoot {
					err = fmt.Errorf("failed to list resource type \"%s\" in API group \"%s\" at the cluster scope: %w", gvr.Resource, gvr.Group, err)
				} else {
					err = fmt.Errorf("failed to list resource type \"%s\" in API group \"%s\" in the namespace \"%s\": %w", gvr.Resource, gvr.Group, o.Namespace, err)
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
	return result, nil
}

func (o *CmdOptions) print(nodeMap NodeMap, rootUID types.UID) error {
	root, ok := nodeMap[rootUID]
	if !ok {
		return fmt.Errorf("Requested object (uid: %s) not found in list of fetched objects", rootUID)
	}

	// Setup Table Printer
	withGroup := false
	if o.PrintFlags.HumanReadableFlags.ShowGroup != nil {
		withGroup = *o.PrintFlags.HumanReadableFlags.ShowGroup
	}
	// Display namespace column only if objects are in different namespaces
	withNamespace := false
	if o.ResourceScope != meta.RESTScopeNameNamespace {
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
	rows, err := printNode(nodeMap, root, withGroup)
	if err != nil {
		return err
	}
	table := &metav1.Table{
		ColumnDefinitions: objectColumnDefinitions,
		Rows:              rows,
	}
	if err = printer.PrintObj(table, o.Out); err != nil {
		return err
	}

	return nil
}

func resourceFor(mapper meta.RESTMapper, resourceArg string) (schema.GroupVersionResource, error) {
	fullySpecifiedGVR, Resource := schema.ParseResourceArg(strings.ToLower(resourceArg))
	gvr := schema.GroupVersionResource{}
	if fullySpecifiedGVR != nil {
		gvr, _ = mapper.ResourceFor(*fullySpecifiedGVR)
	}
	if gvr.Empty() {
		var err error
		gvr, err = mapper.ResourceFor(Resource.WithVersion(""))
		if err != nil {
			if len(Resource.Group) == 0 {
				err = fmt.Errorf("the server doesn't have a resource type \"%s\"", Resource.Resource)
			} else {
				err = fmt.Errorf("the server doesn't have a resource type \"%s\" in group \"%s\"", Resource.Resource, Resource.Group)
			}
			return schema.GroupVersionResource{Resource: resourceArg}, err
		}
	}

	return gvr, nil
}

func resourceScopeFor(mapper meta.RESTMapper, gvr schema.GroupVersionResource) (meta.RESTScopeName, error) {
	ret := meta.RESTScopeNameNamespace
	gk, err := mapper.KindFor(gvr)
	if gk.Empty() {
		if err != nil {
			if len(gvr.Group) == 0 {
				err = fmt.Errorf("the server couldn't resolve a kind for resource type \"%s\"", gvr.Resource)
			} else {
				err = fmt.Errorf("the server couldn't resolve a kind for resource type \"%s\" in group \"%s\"", gvr.Resource, gvr.Group)
			}
			return ret, err
		}
	}
	mapping, err := mapper.RESTMapping(gk.GroupKind())
	if err != nil {
		if len(gvr.Group) == 0 {
			err = fmt.Errorf("the server couldn't resolve a kind for resource type \"%s\"", gvr.Resource)
		} else {
			err = fmt.Errorf("the server couldn't resolve a kind for resource type \"%s\" in group \"%s\"", gvr.Resource, gvr.Group)
		}
		return ret, err
	}
	ret = mapping.Scope.Name()

	return ret, nil
}

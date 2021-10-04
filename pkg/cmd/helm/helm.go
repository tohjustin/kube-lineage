package helm

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
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
	cmdName    = "helm"
	cmdUse     = "%CMD% [RELEASE_NAME] [flags]"
	cmdExample = templates.Examples(`
		# List all resources associated with release named "bar" in the current namespace
		%CMD_PATH% bar

		# List all resources associated with release named "bar" in namespace "foo"
		%CMD_PATH% bar -n foo

		# List all resources associated with release named "bar" & the corresponding relationship type(s)
		%CMD_PATH% bar -o wide`)
	cmdShort = "Display resources associated with a Helm release & their dependents"
	cmdLong  = templates.LongDesc(`
		Display resources associated with a Helm release & their dependents.

		RELEASE_NAME is the name of a particular Helm release.`)
)

var (
	configmapsResource = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	secretsResource    = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
)

// CmdOptions contains all the options for running the helm command.
type CmdOptions struct {
	// RequestRelease represents the requested release
	RequestRelease string

	HelmDriver    string
	ConfigFlags   *ConfigFlags
	ActionConfig  *action.Configuration
	ClientConfig  *rest.Config
	DynamicClient dynamic.Interface
	Namespace     string

	PrintFlags *lineageprinters.PrintFlags
	ToPrinter  func(withGroup bool, withNamespace bool) (printers.ResourcePrinterFunc, error)

	genericclioptions.IOStreams
}

// NewCmd returns an initialized Command for the helm command.
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
		Example:               strings.ReplaceAll(cmdExample, "%CMD_PATH%", cmdPath),
		Short:                 cmdShort,
		Long:                  cmdLong,
		Args:                  cobra.MaximumNArgs(1),
		DisableFlagsInUseLine: true,
		DisableSuggestions:    true,
		SilenceUsage:          true,
		Run: func(c *cobra.Command, args []string) {
			klog.V(4).Infof("Version: %s", c.Root().Version)
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
func (o *CmdOptions) Complete(cmd *cobra.Command, args []string) error {
	var err error
	var releaseName string
	switch len(args) {
	case 1:
		releaseName = args[0]
	default:
		return fmt.Errorf("release name must be specified\nSee '%s -h' for help and examples", cmdPath)
	}
	o.RequestRelease = releaseName

	o.HelmDriver = os.Getenv("HELM_DRIVER")
	o.Namespace, _, err = o.ConfigFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.ActionConfig = new(action.Configuration)
	err = o.ActionConfig.Init(o.ConfigFlags, o.Namespace, o.HelmDriver, klog.V(4).Infof)
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

// Validate validates all the required options for the helm command.
func (o *CmdOptions) Validate() error {
	if len(o.RequestRelease) == 0 {
		return fmt.Errorf("release NAME must be specified")
	}

	klog.V(4).Infof("Namespace: %s", o.Namespace)
	klog.V(4).Infof("RequestRelease: %v", o.RequestRelease)
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
//nolint:funlen
func (o *CmdOptions) Run() error {
	var err error
	ctx := context.Background()

	// Fetch the release to ensure it exists before proceeding
	client := action.NewGet(o.ActionConfig)
	r, err := client.Run(o.RequestRelease)
	if err != nil {
		return err
	}
	klog.V(4).Infof("Release manifest:\n%s\n", r.Manifest)

	// Fetch all resources in the manifests from the cluster
	objects, err := o.getManifestObjects(r)
	if err != nil {
		return err
	}
	klog.V(4).Infof("Got %d objects from release manifest", len(objects))

	// Construct relationship tree to print out
	rootNode := releaseToNode(r)
	rootUID := rootNode.GetUID()
	nodeMap := graph.NodeMap{rootUID: rootNode}
	for ix, obj := range objects {
		gvk := obj.GroupVersionKind()
		node := graph.Node{
			Unstructured:    &objects[ix],
			UID:             obj.GetUID(),
			Name:            obj.GetName(),
			Namespace:       obj.GetNamespace(),
			Group:           gvk.Group,
			Kind:            gvk.Kind,
			OwnerReferences: obj.GetOwnerReferences(),
			Dependents:      map[types.UID]graph.RelationshipSet{},
		}
		uid := node.UID
		nodeMap[uid] = &node
		rootNode.Dependents[uid] = graph.RelationshipSet{
			graph.RelationshipHelmRelease: {},
		}
	}

	// Fetch the storage object
	stgObj, err := o.getStorageObject(ctx, r)
	if err != nil {
		return err
	}
	if stgObj != nil {
		gvk := stgObj.GroupVersionKind()
		node := graph.Node{
			Unstructured:    stgObj,
			UID:             stgObj.GetUID(),
			Name:            stgObj.GetName(),
			Namespace:       stgObj.GetNamespace(),
			Group:           gvk.Group,
			Kind:            gvk.Kind,
			OwnerReferences: stgObj.GetOwnerReferences(),
			Dependents:      map[types.UID]graph.RelationshipSet{},
		}
		uid := node.UID
		nodeMap[uid] = &node
		rootNode.Dependents[uid] = graph.RelationshipSet{
			graph.RelationshipHelmStorage: {},
		}
	}

	// Print output
	return o.printObj(nodeMap, rootUID)
}

// getManifestObjects fetches all objects found in the manifest of the provided
// Helm release.
func (o *CmdOptions) getManifestObjects(release *release.Release) ([]unstructuredv1.Unstructured, error) {
	var objects []unstructuredv1.Unstructured
	name, ns := release.Name, release.Namespace
	r := strings.NewReader(release.Manifest)
	source := fmt.Sprintf("manifest for release \"%s\" in the namespace \"%s\"", name, ns)
	result := resource.NewBuilder(o.ActionConfig.RESTClientGetter).
		Unstructured().
		NamespaceParam(ns).
		DefaultNamespace().
		ContinueOnError().
		Latest().
		Flatten().
		Stream(r, source).
		Do()
	infos, err := result.Infos()
	if err != nil {
		return nil, err
	}
	for _, info := range infos {
		u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(info.Object)
		if err != nil {
			return nil, err
		}
		objects = append(objects, unstructuredv1.Unstructured{Object: u})
	}

	return objects, nil
}

// getStorageObject fetches the underlying object that stores the information of
// the provided Helm release.
func (o *CmdOptions) getStorageObject(ctx context.Context, r *release.Release) (*unstructuredv1.Unstructured, error) {
	var gvr schema.GroupVersionResource
	switch o.HelmDriver {
	case "secret", "":
		gvr = secretsResource
	case "configmap":
		gvr = configmapsResource
	case "memory", "sql":
		return nil, nil
	default:
		panic("Unknown driver in HELM_DRIVER: " + o.HelmDriver)
	}
	ri := o.DynamicClient.Resource(gvr).Namespace(o.Namespace)
	return ri.Get(ctx, makeKey(r.Name, r.Version), metav1.GetOptions{})
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
	for _, node := range nodeMap {
		if root.Namespace != node.Namespace {
			withNamespace = true
			break
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

// getReleaseReadyStatus returns the ready & status value of a Helm release
// object.
func getReleaseReadyStatus(r *release.Release) (string, string) {
	switch r.Info.Status {
	case release.StatusDeployed:
		return "True", "Deployed"
	case release.StatusFailed:
		return "False", "Failed"
	case release.StatusPendingInstall:
		return "False", "PendingInstall"
	case release.StatusPendingRollback:
		return "False", "PendingRollback"
	case release.StatusPendingUpgrade:
		return "False", "PendingUpgrade"
	case release.StatusSuperseded:
		return "False", "Superseded"
	case release.StatusUninstalled:
		return "False", "Uninstalled"
	case release.StatusUninstalling:
		return "False", "Uninstalling"
	case release.StatusUnknown:
		fallthrough
	default:
		return "False", "Unknown"
	}
}

// releaseToNode converts a Helm release object into a Node in the relationship
// tree.
func releaseToNode(r *release.Release) *graph.Node {
	root := new(unstructuredv1.Unstructured)
	ready, status := getReleaseReadyStatus(r)
	// Set "Ready" condition values based on the printer.objectReadyReasonJSONPath
	// & printer.objectReadyStatusJSONPath paths
	root.SetUnstructuredContent(
		map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": ready,
						"reason": status,
					},
				},
			},
		},
	)
	root.SetName(r.Name)
	root.SetNamespace(r.Namespace)
	root.SetCreationTimestamp(metav1.Time{Time: r.Info.FirstDeployed.Time})
	root.SetUID(types.UID(""))
	return &graph.Node{
		Unstructured: root,
		UID:          root.GetUID(),
		Name:         root.GetName(),
		Namespace:    root.GetNamespace(),
		Dependents:   map[types.UID]graph.RelationshipSet{},
	}
}

// makeKey concatenates the Kubernetes storage object type, a release name and version
// into a string with format:```<helm_storage_type>.<release_name>.v<release_version>```.
// The storage type is prepended to keep name uniqueness between different
// release storage types. An example of clash when not using the type:
// https://github.com/helm/helm/issues/6435.
// This key is used to uniquely identify storage objects.
//
// NOTE: Unfortunately the makeKey function isn't exported by the
// helm.sh/helm/v3/pkg/storage package so we will have to copy-paste it here.
// ref: https://github.com/helm/helm/blob/v3.7.0/pkg/storage/storage.go#L245-L253
func makeKey(rlsname string, version int) string {
	return fmt.Sprintf("%s.%s.v%d", storage.HelmStorageType, rlsname, version)
}

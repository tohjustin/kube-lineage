package helm

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
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

// CmdOptions contains all the options for running the helm command.
type CmdOptions struct {
	// RequestRelease represents the requested release
	RequestRelease string

	ConfigFlags  *ConfigFlags
	ActionConfig *action.Configuration
	Namespace    string

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

	o.Namespace, _, err = o.ConfigFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.ActionConfig = new(action.Configuration)
	err = o.ActionConfig.Init(o.ConfigFlags, o.Namespace, os.Getenv("HELM_DRIVER"), klog.V(4).Infof)
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
func (o *CmdOptions) Run() error {
	var err error

	// Fetch the release to ensure it exists before proceeding
	client := action.NewGet(o.ActionConfig)
	release, err := client.Run(o.RequestRelease)
	if err != nil {
		return err
	}
	klog.V(4).Infof("Release manifest:\n%s\n", release.Manifest)

	// Fetch all resources in the manifests from the cluster
	objects, err := o.getManifestObjects(release)
	if err != nil {
		return err
	}
	klog.V(4).Infof("Got %d objects from release manifest", len(objects))

	// Construct relationship tree to print out
	rootNode := releaseToNode(release)
	rootUID := rootNode.GetUID()
	nodeMap := graph.NodeMap{rootUID: rootNode}
	for ix, o := range objects {
		gvk := o.GroupVersionKind()
		node := graph.Node{
			Unstructured:    &objects[ix],
			UID:             o.GetUID(),
			Name:            o.GetName(),
			Namespace:       o.GetNamespace(),
			Group:           gvk.Group,
			Kind:            gvk.Kind,
			OwnerReferences: o.GetOwnerReferences(),
			Dependents:      map[types.UID]graph.RelationshipSet{},
		}
		uid := node.UID
		nodeMap[uid] = &node
		rootNode.Dependents[uid] = graph.RelationshipSet{
			graph.RelationshipHelmRelease: {},
		}
	}

	// Print output
	return o.printObj(nodeMap, rootUID)
}

// getManifestObjects fetches all objects found in the Helm release's manifest.
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

// releaseToNode converts a Helm release object into a Node in the relationship
// tree.
func releaseToNode(release *release.Release) *graph.Node {
	rootUID := types.UID("")
	root := unstructuredv1.Unstructured{Object: map[string]interface{}{}}
	root.SetName(release.Name)
	root.SetNamespace(release.Namespace)
	root.SetCreationTimestamp(metav1.Time{Time: release.Info.FirstDeployed.Time})
	root.SetUID(rootUID)
	return &graph.Node{
		Unstructured: &root,
		UID:          root.GetUID(),
		Name:         root.GetName(),
		Namespace:    root.GetNamespace(),
		Dependents:   map[types.UID]graph.RelationshipSet{},
	}
}

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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/tohjustin/kube-lineage/internal/client"
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
	// RequestRelease represents the requested Helm release.
	RequestRelease string
	Flags          *Flags

	Namespace    string
	HelmDriver   string
	ActionConfig *action.Configuration
	Client       client.Interface
	ClientFlags  *client.Flags

	Printer    lineageprinters.Interface
	PrintFlags *lineageprinters.Flags

	genericclioptions.IOStreams
}

// NewCmd returns an initialized Command for the helm command.
func NewCmd(streams genericclioptions.IOStreams, name, parentCmdPath string) *cobra.Command {
	o := &CmdOptions{
		Flags:       NewFlags(),
		ClientFlags: client.NewFlags(),
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

	o.Flags.AddFlags(cmd.Flags())
	o.ClientFlags.AddFlags(cmd.Flags())
	o.PrintFlags.AddFlags(cmd.Flags())
	log.AddFlags(cmd.Flags())

	return cmd
}

// Complete completes all the required options for command.
func (o *CmdOptions) Complete(cmd *cobra.Command, args []string) error {
	var err error
	switch len(args) {
	case 1:
		o.RequestRelease = args[0]
	default:
		return fmt.Errorf("release name must be specified\nSee '%s -h' for help and examples", cmdPath)
	}

	// Setup client
	o.Namespace, _, err = o.ClientFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.Client, err = o.ClientFlags.ToClient()
	if err != nil {
		return err
	}
	o.HelmDriver = os.Getenv("HELM_DRIVER")
	o.ActionConfig = new(action.Configuration)
	err = o.ActionConfig.Init(o.ClientFlags, o.Namespace, o.HelmDriver, klog.V(4).Infof)
	if err != nil {
		return err
	}

	// Setup printer
	o.Printer, err = o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	return nil
}

// Validate validates all the required options for the helm command.
func (o *CmdOptions) Validate() error {
	if len(o.RequestRelease) == 0 {
		return fmt.Errorf("release NAME must be specified")
	}

	if o.Client == nil {
		return fmt.Errorf("client must be provided")
	}

	klog.V(4).Infof("Namespace: %s", o.Namespace)
	klog.V(4).Infof("RequestRelease: %v", o.RequestRelease)
	klog.V(4).Infof("Flags.AllNamespaces: %t", *o.Flags.AllNamespaces)
	klog.V(4).Infof("Flags.Depth: %v", *o.Flags.Depth)
	klog.V(4).Infof("Flags.Scopes: %v", *o.Flags.Scopes)
	klog.V(4).Infof("ClientFlags.Context: %s", *o.ClientFlags.Context)
	klog.V(4).Infof("ClientFlags.Namespace: %s", *o.ClientFlags.Namespace)
	klog.V(4).Infof("PrintFlags.OutputFormat: %s", *o.PrintFlags.OutputFormat)
	klog.V(4).Infof("PrintFlags.NoHeaders: %t", *o.PrintFlags.HumanReadableFlags.NoHeaders)
	klog.V(4).Infof("PrintFlags.ShowGroup: %t", *o.PrintFlags.HumanReadableFlags.ShowGroup)
	klog.V(4).Infof("PrintFlags.ShowLabels: %t", *o.PrintFlags.HumanReadableFlags.ShowLabels)
	klog.V(4).Infof("PrintFlags.ShowNamespace: %t", *o.PrintFlags.HumanReadableFlags.ShowNamespace)

	return nil
}

// Run implements all the necessary functionality for command.
//nolint:funlen
func (o *CmdOptions) Run() error {
	ctx := context.Background()

	// First check if Kubernetes cluster is reachable
	if err := o.Client.IsReachable(); err != nil {
		return err
	}

	// Fetch the release to ensure it exists before proceeding
	helmClient := action.NewGet(o.ActionConfig)
	rls, err := helmClient.Run(o.RequestRelease)
	if err != nil {
		return err
	}
	klog.V(4).Infof("Release manifest:\n%s\n", rls.Manifest)

	// Fetch all Helm release objects (i.e. resources found in the helm release
	// manifests) from the cluster
	rlsObjs, err := o.getManifestObjects(ctx, rls)
	if err != nil {
		return err
	}
	klog.V(4).Infof("Got %d objects from release manifest", len(rlsObjs))

	// Fetch the Helm storage object
	stgObj, err := o.getStorageObject(ctx, rls)
	if err != nil {
		return err
	}

	// Determine the namespaces to list objects
	var namespaces []string
	nsSet := map[string]struct{}{o.Namespace: {}}
	for _, obj := range rlsObjs {
		nsSet[obj.GetNamespace()] = struct{}{}
	}
	for ns := range nsSet {
		namespaces = append(namespaces, ns)
	}
	if o.Flags.AllNamespaces != nil && *o.Flags.AllNamespaces {
		namespaces = append(namespaces, "")
	}
	if o.Flags.Scopes != nil {
		namespaces = append(namespaces, *o.Flags.Scopes...)
	}

	// Fetch all resources in the cluster
	objs, err := o.Client.List(ctx, client.ListOptions{Namespaces: namespaces})
	if err != nil {
		return err
	}

	// Include release objects into objects to handle cases where user has access
	// to get the release objects but unable to list its resource type
	objs.Items = append(objs.Items, rlsObjs...)

	// Collect UIDs from release & storage objects
	var uids []types.UID
	for _, obj := range rlsObjs {
		uids = append(uids, obj.GetUID())
	}
	if stgObj != nil {
		uids = append(uids, stgObj.GetUID())
	}

	// Find all dependents of the release & storage objects
	mapper := o.Client.GetMapper()
	nodeMap, err := graph.ResolveDependents(mapper, objs.Items, uids)
	if err != nil {
		return err
	}

	// Add the Helm release object to the root of the relationship tree
	rootNode := newReleaseNode(rls)
	for _, obj := range rlsObjs {
		rootNode.AddDependent(obj.GetUID(), graph.RelationshipHelmRelease)
	}
	if stgObj != nil {
		rootNode.AddDependent(stgObj.GetUID(), graph.RelationshipHelmStorage)
	}
	for _, node := range nodeMap {
		node.Depth++
	}
	rootUID := rootNode.GetUID()
	nodeMap[rootUID] = rootNode

	// Print output
	return o.Printer.Print(o.Out, nodeMap, rootUID, *o.Flags.Depth)
}

// getManifestObjects fetches all objects found in the manifest of the provided
// Helm release.
func (o *CmdOptions) getManifestObjects(_ context.Context, rls *release.Release) ([]unstructuredv1.Unstructured, error) {
	var objs []unstructuredv1.Unstructured
	name, ns := rls.Name, rls.Namespace
	r := strings.NewReader(rls.Manifest)
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
		objs = append(objs, unstructuredv1.Unstructured{Object: u})
	}

	return objs, nil
}

// getStorageObject fetches the underlying object that stores the information of
// the provided Helm release.
func (o *CmdOptions) getStorageObject(ctx context.Context, rls *release.Release) (*unstructuredv1.Unstructured, error) {
	var api client.APIResource
	switch o.HelmDriver {
	case "secret", "":
		api = client.APIResource{Version: "v1", Kind: "Secret", Name: "secrets", Namespaced: true}
	case "configmap":
		api = client.APIResource{Version: "v1", Kind: "ConfigMap", Name: "configmaps", Namespaced: true}
	case "memory", "sql":
		return nil, nil
	default:
		return nil, fmt.Errorf("helm driver \"%s\" not supported", o.HelmDriver)
	}
	return o.Client.Get(ctx, makeKey(rls.Name, rls.Version), client.GetOptions{
		APIResource: api,
		Namespace:   o.Namespace,
	})
}

// getReleaseReadyStatus returns the ready & status value of a Helm release
// object.
func getReleaseReadyStatus(rls *release.Release) (string, string) {
	switch rls.Info.Status {
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

// newReleaseNode converts a Helm release object into a Node in the relationship
// tree.
func newReleaseNode(rls *release.Release) *graph.Node {
	root := new(unstructuredv1.Unstructured)
	ready, status := getReleaseReadyStatus(rls)
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
	root.SetUID(types.UID(""))
	root.SetName(rls.Name)
	root.SetNamespace(rls.Namespace)
	root.SetCreationTimestamp(metav1.Time{Time: rls.Info.FirstDeployed.Time})

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

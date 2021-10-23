package lineage

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/tohjustin/kube-lineage/internal/client"
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
		%CMD_PATH% cronjobs.batch/bar --namespace=foo

		# List all dependents of the node named "k3d-dev-server" & the corresponding relationship type(s)
		%CMD_PATH% node/k3d-dev-server --output=wide

		# List all dependencies of the pod named "bar-5cc79d4bf5-xgvkc"
		%CMD_PATH% pod.v1. bar-5cc79d4bf5-xgvkc --dependencies

		# List all dependencies of the serviceaccount named "default" in the current namespace, grouped by resource type
		%CMD_PATH% sa/default --dependencies --output=split`)
	cmdShort = "Display all dependencies or dependents of a Kubernetes object"
	cmdLong  = templates.LongDesc(`
		Display all dependencies or dependents of a Kubernetes object.

		TYPE is a Kubernetes resource. Shortcuts and groups will be resolved.
		NAME is the name of a particular Kubernetes resource.`)
)

// CmdOptions contains all the options for running the lineage command.
type CmdOptions struct {
	// RequestType represents the type of the requested object.
	RequestType string
	// RequestName represents the name of the requested object.
	RequestName string
	Flags       *Flags

	Namespace   string
	Client      client.Interface
	ClientFlags *client.Flags

	Printer    lineageprinters.Interface
	PrintFlags *lineageprinters.Flags

	genericclioptions.IOStreams
}

// NewCmd returns an initialized Command for the lineage command.
func NewCmd(streams genericclioptions.IOStreams, name, parentCmdPath string) *cobra.Command {
	o := &CmdOptions{
		Flags:       NewFlags(),
		ClientFlags: client.NewFlags(),
		PrintFlags:  lineageprinters.NewFlags(),
		IOStreams:   streams,
	}

	f := cmdutil.NewFactory(o.ClientFlags)
	util.SetFactoryForCompletion(f)

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
			klog.V(4).Infof("Version: %s", c.Root().Version)
			cmdutil.CheckErr(o.Complete(c, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var comps []string
			switch len(args) {
			case 0:
				comps = compGetResourceList(o, toComplete)
			case 1:
				comps = get.CompGetResource(f, cmd, args[0], toComplete)
			}
			return comps, cobra.ShellCompDirectiveNoFileComp
		},
	}

	// Setup flags
	o.Flags.AddFlags(cmd.Flags())
	o.ClientFlags.AddFlags(cmd.Flags())
	o.PrintFlags.AddFlags(cmd.Flags())
	log.AddFlags(cmd.Flags())

	// Setup flag completion function
	o.Flags.RegisterFlagCompletionFunc(cmd, f)
	o.ClientFlags.RegisterFlagCompletionFunc(cmd, f)

	return cmd
}

// Complete completes all the required options for command.
func (o *CmdOptions) Complete(cmd *cobra.Command, args []string) error {
	var err error

	switch len(args) {
	case 1:
		resourceTokens := strings.SplitN(args[0], "/", 2)
		if len(resourceTokens) != 2 {
			return fmt.Errorf("arguments in <resource>/<name> form must have a single resource and name\nSee '%s -h' for help and examples", cmdPath)
		}
		o.RequestType = resourceTokens[0]
		o.RequestName = resourceTokens[1]
	case 2:
		o.RequestType = args[0]
		o.RequestName = args[1]
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

	// Setup printer
	o.Printer, err = o.PrintFlags.ToPrinter(o.Client)
	if err != nil {
		return err
	}

	return nil
}

// Validate validates all the required options for the lineage command.
func (o *CmdOptions) Validate() error {
	if len(o.RequestType) == 0 || len(o.RequestName) == 0 {
		return fmt.Errorf("resource must be specified as <resource> <name> or <resource>/<name>\nSee '%s -h' for help and examples", cmdPath)
	}

	klog.V(4).Infof("Namespace: %s", o.Namespace)
	klog.V(4).Infof("RequestType: %v", o.RequestType)
	klog.V(4).Infof("RequestName: %v", o.RequestName)
	klog.V(4).Infof("Flags.AllNamespaces: %t", *o.Flags.AllNamespaces)
	klog.V(4).Infof("Flags.Dependencies: %t", *o.Flags.Dependencies)
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
func (o *CmdOptions) Run() error {
	ctx := context.Background()

	// First check if Kubernetes cluster is reachable
	if err := o.Client.IsReachable(); err != nil {
		return err
	}

	// Fetch the provided object to ensure it exists before proceeding
	api, err := o.Client.ResolveAPIResource(o.RequestType)
	if err != nil {
		return err
	}
	obj := client.ObjectMeta{
		APIResource: *api,
		Name:        o.RequestName,
		Namespace:   o.Namespace,
	}
	root, err := o.Client.Get(ctx, obj.Name, client.GetOptions{
		APIResource: obj.APIResource,
		Namespace:   o.Namespace,
	})
	if err != nil {
		return err
	}

	// Determine the namespaces to list objects
	namespaces := []string{o.Namespace}
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

	// Include root object into objects to handle cases where user has access
	// to get the root object but unable to list its resource type
	objs.Items = append(objs.Items, *root)

	// Find either all dependencies or dependents of the root object
	depsIsDependencies, resolveDeps := false, graph.ResolveDependents
	if o.Flags.Dependencies != nil && *o.Flags.Dependencies {
		depsIsDependencies, resolveDeps = true, graph.ResolveDependencies
	}
	mapper := o.Client.GetMapper()
	rootUID := root.GetUID()
	nodeMap, err := resolveDeps(mapper, objs.Items, []types.UID{rootUID})
	if err != nil {
		return err
	}

	// Print output
	return o.Printer.Print(o.Out, nodeMap, rootUID, *o.Flags.Depth, depsIsDependencies)
}

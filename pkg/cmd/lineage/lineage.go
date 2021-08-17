package lineage

import (
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var (
	cmdExample = `
	# List all dependents of the deployment named "bar" in the current namespace
	%[1]s lineage deployments bar
`
)

// CmdOptions contains all the options for running the lineage command.
type CmdOptions struct {
	ConfigFlags *genericclioptions.ConfigFlags

	genericclioptions.IOStreams
}

// New returns an initialized Command for the lineage command.
func New(streams genericclioptions.IOStreams) *cobra.Command {
	o := &CmdOptions{
		ConfigFlags: genericclioptions.NewConfigFlags(true),
		IOStreams:   streams,
	}

	cmd := &cobra.Command{
		Use:          "lineage (TYPE[.VERSION][.GROUP] [NAME] | TYPE[.VERSION][.GROUP]/NAME) [flags]",
		Short:        "Display all dependents of a Kubernetes object",
		Example:      cmdExample,
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			return nil
		},
	}

	o.ConfigFlags.AddFlags(cmd.Flags())

	return cmd
}

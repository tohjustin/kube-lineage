package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/tohjustin/kubectl-lineage/pkg/cmd/lineage"
)

func New(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := lineage.New(streams)
	addVersionFlag(cmd)

	return cmd
}

func main() {
	flags := pflag.NewFlagSet("kubectl-lineage", pflag.ExitOnError)
	pflag.CommandLine = flags

	streams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	root := New(streams)

	klog.V(4).Infof("Version: %s", getVersion())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

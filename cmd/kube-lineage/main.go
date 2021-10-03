package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tohjustin/kube-lineage/pkg/cmd/lineage"
)

func New(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := lineage.New(streams)
	addLogFlags(cmd)
	addVersionFlags(cmd)
	return cmd
}

func main() {
	flags := pflag.NewFlagSet("kube-lineage", pflag.ExitOnError)
	pflag.CommandLine = flags

	streams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	rootCmd := New(streams)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

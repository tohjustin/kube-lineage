package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tohjustin/kube-lineage/pkg/cmd/lineage"
)

var rootCmdName = "kube-lineage"

//nolint:gochecknoinits
func init() {
	// If executed as a kubectl plugin
	if strings.HasPrefix(filepath.Base(os.Args[0]), "kubectl-") {
		rootCmdName = "kubectl lineage"
	}
}

func NewCmd(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := lineage.NewCmd(streams, rootCmdName, "")
	addLogFlags(cmd)
	addVersionFlags(cmd)
	return cmd
}

func main() {
	flags := pflag.NewFlagSet("kube-lineage", pflag.ExitOnError)
	pflag.CommandLine = flags

	streams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	rootCmd := NewCmd(streams)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

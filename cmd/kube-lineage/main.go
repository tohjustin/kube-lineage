package main

import (
	"os"

	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tohjustin/kube-lineage/pkg/cmd/lineage"
)

func main() {
	flags := pflag.NewFlagSet("kubectl-lineage", pflag.ExitOnError)
	pflag.CommandLine = flags

	root := lineage.New(genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	})
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

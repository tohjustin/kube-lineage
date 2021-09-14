package lineage

import (
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	flagAllNamespaces          = "all-namespaces"
	flagAllNamespacesShorthand = "A"
)

// ConfigFlags composes common configuration flag structs used in the command.
type ConfigFlags struct {
	*genericclioptions.ConfigFlags
	AllNamespaces *bool
}

// Copy returns a copy of ConfigFlags for mutation.
func (f *ConfigFlags) Copy() ConfigFlags {
	ConfigFlags := *f
	return ConfigFlags
}

// AddFlags receives a *cobra.Command reference and binds flags related to
// configuration to it.
func (f *ConfigFlags) AddFlags(flags *pflag.FlagSet) {
	f.ConfigFlags.AddFlags(flags)

	if f.AllNamespaces != nil {
		flags.BoolVarP(f.AllNamespaces, flagAllNamespaces, flagAllNamespacesShorthand, *f.AllNamespaces, "If present, list object relationships across all namespaces.")
	}
}

// NewConfigFlags returns flags associated with command configuration,
// with default values set.
func NewConfigFlags() *ConfigFlags {
	allNamespaces := false

	return &ConfigFlags{
		ConfigFlags:   genericclioptions.NewConfigFlags(true),
		AllNamespaces: &allNamespaces,
	}
}

package lineage

import (
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// ConfigFlags composes common configuration flag structs used in the command.
type ConfigFlags struct {
	*genericclioptions.ConfigFlags
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
}

// NewConfigFlags returns flags associated with command configuration,s
// with default values set.
func NewConfigFlags() *ConfigFlags {
	return &ConfigFlags{
		ConfigFlags: genericclioptions.NewConfigFlags(true),
	}
}

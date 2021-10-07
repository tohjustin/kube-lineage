package client

import (
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// Flags composes common client configuration flag structs used in the command.
type Flags struct {
	*genericclioptions.ConfigFlags
}

// Copy returns a copy of Flags for mutation.
func (f *Flags) Copy() Flags {
	Flags := *f
	return Flags
}

// AddFlags receives a *cobra.Command reference and binds flags related to
// client configuration to it.
func (f *Flags) AddFlags(flags *pflag.FlagSet) {
	f.ConfigFlags.AddFlags(flags)
}

// NewFlags returns flags associated with client configuration, with default
// values set.
func NewFlags() *Flags {
	return &Flags{
		ConfigFlags: genericclioptions.NewConfigFlags(true),
	}
}

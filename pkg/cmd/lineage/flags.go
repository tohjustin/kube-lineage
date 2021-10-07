package lineage

import (
	"github.com/spf13/pflag"
)

const (
	flagAllNamespaces          = "all-namespaces"
	flagAllNamespacesShorthand = "A"
)

// Flags composes common configuration flag structs used in the command.
type Flags struct {
	AllNamespaces *bool
}

// Copy returns a copy of Flags for mutation.
func (f *Flags) Copy() Flags {
	Flags := *f
	return Flags
}

// AddFlags receives a *cobra.Command reference and binds flags related to
// configuration to it.
func (f *Flags) AddFlags(flags *pflag.FlagSet) {
	if f.AllNamespaces != nil {
		flags.BoolVarP(f.AllNamespaces, flagAllNamespaces, flagAllNamespacesShorthand, *f.AllNamespaces, "If present, list object relationships across all namespaces.")
	}
}

// NewConfigFlags returns flags associated with command configuration,
// with default values set.
func NewFlags() *Flags {
	allNamespaces := false

	return &Flags{
		AllNamespaces: &allNamespaces,
	}
}

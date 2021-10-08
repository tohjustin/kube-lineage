package helm

import (
	"github.com/spf13/pflag"
)

// Flags composes common configuration flag structs used in the command.
type Flags struct {
}

// Copy returns a copy of Flags for mutation.
func (f *Flags) Copy() Flags {
	Flags := *f
	return Flags
}

// AddFlags receives a *cobra.Command reference and binds flags related to
// configuration to it.
func (f *Flags) AddFlags(flags *pflag.FlagSet) {
}

// NewConfigFlags returns flags associated with command configuration,
// with default values set.
func NewFlags() *Flags {
	return &Flags{}
}

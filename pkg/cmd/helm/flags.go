package helm

import (
	"github.com/spf13/pflag"
)

const (
	flagAllNamespaces          = "all-namespaces"
	flagAllNamespacesShorthand = "A"
	flagDepth                  = "depth"
	flagDepthShorthand         = "d"
	flagScopes                 = "scopes"
	flagScopesShorthand        = "S"
)

// Flags composes common configuration flag structs used in the command.
type Flags struct {
	AllNamespaces *bool
	Depth         *uint
	Scopes        *[]string
}

// Copy returns a copy of Flags for mutation.
func (f *Flags) Copy() Flags {
	Flags := *f
	return Flags
}

// AddFlags receives a *pflag.FlagSet reference and binds flags related to
// configuration to it.
func (f *Flags) AddFlags(flags *pflag.FlagSet) {
	if f.AllNamespaces != nil {
		flags.BoolVarP(f.AllNamespaces, flagAllNamespaces, flagAllNamespacesShorthand, *f.AllNamespaces, "If present, list object relationships across all namespaces")
	}
	if f.Depth != nil {
		flags.UintVarP(f.Depth, flagDepth, flagDepthShorthand, *f.Depth, "Maximum depth to find relationships")
	}
	if f.Scopes != nil {
		flags.StringSliceVarP(f.Scopes, flagScopes, flagScopesShorthand, *f.Scopes, "Accepts a comma separated list of additional namespaces to find relationships. You can also use multiple flag options like -S namespace1 -S namespace2...")
	}
}

// NewConfigFlags returns flags associated with command configuration,
// with default values set.
func NewFlags() *Flags {
	allNamespaces := false
	depth := uint(0)
	scopes := []string{}

	return &Flags{
		AllNamespaces: &allNamespaces,
		Depth:         &depth,
		Scopes:        &scopes,
	}
}

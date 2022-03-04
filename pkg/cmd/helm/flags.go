package helm

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/tohjustin/kube-lineage/internal/completion"
)

const (
	flagAllNamespaces          = "all-namespaces"
	flagAllNamespacesShorthand = "A"
	flagDepth                  = "depth"
	flagDepthShorthand         = "d"
	flagExcludeTypes           = "exclude-types"
	flagIncludeTypes           = "include-types"
	flagScopes                 = "scopes"
	flagScopesShorthand        = "S"
)

// Flags composes common configuration flag structs used in the command.
type Flags struct {
	AllNamespaces *bool
	Depth         *uint
	ExcludeTypes  *[]string
	IncludeTypes  *[]string
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
	if f.ExcludeTypes != nil {
		usage := fmt.Sprintf("Accepts a comma separated list of resource types to exclude from relationship discovery. You can also use multiple flag options like --%s kind1 --%s kind1...", flagExcludeTypes, flagExcludeTypes)
		flags.StringSliceVar(f.ExcludeTypes, flagExcludeTypes, *f.ExcludeTypes, usage)
	}
	if f.IncludeTypes != nil {
		usage := fmt.Sprintf("Accepts a comma separated list of resource types to only include in relationship discovery. You can also use multiple flag options like --%s kind1 --%s kind1...", flagIncludeTypes, flagIncludeTypes)
		flags.StringSliceVar(f.IncludeTypes, flagIncludeTypes, *f.IncludeTypes, usage)
	}
	if f.Scopes != nil {
		usage := fmt.Sprintf("Accepts a comma separated list of additional namespaces to find relationships. You can also use multiple flag options like -%s namespace1 -%s namespace2...", flagScopesShorthand, flagScopesShorthand)
		flags.StringSliceVarP(f.Scopes, flagScopes, flagScopesShorthand, *f.Scopes, usage)
	}
}

// RegisterFlagCompletionFunc receives a *cobra.Command & register functions to
// to provide completion for flags related to configuration.
func (*Flags) RegisterFlagCompletionFunc(cmd *cobra.Command, f cmdutil.Factory) {
	cmdutil.CheckErr(cmd.RegisterFlagCompletionFunc(
		flagScopes,
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completion.GetScopeNamespaceList(f, cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
}

// NewConfigFlags returns flags associated with command configuration,
// with default values set.
func NewFlags() *Flags {
	allNamespaces := false
	depth := uint(0)
	excludeTypes := []string{}
	includeTypes := []string{}
	scopes := []string{}

	return &Flags{
		AllNamespaces: &allNamespaces,
		Depth:         &depth,
		ExcludeTypes:  &excludeTypes,
		IncludeTypes:  &includeTypes,
		Scopes:        &scopes,
	}
}

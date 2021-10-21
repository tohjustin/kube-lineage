package client

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util"
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

// AddFlags receives a pflag.FlagSet reference and binds flags related to client
// configuration to it.
func (f *Flags) AddFlags(flags *pflag.FlagSet) {
	f.ConfigFlags.AddFlags(flags)
}

// RegisterFlagCompletionFunc receives a *cobra.Command & register functions to
// to provide completion for flags related to client configuration.
//
// Based off `registerCompletionFuncForGlobalFlags` from
// https://github.com/kubernetes/kubectl/blob/v0.22.1/pkg/cmd/cmd.go#L439-L460
func (*Flags) RegisterFlagCompletionFunc(cmd *cobra.Command, f cmdutil.Factory) {
	cmdutil.CheckErr(cmd.RegisterFlagCompletionFunc(
		"namespace",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return get.CompGetResource(f, cmd, "namespace", toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
	cmdutil.CheckErr(cmd.RegisterFlagCompletionFunc(
		"context",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return util.ListContextsInConfig(toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
	cmdutil.CheckErr(cmd.RegisterFlagCompletionFunc(
		"cluster",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return util.ListClustersInConfig(toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
	cmdutil.CheckErr(cmd.RegisterFlagCompletionFunc(
		"user",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return util.ListUsersInConfig(toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
}

// ToClient returns a client based on the flag configuration.
func (f *Flags) ToClient() (Interface, error) {
	config, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	config.WarningHandler = rest.NoWarnings{}
	config.QPS = clientQPS
	config.Burst = clientBurst
	f.WithDiscoveryBurst(clientBurst)

	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	dis, err := f.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	mapper, err := f.ToRESTMapper()
	if err != nil {
		return nil, err
	}
	c := &client{
		configFlags:     f,
		discoveryClient: dis,
		dynamicClient:   dyn,
		mapper:          mapper,
	}

	return c, nil
}

// NewFlags returns flags associated with client configuration, with default
// values set.
func NewFlags() *Flags {
	return &Flags{
		ConfigFlags: genericclioptions.NewConfigFlags(true),
	}
}

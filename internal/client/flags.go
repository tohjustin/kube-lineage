package client

import (
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
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

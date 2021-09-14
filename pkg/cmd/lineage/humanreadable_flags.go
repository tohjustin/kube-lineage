package lineage

import (
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
)

const (
	flagNoHeaders  = "no-headers"
	flagShowLabels = "show-labels"
	flagShowGroup  = "show-group"
)

// HumanPrintFlags provides default flags necessary for printing. Given the
// following flag values, a printer can be requested that knows how to handle
// printing based on these values.
type HumanPrintFlags struct {
	NoHeaders     *bool
	ShowGroup     *bool
	ShowLabels    *bool
	WithNamespace bool
}

// EnsureWithGroup sets the "ShowGroup" human-readable option to true.
func (f *HumanPrintFlags) EnsureWithGroup() error {
	showGroup := true
	f.ShowGroup = &showGroup
	return nil
}

// EnsureWithNamespace sets the "WithNamespace" human-readable option to true.
func (f *HumanPrintFlags) EnsureWithNamespace() error {
	f.WithNamespace = true
	return nil
}

// AllowedFormats returns more customized formating options.
func (f *HumanPrintFlags) AllowedFormats() []string {
	return []string{"wide"}
}

// ToPrinter receives an outputFormat and returns a printer capable of handling
// human-readable output.
func (f *HumanPrintFlags) ToPrinter(outputFormat string) (printers.ResourcePrinter, error) {
	if len(outputFormat) > 0 && outputFormat != "wide" {
		return nil, genericclioptions.NoCompatiblePrinterError{Options: f, AllowedFormats: f.AllowedFormats()}
	}
	noHeaders := false
	if f.NoHeaders != nil {
		noHeaders = *f.NoHeaders
	}
	showLabels := false
	if f.ShowLabels != nil {
		showLabels = *f.ShowLabels
	}
	p := printers.NewTablePrinter(printers.PrintOptions{
		NoHeaders:     noHeaders,
		ShowLabels:    showLabels,
		Wide:          outputFormat == "wide",
		WithNamespace: f.WithNamespace,
	})
	return p, nil
}

// AddFlags receives a *cobra.Command reference and binds flags related to
// human-readable printing to it.
func (f *HumanPrintFlags) AddFlags(flags *pflag.FlagSet) {
	if f.NoHeaders != nil {
		flags.BoolVar(f.NoHeaders, flagNoHeaders, *f.NoHeaders, "When using the default output format, don't print headers (default print headers).")
	}
	if f.ShowLabels != nil {
		flags.BoolVar(f.ShowLabels, flagShowLabels, *f.ShowLabels, "When printing, show all labels as the last column (default hide labels column)")
	}
	if f.ShowGroup != nil {
		flags.BoolVar(f.ShowGroup, flagShowGroup, *f.ShowGroup, "If present, include the resource group for the requested object(s).")
	}
}

// NewHumanPrintFlags returns flags associated with human-readable printing,
// with default values set.
func NewHumanPrintFlags() *HumanPrintFlags {
	noHeaders := false
	showGroup := false
	showLabels := false

	return &HumanPrintFlags{
		NoHeaders:     &noHeaders,
		ShowGroup:     &showGroup,
		ShowLabels:    &showLabels,
		WithNamespace: false,
	}
}

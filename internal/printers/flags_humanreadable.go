package printers

import (
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
)

const (
	flagColumnLabels          = "label-columns"
	flagColumnLabelsShorthand = "L"
	flagNoHeaders             = "no-headers"
	flagShowGroup             = "show-group"
	flagShowLabels            = "show-labels"
	flagShowNamespace         = "show-namespace"
)

// HumanPrintFlags provides default flags necessary for printing. Given the
// following flag values, a printer can be requested that knows how to handle
// printing based on these values.
type HumanPrintFlags struct {
	ColumnLabels  *[]string
	NoHeaders     *bool
	ShowGroup     *bool
	ShowLabels    *bool
	ShowNamespace *bool
}

// EnsureWithGroup sets the "ShowGroup" human-readable option to true.
func (f *HumanPrintFlags) EnsureWithGroup() error {
	showGroup := true
	f.ShowGroup = &showGroup
	return nil
}

// EnsureWithNamespace sets the "ShowNamespace" human-readable option to true.
func (f *HumanPrintFlags) EnsureWithNamespace() error {
	showNamespace := true
	f.ShowNamespace = &showNamespace
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
	columnLabels := []string{}
	if f.ColumnLabels != nil {
		columnLabels = *f.ColumnLabels
	}
	noHeaders := false
	if f.NoHeaders != nil {
		noHeaders = *f.NoHeaders
	}
	showLabels := false
	if f.ShowLabels != nil {
		showLabels = *f.ShowLabels
	}
	showNamespace := false
	if f.ShowLabels != nil {
		showNamespace = *f.ShowNamespace
	}
	p := printers.NewTablePrinter(printers.PrintOptions{
		ColumnLabels:  columnLabels,
		NoHeaders:     noHeaders,
		ShowLabels:    showLabels,
		Wide:          outputFormat == "wide",
		WithNamespace: showNamespace,
	})
	return p, nil
}

// AddFlags receives a *cobra.Command reference and binds flags related to
// human-readable printing to it.
func (f *HumanPrintFlags) AddFlags(flags *pflag.FlagSet) {
	if f.ColumnLabels != nil {
		flags.StringSliceVarP(f.ColumnLabels, flagColumnLabels, flagColumnLabelsShorthand, *f.ColumnLabels, "Accepts a comma separated list of labels that are going to be presented as columns. Names are case-sensitive. You can also use multiple flag options like -L label1 -L label2...")
	}
	if f.NoHeaders != nil {
		flags.BoolVar(f.NoHeaders, flagNoHeaders, *f.NoHeaders, "When using the default output format, don't print headers (default print headers)")
	}
	if f.ShowGroup != nil {
		flags.BoolVar(f.ShowGroup, flagShowGroup, *f.ShowGroup, "If present, include the resource group for the requested object(s)")
	}
	if f.ShowLabels != nil {
		flags.BoolVar(f.ShowLabels, flagShowLabels, *f.ShowLabels, "When printing, show all labels as the last column (default hide labels column)")
	}
	if f.ShowNamespace != nil {
		flags.BoolVar(f.ShowNamespace, flagShowNamespace, *f.ShowNamespace, "When printing, show namespace as the first column (default hide namespace column if all objects are in the same namespace)")
	}
}

// NewHumanPrintFlags returns flags associated with human-readable printing,
// with default values set.
func NewHumanPrintFlags() *HumanPrintFlags {
	columnLabels := []string{}
	noHeaders := false
	showGroup := false
	showLabels := false
	showNamespace := false

	return &HumanPrintFlags{
		ColumnLabels:  &columnLabels,
		NoHeaders:     &noHeaders,
		ShowGroup:     &showGroup,
		ShowLabels:    &showLabels,
		ShowNamespace: &showNamespace,
	}
}

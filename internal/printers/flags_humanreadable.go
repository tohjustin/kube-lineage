package printers

import (
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
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

// List of supported table output formats.
const (
	outputFormatWide      = "wide"
	outputFormatSplit     = "split"
	outputFormatSplitWide = "split-wide"
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
func (f *HumanPrintFlags) EnsureWithGroup() {
	showGroup := true
	f.ShowGroup = &showGroup
}

// SetShowNamespace sets the "ShowNamespace" human-readable option.
func (f *HumanPrintFlags) SetShowNamespace(b bool) {
	f.ShowNamespace = &b
}

// AllowedFormats returns more customized formating options.
func (f *HumanPrintFlags) AllowedFormats() []string {
	return []string{
		outputFormatWide,
		outputFormatSplit,
		outputFormatSplitWide,
	}
}

// IsSupportedOutputFormat returns true if provided output format is supported.
func (f *HumanPrintFlags) IsSupportedOutputFormat(outputFormat string) bool {
	return sets.NewString(f.AllowedFormats()...).Has(outputFormat)
}

// IsSplitOutputFormat returns true if provided output format is a split table
// format.
func (f *HumanPrintFlags) IsSplitOutputFormat(outputFormat string) bool {
	return outputFormat == outputFormatSplit || outputFormat == outputFormatSplitWide
}

// IsWideOutputFormat returns true if provided output format is a wide table
// format.
func (f *HumanPrintFlags) IsWideOutputFormat(outputFormat string) bool {
	return outputFormat == outputFormatWide || outputFormat == outputFormatSplitWide
}

// ToPrinter receives an outputFormat and returns a printer capable of handling
// human-readable output.
func (f *HumanPrintFlags) ToPrinterWithGK(outputFormat string, gk schema.GroupKind) (printers.ResourcePrinter, error) {
	if len(outputFormat) > 0 && !f.IsSupportedOutputFormat(outputFormat) {
		return nil, genericclioptions.NoCompatiblePrinterError{
			Options:        f,
			AllowedFormats: f.AllowedFormats(),
		}
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
		Kind:          gk,
		WithKind:      !gk.Empty(),
		Wide:          f.IsWideOutputFormat(outputFormat),
		WithNamespace: showNamespace,
	})
	return p, nil
}

// ToPrinter receives an outputFormat and returns a printer capable of handling
// human-readable output.
func (f *HumanPrintFlags) ToPrinter(outputFormat string) (printers.ResourcePrinter, error) {
	return f.ToPrinterWithGK(outputFormat, schema.GroupKind{})
}

// AddFlags receives a *pflag.FlagSet reference and binds flags related to
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

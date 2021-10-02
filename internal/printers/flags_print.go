package printers

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
)

const (
	flagOutputFormat = "output"
)

// PrintFlags composes common printer flag structs used in the command.
type PrintFlags struct {
	HumanReadableFlags *HumanPrintFlags
	OutputFormat       *string
}

// AllowedFormats is the list of formats in which data can be displayed.
func (f *PrintFlags) AllowedFormats() []string {
	formats := []string{}
	formats = append(formats, f.HumanReadableFlags.AllowedFormats()...)
	return formats
}

// Copy returns a copy of PrintFlags for mutation.
func (f *PrintFlags) Copy() PrintFlags {
	printFlags := *f
	return printFlags
}

// EnsureWithGroup ensures that human-readable flags return a printer capable of
// including resource kinds.
func (f *PrintFlags) EnsureWithGroup() error {
	return f.HumanReadableFlags.EnsureWithGroup()
}

// EnsureWithNamespace ensures that human-readable flags return a printer capable
// of printing with a "namespace" column.
func (f *PrintFlags) EnsureWithNamespace() error {
	return f.HumanReadableFlags.EnsureWithNamespace()
}

// ToPrinter attempts to find a composed set of PrintFlags suitable for
// returning a printer based on current flag values.
func (f *PrintFlags) ToPrinter() (printers.ResourcePrinter, error) {
	outputFormat := ""
	if f.OutputFormat != nil {
		outputFormat = *f.OutputFormat
	}

	p, err := f.HumanReadableFlags.ToPrinter(outputFormat)
	if !genericclioptions.IsNoCompatiblePrinterError(err) {
		return p, err
	}

	return nil, genericclioptions.NoCompatiblePrinterError{
		AllowedFormats: f.AllowedFormats(),
		OutputFormat:   &outputFormat,
	}
}

// AddFlags receives a *cobra.Command reference and binds flags related to
// human-readable printing to it.
func (f *PrintFlags) AddFlags(flags *pflag.FlagSet) {
	f.HumanReadableFlags.AddFlags(flags)

	if f.OutputFormat != nil {
		flags.StringVarP(f.OutputFormat, flagOutputFormat, "o", *f.OutputFormat, fmt.Sprintf("Output format. One of: %s.", strings.Join(f.AllowedFormats(), "|")))
	}
}

// NewFlags returns flags associated with human-readable printing, with default
// values set.
func NewFlags() *PrintFlags {
	outputFormat := ""

	return &PrintFlags{
		OutputFormat:       &outputFormat,
		HumanReadableFlags: NewHumanPrintFlags(),
	}
}

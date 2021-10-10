package printers

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tohjustin/kube-lineage/internal/client"
)

const (
	flagOutputFormat = "output"
)

// Flags composes common printer flag structs used in the command.
type Flags struct {
	HumanReadableFlags *HumanPrintFlags
	OutputFormat       *string
}

// AddFlags receives a *cobra.Command reference and binds flags related to
// human-readable printing to it.
func (f *Flags) AddFlags(flags *pflag.FlagSet) {
	f.HumanReadableFlags.AddFlags(flags)

	if f.OutputFormat != nil {
		flags.StringVarP(f.OutputFormat, flagOutputFormat, "o", *f.OutputFormat, fmt.Sprintf("Output format. One of: %s.", strings.Join(f.AllowedFormats(), "|")))
	}
}

// AllowedFormats is the list of formats in which data can be displayed.
func (f *Flags) AllowedFormats() []string {
	formats := []string{}
	formats = append(formats, f.HumanReadableFlags.AllowedFormats()...)
	return formats
}

// Copy returns a copy of Flags for mutation.
func (f *Flags) Copy() Flags {
	printFlags := *f
	return printFlags
}

// EnsureWithGroup ensures that human-readable flags return a printer capable of
// including resource group.
func (f *Flags) EnsureWithGroup() {
	f.HumanReadableFlags.EnsureWithGroup()
}

// IsTableOutputFormat returns true if provided output format is a table format.
func (f *Flags) IsTableOutputFormat(outputFormat string) bool {
	return f.HumanReadableFlags.IsSupportedOutputFormat(outputFormat)
}

// SetShowNamespace configures whether human-readable flags return a printer
// capable of printing with a "namespace" column.
func (f *Flags) SetShowNamespace(b bool) {
	f.HumanReadableFlags.SetShowNamespace(b)
}

// ToPrinter returns a printer based on current flag values.
func (f *Flags) ToPrinter(client client.Interface) (Interface, error) {
	outputFormat := ""
	if f.OutputFormat != nil {
		outputFormat = *f.OutputFormat
	}

	var printer Interface
	switch {
	case f.IsTableOutputFormat(outputFormat):
		configFlags := f.Copy()
		printer = &tablePrinter{
			configFlags:  configFlags.HumanReadableFlags,
			outputFormat: outputFormat,
			client:       client,
		}
	default:
		return nil, genericclioptions.NoCompatiblePrinterError{
			AllowedFormats: f.AllowedFormats(),
			OutputFormat:   &outputFormat,
		}
	}

	return printer, nil
}

// NewFlags returns flags associated with human-readable printing, with default
// values set.
func NewFlags() *Flags {
	outputFormat := ""

	return &Flags{
		OutputFormat:       &outputFormat,
		HumanReadableFlags: NewHumanPrintFlags(),
	}
}

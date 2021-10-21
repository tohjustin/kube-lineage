package lineage

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// compGetResourceList provides dynamic auto-completion for resource names.
func compGetResourceList(opts *CmdOptions, toComplete string) []string {
	cobra.CompDebugln(fmt.Sprintf("compGetResourceList with \"%s\"", toComplete), false)
	if err := opts.Complete(nil, nil); err != nil {
		return nil
	}

	var choices []string
	apis, err := opts.Client.GetAPIResources(context.Background())
	if err != nil {
		cobra.CompErrorln(fmt.Sprintf("Failed to list API resources: %s", err))
		return nil
	}
	for _, api := range apis {
		choices = append(choices, api.WithGroupString())
	}
	if len(choices) == 0 {
		cobra.CompDebugln("No API resources found", false)
		return nil
	}

	return choices
}

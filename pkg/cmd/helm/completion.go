package helm

import (
	"fmt"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
)

// Provide dynamic auto-completion for release names.
//
// Based off `compListReleases` from https://github.com/helm/helm/blob/v3.7.0/cmd/helm/list.go#L221-L243
func compGetHelmReleaseList(opts *CmdOptions, toComplete string) []string {
	cobra.CompDebugln(fmt.Sprintf("completeHelm with \"%s\"", toComplete), false)
	if err := opts.Complete(nil, nil); err != nil {
		return nil
	}
	helmClient := action.NewList(opts.ActionConfig)
	helmClient.All = true
	helmClient.Limit = 0
	helmClient.Filter = fmt.Sprintf("^%s", toComplete)
	helmClient.SetStateMask()

	var choices []string
	releases, err := helmClient.Run()
	if err != nil {
		cobra.CompErrorln(fmt.Sprintf("Failed to list releases: %s", err))
		return nil
	}
	for _, rel := range releases {
		choices = append(choices,
			fmt.Sprintf("%s\t%s-%s -> %s", rel.Name, rel.Chart.Metadata.Name, rel.Chart.Metadata.Version, rel.Info.Status.String()))
	}
	if len(choices) == 0 {
		cobra.CompDebugln("No releases found", false)
		return nil
	}

	return choices
}

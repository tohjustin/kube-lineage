package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tohjustin/kube-lineage/internal/log"
	"github.com/tohjustin/kube-lineage/internal/version"
)

func addLogFlags(cmd *cobra.Command) {
	log.AddFlags(cmd.Flags())
}

func addVersionFlags(cmd *cobra.Command) {
	cmd.SetVersionTemplate("{{printf \"%s\" .Version}}\n")
	cmd.Version = fmt.Sprintf("%#v", version.Get())
}

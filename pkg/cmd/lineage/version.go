package lineage

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tohjustin/kube-lineage/internal/version"
)

func getVersion() string {
	return fmt.Sprintf("%#v", version.Get())
}

func addVersionFlag(c *cobra.Command) {
	c.Version = getVersion()
	c.SetVersionTemplate("{{printf \"%s\" .Version}}\n")
}

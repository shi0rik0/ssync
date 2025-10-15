package cli

import (
	"github.com/shi0rik0/ssync/internal/core"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update <directory> <old-manifest> <new-manifest>",
	Short: "Updates a manifest file.",
	Args:  cobra.ExactArgs(3),
	Run:   core.Update,
}

package cli

import (
	"github.com/shi0rik0/ssync/internal/core"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <directory> <manifest>",
	Short: "Generates a manifest file for a directory.",
	Args:  cobra.ExactArgs(2),
	Run:   core.Create,
}

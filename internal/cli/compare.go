package cli

import (
	"github.com/shi0rik0/ssync/internal/core"
	"github.com/spf13/cobra"
)

var compareCmd = &cobra.Command{
	Use:   "compare <directory1> <directory2>",
	Short: "Compares two directories and outputs differences.",
	Args:  cobra.ExactArgs(2),
	Run:   core.Compare,
}

func init() {
	compareCmd.Flags().BoolVarP(new(bool), "strict", "s", false, "Perform a strict comparison.")
}

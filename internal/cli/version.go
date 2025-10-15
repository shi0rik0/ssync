package cli

import (
	"fmt"

	"github.com/shi0rik0/ssync/internal/core"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display the version of ssync",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ssync version %s\n", core.ProgramVersion)
	},
}

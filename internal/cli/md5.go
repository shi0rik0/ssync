package cli

import (
	"github.com/shi0rik0/ssync/internal/core"
	"github.com/spf13/cobra"
)

var md5Cmd = &cobra.Command{
	Use:   "md5 <file>",
	Short: "Calculates the MD5 checksum of a file.",
	Args:  cobra.ExactArgs(1),
	Run:   core.MD5,
}

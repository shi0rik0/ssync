package cli

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "ssync",
	Short: "A simple file synchronization utility.",
	Long:  `ssync is a command-line tool for synchronizing files.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// 将所有子命令添加到根命令
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(compareCmd)
	rootCmd.AddCommand(md5Cmd)
	rootCmd.AddCommand(versionCmd)
}

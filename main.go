package main

import (
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "ssync",
	Short: "A simple synchronization utility.",
	Long:  `ssync is a command-line tool for managing synchronization tasks.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// Log and exit on error.
		log.Printf("Error executing command: %v", err)
		os.Exit(1)
	}
}

func main() {
	logrus.SetLevel(logrus.DebugLevel) // Set log level to debug for detailed output.

	Execute()
}

func init() {
	// Add subcommands to the root command.
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(verifyCmd)

	// Add flags to the commands.
	var updateQuickFlag bool
	updateCmd.Flags().BoolVarP(&updateQuickFlag, "quick", "q", false, "Perform a quick update without extensive checks.")
	var verifyQuickFlag bool
	verifyCmd.Flags().BoolVarP(&verifyQuickFlag, "quick", "q", false, "Perform a quick verification without extensive checks.")
}

var createCmd = &cobra.Command{
	Use:   "create <directory> <manifest>",
	Short: "Generates a manifest file for a directory.",
	Args:  cobra.ExactArgs(2), // Enforces exactly 2 arguments.
	Run:   runCreate,
}

var updateCmd = &cobra.Command{
	Use:   "update <directory> <old-manifest> <new-manifest>",
	Short: "Updates a manifest file.",
	Args:  cobra.ExactArgs(3), // Enforces exactly 3 arguments.
	Run:   runUpdate,
}

var verifyCmd = &cobra.Command{
	Use:   "verify <directory> <manifest>",
	Short: "Verifies the integrity of a manifest file against a directory.",
	Args:  cobra.ExactArgs(2), // Enforces exactly 2 arguments.
	Run:   runVerify,
}

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func execute() {
	// rootCmd represents the base command when called without any subcommands.
	// It is defined here to avoid being a global variable.
	var rootCmd = &cobra.Command{
		Use:   "ssync",
		Short: "A simple synchronization utility.",
		Long:  `ssync is a command-line tool for managing synchronization tasks.`,
	}

	// createCmd defines the 'create' subcommand.
	var createCmd = &cobra.Command{
		Use:   "create <directory> <manifest>",
		Short: "Generates a manifest file for a directory.",
		Args:  cobra.ExactArgs(2), // Enforces exactly 2 arguments.
		Run:   runCreate,
	}

	// updateCmd defines the 'update' subcommand.
	var updateCmd = &cobra.Command{
		Use:   "update <directory> <old-manifest> <new-manifest>",
		Short: "Updates a manifest file.",
		Args:  cobra.ExactArgs(3), // Enforces exactly 3 arguments.
		Run:   runUpdate,
	}

	// verifyCmd defines the 'verify' subcommand.
	var verifyCmd = &cobra.Command{
		Use:   "verify <directory> <manifest>",
		Short: "Verifies the integrity of a manifest file against a directory.",
		Args:  cobra.ExactArgs(2), // Enforces exactly 2 arguments.
		Run:   runVerify,
	}

	// Add subcommands to the root command.
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(verifyCmd)

	// Add flags to the respective commands.
	// The flag variables are not needed globally as their values are accessed via cmd.Flags().GetBool().
	updateCmd.Flags().BoolVarP(new(bool), "quick", "q", false, "Perform a quick update without extensive checks.")
	verifyCmd.Flags().BoolVarP(new(bool), "quick", "q", false, "Perform a quick verification without extensive checks.")

	// Execute the root command.
	if err := rootCmd.Execute(); err != nil {
		logrus.Errorf("Error executing command: %v", err)
		os.Exit(1)
	}
}

func main() {
	logLevel := strings.ToLower(os.Getenv("LOG"))
	if logLevel != "" {
		fmt.Printf("You have set the log level to '%s'\n", logLevel)
		switch logLevel {
		case "debug":
			logrus.SetLevel(logrus.DebugLevel)
		case "info":
			logrus.SetLevel(logrus.InfoLevel)
		case "warn":
			logrus.SetLevel(logrus.WarnLevel)
		case "error":
			logrus.SetLevel(logrus.ErrorLevel)
		default:
			fmt.Printf("Unknown log level '%s', defaulting to 'info'\n", logLevel)
			logrus.SetLevel(logrus.InfoLevel)
		}
	} else {
		logrus.SetOutput(io.Discard) // Disable logging if LOG is not set.
	}

	// Call execute to set up and run the CLI.
	execute()
}

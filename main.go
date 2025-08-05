package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const ProgramVersion = "0.1.0-alpha5"

func execute() {
	var rootCmd = &cobra.Command{
		Use:   "ssync",
		Short: "A simple file synchronization utility.",
		Long:  `ssync is a command-line tool for synchronizing files.`,
	}

	// Create subcommands.
	var createCmd = &cobra.Command{
		Use:   "create <directory> <manifest>",
		Short: "Generates a manifest file for a directory.",
		Args:  cobra.ExactArgs(2),
		Run:   runCreate,
	}

	var updateCmd = &cobra.Command{
		Use:   "update <directory> <old-manifest> <new-manifest>",
		Short: "Updates a manifest file.",
		Args:  cobra.ExactArgs(3),
		Run:   runUpdate,
	}

	var compareCmd = &cobra.Command{
		Use:   "compare <directory1> <directory2>",
		Short: "Compares two directories and outputs differences.",
		Args:  cobra.ExactArgs(2),
		Run:   runCompare,
	}
	compareCmd.Flags().BoolVarP(new(bool), "strict", "s", false, "Perform a strict comparison.")

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Display the version of ssync",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("ssync version %s\n", ProgramVersion)
		},
	}

	// Add subcommands to the root command.
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(compareCmd)
	rootCmd.AddCommand(versionCmd)

	// Execute the root command.
	if err := rootCmd.Execute(); err != nil {
		logrus.Errorf("Error executing command: %v", err)
		os.Exit(1)
	}
}

func main() {
	// Set up logrus logging.
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

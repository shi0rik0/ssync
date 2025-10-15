package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/shi0rik0/ssync/internal/cli"
	"github.com/sirupsen/logrus"
)

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

	if err := cli.Execute(); err != nil {
		log.Fatal(err)
	}
}

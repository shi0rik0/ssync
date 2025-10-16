package core

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func MD5(cmd *cobra.Command, args []string) {
	filePath := args[0]
	logrus.Debugf("Executing 'md5' command with file: '%s'", filePath)

	if filePath == "?" {
		filePath2, err := openSelectFileDialog("Select File to Calculate MD5", "All files", "*")
		if err != nil {
			logrus.Errorf("Error selecting file: %v", err)
			return
		}
		filePath = filePath2
	}

	runMD5(filePath)
}

func runMD5(filePath string) {
	hash, err := calculateMD5(filePath)
	if err != nil {
		logrus.Errorf("Error computing MD5 for file %s: %v", filePath, err)
		return
	}
	fmt.Printf("MD5 checksum for file %s: %s\n", filePath, hash)
}

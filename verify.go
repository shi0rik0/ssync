package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func runVerify(cmd *cobra.Command, args []string) {
	quickFlag, err := cmd.Flags().GetBool("quick") // Retrieve the quick flag value if needed.
	if err != nil {
		fmt.Printf("Error retrieving quick flag: %v\n", err)
		return
	}

	// Extract arguments.
	directoryPath := args[0]
	manifestPath := args[1]
	logrus.Debugf("Executing 'verify' command with directory: '%s', manifest: '%s', quick: %v", directoryPath, manifestPath, quickFlag)

	manifestSlice, err := readManifest(manifestPath)
	if err != nil {
		fmt.Printf("Error reading manifest: %v\n", err)
		return
	}

	manifestMap := make(map[string]FileInfo)
	for _, fileInfo := range manifestSlice {
		manifestMap[fileInfo.Path] = fileInfo
	}

	filepathWalkDataChan := make(chan filepathWalkData)
	var wg sync.WaitGroup // WaitGroup to synchronize worker goroutines.

	// Launch worker goroutines.
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1) // Increment counter for each worker.
		go func() {
			defer wg.Done() // Decrement counter when worker exits.
			for data := range filepathWalkDataChan {
				// Process the file.
				path := data.Path
				relativePath := data.RelativePath
				modifiedTime := data.Info.ModTime()
				size := data.Info.Size()

				fileInfo, exists := manifestMap[relativePath]
				if !exists {
					fmt.Printf("[+] %s\n", relativePath)
					continue
				}

				if quickFlag {
					// In quick mode, only check mtime and size.
					if modifiedTime.Unix() != fileInfo.ModifiedTime.Unix() || size != fileInfo.Size {
						fmt.Printf("[M] %s\n", relativePath)
					}
				} else {
					hash, err := calculateMD5(path)
					if err != nil {
						panic(fmt.Sprintf("Error calculating MD5 for file %s: %v", path, err))
					}
					if modifiedTime.Unix() != fileInfo.ModifiedTime.Unix() || size != fileInfo.Size || hash != fileInfo.Hash {
						fmt.Printf("[M] %s\n", relativePath)
					}
				}

			}
		}()
	}

	accessedPathSlice := make([]string, 0)

	// Traverse the directory and send file paths to the channel.
	err = filepath.Walk(directoryPath, func(path string, info os.FileInfo, err error) error {
		// if err != nil {
		// 	return fmt.Errorf("error accessing path %q: %v", path, err)
		// }

		relativePath, err := filepath.Rel(directoryPath, path) // Make the path relative to the directory.
		relativePath = filepath.ToSlash(relativePath)          // Convert to forward slashes for consistency.
		if err != nil {
			return fmt.Errorf("error accessing path %q: %v", path, err)
		}
		if info.IsDir() {
			// fmt.Printf("Directory: %s\n", path)
		} else {
			// fmt.Printf("File: %s\n", path)
			filepathWalkDataChan <- filepathWalkData{Path: path, RelativePath: relativePath, Info: info}
			accessedPathSlice = append(accessedPathSlice, relativePath) // Collect accessed paths.
		}
		return nil
	})

	// Close the channel after all paths have been sent.
	// This signals to worker goroutines that no more data will be sent.
	close(filepathWalkDataChan)

	wg.Wait()

	if err != nil {
		fmt.Printf("Error traversing directory: %v\n", err)
		return
	}

	accessedPathSet := make(map[string]bool)
	for _, path := range accessedPathSlice {
		accessedPathSet[path] = true
	}

	// Check for files in the manifest that were not accessed.
	for _, fileInfo := range manifestSlice {
		if !accessedPathSet[fileInfo.Path] {
			fmt.Printf("[-] %s\n", fileInfo.Path)
		}
	}
}

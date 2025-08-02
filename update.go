package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"

	"github.com/manifoldco/promptui"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func runUpdate(cmd *cobra.Command, args []string) {
	quickFlag, err := cmd.Flags().GetBool("quick")
	if err != nil {
		fmt.Printf("Error retrieving quick flag: %v\n", err)
		return
	}

	// Extract arguments.
	directoryPath := args[0]
	oldManifestPath := args[1]
	newManifestPath := args[2]
	logrus.Debugf("Executing 'update' command with directory: '%s', old manifest: '%s', new manifest: '%s'", directoryPath, oldManifestPath, newManifestPath)

	// Check if newManifestPath already exists
	if _, err := os.Stat(newManifestPath); err == nil {
		// If it exists, prompt the user for confirmation to overwrite.
		prompt := promptui.Prompt{
			Label:     "Manifest file already exists. Overwrite?",
			IsConfirm: true,
		}
		if _, err := prompt.Run(); err != nil {
			fmt.Println("Operation cancelled.")
			return
		}
	}

	file, err := createFile(newManifestPath)
	if err != nil {
		fmt.Printf("Error creating new manifest file: %v\n", err)
		return
	}
	defer file.Close()

	oldManifestSlice, err := readManifest(oldManifestPath)
	if err != nil {
		fmt.Printf("Error reading manifest: %v\n", err)
		return
	}

	oldManifestMap := make(map[string]FileInfo)
	for _, fileInfo := range oldManifestSlice {
		oldManifestMap[fileInfo.Path] = fileInfo
	}

	newManifestSlice := make([]FileInfo, 0)

	filepathWalkDataChan := make(chan filepathWalkData)
	fileInfoChan := make(chan FileInfo)
	var wg sync.WaitGroup // WaitGroup to synchronize worker goroutines.

	// Launch worker goroutines.
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1) // Increment counter for each worker.
		go func() {
			defer wg.Done() // Decrement counter when worker exits.

			for data := range filepathWalkDataChan {
				logrus.Debugf("Processing file: %s", data.Path)
				// Process the file.
				path := data.Path
				relativePath := data.RelativePath
				modifiedTime := data.Info.ModTime()
				size := data.Info.Size()

				fileInfo, exists := oldManifestMap[relativePath]
				if !exists {
					hash, err := calculateMD5(path)
					if err != nil {
						panic(fmt.Sprintf("Error calculating MD5 for file %s: %v", path, err))
					}
					fileInfo = FileInfo{
						Path:         relativePath,
						ModifiedTime: modifiedTime,
						Size:         size,
						Hash:         hash,
					}
					fileInfoChan <- fileInfo
					continue
				}

				if quickFlag {
					// In quick mode, only check mtime and size.
					if modifiedTime.Unix() != fileInfo.ModifiedTime.Unix() || size != fileInfo.Size {
						hash, err := calculateMD5(path)
						if err != nil {
							panic(fmt.Sprintf("Error calculating MD5 for file %s: %v", path, err))
						}
						fileInfo = FileInfo{
							Path:         relativePath,
							ModifiedTime: modifiedTime,
							Size:         size,
							Hash:         hash,
						}
						fileInfoChan <- fileInfo
					} else {
						// If the file hasn't changed, use the old manifest entry.
						fileInfo = FileInfo{
							Path:         relativePath,
							ModifiedTime: modifiedTime,
							Size:         size,
							Hash:         fileInfo.Hash,
						}
						fileInfoChan <- fileInfo
					}
				} else {
					hash, err := calculateMD5(path)
					if err != nil {
						panic(fmt.Sprintf("Error calculating MD5 for file %s: %v", path, err))
					}
					fileInfo = FileInfo{
						Path:         relativePath,
						ModifiedTime: modifiedTime,
						Size:         size,
						Hash:         hash,
					}
					fileInfoChan <- fileInfo
				}
			}
		}()
	}

	go func() {
		// This goroutine collects file info.
		for fileInfo := range fileInfoChan {
			newManifestSlice = append(newManifestSlice, fileInfo)
		}
	}()

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
		}
		return nil
	})

	// Close the channel after all paths have been sent.
	// This signals to worker goroutines that no more data will be sent.
	close(filepathWalkDataChan)

	wg.Wait()

	close(fileInfoChan)

	if err != nil {
		fmt.Printf("Error traversing directory: %v\n", err)
		return
	}

	sort.Slice(newManifestSlice, func(i, j int) bool {
		return newManifestSlice[i].Path < newManifestSlice[j].Path
	})

	err = writeManifest(file, newManifestSlice)
	if err != nil {
		fmt.Printf("Error writing new manifest file: %v\n", err)
		return
	}
}

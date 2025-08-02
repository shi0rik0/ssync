package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

type filepathWalkData struct {
	Path         string
	RelativePath string
	Info         os.FileInfo
}

type FileInfo struct {
	Path         string
	ModifiedTime time.Time
	Size         int64
	Hash         string
}

func runCreate(cmd *cobra.Command, args []string) {
	// Extract arguments.
	directoryPath := args[0]
	manifestPath := args[1]

	fmt.Printf("Executing 'create' command with directory: '%s', manifest: '%s'\n", directoryPath, manifestPath)

	// Check if manifestPath already exists
	if _, err := os.Stat(manifestPath); err == nil {
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

	file, err := createFile(manifestPath)
	if err != nil {
		fmt.Printf("Error creating manifest file: %v\n", err)
		return
	}
	defer file.Close()

	// Create a channel to receive walked file paths.
	filepathWalkDataChan := make(chan filepathWalkData, runtime.NumCPU())
	fileInfoChan := make(chan FileInfo)

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
				hash, err := calculateMD5(path)
				if err != nil {
					panic(fmt.Sprintf("Error calculating MD5 for file %s: %v", path, err))
				}
				fileInfo := FileInfo{
					Path:         relativePath, // Use relative path for the manifest.
					ModifiedTime: modifiedTime,
					Size:         size,
					Hash:         hash,
				}
				fileInfoChan <- fileInfo
			}
		}()
	}

	fileInfoSlice := make([]FileInfo, 0)

	var wg2 sync.WaitGroup

	wg2.Add(1) // Increment counter for the goroutine collecting file info.
	// This goroutine collects file info.
	go func() {
		defer wg2.Done()
		for fileInfo := range fileInfoChan {
			fileInfoSlice = append(fileInfoSlice, fileInfo)
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

	// Wait for all worker goroutines to finish processing.
	wg.Wait()

	close(fileInfoChan) // Close the fileInfoChan to signal no more data will be sent.

	wg2.Wait() // Wait for the goroutine collecting file info to finish.

	if err != nil {
		fmt.Printf("Error traversing directory: %v\n", err)
		return
	}

	fmt.Println("All files processed successfully.")

	// Sort fileInfoSlice by Path field
	sort.Slice(fileInfoSlice, func(i, j int) bool {
		return fileInfoSlice[i].Path < fileInfoSlice[j].Path
	})

	err = writeManifest(file, fileInfoSlice)
	if err != nil {
		fmt.Printf("Error writing manifest file: %v\n", err)
		return
	}

	fmt.Printf("Manifest written to %s\n", manifestPath)
}

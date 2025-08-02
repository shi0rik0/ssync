package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type filepathWalkData struct {
	Path         string
	RelativePath string // normalized to use "/" as the path separator
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

	logrus.Debugf("Executing 'create' command with directory: '%s', manifest: '%s'", directoryPath, manifestPath)

	file, err := createFile(manifestPath)
	if err != nil {
		fmt.Printf("Error creating manifest file: %v\n", err)
		return
	}
	defer file.Close()

	// Channels for communication between goroutines.
	filepathWalkDataChan := make(chan filepathWalkData, runtime.NumCPU())
	fileInfoChan := make(chan FileInfo)

	// WaitGroup to synchronize goroutines.
	var wg sync.WaitGroup
	var wg2 sync.WaitGroup

	// [Goroutine] Worker goroutines to process file paths.
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

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

	wg2.Add(1)
	// [Goroutine] This goroutine collects file info.
	go func() {
		defer wg2.Done()

		for fileInfo := range fileInfoChan {
			fileInfoSlice = append(fileInfoSlice, fileInfo)
		}
	}()

	// Traverse the directory and send file paths to the channel.
	err = filepath.Walk(directoryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing path %q: %v\n", path, err)
			return nil
		}

		if info.IsDir() {
			// Skip directories.
			return nil
		}

		relativePath, err := filepath.Rel(directoryPath, path) // Make the path relative to the directory.
		relativePath = filepath.ToSlash(relativePath)          // Convert to forward slashes for consistency.
		if err != nil {
			fmt.Printf("Error processing file %q: %v\n", path, err)
			return nil
		}

		fmt.Printf("Processing file: %s\n", path)
		filepathWalkDataChan <- filepathWalkData{Path: path, RelativePath: relativePath, Info: info}
		return nil
	})

	// Notify the goroutines to stop and wait for them to finish.
	close(filepathWalkDataChan)
	wg.Wait()
	close(fileInfoChan)
	wg2.Wait()

	if err != nil {
		fmt.Printf("Error traversing directory: %v\n", err)
	} else {
		fmt.Println("All files processed successfully.")
	}

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

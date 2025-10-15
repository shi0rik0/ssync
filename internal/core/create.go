package core

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func Create(cmd *cobra.Command, args []string) {
	// Extract arguments.
	directoryPath := args[0]
	manifestPath := args[1]

	logrus.Debugf("Executing 'create' command with directory: '%s', manifest: '%s'", directoryPath, manifestPath)

	isNTFS, err := isNTFS(directoryPath)
	if err != nil {
		fmt.Printf("Error checking file system type: %v\n", err)
		return
	}
	if !isNTFS {
		fmt.Printf("The directory is not on an NTFS file system. NTFS file IDs will not be available.\n")
		return
	}

	// Create the manifest file.
	file, err := createFile(manifestPath)
	if err != nil {
		fmt.Printf("Error creating manifest file: %v\n", err)
		return
	}
	defer file.Close()

	totalFileSize := int64(0)
	// Traverse the directory to get the total file size.
	_ = filepath.WalkDir(directoryPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		totalFileSize += info.Size()
		return nil
	})

	processedFileSizeChan := make(chan int64)
	var wg sync.WaitGroup
	wg.Add(1)
	// Start a goroutine to print progress.
	go func() {
		defer wg.Done()
		totalSize := int64(0)
		for size := range processedFileSizeChan {
			totalSize += size
			fmt.Printf("\r%80s", "") // Clear the line
			fmt.Printf("\rProcessed file size: %s, Total: %s", toFriendlySize(totalSize), toFriendlySize(totalFileSize))
		}
		fmt.Println()
	}()

	fileInfoSlice := make([]FileInfo, 0)

	err = filepath.WalkDir(directoryPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Log error but continue walking.
			fmt.Printf("Warning: Error processing file %q: %v\n", path, err)
			return nil
		}

		if d.IsDir() {
			// Skip directories.
			return nil
		}

		// Get os.FileInfo from fs.DirEntry to access ModTime and Size.
		info, err := d.Info()
		if err != nil {
			// Log error but continue walking if info can't be retrieved for a file.
			fmt.Printf("Warning: Error getting file info for %q: %v\n", path, err)
			return nil
		}

		// Make the path relative to the directory and normalize slashes.
		relativePath, err := filepath.Rel(directoryPath, path)
		if err != nil {
			// Log error but continue walking.
			fmt.Printf("Error processing file %q: %v\n", path, err)
			return nil
		}
		relativePath = filepath.ToSlash(relativePath)

		// Process the file details.
		modifiedTime := info.ModTime()
		size := info.Size()
		hash, err := calculateMD5(path)
		if err != nil {
			// Panic on MD5 calculation error as it's critical.
			panic(fmt.Sprintf("Error calculating MD5 for file %s: %v", path, err))
		}
		fileID, err := getNTFSFileID(path)
		if err != nil {
			panic(fmt.Sprintf("Error getting NTFS file ID for file %s: %v", path, err))
		}

		// Create FileInfo and append to slice.
		fileInfo := FileInfo{
			Path:         relativePath,
			ModifiedTime: modifiedTime,
			Size:         size,
			Hash:         hash,
			NTFSFileID:   fileID,
		}
		fileInfoSlice = append(fileInfoSlice, fileInfo)
		processedFileSizeChan <- size
		return nil
	})

	close(processedFileSizeChan)
	wg.Wait()

	if err != nil {
		fmt.Printf("Error traversing directory: %v\n", err)
	}

	// Sort fileInfoSlice by Path for consistent manifest generation.
	sort.Slice(fileInfoSlice, func(i, j int) bool {
		return fileInfoSlice[i].Path < fileInfoSlice[j].Path
	})

	// Write the collected file information to the manifest file.
	err = writeManifest(file, fileInfoSlice)
	if err != nil {
		fmt.Printf("Error writing manifest file: %v\n", err)
		return
	}

	fmt.Printf("Manifest written to %s\n", manifestPath)
}

package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// runUpdate updates an existing manifest file by traversing a directory.
func runUpdate(cmd *cobra.Command, args []string) {
	// Extract arguments.
	directoryPath := args[0]
	oldManifestPath := args[1]
	newManifestPath := args[2]
	logrus.Debugf("Executing 'update' command with directory: '%s', old manifest: '%s', new manifest: '%s'", directoryPath, oldManifestPath, newManifestPath)

	isNTFS, err := isNTFS(directoryPath)
	if err != nil {
		fmt.Printf("Error checking file system type: %v\n", err)
		return
	}
	if !isNTFS {
		fmt.Printf("The directory is not on an NTFS file system. NTFS file IDs will not be available.\n")
		return
	}

	// Create the new manifest file.
	file, err := createFile(newManifestPath)
	if err != nil {
		fmt.Printf("Error creating new manifest file: %v\n", err)
		return
	}
	defer file.Close()

	oldManifestSlice, err := readManifest(oldManifestPath)
	if err != nil {
		fmt.Printf("Error reading old manifest: %v\n", err)
		return
	}

	oldManifestMapByPath := make(map[string]FileInfo)
	oldManifestMapByFileID := make(map[uint64]FileInfo)
	for _, fileInfo := range oldManifestSlice {
		oldManifestMapByPath[fileInfo.Path] = fileInfo
		oldManifestMapByFileID[fileInfo.NTFSFileID] = fileInfo
	}

	newManifestSlice := make([]FileInfo, 0)

	err = filepath.WalkDir(directoryPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Log error but continue walking the directory.
			fmt.Printf("Error accessing path %q: %v\n", path, err)
			return nil
		}

		if d.IsDir() {
			// Skip directories.
			return nil
		}

		// Get os.FileInfo from fs.DirEntry to access ModTime and Size.
		info, err := d.Info()
		if err != nil {
			// Log error but continue walking if file info cannot be retrieved.
			fmt.Printf("Error getting file info for %q: %v\n", path, err)
			return nil
		}

		// Make the path relative to the base directory and normalize slashes.
		relativePath, err := filepath.Rel(directoryPath, path)
		if err != nil {
			// Log error but continue walking.
			fmt.Printf("Error processing relative path for %q: %v\n", path, err)
			return nil
		}
		relativePath = filepath.ToSlash(relativePath)
		logrus.Debugf("Processing file: %s", path)

		// Get current file's modification time and size.
		modifiedTime := info.ModTime()
		size := info.Size()
		var currentHash string

		fileID, err := getNTFSFileID(path)
		if err != nil {
			panic(fmt.Sprintf("Error getting NTFS file ID for file %s: %v", path, err))
		}

		var fileInfo FileInfo
		oldFileInfo1, exists := oldManifestMapByPath[relativePath]
		// condition1: The file is unchanged and unmoved.
		condition1 := exists && modifiedTime.Unix() == oldFileInfo1.ModifiedTime.Unix() && size == oldFileInfo1.Size
		oldFileInfo2, exists := oldManifestMapByFileID[fileID]
		// condition2: The file is unchanged but moved.
		condition2 := exists && modifiedTime.Unix() == oldFileInfo2.ModifiedTime.Unix() && size == oldFileInfo2.Size
		if condition1 {
			fileInfo = oldFileInfo1
		} else if condition2 {
			fileInfo = oldFileInfo2
			fileInfo.Path = relativePath // Update path to the new relative path.
		} else {
			// File is new, calculate its MD5 hash.
			hash, err := calculateMD5(path)
			if err != nil {
				panic(fmt.Sprintf("Error calculating MD5 for new file %s: %v", path, err))
			}
			currentHash = hash
			fileInfo = FileInfo{
				Path:         relativePath,
				ModifiedTime: modifiedTime,
				Size:         size,
				Hash:         currentHash,
				NTFSFileID:   fileID,
			}
		}

		newManifestSlice = append(newManifestSlice, fileInfo)

		return nil
	})

	if err != nil {
		fmt.Printf("Error traversing directory: %v\n", err)
		return // Exit if directory traversal failed.
	}

	fmt.Println("All files processed successfully.")

	// Sort the new manifest slice by Path for consistent manifest generation.
	sort.Slice(newManifestSlice, func(i, j int) bool {
		return newManifestSlice[i].Path < newManifestSlice[j].Path
	})

	// Write the collected file information to the new manifest file.
	err = writeManifest(file, newManifestSlice) // Assumes writeManifest is defined elsewhere.
	if err != nil {
		fmt.Printf("Error writing new manifest file: %v\n", err)
		return
	}

	fmt.Printf("New manifest written to %s\n", newManifestPath)
}

package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func walkDir(dir string, strict bool) ([]FileInfo, error) {
	var fileInfoSlice []FileInfo
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil // Skip directories
		}

		relativePath, err := filepath.Rel(dir, path) // Make the path relative to the directory.

		if err != nil {
			return fmt.Errorf("error getting relative path for %s: %w", path, err)
		}

		relativePath = filepath.ToSlash(relativePath) // Convert to forward slashes for consistency.

		fi, err := d.Info()
		if err != nil {
			return err
		}
		fileInfo := FileInfo{
			Path:         relativePath,
			ModifiedTime: fi.ModTime(),
			Size:         fi.Size(),
		}
		if strict {
			fileInfo.Hash, err = calculateMD5(path)
			if err != nil {
				return fmt.Errorf("error calculating MD5 for %s: %w", path, err)
			}
		}
		fileInfoSlice = append(fileInfoSlice, fileInfo)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return fileInfoSlice, nil
}

func fileInfoSliceToMap(fileInfoSlice []FileInfo) map[string]FileInfo {
	fileInfoMap := make(map[string]FileInfo)
	for _, fi := range fileInfoSlice {
		fileInfoMap[fi.Path] = fi
	}
	return fileInfoMap
}

func runCompare(cmd *cobra.Command, args []string) {
	dir1 := args[0]
	dir2 := args[1]
	strictFlag, err := cmd.Flags().GetBool("strict")
	if err != nil {
		fmt.Printf("Error retrieving strict flag: %v\n", err)
		return
	}
	logrus.Debugf("Executing 'compare' command with arguments: dir1='%s', dir2='%s', strict=%t", dir1, dir2, strictFlag)

	if !strictFlag {
		fmt.Printf("Warning: Strict comparison is disabled.\n")
	}

	var fileInfoSlice1, fileInfoSlice2 []FileInfo
	var err1, err2 error
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		fileInfoSlice1, err1 = walkDir(dir1, strictFlag)
	}()

	go func() {
		defer wg.Done()
		fileInfoSlice2, err2 = walkDir(dir2, strictFlag)
	}()

	wg.Wait()

	if err1 != nil {
		fmt.Printf("Error walking directory %s: %v\n", dir1, err1)
		return
	}
	if err2 != nil {
		fmt.Printf("Error walking directory %s: %v\n", dir2, err2)
		return
	}

	fileInfoMap1 := fileInfoSliceToMap(fileInfoSlice1)
	fileInfoMap2 := fileInfoSliceToMap(fileInfoSlice2)

	type output struct {
		Path string
		Msg  string
	}
	outputs := make([]output, 0)

	for path, fi1 := range fileInfoMap1 {
		fi2, exists := fileInfoMap2[path]
		if !exists {
			outputs = append(outputs, output{Path: path, Msg: fmt.Sprintf("[<--] %s", path)})
			continue
		}

		mtimeEqual := fi1.ModifiedTime.Unix() == fi2.ModifiedTime.Unix()
		sizeEqual := fi1.Size == fi2.Size
		hashEqual := !strictFlag || fi1.Hash == fi2.Hash
		if !mtimeEqual || !sizeEqual || !hashEqual {
			reason := ""
			if !mtimeEqual {
				reason += "modified time differs, "
			}
			if !sizeEqual {
				reason += "size differs, "
				if !hashEqual {
					reason += "hash differs, "
				}
				reason = reason[:len(reason)-2] // Remove trailing comma and space
				outputs = append(outputs, output{Path: path, Msg: fmt.Sprintf("[=/=] %s: %s", path, reason)})
			}
		}
	}

	for path := range fileInfoMap2 {
		_, exists := fileInfoMap1[path]
		if !exists {
			outputs = append(outputs, output{Path: path, Msg: fmt.Sprintf("[-->] %s", path)})
		}
	}

	// Sort outputs by path for consistent output
	sort.Slice(outputs, func(i, j int) bool {
		return outputs[i].Path < outputs[j].Path
	})

	for _, out := range outputs {
		fmt.Println(out.Msg)
	}

	fmt.Printf("Comparison completed between %s and %s\n", dir1, dir2)
}

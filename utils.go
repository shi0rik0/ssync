package main

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// FileInfo struct holds details for a file entry in the manifest.
type FileInfo struct {
	Path         string    // relative path, normalized to use "/" as the path separator
	ModifiedTime time.Time // The precision may be higher than seconds, but only seconds will be used.
	Size         int64     // in bytes
	Hash         string    // MD5 hash
	NTFSFileID   uint64
}

// toCSVField converts a string into an RFC 4180 compliant CSV field.
// It quotes the string only when necessary (i.e., if it contains a comma,
// a double quote, or a newline character). Internal double quotes are escaped.
func toCSVField(s string) string {
	// Check if the string contains characters that require quoting based on RFC 4180:
	// comma (,), double quote ("), or newline (\n or \r).
	needsQuote := false
	if strings.ContainsRune(s, ',') ||
		strings.ContainsRune(s, '\n') ||
		strings.ContainsRune(s, '\r') ||
		strings.ContainsRune(s, '"') {
		needsQuote = true
	}

	if needsQuote {
		// If quoting is necessary, first escape any internal double quotes
		// by replacing each " with "".
		escapedS := strings.ReplaceAll(s, `"`, `""`)
		// Then, wrap the entire escaped string in double quotes.
		return `"` + escapedS + `"`
	}

	// If no special characters are found, the string does not need quoting.
	// Return the original string as is.
	return s
}

func readManifest(manifestPath string) ([]FileInfo, error) {
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("error opening manifest file: %v", err)
	}
	defer file.Close()

	var fileInfoSlice []FileInfo
	scanner := bufio.NewScanner(file)
	firstLine := true
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" || firstLine {
			firstLine = false
			continue
		}

		fields := strings.SplitN(line, ",", 5)
		if len(fields) != 5 {
			return nil, fmt.Errorf("invalid manifest line: %s", line)
		}

		unixTime, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing modified time: %v", err)
		}
		modifiedTime := time.Unix(unixTime, 0)

		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing size: %v", err)
		}

		fileID, err := strconv.ParseUint(fields[4], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing NTFS file ID: %v", err)
		}

		fileInfoSlice = append(fileInfoSlice, FileInfo{
			Path:         fields[0],
			ModifiedTime: modifiedTime,
			Size:         size,
			Hash:         fields[3],
			NTFSFileID:   fileID,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading manifest file: %v", err)
	}

	return fileInfoSlice, nil
}

func writeManifest(file *os.File, fileInfoSlice []FileInfo) error {
	writer := bufio.NewWriter(file)
	// Write the header line.
	_, err := writer.WriteString("Path,ModifiedTime,Size,Hash,NtfsFileId\n")
	if err != nil {
		return fmt.Errorf("error writing header to manifest file: %v", err)
	}

	for _, fileInfo := range fileInfoSlice {
		line := fmt.Sprintf("%s,%s,%s,%s,%s\n",
			toCSVField(fileInfo.Path),
			toCSVField(fmt.Sprintf("%d", fileInfo.ModifiedTime.Unix())),
			toCSVField(fmt.Sprintf("%d", fileInfo.Size)),
			toCSVField(fileInfo.Hash),
			toCSVField(fmt.Sprintf("%d", fileInfo.NTFSFileID)))
		_, err = writer.WriteString(line)
		if err != nil {
			return fmt.Errorf("error writing line to manifest file: %v", err)
		}
	}

	return writer.Flush()
}

func createFile(path string) (*os.File, error) {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return nil, fmt.Errorf("error creating parent directory: %v", err)
	}

	_, err := os.Stat(path)
	if err == nil {
		return nil, fmt.Errorf("file already exists: %s", path)
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("error creating file: %v", err)
	}
	return file, nil
}

// calculateMD5 calculates the MD5 checksum of a given file.
func calculateMD5(filePath string) (string, error) {
	// 1. Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close() // Ensure the file is closed when the function exits

	// 2. Create a new MD5 hasher
	hash := md5.New()

	// 3. Copy the file content to the hasher
	// io.Copy reads data from 'file' and writes it to 'hash'.
	// This is efficient for large files as it doesn't load the entire file into memory.
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to copy file content to hasher: %w", err)
	}

	// 4. Get the hash sum (as a byte slice)
	// The nil argument means we don't append to an existing slice.
	hashInBytes := hash.Sum(nil)

	// 5. Convert the byte slice to a hexadecimal string
	md5String := hex.EncodeToString(hashInBytes)

	return md5String, nil
}

func toFriendlySize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(size)/1024)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(size)/(1024*1024))
	} else {
		return fmt.Sprintf("%.2f GB", float64(size)/(1024*1024*1024))
	}
}

// getNTFSFileID retrieves the NTFS file ID for a given file.
// The file should be on an NTFS file system.
func getNTFSFileID(path string) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer file.Close() // Ensure the file is closed when the function exits

	fh := windows.Handle(file.Fd())

	var info windows.ByHandleFileInformation

	err = windows.GetFileInformationByHandle(fh, &info)
	if err != nil {
		return 0, err
	}

	// File ID = (FileIndexHigh << 32) | FileIndexLow
	fileID := (uint64(info.FileIndexHigh) << 32) | uint64(info.FileIndexLow)
	return fileID, nil
}

// isNTFS checks if the disk file system where the given directory path resides is NTFS.
// It determines the root drive of the path and queries its file system type.
func isNTFS(dirPath string) (bool, error) {
	// Get the absolute path to handle relative paths like "." or "..".
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return false, fmt.Errorf("failed to get absolute path for '%s': %w", dirPath, err)
	}

	// Extract the volume name (e.g., "C:", "D:") from the absolute path.
	volumeName := filepath.VolumeName(absPath)

	// GetVolumeInformationW expects the root path to end with a backslash (e.g., "C:\").
	rootPath := volumeName
	if rootPath != "" && !strings.HasSuffix(rootPath, `\`) {
		rootPath += `\`
	} else if rootPath == "" {
		return false, fmt.Errorf("could not determine local volume for path '%s'. This function does not support UNC paths or invalid paths", dirPath)
	}

	// Convert the root path string to a UTF-16 pointer for Windows API calls.
	rootPathPtr, err := syscall.UTF16PtrFromString(rootPath)
	if err != nil {
		return false, fmt.Errorf("failed to convert root path '%s' to UTF16: %w", rootPath, err)
	}

	// Prepare buffers for the API call results.
	// windows.MAX_PATH is 260, so MAX_PATH+1 is for the null terminator.
	var (
		volumeNameBuffer       [windows.MAX_PATH + 1]uint16
		fileSystemNameBuffer   [windows.MAX_PATH + 1]uint16
		volumeSerialNumber     uint32
		maximumComponentLength uint32
		fileSystemFlags        uint32
	)

	// Call the windows.GetVolumeInformation function.
	// It wraps the underlying Syscall and directly returns Go's error type.
	err = windows.GetVolumeInformation(
		rootPathPtr,
		&volumeNameBuffer[0],
		uint32(len(volumeNameBuffer)),
		&volumeSerialNumber,
		&maximumComponentLength,
		&fileSystemFlags,
		&fileSystemNameBuffer[0],
		uint32(len(fileSystemNameBuffer)),
	)

	// Check if the API call was successful.
	if err != nil {
		return false, fmt.Errorf("GetVolumeInformation call failed for path '%s': %w", rootPath, err)
	}

	// Convert the UTF-16 file system name buffer to a Go string.
	fileSystemName := syscall.UTF16ToString(fileSystemNameBuffer[:])

	// Compare the obtained file system name with "NTFS".
	return fileSystemName == "NTFS", nil
}

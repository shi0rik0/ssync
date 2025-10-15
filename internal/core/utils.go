package core

import (
	"crypto/md5"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sqweek/dialog"
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

// readManifest reads a manifest CSV file and returns a slice of FileInfo.
func readManifest(manifestPath string) ([]FileInfo, error) {
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("error opening manifest file: %v", err)
	}
	defer file.Close()

	r := csv.NewReader(file)
	r.FieldsPerRecord = 5

	var fileInfoSlice []FileInfo

	// Read and skip the header
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("error reading manifest header: %v", err)
	}
	if len(header) != 5 {
		return nil, fmt.Errorf("invalid manifest header: %v", header)
	}

	for {
		fields, err := r.Read()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("error reading manifest line: %v", err)
		}
		if len(fields) != 5 {
			return nil, fmt.Errorf("invalid manifest line (field count): %v", fields)
		}

		unixTime, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing ModifiedTime in line %v: %v", fields, err)
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing Size in line %v: %v", fields, err)
		}
		fileID, err := strconv.ParseUint(fields[4], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing NTFSFileID in line %v: %v", fields, err)
		}

		fileInfoSlice = append(fileInfoSlice, FileInfo{
			Path:         fields[0],
			ModifiedTime: time.Unix(unixTime, 0),
			Size:         size,
			Hash:         fields[3],
			NTFSFileID:   fileID,
		})
	}

	return fileInfoSlice, nil
}

// writeManifest writes a slice of FileInfo to a CSV manifest file.
func writeManifest(file io.Writer, fileInfoSlice []FileInfo) error {
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 验证是否有重复的 Path
	pathSet := make(map[string]bool)
	for _, fi := range fileInfoSlice {
		if pathSet[fi.Path] {
			return fmt.Errorf("duplicate path found in manifest: %s", fi.Path)
		}
		pathSet[fi.Path] = true
	}

	// 按照 Path 字段进行排序
	sort.Slice(fileInfoSlice, func(i, j int) bool {
		return fileInfoSlice[i].Path < fileInfoSlice[j].Path
	})

	// Write the header line.
	header := []string{"Path", "ModifiedTime", "Size", "Hash", "NtfsFileId"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("error writing header to manifest file: %v", err)
	}

	for _, fileInfo := range fileInfoSlice {
		line := []string{
			fileInfo.Path,
			strconv.FormatInt(fileInfo.ModifiedTime.Unix(), 10),
			strconv.FormatInt(fileInfo.Size, 10),
			fileInfo.Hash,
			strconv.FormatUint(fileInfo.NTFSFileID, 10),
		}
		if err := writer.Write(line); err != nil {
			return fmt.Errorf("error writing line to manifest file: %v", err)
		}
	}

	return nil
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

// 打开 Windows 的目录选择对话框, 返回用户选择的目录路径.
func openSelectFolderDialog(title string) (string, error) {
	folderPath, err := dialog.Directory().Title(title).Browse()
	if err != nil {
		return "", fmt.Errorf("failed to open folder selection dialog: %w", err)
	}
	return folderPath, nil
}

func openSelectFileDialog(title string, desc string, extensions ...string) (string, error) {
	filePath, err := dialog.File().Title(title).Filter(desc, extensions...).Load()
	if err != nil {
		return "", fmt.Errorf("failed to open file selection dialog: %w", err)
	}
	return filePath, nil
}

func openSaveFileDialog(title string, defaultName string, desc string, extensions ...string) (string, error) {
	filePath, err := dialog.File().Title(title).Filter(desc, extensions...).SetStartFile(defaultName).Save()
	if err != nil {
		return "", fmt.Errorf("failed to open save file dialog: %w", err)
	}
	return filePath, nil
}

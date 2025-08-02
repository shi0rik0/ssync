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
	"time"
)

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

		fields := strings.SplitN(line, ",", 4)
		if len(fields) != 4 {
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

		fileInfoSlice = append(fileInfoSlice, FileInfo{
			Path:         fields[0],
			ModifiedTime: modifiedTime,
			Size:         size,
			Hash:         fields[3],
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
	_, err := writer.WriteString("Path,ModifiedTime,Size,Hash\n")
	if err != nil {
		return fmt.Errorf("error writing header to manifest file: %v", err)
	}

	for _, fileInfo := range fileInfoSlice {
		line := fmt.Sprintf("%s,%s,%s,%s\n",
			toCSVField(fileInfo.Path),
			toCSVField(fmt.Sprintf("%d", fileInfo.ModifiedTime.Unix())),
			toCSVField(fmt.Sprintf("%d", fileInfo.Size)),
			toCSVField(fileInfo.Hash))
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

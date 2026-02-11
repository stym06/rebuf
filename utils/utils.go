// Package utils provides filesystem helpers for rebuf's segment management.
package utils

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// IsDirectoryEmpty reports whether dirPath contains no files other than .tmp files.
func IsDirectoryEmpty(dirPath string) (bool, error) {
	dir, err := os.Open(dirPath)
	if err != nil {
		return false, err
	}
	defer dir.Close()

	entries, err := dir.ReadDir(-1)
	if err != nil && err != io.EOF {
		return false, err
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".tmp") {
			return false, nil
		}
	}
	return true, nil
}

// ParseSegmentId extracts the numeric segment ID from a filename like "rebuf-3".
func ParseSegmentId(fileName string) (int, error) {
	if !strings.HasPrefix(fileName, "rebuf-") {
		return 0, fmt.Errorf("utils: not a segment file: %s", fileName)
	}
	parts := strings.SplitN(fileName, "-", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("utils: invalid segment filename: %s", fileName)
	}
	return strconv.Atoi(parts[1])
}

// GetSortedSegmentFiles returns all segment files sorted by segment ID,
// with the active "rebuf.tmp" file (if present) appended at the end.
func GetSortedSegmentFiles(logDir string) ([]string, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}

	var segments []string
	hasTmp := false

	for _, entry := range entries {
		name := entry.Name()
		if name == "rebuf.tmp" {
			hasTmp = true
			continue
		}
		if strings.HasPrefix(name, "rebuf-") {
			segments = append(segments, name)
		}
	}

	sort.Slice(segments, func(i, j int) bool {
		idI, _ := ParseSegmentId(segments[i])
		idJ, _ := ParseSegmentId(segments[j])
		return idI < idJ
	})

	if hasTmp {
		segments = append(segments, "rebuf.tmp")
	}

	return segments, nil
}

// GetLatestSegmentId returns the highest segment ID found in logDir.
// Returns an error if no numbered segment files exist.
func GetLatestSegmentId(logDir string) (int, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return 0, err
	}

	maxId := -1
	for _, entry := range entries {
		id, err := ParseSegmentId(entry.Name())
		if err != nil {
			continue
		}
		if id > maxId {
			maxId = id
		}
	}

	if maxId == -1 {
		return 0, fmt.Errorf("utils: no segment files found in %s", logDir)
	}
	return maxId, nil
}

// GetNumSegments returns the number of numbered segment files (excluding .tmp).
func GetNumSegments(logDir string) (int, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "rebuf-") {
			count++
		}
	}
	return count, nil
}

// FileSize returns the size of the given open file in bytes.
func FileSize(f *os.File) (int64, error) {
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// GetOldestSegmentFile returns the segment file with the lowest ID.
// Returns an error if no numbered segment files exist.
func GetOldestSegmentFile(logDir string) (string, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return "", err
	}

	minId := int(^uint(0) >> 1) // max int
	var oldestFile string

	for _, entry := range entries {
		id, err := ParseSegmentId(entry.Name())
		if err != nil {
			continue
		}
		if id < minId {
			minId = id
			oldestFile = entry.Name()
		}
	}

	if oldestFile == "" {
		return "", fmt.Errorf("utils: no segment files found in %s", logDir)
	}
	return oldestFile, nil
}

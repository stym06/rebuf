package utils

import (
	"io"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"
)

func IsDirectoryEmpty(dirPath string) (bool, error) {
	// Open the directory
	dir, err := os.Open(dirPath)
	if err != nil {
		return false, err
	}
	defer dir.Close()

	// Read the directory contents
	files, err := dir.ReadDir(1) // Read the first entry
	var filteredFiles []os.DirEntry
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".tmp") {
			filteredFiles = append(filteredFiles, file)
		}
	}
	if err != nil && err != io.EOF {
		return false, err
	}

	// If the list of files is empty, the directory is empty
	return len(filteredFiles) == 0, nil
}

func GetLatestSegmentId(logDir string) (int, error) {
	files, err := os.ReadDir(logDir)
	if err != nil {
		return 0, err
	}
	// Filter out .tmp files
	var filteredFiles []os.DirEntry
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".tmp") {
			filteredFiles = append(filteredFiles, file)
		}
	}
	sort.Slice(filteredFiles, func(i, j int) bool {
		return filteredFiles[i].Name() > filteredFiles[j].Name()
	})
	segmentCount, err := strconv.Atoi(strings.Split(filteredFiles[0].Name(), "-")[1])
	if err != nil {
		return 0, err
	}
	return segmentCount, nil
}

func GetNumSegments(logDir string) (int, error) {
	files, err := os.ReadDir(logDir)
	if err != nil {
		return 0, err
	}
	return len(files) - 1, nil
}

func FileSize(f *os.File) (int64, error) {
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func GetOldestSegmentFile(logDir string) (string, error) {
	files, err := os.ReadDir(logDir)
	if err != nil {
		return "0", err
	}
	// Filter out .tmp files
	var filteredFiles []os.DirEntry
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".tmp") {
			filteredFiles = append(filteredFiles, file)
		}
	}
	sort.Slice(filteredFiles, func(i, j int) bool {
		return filteredFiles[i].Name() < filteredFiles[j].Name()
	})
	return filteredFiles[0].Name(), nil
}

func GetAllSegmentsList(logDir string) ([]fs.DirEntry, error) {
	files, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}
	// Filter out .tmp files
	var filteredFiles []os.DirEntry
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".tmp") {
			filteredFiles = append(filteredFiles, file)
		}
	}
	sort.Slice(filteredFiles, func(i, j int) bool {
		return filteredFiles[i].Name() < filteredFiles[j].Name()
	})
	return filteredFiles, nil
}

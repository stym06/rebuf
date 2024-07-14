package utils

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
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
		fmt.Printf("File name: %s", file.Name())
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
	latestModifiedTime := time.Time{}
	var latestFileName string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".tmp") {
			continue
		}
		fileInfo, err := file.Info()

		if err != nil {
			return 0, err
		}

		if fileInfo.ModTime().After(latestModifiedTime) {
			latestModifiedTime = fileInfo.ModTime()
			latestFileName = file.Name()
		}
	}
	fmt.Println(latestFileName)
	segmentCount, err := strconv.Atoi(strings.Split(latestFileName, "-")[1])
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
	oldestModifedTime := time.Now()
	var oldestFileName string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".tmp") {
			continue
		}
		fileInfo, err := file.Info()

		if err != nil {
			return "", err
		}

		if fileInfo.ModTime().Before(oldestModifedTime) {
			oldestModifedTime = fileInfo.ModTime()
			oldestFileName = file.Name()
		}
	}
	return oldestFileName, nil
}

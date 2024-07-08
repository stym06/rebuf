package utils

import (
	"io"
	"os"
)

func IsDirectoryEmpty(dirPath string) (bool, error) {
	// Open the directory
	dir, err := os.Open(dirPath)
	if err != nil {
		return false, err
	}
	defer dir.Close()

	// Read the directory contents
	fileList, err := dir.ReadDir(1) // Read the first entry
	if err != nil && err != io.EOF {
		return false, err
	}

	// If the list of files is empty, the directory is empty
	return len(fileList) == 0, nil
}

func FileSize(f *os.File) (int64, error) {
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

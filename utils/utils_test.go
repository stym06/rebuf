package utils

import (
	"log"
	"os"
	"path/filepath"
	"testing"
)

func setupSuite(t testing.TB) func(t testing.TB) {
	log.Println("Setting up logDir empty")

	dirPath := os.Getenv("TEST_LOG_DIR")

	if _, err := os.Stat(filepath.Join(dirPath)); err != nil {
		if os.IsNotExist(err) {
			os.Mkdir(dirPath, 0700)
		}
	} else {
		t.Fatal("Error creating dirPath in setup suite")
	}

	// Return a function to teardown the test
	return func(tb testing.TB) {
		log.Printf("Deleting everything in %v", dirPath)
		os.RemoveAll(dirPath)

	}
}

func createFile(fileName string) (*os.File, error) {
	file, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func TestIsDirectoryEmpty(t *testing.T) {

	teardownSuite := setupSuite(t)
	defer teardownSuite(t)
	dirPath := os.Getenv("TEST_LOG_DIR")

	t.Run("directory exists without .tmp file", func(t *testing.T) {
		empty, err := IsDirectoryEmpty(dirPath)
		if err != nil {
			t.Fatalf("Error in running IsDirectoryEmpty with %v", dirPath)
		}

		//empty should be true
		if empty == false {
			t.Fatalf("Expected %v. Got %v", false, empty)
		}
	})

	t.Run("directory exists with .tmp file", func(t *testing.T) {

		file, err := createFile(filepath.Join(dirPath, "rebuf.tmp"))
		if err != nil {
			t.Fatalf("Error in creating file %v", file)
		}

		empty, err := IsDirectoryEmpty(dirPath)
		if err != nil {
			t.Fatalf("Error in running IsDirectoryEmpty with %v", dirPath)
		}

		//empty should be true
		if empty == false {
			t.Fatalf("Expected %v. Got %v", false, empty)
		}
	})

	// t.Run("directory exists with .tmp file and data file", func(t *testing.T) {

	// 	dataFileName := filepath.Join(dirPath, "rebuf-1")
	// 	dataFile, err := os.OpenFile(dataFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	// 	if err != nil {
	// 		t.Fatalf("Error in creating file %v", dataFile)
	// 	}
	// 	defer dataFile.Sync()
	// 	defer dataFile.Close()

	// 	data := []byte{0x1}
	// 	dataFileWriter := bufio.NewWriter(dataFile)
	// 	_, err = dataFileWriter.Write(data)
	// 	if err != nil {
	// 		t.Fatalf("Error in writing file %v", dataFile)
	// 	}
	// 	dataFileWriter.Flush()

	// 	empty, err := IsDirectoryEmpty(dirPath)
	// 	if err != nil {
	// 		t.Fatalf("Error in running IsDirectoryEmpty with %v", empty)
	// 	}

	// 	//empty should be false
	// 	if empty == true {
	// 		t.Fatalf("Expected %v. Got %v", false, empty)
	// 	}
	// })
}

func TestGetLatestSegmentId(t *testing.T) {
	//test2
}

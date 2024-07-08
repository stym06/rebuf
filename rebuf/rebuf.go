package rebuf

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/stym06/rebuf/utils"
)

type RebufOptions struct {
	LogDir      string
	MaxLogSize  int64
	MaxSegments int
}

type Rebuf struct {
	logDir           string
	currentSegmentId int
	maxLogSize       int64
	maxSegments      int
	segmentCount     int
	bufWriter        *bufio.Writer
	logSize          int64
	file             *os.File
	tmpLogFile       *os.File
}

func Init(options *RebufOptions) (*Rebuf, error) {

	if strings.HasSuffix(options.LogDir, "/") {
		panic("Log Directory should not have trailing slash")
	}

	//ensure dir is created
	if _, err := os.Stat(options.LogDir); err != nil {
		if os.IsNotExist(err) {
			os.Mkdir(options.LogDir, 0700)
		}
	}

	//open temp file
	tmpLogFileName := options.LogDir + "/" + "rebuf.tmp"
	tmpLogFile, err := os.OpenFile(tmpLogFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	rebuf := &Rebuf{
		logDir:      options.LogDir,
		maxLogSize:  options.MaxLogSize,
		maxSegments: options.MaxSegments,
		tmpLogFile:  tmpLogFile,
	}

	err = rebuf.openExistingOrCreateNew(options.LogDir)

	if err != nil {
		return nil, err
	}

	return rebuf, err
}

func (rebuf *Rebuf) Write(data []byte) error {
	if rebuf.logSize+int64(len(data))+8 > rebuf.maxLogSize {

		fmt.Println("Log size will be greater than " + string(rebuf.logSize))
		rebuf.bufWriter.Flush()

		//rename this file to current segment count
		os.Rename(rebuf.logDir+"/rebuf.tmp", rebuf.logDir+"/rebuf-"+strconv.Itoa(rebuf.currentSegmentId+1))

		//increase segment count by 1
		rebuf.currentSegmentId += 1
		rebuf.segmentCount += 1

		//change writer to this temp file
		rebuf.bufWriter = bufio.NewWriter(rebuf.tmpLogFile)
		rebuf.logSize = 0

		if rebuf.segmentCount > rebuf.maxSegments {
			//delete the oldest log file
			oldestLogFileName, err := utils.GetOldestSegmentFile(rebuf.logDir)
			if err != nil {
				return err
			}
			os.Remove(rebuf.logDir + "/" + oldestLogFileName)
			return nil
		}

		return nil
	}

	//seek to the end of the file
	_, err := rebuf.tmpLogFile.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	//write the size of the byte array and then the byte array itself
	size := uint64(len(data))
	//creating a byte array of size 8 and putting the length of `data` into the array
	sizeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(sizeBuf, size)

	_, err = rebuf.bufWriter.Write(sizeBuf)
	if err != nil {
		return err
	}
	_, err = rebuf.bufWriter.Write(data)
	if err != nil {
		return err
	}
	rebuf.logSize = rebuf.logSize + int64(len(data)) + 8
	rebuf.tmpLogFile.Close()

	return nil
}

func (rebuf *Rebuf) openExistingOrCreateNew(logDir string) error {
	//check if directory is empty (only containing .tmp file)
	empty, err := utils.IsDirectoryEmpty(logDir)
	if err != nil {
		return err
	}

	logFileName := logDir + "/" + "rebuf.tmp"
	file, err := os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	rebuf.file = file
	rebuf.bufWriter = bufio.NewWriter(file)

	if empty {
		rebuf.currentSegmentId = 0
		rebuf.segmentCount = 0
		rebuf.logSize, err = utils.FileSize(file)
		if err != nil {
			return err
		}
	} else {
		rebuf.currentSegmentId, err = utils.GetLatestSegmentId(logDir)
		if err != nil {
			return err
		}

		rebuf.segmentCount, err = utils.GetNumSegments(logDir)
		if err != nil {
			return err
		}
		rebuf.logSize, _ = utils.FileSize(rebuf.tmpLogFile)
	}

	return nil
}

func (rebuf *Rebuf) Replay(callbackFn func([]byte) error) error {
	files, err := utils.GetAllSegmentsList(rebuf.logDir)
	if err != nil {
		return err
	}
	for _, fileInfo := range files {
		file, err := os.Open(rebuf.logDir + "/" + fileInfo.Name())
		if err != nil {
			return err
		}
		defer file.Close()

		var readBytes []byte
		for err == nil {
			bufReader := bufio.NewReader(file)
			readBytes, err = bufReader.Peek(8)
			if err != nil {
				break
			}
			size := int(binary.BigEndian.Uint64(readBytes))
			_, err := bufReader.Discard(8)
			if err != nil {
				break
			}

			data, err := bufReader.Peek(size)
			if err != nil {
				break
			}
			err = callbackFn(data)
			if err != nil {
				break
			}
			_, _ = bufReader.Discard(size)
		}

	}
	return nil
}

func (rebuf *Rebuf) Close() error {
	if rebuf.bufWriter != nil {
		err := rebuf.bufWriter.Flush()
		return err
	}
	return nil
}

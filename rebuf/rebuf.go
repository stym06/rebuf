package rebuf

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/stym06/rebuf/utils"
)

type RebufOptions struct {
	LogDir      string
	MaxLogSize  int64
	MaxSegments int
	SyncMaxWait time.Duration
}

type Rebuf struct {
	logDir           string
	logFileName      string
	currentSegmentId int
	maxLogSize       int64
	maxSegments      int
	segmentCount     int
	syncMaxWait      time.Duration
	bufWriter        *bufio.Writer
	bufSize          int64
}

func Init(options *RebufOptions) (*Rebuf, error) {

	if strings.HasSuffix(options.LogDir, "/") {
		panic("Log Directory should not have trailing slash")
	}

	rebuf := &Rebuf{
		logDir:      options.LogDir,
		maxLogSize:  options.MaxLogSize,
		maxSegments: options.MaxSegments,
		syncMaxWait: options.SyncMaxWait,
	}

	err := rebuf.openExistingOrCreateNew(options.LogDir)

	return rebuf, err
}

func (rebuf *Rebuf) Write(data []byte) error {
	if rebuf.bufSize+int64(len(data)) > rebuf.maxLogSize {
		fmt.Println("Log size will be greater than " + string(rebuf.bufSize))
		logFileName := rebuf.logDir + "/" + "rebuf-" + strconv.Itoa(rebuf.segmentCount+1)
		file, err := os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		rebuf.bufWriter.Flush()
		rebuf.bufWriter = bufio.NewWriter(file)
		_, err = rebuf.bufWriter.Write(data)
		return err
	}
	_, err := rebuf.bufWriter.Write(data)
	return err
}

func (rebuf *Rebuf) openExistingOrCreateNew(logDir string) error {

	//ensure dir is created
	if _, err := os.Stat(logDir); err != nil {
		if os.IsNotExist(err) {
			os.Mkdir(logDir, 0700)
		}
	}
	//check if directory is empty
	empty, err := utils.IsDirectoryEmpty(logDir)
	if err != nil {
		return err
	}
	if empty {
		rebuf.currentSegmentId = 0
		firstLogFileName := logDir + "/" + "rebuf-0"
		file, err := os.OpenFile(firstLogFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		rebuf.bufWriter = bufio.NewWriter(file)
		rebuf.bufSize = 0
		rebuf.segmentCount = 0
	} else {
		files, err := os.ReadDir(logDir)
		if err != nil {
			return err
		}
		sort.Slice(files, func(i, j int) bool {
			return files[i].Name() > files[j].Name()
		})
		firstLogFileName := logDir + "/" + files[0].Name()
		file, err := os.OpenFile(firstLogFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		rebuf.bufWriter = bufio.NewWriter(file)
		rebuf.bufSize, err = utils.FileSize(file)
		if err != nil {
			return err
		}
		rebuf.segmentCount, err = strconv.Atoi(strings.Split(files[0].Name(), "-")[1])
		rebuf.logFileName = files[0].Name()
		if err != nil {
			return err
		}
	}
	return nil
}

func (rebuf *Rebuf) Replay() {

}

func (rebuf *Rebuf) Close() error {
	err := rebuf.bufWriter.Flush()
	if err != nil {
		panic(err)
	}
	return nil
}

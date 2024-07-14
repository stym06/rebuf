package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/stym06/rebuf/rebuf"
)

func writeToStdout(data []byte) error {
	slog.Info(string(data))
	return nil
}

func main() {

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	//Init the RebufOptions
	rebufOptions := &rebuf.RebufOptions{
		LogDir:      "/Users/satyamraj/personal/rebuf/data",
		FsyncTime:   5 * time.Second,
		MaxLogSize:  50,
		MaxSegments: 5,
		Logger:      logger,
	}

	//Init Rebuf
	rebuf, err := rebuf.Init(rebufOptions)
	if err != nil {
		logger.Info("Error during Rebuf creation: " + err.Error())
	}

	defer rebuf.Close()

	// Write Bytes
	for i := 0; i < 30; i++ {
		logger.Info("Writing data: ", "iter", i)
		go rebuf.Write([]byte("Hello world"))
		time.Sleep(300 * time.Millisecond)
	}

	//Replay and write to stdout
	rebuf.Replay(writeToStdout)

	//Get oldest and latest offset
	oldestOffset, err := rebuf.GetOldestOffset()
	if err != nil {
		logger.Info("Error during Rebuf creation: " + err.Error())
	}
	logger.Info("oldest offset is: " + fmt.Sprint(oldestOffset))
	latestOffset, err := rebuf.GetLatestOffset()
	if err != nil {
		logger.Info("Error during Rebuf creation: " + err.Error())
	}
	logger.Info("latest offset is: " + fmt.Sprintln(latestOffset))

	if err != nil {
		logger.Info(err.Error())
	}

	time.Sleep(30 * time.Second)

}

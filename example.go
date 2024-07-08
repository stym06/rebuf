package main

import (
	"fmt"
	"time"

	"github.com/stym06/rebuf/rebuf"
)

func writeToStdout(data []byte) error {
	fmt.Println(string(data))
	return nil
}

func main() {

	//Init the RebufOptions
	rebufOptions := &rebuf.RebufOptions{
		LogDir:      "/Users/satyamraj/personal/rebuf/data",
		MaxLogSize:  50,
		MaxSegments: 2,
		SyncMaxWait: 5 * time.Second,
	}

	//Init Rebuf
	rebuf, err := rebuf.Init(rebufOptions)
	if err != nil {
		fmt.Println("Error during Rebuf creation: " + err.Error())
	}

	defer rebuf.Close()

	//Write Bytes
	err = rebuf.Write([]byte("Hello world"))

	//Replay and write to stdout
	rebuf.Replay(writeToStdout)

	if err != nil {
		fmt.Println(err.Error())
	}

}

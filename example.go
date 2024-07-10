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
		FsyncTime:   5 * time.Second,
		MaxLogSize:  50,
		MaxSegments: 5,
	}

	//Init Rebuf
	rebuf, err := rebuf.Init(rebufOptions)
	if err != nil {
		fmt.Println("Error during Rebuf creation: " + err.Error())
	}

	defer rebuf.Close()

	// Write Bytes
	for i := 0; i < 30; i++ {
		fmt.Printf("Writing data iter#%d \n", i)
		err = rebuf.Write([]byte("Hello world"))
		time.Sleep(300 * time.Millisecond)
	}

	//Replay and write to stdout
	rebuf.Replay(writeToStdout)

	if err != nil {
		fmt.Println(err.Error())
	}

	time.Sleep(30 * time.Second)

}

package main

import (
	"fmt"
	"time"

	"github.com/stym06/rebuf/rebuf"
)

func main() {
	rebufOptions := &rebuf.RebufOptions{
		LogDir:      "/Users/satyamraj/personal/rebuf/data",
		MaxLogSize:  50,
		MaxSegments: 2,
		SyncMaxWait: 5 * time.Second,
	}

	rebuf, err := rebuf.Init(rebufOptions)
	if err != nil {
		fmt.Println("Error during Rebuf creation: " + err.Error())
	}

	defer rebuf.Close()

	err = rebuf.Write([]byte("Hello world"))
	if err != nil {
		fmt.Println(err.Error())
	}

}

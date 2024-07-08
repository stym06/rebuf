package main

import (
	"fmt"
	"time"

	"github.com/stym06/rebuf/rebuf"
)

func main() {
	rebufOptions := &rebuf.RebufOptions{
		LogDir:      "./data",
		MaxLogSize:  50,
		MaxSegments: 1,
		SyncMaxWait: 5 * time.Second,
	}

	rebuf, err := rebuf.Init(rebufOptions)
	if err != nil {
		fmt.Println("Error during Rebuf creation")
	}

	defer rebuf.Close()

	err = rebuf.Write([]byte("Hello world"))

}

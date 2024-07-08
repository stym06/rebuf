# rebuf
[![Go](https://github.com/stym06/rebuf/actions/workflows/go.yml/badge.svg)](https://github.com/stym06/rebuf/actions/workflows/go.yml)

`rebuf` is a Golang implementation of WAL (Write Ahead||After Logging) which can also be used to log data bytes during a downstream service issue which can later be replayed on-demand

## Features

- Create and replay log data on any filesystem.
- Lightweight and easy to use.
- Efficient storage and retrieval of log data.

## Installation

1. Clone the repository: `git clone https://github.com/stym06/rebuf.git`
2. Navigate to the project directory: `cd rebuf`
3. Install the necessary dependencies by running: `go mod download`

## Usage

```
example.go

rebufOptions := &rebuf.RebufOptions{
    LogDir:      "/Users/data",
    MaxLogSize:  50,
    MaxSegments: 2,
}

//Init Rebuf
rebuf, err := rebuf.Init(rebufOptions)
if err != nil {
    fmt.Println("Error during Rebuf creation: " + err.Error())
}

//Replay
rebuf.Replay(writeToStdout)


func writeToStdout(data []byte) error {
	fmt.Println(string(data))
	return nil
}
```


## License

This project is licensed under the MIT License. See the `LICENSE` file for more information.

## Contact Information

If you have any questions or concerns, please feel free to reach out to the author on GitHub: [@stym06](https://github.com/stym06).

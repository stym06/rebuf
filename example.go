package main

import (
	"context"
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

	r, err := rebuf.New(
		context.Background(),
		"./data",
		rebuf.WithMaxLogSize(50),
		rebuf.WithMaxSegments(5),
		rebuf.WithFsyncTime(5*time.Second),
		rebuf.WithSyncStrategy(rebuf.SyncEveryWrite),
		rebuf.WithLogger(logger),
	)
	if err != nil {
		logger.Error("failed to create rebuf", "error", err)
		os.Exit(1)
	}
	defer r.Close()

	// Write entries.
	for i := 0; i < 30; i++ {
		logger.Info("writing data", "iter", i)
		if err := r.Write([]byte("Hello world")); err != nil {
			logger.Error("write failed", "error", err)
		}
		time.Sleep(300 * time.Millisecond)
	}

	// Replay all entries.
	if err := r.Replay(writeToStdout); err != nil {
		logger.Error("replay failed", "error", err)
	}
}

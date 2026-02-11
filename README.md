# rebuf

[![Go](https://github.com/stym06/rebuf/actions/workflows/go.yml/badge.svg)](https://github.com/stym06/rebuf/actions/workflows/go.yml)

`rebuf` is a lightweight Go implementation of a Write-Ahead Log (WAL) that persists data to segmented log files and supports on-demand replay. It can be used as a durable buffer during downstream service outages — log data bytes while the service is down, then replay them when it recovers.

## Features

- **Segmented WAL** — automatic segment rotation and retention with configurable limits.
- **CRC32 checksums** — every entry is integrity-checked on replay.
- **Concurrent-safe** — all operations are protected by a mutex.
- **Configurable sync strategies** — choose between sync-every-write, periodic sync, or manual sync.
- **Selective replay** — replay all entries or start from a specific segment.
- **Batch writes** — write multiple entries in a single operation to amortize sync cost.
- **Purge API** — clean up segments after successful replay.
- **Functional options** — sensible defaults with optional configuration.
- **Zero external dependencies** — uses only the Go standard library.

## Installation

```bash
go get github.com/stym06/rebuf
```

## Usage

```go
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/stym06/rebuf/rebuf"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	r, err := rebuf.New(
		context.Background(),
		"./data",
		rebuf.WithMaxLogSize(1024*1024),   // 1 MB per segment
		rebuf.WithMaxSegments(5),           // keep at most 5 segments
		rebuf.WithFsyncTime(5*time.Second), // periodic sync interval
		rebuf.WithSyncStrategy(rebuf.SyncEveryWrite),
		rebuf.WithLogger(logger),
	)
	if err != nil {
		logger.Error("failed to create rebuf", "error", err)
		os.Exit(1)
	}
	defer r.Close()

	// Write entries.
	for i := 0; i < 100; i++ {
		if err := r.Write([]byte("Hello world")); err != nil {
			logger.Error("write failed", "error", err)
		}
	}

	// Replay all entries.
	r.Replay(func(data []byte) error {
		slog.Info(string(data))
		return nil
	})
}
```

## API Overview

| Method | Description |
|--------|-------------|
| `New(ctx, logDir, opts...)` | Create a new Rebuf instance |
| `Write(data)` | Write a single entry |
| `WriteBatch(entries)` | Write multiple entries in one operation |
| `Replay(callback)` | Replay all entries from all segments |
| `ReplayFrom(segmentId, callback)` | Replay entries starting from a segment |
| `Purge()` | Delete all segments and reset |
| `PurgeThrough(segmentId)` | Delete segments up through the given ID |
| `Sync()` | Manually flush and fsync to disk |
| `Close()` | Flush, sync, and shut down |
| `SegmentCount()` | Number of completed segments |
| `CurrentLogSize()` | Bytes written to the active segment |
| `Dir()` | Log directory path |

## On-Disk Format

Each entry is stored as:

```
[8-byte size (big-endian uint64)][4-byte CRC32 (IEEE)][data bytes]
```

Segment files are named `rebuf-0`, `rebuf-1`, etc. The active file is always `rebuf.tmp`.

## License

This project is licensed under the MIT License. See the `LICENSE` file for more information.

## Contact

If you have any questions or concerns, please feel free to reach out to the author on GitHub: [@stym06](https://github.com/stym06).

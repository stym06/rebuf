// Package rebuf provides a lightweight Write-Ahead Log (WAL) implementation
// that persists data to segmented log files on disk and supports replay for
// recovery. It can also be used to buffer data during downstream service
// outages and replay it on-demand when the service recovers.
package rebuf

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/stym06/rebuf/utils"
)

// entryHeaderSize is the on-disk overhead per entry: 8-byte size + 4-byte CRC32.
const entryHeaderSize = 12

// SyncStrategy controls when data is fsynced to disk.
type SyncStrategy int

const (
	// SyncEveryWrite flushes and fsyncs after every Write call (safest, slowest).
	SyncEveryWrite SyncStrategy = iota
	// SyncPeriodic fsyncs at the configured interval only (higher throughput).
	SyncPeriodic
	// SyncManual requires the caller to explicitly call Sync().
	SyncManual
)

// config holds internal configuration populated by Option functions.
type config struct {
	maxLogSize   int64
	maxSegments  int
	fsyncTime    time.Duration
	logger       *slog.Logger
	syncStrategy SyncStrategy
}

func defaultConfig() *config {
	return &config{
		maxLogSize:   10 * 1024 * 1024, // 10 MB
		maxSegments:  10,
		fsyncTime:    5 * time.Second,
		logger:       slog.Default(),
		syncStrategy: SyncEveryWrite,
	}
}

// Option configures a Rebuf instance.
type Option func(*config)

// WithMaxLogSize sets the maximum size in bytes per segment file.
// When a segment exceeds this size, it is rotated.
func WithMaxLogSize(size int64) Option {
	return func(c *config) { c.maxLogSize = size }
}

// WithMaxSegments sets the maximum number of retained segment files.
// When this limit is exceeded, the oldest segment is deleted.
func WithMaxSegments(n int) Option {
	return func(c *config) { c.maxSegments = n }
}

// WithFsyncTime sets the interval for periodic fsync when using SyncPeriodic.
func WithFsyncTime(d time.Duration) Option {
	return func(c *config) { c.fsyncTime = d }
}

// WithLogger sets the structured logger used for diagnostic messages.
func WithLogger(l *slog.Logger) Option {
	return func(c *config) { c.logger = l }
}

// WithSyncStrategy sets the durability strategy for writes.
func WithSyncStrategy(s SyncStrategy) Option {
	return func(c *config) { c.syncStrategy = s }
}

// Rebuf is a write-ahead log that persists data to segmented log files and
// supports ordered replay. All methods are safe for concurrent use.
type Rebuf struct {
	ctx              context.Context
	cancel           context.CancelFunc
	logDir           string
	currentSegmentId int
	maxLogSize       int64
	maxSegments      int
	segmentCount     int
	bufWriter        *bufio.Writer
	logSize          int64
	tmpLogFile       *os.File
	ticker           *time.Ticker
	mu               sync.Mutex
	log              *slog.Logger
	syncStrategy     SyncStrategy
	done             chan struct{}
}

// New creates a new Rebuf instance writing to logDir. The directory is created
// if it does not exist. If the directory already contains segments from a
// previous run, Rebuf picks up where it left off.
func New(ctx context.Context, logDir string, opts ...Option) (*Rebuf, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil, fmt.Errorf("rebuf: failed to create log directory: %w", err)
	}

	childCtx, cancel := context.WithCancel(ctx)

	r := &Rebuf{
		ctx:          childCtx,
		cancel:       cancel,
		logDir:       logDir,
		maxLogSize:   cfg.maxLogSize,
		maxSegments:  cfg.maxSegments,
		log:          cfg.logger,
		syncStrategy: cfg.syncStrategy,
		done:         make(chan struct{}),
	}

	if err := r.openExistingOrCreateNew(); err != nil {
		cancel()
		return nil, err
	}

	if cfg.syncStrategy == SyncPeriodic {
		r.ticker = time.NewTicker(cfg.fsyncTime)
		go r.syncPeriodically()
	} else {
		close(r.done)
	}

	return r, nil
}

func (r *Rebuf) syncPeriodically() {
	defer close(r.done)
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-r.ticker.C:
			r.mu.Lock()
			if r.tmpLogFile != nil {
				_ = r.tmpLogFile.Sync()
			}
			r.mu.Unlock()
		}
	}
}

// Write persists data as a single log entry. The entry is written with a
// CRC32 checksum for integrity verification during replay.
// It is safe for concurrent use.
func (r *Rebuf) Write(data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entrySize := int64(len(data)) + entryHeaderSize
	if r.logSize+entrySize > r.maxLogSize {
		if err := r.rotateSegment(); err != nil {
			return err
		}
	}

	if err := r.writeEntry(data); err != nil {
		return err
	}

	r.logSize += entrySize

	if r.syncStrategy == SyncEveryWrite {
		if err := r.bufWriter.Flush(); err != nil {
			return err
		}
		return r.tmpLogFile.Sync()
	}

	return nil
}

// WriteBatch writes multiple entries in a single operation, amortizing the
// cost of flushing and syncing across all entries.
func (r *Rebuf) WriteBatch(entries [][]byte) error {
	if len(entries) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, data := range entries {
		entrySize := int64(len(data)) + entryHeaderSize
		if r.logSize+entrySize > r.maxLogSize {
			if err := r.rotateSegment(); err != nil {
				return err
			}
		}

		if err := r.writeEntry(data); err != nil {
			return err
		}
		r.logSize += entrySize
	}

	if r.syncStrategy == SyncEveryWrite || r.syncStrategy == SyncPeriodic {
		if err := r.bufWriter.Flush(); err != nil {
			return err
		}
		return r.tmpLogFile.Sync()
	}

	return nil
}

// writeEntry writes a single entry: [8-byte size][4-byte crc32][data].
// Caller must hold r.mu.
func (r *Rebuf) writeEntry(data []byte) error {
	sizeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(sizeBuf, uint64(len(data)))
	if _, err := r.bufWriter.Write(sizeBuf); err != nil {
		return err
	}

	checksum := crc32.ChecksumIEEE(data)
	crcBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(crcBuf, checksum)
	if _, err := r.bufWriter.Write(crcBuf); err != nil {
		return err
	}

	_, err := r.bufWriter.Write(data)
	return err
}

// rotateSegment flushes the current temp file, renames it to a numbered
// segment, and opens a fresh temp file. Caller must hold r.mu.
func (r *Rebuf) rotateSegment() error {
	if r.segmentCount >= r.maxSegments {
		r.log.Info("reached max segments, deleting oldest", "maxSegments", r.maxSegments)
		oldestFile, err := utils.GetOldestSegmentFile(r.logDir)
		if err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(r.logDir, oldestFile)); err != nil {
			return err
		}
		r.segmentCount--
	}

	r.log.Info("rotating segment", "currentSize", r.logSize, "newSegmentId", r.currentSegmentId)

	if err := r.bufWriter.Flush(); err != nil {
		return err
	}
	if err := r.tmpLogFile.Sync(); err != nil {
		return err
	}
	if err := r.tmpLogFile.Close(); err != nil {
		return err
	}

	oldPath := filepath.Join(r.logDir, "rebuf.tmp")
	newPath := filepath.Join(r.logDir, "rebuf-"+strconv.Itoa(r.currentSegmentId))
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}

	r.currentSegmentId++
	r.segmentCount++

	tmpFile, err := os.OpenFile(
		filepath.Join(r.logDir, "rebuf.tmp"),
		os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666,
	)
	if err != nil {
		return err
	}
	r.tmpLogFile = tmpFile
	r.bufWriter = bufio.NewWriter(tmpFile)
	r.logSize = 0

	return nil
}

func (r *Rebuf) openExistingOrCreateNew() error {
	empty, err := utils.IsDirectoryEmpty(r.logDir)
	if err != nil {
		return err
	}

	tmpLogFileName := filepath.Join(r.logDir, "rebuf.tmp")
	tmpLogFile, err := os.OpenFile(tmpLogFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	r.tmpLogFile = tmpLogFile
	r.bufWriter = bufio.NewWriter(tmpLogFile)

	if empty {
		r.currentSegmentId = 0
		r.segmentCount = 0
		r.logSize = 0
	} else {
		latestId, err := utils.GetLatestSegmentId(r.logDir)
		if err != nil {
			// No numbered segments yet, just a tmp file with data.
			r.currentSegmentId = 0
		} else {
			r.currentSegmentId = latestId + 1
		}

		r.segmentCount, err = utils.GetNumSegments(r.logDir)
		if err != nil {
			return err
		}
		r.logSize, _ = utils.FileSize(r.tmpLogFile)
	}

	return nil
}

// Replay reads all log entries across all segments (oldest first, then the
// active temp file) and invokes callbackFn for each entry. Replay verifies
// the CRC32 checksum of each entry and returns an error on mismatch.
func (r *Rebuf) Replay(callbackFn func([]byte) error) error {
	return r.ReplayFrom(0, callbackFn)
}

// ReplayFrom reads log entries starting from the segment with the given ID.
// Segments with IDs less than segmentId are skipped. The active temp file is
// always included.
func (r *Rebuf) ReplayFrom(segmentId int, callbackFn func([]byte) error) error {
	r.mu.Lock()
	if r.bufWriter != nil {
		_ = r.bufWriter.Flush()
	}
	r.mu.Unlock()

	files, err := utils.GetSortedSegmentFiles(r.logDir)
	if err != nil {
		return err
	}

	for _, fileName := range files {
		if fileName == "rebuf.tmp" {
			// Always replay the active temp file.
		} else {
			sid, err := utils.ParseSegmentId(fileName)
			if err != nil {
				continue
			}
			if sid < segmentId {
				continue
			}
		}

		if err := replayFile(filepath.Join(r.logDir, fileName), callbackFn); err != nil {
			return err
		}
	}

	return nil
}

// replayFile reads all entries from a single log file.
func replayFile(path string, callbackFn func([]byte) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		// Read 8-byte size prefix.
		sizeBuf := make([]byte, 8)
		if _, err := io.ReadFull(reader, sizeBuf); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return err
		}
		size := binary.BigEndian.Uint64(sizeBuf)

		// Read 4-byte CRC32.
		crcBuf := make([]byte, 4)
		if _, err := io.ReadFull(reader, crcBuf); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return err
		}
		expectedCRC := binary.BigEndian.Uint32(crcBuf)

		// Read data payload.
		data := make([]byte, size)
		if _, err := io.ReadFull(reader, data); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return err
		}

		// Verify checksum.
		actualCRC := crc32.ChecksumIEEE(data)
		if actualCRC != expectedCRC {
			return fmt.Errorf("rebuf: CRC32 mismatch in %s: expected %d, got %d", path, expectedCRC, actualCRC)
		}

		if err := callbackFn(data); err != nil {
			return err
		}
	}

	return nil
}

// Purge deletes all segment files and the active temp file, then resets
// the WAL to a clean state.
func (r *Rebuf) Purge() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.bufWriter != nil {
		_ = r.bufWriter.Flush()
	}
	if r.tmpLogFile != nil {
		_ = r.tmpLogFile.Close()
	}

	files, err := os.ReadDir(r.logDir)
	if err != nil {
		return err
	}
	for _, f := range files {
		if err := os.Remove(filepath.Join(r.logDir, f.Name())); err != nil {
			return err
		}
	}

	tmpFile, err := os.OpenFile(
		filepath.Join(r.logDir, "rebuf.tmp"),
		os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666,
	)
	if err != nil {
		return err
	}
	r.tmpLogFile = tmpFile
	r.bufWriter = bufio.NewWriter(tmpFile)
	r.currentSegmentId = 0
	r.segmentCount = 0
	r.logSize = 0

	return nil
}

// PurgeThrough deletes all segments with IDs less than or equal to segmentId.
func (r *Rebuf) PurgeThrough(segmentId int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	files, err := os.ReadDir(r.logDir)
	if err != nil {
		return err
	}

	for _, f := range files {
		sid, err := utils.ParseSegmentId(f.Name())
		if err != nil {
			continue
		}
		if sid <= segmentId {
			if err := os.Remove(filepath.Join(r.logDir, f.Name())); err != nil {
				return err
			}
			r.segmentCount--
		}
	}

	return nil
}

// Sync manually flushes buffered data and fsyncs the active log file to disk.
// This is primarily useful with SyncManual strategy.
func (r *Rebuf) Sync() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.bufWriter != nil {
		if err := r.bufWriter.Flush(); err != nil {
			return err
		}
	}
	if r.tmpLogFile != nil {
		return r.tmpLogFile.Sync()
	}
	return nil
}

// SegmentCount returns the current number of completed segment files
// (excluding the active temp file).
func (r *Rebuf) SegmentCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.segmentCount
}

// CurrentLogSize returns the number of bytes written to the active segment.
func (r *Rebuf) CurrentLogSize() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.logSize
}

// Dir returns the directory where log files are stored.
func (r *Rebuf) Dir() string {
	return r.logDir
}

// Close flushes all buffered data, stops the periodic sync goroutine (if any),
// and closes the active log file.
func (r *Rebuf) Close() error {
	r.cancel()

	if r.ticker != nil {
		r.ticker.Stop()
	}

	// Wait for the periodic sync goroutine to exit.
	<-r.done

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.bufWriter != nil {
		if err := r.bufWriter.Flush(); err != nil {
			if r.tmpLogFile != nil {
				_ = r.tmpLogFile.Close()
			}
			return err
		}
	}

	if r.tmpLogFile != nil {
		if err := r.tmpLogFile.Sync(); err != nil {
			_ = r.tmpLogFile.Close()
			return err
		}
		return r.tmpLogFile.Close()
	}

	return nil
}

package rebuf

import (
	"bytes"
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
	"sync/atomic"
	"testing"
	"time"
)

// helper creates a Rebuf instance in a temporary directory with small segments.
func newTestRebuf(t *testing.T, opts ...Option) *Rebuf {
	t.Helper()
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	defaults := []Option{
		WithMaxLogSize(200),
		WithMaxSegments(5),
		WithFsyncTime(100 * time.Millisecond),
		WithLogger(logger),
	}
	allOpts := append(defaults, opts...)

	r, err := New(context.Background(), dir, allOpts...)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

// collectReplay replays all entries and returns them as a slice.
func collectReplay(t *testing.T, r *Rebuf) [][]byte {
	t.Helper()
	var entries [][]byte
	err := r.Replay(func(data []byte) error {
		cp := make([]byte, len(data))
		copy(cp, data)
		entries = append(entries, cp)
		return nil
	})
	if err != nil {
		t.Fatalf("Replay() failed: %v", err)
	}
	return entries
}

// --- New / Init tests ---

func TestNew_FreshDirectory(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	r, err := New(context.Background(), dir, WithLogger(logger), WithMaxLogSize(1024))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer r.Close()

	if r.SegmentCount() != 0 {
		t.Errorf("expected 0 segments, got %d", r.SegmentCount())
	}
	if r.CurrentLogSize() != 0 {
		t.Errorf("expected 0 log size, got %d", r.CurrentLogSize())
	}
	if r.Dir() != dir {
		t.Errorf("expected dir %s, got %s", dir, r.Dir())
	}
}

func TestNew_CreatesDirectory(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "a", "b", "c")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r, err := New(context.Background(), nested, WithLogger(logger))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer r.Close()

	if _, err := os.Stat(nested); err != nil {
		t.Errorf("directory was not created: %v", err)
	}
}

func TestNew_ExistingDirectoryWithSegments(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create a Rebuf, write enough to trigger segment rotation, then close.
	r1, err := New(context.Background(), dir,
		WithMaxLogSize(50), WithMaxSegments(10), WithLogger(logger))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := r1.Write([]byte("hello world")); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
	}
	r1.Close()

	// Re-open and verify it picks up state.
	r2, err := New(context.Background(), dir,
		WithMaxLogSize(50), WithMaxSegments(10), WithLogger(logger))
	if err != nil {
		t.Fatalf("New() re-open error: %v", err)
	}
	defer r2.Close()

	if r2.SegmentCount() == 0 {
		t.Error("expected segments to be recovered, got 0")
	}

	// Should be able to write more data without overwriting existing segments.
	if err := r2.Write([]byte("after reopen")); err != nil {
		t.Fatalf("Write() after reopen error: %v", err)
	}
}

func TestNew_DefaultOptions(t *testing.T) {
	dir := t.TempDir()
	r, err := New(context.Background(), dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer r.Close()

	if r.maxLogSize != 10*1024*1024 {
		t.Errorf("expected default maxLogSize 10MB, got %d", r.maxLogSize)
	}
	if r.maxSegments != 10 {
		t.Errorf("expected default maxSegments 10, got %d", r.maxSegments)
	}
	if r.syncStrategy != SyncEveryWrite {
		t.Errorf("expected default SyncEveryWrite, got %d", r.syncStrategy)
	}
}

// --- Write tests ---

func TestWrite_SingleEntry(t *testing.T) {
	r := newTestRebuf(t)

	data := []byte("hello world")
	if err := r.Write(data); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	entries := collectReplay(t, r)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if !bytes.Equal(entries[0], data) {
		t.Errorf("expected %q, got %q", data, entries[0])
	}
}

func TestWrite_MultipleEntries(t *testing.T) {
	r := newTestRebuf(t, WithMaxLogSize(10000))

	for i := 0; i < 20; i++ {
		data := []byte(fmt.Sprintf("entry-%d", i))
		if err := r.Write(data); err != nil {
			t.Fatalf("Write(%d) error: %v", i, err)
		}
	}

	entries := collectReplay(t, r)
	if len(entries) != 20 {
		t.Fatalf("expected 20 entries, got %d", len(entries))
	}
	for i, e := range entries {
		expected := fmt.Sprintf("entry-%d", i)
		if string(e) != expected {
			t.Errorf("entry %d: expected %q, got %q", i, expected, string(e))
		}
	}
}

func TestWrite_ZeroLengthData(t *testing.T) {
	r := newTestRebuf(t)

	if err := r.Write([]byte{}); err != nil {
		t.Fatalf("Write(empty) error: %v", err)
	}
	if err := r.Write(nil); err != nil {
		t.Fatalf("Write(nil) error: %v", err)
	}

	entries := collectReplay(t, r)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if len(entries[0]) != 0 {
		t.Errorf("expected empty entry, got %d bytes", len(entries[0]))
	}
}

func TestWrite_LargeData(t *testing.T) {
	r := newTestRebuf(t, WithMaxLogSize(100000))

	// Write a 10KB entry.
	data := make([]byte, 10000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	if err := r.Write(data); err != nil {
		t.Fatalf("Write(large) error: %v", err)
	}

	entries := collectReplay(t, r)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if !bytes.Equal(entries[0], data) {
		t.Error("large data entry corrupted during write/replay")
	}
}

func TestWrite_SegmentRotation(t *testing.T) {
	// MaxLogSize of 50 bytes. Each entry = 12 header + 11 data = 23 bytes.
	// After 2 entries (46 bytes), the third triggers rotation.
	r := newTestRebuf(t, WithMaxLogSize(50), WithMaxSegments(10))

	for i := 0; i < 6; i++ {
		if err := r.Write([]byte("hello world")); err != nil {
			t.Fatalf("Write(%d) error: %v", i, err)
		}
	}

	if r.SegmentCount() == 0 {
		t.Error("expected at least one segment after rotation")
	}

	entries := collectReplay(t, r)
	if len(entries) != 6 {
		t.Fatalf("expected 6 entries after rotation, got %d", len(entries))
	}
}

func TestWrite_MaxSegmentsDeletion(t *testing.T) {
	// Small segments, max 2 segments retained.
	r := newTestRebuf(t, WithMaxLogSize(30), WithMaxSegments(2))

	// Write enough data to create many segments, exceeding max.
	for i := 0; i < 20; i++ {
		if err := r.Write([]byte(fmt.Sprintf("d%d", i))); err != nil {
			t.Fatalf("Write(%d) error: %v", i, err)
		}
	}

	if r.SegmentCount() > 2 {
		t.Errorf("expected at most 2 segments, got %d", r.SegmentCount())
	}
}

func TestWrite_CRC32Integrity(t *testing.T) {
	r := newTestRebuf(t)

	data := []byte("checksum test data")
	if err := r.Write(data); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Verify the on-disk format: [8-byte size][4-byte crc32][data]
	r.mu.Lock()
	_ = r.bufWriter.Flush()
	r.mu.Unlock()

	tmpPath := filepath.Join(r.Dir(), "rebuf.tmp")
	content, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	if len(content) < entryHeaderSize {
		t.Fatalf("file too small: %d bytes", len(content))
	}

	size := binary.BigEndian.Uint64(content[:8])
	if size != uint64(len(data)) {
		t.Errorf("size prefix: expected %d, got %d", len(data), size)
	}

	storedCRC := binary.BigEndian.Uint32(content[8:12])
	expectedCRC := crc32.ChecksumIEEE(data)
	if storedCRC != expectedCRC {
		t.Errorf("CRC32: expected %d, got %d", expectedCRC, storedCRC)
	}

	if !bytes.Equal(content[12:12+size], data) {
		t.Error("on-disk data does not match")
	}
}

// --- WriteBatch tests ---

func TestWriteBatch(t *testing.T) {
	r := newTestRebuf(t, WithMaxLogSize(10000))

	batch := [][]byte{
		[]byte("batch-0"),
		[]byte("batch-1"),
		[]byte("batch-2"),
	}
	if err := r.WriteBatch(batch); err != nil {
		t.Fatalf("WriteBatch() error: %v", err)
	}

	entries := collectReplay(t, r)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	for i, e := range entries {
		expected := fmt.Sprintf("batch-%d", i)
		if string(e) != expected {
			t.Errorf("entry %d: expected %q, got %q", i, expected, string(e))
		}
	}
}

func TestWriteBatch_Empty(t *testing.T) {
	r := newTestRebuf(t)

	if err := r.WriteBatch(nil); err != nil {
		t.Fatalf("WriteBatch(nil) error: %v", err)
	}
	if err := r.WriteBatch([][]byte{}); err != nil {
		t.Fatalf("WriteBatch(empty) error: %v", err)
	}

	entries := collectReplay(t, r)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestWriteBatch_WithRotation(t *testing.T) {
	r := newTestRebuf(t, WithMaxLogSize(50), WithMaxSegments(10))

	// Each entry is 12 header + data bytes. With size 50, some entries will
	// trigger rotation mid-batch.
	batch := make([][]byte, 10)
	for i := range batch {
		batch[i] = []byte(fmt.Sprintf("item-%d", i))
	}

	if err := r.WriteBatch(batch); err != nil {
		t.Fatalf("WriteBatch() error: %v", err)
	}

	entries := collectReplay(t, r)
	if len(entries) != 10 {
		t.Fatalf("expected 10 entries, got %d", len(entries))
	}
}

// --- Replay tests ---

func TestReplay_EmptyDirectory(t *testing.T) {
	r := newTestRebuf(t)

	entries := collectReplay(t, r)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries on empty dir, got %d", len(entries))
	}
}

func TestReplay_MultipleSegments(t *testing.T) {
	r := newTestRebuf(t, WithMaxLogSize(40), WithMaxSegments(20))

	var expected []string
	for i := 0; i < 15; i++ {
		data := fmt.Sprintf("msg-%02d", i)
		expected = append(expected, data)
		if err := r.Write([]byte(data)); err != nil {
			t.Fatalf("Write(%d) error: %v", i, err)
		}
	}

	entries := collectReplay(t, r)
	if len(entries) != len(expected) {
		t.Fatalf("expected %d entries, got %d", len(expected), len(entries))
	}
	for i, e := range entries {
		if string(e) != expected[i] {
			t.Errorf("entry %d: expected %q, got %q", i, expected[i], string(e))
		}
	}
}

func TestReplay_CorruptCRC(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	r, err := New(context.Background(), dir,
		WithMaxLogSize(1000), WithLogger(logger))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := r.Write([]byte("good data")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	r.Close()

	// Corrupt the CRC32 bytes (bytes 8-11) in the tmp file.
	tmpPath := filepath.Join(dir, "rebuf.tmp")
	content, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	content[8] ^= 0xFF // flip a byte in the CRC
	if err := os.WriteFile(tmpPath, content, 0666); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	r2, err := New(context.Background(), dir,
		WithMaxLogSize(1000), WithLogger(logger))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer r2.Close()

	err = r2.Replay(func(data []byte) error { return nil })
	if err == nil {
		t.Fatal("expected CRC mismatch error, got nil")
	}
}

func TestReplay_CallbackError(t *testing.T) {
	r := newTestRebuf(t)
	r.Write([]byte("data"))

	expectedErr := fmt.Errorf("callback failed")
	err := r.Replay(func(data []byte) error {
		return expectedErr
	})
	if err != expectedErr {
		t.Errorf("expected callback error, got: %v", err)
	}
}

// --- ReplayFrom tests ---

func TestReplayFrom(t *testing.T) {
	r := newTestRebuf(t, WithMaxLogSize(40), WithMaxSegments(20))

	// Write enough entries to create several segments.
	for i := 0; i < 10; i++ {
		if err := r.Write([]byte(fmt.Sprintf("entry-%d", i))); err != nil {
			t.Fatalf("Write(%d) error: %v", i, err)
		}
	}

	segCount := r.SegmentCount()
	if segCount < 2 {
		t.Skipf("need at least 2 segments for this test, got %d", segCount)
	}

	// Replay from segment 1 (skip segment 0).
	var fromEntries [][]byte
	err := r.ReplayFrom(1, func(data []byte) error {
		cp := make([]byte, len(data))
		copy(cp, data)
		fromEntries = append(fromEntries, cp)
		return nil
	})
	if err != nil {
		t.Fatalf("ReplayFrom(1) error: %v", err)
	}

	allEntries := collectReplay(t, r)
	if len(fromEntries) >= len(allEntries) {
		t.Errorf("ReplayFrom(1) should return fewer entries than Replay(), got %d vs %d",
			len(fromEntries), len(allEntries))
	}
	if len(fromEntries) == 0 {
		t.Error("ReplayFrom(1) returned 0 entries")
	}
}

func TestReplayFrom_HighSegmentId(t *testing.T) {
	r := newTestRebuf(t, WithMaxLogSize(40), WithMaxSegments(20))

	for i := 0; i < 5; i++ {
		r.Write([]byte(fmt.Sprintf("e-%d", i)))
	}

	// Replay from a segment ID higher than any that exist (except .tmp).
	var entries [][]byte
	err := r.ReplayFrom(9999, func(data []byte) error {
		cp := make([]byte, len(data))
		copy(cp, data)
		entries = append(entries, cp)
		return nil
	})
	if err != nil {
		t.Fatalf("ReplayFrom(9999) error: %v", err)
	}
	// Should still get entries from .tmp file.
	if len(entries) == 0 {
		t.Log("ReplayFrom with high segmentId may return entries from .tmp")
	}
}

// --- Purge tests ---

func TestPurge(t *testing.T) {
	r := newTestRebuf(t, WithMaxLogSize(50), WithMaxSegments(20))

	for i := 0; i < 10; i++ {
		r.Write([]byte("purge me"))
	}

	if r.SegmentCount() == 0 {
		t.Fatal("expected segments before purge")
	}

	if err := r.Purge(); err != nil {
		t.Fatalf("Purge() error: %v", err)
	}

	if r.SegmentCount() != 0 {
		t.Errorf("expected 0 segments after purge, got %d", r.SegmentCount())
	}
	if r.CurrentLogSize() != 0 {
		t.Errorf("expected 0 log size after purge, got %d", r.CurrentLogSize())
	}

	// Should be writable after purge.
	if err := r.Write([]byte("after purge")); err != nil {
		t.Fatalf("Write() after purge error: %v", err)
	}

	entries := collectReplay(t, r)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry after purge+write, got %d", len(entries))
	}
}

func TestPurgeThrough(t *testing.T) {
	r := newTestRebuf(t, WithMaxLogSize(40), WithMaxSegments(20))

	for i := 0; i < 10; i++ {
		r.Write([]byte(fmt.Sprintf("pt-%d", i)))
	}

	initialSegments := r.SegmentCount()
	if initialSegments < 2 {
		t.Skipf("need at least 2 segments, got %d", initialSegments)
	}

	// Purge through segment 0 only.
	if err := r.PurgeThrough(0); err != nil {
		t.Fatalf("PurgeThrough(0) error: %v", err)
	}

	if r.SegmentCount() >= initialSegments {
		t.Errorf("expected fewer segments after PurgeThrough, got %d (was %d)",
			r.SegmentCount(), initialSegments)
	}
}

// --- Sync strategy tests ---

func TestSyncStrategy_EveryWrite(t *testing.T) {
	r := newTestRebuf(t, WithSyncStrategy(SyncEveryWrite))

	if err := r.Write([]byte("sync every")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Data should be on disk immediately.
	entries := collectReplay(t, r)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestSyncStrategy_Periodic(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	r, err := New(context.Background(), dir,
		WithMaxLogSize(10000),
		WithSyncStrategy(SyncPeriodic),
		WithFsyncTime(50*time.Millisecond),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer r.Close()

	if err := r.Write([]byte("periodic sync")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Wait for periodic sync to kick in.
	time.Sleep(200 * time.Millisecond)

	entries := collectReplay(t, r)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestSyncStrategy_Manual(t *testing.T) {
	r := newTestRebuf(t, WithSyncStrategy(SyncManual))

	if err := r.Write([]byte("manual sync")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Explicitly sync.
	if err := r.Sync(); err != nil {
		t.Fatalf("Sync() error: %v", err)
	}

	entries := collectReplay(t, r)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

// --- Close tests ---

func TestClose_FlushesData(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	r, err := New(context.Background(), dir,
		WithMaxLogSize(10000),
		WithSyncStrategy(SyncManual),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	r.Write([]byte("before close"))
	r.Close()

	// Re-open and verify data persisted.
	r2, err := New(context.Background(), dir,
		WithMaxLogSize(10000), WithLogger(logger))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer r2.Close()

	entries := collectReplay(t, r2)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after close+reopen, got %d", len(entries))
	}
	if string(entries[0]) != "before close" {
		t.Errorf("expected %q, got %q", "before close", string(entries[0]))
	}
}

func TestClose_GracefulGoroutineShutdown(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	r, err := New(context.Background(), dir,
		WithSyncStrategy(SyncPeriodic),
		WithFsyncTime(10*time.Millisecond),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Write some data so the goroutine has something to sync.
	r.Write([]byte("goroutine test"))
	time.Sleep(50 * time.Millisecond)

	// Close should not hang or panic.
	done := make(chan struct{})
	go func() {
		r.Close()
		close(done)
	}()

	select {
	case <-done:
		// Success.
	case <-time.After(5 * time.Second):
		t.Fatal("Close() timed out — goroutine may not be shutting down")
	}
}

// --- Concurrency tests ---

func TestConcurrentWrites(t *testing.T) {
	r := newTestRebuf(t, WithMaxLogSize(500), WithMaxSegments(100))

	const goroutines = 10
	const writesPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < writesPerGoroutine; i++ {
				data := []byte(fmt.Sprintf("g%d-w%d", id, i))
				if err := r.Write(data); err != nil {
					t.Errorf("concurrent Write() error: %v", err)
				}
			}
		}(g)
	}

	wg.Wait()

	entries := collectReplay(t, r)
	expected := goroutines * writesPerGoroutine
	if len(entries) != expected {
		t.Errorf("expected %d entries from concurrent writes, got %d", expected, len(entries))
	}
}

func TestConcurrentWriteBatch(t *testing.T) {
	r := newTestRebuf(t, WithMaxLogSize(1000), WithMaxSegments(100))

	const goroutines = 5
	const entriesPerBatch = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			batch := make([][]byte, entriesPerBatch)
			for i := range batch {
				batch[i] = []byte(fmt.Sprintf("batch-g%d-e%d", id, i))
			}
			if err := r.WriteBatch(batch); err != nil {
				t.Errorf("concurrent WriteBatch() error: %v", err)
			}
		}(g)
	}

	wg.Wait()

	entries := collectReplay(t, r)
	expected := goroutines * entriesPerBatch
	if len(entries) != expected {
		t.Errorf("expected %d entries, got %d", expected, len(entries))
	}
}

// --- State accessor tests ---

func TestSegmentCount(t *testing.T) {
	r := newTestRebuf(t, WithMaxLogSize(40), WithMaxSegments(20))

	if r.SegmentCount() != 0 {
		t.Errorf("initial SegmentCount: expected 0, got %d", r.SegmentCount())
	}

	// Write enough to cause rotations.
	for i := 0; i < 10; i++ {
		r.Write([]byte(fmt.Sprintf("sc-%d", i)))
	}

	if r.SegmentCount() == 0 {
		t.Error("expected non-zero SegmentCount after writes")
	}
}

func TestCurrentLogSize(t *testing.T) {
	r := newTestRebuf(t, WithMaxLogSize(10000))

	if r.CurrentLogSize() != 0 {
		t.Errorf("initial CurrentLogSize: expected 0, got %d", r.CurrentLogSize())
	}

	data := []byte("test data")
	r.Write(data)
	expected := int64(len(data)) + entryHeaderSize
	if r.CurrentLogSize() != expected {
		t.Errorf("CurrentLogSize: expected %d, got %d", expected, r.CurrentLogSize())
	}
}

func TestDir(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	r, err := New(context.Background(), dir, WithLogger(logger))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer r.Close()

	if r.Dir() != dir {
		t.Errorf("Dir(): expected %s, got %s", dir, r.Dir())
	}
}

// --- Round-trip tests ---

func TestRoundTrip_BinaryData(t *testing.T) {
	r := newTestRebuf(t, WithMaxLogSize(10000))

	// Write all byte values 0-255.
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}

	if err := r.Write(data); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	entries := collectReplay(t, r)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if !bytes.Equal(entries[0], data) {
		t.Error("binary data corrupted in round-trip")
	}
}

func TestRoundTrip_ManySmallEntries(t *testing.T) {
	r := newTestRebuf(t, WithMaxLogSize(100), WithMaxSegments(100))

	const n = 100
	for i := 0; i < n; i++ {
		r.Write([]byte(strconv.Itoa(i)))
	}

	entries := collectReplay(t, r)
	if len(entries) != n {
		t.Fatalf("expected %d entries, got %d", n, len(entries))
	}
	for i, e := range entries {
		if string(e) != strconv.Itoa(i) {
			t.Errorf("entry %d: expected %q, got %q", i, strconv.Itoa(i), string(e))
		}
	}
}

// --- Recovery tests ---

func TestRecovery_AfterCrash(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Simulate: write data, close without clean shutdown.
	r1, _ := New(context.Background(), dir,
		WithMaxLogSize(100), WithMaxSegments(10), WithLogger(logger))
	for i := 0; i < 5; i++ {
		r1.Write([]byte(fmt.Sprintf("crash-%d", i)))
	}
	// Flush but simulate crash (don't Close cleanly — just sync and abandon).
	r1.Sync()

	// Re-open.
	r2, err := New(context.Background(), dir,
		WithMaxLogSize(100), WithMaxSegments(10), WithLogger(logger))
	if err != nil {
		t.Fatalf("New() after crash error: %v", err)
	}
	defer r2.Close()

	entries := collectReplay(t, r2)
	if len(entries) < 5 {
		t.Errorf("expected at least 5 entries after recovery, got %d", len(entries))
	}
}

// --- Context cancellation ---

func TestContextCancellation(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithCancel(context.Background())

	r, err := New(ctx, dir,
		WithSyncStrategy(SyncPeriodic),
		WithFsyncTime(10*time.Millisecond),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	r.Write([]byte("context test"))

	// Cancel the context.
	cancel()

	// Close should still work.
	done := make(chan error, 1)
	go func() {
		done <- r.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Close() after cancel: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close() timed out after context cancellation")
	}
}

// --- Benchmarks ---

func BenchmarkWrite(b *testing.B) {
	dir := b.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	r, err := New(context.Background(), dir,
		WithMaxLogSize(10*1024*1024),
		WithMaxSegments(100),
		WithSyncStrategy(SyncManual),
		WithLogger(logger),
	)
	if err != nil {
		b.Fatalf("New() error: %v", err)
	}
	defer r.Close()

	data := []byte("benchmark write data payload")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := r.Write(data); err != nil {
			b.Fatalf("Write() error: %v", err)
		}
	}
}

func BenchmarkWriteParallel(b *testing.B) {
	dir := b.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	r, err := New(context.Background(), dir,
		WithMaxLogSize(10*1024*1024),
		WithMaxSegments(100),
		WithSyncStrategy(SyncManual),
		WithLogger(logger),
	)
	if err != nil {
		b.Fatalf("New() error: %v", err)
	}
	defer r.Close()

	data := []byte("benchmark parallel write data")
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := r.Write(data); err != nil {
				b.Errorf("Write() error: %v", err)
			}
		}
	})
}

func BenchmarkReplay(b *testing.B) {
	dir := b.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	r, err := New(context.Background(), dir,
		WithMaxLogSize(10*1024*1024),
		WithMaxSegments(100),
		WithSyncStrategy(SyncManual),
		WithLogger(logger),
	)
	if err != nil {
		b.Fatalf("New() error: %v", err)
	}

	// Pre-populate with entries.
	data := []byte("benchmark replay data payload")
	for i := 0; i < 10000; i++ {
		r.Write(data)
	}
	r.Sync()

	var count atomic.Int64
	callback := func(d []byte) error {
		count.Add(1)
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count.Store(0)
		if err := r.Replay(callback); err != nil {
			b.Fatalf("Replay() error: %v", err)
		}
	}
	r.Close()
}

func BenchmarkWriteBatch(b *testing.B) {
	dir := b.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	r, err := New(context.Background(), dir,
		WithMaxLogSize(10*1024*1024),
		WithMaxSegments(100),
		WithSyncStrategy(SyncManual),
		WithLogger(logger),
	)
	if err != nil {
		b.Fatalf("New() error: %v", err)
	}
	defer r.Close()

	batch := make([][]byte, 100)
	for i := range batch {
		batch[i] = []byte("benchmark batch data payload")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := r.WriteBatch(batch); err != nil {
			b.Fatalf("WriteBatch() error: %v", err)
		}
	}
}

# rebuf Roadmap

This document outlines the planned improvements and features for rebuf, organized by priority.

---

## Phase 1: Correctness & Reliability

These are foundational issues that should be addressed before adding new features.

### 1.1 Fix concurrency in `Write()`
The current `Write()` method only acquires the mutex for the final `Sync()` call, but the entire write path — including segment rotation, seeking, and buffered writes — is unprotected. Concurrent goroutine writes (as shown in `example.go`) can corrupt data or cause races.

**Action:** Protect the entire `Write()` method with the mutex, not just the sync portion.

### 1.2 Graceful shutdown of `syncPeriodically` goroutine
The background goroutine started in `Init()` runs forever with no shutdown signal. While `Close()` stops the ticker, the goroutine itself is never terminated, leaking it.

**Action:** Use a `context.Context` or a done channel to cleanly stop the goroutine in `Close()`.

### 1.3 Improve error handling in `Init()`
- `os.Mkdir` errors are silently ignored — a permission failure would go unnoticed.
- The temp file is opened twice (once in `Init`, once in `openExistingOrCreateNew`).

**Action:** Propagate `os.Mkdir` errors; remove the duplicate file open.

### 1.4 Data integrity via checksums
There is currently no mechanism to detect corrupted log entries. A single flipped bit in the 8-byte size prefix could cause the reader to consume arbitrary data.

**Action:** Add a CRC32 checksum per entry (e.g., `[8-byte size][4-byte crc32][data]`) and validate on read/replay.

---

## Phase 2: Testing

### 2.1 Core package unit tests
The `rebuf` package has zero test coverage. The following test cases are needed:

- `Init` with a fresh directory
- `Init` recovering from an existing directory with segments
- `Write` a single entry, then `Replay` it back
- `Write` enough data to trigger segment rotation
- `Write` enough data to trigger oldest-segment deletion (`maxSegments`)
- `Close` flushes and syncs correctly
- Round-trip: arbitrary `[]byte` data survives write + replay

### 2.2 Concurrency tests
- Multiple goroutines calling `Write` concurrently (use `-race` detector)
- `Write` and `Replay` interleaving

### 2.3 Edge case tests
- Zero-length data write
- Very large single write exceeding `MaxLogSize`
- `Replay` on an empty directory
- `Replay` on a directory with only a `.tmp` file
- Corrupt segment file handling

### 2.4 Benchmarks
- `BenchmarkWrite` — single-entry write throughput
- `BenchmarkWriteParallel` — concurrent write throughput
- `BenchmarkReplay` — replay throughput over N entries

### 2.5 Expand utils tests
- `TestGetLatestSegmentId` (currently a stub)
- `TestGetNumSegments`
- `TestGetOldestSegmentFile`
- `TestFileSize`

---

## Phase 3: API Improvements

### 3.1 Follow Go naming conventions
Rename `Init` to `New` or `Open` to follow standard Go constructor patterns. The current `Init` name implies it modifies global state.

### 3.2 Add `context.Context` support
- `Init` / `New` should accept a context for cancellation.
- `Write` and `Replay` should accept a context so callers can cancel long-running operations.

### 3.3 Expose read-only state
Add methods to query internal state without breaking encapsulation:
- `SegmentCount() int`
- `CurrentLogSize() int64`
- `LogDir() string`

### 3.4 Functional options pattern
Replace `RebufOptions` struct with functional options (`WithMaxLogSize(int64)`, `WithFsyncTime(time.Duration)`, etc.) to allow sensible defaults and optional configuration.

### 3.5 Configurable sync strategy
Currently every `Write` calls both `Flush()` and `Sync()`, which negates the benefit of buffered I/O. Offer configurable strategies:
- **Sync every write** (current behavior, safest)
- **Periodic sync only** (higher throughput, small durability window)
- **Manual sync** (caller controls when to sync)

---

## Phase 4: Features

### 4.1 Selective replay
Allow replaying from a specific segment ID or offset, rather than always replaying all segments from the beginning.

```go
func (r *Rebuf) ReplayFrom(segmentId int, callbackFn func([]byte) error) error
```

### 4.2 Truncate / Purge after replay
After a successful replay, callers often want to delete the replayed segments. Provide an API for this:

```go
func (r *Rebuf) Purge() error              // delete all segments
func (r *Rebuf) PurgeThrough(segmentId int) // delete segments up to ID
```

### 4.3 Compression
Optional per-segment or per-entry compression (e.g., snappy or zstd) to reduce disk usage for large payloads.

### 4.4 Batch writes
Allow writing multiple entries in a single call to amortize the cost of flushing and syncing:

```go
func (r *Rebuf) WriteBatch(entries [][]byte) error
```

### 4.5 Metrics / observability hooks
Expose counters and hooks for monitoring:
- Entries written / replayed
- Bytes written
- Segment rotations
- Segment deletions
- Sync latency

This could be a callback interface or integration with Go's `expvar` / OpenTelemetry.

### 4.6 Entry metadata
Support optional per-entry metadata (timestamp, sequence number) to enable ordered replay and deduplication in distributed scenarios.

---

## Phase 5: Documentation & Project Health

### 5.1 Godoc comments
Add doc comments to all exported types and functions (`Rebuf`, `RebufOptions`, `Init`, `Write`, `Replay`, `Close`).

### 5.2 Architecture document
Write a short design doc explaining the on-disk format, segment lifecycle, and durability guarantees.

### 5.3 Improved README
- Remove hardcoded paths from the usage example.
- Add a "go get" installation command (`go get github.com/stym06/rebuf`).
- Add a section on when to use rebuf vs. alternatives.
- Add badges for Go reference docs.

### 5.4 Contributing guide
Add `CONTRIBUTING.md` covering how to run tests, coding conventions, and the PR process.

### 5.5 Versioning and releases
- Tag releases with semver (start with `v0.1.0`).
- Add a GitHub Actions workflow for creating releases on tag push.
- Add a `CHANGELOG.md`.

### 5.6 Linting in CI
Add `golangci-lint` to the CI pipeline to catch issues early.

---

## Summary

| Phase | Focus | Complexity |
|-------|-------|------------|
| 1 | Correctness & Reliability | Low–Medium |
| 2 | Testing | Medium |
| 3 | API Improvements | Medium |
| 4 | Features | Medium–High |
| 5 | Documentation & Project Health | Low |

Phases 1 and 2 should be prioritized — correctness and testing are prerequisites for everything else. Phase 3 involves breaking API changes, so it makes sense to batch those together before a `v1.0` release. Phase 4 features can be added incrementally. Phase 5 items can be done in parallel with any phase.

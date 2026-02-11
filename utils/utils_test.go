package utils

import (
	"os"
	"path/filepath"
	"testing"
)

// --- IsDirectoryEmpty tests ---

func TestIsDirectoryEmpty_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	empty, err := IsDirectoryEmpty(dir)
	if err != nil {
		t.Fatalf("IsDirectoryEmpty() error: %v", err)
	}
	if !empty {
		t.Error("expected empty directory to return true")
	}
}

func TestIsDirectoryEmpty_OnlyTmpFile(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "rebuf.tmp"))

	empty, err := IsDirectoryEmpty(dir)
	if err != nil {
		t.Fatalf("IsDirectoryEmpty() error: %v", err)
	}
	if !empty {
		t.Error("expected directory with only .tmp file to return true")
	}
}

func TestIsDirectoryEmpty_WithSegmentFile(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "rebuf.tmp"))
	createFile(t, filepath.Join(dir, "rebuf-0"))

	empty, err := IsDirectoryEmpty(dir)
	if err != nil {
		t.Fatalf("IsDirectoryEmpty() error: %v", err)
	}
	if empty {
		t.Error("expected directory with segment file to return false")
	}
}

func TestIsDirectoryEmpty_MultipleTmpFiles(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "foo.tmp"))
	createFile(t, filepath.Join(dir, "bar.tmp"))

	empty, err := IsDirectoryEmpty(dir)
	if err != nil {
		t.Fatalf("IsDirectoryEmpty() error: %v", err)
	}
	if !empty {
		t.Error("expected directory with only .tmp files to return true")
	}
}

func TestIsDirectoryEmpty_NonExistentDir(t *testing.T) {
	_, err := IsDirectoryEmpty("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

// --- ParseSegmentId tests ---

func TestParseSegmentId_Valid(t *testing.T) {
	tests := []struct {
		filename string
		expected int
	}{
		{"rebuf-0", 0},
		{"rebuf-1", 1},
		{"rebuf-42", 42},
		{"rebuf-999", 999},
	}

	for _, tt := range tests {
		id, err := ParseSegmentId(tt.filename)
		if err != nil {
			t.Errorf("ParseSegmentId(%q) error: %v", tt.filename, err)
		}
		if id != tt.expected {
			t.Errorf("ParseSegmentId(%q) = %d, want %d", tt.filename, id, tt.expected)
		}
	}
}

func TestParseSegmentId_Invalid(t *testing.T) {
	invalid := []string{
		"rebuf.tmp",
		"other-file",
		"rebuf-",
		"rebuf-abc",
		"",
	}

	for _, name := range invalid {
		_, err := ParseSegmentId(name)
		if err == nil {
			t.Errorf("ParseSegmentId(%q) expected error, got nil", name)
		}
	}
}

// --- GetSortedSegmentFiles tests ---

func TestGetSortedSegmentFiles_Empty(t *testing.T) {
	dir := t.TempDir()

	files, err := GetSortedSegmentFiles(dir)
	if err != nil {
		t.Fatalf("GetSortedSegmentFiles() error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestGetSortedSegmentFiles_OnlyTmp(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "rebuf.tmp"))

	files, err := GetSortedSegmentFiles(dir)
	if err != nil {
		t.Fatalf("GetSortedSegmentFiles() error: %v", err)
	}
	if len(files) != 1 || files[0] != "rebuf.tmp" {
		t.Errorf("expected [rebuf.tmp], got %v", files)
	}
}

func TestGetSortedSegmentFiles_Sorted(t *testing.T) {
	dir := t.TempDir()
	// Create out of order.
	createFile(t, filepath.Join(dir, "rebuf-2"))
	createFile(t, filepath.Join(dir, "rebuf-0"))
	createFile(t, filepath.Join(dir, "rebuf-1"))
	createFile(t, filepath.Join(dir, "rebuf.tmp"))

	files, err := GetSortedSegmentFiles(dir)
	if err != nil {
		t.Fatalf("GetSortedSegmentFiles() error: %v", err)
	}

	expected := []string{"rebuf-0", "rebuf-1", "rebuf-2", "rebuf.tmp"}
	if len(files) != len(expected) {
		t.Fatalf("expected %d files, got %d: %v", len(expected), len(files), files)
	}
	for i, f := range files {
		if f != expected[i] {
			t.Errorf("files[%d] = %q, want %q", i, f, expected[i])
		}
	}
}

func TestGetSortedSegmentFiles_IgnoresNonRebufFiles(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "rebuf-0"))
	createFile(t, filepath.Join(dir, ".DS_Store"))
	createFile(t, filepath.Join(dir, "other.log"))
	createFile(t, filepath.Join(dir, "rebuf.tmp"))

	files, err := GetSortedSegmentFiles(dir)
	if err != nil {
		t.Fatalf("GetSortedSegmentFiles() error: %v", err)
	}

	expected := []string{"rebuf-0", "rebuf.tmp"}
	if len(files) != len(expected) {
		t.Fatalf("expected %d files, got %d: %v", len(expected), len(files), files)
	}
}

// --- GetLatestSegmentId tests ---

func TestGetLatestSegmentId(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "rebuf-0"))
	createFile(t, filepath.Join(dir, "rebuf-3"))
	createFile(t, filepath.Join(dir, "rebuf-1"))
	createFile(t, filepath.Join(dir, "rebuf.tmp"))

	id, err := GetLatestSegmentId(dir)
	if err != nil {
		t.Fatalf("GetLatestSegmentId() error: %v", err)
	}
	if id != 3 {
		t.Errorf("expected 3, got %d", id)
	}
}

func TestGetLatestSegmentId_NoSegments(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "rebuf.tmp"))

	_, err := GetLatestSegmentId(dir)
	if err == nil {
		t.Error("expected error when no segments exist")
	}
}

func TestGetLatestSegmentId_SingleSegment(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "rebuf-5"))

	id, err := GetLatestSegmentId(dir)
	if err != nil {
		t.Fatalf("GetLatestSegmentId() error: %v", err)
	}
	if id != 5 {
		t.Errorf("expected 5, got %d", id)
	}
}

// --- GetNumSegments tests ---

func TestGetNumSegments(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "rebuf-0"))
	createFile(t, filepath.Join(dir, "rebuf-1"))
	createFile(t, filepath.Join(dir, "rebuf-2"))
	createFile(t, filepath.Join(dir, "rebuf.tmp"))

	count, err := GetNumSegments(dir)
	if err != nil {
		t.Fatalf("GetNumSegments() error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 segments, got %d", count)
	}
}

func TestGetNumSegments_Empty(t *testing.T) {
	dir := t.TempDir()

	count, err := GetNumSegments(dir)
	if err != nil {
		t.Fatalf("GetNumSegments() error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 segments, got %d", count)
	}
}

func TestGetNumSegments_OnlyTmp(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "rebuf.tmp"))

	count, err := GetNumSegments(dir)
	if err != nil {
		t.Fatalf("GetNumSegments() error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 segments with only .tmp, got %d", count)
	}
}

func TestGetNumSegments_IgnoresNonRebufFiles(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "rebuf-0"))
	createFile(t, filepath.Join(dir, ".DS_Store"))
	createFile(t, filepath.Join(dir, "random.txt"))

	count, err := GetNumSegments(dir)
	if err != nil {
		t.Fatalf("GetNumSegments() error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 segment, got %d", count)
	}
}

// --- GetOldestSegmentFile tests ---

func TestGetOldestSegmentFile(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "rebuf-5"))
	createFile(t, filepath.Join(dir, "rebuf-2"))
	createFile(t, filepath.Join(dir, "rebuf-8"))
	createFile(t, filepath.Join(dir, "rebuf.tmp"))

	oldest, err := GetOldestSegmentFile(dir)
	if err != nil {
		t.Fatalf("GetOldestSegmentFile() error: %v", err)
	}
	if oldest != "rebuf-2" {
		t.Errorf("expected rebuf-2, got %s", oldest)
	}
}

func TestGetOldestSegmentFile_NoSegments(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "rebuf.tmp"))

	_, err := GetOldestSegmentFile(dir)
	if err == nil {
		t.Error("expected error when no segments exist")
	}
}

func TestGetOldestSegmentFile_SingleSegment(t *testing.T) {
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "rebuf-7"))

	oldest, err := GetOldestSegmentFile(dir)
	if err != nil {
		t.Fatalf("GetOldestSegmentFile() error: %v", err)
	}
	if oldest != "rebuf-7" {
		t.Errorf("expected rebuf-7, got %s", oldest)
	}
}

// --- FileSize tests ---

func TestFileSize_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	f := createFile(t, filepath.Join(dir, "empty"))

	size, err := FileSize(f)
	if err != nil {
		t.Fatalf("FileSize() error: %v", err)
	}
	if size != 0 {
		t.Errorf("expected 0, got %d", size)
	}
	f.Close()
}

func TestFileSize_WithContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data")

	if err := os.WriteFile(path, []byte("hello world"), 0666); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	size, err := FileSize(f)
	if err != nil {
		t.Fatalf("FileSize() error: %v", err)
	}
	if size != 11 {
		t.Errorf("expected 11, got %d", size)
	}
}

// --- helpers ---

func createFile(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		t.Fatalf("createFile(%s) error: %v", path, err)
	}
	return f
}

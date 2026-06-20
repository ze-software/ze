package auth

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// VALIDATES: SeqPersister.Store followed by Close writes the latest
// value to disk, and a fresh persister loads it via Start().
// PREVENTS: regression where the coalescing writer drops the pending
// flush on Close.
func TestSeqPersistWriteLoad(t *testing.T) {
	dir := t.TempDir()
	p, err := NewSeqPersister(dir, "sessionA")
	if err != nil {
		t.Fatalf("NewSeqPersister: %v", err)
	}
	if p.Start() != 0 {
		t.Fatalf("first-run Start = %d, want 0", p.Start())
	}
	p.Store(42)
	// The writer goroutine coalesces; Close flushes before exit.
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify the file exists and reads back 42.
	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Fatal("no file written")
	}

	p2, err := NewSeqPersister(dir, "sessionA")
	if err != nil {
		t.Fatalf("second NewSeqPersister: %v", err)
	}
	defer func() { _ = p2.Close() }()
	if got := p2.Start(); got != 42 {
		t.Fatalf("reload Start = %d, want 42", got)
	}
}

// VALIDATES: SeqPersister records a write failure via the logged
// latch and does not block the express loop when the directory is
// read-only.
// PREVENTS: a disk-full or RO filesystem wedging the hot path.
func TestSeqPersistWriteFailure(t *testing.T) {
	dir := t.TempDir()
	p, err := newTestSeqPersister(dir, "sessionRO", 5*time.Millisecond, func(_ string, _ uint32) error {
		return os.ErrPermission
	})
	if err != nil {
		t.Fatalf("newTestSeqPersister: %v", err)
	}
	p.Store(1)
	// Give the ticker at least one cycle to process.
	time.Sleep(25 * time.Millisecond)
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !p.logged.Load() {
		t.Fatal("logged latch not set after forced write failure")
	}
}

// VALIDATES: NewSeqPersister rejects a relative directory.
// PREVENTS: accidental creation of a sequence file under the CWD of
// whatever process happens to run ze.
func TestSeqPersistRelativeDirRejected(t *testing.T) {
	if _, err := NewSeqPersister("relative", "x"); err == nil {
		t.Fatal("NewSeqPersister with relative dir returned nil")
	}
}

// VALIDATES: sanitizeSessionKey strips non-alphanumerics.
// PREVENTS: directory traversal or weird filenames.
func TestSanitizeSessionKey(t *testing.T) {
	cases := map[string]string{
		"203.0.113.9-default-single-hop": "203_0_113_9_default_single_hop",
		"":                               "session",
		"/etc/passwd":                    "_etc_passwd",
	}
	for in, want := range cases {
		if got := sanitizeSessionKey(in); got != want {
			t.Errorf("sanitize %q = %q, want %q", in, got, want)
		}
	}
}

// VALIDATES: readSeqFile round-trip with writeSeqFile.
// PREVENTS: decimal/hex mismatch between the two halves of persistence.
func TestReadWriteSeqFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.seq")
	if err := writeSeqFile(path, 12345); err != nil {
		t.Fatalf("writeSeqFile: %v", err)
	}
	got, err := readSeqFile(path)
	if err != nil {
		t.Fatalf("readSeqFile: %v", err)
	}
	if got != 12345 {
		t.Fatalf("readSeqFile = %d, want 12345", got)
	}
	// And a sanity check that the file is the decimal string we
	// expect, not some other encoding.
	raw, _ := os.ReadFile(path)
	if string(raw) != strconv.FormatUint(12345, 10) {
		t.Fatalf("file content = %q, want %q", raw, "12345")
	}
}

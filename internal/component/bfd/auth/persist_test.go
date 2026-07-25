package auth

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

// newStore creates an empty database.zefs and registers it as the
// process-wide state store so the sequence persistence round-trips through
// the real zefs store (not a loose file), mirroring
// internal/plugins/ddos/detect/persist_test.go. The store is unregistered
// and closed on cleanup.
func newStore(t *testing.T) {
	t.Helper()
	bs, err := zefs.Create(filepath.Join(t.TempDir(), "database.zefs"))
	if err != nil {
		t.Fatalf("zefs.Create: %v", err)
	}
	statestore.SetStore(bs)
	t.Cleanup(func() {
		statestore.SetStore(nil)
		if err := bs.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	})
}

// VALIDATES: SeqPersister.Store followed by Close writes the latest value
// into the shared zefs store under zefs.KeyBFDAuthSeq, and a fresh
// persister loads it via Start(). Also asserts the concrete per-session
// key mapping end to end.
// PREVENTS: regression where the coalescing writer drops the pending
// flush on Close, or the session key is stored under the wrong zefs key.
func TestSeqPersistWriteLoad(t *testing.T) {
	newStore(t)
	const sessionKey = "203.0.113.9-default-single-hop"

	p, err := newSeqPersisterAt(sessionKey, DefaultFlushInterval, writeSeq)
	if err != nil {
		t.Fatalf("newSeqPersisterAt: %v", err)
	}
	if p.Start() != 0 {
		t.Fatalf("first-run Start = %d, want 0", p.Start())
	}
	p.Store(42)
	// The writer goroutine coalesces; Close flushes before exit.
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The value must land under the registered per-session key.
	wantKey := zefs.KeyBFDAuthSeq.Key(sanitizeSessionKey(sessionKey))
	raw, ok := statestore.Get(wantKey)
	if !ok {
		t.Fatalf("no blob under %q after Close", wantKey)
	}
	if string(raw) != "42" {
		t.Fatalf("stored blob = %q, want %q", raw, "42")
	}

	// A fresh persister on the same store + key sees the stored value.
	p2, err := newSeqPersisterAt(sessionKey, DefaultFlushInterval, writeSeq)
	if err != nil {
		t.Fatalf("second newSeqPersisterAt: %v", err)
	}
	defer func() { _ = p2.Close() }()
	if got := p2.Start(); got != 42 {
		t.Fatalf("reload Start = %d, want 42", got)
	}
}

// VALIDATES: NewSeqPersister (the production constructor) is a best-effort
// no-op when no store exists: it never errors and starts from zero.
// PREVENTS: a missing database.zefs wedging session setup.
func TestSeqPersistNoStore(t *testing.T) {
	// No store registered (filesystem-fallback mode): writes no-op,
	// reads return not-found, construction still succeeds.
	statestore.SetStore(nil)
	p, err := newSeqPersisterAt("sessionA", 5*time.Millisecond, writeSeq)
	if err != nil {
		t.Fatalf("newSeqPersisterAt (absent store): %v", err)
	}
	if p.Start() != 0 {
		t.Fatalf("Start = %d, want 0 for absent store", p.Start())
	}
	p.Store(7)
	time.Sleep(25 * time.Millisecond)
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// A best-effort no-op write is not a failure.
	if p.logged.Load() {
		t.Fatal("logged latch set for an absent store (should be a silent no-op)")
	}
}

// VALIDATES: SeqPersister records a write failure via the logged latch and
// does not block the express loop when the store write fails.
// PREVENTS: a disk-full or read-only store wedging the hot path.
func TestSeqPersistWriteFailure(t *testing.T) {
	newStore(t)
	p, err := newSeqPersisterAt("sessionRO", 5*time.Millisecond, func(_ string, _ uint32) error {
		return errors.New("forced write failure")
	})
	if err != nil {
		t.Fatalf("newSeqPersisterAt: %v", err)
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

// VALIDATES: an unsafe session-key segment is rejected rather than reaching
// zefs.KeyEntry.Key (which would panic on "/" or "..").
// PREVENTS: a sanitizer regression from writing a traversal segment into
// the shared store.
func TestSeqPersistUnsafeSegmentRejected(t *testing.T) {
	if safeSegment("") {
		t.Error("empty segment must be rejected")
	}
	if safeSegment("a/b") {
		t.Error("segment with '/' must be rejected")
	}
	if safeSegment("..") {
		t.Error("segment with '..' must be rejected")
	}
	if !safeSegment("203_0_113_9_default_single_hop") {
		t.Error("a sanitized slug must be accepted")
	}
}

// VALIDATES: sanitizeSessionKey maps everything outside [0-9A-Za-z] to '_'
// so the derived zefs key segment is always safe.
// PREVENTS: directory traversal or path separators leaking into a key.
func TestSanitizeSessionKey(t *testing.T) {
	cases := map[string]string{
		"203.0.113.9-default-single-hop": "203_0_113_9_default_single_hop",
		"":                               "session",
		"/etc/passwd":                    "_etc_passwd",
		"../../escape":                   "______escape",
	}
	for in, want := range cases {
		if got := sanitizeSessionKey(in); got != want {
			t.Errorf("sanitize %q = %q, want %q", in, got, want)
		}
		// Whatever the input, the result is a safe zefs segment.
		if !safeSegment(sanitizeSessionKey(in)) {
			t.Errorf("sanitize %q produced an unsafe segment %q", in, sanitizeSessionKey(in))
		}
	}
}

// VALIDATES: writeSeq/readSeq round-trip a value through the real store as
// the decimal string the two halves agree on.
// PREVENTS: a decimal/hex mismatch between the write and read paths.
func TestWriteReadSeqRoundTrip(t *testing.T) {
	newStore(t)
	key := zefs.KeyBFDAuthSeq.Key("session")
	if err := writeSeq(key, 12345); err != nil {
		t.Fatalf("writeSeq: %v", err)
	}
	got, ok := readSeq(key)
	if !ok {
		t.Fatal("readSeq returned ok=false for a freshly written value")
	}
	if got != 12345 {
		t.Fatalf("readSeq = %d, want 12345", got)
	}
	// And a sanity check that the blob is the decimal string we expect.
	raw, ok := statestore.Get(key)
	if !ok || string(raw) != "12345" {
		t.Fatalf("stored blob = %q (ok=%v), want %q", raw, ok, "12345")
	}
}

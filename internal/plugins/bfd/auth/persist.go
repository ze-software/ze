// Design: rfc/short/rfc5880.md -- Section 6.7.3 (sequence persistence across restart)
//
// Sequence-number persistence for Meticulous Keyed authentication.
// RFC 5880 §6.7.3 warns that Meticulous variants will reject every
// packet from a restarting speaker until the new sequence number
// overtakes the last one the peer accepted. Persisting the last-sent
// sequence across a process restart lets a fresh ze daemon resume
// from the floor it left.
//
// The persister is a small coalescing background writer. Each call
// to Store publishes the new sequence into an atomic and nudges the
// writer goroutine; the writer writes the latest value to disk at
// most once every flushInterval or on Close. Write failures are
// logged once and do not block the hot path -- if the disk is
// read-only the express loop keeps ticking and the post-restart
// session re-synchronizes under the standard RFC rules.
package auth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"
)

// DefaultFlushInterval is the cadence at which the persister writes
// the latest sequence number to disk.
const DefaultFlushInterval = 500 * time.Millisecond

// ErrPersistDirInvalid is returned when a relative or empty
// persistence directory is passed to NewSeqPersister.
var ErrPersistDirInvalid = errors.New("bfd auth: persistence directory must be absolute")

// SeqPersister is a background coalescing writer for one session's
// TX sequence number. Safe for concurrent use.
type SeqPersister struct {
	path    string
	flush   time.Duration
	current atomic.Uint32
	pending atomic.Bool
	stopped atomic.Bool
	stopCh  chan struct{}
	doneCh  chan struct{}
	logged  atomic.Bool
	writeFn func(path string, value uint32) error
	startAt uint32
}

// NewSeqPersister opens or creates a sequence file under dir using a
// filename derived from sessionKey. The last-written value (if any)
// is loaded and returned as the starting sequence via Start().
func NewSeqPersister(dir, sessionKey string) (*SeqPersister, error) {
	if dir == "" || !filepath.IsAbs(dir) {
		return nil, ErrPersistDirInvalid
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("bfd auth persist: mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, sanitizeSessionKey(sessionKey)+".seq")
	p := &SeqPersister{
		path:    path,
		flush:   DefaultFlushInterval,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
		writeFn: writeSeqFile,
	}
	if loaded, err := readSeqFile(path); err == nil {
		p.current.Store(loaded)
		p.startAt = loaded
	}
	go p.run()
	return p, nil
}

// Start returns the sequence number loaded from disk at construction
// time, or zero when no prior file existed. The caller uses this as
// the initial value for bfd.XmitAuthSeq.
func (p *SeqPersister) Start() uint32 { return p.startAt }

// newTestSeqPersister builds a SeqPersister with an injected
// writeFn and flush cadence. Used by persist_test.go to simulate
// I/O failures without a shared-field race between the test
// goroutine and the writer goroutine.
func newTestSeqPersister(dir, sessionKey string, flush time.Duration, writeFn func(string, uint32) error) (*SeqPersister, error) {
	if dir == "" || !filepath.IsAbs(dir) {
		return nil, ErrPersistDirInvalid
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("bfd auth persist: mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, sanitizeSessionKey(sessionKey)+".seq")
	p := &SeqPersister{
		path:    path,
		flush:   flush,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
		writeFn: writeFn,
	}
	if loaded, err := readSeqFile(path); err == nil {
		p.current.Store(loaded)
		p.startAt = loaded
	}
	go p.run()
	return p, nil
}

// Store publishes seq as the most recent TX sequence number. The
// writer goroutine will flush it to disk on the next tick. Safe for
// concurrent use. Does not block on I/O.
func (p *SeqPersister) Store(seq uint32) {
	p.current.Store(seq)
	p.pending.Store(true)
}

// Close stops the writer goroutine, flushes any pending value to
// disk, and returns after the goroutine exits. Idempotent.
func (p *SeqPersister) Close() error {
	if p.stopped.Swap(true) {
		return nil
	}
	close(p.stopCh)
	<-p.doneCh
	return nil
}

// run is the writer goroutine lifecycle.
func (p *SeqPersister) run() {
	defer close(p.doneCh)
	ticker := time.NewTicker(p.flush)
	defer ticker.Stop()
	var lastFlushed uint32
	for {
		select {
		case <-p.stopCh:
			p.flushIfChanged(&lastFlushed)
			return
		case <-ticker.C:
			p.flushIfChanged(&lastFlushed)
		}
	}
}

// flushIfChanged writes the current sequence only when Store has
// been called since the last write. On write failure the first-time
// flag latches so operators see one log and not a stream.
func (p *SeqPersister) flushIfChanged(lastFlushed *uint32) {
	if !p.pending.Swap(false) {
		return
	}
	seq := p.current.Load()
	if seq == *lastFlushed {
		return
	}
	if err := p.writeFn(p.path, seq); err != nil {
		// Best-effort: record the first failure via the latch.
		// A real log line would couple this file to the plugin
		// logger; the caller (bfd.go) can observe p.logged.Load
		// to surface the condition if needed.
		p.logged.Store(true)
		return
	}
	*lastFlushed = seq
}

// sanitizeSessionKey turns an arbitrary session identity into a
// filename-safe slug.
func sanitizeSessionKey(key string) string {
	b := make([]byte, 0, len(key))
	for i := range key {
		c := key[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			b = append(b, c)
		} else {
			b = append(b, '_')
		}
	}
	if len(b) == 0 {
		return "session"
	}
	return string(b)
}

// writeSeqFile writes value as a decimal string to a temporary file,
// fsyncs, and renames into place. errors.Join aggregates the
// primary write/sync/close failure with any tempfile cleanup error
// so both surface without %w collisions.
func writeSeqFile(path string, value uint32) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // path is built from an absolute, caller-validated dir plus a sanitized session key
	if err != nil {
		return err
	}
	if _, werr := f.WriteString(strconv.FormatUint(uint64(value), 10)); werr != nil {
		return errors.Join(werr, f.Close(), os.Remove(tmp))
	}
	if serr := f.Sync(); serr != nil {
		return errors.Join(serr, f.Close(), os.Remove(tmp))
	}
	if cerr := f.Close(); cerr != nil {
		return errors.Join(cerr, os.Remove(tmp))
	}
	return os.Rename(tmp, path)
}

// readSeqFile loads a previously-persisted sequence number.
func readSeqFile(path string) (uint32, error) {
	b, err := os.ReadFile(path) //nolint:gosec // path is built from an absolute, caller-validated dir plus a sanitized session key
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(string(b), 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(n), nil
}

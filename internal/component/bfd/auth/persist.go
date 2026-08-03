// RFC: rfc/short/rfc5880.md -- Section 6.7.3 (sequence persistence across restart)
// Design: ai/rules/architecture.md -- runtime state lives in the managed zefs
// store (<config-dir>/database.zefs), never as loose files.
//
// Sequence-number persistence for Meticulous Keyed authentication.
// RFC 5880 §6.7.3 warns that Meticulous variants will reject every
// packet from a restarting speaker until the new sequence number
// overtakes the last one the peer accepted. Persisting the last-sent
// sequence across a process restart lets a fresh ze daemon resume
// from the floor it left.
//
// The value is stored through internal/core/statestore under the
// registered per-session key zefs.KeyBFDAuthSeq, so the sequence lives
// inside the shared, integrity-checked database.zefs alongside the
// other appliance runtime state rather than in a loose <session>.seq
// file. Writes are best-effort: statestore is a no-op when the store
// does not exist yet, mirroring the pre-migration "read-only disk"
// behavior where the express loop keeps ticking and the post-restart
// session re-synchronizes under the standard RFC rules.
//
// The persister is a small coalescing background writer. Each call
// to Store publishes the new sequence into an atomic and nudges the
// writer goroutine; the writer writes the latest value to the store
// at most once every flushInterval or on Close. Write failures are
// latched once and do not block the hot path.
package auth

import (
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// DefaultFlushInterval is the cadence at which the persister writes
// the latest sequence number to the store.
const DefaultFlushInterval = 500 * time.Millisecond

// ErrSessionKeyInvalid is returned when a session key does not sanitize
// to a safe zefs key segment (empty, or containing "/" or ".."). In
// practice sanitizeSessionKey guarantees a safe slug; this guards
// against a future sanitizer regression producing a traversal segment.
var ErrSessionKeyInvalid = errors.New("bfd auth: session key is not a safe zefs segment")

// SeqPersister is a background coalescing writer for one session's
// TX sequence number. Safe for concurrent use.
type SeqPersister struct {
	key     string // full zefs key: meta/bfd/auth/<sanitized-session>
	flush   time.Duration
	current atomic.Uint32
	pending atomic.Bool
	stopped atomic.Bool
	stopCh  chan struct{}
	doneCh  chan struct{}
	logged  atomic.Bool
	writeFn func(key string, value uint32) error
	startAt uint32
}

// NewSeqPersister builds a persister for sessionKey backed by the
// process-wide shared zefs store (internal/core/statestore). The
// last-written value (if any) is loaded and returned as the starting
// sequence via Start().
func NewSeqPersister(sessionKey string) (*SeqPersister, error) {
	return newSeqPersisterAt(sessionKey, DefaultFlushInterval, writeSeq)
}

// Start returns the sequence number loaded from the store at construction
// time, or zero when no prior value existed. The caller uses this as the
// initial value for bfd.XmitAuthSeq.
func (p *SeqPersister) Start() uint32 { return p.startAt }

// newSeqPersisterAt builds a SeqPersister with an injected writeFn and
// flush cadence. Production uses NewSeqPersister (writeFn = writeSeq,
// routing to the process-wide statestore); tests register a real
// database.zefs via statestore.SetStore to round-trip through the store,
// or inject a stub writeFn to simulate I/O failures without a shared-field
// race between the test goroutine and the writer goroutine.
func newSeqPersisterAt(sessionKey string, flush time.Duration, writeFn func(key string, value uint32) error) (*SeqPersister, error) {
	seg := sanitizeSessionKey(sessionKey)
	if !safeSegment(seg) {
		return nil, ErrSessionKeyInvalid
	}
	key := zefs.KeyBFDAuthSeq.Key(seg)
	p := &SeqPersister{
		key:     key,
		flush:   flush,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
		writeFn: writeFn,
	}
	if loaded, ok := readSeq(key); ok {
		p.current.Store(loaded)
		p.startAt = loaded
	}
	go p.run()
	return p, nil
}

// Store publishes seq as the most recent TX sequence number. The
// writer goroutine will flush it to the store on the next tick. Safe
// for concurrent use. Does not block on I/O.
func (p *SeqPersister) Store(seq uint32) {
	p.current.Store(seq)
	p.pending.Store(true)
}

// Close stops the writer goroutine, flushes any pending value to the
// store, and returns after the goroutine exits. Idempotent.
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
// flag latches so operators see one log and not a stream. A best-effort
// no-op write (absent store) reports no error and is treated as flushed.
func (p *SeqPersister) flushIfChanged(lastFlushed *uint32) {
	if !p.pending.Swap(false) {
		return
	}
	seq := p.current.Load()
	if seq == *lastFlushed {
		return
	}
	if err := p.writeFn(p.key, seq); err != nil {
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
// key-safe slug (alphanumerics preserved, everything else mapped to
// '_'), so the derived zefs key segment can never carry a path
// separator or traversal component.
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

// safeSegment reports whether seg is usable as a zefs key segment: it
// must be non-empty and free of "/" and ".." so zefs.KeyEntry.Key never
// panics and no traversal segment reaches the store.
func safeSegment(seg string) bool {
	if seg == "" {
		return false
	}
	if strings.Contains(seg, "/") || strings.Contains(seg, "..") {
		return false
	}
	return true
}

// writeSeq persists value as a decimal string under key in the process-wide
// shared zefs store. Best-effort: statestore.Put is a no-op (returns
// false, nil) when no store is registered, and surfaces real write failures
// as a non-nil error for the caller's latch.
func writeSeq(key string, value uint32) error {
	_, err := statestore.Put(key, []byte(textbuf.StringUint(uint64(value))))
	return err
}

// readSeq loads a previously-persisted sequence number from the store.
// ok is false when the store or key is absent or the blob is malformed.
func readSeq(key string) (uint32, bool) {
	data, ok := statestore.Get(key)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseUint(string(data), 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}

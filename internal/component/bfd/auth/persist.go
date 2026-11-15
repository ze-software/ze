// RFC: rfc/short/rfc5880.md -- Section 6.7.3 (sequence persistence across restart)
// Design: ai/rules/architecture.md -- runtime state lives in the managed
// store (<config-dir>/database/), never as loose files.
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
// inside the shared, integrity-checked database/ tree alongside the
// other appliance runtime state rather than in a loose <session>.seq
// file. Writes are best-effort: statestore is a no-op when the store
// does not exist yet, mirroring the pre-migration "read-only disk"
// behavior where the express loop keeps ticking and the post-restart
// session re-synchronizes under the standard RFC rules.
//
// The persister is a small coalescing background writer. Each call
// to Store publishes the new sequence into an atomic and nudges the
// writer goroutine; the writer writes the latest value to the store
// at most once every flushInterval or on Close. A failed write never
// blocks the hot path: the first failure of each run of failed writes is
// handed to the owner's onWriteFailure callback, the failure stays latched
// until a write lands, and Close answers it when the last write failed.
package auth

import (
	"errors"
	"fmt"
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
	// failure is the error of the last write, nil while the last write
	// landed or none was attempted. The writer goroutine sets and clears it;
	// Close reads it after the writer exits.
	failure atomic.Pointer[error]
	writeFn func(key string, value uint32) error
	// onWriteFailure is called on the writer goroutine with the first failed
	// write of each run of failed writes, nil when only Close reports one.
	onWriteFailure func(err error)
	startAt        uint32
}

// NewSeqPersister builds a persister for sessionKey backed by the
// process-wide shared zefs store (internal/core/statestore). The
// last-written value (if any) is loaded and returned as the starting
// sequence via Start(). onWriteFailure, when not nil, is called on the
// writer goroutine with the first failed write after a landed one (or the
// first ever), so a store that keeps refusing is reported once per run; it
// MUST NOT block and MUST NOT take a lock the caller of Close holds.
func NewSeqPersister(sessionKey string, onWriteFailure func(err error)) (*SeqPersister, error) {
	return newSeqPersisterAt(sessionKey, DefaultFlushInterval, writeSeq, onWriteFailure)
}

// Start returns the sequence number loaded from the store at construction
// time, or zero when no prior value existed. The caller uses this as the
// initial value for bfd.XmitAuthSeq.
func (p *SeqPersister) Start() uint32 { return p.startAt }

// newSeqPersisterAt builds a SeqPersister with an injected writeFn and
// flush cadence. Production uses NewSeqPersister (writeFn = writeSeq,
// routing to the process-wide statestore); tests register a real
// store via statestore.SetStore to round-trip through it,
// or inject a stub writeFn to simulate I/O failures without a shared-field
// race between the test goroutine and the writer goroutine.
func newSeqPersisterAt(sessionKey string, flush time.Duration, writeFn func(key string, value uint32) error,
	onWriteFailure func(err error),
) (*SeqPersister, error) {
	seg := sanitizeSessionKey(sessionKey)
	if !safeSegment(seg) {
		return nil, ErrSessionKeyInvalid
	}
	key := zefs.KeyBFDAuthSeq.Key(seg)
	p := &SeqPersister{
		key:            key,
		flush:          flush,
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
		writeFn:        writeFn,
		onWriteFailure: onWriteFailure,
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
// store, and returns after the goroutine exits. It answers the failure of
// the last write when that write failed, so the stored sequence number is
// older than the last one sent; nil when the last write landed, when
// nothing was written, or on a second call. Idempotent.
func (p *SeqPersister) Close() error {
	if p.stopped.Swap(true) {
		return nil
	}
	close(p.stopCh)
	<-p.doneCh
	if failure := p.failure.Load(); failure != nil {
		return fmt.Errorf("bfd auth: the last TX sequence number %d is not saved: %w", p.current.Load(), *failure)
	}
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
// been called since the last write. A failed write is latched in failure
// until a write lands, and the first failure of each run is handed to
// onWriteFailure, so a store that keeps refusing is reported once and not on
// every tick. A best-effort no-op write (absent store) reports no error and
// is treated as flushed.
func (p *SeqPersister) flushIfChanged(lastFlushed *uint32) {
	if !p.pending.Swap(false) {
		return
	}
	seq := p.current.Load()
	if seq == *lastFlushed {
		return
	}
	if err := p.writeFn(p.key, seq); err != nil {
		if p.failure.Swap(&err) == nil && p.onWriteFailure != nil {
			p.onWriteFailure(err)
		}
		return
	}
	p.failure.Store(nil)
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
// as a non-nil error, which flushIfChanged latches and reports.
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

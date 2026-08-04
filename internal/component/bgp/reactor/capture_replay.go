// Design: plan/spec-improve-3-event-replay.md -- per-session JSONL protocol event capture
// Related: session_read.go -- the standard read path's tee point
// Related: session_coalesce.go -- the coalesced read path's tee point
// Overview: capture.go -- the unrelated in-memory diagnostic ring of the same name

package reactor

import (
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/capture"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/core/version"
)

// CaptureLimitAction is what a capture does when its file reaches the size cap.
type CaptureLimitAction uint8

const (
	// CaptureLimitRotate renames the full file aside once and starts a fresh
	// one. Disk use per peer is bounded at twice the cap.
	CaptureLimitRotate CaptureLimitAction = iota
	// CaptureLimitStop closes the file and captures nothing further. Disk use
	// per peer is bounded at the cap.
	CaptureLimitStop
)

// CaptureSettings is the per-peer protocol event capture configuration, parsed
// from the peer's YANG `capture` container.
type CaptureSettings struct {
	// Directory holds the capture files. Created if absent.
	Directory string
	// Enabled turns the capture on for this peer. Off by default: a disabled
	// capture costs one nil check per received message and nothing else.
	Enabled bool
	// MaximumSize is the per-file cap in megabytes (YANG range 1..1024).
	MaximumSize uint32
	// OnLimit is what happens when a file reaches MaximumSize.
	OnLimit CaptureLimitAction
}

const (
	// captureQueueDepth is the hand-off queue between the session read
	// goroutine and the capture writer goroutine. The read goroutine never
	// blocks on it: a full queue sheds the event and counts it, because a
	// stalled BGP read loop is a far worse outcome than a gap in a debug
	// capture, and the gap is recorded in the stream so replay knows.
	captureQueueDepth = 1024

	// captureTailReserve is the byte budget held back from the file cap so the
	// final drops tally and capture-stop line always fit. Without it a file
	// that fills exactly would have to end without its terminator.
	captureTailReserve = 256

	// rotatedSuffix names the one rotated-aside generation.
	rotatedSuffix = ".1"

	// captureFilePerm and captureDirPerm keep a capture readable only by the
	// daemon user and its group: the file holds a peer's full routing data.
	captureFilePerm = 0o640
	captureDirPerm  = 0o750

	// captureLogSubsystem is the slog subsystem of every capture log line.
	captureLogSubsystem = "bgp.capture"

	// DefaultCaptureDirectory and DefaultCaptureMaximumSize MUST match the
	// YANG defaults of the peer's `capture` container in ze-bgp-conf.yang.
	DefaultCaptureDirectory = "/var/lib/ze/capture"
	// DefaultCaptureMaximumSize is the per-file cap in megabytes.
	DefaultCaptureMaximumSize = 100

	// MinimumCaptureSize and MaximumCaptureSize bound the YANG leaf's range.
	// They exist so config parsing rejects the same values YANG rejects, for
	// callers that build settings without going through the schema.
	MinimumCaptureSize = 1
	MaximumCaptureSize = 1024
)

// ErrCaptureDirectory means the configured capture directory or file cannot
// be created.
var ErrCaptureDirectory = errors.New("bgp capture directory is not usable")

// captureKind discriminates a queued item without a type switch.
type captureKind uint8

const (
	captureKindMessage captureKind = iota
	captureKindConfig
	captureKindSession
)

// captureItem is one queued event. It is pooled and reused: the hot path fills
// one, hands it over, and the writer goroutine returns it.
type captureItem struct {
	ts       time.Time
	data     []byte
	op       string
	txID     string
	event    string
	sourceID uint32
	ctxID    uint16
	msgType  uint8
	kind     captureKind
}

// sessionCapture writes one peer's protocol event capture to a bounded JSONL
// file.
//
// # Threading
//
// recordMessage, recordConfig and recordSession run on the session read
// goroutine. They never block, never do I/O, and never allocate once the item
// pool is warm. One long-lived writer goroutine owns the file, the encoder, and
// rotation. Close drains the queue and ends the file.
//
// # The bound
//
// Each file is capped at MaximumSize megabytes exactly: the encoder refuses a
// line that would cross the cap rather than writing part of it. Under
// CaptureLimitRotate a peer therefore uses at most two files and 2 x MaximumSize
// megabytes; under CaptureLimitStop, one file and MaximumSize megabytes. The
// queue is bounded at captureQueueDepth events, and overflow is shed rather than
// buffered, with the count written into the stream.
type sessionCapture struct {
	clock  clock.Clock
	onDrop func()

	ch       chan *captureItem
	pool     sync.Pool
	finished chan struct{}

	// sendMu guards the channel against a send racing its close. RLock is the
	// producer side, Lock the one-time close.
	sendMu sync.RWMutex
	closed bool

	drops atomic.Uint64

	path      string
	limit     int64
	onLimit   CaptureLimitAction
	header    capture.Header
	startOnce sync.Once

	// Writer-goroutine-owned once Start has run.
	file        *os.File
	enc         *capture.Writer
	stopped     bool
	dropsMarked uint64
}

// captureFileName returns a path-safe file name for a peer's capture. IPv6
// colons become underscores so the name is usable on every filesystem.
func captureFileName(peer netip.Addr) string {
	var b textbuf.Buffer
	name := b.Str("bgp-").Addr(peer).Str(".jsonl").String()
	return strings.ReplaceAll(name, ":", "_")
}

// capturePath is the file one peer's capture writes to. It is the one place the
// path is built, so the exclusivity check in startCapture and the file the
// writer opens can never disagree.
func capturePath(settings CaptureSettings, peer netip.Addr) string {
	return filepath.Join(settings.Directory, captureFileName(peer))
}

// newSessionCapture creates the directory, opens the file, and writes the header
// and the capture-start event. It does NOT start the writer goroutine; call
// Start for that.
//
// It fails closed: an unusable directory or file is an error the caller reports,
// never a silent no-op that leaves the operator waiting for a capture that will
// never appear.
func newSessionCapture(ps *PeerSettings, coalesce bool, c clock.Clock, onDrop func()) (*sessionCapture, error) {
	settings := ps.Capture
	if err := os.MkdirAll(settings.Directory, captureDirPerm); err != nil {
		return nil, errors.Join(ErrCaptureDirectory, err)
	}

	sc := &sessionCapture{
		clock:    c,
		onDrop:   onDrop,
		ch:       make(chan *captureItem, captureQueueDepth),
		finished: make(chan struct{}),
		path:     capturePath(settings, ps.Address),
		limit:    int64(settings.MaximumSize) << 20,
		onLimit:  settings.OnLimit,
		header: capture.Header{
			Peer:          ps.Address.String(),
			Started:       c.Now().UTC().Format(capture.TimeFormat),
			DaemonVersion: version.Short(),
			LocalAS:       ps.LocalAS,
			PeerAS:        ps.PeerAS,
			RouterID:      ps.RouterID,
			Coalesce:      coalesce,
		},
	}
	sc.pool.New = func() any { return &captureItem{data: make([]byte, 0, 4096)} }

	// Move a previous session's file aside rather than truncating it. A session
	// that fails is the one an operator wants, and the peer reconnects within
	// seconds: opening O_TRUNC over the same path would erase the evidence
	// before anyone read it. This uses the same one rotated-aside generation the
	// size cap uses, so the disk bound is unchanged.
	sc.rotateAside()

	if err := sc.openFile(); err != nil {
		return nil, err
	}
	// The start marker rides the encoder directly: the writer goroutine is not
	// running yet, and starting it before the file carries a header would let
	// the first drained event land in an unreadable file.
	if err := sc.enc.WriteSession(c.Now(), capture.SessionCaptureStart, 0); err != nil {
		sc.logWriteError("capture-start", err)
	}
	return sc, nil
}

// rotateAside renames the current capture file to the one rotated generation,
// replacing any earlier rotation. A missing file is the normal case and is not
// an error.
func (sc *sessionCapture) rotateAside() {
	if err := os.Rename(sc.path, sc.path+rotatedSuffix); err != nil && !os.IsNotExist(err) {
		slogutil.LazyLogger(captureLogSubsystem)().Warn("capture file could not be moved aside; it will be overwritten",
			"path", sc.path, "error", err)
	}
}

// openFile opens the capture path fresh and writes the format header. The
// encoder's bound is the file cap less the tail reserve, so the closing lines
// always fit.
func (sc *sessionCapture) openFile() error {
	f, err := os.OpenFile(sc.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, captureFilePerm)
	if err != nil {
		return errors.Join(ErrCaptureDirectory, err)
	}
	sc.file = f
	sc.enc = capture.NewWriter(f, sc.limit-captureTailReserve)
	headerErr := sc.enc.WriteHeader(sc.header)
	if headerErr == nil {
		return nil
	}
	log := slogutil.LazyLogger(captureLogSubsystem)
	if closeErr := f.Close(); closeErr != nil {
		log().Warn("closing a capture file whose header failed", "path", sc.path, "error", closeErr)
	}
	// Leave no empty file behind: an operator who finds one reads it as a
	// capture that recorded nothing, not as a capture that never opened.
	if rmErr := os.Remove(sc.path); rmErr != nil {
		log().Warn("removing a capture file whose header failed", "path", sc.path, "error", rmErr)
	}
	sc.file = nil
	sc.enc = nil
	return headerErr
}

// Start launches the writer goroutine. One long-lived worker per capture
// lifetime, never a goroutine per event.
func (sc *sessionCapture) Start() {
	sc.startOnce.Do(func() { go sc.run() })
}

// Path is the capture file this session writes.
func (sc *sessionCapture) Path() string { return sc.path }

// Drops is the cumulative number of events shed because the queue was full.
func (sc *sessionCapture) Drops() uint64 { return sc.drops.Load() }

// recordMessage queues one complete inbound wire message. Hot path: no I/O, no
// blocking, and no allocation once the pool is warm.
func (sc *sessionCapture) recordMessage(ts time.Time, msgType uint8, wire []byte, sourceID uint32, ctxID uint16) {
	it := sc.get()
	it.kind = captureKindMessage
	it.ts = ts
	it.msgType = msgType
	it.sourceID = sourceID
	it.ctxID = ctxID
	it.data = append(it.data[:0], wire...)
	sc.offer(it)
}

// recordConfig queues one config operation. The payload is redacted here, on the
// caller's goroutine, so a secret never sits in the queue or reaches the file.
func (sc *sessionCapture) recordConfig(ts time.Time, op, txID string, payload []byte) {
	safe, err := capture.RedactPayload(payload)
	if err != nil {
		slogutil.LazyLogger(captureLogSubsystem)().Warn("config payload could not be parsed for redaction; recording a placeholder",
			"op", op, "tx-id", txID, "error", err)
	}
	it := sc.get()
	it.kind = captureKindConfig
	it.ts = ts
	it.op = op
	it.txID = txID
	it.data = append(it.data[:0], safe...)
	sc.offer(it)
}

// recordSession queues one session-lifecycle event.
func (sc *sessionCapture) recordSession(ts time.Time, event string) {
	it := sc.get()
	it.kind = captureKindSession
	it.ts = ts
	it.event = event
	sc.offer(it)
}

// get takes a pooled item and clears every field the caller can leave unset, so
// a stale value from a previous event can never be written under a new one.
func (sc *sessionCapture) get() *captureItem {
	it, ok := sc.pool.Get().(*captureItem)
	if !ok || it == nil {
		it = &captureItem{data: make([]byte, 0, 4096)}
	}
	it.op = ""
	it.txID = ""
	it.event = ""
	it.sourceID = 0
	it.ctxID = 0
	it.msgType = 0
	it.data = it.data[:0]
	return it
}

// offer hands an item to the writer, shedding it when the queue is full. This is
// the one place the capture is allowed to lose data, and it is deliberate: the
// BGP read loop must never wait on a debug feature. Every shed event is counted
// and the count is written into the stream, so a replay never mistakes a gap for
// a quiet peer.
func (sc *sessionCapture) offer(it *captureItem) {
	sc.sendMu.RLock()
	defer sc.sendMu.RUnlock()
	if sc.closed {
		sc.pool.Put(it)
		return
	}
	select {
	case sc.ch <- it:
	default:
		sc.pool.Put(it)
		sc.drops.Add(1)
		if sc.onDrop != nil {
			sc.onDrop()
		}
	}
}

// Close stops accepting events, drains what is queued, ends the file, and
// waits for the writer goroutine to exit. Safe to call more than once.
func (sc *sessionCapture) Close() {
	sc.sendMu.Lock()
	if sc.closed {
		sc.sendMu.Unlock()
		<-sc.finished
		return
	}
	sc.closed = true
	close(sc.ch)
	sc.sendMu.Unlock()

	// Start is not always called first (a capture that saw no traffic, or an
	// error path). Draining now is what leaves the file readable.
	sc.Start()
	<-sc.finished
}

// run is the writer goroutine: one long-lived worker over the hand-off queue.
func (sc *sessionCapture) run() {
	defer close(sc.finished)
	for it := range sc.ch {
		sc.write(it)
		sc.pool.Put(it)
	}
	sc.terminate()
}

// write encodes one item, marking any gap that opened since the last line.
func (sc *sessionCapture) write(it *captureItem) { sc.writeItem(it, false) }

// writeItem encodes one item. rotated says whether this item has already been
// refused once and rotated for; it is what stops the write-refuse-rotate cycle
// from repeating (see atLimit).
func (sc *sessionCapture) writeItem(it *captureItem, rotated bool) {
	if sc.stopped {
		return
	}
	sc.markDrops(it.ts)
	var err error
	switch it.kind {
	case captureKindMessage:
		err = sc.enc.WriteMessage(it.ts, capture.DirectionReceived, it.msgType, it.data, it.sourceID, it.ctxID)
	case captureKindConfig:
		err = sc.enc.WriteConfig(it.ts, it.op, it.txID, it.data)
	case captureKindSession:
		err = sc.enc.WriteSession(it.ts, it.event, 0)
	default:
		slogutil.LazyLogger(captureLogSubsystem)().Warn("unhandled capture item kind", "kind", uint8(it.kind))
		return
	}
	switch {
	case errors.Is(err, capture.ErrLimitReached):
		sc.atLimit(it, rotated)
	case err != nil:
		sc.logWriteError("event", err)
		sc.terminate()
	}
}

// markDrops writes a drops event when the shed counter moved since the last
// line, so a reader sees the gap where it happened rather than only at the end.
//
// The counter advances only once the line is on the file. A refused or failed
// drops line that still advanced it would leave the stream claiming there was no
// gap, which is the one thing this record exists to prevent.
func (sc *sessionCapture) markDrops(ts time.Time) {
	n := sc.drops.Load()
	if n == sc.dropsMarked {
		return
	}
	if err := sc.enc.WriteSession(ts, capture.SessionDrops, n); err != nil {
		sc.logWriteError("drops", err)
		return
	}
	sc.dropsMarked = n
}

// atLimit applies the configured limit policy, then retries the refused item on
// the fresh file when the policy is rotate. The item is never written twice: a
// refused write emitted nothing at all.
//
// rotated bounds the retry to ONE attempt. An item that an empty file also
// refuses cannot ever be written, and retrying would rotate again on every turn:
// each turn renames the live file over the one rotated generation, so the cycle
// destroys both files, makes no progress, and recurses until the stack ends the
// daemon. Stopping the capture loses this one capture; the alternative loses the
// process.
func (sc *sessionCapture) atLimit(it *captureItem, rotated bool) {
	log := slogutil.LazyLogger(captureLogSubsystem)
	if sc.onLimit == CaptureLimitStop {
		sc.terminate()
		return
	}
	if rotated {
		log().Warn("capture event does not fit an empty capture file; capture stopped",
			"path", sc.path, "maximum-size-bytes", sc.limit, "event-bytes", len(it.data))
		sc.terminate()
		return
	}
	sc.closeFile()
	if err := os.Rename(sc.path, sc.path+rotatedSuffix); err != nil {
		log().Warn("capture rotation failed; capture stopped", "path", sc.path, "error", err)
		sc.stopped = true
		return
	}
	if err := sc.openFile(); err != nil {
		log().Warn("capture could not reopen after rotation; capture stopped", "path", sc.path, "error", err)
		sc.stopped = true
		return
	}
	sc.writeItem(it, true)
}

// terminate ends the capture for good: the file gets its final marker and is
// closed, and every later event is discarded.
func (sc *sessionCapture) terminate() {
	if sc.stopped {
		return
	}
	sc.closeFile()
	sc.stopped = true
}

// closeFile writes the final drops tally and the terminating event, then closes.
// The encoder's bound is raised to the full file cap first: captureTailReserve
// was held back for exactly these two lines, so the file still never exceeds the
// configured maximum.
func (sc *sessionCapture) closeFile() {
	if sc.enc == nil || sc.file == nil {
		return
	}
	sc.enc.SetLimit(sc.limit)
	now := sc.clock.Now()
	sc.markDrops(now)
	if err := sc.enc.WriteSession(now, capture.SessionCaptureStop, sc.drops.Load()); err != nil {
		sc.logWriteError("capture-stop", err)
	}
	if err := sc.file.Close(); err != nil {
		sc.logWriteError("close", err)
	}
	sc.file = nil
	sc.enc = nil
}

// teeCapture records one complete inbound wire message, header included, into
// this peer's capture.
//
// It is called from BOTH read paths (session_read.go and session_coalesce.go) at
// the point where the body read has completed and before anything can consume or
// rewrite the buffer. That placement is the whole design: it sits ahead of RFC
// 7606 enforcement and ahead of coalescing, so a malformed UPDATE is captured as
// the peer sent it and a coalesced batch is captured as N separate messages
// rather than as the synthetic one the reactor builds.
//
// Cost when capture is off: one nil comparison, no allocation, no clock read.
func (s *Session) teeCapture(msgType uint8, wire []byte) {
	if s.captureWriter == nil {
		return
	}
	s.mu.RLock()
	ctxID := s.recvCtxID
	sourceID := s.sourceID
	s.mu.RUnlock()
	s.captureWriter.recordMessage(s.clock.Now(), msgType, wire, uint32(sourceID), uint16(ctxID))
}

// startCapture opens this peer's protocol event capture and attaches it to the
// session, or returns nil when the peer did not opt in.
//
// A capture that cannot be opened is reported and the session proceeds without
// one: a debug feature must never stop a BGP session from establishing. The
// operator hears about it through the log line and through `ze doctor`, which
// checks the directory (internal/component/doctor).
func (p *Peer) startCapture(session *Session) *sessionCapture {
	settings := p.settings.Capture
	if !settings.Enabled {
		return nil
	}
	// One capture per file. A second capture on the same path would rotate the
	// first one's live inode into the single `.1` slot and orphan it, so the
	// session that is actually running would lose its file.
	if p.reactor != nil && p.reactor.captureHeldForPath(capturePath(settings, p.settings.Address)) {
		slogutil.LazyLogger(captureLogSubsystem)().Warn("a capture is already open for this peer's file; this session is not captured",
			"peer", p.settings.Address.String(), "file", capturePath(settings, p.settings.Address))
		return nil
	}
	var onDrop func()
	if p.reactor != nil && p.reactor.rmetrics != nil {
		label := p.settings.Address.String()
		metric := p.reactor.rmetrics.captureDroppedEvents
		onDrop = func() { metric.With(label).Inc() }
	}
	sc, err := newSessionCapture(p.settings, session.coalesceEnabled, p.clock, onDrop)
	if err != nil {
		slogutil.LazyLogger(captureLogSubsystem)().Error("bgp protocol event capture could not start; the session continues without one",
			"peer", p.settings.Address.String(), "directory", settings.Directory, "error", err)
		return nil
	}
	sc.Start()
	sc.recordSession(p.clock.Now(), capture.SessionConnect)
	session.captureWriter = sc
	if p.reactor != nil {
		p.reactor.registerCapture(sc)
	}
	slogutil.LazyLogger(captureLogSubsystem)().Info("bgp protocol event capture started",
		"peer", p.settings.Address.String(), "file", sc.Path())
	return sc
}

// stopCapture stops a capture at the end of a session.
func (p *Peer) stopCapture(sc *sessionCapture) {
	if sc == nil {
		return
	}
	sc.recordSession(p.clock.Now(), capture.SessionDisconnect)
	if p.reactor != nil {
		p.reactor.unregisterCapture(sc)
	}
	sc.Close()
	slogutil.LazyLogger(captureLogSubsystem)().Info("bgp protocol event capture stopped",
		"peer", p.settings.Address.String(), "file", sc.Path(), "dropped-events", sc.Drops())
}

// registerCapture adds a live capture to the reactor's set, so config events
// reach it.
func (r *Reactor) registerCapture(sc *sessionCapture) {
	if r == nil || sc == nil {
		return
	}
	r.sessionCapturesMu.Lock()
	if r.sessionCaptures == nil {
		r.sessionCaptures = make(map[*sessionCapture]struct{})
	}
	r.sessionCaptures[sc] = struct{}{}
	r.sessionCapturesMu.Unlock()
}

// unregisterCapture removes a capture whose session has ended.
func (r *Reactor) unregisterCapture(sc *sessionCapture) {
	if r == nil || sc == nil {
		return
	}
	r.sessionCapturesMu.Lock()
	delete(r.sessionCaptures, sc)
	r.sessionCapturesMu.Unlock()
}

// captureHeldForPath reports whether a live capture already owns path.
//
// Two captures on one path destroy each other's files: each one's rotation
// renames the other's live inode to the single `.1` slot, so the session still
// running ends up writing to an unlinked file. The path comes from the peer
// ADDRESS, and Peer.Stop only cancels a context (peer.go), so a RemovePeer plus
// AddPeer on one address can overlap the old capture with the new one. That is
// the same race doRemovePeer already handles synchronously for the RFC 6286
// identifier claim.
func (r *Reactor) captureHeldForPath(path string) bool {
	if r == nil {
		return false
	}
	r.sessionCapturesMu.Lock()
	defer r.sessionCapturesMu.Unlock()
	for sc := range r.sessionCaptures {
		if sc.path == path {
			return true
		}
	}
	return false
}

// closeCapturesForPeer ends every live capture of one peer and waits for the
// files to be terminated. doRemovePeer calls it so a removal releases the
// capture path before the caller can add the address back, exactly as it
// releases the identifier claim rather than leaving it to the peer goroutine.
//
// Closing twice is safe: the session's own deferred stopCapture (peer_run.go)
// still runs and Close is idempotent.
func (r *Reactor) closeCapturesForPeer(addr netip.Addr) {
	if r == nil {
		return
	}
	want := addr.Unmap().String()
	r.sessionCapturesMu.Lock()
	var doomed []*sessionCapture
	for sc := range r.sessionCaptures {
		if sc.header.Peer == want {
			doomed = append(doomed, sc)
		}
	}
	for _, sc := range doomed {
		delete(r.sessionCaptures, sc)
	}
	r.sessionCapturesMu.Unlock()

	for _, sc := range doomed {
		sc.Close()
	}
}

// CapturesOpen reports whether any protocol event capture is open right now.
//
// It is the guard a caller takes BEFORE it builds a capture payload. Building
// one is not free: a reconcile payload is a json.Marshal of the WHOLE BGP config
// tree, which grows with the peer count, and a daemon that is not capturing must
// not pay it on every config reload. CaptureConfigEvent costs one mutex lock,
// but its ARGUMENT is what the caller must not compute.
func (r *Reactor) CapturesOpen() bool {
	if r == nil {
		return false
	}
	r.sessionCapturesMu.Lock()
	defer r.sessionCapturesMu.Unlock()
	return len(r.sessionCaptures) != 0
}

// CaptureConfigEvent records one config operation into every capture that is
// open right now. A config operation is peer-scoped, but its effect on the
// reactor is global, so a replay of any captured session needs to see it.
//
// The event records the operation as SUBMITTED, not as accepted: a verify that
// the reactor later rejects still appears, because a replay of the session needs
// to see what the operator attempted, and the outcome shows in the operations
// that follow it.
//
// It is a no-op when no capture is open, which is the normal case.
func (r *Reactor) CaptureConfigEvent(op, txID string, payload []byte) {
	if r == nil {
		return
	}
	r.sessionCapturesMu.Lock()
	if len(r.sessionCaptures) == 0 {
		r.sessionCapturesMu.Unlock()
		return
	}
	live := make([]*sessionCapture, 0, len(r.sessionCaptures))
	for sc := range r.sessionCaptures {
		live = append(live, sc)
	}
	r.sessionCapturesMu.Unlock()

	now := r.clock.Now()
	for _, sc := range live {
		sc.recordConfig(now, op, txID, payload)
	}
}

func (sc *sessionCapture) logWriteError(what string, err error) {
	slogutil.LazyLogger(captureLogSubsystem)().Warn("bgp capture write failed",
		"path", sc.path, "what", what, "error", err)
}

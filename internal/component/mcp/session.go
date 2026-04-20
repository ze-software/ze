// Design: docs/architecture/mcp/overview.md -- MCP session registry and SSE writer
// Related: streamable.go -- Streamable HTTP transport that creates/uses sessions

// Package mcp session management.
//
// A session is the stateful context created at `initialize` and referenced by
// the `Mcp-Session-Id` header on subsequent requests. It owns an outbound
// message queue that the SSE writer drains to push notifications and
// server-initiated requests to the client.

package mcp

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// sessionRegistry manages active MCP sessions with TTL-based garbage collection.
//
// Lifecycle: create with newSessionRegistry; MUST call Close when shutting down
// so the GC goroutine exits. Zero value is NOT usable.
type sessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]*session

	ttl         time.Duration
	maxLifetime time.Duration // 0 = unlimited; hard cap on total session age regardless of activity
	queueSize   int
	maxSessions int // 0 = unlimited

	now func() time.Time // injectable for tests

	closeOnce sync.Once
	stop      chan struct{}
	stopped   chan struct{}
	stopFlag  atomic.Bool
}

// session represents one MCP conversation identified by Mcp-Session-Id.
//
// Safe for concurrent use. Outbound() delivers JSON-RPC frames produced by the
// server side (responses, notifications, or server-initiated requests) that
// the SSE writer drains and writes to the wire.
type session struct {
	// Immutable after Create.
	id              string
	createdAt       time.Time
	protocolVersion string
	identity        Identity
	clientElicit    bool // client declared capabilities.elicitation={} at initialize

	mu           sync.Mutex
	lastSeenAt   time.Time
	correlations map[string]chan elicitResponse // pending server-initiated requests keyed by JSON-RPC id

	sendMu       sync.Mutex  // serializes Send across producers (elicitation, tasks, transport)
	streamActive atomic.Bool // true while a GET SSE stream holds this session
	outbound     chan []byte
	closed       atomic.Bool

	// postMu guards activePostSink. The sink is request-scoped (set at POST
	// entry, cleared at POST exit) but Elicit reads it from the handler's
	// goroutine (same goroutine as handlePOST), so the mutex only matters
	// against late writes during SSE upgrade — it is never held across a
	// WriteFrame call to avoid inversions with the sink's own locking.
	postMu         sync.Mutex
	activePostSink replySink
}

// replySink is the per-POST frame writer. Two implementations live in
// streamable.go: jsonReplySink writes a single application/json response;
// sseReplySink writes text/event-stream frames. Elicit swaps jsonReplySink
// for sseReplySink on the session when it needs to emit a server-initiated
// request mid-call.
type replySink interface {
	// WriteFrame writes one JSON-RPC frame. For jsonReplySink, only the
	// first call succeeds (subsequent calls error). For sseReplySink every
	// call succeeds until Close.
	WriteFrame(frame []byte) error
	// IsSSE reports whether the sink already emits SSE frames. Used by
	// upgrade to avoid re-upgrading an already-SSE sink.
	IsSSE() bool
	// UpgradeToSSE replaces this sink with an SSE variant if it is not
	// already one. The returned sink writes the SSE response headers,
	// flushes, and is ready for data frames. The caller MUST swap the
	// returned sink into the session's activePostSink slot.
	UpgradeToSSE() (replySink, error)
}

// elicitResponse is the resolved-value type delivered on the per-elicit
// channel by ResolveElicit. Action is one of "accept", "decline", or
// "cancel"; Content is populated only for Action=="accept".
type elicitResponse struct {
	Action  string
	Content map[string]any
}

// Pending-elicit cap per session. Bounds the correlations map so a
// runaway tool (or malicious client that holds responses back) cannot
// exhaust memory. 32 is comfortably above any realistic interactive flow.
const maxPendingElicits = 32

const (
	sessionIDRawBytes   = 16 // 128 bits of entropy
	sessionIDEncodedLen = 22 // base64url no padding of 16 bytes

	defaultSessionQueue    = 64
	defaultSessionTTL      = 30 * time.Minute
	minSessionTTL          = 60 * time.Second
	maxSessionTTL          = 24 * time.Hour
	defaultMaxSessions     = 1024
	sessionGCInterval      = 30 * time.Second
	sessionHeartbeatWindow = 20 * time.Second // SSE heartbeat cadence; also bounds touch-via-stream freshness
	minHeartbeatInterval   = 50 * time.Millisecond
)

var (
	errSessionRegistryClosed = errors.New("mcp: session registry closed")
	errSessionClosed         = errors.New("mcp: session closed")
	errSessionQueueFull      = errors.New("mcp: session outbound queue full")
	errSessionLimitReached   = errors.New("mcp: session limit reached")
)

// newSessionRegistry returns a running registry.
//
//   - ttl is the idle TTL, clamped to [minSessionTTL, maxSessionTTL]; zero uses defaultSessionTTL.
//   - maxLifetime caps absolute session age regardless of activity (defends against
//     stream-hold DoS); zero disables the cap. Clamped to [ttl, maxSessionTTL*N].
//   - maxSessions zero uses defaultMaxSessions; negative disables the cap (unlimited).
//
// MUST call Close before process exit.
func newSessionRegistry(ttl, maxLifetime time.Duration, maxSessions int) *sessionRegistry {
	switch {
	case ttl == 0:
		ttl = defaultSessionTTL
	case ttl < minSessionTTL:
		ttl = minSessionTTL
	case ttl > maxSessionTTL:
		ttl = maxSessionTTL
	}
	if maxLifetime < 0 {
		maxLifetime = 0
	}
	// Cap semantics, unified with StreamableConfig.MaxSessions:
	//   <0 -> unlimited (represented internally as 0; Create's > 0 check disables it)
	//   0  -> use defaultMaxSessions (1024)
	//   >0 -> hard cap
	switch {
	case maxSessions < 0:
		maxSessions = 0
	case maxSessions == 0:
		maxSessions = defaultMaxSessions
	}
	r := &sessionRegistry{
		sessions:    make(map[string]*session),
		ttl:         ttl,
		maxLifetime: maxLifetime,
		queueSize:   defaultSessionQueue,
		maxSessions: maxSessions,
		now:         time.Now,
		stop:        make(chan struct{}),
		stopped:     make(chan struct{}),
	}
	go r.runGC()
	return r
}

// Create allocates a new session with a cryptographically random ID, the
// caller-declared protocol version, and the authenticated identity. The
// identity is bound once at initialize and is immutable for the life of
// the session; subsequent requests on the same Mcp-Session-Id are trusted
// by session-id validity alone (MCP 2025-06-18 auth is per-session, not
// per-request). Returns errSessionLimitReached when the registry is at
// maxSessions.
//
// The cap check runs BEFORE any allocation so a rejected create costs only a
// map lookup; a flood against the cap does not burn crypto/rand + chan
// allocation per attempt.
//
// clientElicit is set from the client's declared capabilities.elicitation
// at initialize; when false, session.Elicit returns ErrElicitUnsupported
// without sending a frame.
func (r *sessionRegistry) CreateWithCapabilities(protocolVersion string, identity Identity, clientElicit bool) (*session, error) {
	s, err := r.Create(protocolVersion, identity)
	if err != nil {
		return nil, err
	}
	s.clientElicit = clientElicit
	return s, nil
}

// Create allocates a new session with the zero capability flags. Prefer
// CreateWithCapabilities in code paths that know the client's declared
// capability set; Create remains for the legacy initialize path that did
// not parse capabilities.
func (r *sessionRegistry) Create(protocolVersion string, identity Identity) (*session, error) {
	if r.stopFlag.Load() {
		return nil, errSessionRegistryClosed
	}

	r.mu.Lock()
	if r.maxSessions > 0 && len(r.sessions) >= r.maxSessions {
		r.mu.Unlock()
		return nil, errSessionLimitReached
	}
	r.mu.Unlock()

	id, err := generateSessionID()
	if err != nil {
		return nil, err
	}
	now := r.now()
	s := &session{
		id:              id,
		createdAt:       now,
		lastSeenAt:      now,
		protocolVersion: protocolVersion,
		identity:        identity,
		outbound:        make(chan []byte, r.queueSize),
	}
	r.mu.Lock()
	// Re-check the cap under the insert lock so two concurrent Creates at the
	// boundary don't both slip past the pre-check.
	if r.maxSessions > 0 && len(r.sessions) >= r.maxSessions {
		r.mu.Unlock()
		return nil, errSessionLimitReached
	}
	if _, exists := r.sessions[id]; exists {
		r.mu.Unlock()
		return nil, errors.New("mcp: session id collision")
	}
	r.sessions[id] = s
	r.mu.Unlock()
	return s, nil
}

// Get returns the session for id and refreshes its last-seen timestamp.
// Returns (nil, false) if the session does not exist or has been deleted.
func (r *sessionRegistry) Get(id string) (*session, bool) {
	if !validSessionID(id) {
		return nil, false
	}
	r.mu.RLock()
	s, ok := r.sessions[id]
	r.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if s.closed.Load() {
		return nil, false
	}
	s.mu.Lock()
	s.lastSeenAt = r.now()
	s.mu.Unlock()
	return s, true
}

// Delete terminates the session with the given id.
// Returns true if the session existed and was deleted.
func (r *sessionRegistry) Delete(id string) bool {
	r.mu.Lock()
	s, ok := r.sessions[id]
	if ok {
		delete(r.sessions, id)
	}
	r.mu.Unlock()
	if !ok {
		return false
	}
	s.close()
	return true
}

// Len returns the current session count.
func (r *sessionRegistry) Len() int {
	r.mu.RLock()
	n := len(r.sessions)
	r.mu.RUnlock()
	return n
}

// Close stops the GC goroutine and terminates all live sessions. Idempotent.
func (r *sessionRegistry) Close() {
	r.closeOnce.Do(func() {
		r.stopFlag.Store(true)
		close(r.stop)
	})
	<-r.stopped

	r.mu.Lock()
	for id, s := range r.sessions {
		delete(r.sessions, id)
		s.close()
	}
	r.mu.Unlock()
}

// runGC sweeps expired sessions on a fixed interval until Close.
func (r *sessionRegistry) runGC() {
	defer close(r.stopped)
	t := time.NewTicker(sessionGCInterval)
	defer t.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-t.C:
			r.sweep()
		}
	}
}

func (r *sessionRegistry) sweep() {
	now := r.now()
	idleCutoff := now.Add(-r.ttl)
	var expired []*session
	r.mu.Lock()
	for id, s := range r.sessions {
		s.mu.Lock()
		stale := s.lastSeenAt.Before(idleCutoff)
		s.mu.Unlock()
		// Absolute lifetime cap — evict even if the stream's heartbeat keeps
		// lastSeenAt fresh. Defends against clients that hold sessions forever
		// via GET SSE streams.
		if !stale && r.maxLifetime > 0 && now.Sub(s.createdAt) > r.maxLifetime {
			stale = true
		}
		if stale {
			delete(r.sessions, id)
			expired = append(expired, s)
		}
	}
	r.mu.Unlock()
	for _, s := range expired {
		s.close()
	}
}

// ClientSupportsElicit reports whether the session's client declared the
// elicitation capability at initialize. Call this before session.Elicit:
// the spec MUSTs that servers not emit elicitation/create without client
// consent, and Elicit itself also guards — this helper is the cheap pre-check.
func (s *session) ClientSupportsElicit() bool { return s.clientElicit }

// RegisterElicit creates a fresh pending-elicitation entry and returns its
// server-generated id plus a channel the caller blocks on. The id is a
// base64url-encoded 128-bit random value, matching the session-id rationale.
// Returns ErrElicitTooMany when the per-session cap is reached.
//
// The channel is buffered with capacity 1 so ResolveElicit never blocks.
// Callers MUST either drain the channel or call CancelElicit(id) to prevent
// a map leak.
func (s *session) RegisterElicit() (string, <-chan elicitResponse, error) {
	id, err := generateSessionID() // reuse session-id RNG; 128 bits, base64url
	if err != nil {
		return "", nil, err
	}
	ch := make(chan elicitResponse, 1)
	s.mu.Lock()
	if s.correlations == nil {
		s.correlations = make(map[string]chan elicitResponse)
	}
	if len(s.correlations) >= maxPendingElicits {
		s.mu.Unlock()
		return "", nil, ErrElicitTooMany
	}
	// Collision on a 128-bit id is vanishingly unlikely but treat it as an
	// error rather than silently overwriting.
	if _, exists := s.correlations[id]; exists {
		s.mu.Unlock()
		return "", nil, errors.New("mcp: elicit id collision")
	}
	s.correlations[id] = ch
	s.mu.Unlock()
	return id, ch, nil
}

// ResolveElicit delivers a client response to the pending-elicitation
// channel for id and removes the map entry. Returns false when id does not
// match any pending correlation (AC-15b: silently drop unknown-id replies).
// The channel write never blocks because correlation channels are buffered.
func (s *session) ResolveElicit(id string, resp elicitResponse) bool {
	s.mu.Lock()
	ch, ok := s.correlations[id]
	if ok {
		delete(s.correlations, id)
	}
	s.mu.Unlock()
	if !ok {
		return false
	}
	// Buffered chan cap=1, so this never blocks.
	ch <- resp
	return true
}

// CancelElicit removes a pending correlation without delivering a response.
// Called on ctx cancel (AC-15c) and by Elicit's defer on any early return.
// Idempotent: a missing entry is silently tolerated.
func (s *session) CancelElicit(id string) {
	s.mu.Lock()
	delete(s.correlations, id)
	s.mu.Unlock()
}

// PendingElicitCount returns the current pending-correlation count. Test
// helper; also used by the session-close path to reason about leaks.
func (s *session) PendingElicitCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.correlations)
}

// SetActivePostSink binds a per-POST reply sink to the session for the
// duration of one POST handler. Returns a cleanup func the caller MUST
// defer so the sink is cleared before handlePOST returns. Returns an error
// if a sink is already bound — concurrent POSTs on the same session would
// race on frame ordering, so we refuse rather than silently overwrite.
func (s *session) SetActivePostSink(sink replySink) (func(), error) {
	s.postMu.Lock()
	if s.activePostSink != nil {
		s.postMu.Unlock()
		return nil, errors.New("mcp: session already has an active POST sink")
	}
	s.activePostSink = sink
	s.postMu.Unlock()
	return func() {
		s.postMu.Lock()
		s.activePostSink = nil
		s.postMu.Unlock()
	}, nil
}

// CurrentPostSink returns the sink currently bound to the active POST, or
// nil when no POST is being handled. Called by Elicit to write frames.
func (s *session) CurrentPostSink() replySink {
	s.postMu.Lock()
	defer s.postMu.Unlock()
	return s.activePostSink
}

// UpgradeCurrentSinkToSSE swaps the active POST's sink from JSON to SSE
// when it is not already SSE. No-op if already SSE. Returns an error if
// no POST is active (called outside a handler) or the sink cannot be
// upgraded (rare; see sseReplySink construction).
func (s *session) UpgradeCurrentSinkToSSE() error {
	s.postMu.Lock()
	defer s.postMu.Unlock()
	if s.activePostSink == nil {
		return errors.New("mcp: no active POST to upgrade")
	}
	if s.activePostSink.IsSSE() {
		return nil
	}
	upgraded, err := s.activePostSink.UpgradeToSSE()
	if err != nil {
		return err
	}
	s.activePostSink = upgraded
	return nil
}

// ID returns the stable session identifier.
func (s *session) ID() string { return s.id }

// ProtocolVersion returns the negotiated protocol version.
func (s *session) ProtocolVersion() string { return s.protocolVersion }

// Identity returns the authenticated identity bound at session create.
// The zero Identity (IsAnonymous) indicates AuthMode=None or pre-Phase-2
// configurations where no identity was attached.
func (s *session) Identity() Identity { return s.identity }

// Touch refreshes the session's last-seen timestamp so an active long-lived
// stream (GET /mcp SSE reader) is not reaped by the TTL sweep. No-op on a
// closed session so stale pointers from the heartbeat loop do not update
// timestamps on sessions the GC has already removed.
func (s *session) Touch(now time.Time) {
	if s.closed.Load() {
		return
	}
	s.mu.Lock()
	s.lastSeenAt = now
	s.mu.Unlock()
}

// Send enqueues a pre-marshaled JSON-RPC frame for delivery on the session's
// SSE stream. Non-blocking: returns errSessionQueueFull when the queue is at
// capacity, errSessionClosed when the session is closed. Safe for concurrent
// producers (transport dispatch, elicitation, task notifications).
func (s *session) Send(frame []byte) error {
	if s.closed.Load() {
		return errSessionClosed
	}
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	if s.closed.Load() {
		return errSessionClosed
	}
	if len(s.outbound) >= cap(s.outbound) {
		return errSessionQueueFull
	}
	s.outbound <- frame
	return nil
}

// Outbound returns the channel that the SSE writer drains. Closed when the
// session closes.
func (s *session) Outbound() <-chan []byte { return s.outbound }

// Close terminates the session. Idempotent.
func (s *session) Close() { s.close() }

// close sets the closed flag and closes the outbound channel. Acquires sendMu
// before closing so any Send that passed the closed-flag check and is about
// to write to the channel finishes first — otherwise the channel close races
// with the pending send and panics.
func (s *session) close() {
	if !s.closed.CompareAndSwap(false, true) {
		return
	}
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	close(s.outbound)
}

// generateSessionID returns a fresh base64url-encoded 128-bit identifier.
func generateSessionID() (string, error) {
	var buf [sessionIDRawBytes]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

// validSessionID reports whether id is visible ASCII (0x21..0x7E) per the
// MCP 2025-06-18 transports spec.
func validSessionID(id string) bool {
	if id == "" {
		return false
	}
	for i := range len(id) {
		c := id[i]
		if c < 0x21 || c > 0x7E {
			return false
		}
	}
	return true
}

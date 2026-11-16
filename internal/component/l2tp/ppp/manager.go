// Design: docs/research/l2tpv2-ze-integration.md -- PPP driver + per-session goroutines
// Related: session.go -- pppSession struct held in the sessions map
// Related: session_run.go -- per-session goroutine main loop
// Related: auth_events.go -- AuthEvent sealed sum, AuthMethod, authResponseMsg

package ppp

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"reflect"
	"sync"
)

// Default channel buffer sizes.
const (
	defaultSessionsInBuf    = 16
	defaultEventsOutBuf     = 64
	defaultAuthEventsOutBuf = 64
	defaultIPEventsOutBuf   = 64
)

// Errors returned by Driver.
var (
	// ErrDriverStopped is returned when StartSession is called after
	// Stop.
	ErrDriverStopped = errors.New("ppp: driver stopped")

	// ErrAlreadyStarted is returned when Start is called twice.
	ErrAlreadyStarted = errors.New("ppp: driver already started")

	// ErrSessionNotFound is returned by StopSession / SessionByID for
	// an unknown (tunnelID, sessionID) pair.
	ErrSessionNotFound = errors.New("ppp: session not found")

	// ErrAuthResponsePending is returned by AuthResponse when the
	// session's buffered(1) authRespCh already holds an unconsumed
	// decision. Signals a caller bug (duplicate AuthResponse for one
	// request) or a race with session teardown.
	ErrAuthResponsePending = errors.New("ppp: auth response already pending")

	// ErrIPResponsePending is returned by IPResponse when the
	// session's buffered(2) ipRespCh already holds two pending
	// decisions (one per family). Signals a caller bug (duplicate
	// IPResponse for one request) or a race with session teardown.
	ErrIPResponsePending = errors.New("ppp: ip response already pending")
)

// sessionKey identifies a PPP session by its (tunnelID, sessionID)
// pair. The transport assigns these; ppp treats them as opaque.
type sessionKey struct {
	tunnelID  uint16
	sessionID uint16
}

// Driver owns all active PPP sessions. It accepts StartSession
// payloads on SessionsIn(), spawns one long-lived goroutine per
// session, and emits Event values on EventsOut() that the transport
// consumes.
//
// (Originally named "Manager" / "Engine" in the design discussion;
// renamed to Driver because both prior names collide with types in
// other ze components and the project's check-existing-patterns
// hook treats bare type names as a global namespace.)
//
// Caller MUST call Start before any other method. Caller MUST call
// Stop before discarding the Driver (Stop blocks until every
// session goroutine exits).
//
// Safe for concurrent use across the documented public methods.
type Driver struct {
	logger *slog.Logger

	sessionsIn    chan StartSession
	eventsOut     chan Event
	authEventsOut chan AuthEvent
	ipEventsOut   chan IPEvent

	dispatchDone chan struct{} // closed when dispatch goroutine exits
	stopCh       chan struct{}

	mu       sync.Mutex
	started  bool
	stopped  bool
	sessions map[sessionKey]*pppSession
	// owners retains each admitted lease until its worker exits, including
	// the interval after StopSession or ReleaseSession removes its key.
	owners map[SessionHandle]sessionKey
	wg     sync.WaitGroup
}

// NewProductionDriver constructs the protocol driver. Each transport supplies
// its own created frame channel and data-plane owner on StartSession.
// Authentication stays on AuthEventsOut/AuthResponse.
// Caller MUST call Start before enqueueing and Stop before discarding it.
func NewProductionDriver(logger *slog.Logger) *Driver {
	if logger == nil {
		panic("BUG: ppp.NewProductionDriver: Logger is required")
	}
	return &Driver{
		logger:        logger,
		sessionsIn:    make(chan StartSession, defaultSessionsInBuf),
		eventsOut:     make(chan Event, defaultEventsOutBuf),
		authEventsOut: make(chan AuthEvent, defaultAuthEventsOutBuf),
		ipEventsOut:   make(chan IPEvent, defaultIPEventsOutBuf),
		dispatchDone:  make(chan struct{}),
		sessions:      make(map[sessionKey]*pppSession),
		owners:        make(map[SessionHandle]sessionKey),
	}
}

// Start launches the dispatch goroutine and registers this component's
// metrics hook (metrics.go). Returns ErrAlreadyStarted if called twice
// (with or without an intervening Stop).
//
// Caller MUST call Stop before discarding the Driver.
func (d *Driver) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.started {
		return ErrAlreadyStarted
	}
	d.started = true
	d.stopCh = make(chan struct{})
	registerPPPMetrics()
	go d.dispatch()
	return nil
}

// Stop signals the dispatch loop and every session goroutine to
// exit, then waits for them. Idempotent. After Stop returns, the
// Driver MUST NOT be re-used; create a new one.
//
// During wg.Wait, session goroutines acquire d.mu to remove their
// own entries from d.sessions via their natural-exit defer. The
// map is therefore empty (or near-empty) by the time Stop returns,
// drained by the goroutines themselves rather than by Stop.
func (d *Driver) Stop() {
	d.mu.Lock()
	if d.stopped || !d.started {
		d.mu.Unlock()
		return
	}
	d.stopped = true
	stopCh := d.stopCh
	d.mu.Unlock()

	close(stopCh)

	// Close every session's chan fd so blocking reads return EBADF
	// and goroutines exit. This is the standard "close-fd shutdown"
	// pattern; see goroutine-lifecycle.md.
	d.mu.Lock()
	for _, s := range d.sessions {
		s.closeFiles()
	}
	d.mu.Unlock()

	d.wg.Wait()
	<-d.dispatchDone
	close(d.eventsOut)
	close(d.authEventsOut)
	close(d.ipEventsOut)
}

// SessionsIn returns the write-only channel the transport uses to
// hand new sessions to the Driver. Send a StartSession to launch
// a per-session goroutine. Caller MUST NOT close this channel; the
// Driver owns it.
func (d *Driver) SessionsIn() chan<- StartSession {
	return d.sessionsIn
}

// EventsOut returns the read-only channel the transport selects on
// to receive PPP lifecycle events. Closed by the Driver during
// Stop after every session has exited.
func (d *Driver) EventsOut() <-chan Event {
	return d.eventsOut
}

// AuthEventsOut returns the read-only channel the external auth
// handler (l2tp-auth plugin in production, test responder in unit
// tests) reads to receive EventAuthRequest / EventAuthSuccess /
// EventAuthFailure. Closed by the Driver during Stop after every
// session has exited.
//
// Caller MUST consume this channel promptly when sessions are
// active; sustained backlog beyond the default buffer (64) blocks
// PPP progress. Separate from EventsOut so L2TP's reactor is not
// forced to pattern-match auth types it does not act on.
func (d *Driver) AuthEventsOut() <-chan AuthEvent {
	return d.authEventsOut
}

// IPEventsOut returns the read-only channel the external IP handler
// (l2tp-pool plugin in production, test responder in unit tests)
// reads to receive EventIPRequest. Closed by the Driver during Stop
// after every session has exited.
//
// Caller MUST consume this channel promptly when sessions are
// active; sustained backlog beyond the default buffer (64) blocks
// PPP progress. Separate from EventsOut and AuthEventsOut so a
// handler subscribes only to the concern it implements.
func (d *Driver) IPEventsOut() <-chan IPEvent {
	return d.ipEventsOut
}

// IPResponseArgs packages the fields Driver.IPResponse hands to the
// per-session goroutine. A struct keeps the method signature stable
// as families grow new optional knobs (reverse DNS, hostname,
// MS-specific NBNS options).
//
// On accept=true for family=ipv4, Local and Peer MUST both be
// non-zero IPv4 addresses; DNSPrimary / DNSSecondary are optional.
//
// On accept=true for family=ipv6, PeerInterfaceID is optional: a
// non-zero value forces the peer to use that identifier, zero
// accepts whatever the peer offered.
type IPResponseArgs struct {
	Accept bool
	Family AddressFamily
	Reason string
	// AllocationID identifies the pool reservation this answer owns. A pool
	// MUST give each reservation a nonzero ID; a drain MUST return that ID
	// through the subscriber event bus if IPResponse cannot deliver it.
	// Zero means the answer owns no pool reservation. PPP does not interpret it.
	AllocationID     uint64
	Local            netip.Addr
	Peer             netip.Addr
	DNSPrimary       netip.Addr
	DNSSecondary     netip.Addr
	PeerInterfaceID  [8]byte
	HasPeerInterface bool
}

// IPResponse delivers the external handler's IP decision to the
// per-session goroutine. Returns ErrSessionNotFound if no session
// matches (tunnelID, sessionID) -- this covers both "never started"
// and "already torn down" from the caller's perspective.
//
// Safe for concurrent use. Non-blocking: the per-session ipRespCh
// is buffered(2) (one slot per family); a duplicate IPResponse for
// the same family returns ErrIPResponsePending rather than blocking.
func (d *Driver) IPResponse(tunnelID, sessionID uint16, args IPResponseArgs) error {
	k := sessionKey{tunnelID, sessionID}
	d.mu.Lock()
	s, ok := d.sessions[k]
	d.mu.Unlock()
	if !ok {
		return ErrSessionNotFound
	}
	msg := ipResponseMsg{
		accept:           args.Accept,
		family:           args.Family,
		reason:           args.Reason,
		local:            args.Local,
		peer:             args.Peer,
		dnsPrimary:       args.DNSPrimary,
		dnsSecondary:     args.DNSSecondary,
		peerInterfaceID:  args.PeerInterfaceID,
		hasPeerInterface: args.HasPeerInterface,
	}
	select {
	case s.ipRespCh <- msg:
		return nil
	default: // buffered(2) already full: duplicate IPResponse or teardown race
		return ErrIPResponsePending
	}
}

// AuthResponse delivers the external handler's auth decision to the
// per-session goroutine. Returns ErrSessionNotFound if no session
// matches (tunnelID, sessionID) -- this covers both "never started"
// and "already torn down" from the caller's perspective.
//
// Safe for concurrent use. Non-blocking: the per-session authRespCh
// is buffered(1); a duplicate AuthResponse for the same request
// returns ErrAuthResponsePending rather than blocking.
func (d *Driver) AuthResponse(tunnelID, sessionID uint16, accept bool, message string, authResponseBlob []byte) error {
	k := sessionKey{tunnelID, sessionID}
	d.mu.Lock()
	s, ok := d.sessions[k]
	d.mu.Unlock()
	if !ok {
		return ErrSessionNotFound
	}
	msg := authResponseMsg{
		accept:           accept,
		message:          message,
		authResponseBlob: authResponseBlob,
	}
	select {
	case s.authRespCh <- msg:
		return nil
	default: // buffered(1) already full: caller issued a duplicate AuthResponse or lost a race with teardown
		return ErrAuthResponsePending
	}
}

// StopSession terminates the session for (tunnelID, sessionID) and
// waits for the goroutine to exit. Returns ErrSessionNotFound if no
// such session exists.
//
// Closes the per-session sessStop first so that a goroutine parked
// in the auth phase (waiting on authRespCh) unblocks; closing
// chanFile alone would only unblock readFrames, which is not running
// yet during the initial auth phase on the proxy-LCP short-circuit
// path.
//
// Idempotent: calling StopSession a second time returns
// ErrSessionNotFound (the first call removed it from the map).
func (d *Driver) StopSession(tunnelID, sessionID uint16) error {
	k := sessionKey{tunnelID, sessionID}
	d.mu.Lock()
	s, ok := d.sessions[k]
	if !ok {
		d.mu.Unlock()
		return ErrSessionNotFound
	}
	delete(d.sessions, k)
	d.mu.Unlock()

	s.sessStopOnce.Do(func() { close(s.sessStop) })
	s.closeFiles()
	<-s.done
	return nil
}

// sessionByID returns a snapshot of a session's state, or false if
// no such session exists. Used for `show l2tp session` and tests.
func (d *Driver) sessionByID(tunnelID, sessionID uint16) (sessionInfo, bool) {
	k := sessionKey{tunnelID, sessionID}
	d.mu.Lock()
	s, ok := d.sessions[k]
	d.mu.Unlock()
	if !ok {
		return sessionInfo{}, false
	}
	s.mu.Lock()
	info := s.snapshot()
	s.mu.Unlock()
	return info, true
}

// dispatch is the single goroutine that reads SessionsIn and spawns
// per-session goroutines. Exits when stopCh closes.
func (d *Driver) dispatch() {
	defer close(d.dispatchDone)
	defer func() {
		// Transport producers stop before Driver.Stop. Every queued item
		// has already transferred ownership, even if never dispatched.
		for {
			select {
			case start, ok := <-d.sessionsIn:
				if !ok {
					return
				}
				start.closeFiles()
			default:
				return
			}
		}
	}()
	for {
		select {
		case <-d.stopCh:
			return
		default:
		}
		select {
		case <-d.stopCh:
			return
		case start, ok := <-d.sessionsIn:
			if !ok {
				return
			}
			d.spawnSession(&start)
		}
	}
}

// spawnSession creates a pppSession, registers it in the map, and
// launches its goroutine. Logs and drops if a duplicate
// (tunnelID, sessionID) arrives, or if the StartSession contains
// missing session ownership or frame delivery.
func (d *Driver) spawnSession(start *StartSession) {
	k := sessionKey{start.TunnelID, start.SessionID}
	comparableOwner := !sessionResourceMissing(start.Dataplane) && reflect.ValueOf(start.Dataplane).Comparable()
	// Detect an existing lease before validating borrowed resources or LCP:
	// a malformed requeue has not transferred that live owner again.
	d.mu.Lock()
	existingKey, existingOwner := d.existingTransferLocked(k, start.Dataplane, comparableOwner)
	stopped := d.stopped
	d.mu.Unlock()
	if stopped {
		if !existingOwner {
			start.closeFiles()
		}
		return
	}
	ownershipConflict := existingKey || existingOwner
	if ownershipConflict {
		d.rejectExistingTransfer(start, existingKey, existingOwner)
		return
	}
	// Missing ownership or missing control delivery is an invalid transfer;
	// release any supplied owner before publishing its refusal.
	if sessionResourceMissing(start.Dataplane) {
		start.closeFiles()
		d.emitRejection(start, "missing session data plane")
		return
	}
	if !comparableOwner {
		start.closeFiles()
		d.emitRejection(start, "session data-plane owner cannot be compared for safe lease admission")
		return
	}
	if sessionResourceMissing(start.Frames) {
		start.closeFiles()
		d.emitRejection(start, "missing frame channel")
		return
	}
	if sessionResourceMissing(start.Frames.Control()) {
		start.closeFiles()
		d.emitRejection(start, "missing control stream")
		return
	}
	maxMRU := start.MaxMRU
	if err := start.LCP.Validate(); err != nil {
		start.closeFiles()
		d.emitRejection(start, err.Error())
		return
	}
	if maxMRU == 0 {
		maxMRU = MaxFrameLen
	}
	// RFC 791 Section 3.2 and RFC 5072 Section 2: admit only protocol-capable MRUs.
	if err := ValidateNetworkMRU(maxMRU, !start.DisableIPv6CP); err != nil {
		start.closeFiles()
		d.emitRejection(start, err.Error())
		return
	}
	d.mu.Lock()
	existingKey, existingOwner = d.existingTransferLocked(k, start.Dataplane, true)
	if d.stopped {
		d.mu.Unlock()
		if !existingOwner {
			start.closeFiles()
		}
		return
	}
	ownershipConflict = existingKey || existingOwner
	if ownershipConflict {
		d.mu.Unlock()
		d.rejectExistingTransfer(start, existingKey, existingOwner)
		return
	}

	fallback := start.AuthFallbackOrder

	// A session that may be handed to a realm relay records its LCP
	// exchange and reads its channel through a file a quiesce can wake
	// (relay_handoff.go). Every other session keeps the plain blocking file.
	var (
		capture       *lcpCapture
		readerQuiesce chan struct{}
		readerGone    chan struct{}
	)
	if start.CaptureProxyLCP {
		capture = &lcpCapture{}
		readerQuiesce = make(chan struct{})
		readerGone = make(chan struct{})
	}
	s := &pppSession{
		lcpConfig:            start.LCP,
		proxyCapture:         capture,
		subscriberID:         start.SubscriberID,
		tunnelID:             start.TunnelID,
		sessionID:            start.SessionID,
		chanFile:             start.Frames.Control(),
		frames:               start.Frames,
		dataplane:            start.Dataplane,
		readerQuiesce:        readerQuiesce,
		readerGone:           readerGone,
		unitFile:             start.Frames.Opened(),
		lnsMode:              start.LNSMode,
		pppoe:                start.PPPoE,
		maxMRU:               maxMRU,
		echoInterval:         start.EchoInterval,
		echoFailures:         start.EchoFailures,
		authTimeout:          start.AuthTimeout,
		authRequired:         start.AuthRequired,
		authFallbackOrder:    fallback,
		reauthInterval:       start.ReauthInterval,
		configuredAuthMethod: initialAuthMethod(start.AuthMethod, fallback),
		disableIPCP:          start.DisableIPCP,
		disableIPv6CP:        start.DisableIPv6CP,
		ipTimeout:            start.IPTimeout,
		eventsOut:            d.eventsOut,
		authEventsOut:        d.authEventsOut,
		ipEventsOut:          d.ipEventsOut,
		authRespCh:           make(chan authResponseMsg, 1),
		ipRespCh:             make(chan ipResponseMsg, 2),
		stopCh:               d.stopCh,
		sessStop:             make(chan struct{}),
		done:                 make(chan struct{}),
		logger: d.logger.With(
			"tunnel-id", start.TunnelID,
			"session-id", start.SessionID,
			"interface", start.Dataplane.Identity().InterfaceName,
		),
	}
	d.sessions[k] = s
	d.owners[start.Dataplane] = k
	d.wg.Add(1)
	d.mu.Unlock()

	go func() {
		defer d.wg.Done()
		defer close(s.done)
		defer func() {
			// On natural exit (peer teardown, LCP failure, auth
			// reject, timeout), remove the session from the map so
			// AuthResponse / SessionByID observe ErrSessionNotFound
			// instead of holding a stale reference. StopSession may have
			// already released the key for a replacement, which this old
			// worker must preserve. Its lease stays reserved until now.
			d.mu.Lock()
			if d.sessions[k] == s {
				delete(d.sessions, k)
			}
			if ownerKey, present := d.owners[s.dataplane]; present {
				if ownerKey == k {
					delete(d.owners, s.dataplane)
				}
			}
			d.mu.Unlock()
		}()
		s.run(start)
	}()
}

// existingTransferLocked uses the reverse index only for comparable owners.
// Caller MUST hold d.mu. The key and owner can belong to different sessions.
func (d *Driver) existingTransferLocked(key sessionKey, owner SessionHandle, comparableOwner bool) (bool, bool) {
	_, existingKey := d.sessions[key]
	if !comparableOwner {
		return existingKey, false
	}
	_, existingOwner := d.owners[owner]
	return existingKey, existingOwner
}

// rejectExistingTransfer refuses a request without releasing any already
// admitted owner. Only an active incoming key forbids transport teardown;
// an owner reused under another key does not establish that new transport.
func (d *Driver) rejectExistingTransfer(start *StartSession, existingKey, existingOwner bool) {
	if !existingOwner {
		start.closeFiles()
	}
	reason := "session data-plane owner is already admitted"
	if existingKey {
		reason = "duplicate (tunnelID, sessionID)"
	}
	subject := start.SubscriberID
	if subject == "" {
		subject = fmt.Sprintf("tunnel %d session %d", start.TunnelID, start.SessionID)
	}
	ReportAdmissionFailure(d.logger, subject, AdmissionOwnership, errors.New(reason),
		map[string]any{"tunnel-id": start.TunnelID, "session-id": start.SessionID})
	d.sendRejection(start, reason, existingKey)
}

// emitRejection is called when spawnSession refuses a new StartSession
// for invalid ownership, frame delivery or negotiation. Emits EventSessionRejected (distinct
// from EventSessionDown) so the transport can discriminate between
// "PPP never took this session" and "PPP ran this session then it
// ended" without parsing reason strings.
//
// The refused request's PPP handle has already been released. The transport
// reconciles its separate session owner.
func (d *Driver) emitRejection(start *StartSession, reason string) {
	d.sendRejection(start, reason, false)
}

func (d *Driver) sendRejection(start *StartSession, reason string, existingSession bool) {
	select {
	case d.eventsOut <- EventSessionRejected{
		TunnelID:        start.TunnelID,
		SessionID:       start.SessionID,
		Reason:          reason,
		ExistingSession: existingSession,
	}:
	case <-d.stopCh:
	}
}

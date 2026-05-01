// Design: docs/research/l2tpv2-ze-integration.md -- PPP driver + per-session goroutines
// Related: session.go -- pppSession struct held in the sessions map
// Related: session_run.go -- per-session goroutine main loop
// Related: auth_events.go -- AuthEvent sealed sum, AuthMethod, authResponseMsg

package ppp

import (
	"errors"
	"log/slog"
	"net/netip"
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

// newChanFileFn is the constructor for the chan fd's
// io.ReadWriteCloser. Production wires NewFDFile (os.NewFile + Go
// runtime poller). Tests swap this via export_test.go to use
// net.Pipe so the Driver can be exercised without /dev/ppp.
var newChanFileFn = NewFDFile

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
	logger  *slog.Logger
	backend IfaceBackend
	ops     pppOps

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
	wg       sync.WaitGroup
}

// DriverConfig captures the dependencies the Driver needs.
// Construction fails (panics on nil required fields) at NewDriver
// to keep the start-time check explicit.
type DriverConfig struct {
	// Logger is required. Use slogutil.Logger("ppp") in production.
	Logger *slog.Logger

	// Backend configures pppN interface properties (MTU, admin
	// state) after LCP-Opened. iface.GetBackend() in production;
	// fake in tests.
	Backend IfaceBackend

	// Ops is the syscall surface for /dev/ppp ioctls. newPPPOps()
	// in production; fake func fields in tests.
	Ops pppOps
}

// NewProductionDriver constructs a Driver wired to the real /dev/ppp
// ioctl ops, the supplied logger, and the iface backend for pppN MTU
// set. The transport (l2tp today, PPPoE later) calls this rather than
// NewDriver because pppOps is package-private.
//
// Authentication is delivered through AuthEventsOut() / AuthResponse();
// the constructor does not take an auth handler. Production wires a
// consumer (l2tp-auth plugin in Phase 8); tests wire an auto-accept
// responder.
//
// Caller MUST call Start before sending on SessionsIn(). Caller MUST
// call Stop before discarding the Driver.
func NewProductionDriver(logger *slog.Logger, backend IfaceBackend) *Driver {
	return NewDriver(DriverConfig{
		Logger:  logger,
		Backend: backend,
		Ops:     newPPPOps(),
	})
}

// NewDriver constructs a Driver. Does NOT start the dispatch
// goroutine; call Start when ready.
//
// Panics with "BUG: ..." if any required field is nil; these are
// programmer errors caught at boot, not runtime conditions.
func NewDriver(cfg DriverConfig) *Driver {
	if cfg.Logger == nil {
		panic("BUG: ppp.NewDriver: Logger is required")
	}
	if cfg.Backend == nil {
		panic("BUG: ppp.NewDriver: Backend is required")
	}
	if cfg.Ops.setMRU == nil {
		panic("BUG: ppp.NewDriver: Ops.setMRU is required")
	}
	return &Driver{
		logger:        cfg.Logger,
		backend:       cfg.Backend,
		ops:           cfg.Ops,
		sessionsIn:    make(chan StartSession, defaultSessionsInBuf),
		eventsOut:     make(chan Event, defaultEventsOutBuf),
		authEventsOut: make(chan AuthEvent, defaultAuthEventsOutBuf),
		ipEventsOut:   make(chan IPEvent, defaultIPEventsOutBuf),
		dispatchDone:  make(chan struct{}),
		sessions:      make(map[sessionKey]*pppSession),
	}
}

// Start launches the dispatch goroutine. Returns ErrAlreadyStarted
// if called twice (with or without an intervening Stop).
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
		if s.chanFile != nil {
			_ = s.chanFile.Close() //nolint:errcheck // shutdown best-effort
		}
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
	Accept           bool
	Family           AddressFamily
	Reason           string
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
	_ = s.chanFile.Close() //nolint:errcheck // shutdown best-effort
	<-s.done
	return nil
}

// SessionByID returns a snapshot of a session's state, or false if
// no such session exists. Used for `show l2tp session` and tests.
func (d *Driver) SessionByID(tunnelID, sessionID uint16) (SessionInfo, bool) {
	k := sessionKey{tunnelID, sessionID}
	d.mu.Lock()
	s, ok := d.sessions[k]
	d.mu.Unlock()
	if !ok {
		return SessionInfo{}, false
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
	for {
		select {
		case <-d.stopCh:
			return
		case start, ok := <-d.sessionsIn:
			if !ok {
				return
			}
			d.spawnSession(start)
		}
	}
}

// spawnSession creates a pppSession, registers it in the map, and
// launches its goroutine. Logs and drops if a duplicate
// (tunnelID, sessionID) arrives, or if the StartSession contains
// invalid file descriptors.
func (d *Driver) spawnSession(start StartSession) {
	// Reject sentinel zero / negative fd values. Production fds always
	// come from socket()/open() and are >= 3 (stdio holds 0-2), so this
	// is a defensive guard against uninitialized StartSession structs
	// rather than a meaningful range check.
	//
	// Even though no goroutine was spawned for this session, we emit
	// EventSessionDown so the transport's reconciliation loop sees
	// the rejection and cleans up its kernel state. PPP does NOT
	// close the fds -- the transport owns them.
	if start.ChanFD <= 0 || start.UnitFD <= 0 {
		d.logger.Warn("ppp: StartSession with invalid fds ignored",
			"tunnel-id", start.TunnelID, "session-id", start.SessionID,
			"chan-fd", start.ChanFD, "unit-fd", start.UnitFD)
		d.emitRejection(start, "invalid fds")
		return
	}
	k := sessionKey{start.TunnelID, start.SessionID}

	d.mu.Lock()
	if _, exists := d.sessions[k]; exists {
		d.mu.Unlock()
		d.logger.Warn("ppp: duplicate StartSession ignored",
			"tunnel-id", start.TunnelID, "session-id", start.SessionID)
		d.emitRejection(start, "duplicate (tunnelID, sessionID)")
		return
	}

	maxMRU := start.MaxMRU
	if maxMRU == 0 {
		maxMRU = MaxFrameLen
	}

	fallback := start.AuthFallbackOrder
	if len(fallback) == 0 {
		fallback = defaultAuthFallbackOrder()
	}

	s := &pppSession{
		tunnelID:             start.TunnelID,
		sessionID:            start.SessionID,
		chanFile:             newChanFileFn(start.ChanFD, "ppp.chan"),
		unitFD:               start.UnitFD,
		unitNum:              start.UnitNum,
		lnsMode:              start.LNSMode,
		maxMRU:               maxMRU,
		echoInterval:         start.EchoInterval,
		echoFailures:         start.EchoFailures,
		authTimeout:          start.AuthTimeout,
		authRequired:         start.AuthRequired,
		authFallbackOrder:    fallback,
		reauthInterval:       start.ReauthInterval,
		configuredAuthMethod: start.AuthMethod,
		disableIPCP:          start.DisableIPCP,
		disableIPv6CP:        start.DisableIPv6CP,
		ipTimeout:            start.IPTimeout,
		backend:              d.backend,
		ops:                  d.ops,
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
			"unit", start.UnitNum,
		),
	}
	d.sessions[k] = s
	d.wg.Add(1)
	d.mu.Unlock()

	go func() {
		defer d.wg.Done()
		defer close(s.done)
		defer func() {
			// On natural exit (peer teardown, LCP failure, auth
			// reject, timeout), remove the session from the map so
			// AuthResponse / SessionByID observe ErrSessionNotFound
			// instead of holding a stale reference. StopSession has
			// already done the delete when it fires; delete of an
			// absent key is a no-op.
			d.mu.Lock()
			delete(d.sessions, k)
			d.mu.Unlock()
		}()
		s.run(start)
	}()
}

// emitRejection is called when spawnSession refuses a StartSession
// (invalid fds, duplicate). Emits EventSessionRejected (distinct
// from EventSessionDown) so the transport can discriminate between
// "PPP never took this session" and "PPP ran this session then it
// ended" without parsing reason strings.
//
// Does NOT close ChanFD or UnitFD: both were opened by the
// transport (L2TP kernel worker in 6a; PPPoE or similar later), and
// the transport owns their lifecycle. Closing here could terminate
// a DIFFERENT session that legitimately holds the same fd -- the
// dedup key is (tunnelID, sessionID), not fd.
func (d *Driver) emitRejection(start StartSession, reason string) {
	select {
	case d.eventsOut <- EventSessionRejected{
		TunnelID:  start.TunnelID,
		SessionID: start.SessionID,
		Reason:    reason,
	}:
	case <-d.stopCh:
	}
}

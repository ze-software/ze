// Design: docs/research/l2tpv2-ze-integration.md -- PPP driver + per-session goroutines
// Related: session.go -- pppSession struct held in the sessions map
// Related: session_run.go -- per-session goroutine main loop

package ppp

import (
	"errors"
	"log/slog"
	"sync"
)

// Default channel buffer sizes.
const (
	defaultSessionsInBuf = 16
	defaultEventsOutBuf  = 64
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
	logger   *slog.Logger
	authHook AuthHook
	backend  IfaceBackend
	ops      pppOps

	sessionsIn chan StartSession
	eventsOut  chan Event

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

	// AuthHook handles the auth-phase decision after LCP-Opened.
	// 6a passes StubAuthHook{Logger: Logger}; 6b replaces this
	// interface with a channel-based dispatcher.
	AuthHook AuthHook

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
// Auth hook is hardwired to StubAuthHook for the 6a phase. spec-6b
// replaces the AuthHook interface with a channel-based dispatcher and
// at that point this constructor changes signature to accept the real
// hook. Per rules/no-layering.md, no layered wrapper or optional
// parameter is added in advance.
//
// Caller MUST call Start before sending on SessionsIn(). Caller MUST
// call Stop before discarding the Driver.
func NewProductionDriver(logger *slog.Logger, backend IfaceBackend) *Driver {
	return NewDriver(DriverConfig{
		Logger:   logger,
		AuthHook: StubAuthHook{Logger: logger},
		Backend:  backend,
		Ops:      newPPPOps(),
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
	if cfg.AuthHook == nil {
		panic("BUG: ppp.NewDriver: AuthHook is required")
	}
	if cfg.Backend == nil {
		panic("BUG: ppp.NewDriver: Backend is required")
	}
	if cfg.Ops.setMRU == nil {
		panic("BUG: ppp.NewDriver: Ops.setMRU is required")
	}
	return &Driver{
		logger:       cfg.Logger,
		authHook:     cfg.AuthHook,
		backend:      cfg.Backend,
		ops:          cfg.Ops,
		sessionsIn:   make(chan StartSession, defaultSessionsInBuf),
		eventsOut:    make(chan Event, defaultEventsOutBuf),
		dispatchDone: make(chan struct{}),
		sessions:     make(map[sessionKey]*pppSession),
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

// StopSession terminates the session for (tunnelID, sessionID) and
// waits up to the goroutine's natural exit (close-fd unblocks read).
// Returns ErrSessionNotFound if no such session exists.
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

	s := &pppSession{
		tunnelID:     start.TunnelID,
		sessionID:    start.SessionID,
		chanFile:     newChanFileFn(start.ChanFD, "ppp.chan"),
		unitFD:       start.UnitFD,
		unitNum:      start.UnitNum,
		lnsMode:      start.LNSMode,
		maxMRU:       maxMRU,
		echoInterval: start.EchoInterval,
		echoFailures: start.EchoFailures,
		authHook:     d.authHook,
		backend:      d.backend,
		ops:          d.ops,
		eventsOut:    d.eventsOut,
		stopCh:       d.stopCh,
		done:         make(chan struct{}),
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

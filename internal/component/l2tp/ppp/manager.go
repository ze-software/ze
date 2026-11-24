// Design: docs/research/l2tpv2-ze-integration.md -- PPP driver + per-session goroutines
// Related: session.go -- pppSession struct held in the sessions map
// Related: session_run.go -- per-session goroutine main loop
// Related: auth_events.go -- AuthEvent sealed sum, AuthMethod, authResponseMsg

package ppp

import (
	"context"
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

	// ErrSessionNotFound is returned by StopSession / sessionForControl for
	// a control that does not identify this exact live worker.
	ErrSessionNotFound = errors.New("ppp: session not found")

	// ErrAuthResponsePending is returned by AuthResponse when the
	// exact request already accepted a decision; it never describes
	// another request or a future authentication round.
	ErrAuthResponsePending = errors.New("ppp: auth response already pending")

	// ErrIPResponsePending is returned by IPResponse when the
	// exact request already accepted a decision. Each NCP wait owns
	// its own buffered-one answer channel.
	ErrIPResponsePending = errors.New("ppp: ip response already pending")

	// ErrRequestExpired refuses an answer to a wait that has already ended.
	ErrRequestExpired = errors.New("ppp: provider request is no longer active")
	// ErrIPResponseFamilyMismatch refuses a decision for another NCP family.
	ErrIPResponseFamilyMismatch = errors.New("ppp: IP response family does not match its request")
)

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
// session goroutine exits). Composition MUST retain it while
// HasPendingNativeRelease reports unconfirmed native ownership.
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
	sessions map[string]*pppSession
	// owners retains each admitted lease through worker exit AND unconfirmed
	// native release, including after StopSession/ReleaseSession removes its key.
	owners              map[SessionHandle]*pppSession
	wg                  sync.WaitGroup
	nativeReleaseBudget *NativeReleaseBudget
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
		sessions:      make(map[string]*pppSession),
		owners:        make(map[SessionHandle]*pppSession),
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
	type stopOwner struct {
		session *pppSession
		retired bool
	}
	d.mu.Lock()
	owners := make([]stopOwner, 0, len(d.owners))
	for _, session := range d.owners {
		owners = append(owners, stopOwner{session: session, retired: session.nativeRetired})
	}
	d.mu.Unlock()
	// Close can enter an admitted policy's bounded seal. Never hold d.mu
	// across it or serialize independent owners' remote timeout budgets.
	// This finite shutdown fanout joins here; no background retry worker exists.
	var closing sync.WaitGroup
	for _, owner := range owners {
		closing.Go(func() {
			session := owner.session
			session.closeFiles()
			// Active workers own exactly one reconciliation at run exit.
			// Only owners ALREADY retired at the snapshot need Stop's retry.
			if owner.retired {
				if err := d.retrySessionOwner(context.Background(), session); err != nil {
					d.reportNativeReleaseFailure(session, err)
				}
			}
		})
	}
	closing.Wait()

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
// On accept=true for family=ipv4, Local MUST be a non-zero IPv4 address.
// Peer MUST be a non-zero IPv4 address unless PeerNegotiated is true
// (RFC 2865 Section 5.8, Framed-IP-Address == 0xFFFFFFFF), in which case
// Peer is zero and IPCP accepts a valid unicast IPv4 address proposed by
// the peer. DNSPrimary / DNSSecondary are optional.
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
	AllocationID   uint64
	Local          netip.Addr
	Peer           netip.Addr
	PeerNegotiated bool
	// CheckPeerIPv4, when non-nil alongside PeerNegotiated, performs a
	// read-only check on whether a peer-proposed IPv4 address is eligible
	// (outside configured pools and not held by another live session) without
	// reserving it. Used while evaluating requests and building Configure-Rejects.
	CheckPeerIPv4 func(netip.Addr) bool
	// AdmitPeerIPv4, when non-nil alongside PeerNegotiated, atomically checks
	// and exclusively reserves a peer-proposed IPv4 address once IPCP is about
	// to send Configure-Ack. It MUST be idempotent for repeated proposals of
	// the same address by the same session and safe for concurrent use.
	AdmitPeerIPv4 func(netip.Addr) bool
	// ReleasePeerIPv4, when non-nil alongside PeerNegotiated, releases any
	// address reserved via AdmitPeerIPv4 when IPCP leaves Opened (TLD) or
	// LCP resets. It MUST be idempotent and safe for concurrent use.
	ReleasePeerIPv4  func()
	DNSPrimary       netip.Addr
	DNSSecondary     netip.Addr
	PeerInterfaceID  [8]byte
	HasPeerInterface bool
}

// IPResponse delivers the external handler's IP decision to the
// per-session goroutine. Returns ErrSessionNotFound if no session
// matches the exact admitted control -- this covers invalid, foreign,
// canceled and already torn-down workers.
//
// Safe for concurrent use. Only the actual emitted request permits an answer.
// Expired waits return ErrRequestExpired; a duplicate returns ErrIPResponsePending
// and another family returns ErrIPResponseFamilyMismatch.
func (d *Driver) IPResponse(request EventIPRequest, args IPResponseArgs) error {
	_, err := d.liveSession(request.Control)
	if err != nil {
		return err
	}
	msg := ipResponseMsg{
		accept:           args.Accept,
		family:           args.Family,
		reason:           args.Reason,
		local:            args.Local,
		peer:             args.Peer,
		peerNegotiated:   args.PeerNegotiated,
		checkPeerIPv4:    args.CheckPeerIPv4,
		admitPeerIPv4:    args.AdmitPeerIPv4,
		releasePeerIPv4:  args.ReleasePeerIPv4,
		dnsPrimary:       args.DNSPrimary,
		dnsSecondary:     args.DNSSecondary,
		peerInterfaceID:  args.PeerInterfaceID,
		hasPeerInterface: args.HasPeerInterface,
	}
	return request.answer(msg)
}

// AuthResponse delivers the external handler's auth decision to the
// per-session goroutine. Returns ErrSessionNotFound if no session
// matches the exact admitted control -- this covers invalid, foreign,
// canceled and already torn-down workers.
//
// Safe for concurrent use. Only the actual emitted request permits an answer.
// Expired waits return ErrRequestExpired; an already answered request returns
// ErrAuthResponsePending. Neither result enqueues into a different wait.
func (d *Driver) AuthResponse(request EventAuthRequest, accept bool, message string, authResponseBlob []byte) error {
	_, err := d.liveSession(request.Control)
	if err != nil {
		return err
	}
	msg := authResponseMsg{
		accept:           accept,
		message:          message,
		authResponseBlob: authResponseBlob,
	}
	return request.answer(msg)
}

// StopSession terminates only the exact admitted worker and waits for its
// goroutine to exit. An invalid, foreign or retired control returns
// ErrSessionNotFound; it never selects a successor by ID or lease.
//
// Closes the per-session sessStop first so that a goroutine parked
// in an authentication wait unblocks; closing
// chanFile alone would only unblock readFrames, which is not running
// yet during the initial auth phase on the proxy-LCP short-circuit
// path.
//
// Idempotent: calling StopSession a second time returns
// ErrSessionNotFound (the first call removed it from the map).
func (d *Driver) StopSession(control SessionControl) error {
	d.mu.Lock()
	s, ok := d.sessionForControlLocked(control)
	if !ok {
		d.mu.Unlock()
		return ErrSessionNotFound
	}
	delete(d.sessions, s.subscriberID)
	d.mu.Unlock()

	control.Cancel()
	s.closeFiles()
	<-s.done
	return nil
}

// sessionForControl returns the exact worker's snapshot, or false when its
// control no longer identifies an active admission.
func (d *Driver) sessionForControl(control SessionControl) (sessionInfo, bool) {
	s, err := d.liveSession(control)
	if err != nil {
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
				d.closeRefusedTransfer(&start)
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
// canonical SubscriberID arrives, or if the StartSession contains
// missing session ownership or frame delivery.
func (d *Driver) spawnSession(start *StartSession) {
	// Only the constructor's immutable binding can classify an existing key.
	// A malformed raw ID must not preserve an unrelated pending admission.
	k := start.Attempt.SubscriberID()
	comparableOwner := !sessionResourceMissing(start.Dataplane) && reflect.ValueOf(start.Dataplane).Comparable()
	// Detect an existing lease before validating borrowed resources or LCP:
	// a malformed requeue has not transferred that live owner again.
	d.mu.Lock()
	existingKey, existingOwner := d.existingTransferLocked(k, start.Dataplane, comparableOwner)
	stopped := d.stopped
	d.mu.Unlock()
	if stopped {
		// The same exact handoff observer must resolve even when an already
		// owned lease is refused during shutdown; this helper never closes it.
		d.closeRefusedTransfer(start)
		return
	}
	ownershipConflict := existingKey || existingOwner
	if ownershipConflict {
		d.rejectExistingTransfer(start, existingKey)
		return
	}
	// Missing ownership or missing control delivery is an invalid transfer;
	// release any supplied owner before publishing its refusal.
	if sessionResourceMissing(start.Dataplane) {
		d.closeRefusedTransfer(start)
		d.emitRejection(start, "missing session data plane")
		return
	}
	if !comparableOwner {
		d.closeRefusedTransfer(start)
		d.emitRejection(start, "session data-plane owner cannot be compared for safe lease admission")
		return
	}
	if start.SubscriberID == "" {
		d.closeRefusedTransfer(start)
		d.emitRejection(start, "missing canonical subscriber identity")
		return
	}
	if !start.Attempt.matches(start.SubscriberID, start.Dataplane) {
		d.closeRefusedTransfer(start)
		d.emitRejection(start, "session attempt does not bind the subscriber identity and data-plane lease")
		return
	}
	if sessionResourceMissing(start.Frames) {
		d.closeRefusedTransfer(start)
		d.emitRejection(start, "missing frame channel")
		return
	}
	if sessionResourceMissing(start.Frames.Control()) {
		d.closeRefusedTransfer(start)
		d.emitRejection(start, "missing control stream")
		return
	}
	claimed, canceled := start.Attempt.admissionState()
	if claimed {
		// A prior claimant owns its lease even in another Driver. A replay
		// never closes it or rebinds the original rendezvous.
		d.sendRejection(start, ErrSessionAttemptUsed.Error(), true)
		return
	}
	if canceled {
		d.closeRefusedTransfer(start)
		d.emitRejection(start, ErrSessionAttemptCanceled.Error())
		return
	}
	maxMRU := start.MaxMRU
	if err := start.LCP.Validate(); err != nil {
		d.closeRefusedTransfer(start)
		d.emitRejection(start, err.Error())
		return
	}
	if maxMRU == 0 {
		maxMRU = MaxFrameLen
	}
	// RFC 791 Section 3.2 and RFC 5072 Section 2: admit only protocol-capable MRUs.
	if err := ValidateNetworkMRU(maxMRU, !start.DisableIPv6CP); err != nil {
		d.closeRefusedTransfer(start)
		d.emitRejection(start, err.Error())
		return
	}
	// Cache exact native report ownership before taking the Driver lock.
	// Neither a delayed close report nor a retry may reread a successor's
	// recycled handle metadata. The factory has already bound/stamped birth.
	identity := start.Dataplane.Identity()
	var nativeBirthTag string
	if native, ok := start.Dataplane.(NativeReleaseHandle); ok {
		nativeBirthTag = native.NativeReleaseBirthTag()
	}
	d.mu.Lock()
	existingKey, existingOwner = d.existingTransferLocked(k, start.Dataplane, true)
	if d.stopped {
		d.mu.Unlock()
		d.closeRefusedTransfer(start)
		return
	}
	ownershipConflict = existingKey || existingOwner
	if ownershipConflict {
		d.mu.Unlock()
		d.rejectExistingTransfer(start, existingKey)
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
		l2tpBirth:            start.L2TPBirth,
		subscriberID:         start.SubscriberID,
		attempt:              start.Attempt,
		driver:               d,
		nativeBirthTag:       nativeBirthTag,
		nativeIdentity:       identity,
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
		stopCh:               d.stopCh,
		sessStop:             make(chan struct{}),
		done:                 make(chan struct{}),
		logger: d.logger.With(
			"tunnel-id", start.TunnelID,
			"session-id", start.SessionID,
			"interface", identity.InterfaceName,
		),
	}
	s.control = SessionControl{driver: d, worker: s}
	if err := start.Attempt.claim(s, true); err != nil {
		d.mu.Unlock()
		if !errors.Is(err, ErrSessionAttemptUsed) {
			d.closeRefusedTransfer(start)
		}
		d.sendRejection(start, err.Error(), errors.Is(err, ErrSessionAttemptUsed))
		return
	}
	d.sessions[k] = s
	d.owners[start.Dataplane] = s
	d.wg.Add(1)
	d.mu.Unlock()

	go func() {
		defer d.wg.Done()
		defer close(s.done)
		defer func() {
			// Publish retirement before retrying the native gate. An End ACK
			// that arrived while run still owned I/O is reconciled here.
			d.mu.Lock()
			s.nativeRetired = true
			if d.sessions[k] == s {
				delete(d.sessions, k)
			}
			d.mu.Unlock()
			if err := s.control.RetryNativeRelease(); err != nil {
				d.reportNativeReleaseFailure(s, err)
			}
		}()
		s.run(start)
	}()
}

// existingTransferLocked uses the reverse index only for comparable owners.
// Caller MUST hold d.mu. The key and owner can belong to different sessions.
func (d *Driver) existingTransferLocked(key string, owner SessionHandle, comparableOwner bool) (bool, bool) {
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
func (d *Driver) rejectExistingTransfer(start *StartSession, existingKey bool) {
	d.closeRefusedTransfer(start)
	reason := "session data-plane owner is already admitted"
	if existingKey {
		reason = "duplicate canonical subscriber identity"
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
// The refused request has no admitted Control. Its Attempt observer says
// whether an incoming lease was acquired and its exact physical release was
// confirmed; a failed Close remains strongly retained for retry.
func (d *Driver) emitRejection(start *StartSession, reason string) {
	// Only the original Attempt can establish prior admission, never raw labels.
	_, alreadyAdmitted := start.Attempt.Control()
	d.sendRejection(start, reason, alreadyAdmitted)
}

func (d *Driver) sendRejection(start *StartSession, reason string, existingSession bool) {
	select {
	case d.eventsOut <- EventSessionRejected{
		Birth:           start.L2TPBirth,
		SubscriberID:    start.SubscriberID,
		Attempt:         start.Attempt,
		TunnelID:        start.TunnelID,
		SessionID:       start.SessionID,
		Reason:          reason,
		ExistingSession: existingSession,
	}:
	case <-d.stopCh:
	}
}

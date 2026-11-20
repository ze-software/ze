// Design: docs/architecture/l2tp/bng-5-pppoe.md -- PPPoE subsystem lifecycle
// Related: config.go -- Parameters consumed at Start
// Related: service.go -- PublishService / LookupService

package pppoe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sync"
	"syscall"

	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/component/traffic"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/pacer"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

var errPppoeSubsystemAlreadyStarted = errors.New("pppoe: subsystem already started")

var _ ze.Subsystem = (*Subsystem)(nil)

const SubsystemName = "pppoe"

// configRoot is the top-level configuration container the PPPoE access
// concentrator reads its settings from.
const configRoot = "pppoe"

// Subsystem is the ze.Subsystem implementation for PPPoE.
type Subsystem struct {
	params Parameters
	logger *slog.Logger

	lifecycleMu sync.Mutex
	// retainedStop forbids treating a joined runtime as running after refused cleanup failed.
	retainedStop bool
	// dispatchMu is held by the discovery reader for each frame it dispatches,
	// and by Reload while it swaps the server map (dispatchFrame). Order:
	// lifecycleMu, then dispatchMu, then mu.
	dispatchMu sync.Mutex
	// discoveryPaused is read/written only under dispatchMu during reload drain.
	discoveryPaused bool
	mu              sync.Mutex
	started         bool
	discFD          int
	servers         map[int]*InterfaceServer
	accesses        *accessConfiguration
	backend         dataplane.Backend
	pppDriver       *ppp.Driver
	readDone        chan struct{}
	eventDone       chan struct{}
	// relayWake is a coalesced hint; tables own every pending end.
	relayWake  chan struct{}
	drainDones []<-chan struct{}
	bus        ze.EventBus
	// routeObserver is owned by the running access lifecycle under lifecycleMu.
	// It MUST outlive every terminal subscriber notice, including stop retry.
	routeObserver *l2tp.SubscriberRouteSubscription

	// stop interrupts paced retries and is checked after each receive timeout.
	// discFD remains open until discovery and PPP event consumers have joined.
	stop  chan struct{}
	pacer pacer.Pacer // paces discoveryReader's retries after a failing read; owned solely by its own goroutine

	pendingAuth sync.Map // pendingAuthKey -> pendingAuthInfo
	// realmRelay holds the realm relays in flight (realm_relay.go). Set by
	// startLocked with the PPP driver, cleared by stopLocked after the
	// drains and the event consumer returned.
	realmRelay *realmRelay
}

type pendingAuthKey struct {
	subscriberID string
	attempt      ppp.SessionAttempt
}

type pendingAuthInfo struct {
	username   string
	authMethod string
}

// NewSubsystem constructs a PPPoE subsystem from parsed Parameters.
func NewSubsystem(p Parameters) *Subsystem {
	return &Subsystem{
		params:    p,
		logger:    slogutil.Logger(SubsystemName),
		discFD:    -1,
		relayWake: make(chan struct{}, 1),
	}
}

func (s *Subsystem) Name() string { return SubsystemName }

// Start implements ze.Subsystem.
func (s *Subsystem) Start(ctx context.Context, bus ze.EventBus, _ ze.ConfigProvider) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return errPppoeSubsystemAlreadyStarted
	}

	s.bus = bus
	if err := s.startLocked(ctx); err != nil {
		return err
	}
	s.started = true
	return nil
}

// startLocked brings up discovery for s.params: the discovery socket, the PPP
// driver, one InterfaceServer per resolved access interface, and the workers
// that serve them. A disabled configuration, or one naming no interface,
// starts nothing and answers nil: the subsystem is then started and idle, and
// a Reload can bring it up. A failed access preparation refuses the start and
// releases every unpublished lease before opening discovery.
//
// MUST be called with lifecycleMu and mu held, on a subsystem that is not
// running (readDone nil). Start calls it at boot, and Reload when a reload
// enables an idle subsystem.
func (s *Subsystem) startLocked(ctx context.Context) (err error) {
	if !s.params.Enabled {
		s.logger.Info("PPPoE subsystem disabled in config, skipping start")
		return nil
	}
	if len(s.params.Interfaces) == 0 {
		s.logger.Warn("PPPoE subsystem enabled but no interfaces configured, skipping start")
		return nil
	}

	backend, err := selectedSessionBackend()
	if err != nil {
		return err
	}
	accesses, err := prepareAccessConfiguration(ctx, backend, s.params.Interfaces, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, closeAccessConfiguration(accesses.accesses, nil))
		}
	}()
	fd, err := openDiscoverySocketInNamespace(accesses.namespace)
	if err != nil {
		return opererr.Wrap(err, "PPPoE discovery socket could not be opened", "The selected access control devices cannot receive discovery traffic.", "ze doctor")
	}
	defer func() {
		if err != nil {
			closeDiscoverySocket(fd)
		}
	}()
	if err := SetRecvTimeout(fd, 100*time.Millisecond); err != nil {
		return fmt.Errorf("pppoe: discovery receive timeout: %w", err)
	}
	driver := ppp.NewProductionDriver(s.logger.With("component", "ppp"))
	if err := driver.Start(); err != nil {
		return fmt.Errorf("pppoe: start PPP driver: %w", err)
	}
	defer func() {
		if err != nil {
			driver.Stop()
		}
	}()

	// Asks for the discovery-refusal counters (metrics.go). The metrics
	// registry can arrive after this method returns, so the call registers a
	// hook rather than reading a registry.
	registerDiscoveryMetrics()

	servers := make(map[int]*InterfaceServer, len(s.params.Interfaces))
	for i := range s.params.Interfaces {
		ic := &s.params.Interfaces[i]
		srv, srvErr := s.newInterfaceServer(ic, &s.params, backend, accesses.accesses[ic.Name])
		if srvErr != nil {
			return srvErr
		}
		srv.discFD = fd
		srv.pppDriver = driver
		servers[srv.ifIndex] = srv
		s.logger.Info("PPPoE interface configured", "interface", ic.Name, "ifindex", srv.ifIndex)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.servers = servers
	s.accesses = accesses
	s.backend = backend
	s.discFD = fd
	s.pppDriver = driver
	s.routeObserver = l2tp.StartSubscriberRouteObserver(
		s.logger, s.bus, subscriber.AccessPPPoE, routeProtocol, routeChange,
	)

	s.readDone = make(chan struct{})
	s.stop = make(chan struct{})
	go s.discoveryReader()

	if s.pppDriver != nil {
		authH := subscriber.GetAuthHandler()
		poolH := subscriber.GetPoolHandler()
		if authH == nil {
			s.logger.Warn("pppoe: no auth handler registered yet; a session arriving before one registers is refused")
		}
		if poolH == nil {
			s.logger.Error("pppoe: no pool handler registered; all IP requests will be rejected")
		}
		s.realmRelay = newRealmRelay(s.logger, s.pppDriver, s.realmSubscriber, s.endRelayedSession, s.releaseRelayedSession)
		s.drainDones = append(s.drainDones,
			startPPPoEAuthDrain(s.logger, s.pppDriver, subscriber.GetAuthHandler, s.bus, &s.pendingAuth, s.realmRelay),
			startPPPoEPoolDrain(s.logger, s.pppDriver, poolH, s.bus),
		)
		s.eventDone = make(chan struct{})
		go s.eventConsumer()
	}

	PublishService(s)
	return nil
}

// Stop implements ze.Subsystem.
func (s *Subsystem) Stop(ctx context.Context) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	if !s.started {
		return nil
	}
	return s.stopLocked(ctx)
}

// stopLocked stops discovery and the PPP driver and joins every worker
// startLocked started, leaving the subsystem not started. Remaining published
// admissions receive PADT while discovery and their access generation live.
// MUST be called with lifecycleMu held and mu not held,
// because the workers it joins take mu.
func (s *Subsystem) stopLocked(ctx context.Context) error {
	if s.retainedStop && s.pppDriver != nil {
		if err := s.pppDriver.RetryPendingNativeRelease(ctx); err != nil {
			return err
		}
	}
	PublishService(nil)
	s.logger.Info("PPPoE subsystem stopping")

	if s.stop != nil {
		select {
		case <-s.stop:
		default:
			close(s.stop)
		}
	}
	if s.readDone != nil {
		<-s.readDone
	}

	if s.pppDriver != nil {
		s.pppDriver.Stop()
	}
	if s.eventDone != nil {
		<-s.eventDone
	}

	for _, done := range s.drainDones {
		<-done
	}
	s.drainDones = nil
	if s.realmRelay != nil {
		s.realmRelay.stopAll()
		s.realmRelay = nil
	}
	// Service relays have no PPP worker to release their admissions. Local
	// PPP workers are joined now, so their final writes have finished too.
	if err := s.closeAdmissions(ctx); err != nil {
		s.retainedStop = true
		return err
	}
	if s.routeObserver != nil {
		s.routeObserver.Stop()
		s.routeObserver = nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.discFD >= 0 {
		closeDiscoverySocket(s.discFD)
		s.discFD = -1
	}
	var accessErr error
	if s.accesses != nil {
		accessErr = closeAccessConfiguration(s.accesses.accesses, nil)
	}
	s.accesses = nil
	s.backend = nil

	s.pppDriver = nil
	s.readDone = nil
	s.eventDone = nil
	s.stop = nil
	s.servers = nil
	s.started = false
	s.retainedStop = false
	return accessReleaseError(accessErr)
}

// selectedSessionBackend refuses unavailable selections; the access
// concentrator never loads a default interface backend or tries a fallback.
func selectedSessionBackend() (dataplane.Backend, error) {
	backend, err := dataplane.LookupSessionDataplane(iface.ActiveBackendName())
	if err != nil {
		return nil, opererr.Wrap(err, "The selected interface backend cannot terminate PPPoE sessions",
			"PPPoE requires a registered session data plane for the active interface backend.", "ze doctor")
	}
	return backend, nil
}

// discoveryReader is the one goroutine that reads the shared AF_PACKET
// discovery socket for every configured access interface (opened once in
// Start; readDiscoveryFrame dispatches by ifindex from recvfrom, see
// bng-5-pppoe.md). A read error that is neither "socket closed" nor
// recovered by the next attempt is logged, counted on
// ze_pppoe_discovery_read_errors_total (metrics.go), and paced by s.pacer,
// so a discovery socket that fails forever costs a bounded slice of a core
// rather than all of it -- and because this one goroutine dispatches for
// every interface, a delay here is a delay on discovery for all of them,
// which is why the pacer's ceiling is short. The pacer's wait observes
// s.stop, closed by Stop before joining workers. The receive timeout bounds
// cancellation while the socket is idle; its descriptor remains valid until
// the reader and PPP event consumer have both returned.
func (s *Subsystem) discoveryReader() {
	defer close(s.readDone)

	buf := make([]byte, EthMaxLen)
	for {
		select {
		case <-s.stop:
			return
		default:
		}
		n, ifindex, err := readDiscoveryFrame(s.discFD, buf)
		if err != nil {
			if errors.Is(err, errSocketClosed) {
				return
			}
			if errors.Is(err, syscall.EAGAIN) {
				continue
			}
			// Not closed, so this is an error the loop cannot classify:
			// it may clear on the next read or it may persist. Log it
			// and count it before pacing the retry, so a socket that
			// never recovers is visible in the log and on the counter
			// rather than only in CPU use.
			s.logger.Debug("pppoe: discovery read error", "error", err)
			countDiscoveryReadError()
			if s.pacer.Wait(s.stop) {
				return
			}
			continue
		}
		s.pacer.Succeed()
		if n < MinDiscFrame {
			continue
		}
		s.dispatchFrame(ifindex, buf[:n])
	}
}

// dispatchFrame hands one discovery frame to the server of the interface it
// arrived on. It holds dispatchMu across the lookup and the handler, so a
// Reload that swaps or removes a server waits for the frame in flight: no
// session is admitted on an interface after Reload has closed it, and no
// server is copied while the reader writes its admission state.
func (s *Subsystem) dispatchFrame(ifindex int, frame []byte) {
	s.dispatchMu.Lock()
	defer s.dispatchMu.Unlock()
	if s.discoveryPaused {
		return
	}

	s.mu.Lock()
	srv := s.servers[ifindex]
	s.mu.Unlock()
	if srv == nil {
		return
	}
	s.dispatchDiscovery(srv, frame)
}

func pppoeSessionID(ifindex int, sid uint16) string {
	return subscriber.SessionID(subscriber.AccessPPPoE, uint64(ifindex), sid)
}

func (s *Subsystem) eventConsumer() {
	defer close(s.eventDone)

	// The driver closes its events only after discovery and PPP workers stop.
	// Service callbacks retain work in the bounded access tables.
	for {
		select {
		case ev, open := <-s.pppDriver.EventsOut():
			if !open {
				s.drainServiceRelayEnds()
				return
			}
			s.handlePPPEvent(ev)
		case <-s.relayWake:
			s.drainServiceRelayEnds()
		}
	}
}

// handlePPPEvent applies one PPP driver lifecycle event: it updates the
// shared subscriber registry and publishes the matching event on the
// subscriber namespace, where the address pool, accounting and shaping
// consumers read it. eventConsumer calls it for every event the driver
// produces, one at a time.
func (s *Subsystem) handlePPPEvent(ev ppp.Event) {
	switch e := ev.(type) {
	case ppp.EventLCPUp:
		if !s.liveAdmission(e.Control, e.SessionID) {
			e.Control.Cancel()
		}
	case ppp.EventLCPDown:
		_, session, found := s.sessionForControl(e.Control, e.SessionID)
		if !found {
			e.Control.Cancel()
			return
		}
		if e.NetworkPhase {
			s.withdrawSubscriber(e.Control, session, e.NetworkLifetime, e.Reason)
		}
		if !s.liveAdmission(e.Control, e.SessionID) {
			e.Cancel()
			return
		}
		e.Acknowledge()
	case ppp.EventSessionUp:
		s.onSessionUp(e)
	case ppp.EventSessionIPAssigned:
		s.onSessionIPAssigned(e)
	case ppp.EventSessionIPReleased:
		s.onSessionIPReleased(e)
	case ppp.EventSessionDown:
		s.onSessionDown(e)
	case ppp.EventSessionRejected:
		if e.ExistingSession {
			return
		}
		server, session, found := s.sessionForAttempt(e.Attempt, e.SessionID)
		if !found {
			return
		}
		s.finishRejectedAdmission(server, session, e)
	case ppp.EventEchoRTT:
		// PPPoE keeps no CQM state: only the L2TP reactor relays echo
		// round-trip times. One arrives per LCP Echo-Reply, so it must
		// never reach the defect arm below.
	default:
		s.reportUnhandledPPPEvent(ev)
	}
}

// pppEventReportSubject is the runtime doctor subject of an unhandled event.
const pppEventReportSubject = "ppp event dispatch"

// reportUnhandledPPPEvent reports a ppp.Event type handlePPPEvent has no case
// for, then the event is ignored. Only a Ze defect produces one: the sealed
// ppp.Event set gained a type and this switch did not.
func (s *Subsystem) reportUnhandledPPPEvent(ev ppp.Event) {
	const problem = "PPPoE ignored a PPP event it has no handler for"
	const fix = "ze show doctor"
	eventType := pppEventTypeName(ev)
	s.logger.Warn(problem, "code", diagnostic.CodeDoctorInternalDefect, "fix", fix, "type", eventType)
	report.RaiseWarningFix(SubsystemName, diagnostic.CodeDoctorInternalDefect, pppEventReportSubject,
		problem, fix, map[string]any{"type": eventType})
}

// pppEventTypeName names the event's Go type, as the %T verb does.
func pppEventTypeName(ev ppp.Event) string {
	if ev == nil {
		return "<nil>"
	}
	return reflect.TypeOf(ev).String()
}

// onSessionIPReleased removes one family from the registered session after
// its NCP left Opened, then publishes the release, which the route observer
// reads to withdraw that family's routes. The other family and the session
// stay. The next assignment of the family sets the fields again.
func (s *Subsystem) onSessionIPReleased(e ppp.EventSessionIPReleased) {
	if !e.Control.Active() {
		return
	}
	family := e.Family.NoticeFamily()
	sess, ok := s.subscriberSession(e.Control, e.SessionID)
	if !ok {
		return
	}
	subevents.ClearReleasedFamily(&sess, family)
	subscriber.DefaultRegistry.Add(&sess)
	if s.bus == nil {
		return
	}
	if _, err := subevents.SessionIPReleased.Emit(s.bus, &subevents.SessionIPReleasedPayload{
		Session:         sess,
		Family:          family,
		NetworkLifetime: e.NetworkLifetime,
	}); err != nil {
		s.logger.Warn("pppoe: subscriber session-ip-released emit failed",
			"code", diagnostic.CodeDoctorInternalDefect, "fix", "ze show doctor", "error", err)
	}
}

// subscriberSession preserves completed NCPs while the remaining NCPs run.
func (s *Subsystem) subscriberSession(control ppp.SessionControl, sid uint16) (subscriber.Session, bool) {
	server, session, live := s.sessionForControl(control, sid)
	if !live {
		return subscriber.Session{}, false
	}
	snap, live := server.sessions.snapshotAdmission(session)
	if !live || snap.State == StateTeardown {
		return subscriber.Session{}, false
	}
	ifindex := snap.AccessIfIndex
	id := snap.SubscriberID
	sess, found := subscriber.DefaultRegistry.Get(id)
	if !found {
		sess = subscriber.Session{
			ID: id, AccessType: subscriber.AccessPPPoE, State: subscriber.StateConfiguring,
			PPPoESID: sid, AccessIfIndex: ifindex, AcctSessionID: id,
		}
	}
	sess.MAC = snap.MAC
	sess.AccessInterface = snap.IfName
	sess.ServiceName = snap.ServiceName
	sess.PppInterface = snap.Identity.InterfaceName
	if val, ok := s.pendingAuth.Load(pendingAuthFor(control)); ok {
		info, valid := val.(pendingAuthInfo)
		if !valid {
			s.logger.Error("pppoe: invalid pending authentication state", "ifindex", ifindex, "session", sid)
			return subscriber.Session{}, false
		}

		if !found {
			sess.Username = info.username
		}
		sess.AuthMethod = info.authMethod
	}
	return sess, true
}

// onSessionUp records a session that has completed LCP, authentication and
// every enabled NCP, then publishes it.
func (s *Subsystem) onSessionUp(e ppp.EventSessionUp) {
	if !e.Control.Active() {
		e.Control.Cancel()
		return
	}
	sess, found := s.subscriberSession(e.Control, e.SessionID)
	if !found {
		e.Control.Cancel()
		return
	}
	wasActive := sess.State == subscriber.StateActive
	sess.State = subscriber.StateActive
	if !wasActive {
		sess.ActivatedAt = time.Now()
	}
	s.pendingAuth.Delete(pendingAuthFor(e.Control))
	// RFC 2865 Section 5.11: the Access-Accept Filter-Id carries the
	// subscriber's rates. The RADIUS handler stores the profile under the same
	// canonical subscriber ID, so a PPPoE subscriber's
	// authorized rates reach the shaper exactly as an L2TP one's do. Without
	// this the two rate fields had no producer at all and every PPPoE session
	// was shaped at the configured default whatever the RADIUS server answered.
	if meta := l2tp.LoadSessionMetadata(sess.ID); meta != nil && meta.FilterID != "" {
		if download, upload, ok := traffic.ParseFilterIDRate(meta.FilterID); ok {
			sess.DownloadRate = download
			sess.UploadRate = upload
		}
	}

	sess.AcctSessionID = sess.ID
	subscriber.DefaultRegistry.Add(&sess)
	if !wasActive {
		subscriber.RecordSessionUp(subscriber.AccessPPPoE)
	}

	if sh := subscriber.GetShaperHandler(); sh != nil && sess.PppInterface != "" {
		sh(sess.PppInterface, sess.DownloadRate, sess.UploadRate)
	}

	if s.bus == nil {
		return
	}
	if _, err := subevents.SessionUp.Emit(s.bus, &subevents.SessionUpPayload{
		Session: sess,
	}); err != nil {
		s.logger.Warn("pppoe: subscriber session-up emit failed", "error", err)
	}
}

// onSessionIPAssigned adds the addresses one completed NCP negotiated to the
// registered session, then publishes the updated snapshot.
// The consumer MUST acknowledge after synchronous publication completes, before
// PPP can enable forwarding.
func (s *Subsystem) onSessionIPAssigned(e ppp.EventSessionIPAssigned) {
	if !e.Control.Active() {
		e.Control.Cancel()
		return
	}

	if e.Peer.IsValid() && !e.Peer.Is4() || !e.Peer.IsValid() && e.InterfaceID == [8]byte{} {
		s.logger.Warn("pppoe: invalid IP-assigned event", "address", e.Peer, "session", e.SessionID)
		e.Cancel()
		return
	}
	sess, ok := s.subscriberSession(e.Control, e.SessionID)
	if !ok {
		e.Cancel()
		return
	}
	sess.Username = e.Username

	if e.Peer.Is4() {
		sess.IPv4Addr = e.Peer
		sess.DNSPrimary = e.DNSPrimary
		sess.DNSSecondary = e.DNSSecondary
	}
	if e.InterfaceID != [8]byte{} {
		sess.IPv6InterfaceID = e.InterfaceID
	}
	subscriber.DefaultRegistry.Add(&sess)

	if s.bus != nil {
		if _, err := subevents.SessionIPAssigned.Emit(s.bus, &subevents.SessionIPAssignedPayload{
			Session:         sess,
			Address:         e.Peer,
			FramedRoutes:    e.FramedRoutes,
			NetworkLifetime: e.NetworkLifetime,
		}); err != nil {
			s.logger.Warn("pppoe: subscriber session-ip-assigned emit failed", "error", err)
		}
	}
	if !s.liveAdmission(e.Control, e.SessionID) {
		e.Cancel()
		return
	}
	e.Acknowledge()
}

// onSessionDown publishes the teardown and sends the PADT that ends the
// PPPoE session on the wire.
//
// The event is published even when the registry holds no session: an address
// can be allocated before its NCP completes.
func (s *Subsystem) onSessionDown(e ppp.EventSessionDown) {
	server, session, found := s.sessionForControl(e.Control, e.SessionID)
	if !found {
		e.Control.Cancel()
		return
	}
	if !server.sessions.publishPPPTerminal(session, e.Control.Attempt(), func() {
		if s.realmRelay != nil {
			s.realmRelay.subscriberGone(e.Control.Attempt())
		}
		s.withdrawSubscriber(e.Control, session, e.NetworkLifetime, e.Reason)
	}) {
		return
	}
	server.handleSessionDown(session)
}

// withdrawSubscriber releases a network lifetime without closing its transport.
// The caller MUST supply the admission already matched to the original control.
func (s *Subsystem) withdrawSubscriber(control ppp.SessionControl, session *Session, network l2tpevents.NetworkLifetime, reason string) {
	s.pendingAuth.Delete(pendingAuthFor(control))
	s.withdrawSubscriberIdentity(session, network, reason)
}

// withdrawSubscriberIdentity uses only an identity retained by the exact
// admission. Rejected attempts have no admitted control.
func (s *Subsystem) withdrawSubscriberIdentity(session *Session, network l2tpevents.NetworkLifetime, reason string) {
	id := session.SubscriberID
	sess, found := subscriber.DefaultRegistry.Get(id)
	if !found {
		sess = subscriber.Session{
			ID: id, AccessType: subscriber.AccessPPPoE,
			PPPoESID: session.SID, AccessIfIndex: session.AccessIfIndex,
		}
	}
	wasActive := sess.State == subscriber.StateActive
	sess.State = subscriber.StateTerminating
	subscriber.DefaultRegistry.Remove(id)
	if wasActive {
		subscriber.RecordSessionDown(subscriber.AccessPPPoE)
	}
	if s.bus != nil {
		if _, err := subevents.SessionDown.Emit(s.bus, &subevents.SessionDownPayload{
			Session:         sess,
			Reason:          reason,
			NetworkLifetime: network,
		}); err != nil {
			s.logger.Warn("pppoe: subscriber session-down emit failed", "error", err)
		}
	}
	l2tp.ClearSessionMetadata(sess.ID)
}

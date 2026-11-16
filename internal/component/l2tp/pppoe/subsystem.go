// Design: docs/architecture/l2tp/bng-5-pppoe.md -- PPPoE subsystem lifecycle
// Related: config.go -- Parameters consumed at Start
// Related: service.go -- PublishService / LookupService

package pppoe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"syscall"

	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/component/traffic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/pacer"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

var errPppoeSubsystemAlreadyStarted = errors.New("pppoe: subsystem already started")

var _ ze.Subsystem = (*Subsystem)(nil)

const SubsystemName = "pppoe"

// Subsystem is the ze.Subsystem implementation for PPPoE.
type Subsystem struct {
	params Parameters
	logger *slog.Logger

	lifecycleMu sync.Mutex
	// dispatchMu is held by the discovery reader for each frame it dispatches,
	// and by Reload while it swaps the server map (dispatchFrame). Order:
	// lifecycleMu, then dispatchMu, then mu.
	dispatchMu sync.Mutex
	mu         sync.Mutex
	started    bool
	discFD     int
	servers    map[int]*InterfaceServer
	accesses   *accessConfiguration
	backend    dataplane.Backend
	pppDriver  *ppp.Driver
	readDone   chan struct{}
	eventDone  chan struct{}
	// relayWake is a coalesced hint; tables own every pending end.
	relayWake  chan struct{}
	drainDones []<-chan struct{}
	bus        ze.EventBus

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
	ifindex   int
	sessionID uint16
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
		s.realmRelay = newRealmRelay(s.logger, s.pppDriver, s.realmSubscriber, s.endRelayedSession)
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
func (s *Subsystem) Stop(_ context.Context) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	if !s.started {
		return nil
	}
	return s.stopLocked()
}

// stopLocked stops discovery and the PPP driver and joins every worker
// startLocked started, leaving the subsystem not started. Remaining published
// admissions receive PADT while discovery and their access generation live.
// MUST be called with lifecycleMu held and mu not held,
// because the workers it joins take mu.
func (s *Subsystem) stopLocked() error {
	PublishService(nil)
	s.logger.Info("PPPoE subsystem stopping")

	if s.stop != nil {
		close(s.stop)
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
	s.closeAdmissions()
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
	case ppp.EventLCPDown:
		defer e.Acknowledge()
		if e.NetworkPhase {
			s.withdrawSubscriber(e.TunnelID, e.SessionID, e.Reason)
		}
	case ppp.EventSessionUp:
		s.onSessionUp(e)
	case ppp.EventSessionIPAssigned:
		s.onSessionIPAssigned(e)
	case ppp.EventSessionDown:
		s.onSessionDown(e)
	case ppp.EventSessionRejected:
		if e.ExistingSession {
			return
		}
		s.onSessionDown(ppp.EventSessionDown{
			TunnelID: e.TunnelID, SessionID: e.SessionID, Reason: e.Reason,
		})
	}
}

// subscriberSession preserves completed NCPs while the remaining NCPs run.
func (s *Subsystem) subscriberSession(tid, sid uint16) (subscriber.Session, bool) {
	ifindex := int(tid)
	s.mu.Lock()
	srv := s.servers[ifindex]
	s.mu.Unlock()
	if srv == nil {
		s.logger.Debug("pppoe: ignoring an event for a removed access interface", "ifindex", ifindex)
		return subscriber.Session{}, false
	}
	snap, live := srv.sessions.LookupSnapshot(sid)
	if !live {
		s.logger.Debug("pppoe: ignoring an event for a removed session", "ifindex", ifindex, "session", sid)
		return subscriber.Session{}, false
	}
	id := pppoeSessionID(ifindex, sid)
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
	if val, ok := s.pendingAuth.Load(pendingAuthKey{ifindex: ifindex, sessionID: sid}); ok {
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
	sess, found := s.subscriberSession(e.TunnelID, e.SessionID)
	if !found {
		return
	}
	wasActive := sess.State == subscriber.StateActive
	sess.State = subscriber.StateActive
	if !wasActive {
		sess.ActivatedAt = time.Now()
	}
	s.pendingAuth.Delete(pendingAuthKey{ifindex: int(e.TunnelID), sessionID: e.SessionID})
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
	defer e.Acknowledge()

	if e.Peer.IsValid() && !e.Peer.Is4() || !e.Peer.IsValid() && e.InterfaceID == [8]byte{} {
		s.logger.Warn("pppoe: invalid IP-assigned event", "address", e.Peer, "session", e.SessionID)
		return
	}
	sess, ok := s.subscriberSession(e.TunnelID, e.SessionID)
	if !ok {
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

	if s.bus == nil {
		return
	}
	if _, err := subevents.SessionIPAssigned.Emit(s.bus, &subevents.SessionIPAssignedPayload{
		Session: sess,
	}); err != nil {
		s.logger.Warn("pppoe: subscriber session-ip-assigned emit failed", "error", err)
	}
}

// onSessionDown publishes the teardown and sends the PADT that ends the
// PPPoE session on the wire.
//
// The event is published even when the registry holds no session: an address
// can be allocated before its NCP completes.
func (s *Subsystem) onSessionDown(e ppp.EventSessionDown) {
	if s.realmRelay != nil {
		s.realmRelay.subscriberGone(int(e.TunnelID), e.SessionID)
	}
	s.withdrawSubscriber(e.TunnelID, e.SessionID, e.Reason)
	ifindex := int(e.TunnelID)
	s.mu.Lock()
	srv := s.servers[ifindex]
	s.mu.Unlock()
	if srv != nil {
		srv.handleSessionDown(e.SessionID)
	}
}

// withdrawSubscriber releases a network lifetime without closing its transport.
func (s *Subsystem) withdrawSubscriber(tid, sid uint16, reason string) {
	ifindex := int(tid)
	s.pendingAuth.Delete(pendingAuthKey{ifindex: ifindex, sessionID: sid})
	id := pppoeSessionID(ifindex, sid)
	sess, found := subscriber.DefaultRegistry.Get(id)
	if !found {
		sess = subscriber.Session{
			ID: id, AccessType: subscriber.AccessPPPoE,
			PPPoESID: sid, AccessIfIndex: ifindex,
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
			Session: sess, Reason: reason,
		}); err != nil {
			s.logger.Warn("pppoe: subscriber session-down emit failed", "error", err)
		}
	}
	l2tp.ClearSessionMetadata(sess.ID)
}

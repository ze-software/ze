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

	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/ze"
)

var errPppoeSubsystemAlreadyStarted = errors.New("pppoe: subsystem already started")

var _ ze.Subsystem = (*Subsystem)(nil)

const SubsystemName = "pppoe"

var ifaceBackendFn = defaultIfaceBackend

func defaultIfaceBackend() ppp.IfaceBackend {
	b := iface.GetBackend()
	if b == nil {
		return nil
	}
	return b
}

// Subsystem is the ze.Subsystem implementation for PPPoE.
type Subsystem struct {
	params Parameters
	logger *slog.Logger

	mu         sync.Mutex
	started    bool
	discFD     int
	servers    map[int]*InterfaceServer
	pppDriver  *ppp.Driver
	readDone   chan struct{}
	eventDone  chan struct{}
	drainDones []<-chan struct{}
	bus        ze.EventBus

	pendingAuth sync.Map // pendingAuthKey -> pendingAuthInfo
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
		params: p,
		logger: slogutil.Logger(SubsystemName),
		discFD: -1,
	}
}

func (s *Subsystem) Name() string { return SubsystemName }

// Start implements ze.Subsystem.
func (s *Subsystem) Start(ctx context.Context, bus ze.EventBus, _ ze.ConfigProvider) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return errPppoeSubsystemAlreadyStarted
	}

	if !s.params.Enabled {
		s.logger.Info("PPPoE subsystem disabled in config, skipping start")
		s.started = true
		return nil
	}
	if len(s.params.Interfaces) == 0 {
		s.logger.Warn("PPPoE subsystem enabled but no interfaces configured, skipping start")
		s.started = true
		return nil
	}

	fd, err := openDiscoverySocket()
	if err != nil {
		return fmt.Errorf("pppoe: discovery socket: %w", err)
	}
	s.discFD = fd

	if backend := ifaceBackendFn(); backend == nil {
		if name := iface.DefaultBackendName(); name != "" {
			if loadErr := iface.LoadBackend(name); loadErr != nil {
				s.logger.Warn("pppoe: fallback iface backend load failed", "error", loadErr.Error())
			}
		}
	}
	if backend := ifaceBackendFn(); backend != nil {
		s.pppDriver = ppp.NewProductionDriver(s.logger.With("component", "ppp"), backend)
	}

	if s.pppDriver != nil {
		if err := s.pppDriver.Start(); err != nil {
			closeDiscoverySocket(s.discFD)
			s.discFD = -1
			return fmt.Errorf("pppoe: start PPP driver: %w", err)
		}
	}

	s.servers = make(map[int]*InterfaceServer)
	for _, ic := range s.params.Interfaces {
		ifindex, hwaddr, mtu, resolveErr := resolveInterface(ic.Name)
		if resolveErr != nil {
			s.logger.Error("pppoe: failed to resolve interface", "interface", ic.Name, "error", resolveErr)
			continue
		}
		svcNames := ic.ServiceNames
		if len(svcNames) == 0 {
			svcNames = s.params.ServiceNames
		}

		cookieKey, keyErr := NewCookieKey()
		if keyErr != nil {
			s.logger.Error("pppoe: failed to generate cookie key", "interface", ic.Name, "error", keyErr)
			continue
		}

		srv := &InterfaceServer{
			ifName:        ic.Name,
			ifIndex:       ifindex,
			hwAddr:        hwaddr,
			mtu:           mtu,
			sessions:      newSessionTable(ic.Name, ic.MaxSessions),
			cookieKey:     cookieKey,
			limiter:       NewPADILimiter(s.params.PADIRateLimit),
			cookieTimeout: s.params.CookieTimeout,
			acName:        s.params.ACName,
			serviceNames:  svcNames,
			authMethod:    s.params.AuthMethod,
			authRequired:  !s.params.AllowNoAuth,
			discFD:        s.discFD,
			pppDriver:     s.pppDriver,
			logger:        s.logger.With("interface", ic.Name),
		}
		s.servers[ifindex] = srv
		s.logger.Info("PPPoE interface configured", "interface", ic.Name, "ifindex", ifindex)
	}

	if len(s.servers) == 0 {
		s.logger.Warn("PPPoE subsystem: no interfaces resolved, nothing to start")
		if s.pppDriver != nil {
			s.pppDriver.Stop()
			s.pppDriver = nil
		}
		closeDiscoverySocket(s.discFD)
		s.discFD = -1
		s.started = true
		return nil
	}

	s.readDone = make(chan struct{})
	go s.discoveryReader()

	if s.pppDriver != nil {
		authH := subscriber.GetAuthHandler()
		poolH := subscriber.GetPoolHandler()
		if authH == nil {
			s.logger.Warn("pppoe: no auth handler registered; all sessions will be accepted")
		}
		if poolH == nil {
			s.logger.Error("pppoe: no pool handler registered; all IP requests will be rejected")
		}
		s.bus = bus
		s.drainDones = append(s.drainDones,
			startPPPoEAuthDrain(s.logger, s.pppDriver, authH, bus, &s.pendingAuth),
			startPPPoEPoolDrain(s.logger, s.pppDriver, poolH),
		)
		s.eventDone = make(chan struct{})
		go s.eventConsumer()
	}

	s.started = true
	PublishService(s)
	return nil
}

// Stop implements ze.Subsystem.
func (s *Subsystem) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	PublishService(nil)
	s.logger.Info("PPPoE subsystem stopping")

	if s.discFD >= 0 {
		closeDiscoverySocket(s.discFD)
		s.discFD = -1
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

	s.pppDriver = nil
	s.servers = nil
	s.started = false
	return nil
}

// Reload implements ze.Subsystem.
func (s *Subsystem) Reload(_ context.Context, _ ze.ConfigProvider) error {
	return nil
}

func (s *Subsystem) discoveryReader() {
	defer close(s.readDone)

	buf := make([]byte, EthMaxLen)
	for {
		n, ifindex, err := readDiscoveryFrame(s.discFD, buf)
		if err != nil {
			if errors.Is(err, errSocketClosed) {
				return
			}
			s.logger.Debug("pppoe: discovery read error", "error", err)
			continue
		}
		if n < MinDiscFrame {
			continue
		}

		s.mu.Lock()
		srv := s.servers[ifindex]
		s.mu.Unlock()

		if srv == nil {
			continue
		}

		pkt, parseErr := ParseDiscovery(buf[:n])
		if parseErr != nil {
			s.logger.Debug("pppoe: parse error", "interface", srv.ifName, "error", parseErr)
			continue
		}

		srv.HandleDiscovery(&pkt)
	}
}

func pppoeSessionID(ifindex int, sid uint16) string {
	var buf textbuf.Buffer
	return buf.Reset().Str("pppoe-").Uint(uint64(ifindex)).Byte('-').Uint16(sid).String()
}

func (s *Subsystem) eventConsumer() {
	defer close(s.eventDone)

	for ev := range s.pppDriver.EventsOut() {
		s.handlePPPEvent(ev)
	}
}

// handlePPPEvent applies one PPP driver lifecycle event: it updates the
// shared subscriber registry and publishes the matching event on the
// subscriber namespace, where the address pool, accounting and shaping
// consumers read it. eventConsumer calls it for every event the driver
// produces, one at a time.
func (s *Subsystem) handlePPPEvent(ev ppp.Event) {
	switch e := ev.(type) {
	case ppp.EventSessionUp:
		s.onSessionUp(e)
	case ppp.EventSessionIPAssigned:
		s.onSessionIPAssigned(e)
	case ppp.EventSessionDown:
		s.onSessionDown(e)
	}
}

// onSessionUp records a session that has completed LCP, authentication and
// every enabled NCP, then publishes it.
func (s *Subsystem) onSessionUp(e ppp.EventSessionUp) {
	ifindex := int(e.TunnelID)
	s.mu.Lock()
	srv := s.servers[ifindex]
	s.mu.Unlock()
	if srv == nil {
		return
	}

	snap := srv.sessions.Lookup(e.SessionID)
	sess := subscriber.Session{
		ID:            pppoeSessionID(ifindex, e.SessionID),
		AccessType:    subscriber.AccessPPPoE,
		State:         subscriber.StateActive,
		PPPoESID:      e.SessionID,
		AccessIfIndex: ifindex,
		ActivatedAt:   time.Now(),
	}
	if snap != nil {
		sess.MAC = snap.MAC
		sess.AccessInterface = snap.IfName
		sess.ServiceName = snap.ServiceName
		sess.PppInterface = "ppp" + textbuf.StringUint(uint64(snap.UnitNum))
	}
	authKey := pendingAuthKey{ifindex: ifindex, sessionID: e.SessionID}
	if val, ok := s.pendingAuth.LoadAndDelete(authKey); ok {
		if info, ok2 := val.(pendingAuthInfo); ok2 {
			sess.Username = info.username
			sess.AuthMethod = info.authMethod
		}
	}
	sess.AcctSessionID = sess.ID
	subscriber.DefaultRegistry.Add(&sess)
	subscriber.RecordSessionUp(subscriber.AccessPPPoE)

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
func (s *Subsystem) onSessionIPAssigned(e ppp.EventSessionIPAssigned) {
	id := pppoeSessionID(int(e.TunnelID), e.SessionID)
	sess, ok := subscriber.DefaultRegistry.Get(id)
	if !ok {
		return
	}

	if e.Peer.IsValid() {
		sess.IPv4Addr = e.Peer
	}
	sess.DNSPrimary = e.DNSPrimary
	sess.DNSSecondary = e.DNSSecondary
	sess.IPv6InterfaceID = e.InterfaceID
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
// The event is published whether or not the registry holds the session. A
// session that fails an NCP, or whose peer disconnects between IPCP and
// session-up, never reaches the registry and still holds the address IPCP
// allocated for it, so a registry miss must not swallow the teardown.
// subscriberBridge.onSessionDown falls back the same way for L2TP.
func (s *Subsystem) onSessionDown(e ppp.EventSessionDown) {
	ifindex := int(e.TunnelID)
	s.pendingAuth.Delete(pendingAuthKey{ifindex: ifindex, sessionID: e.SessionID})

	id := pppoeSessionID(ifindex, e.SessionID)
	sess, found := subscriber.DefaultRegistry.Get(id)
	if !found {
		sess = subscriber.Session{
			ID:            id,
			AccessType:    subscriber.AccessPPPoE,
			PPPoESID:      e.SessionID,
			AccessIfIndex: ifindex,
		}
	}
	sess.State = subscriber.StateTerminating
	subscriber.DefaultRegistry.Remove(id)
	if found {
		subscriber.RecordSessionDown(subscriber.AccessPPPoE)
	}

	if s.bus != nil {
		if _, err := subevents.SessionDown.Emit(s.bus, &subevents.SessionDownPayload{
			Session: sess,
			Reason:  e.Reason,
		}); err != nil {
			s.logger.Warn("pppoe: subscriber session-down emit failed", "error", err)
		}
	}

	s.mu.Lock()
	srv := s.servers[ifindex]
	s.mu.Unlock()
	if srv != nil {
		srv.handleSessionDown(e.SessionID)
	}
}

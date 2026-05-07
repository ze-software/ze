// Design: plan/spec-bng-5-pppoe.md -- PPPoE subsystem lifecycle
// Related: config.go -- Parameters consumed at Start
// Related: service.go -- PublishService / LookupService

package pppoe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

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
		return fmt.Errorf("pppoe: subsystem already started")
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
			sessions:      NewSessionTable(ic.Name, ic.MaxSessions),
			cookieKey:     cookieKey,
			limiter:       NewPADILimiter(s.params.PADIRateLimit),
			cookieTimeout: s.params.CookieTimeout,
			acName:        s.params.ACName,
			serviceNames:  svcNames,
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

func (s *Subsystem) eventConsumer() {
	defer close(s.eventDone)

	for ev := range s.pppDriver.EventsOut() {
		down, ok := ev.(ppp.EventSessionDown)
		if !ok {
			continue
		}

		ifindex := int(down.TunnelID)

		s.mu.Lock()
		srv := s.servers[ifindex]
		s.mu.Unlock()

		if srv == nil {
			continue
		}

		srv.handleSessionDown(down.SessionID)
	}
}

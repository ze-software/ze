// Design: docs/architecture/l2tp/bng-5-pppoe.md -- reload applies the diff live
// Related: subsystem.go -- startLocked and stopLocked, which Reload reuses
// Related: config.go -- ExtractParameters, the one parser of the pppoe root

package pppoe

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/ze"
)

var (
	errPppoeReloadNilProvider = errors.New("pppoe reload: no config provider")
	errPppoeReloadNotStarted  = errors.New("pppoe reload: subsystem not started")
)

// Reload applies a committed `pppoe` configuration to the running access
// concentrator. Every leaf applies live; none is accepted and left for a
// restart:
//
//   - ac-name, service-name, cookie-timeout, padi-rate-limit, max-sessions,
//     max-sessions-per-mac, auth-method and allow-no-auth apply to the next
//     discovery exchange and the next admitted session. A session already up
//     keeps the terms it was admitted under; a lowered max-sessions refuses
//     new sessions, it never ends one. Outstanding AC-Cookies stay valid,
//     because each interface keeps its cookie key.
//   - an added access interface is resolved and served at once. One that does
//     not exist refuses the whole reload, so the commit fails and names it.
//   - a removed access interface stops answering, and each of its sessions
//     is ended with a PADT and its PPP session stopped.
//   - enabled true, or a first interface, starts an idle subsystem; enabled
//     false, or no interface left, ends every session and stops it.
//
// Preparation errors refuse the reload before anything changes. Cleanup
// errors after removing a served access are reported after its sessions stop.
func (s *Subsystem) Reload(ctx context.Context, cfg ze.ConfigProvider) error {
	next, err := parametersFromProvider(cfg)
	if err != nil {
		return err
	}

	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	if !s.started {
		return errPppoeReloadNotStarted
	}
	running := s.readDone != nil
	wantRun := next.Enabled && len(next.Interfaces) > 0
	if !running {
		return s.reloadIdle(ctx, next, wantRun)
	}
	var prepared *accessConfiguration
	var additions map[string]*InterfaceServer
	if wantRun {
		selected, err := selectedSessionBackend()
		if err != nil {
			return err
		}
		if selected.Name() != s.backend.Name() {
			return opererr.New("The PPPoE interface backend changed while discovery is running",
				"Existing sessions and control mappings belong to the previously selected backend.", "ze doctor")
		}
		prepared, err = prepareAccessConfiguration(ctx, s.backend, next.Interfaces, s.accesses.accesses)
		if err != nil {
			return err
		}
		if prepared.namespace != s.accesses.namespace {
			return errors.Join(opererr.New("The PPPoE discovery namespace changed during a live reload",
				"Existing sessions still use the running discovery socket.", "ze doctor"),
				closeAccessConfiguration(prepared.accesses, s.accesses.accesses))
		}
		additions, err = s.prepareAddedServers(&next, prepared)
		if err != nil {
			return errors.Join(err, closeAccessConfiguration(prepared.accesses, s.accesses.accesses))
		}
	}
	previous := s.accesses
	removed := s.swapServers(&next, wantRun, additions)
	if wantRun {
		s.accesses = prepared
	}
	for _, srv := range removed {
		s.closeInterface(srv)
	}
	if !wantRun {
		stopErr := s.stopLocked()
		s.started = true
		s.logger.Info("pppoe reload: subsystem stopped by configuration")
		return stopErr
	}
	return accessReleaseError(closeAccessConfiguration(previous.accesses, prepared.accesses))
}

// parametersFromProvider reads the `pppoe` root through ExtractParameters, the
// parser boot uses, so a reload and a restart read one configuration the same
// way. A root the reload removed reads as empty, which is disabled.
func parametersFromProvider(cfg ze.ConfigProvider) (Parameters, error) {
	if cfg == nil {
		return Parameters{}, errPppoeReloadNilProvider
	}
	root, err := cfg.Get(SubsystemName)
	if err != nil {
		return Parameters{}, fmt.Errorf("pppoe reload: read the pppoe config: %w", err)
	}
	p, err := ExtractParameters(map[string]any{SubsystemName: root})
	if err != nil {
		return Parameters{}, fmt.Errorf("pppoe reload: %w", err)
	}
	return p, nil
}

// configuredInterface reports whether the running configuration names the
// access interface.
func (s *Subsystem) configuredInterface(name string) bool {
	return slices.ContainsFunc(s.params.Interfaces, func(ic InterfaceConfig) bool { return ic.Name == name })
}

// reloadIdle applies a reload to a subsystem that serves nothing: it records
// the configuration and, when that configuration asks for discovery, starts it.
func (s *Subsystem) reloadIdle(ctx context.Context, next Parameters, wantRun bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous := s.params
	s.params = next
	if !wantRun {
		return nil
	}
	if err := s.startLocked(ctx); err != nil {
		s.params = previous
		return fmt.Errorf("pppoe reload: start: %w", err)
	}
	s.logger.Info("pppoe reload: subsystem started by configuration", "interfaces", len(s.servers))
	return nil
}

// swapServers installs next and the server map it describes, and returns the
// servers it took out of service. A kept interface gets a new server carrying
// the new configuration over the same session table, cookie key and, unless
// padi-rate-limit changed, the same PADI limiter. With wantRun false every
// server is taken out of service.
//
// It holds dispatchMu, so the discovery reader is between frames: no frame is
// handled by a server after it is taken out, and the copy of a kept server
// does not race the reader's write of its admission state.
func (s *Subsystem) swapServers(next *Parameters, wantRun bool, additions map[string]*InterfaceServer) []*InterfaceServer {
	s.dispatchMu.Lock()
	defer s.dispatchMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()

	byName := make(map[string]*InterfaceServer, len(s.servers))
	for _, srv := range s.servers {
		byName[srv.ifName] = srv
	}
	servers := make(map[int]*InterfaceServer, len(next.Interfaces))
	interfaces := next.Interfaces
	if !wantRun {
		interfaces = nil
	}
	for i := range interfaces {
		ic := &interfaces[i]
		if old, ok := byName[ic.Name]; ok {
			servers[old.ifIndex] = s.reconfiguredServer(old, ic, next)
			delete(byName, ic.Name)
			continue
		}
		srv := additions[ic.Name]
		servers[srv.ifIndex] = srv
		s.logger.Info("pppoe reload: access interface added", "interface", ic.Name, "ifindex", srv.ifIndex)
	}

	removed := make([]*InterfaceServer, 0, len(byName))
	for _, srv := range byName {
		removed = append(removed, srv)
		s.logger.Info("pppoe reload: access interface removed", "interface", srv.ifName, "sessions", srv.sessions.Count())
	}
	if s.params.ACName != next.ACName {
		s.logger.Info("pppoe reload: ac-name updated", "previous", s.params.ACName, "new", next.ACName)
	}
	s.servers = servers
	s.params = *next
	return removed
}

// reconfiguredServer returns a copy of old carrying next's configuration. The
// copy keeps old's identity and state (interface, session table, cookie key,
// discovery socket, PPP driver), so the swap is invisible to a live session.
func (s *Subsystem) reconfiguredServer(old *InterfaceServer, ic *InterfaceConfig, next *Parameters) *InterfaceServer {
	srv := *old
	applyInterfaceConfig(&srv, ic, next)
	if s.params.PADIRateLimit != next.PADIRateLimit {
		srv.limiter = NewPADILimiter(next.PADIRateLimit)
	}
	srv.sessions.setMaxSessions(ic.MaxSessions)
	return &srv
}

// prepareAddedServers builds every new server before the live map changes.
// Its accesses MUST already be prepared and owned by the pending reload.
func (s *Subsystem) prepareAddedServers(next *Parameters, prepared *accessConfiguration) (map[string]*InterfaceServer, error) {
	additions := make(map[string]*InterfaceServer)
	for i := range next.Interfaces {
		ic := &next.Interfaces[i]
		if s.configuredInterface(ic.Name) {
			continue
		}
		srv, err := s.newInterfaceServer(ic, next, s.backend, prepared.accesses[ic.Name])
		if err != nil {
			return nil, err
		}
		additions[ic.Name] = srv
	}
	return additions, nil
}

// newInterfaceServer consumes an immutable prepared access snapshot. The
// caller MUST retain that access lease until sessions stop and the server is
// detached from discovery. Safe only under the subsystem's lifecycle lock.
func (s *Subsystem) newInterfaceServer(ic *InterfaceConfig, p *Parameters, backend dataplane.Backend, access dataplane.Access) (*InterfaceServer, error) {
	cookieKey, err := NewCookieKey()
	if err != nil {
		return nil, fmt.Errorf("pppoe: generate cookie key for %s: %w", ic.Name, err)
	}
	srv := &InterfaceServer{
		ifName:    ic.Name,
		device:    access.Device(),
		ifIndex:   access.DiscoveryIfIndex(),
		hwAddr:    access.HWAddr(),
		mtu:       access.MTU(),
		sessions:  newSessionTable(ic.Name, ic.MaxSessions),
		cookieKey: cookieKey,
		limiter:   NewPADILimiter(p.PADIRateLimit),
		discFD:    s.discFD,
		pppDriver: s.pppDriver,
		openSession: func(ctx context.Context, request dataplane.Request) (dataplane.OpenedSession, error) {
			request.Device = access.Device()
			request.ControlDevice = access.ControlDevice()
			request.Namespace = access.Namespace()
			return backend.Open(ctx, request)
		},
		peerLeft:  s.peerLeftSession,
		relayWake: s.relayWake,
		logger:    s.logger.With("interface", ic.Name),
	}
	applyInterfaceConfig(srv, ic, p)
	return srv, nil
}

// applyInterfaceConfig sets every field of srv the configuration decides. It
// is the one place that maps Parameters onto a server, for boot and reload.
func applyInterfaceConfig(srv *InterfaceServer, ic *InterfaceConfig, p *Parameters) {
	serviceNames := ic.ServiceNames
	if len(serviceNames) == 0 {
		serviceNames = p.ServiceNames
	}
	srv.maxSessionsPerMAC = ic.MaxSessionsPerMAC
	srv.cookieTimeout = p.CookieTimeout
	srv.acName = p.ACName
	srv.serviceNames = serviceNames
	srv.authMethod = p.AuthMethod
	srv.lcpConfig = p.LCP
	srv.authRequired = !p.AllowNoAuth
	// Each server generation owns its policy; reload never mutates the slice
	// held by a session already handed to the driver.
	srv.authAllow = slices.Clone(p.AuthAllow)
}

// closeInterface ends every session on an access interface the reload took
// out of service. Each session's transport is closed and a PADT sent first
// (handleSessionDown), then its PPP session is stopped; the driver's
// session-down event then withdraws the subscriber (onSessionDown). MUST be
// called with mu, dispatchMu and every session-table lock not held: teardown
// and the PPP stop both need the event consumer to make progress.
func (s *Subsystem) closeInterface(srv *InterfaceServer) {
	for _, admission := range srv.sessions.admissions() {
		sid := admission.session.SID
		// RFC 2516 Section 5.5: "This packet may be sent anytime after a
		// session is established to indicate that a PPPoE session has been
		// terminated.  It may be sent by either the Host or the Access
		// Concentrator."
		srv.handleSessionDown(sid)
		// Another owner may already be sending PADT or releasing its lease.
		// Join outside all locks before the old access generation can close.
		<-admission.session.closeDone
		s.pendingAuth.Delete(pendingAuthKey{ifindex: srv.ifIndex, sessionID: sid})
		if s.pppDriver == nil {
			continue
		}
		err := s.pppDriver.StopSession(uint16(srv.ifIndex), sid)
		if err != nil && !errors.Is(err, ppp.ErrSessionNotFound) {
			s.logger.Warn("pppoe reload: stop PPP session", "interface", srv.ifName, "sid", sid, "error", err)
		}
	}
}

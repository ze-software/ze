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
	"github.com/ze-software/ze/pkg/ze"
)

var (
	errPppoeReloadNilProvider = errors.New("pppoe reload: no config provider")
	errPppoeReloadNotStarted  = errors.New("pppoe reload: subsystem not started")
)

// resolveInterfaceFn resolves an access interface name to its index, MAC
// address and MTU. A test replaces it, because the real resolver reads the
// kernel's interface table.
var resolveInterfaceFn = resolveInterface

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
// An error refuses the reload before anything changed, and the engine rolls
// the reload back (cmd/ze/hub/main_reload.go, runReloadContext).
func (s *Subsystem) Reload(_ context.Context, cfg ze.ConfigProvider) error {
	next, err := parametersFromProvider(cfg)
	if err != nil {
		return err
	}

	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	if !s.started {
		return errPppoeReloadNotStarted
	}
	if err := s.checkAddedInterfaces(&next); err != nil {
		return err
	}

	running := s.readDone != nil
	wantRun := next.Enabled && len(next.Interfaces) > 0
	if !running {
		return s.reloadIdle(next, wantRun)
	}
	removed := s.swapServers(&next, wantRun)
	for _, srv := range removed {
		s.closeInterface(srv)
	}
	if !wantRun {
		s.stopLocked()
		s.started = true
		s.logger.Info("pppoe reload: subsystem stopped by configuration")
	}
	return nil
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

// checkAddedInterfaces refuses a reload that adds an access interface the
// kernel does not have. An interface the running configuration already named
// is not checked: boot logged it when it failed to resolve, and refusing every
// later commit for it would block unrelated changes.
func (s *Subsystem) checkAddedInterfaces(next *Parameters) error {
	if !next.Enabled {
		return nil
	}
	for i := range next.Interfaces {
		name := next.Interfaces[i].Name
		if s.configuredInterface(name) {
			continue
		}
		if _, _, _, err := resolveInterfaceFn(name); err != nil {
			return fmt.Errorf("pppoe reload: access interface %q does not exist (%w): create it, or remove it with `delete pppoe interface %s`, then commit again", name, err, name)
		}
	}
	return nil
}

// configuredInterface reports whether the running configuration names the
// access interface.
func (s *Subsystem) configuredInterface(name string) bool {
	return slices.ContainsFunc(s.params.Interfaces, func(ic InterfaceConfig) bool { return ic.Name == name })
}

// reloadIdle applies a reload to a subsystem that serves nothing: it records
// the configuration and, when that configuration asks for discovery, starts it.
func (s *Subsystem) reloadIdle(next Parameters, wantRun bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.params = next
	if !wantRun {
		return nil
	}
	if err := s.startLocked(); err != nil {
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
func (s *Subsystem) swapServers(next *Parameters, wantRun bool) []*InterfaceServer {
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
		srv, err := s.newInterfaceServer(ic, next)
		if err != nil {
			s.logger.Error("pppoe reload: access interface not started", "interface", ic.Name, "error", err)
			continue
		}
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

// newInterfaceServer resolves one access interface and builds its server for
// configuration p, with a fresh session table and cookie key. MUST be called
// with mu held after the discovery socket is open.
func (s *Subsystem) newInterfaceServer(ic *InterfaceConfig, p *Parameters) (*InterfaceServer, error) {
	ifindex, hwaddr, mtu, err := resolveInterfaceFn(ic.Name)
	if err != nil {
		return nil, err
	}
	cookieKey, err := NewCookieKey()
	if err != nil {
		return nil, fmt.Errorf("pppoe: generate cookie key for %s: %w", ic.Name, err)
	}
	srv := &InterfaceServer{
		ifName:    ic.Name,
		ifIndex:   ifindex,
		hwAddr:    hwaddr,
		mtu:       mtu,
		sessions:  newSessionTable(ic.Name, ic.MaxSessions),
		cookieKey: cookieKey,
		limiter:   NewPADILimiter(p.PADIRateLimit),
		discFD:    s.discFD,
		pppDriver: s.pppDriver,
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
	srv.authRequired = !p.AllowNoAuth
}

// closeInterface ends every session on an access interface the reload took
// out of service. Each session's transport is closed and a PADT sent first
// (handleSessionDown), then its PPP session is stopped; the driver's
// session-down event then withdraws the subscriber (onSessionDown). MUST be
// called with mu not held: the stop waits for that event to be consumed, and
// its consumer takes mu.
func (s *Subsystem) closeInterface(srv *InterfaceServer) {
	for _, sid := range srv.sessions.sids() {
		// RFC 2516 Section 5.5: "This packet may be sent anytime after a
		// session is established to indicate that a PPPoE session has been
		// terminated.  It may be sent by either the Host or the Access
		// Concentrator."
		srv.handleSessionDown(sid)
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

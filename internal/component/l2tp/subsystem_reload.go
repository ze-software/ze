// Design: docs/architecture/l2tp.md -- subsystem Reload semantics
// Related: subsystem.go -- owns the Parameters field Reload mutates
// Related: config.go -- ExtractParameters produces the diff input

package l2tp

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Reload re-reads L2TP configuration from the supplied ConfigProvider
// and applies each changed knob according to the spec-l2tp-7 diff-apply
// policy:
//
//   - shared-secret, hello-interval, max-tunnels, max-sessions,
//     auth-method, allow-no-auth: hot-apply. Takes effect on new tunnels,
//     new sessions, or new admission decisions. Live tunnels are not
//     re-keyed or re-timed.
//   - enabled flip (true<->false): rejected with WARN. Operator must
//     restart.
//   - environment/l2tp/server/* listener endpoints: rejected with WARN.
//     Binding new UDP sockets mid-run without disturbing live tunnels
//     is out of scope; restart is acceptable.
//
// Reload never tears down a live tunnel simply because the config text
// changed. Operator-visible WARN lines name the rejected field.
//
// MUST be called after Start; calling before Start is a programmer
// error and returns ErrSubsystemNotStarted.
func (s *Subsystem) Reload(_ context.Context, cfg ze.ConfigProvider) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return ErrSubsystemNotStarted
	}

	next, err := extractFromProvider(cfg)
	if err != nil {
		return fmt.Errorf("l2tp reload: %w", err)
	}

	prev := s.params
	applied := 0
	rejected := 0

	// enabled flip is a restart-only change.
	if prev.Enabled != next.Enabled {
		s.logger.Warn("l2tp reload: 'enabled' flip ignored; restart ze to apply",
			"previous", prev.Enabled, "requested", next.Enabled)
		rejected++
	}

	// listener endpoints (ip/port) are restart-only.
	if !listenAddrsEqual(prev.ListenAddrs, next.ListenAddrs) {
		s.logger.Warn("l2tp reload: listener endpoint change ignored; restart ze to apply",
			"previous", formatListenAddrs(prev.ListenAddrs),
			"requested", formatListenAddrs(next.ListenAddrs))
		rejected++
	}

	// CQM observer parameters are restart-only (pools pre-allocated at Start).
	if prev.CQMEnabled != next.CQMEnabled {
		s.logger.Warn("l2tp reload: 'cqm-enabled' change ignored; restart ze to apply")
		rejected++
	}
	if prev.MaxLogins != next.MaxLogins {
		s.logger.Warn("l2tp reload: 'max-logins' change ignored; restart ze to apply")
		rejected++
	}
	if prev.EventRingSizePerSession != next.EventRingSizePerSession {
		s.logger.Warn("l2tp reload: 'event-ring-size-per-session' change ignored; restart ze to apply")
		rejected++
	}
	if prev.SampleRetentionSeconds != next.SampleRetentionSeconds {
		s.logger.Warn("l2tp reload: 'sample-retention-seconds' change ignored; restart ze to apply")
		rejected++
	}

	// Hot-apply: shared-secret.
	if prev.SharedSecret != next.SharedSecret {
		s.params.SharedSecret = next.SharedSecret
		for _, r := range s.reactors {
			r.setSharedSecret(next.SharedSecret)
		}
		s.logger.Info("l2tp reload: shared-secret updated (applies to new tunnels only)",
			"now-set", next.SharedSecret != "")
		applied++
	}

	// Hot-apply: hello-interval.
	if prev.HelloInterval != next.HelloInterval {
		s.params.HelloInterval = next.HelloInterval
		for _, r := range s.reactors {
			r.setHelloInterval(next.HelloInterval)
		}
		s.logger.Info("l2tp reload: hello-interval updated (applies to new tunnels only)",
			"previous", prev.HelloInterval.String(), "new", next.HelloInterval.String())
		applied++
	}

	// Hot-apply: max-tunnels.
	if prev.MaxTunnels != next.MaxTunnels {
		s.params.MaxTunnels = next.MaxTunnels
		for _, r := range s.reactors {
			r.setMaxTunnels(next.MaxTunnels)
		}
		s.logger.Info("l2tp reload: max-tunnels updated",
			"previous", prev.MaxTunnels, "new", next.MaxTunnels)
		applied++
	}

	// Hot-apply: max-sessions.
	if prev.MaxSessions != next.MaxSessions {
		s.params.MaxSessions = next.MaxSessions
		for _, r := range s.reactors {
			r.setMaxSessions(next.MaxSessions)
		}
		s.logger.Info("l2tp reload: max-sessions updated",
			"previous", prev.MaxSessions, "new", next.MaxSessions)
		applied++
	}

	// Hot-apply: PPP auth policy for new sessions.
	if prev.AuthMethod != next.AuthMethod {
		s.params.AuthMethod = next.AuthMethod
		for _, r := range s.reactors {
			r.setPPPAuthMethod(next.AuthMethod)
		}
		s.logger.Info("l2tp reload: auth-method updated",
			"previous", prev.AuthMethod.String(), "new", next.AuthMethod.String())
		applied++
	}
	if prev.AllowNoAuth != next.AllowNoAuth {
		s.params.AllowNoAuth = next.AllowNoAuth
		for _, r := range s.reactors {
			r.setPPPAuthRequired(!next.AllowNoAuth)
		}
		s.logger.Info("l2tp reload: allow-no-auth updated",
			"previous", prev.AllowNoAuth, "new", next.AllowNoAuth)
		applied++
	}

	if applied == 0 && rejected == 0 {
		s.logger.Debug("l2tp reload: no changes detected")
	}
	return nil
}

// extractFromProvider pulls Parameters out of a ConfigProvider.
// ConfigProvider exposes config subtrees as map[string]any (Get("l2tp"),
// Get("environment")); this helper walks the maps and builds Parameters
// without going through config.Tree.
func extractFromProvider(cfg ze.ConfigProvider) (Parameters, error) {
	if cfg == nil {
		return Parameters{}, fmt.Errorf("nil config provider")
	}
	l2tpRoot, err := cfg.Get("l2tp")
	if err != nil {
		return Parameters{}, fmt.Errorf("get l2tp root: %w", err)
	}
	if len(l2tpRoot) == 0 {
		return Parameters{}, nil
	}
	p := Parameters{
		Enabled:       true,
		HelloInterval: time.Duration(DefaultHelloSecs) * time.Second,
		MaxTunnels:    DefaultMaxTunnels,
		MaxSessions:   DefaultMaxSessions,
		AuthMethod:    DefaultAuthMethod,
	}
	if v, ok := l2tpRoot["enabled"].(string); ok {
		p.Enabled = v == configTrue
	}
	if v, ok := l2tpRoot["shared-secret"].(string); ok {
		p.SharedSecret = v
	}
	if v, ok := l2tpRoot["hello-interval"].(string); ok {
		n, perr := strconv.ParseUint(v, 10, 16)
		if perr != nil {
			return Parameters{}, fmt.Errorf("hello-interval: %w", perr)
		}
		if n == 0 {
			return Parameters{}, fmt.Errorf("hello-interval: must be > 0")
		}
		p.HelloInterval = time.Duration(n) * time.Second
	}
	if v, ok := l2tpRoot["max-tunnels"].(string); ok {
		n, perr := strconv.ParseUint(v, 10, 16)
		if perr != nil {
			return Parameters{}, fmt.Errorf("max-tunnels: %w", perr)
		}
		p.MaxTunnels = uint16(n)
	}
	if v, ok := l2tpRoot["max-sessions"].(string); ok {
		n, perr := strconv.ParseUint(v, 10, 16)
		if perr != nil {
			return Parameters{}, fmt.Errorf("max-sessions: %w", perr)
		}
		p.MaxSessions = uint16(n)
	}
	if v, ok := l2tpRoot["auth-method"].(string); ok {
		m, perr := parsePPPAuthMethod(v)
		if perr != nil {
			return Parameters{}, fmt.Errorf("auth-method: %w", perr)
		}
		p.AuthMethod = m
	}
	if v, ok := l2tpRoot["allow-no-auth"].(string); ok {
		p.AllowNoAuth = v == configTrue
	}
	if p.AuthMethod == ppp.AuthMethodNone && !p.AllowNoAuth {
		return Parameters{}, fmt.Errorf("auth-method none requires allow-no-auth true")
	}
	if v, ok := l2tpRoot["cqm-enabled"].(string); ok {
		p.CQMEnabled = v == configTrue
	}
	p.MaxLogins = 1000
	if v, ok := l2tpRoot["max-logins"].(string); ok {
		n, perr := strconv.ParseUint(v, 10, 32)
		if perr != nil {
			return Parameters{}, fmt.Errorf("max-logins: %w", perr)
		}
		if n == 0 || n > 1000000 {
			return Parameters{}, fmt.Errorf("max-logins: must be 1-1000000")
		}
		p.MaxLogins = int(n)
	}
	p.EventRingSizePerSession = 256
	if v, ok := l2tpRoot["event-ring-size-per-session"].(string); ok {
		n, perr := strconv.ParseUint(v, 10, 16)
		if perr != nil {
			return Parameters{}, fmt.Errorf("event-ring-size-per-session: %w", perr)
		}
		if n < 16 || n > 4096 {
			return Parameters{}, fmt.Errorf("event-ring-size-per-session: must be 16-4096")
		}
		p.EventRingSizePerSession = int(n)
	}
	p.SampleRetentionSeconds = 86400
	if v, ok := l2tpRoot["sample-retention-seconds"].(string); ok {
		n, perr := strconv.ParseUint(v, 10, 32)
		if perr != nil {
			return Parameters{}, fmt.Errorf("sample-retention-seconds: %w", perr)
		}
		if n < 100 || n > 86400 {
			return Parameters{}, fmt.Errorf("sample-retention-seconds: must be 100-86400")
		}
		p.SampleRetentionSeconds = int(n)
	}
	env, err := cfg.Get("environment")
	if err != nil {
		return Parameters{}, fmt.Errorf("get environment root: %w", err)
	}
	if err := appendListenersFromEnv(&p, env); err != nil {
		return Parameters{}, err
	}
	return p, nil
}

// appendListenersFromEnv reads environment/l2tp/server entries out of
// the environment root and appends each to p.ListenAddrs.
func appendListenersFromEnv(p *Parameters, env map[string]any) error {
	if len(env) == 0 {
		return nil
	}
	l2tpEnv, _ := env["l2tp"].(map[string]any)
	if len(l2tpEnv) == 0 {
		return nil
	}
	servers, _ := l2tpEnv["server"].(map[string]any)
	if len(servers) == 0 {
		return nil
	}
	for name, v := range servers {
		entry, _ := v.(map[string]any)
		if entry == nil {
			return fmt.Errorf("l2tp server %q: unexpected shape", name)
		}
		ip := DefaultListenIP
		port := strconv.Itoa(DefaultListenPort)
		if s, ok := entry["ip"].(string); ok && s != "" {
			ip = s
		}
		if s, ok := entry["port"].(string); ok && s != "" {
			port = s
		}
		addr, err := parseListen(ip, port)
		if err != nil {
			return fmt.Errorf("l2tp server %q: %w", name, err)
		}
		p.ListenAddrs = append(p.ListenAddrs, addr)
	}
	return nil
}

// listenAddrsEqual returns true when both slices contain the same set
// of endpoints regardless of order.
func listenAddrsEqual(a, b []netip.AddrPort) bool {
	if len(a) != len(b) {
		return false
	}
	as := append([]netip.AddrPort(nil), a...)
	bs := append([]netip.AddrPort(nil), b...)
	slices.SortFunc(as, compareAddrPort)
	slices.SortFunc(bs, compareAddrPort)
	return slices.Equal(as, bs)
}

// compareAddrPort orders netip.AddrPort by address then port.
func compareAddrPort(a, b netip.AddrPort) int {
	if c := a.Addr().Compare(b.Addr()); c != 0 {
		return c
	}
	switch {
	case a.Port() < b.Port():
		return -1
	case a.Port() > b.Port():
		return 1
	}
	return 0
}

// formatListenAddrs renders a slice of endpoints as a readable string
// for log output. The Parameters comparison uses the raw slice.
func formatListenAddrs(addrs []netip.AddrPort) string {
	if len(addrs) == 0 {
		return "<none>"
	}
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		parts = append(parts, a.String())
	}
	return strings.Join(parts, ",")
}

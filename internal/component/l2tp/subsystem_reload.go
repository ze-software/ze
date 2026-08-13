// Design: docs/guide/l2tp.md -- subsystem Reload semantics
// Related: subsystem.go -- owns the Parameters field Reload mutates
// Related: config.go -- ExtractParameters produces the diff input

package l2tp

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/ze"
)

var (
	errNilConfigProvider               = errors.New("nil config provider")
	errHelloIntervalMustBe0            = errors.New("hello-interval: must be > 0")
	errAuthMethodNoneRequiresAllowNo   = errors.New("auth-method none requires allow-no-auth true")
	errMaxLoginsMustBe11000000         = errors.New("max-logins: must be 1-1000000")
	errEventRingSizePerSessionMust     = errors.New("event-ring-size-per-session: must be 16-4096")
	errSampleRetentionSecondsMustBe100 = errors.New("sample-retention-seconds: must be 100-86400")
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

	// Hot-apply: hello-retries (dead-peer detection threshold).
	if prev.HelloRetries != next.HelloRetries {
		s.params.HelloRetries = next.HelloRetries
		for _, r := range s.reactors {
			r.setHelloRetries(next.HelloRetries)
		}
		s.logger.Info("l2tp reload: hello-retries updated",
			"previous", prev.HelloRetries, "new", next.HelloRetries)
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

	// Hot-apply: auth/NCP settings for new sessions.
	if prev.AuthTimeout != next.AuthTimeout {
		s.params.AuthTimeout = next.AuthTimeout
		for _, r := range s.reactors {
			r.setAuthTimeout(next.AuthTimeout)
		}
		s.logger.Info("l2tp reload: authentication timeout updated",
			"previous", prev.AuthTimeout.String(), "new", next.AuthTimeout.String())
		applied++
	}
	if prev.ReauthInterval != next.ReauthInterval {
		s.params.ReauthInterval = next.ReauthInterval
		for _, r := range s.reactors {
			r.setReauthInterval(next.ReauthInterval)
		}
		s.logger.Info("l2tp reload: reauth-interval updated",
			"previous", prev.ReauthInterval.String(), "new", next.ReauthInterval.String())
		applied++
	}
	if prev.EnableIPCP != next.EnableIPCP {
		s.params.EnableIPCP = next.EnableIPCP
		for _, r := range s.reactors {
			r.setEnableIPCP(next.EnableIPCP)
		}
		s.logger.Info("l2tp reload: enable-ipcp updated",
			"previous", prev.EnableIPCP, "new", next.EnableIPCP)
		applied++
	}
	if prev.EnableIPv6CP != next.EnableIPv6CP {
		s.params.EnableIPv6CP = next.EnableIPv6CP
		for _, r := range s.reactors {
			r.setEnableIPv6CP(next.EnableIPv6CP)
		}
		s.logger.Info("l2tp reload: enable-ipv6cp updated",
			"previous", prev.EnableIPv6CP, "new", next.EnableIPv6CP)
		applied++
	}
	if prev.NCPTimeout != next.NCPTimeout {
		s.params.NCPTimeout = next.NCPTimeout
		for _, r := range s.reactors {
			r.setNCPTimeout(next.NCPTimeout)
		}
		s.logger.Info("l2tp reload: ncp timeout updated",
			"previous", prev.NCPTimeout.String(), "new", next.NCPTimeout.String())
		applied++
	}

	// Hot-apply: dial targets (remotes) and PPPoE relay bindings. Both are
	// declarative capability consulted only when a NEW dial or relay decision
	// is made; live tunnels are untouched. Updating s.params re-points the
	// resolver the outgoing-call RPC and the relay call-sink read.
	if !remotesEqual(prev.Remotes, next.Remotes) {
		s.params.Remotes = next.Remotes
		s.logger.Info("l2tp reload: dial targets updated (applies to new dials only)",
			"previous", len(prev.Remotes), "new", len(next.Remotes))
		applied++
	}
	if !relaysEqual(prev.Relays, next.Relays) {
		s.params.Relays = next.Relays
		s.logger.Info("l2tp reload: relay bindings updated (applies to new subscribers only)",
			"previous", len(prev.Relays), "new", len(next.Relays))
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
		return Parameters{}, errNilConfigProvider
	}
	l2tpRoot, err := cfg.Get("l2tp")
	if err != nil {
		return Parameters{}, fmt.Errorf("get l2tp root: %w", err)
	}
	if len(l2tpRoot) == 0 {
		return Parameters{}, nil
	}
	p := Parameters{
		Enabled:        true,
		HelloInterval:  time.Duration(DefaultHelloSecs) * time.Second,
		HelloRetries:   DefaultHelloRetries,
		MaxTunnels:     DefaultMaxTunnels,
		MaxSessions:    DefaultMaxSessions,
		AuthMethod:     DefaultAuthMethod,
		AuthTimeout:    DefaultAuthTimeoutSecs * time.Second,
		ReauthInterval: DefaultReauthIntervalSecs * time.Second,
		EnableIPCP:     true,
		EnableIPv6CP:   true,
		NCPTimeout:     DefaultNCPTimeoutSecs * time.Second,
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
			return Parameters{}, errHelloIntervalMustBe0
		}
		p.HelloInterval = time.Duration(n) * time.Second
	}
	if v, ok := l2tpRoot["hello-retries"].(string); ok {
		n, perr := strconv.ParseUint(v, 10, 8)
		if perr != nil {
			return Parameters{}, fmt.Errorf("hello-retries: %w", perr)
		}
		p.HelloRetries = uint8(n)
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
		m, perr := ppp.ParseAuthMethod(v)
		if perr != nil {
			return Parameters{}, fmt.Errorf("auth-method: %w", perr)
		}
		p.AuthMethod = m
	}
	if v, ok := l2tpRoot["allow-no-auth"].(string); ok {
		p.AllowNoAuth = v == configTrue
	}
	if p.AuthMethod == ppp.AuthMethodNone && !p.AllowNoAuth {
		return Parameters{}, errAuthMethodNoneRequiresAllowNo
	}
	if authC, ok := l2tpRoot["authentication"].(map[string]any); ok {
		if v, ok := authC["timeout"].(string); ok {
			n, perr := strconv.ParseUint(v, 10, 16)
			if perr != nil {
				return Parameters{}, fmt.Errorf("authentication timeout: %w", perr)
			}
			p.AuthTimeout = time.Duration(n) * time.Second
		}
		if v, ok := authC["reauth-interval"].(string); ok {
			n, perr := strconv.ParseUint(v, 10, 32)
			if perr != nil {
				return Parameters{}, fmt.Errorf("authentication reauth-interval: %w", perr)
			}
			p.ReauthInterval = time.Duration(n) * time.Second
		}
	}
	if ncpC, ok := l2tpRoot["ncp"].(map[string]any); ok {
		if v, ok := ncpC["enable-ipcp"].(string); ok {
			p.EnableIPCP = v == configTrue
		}
		if v, ok := ncpC["enable-ipv6cp"].(string); ok {
			p.EnableIPv6CP = v == configTrue
		}
		if v, ok := ncpC["timeout"].(string); ok {
			n, perr := strconv.ParseUint(v, 10, 16)
			if perr != nil {
				return Parameters{}, fmt.Errorf("ncp timeout: %w", perr)
			}
			p.NCPTimeout = time.Duration(n) * time.Second
		}
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
			return Parameters{}, errMaxLoginsMustBe11000000
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
			return Parameters{}, errEventRingSizePerSessionMust
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
			return Parameters{}, errSampleRetentionSecondsMustBe100
		}
		p.SampleRetentionSeconds = int(n)
	}
	if err := appendRemotesFromProvider(&p, l2tpRoot); err != nil {
		return Parameters{}, err
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

// appendRemotesFromProvider parses the l2tp/remote and l2tp/relay lists out
// of the provider-map form of the config (Reload path), mirroring the
// config.Tree parse in ExtractParameters. Lists arrive as a map keyed by the
// list key (name for remote, service for relay).
func appendRemotesFromProvider(p *Parameters, l2tpRoot map[string]any) error {
	if remotes, ok := l2tpRoot["remote"].(map[string]any); ok {
		for name, v := range remotes {
			entry, _ := v.(map[string]any)
			if entry == nil {
				return fmt.Errorf("l2tp remote %q: unexpected shape", name)
			}
			address, _ := entry["address"].(string)
			port := strconv.Itoa(DefaultListenPort)
			if s, ok := entry["port"].(string); ok && s != "" {
				port = s
			}
			secret, _ := entry["shared-secret"].(string)
			outgoing := false
			if s, ok := entry["outgoing-calls"].(string); ok {
				outgoing = s == configTrue
			}
			rem, err := buildRemote(name, address, port, secret, outgoing)
			if err != nil {
				return err
			}
			if err := appendRemote(p, rem); err != nil {
				return err
			}
		}
	}
	if relays, ok := l2tpRoot["relay"].(map[string]any); ok {
		for service, v := range relays {
			entry, _ := v.(map[string]any)
			if entry == nil {
				return fmt.Errorf("l2tp relay %q: unexpected shape", service)
			}
			remoteName, _ := entry["remote"].(string)
			p.Relays = append(p.Relays, RelayBinding{Service: service, Remote: remoteName})
		}
	}
	return validateRelays(p)
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

// remotesEqual returns true when both remote slices are identical (same
// entries, same order). Dial targets are keyed by name; the config tree
// preserves declaration order, so a positional comparison is sufficient and
// avoids sorting a secret-bearing struct.
func remotesEqual(a, b []Remote) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// relaysEqual returns true when both relay-binding slices are identical
// (same entries, same order).
func relaysEqual(a, b []RelayBinding) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
	return textbuf.Join(parts, ",")
}

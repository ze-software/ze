// Design: docs/research/l2tpv2-ze-integration.md -- subsystem config extraction
// Related: subsystem.go -- consumes Parameters returned by ExtractParameters

package l2tp

import (
	"fmt"
	"net/netip"
	"strconv"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// Env var registrations. Each YANG leaf under `environment/l2tp/` that has
// a runtime-visible counterpart is listed here; the env value overrides the
// YANG default when both are present. See `rules/config-design.md`.
var (
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.log.l2tp",
		Type:        "string",
		Default:     "warn",
		Description: "Log level for the L2TP subsystem",
		Private:     true,
	})
	// Test-only: skip the modprobe l2tp_ppp / pppol2tp probe at Start.
	// The .ci test harness sets this so show-l2tp-empty.ci can verify
	// the CLI wiring without CAP_NET_ADMIN. Production paths leave it
	// unset, so the real probe runs and surfaces loader errors.
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.l2tp.skip-kernel-probe",
		Type:        "bool",
		Default:     "false",
		Description: "Skip kernel module probe at Start (test-only; bypasses modprobe for L2TP CLI tests)",
		Private:     true,
	})
	// spec-l2tp-6c-ncp: NCP enablement and timeout. Default enabled
	// for both families; ip-timeout bounds how long PPP waits for the
	// IP handler's response after emitting EventIPRequest.
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.l2tp.ncp.enable-ipcp",
		Type:        "bool",
		Default:     "true",
		Description: "Enable the IPCP NCP (RFC 1332) for new L2TP sessions",
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.l2tp.ncp.enable-ipv6cp",
		Type:        "bool",
		Default:     "true",
		Description: "Enable the IPv6CP NCP (RFC 5072) for new L2TP sessions",
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.l2tp.ncp.ip-timeout",
		Type:        "string",
		Default:     "30s",
		Description: "Duration the NCP phase waits for an IP handler response (spec-l2tp-6c-ncp AC-17)",
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.l2tp.metrics.poll-interval",
		Type:        "string",
		Default:     "30s",
		Description: "Interval between pppN interface stats reads for Prometheus counters",
	})
)

// Default listener and protocol values.
const (
	DefaultListenIP           = "0.0.0.0"
	DefaultListenPort         = 1701
	DefaultHelloSecs          = 60
	DefaultMaxTunnels  uint16 = 1024
	DefaultMaxSessions uint16 = 1024
	DefaultAuthMethod         = ppp.AuthMethodCHAPMD5

	configTrue = "true"
)

// Parameters is the parsed L2TP subsystem configuration.
//
// The zero value is a disabled subsystem. Start is safe to call on the zero
// value and is a no-op (returns nil) until Enabled is true.
type Parameters struct {
	Enabled       bool
	ListenAddrs   []netip.AddrPort
	MaxTunnels    uint16
	MaxSessions   uint16
	AuthMethod    ppp.AuthMethod
	AllowNoAuth   bool
	HelloInterval time.Duration
	// SharedSecret is the CHAP-MD5 tunnel authentication secret (RFC 2661
	// S4.2). Empty means peers that include a Challenge AVP in SCCRQ will
	// be rejected with StopCCN Result Code 4 (Not Authorized).
	SharedSecret string

	// CQM observer parameters (spec-l2tp-9-observer).
	CQMEnabled              bool
	MaxLogins               int
	EventRingSizePerSession int
	SampleRetentionSeconds  int
}

// ExtractParameters pulls L2TP configuration out of the parsed config tree.
//
// Protocol settings (enabled, shared-secret, auth-method, allow-no-auth,
// hello-interval, max-tunnels, max-sessions) live under the root-level
// `l2tp {}` block. Listener endpoints live under `environment { l2tp {
// server <name> { ip ...; port ...; } } }`.
//
// Returns a zero-value Parameters (Enabled=false) if no `l2tp {}` block is
// present.
func ExtractParameters(tree *config.Tree) (Parameters, error) {
	if tree == nil {
		return Parameters{}, nil
	}

	// Protocol settings from root-level l2tp{}.
	l2tpRoot := tree.GetContainer("l2tp")
	if l2tpRoot == nil {
		return Parameters{}, nil
	}

	p := Parameters{
		Enabled:       true, // presence of l2tp{} implies enabled
		HelloInterval: time.Duration(DefaultHelloSecs) * time.Second,
		MaxTunnels:    DefaultMaxTunnels,
		MaxSessions:   DefaultMaxSessions,
		AuthMethod:    DefaultAuthMethod,
	}

	if v, ok := l2tpRoot.Get("enabled"); ok {
		p.Enabled = v == configTrue
	}

	if v, ok := l2tpRoot.Get("max-tunnels"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp max-tunnels: %w", err)
		}
		p.MaxTunnels = uint16(n)
	}

	if v, ok := l2tpRoot.Get("max-sessions"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp max-sessions: %w", err)
		}
		p.MaxSessions = uint16(n)
	}

	if v, ok := l2tpRoot.Get("auth-method"); ok {
		m, err := parsePPPAuthMethod(v)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp auth-method: %w", err)
		}
		p.AuthMethod = m
	}

	if v, ok := l2tpRoot.Get("allow-no-auth"); ok {
		p.AllowNoAuth = v == configTrue
	}
	if p.AuthMethod == ppp.AuthMethodNone && !p.AllowNoAuth {
		return Parameters{}, fmt.Errorf("l2tp auth-method none requires allow-no-auth true")
	}

	if v, ok := l2tpRoot.Get("hello-interval"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp hello-interval: %w", err)
		}
		if n == 0 {
			return Parameters{}, fmt.Errorf("l2tp hello-interval: must be > 0")
		}
		p.HelloInterval = time.Duration(n) * time.Second
	}

	if v, ok := l2tpRoot.Get("shared-secret"); ok {
		p.SharedSecret = v
	}

	// CQM observer (spec-l2tp-9-observer).
	if v, ok := l2tpRoot.Get("cqm-enabled"); ok {
		p.CQMEnabled = v == configTrue
	}
	p.MaxLogins = 1000
	if v, ok := l2tpRoot.Get("max-logins"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp max-logins: %w", err)
		}
		if n == 0 || n > 1000000 {
			return Parameters{}, fmt.Errorf("l2tp max-logins: must be 1-1000000")
		}
		p.MaxLogins = int(n)
	}
	p.EventRingSizePerSession = 256
	if v, ok := l2tpRoot.Get("event-ring-size-per-session"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp event-ring-size-per-session: %w", err)
		}
		if n < 16 || n > 4096 {
			return Parameters{}, fmt.Errorf("l2tp event-ring-size-per-session: must be 16-4096")
		}
		p.EventRingSizePerSession = int(n)
	}
	p.SampleRetentionSeconds = 86400
	if v, ok := l2tpRoot.Get("sample-retention-seconds"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp sample-retention-seconds: %w", err)
		}
		if n < 100 || n > 86400 {
			return Parameters{}, fmt.Errorf("l2tp sample-retention-seconds: must be 100-86400")
		}
		p.SampleRetentionSeconds = int(n)
	}

	// Listener endpoints from environment { l2tp { server ... } }.
	if envC := tree.GetContainer("environment"); envC != nil {
		if l2tpEnv := envC.GetContainer("l2tp"); l2tpEnv != nil {
			servers := l2tpEnv.GetListOrdered("server")
			for _, s := range servers {
				ip := DefaultListenIP
				port := strconv.Itoa(DefaultListenPort)
				if v, ok := s.Value.Get("ip"); ok && v != "" {
					ip = v
				}
				if v, ok := s.Value.Get("port"); ok && v != "" {
					port = v
				}
				addr, err := parseListen(ip, port)
				if err != nil {
					return Parameters{}, fmt.Errorf("l2tp server %q: %w", s.Key, err)
				}
				p.ListenAddrs = append(p.ListenAddrs, addr)
			}
		}
	}

	return p, nil
}

func parsePPPAuthMethod(v string) (ppp.AuthMethod, error) {
	switch v {
	case "none":
		return ppp.AuthMethodNone, nil
	case "pap":
		return ppp.AuthMethodPAP, nil
	case "chap-md5":
		return ppp.AuthMethodCHAPMD5, nil
	case "ms-chap-v2":
		return ppp.AuthMethodMSCHAPv2, nil
	}
	return ppp.AuthMethodNone, fmt.Errorf("unsupported method %q", v)
}

func parseListen(ip, port string) (netip.AddrPort, error) {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("ip %q: %w", ip, err)
	}
	p, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("port %q: %w", port, err)
	}
	if p == 0 {
		return netip.AddrPort{}, fmt.Errorf("port %q: must be 1-65535", port)
	}
	return netip.AddrPortFrom(addr, uint16(p)), nil
}

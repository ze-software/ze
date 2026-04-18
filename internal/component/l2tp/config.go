// Design: docs/research/l2tpv2-ze-integration.md -- subsystem config extraction
// Related: subsystem.go -- consumes Parameters returned by ExtractParameters

package l2tp

import (
	"fmt"
	"net/netip"
	"strconv"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
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
)

// Default listener values. Phase 3 only implements a single well-known-port
// listener; phase 7 will add shared-secret, limits, etc.
const (
	DefaultListenIP   = "0.0.0.0"
	DefaultListenPort = 1701
	DefaultHelloSecs  = 60
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
	HelloInterval time.Duration
	// SharedSecret is the CHAP-MD5 tunnel authentication secret (RFC 2661
	// S4.2). Empty means peers that include a Challenge AVP in SCCRQ will
	// be rejected with StopCCN Result Code 4 (Not Authorized).
	SharedSecret string
}

// ExtractParameters pulls L2TP configuration out of the parsed config tree.
//
// Protocol settings (enabled, shared-secret, hello-interval, max-tunnels)
// live under the root-level `l2tp {}` block. Listener endpoints live under
// `environment { l2tp { server <name> { ip ...; port ...; } } }`.
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
	}

	if v, ok := l2tpRoot.Get("enabled"); ok {
		p.Enabled = v == "true"
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

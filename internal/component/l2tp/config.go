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
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.l2tp.enabled",
		Type:        "bool",
		Default:     "false",
		Description: "Enable the L2TP subsystem (overrides YANG environment/l2tp/enabled)",
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.l2tp.max-tunnels",
		Type:        "int",
		Default:     "0",
		Description: "Maximum concurrent L2TP tunnels (0 = unbounded)",
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.l2tp.hello-interval",
		Type:        "int",
		Default:     "60",
		Description: "Seconds of peer silence before sending HELLO",
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.l2tp.shared-secret",
		Type:        "string",
		Description: "Shared secret for CHAP-MD5 tunnel authentication (RFC 2661 S4.2)",
		Secret:      true,
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
	HelloInterval time.Duration
	// SharedSecret is the CHAP-MD5 tunnel authentication secret (RFC 2661
	// S4.2). Empty means peers that include a Challenge AVP in SCCRQ will
	// be rejected with StopCCN Result Code 4 (Not Authorized).
	SharedSecret string
}

// ExtractParameters pulls L2TP configuration out of the parsed config tree.
// Returns a zero-value Parameters (Enabled=false) if no
// `environment { l2tp {} }` block is present.
func ExtractParameters(tree *config.Tree) (Parameters, error) {
	if tree == nil {
		return Parameters{}, nil
	}
	envC := tree.GetContainer("environment")
	if envC == nil {
		return Parameters{}, nil
	}
	l2tpC := envC.GetContainer("l2tp")
	if l2tpC == nil {
		return Parameters{}, nil
	}

	p := Parameters{
		HelloInterval: time.Duration(DefaultHelloSecs) * time.Second,
	}

	if v, ok := l2tpC.Get("enabled"); ok {
		p.Enabled = v == "true"
	}

	servers := l2tpC.GetListOrdered("server")
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

	if v, ok := l2tpC.Get("max-tunnels"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp max-tunnels: %w", err)
		}
		p.MaxTunnels = uint16(n)
	}

	if v, ok := l2tpC.Get("hello-interval"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp hello-interval: %w", err)
		}
		if n == 0 {
			return Parameters{}, fmt.Errorf("l2tp hello-interval: must be > 0")
		}
		p.HelloInterval = time.Duration(n) * time.Second
	}

	if v, ok := l2tpC.Get("shared-secret"); ok {
		p.SharedSecret = v
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

// Design: docs/architecture/ospf/ospf-ext-2-traffic-engineering.md -- traffic-engineering config.
// RFC: rfc/short/rfc3630.md (per-link TE attributes), rfc/short/rfc5392.md (inter-AS).
//
// The per-interface `traffic-engineering` block and its `inter-as` sub-block are TE-owned
// config, kept out of the generic config.go so removing the TE consumer removes all TE
// spelling (ai/rules/plugins.md). Cross-field validation (inter-as needs a
// remote-as and at least one remote-asbr, RFC 5392 sec 3.2.1/3.3.1) lives in the plugin's
// validateConfig, which is the authoritative in-process verifier: a per-leaf YANG
// ze:validate cannot express a requirement spanning sibling leaves.

package ospf

import (
	"errors"
	"fmt"
	"net/netip"
)

var (
	ErrTERouterAddress     = errors.New("ospf: traffic-engineering router-address must be a valid IPv4 address")
	ErrTEMetricRange       = errors.New("ospf: traffic-engineering te-metric out of range (0..4294967295)")
	ErrTEAdminGroupRange   = errors.New("ospf: traffic-engineering admin-group out of range (0..4294967295)")
	ErrTERemoteASRange     = errors.New("ospf: traffic-engineering inter-as remote-as out of range (0..4294967295)")
	ErrTEInterASRemoteAS   = errors.New("ospf: traffic-engineering inter-as requires remote-as (RFC 5392 sec 3.3.1)")
	ErrTEInterASRemoteASBR = errors.New("ospf: traffic-engineering inter-as requires at least one remote-asbr (IPv4 or IPv6) (RFC 5392 sec 3.2.1)")
	ErrTEInterASScope      = errors.New("ospf: traffic-engineering inter-as scope must be area or as (RFC 5392 sec 3.1.1)")
	ErrTERemoteASBRv4      = errors.New("ospf: traffic-engineering inter-as remote-asbr-ipv4 must be a valid IPv4 address")
	ErrTERemoteASBRv6      = errors.New("ospf: traffic-engineering inter-as remote-asbr-ipv6 must be a valid IPv6 address")
)

// teConfig is the per-interface `traffic-engineering` block (RFC 3630 sec 2.5 link
// attributes plus the RFC 5392 inter-AS block). Bandwidth is stored as float64 (bytes/sec)
// to match the wire IEEE-754 and the rsvpte admission representation. The TE metric is a
// SEPARATE uint32 (RFC 3630 sec 2.5.5) and must not alias interfaceConfig.Cost.
type teConfig struct {
	Enabled          bool
	Metric           uint32
	HasMetric        bool
	MaxBandwidth     float64 // bytes/sec (sub-TLV 6)
	HasMaxBandwidth  bool
	MaxReservable    float64 // bytes/sec (sub-TLV 7)
	HasMaxReservable bool
	AdminGroup       uint32 // 32-bit mask (sub-TLV 9)
	HasAdminGroup    bool
	// InterAS, when non-nil, makes this an RFC 5392 inter-AS TE link (Opaque type 6): no
	// OSPF adjacency, a proxied advertisement carrying the remote AS + remote ASBR.
	InterAS *interASConfig
}

// active reports whether TE should originate for this interface: TE explicitly enabled, or
// an inter-AS proxy link configured (RFC 5392 sec 4 advertises even without an adjacency).
// nil-safe: an interface with no traffic-engineering block is not active.
func (te *teConfig) active() bool { return te != nil && (te.Enabled || te.InterAS != nil) }

// interASConfig is the RFC 5392 inter-AS TE sub-block: the remote AS, at least one remote
// ASBR identifier, and the Type 10 vs Type 11 flooding-scope policy (RFC 5392 sec 3.1.1).
type interASConfig struct {
	RemoteAS        uint32
	HasRemoteAS     bool
	RemoteASBRv4    [4]byte
	HasRemoteASBRv4 bool
	RemoteASBRv6    [16]byte
	HasRemoteASBRv6 bool
	// Scope is OpaqueScopeArea (Type 10, the sec 3.1.1 SHOULD default) or OpaqueScopeAS
	// (Type 11, AS-wide) by operator policy.
	Scope OpaqueScope
}

// parseTE resolves the per-interface `traffic-engineering` block (RFC 3630 / RFC 5392).
// Ranges are also enforced natively in YANG; the guards here defend the non-YANG doctor
// and verifier parse paths against a value that would silently truncate.
func parseTE(m map[string]any, ifaceName string) (teConfig, error) {
	te := teConfig{Enabled: configBool(m["enable"], false)}
	if v, ok := configNumber(m["te-metric"]); ok {
		// RFC 3630 sec 2.5.5: the TE metric is a uint32 independent of the OSPF cost.
		if v > 0xFFFFFFFF {
			return te, fmt.Errorf("%w: interface %q", ErrTEMetricRange, ifaceName)
		}
		te.Metric = uint32(v)
		te.HasMetric = true
	}
	if v, ok := configNumber(m["max-bandwidth"]); ok {
		te.MaxBandwidth = float64(v)
		te.HasMaxBandwidth = true
	}
	if v, ok := configNumber(m["max-reservable-bandwidth"]); ok {
		te.MaxReservable = float64(v)
		te.HasMaxReservable = true
	}
	if v, ok := configNumber(m["admin-group"]); ok {
		if v > 0xFFFFFFFF {
			return te, fmt.Errorf("%w: interface %q", ErrTEAdminGroupRange, ifaceName)
		}
		te.AdminGroup = uint32(v)
		te.HasAdminGroup = true
	}
	if ia, ok := m["inter-as"].(map[string]any); ok {
		cfg, err := parseInterAS(ia, ifaceName)
		if err != nil {
			return te, err
		}
		te.InterAS = &cfg
	}
	return te, nil
}

// parseInterAS resolves the `traffic-engineering inter-as` sub-block (RFC 5392). The
// cross-field requirement (remote-as plus at least one remote-asbr) is enforced in
// validateTEInterface; per-leaf format is validated here and natively in YANG.
func parseInterAS(m map[string]any, ifaceName string) (interASConfig, error) {
	ia := interASConfig{Scope: OpaqueScopeArea}
	if v, ok := configNumber(m["remote-as"]); ok {
		if v > 0xFFFFFFFF {
			return ia, fmt.Errorf("%w: interface %q", ErrTERemoteASRange, ifaceName)
		}
		ia.RemoteAS = uint32(v)
		ia.HasRemoteAS = true
	}
	if s := configString(m["remote-asbr-ipv4"]); s != "" {
		addr, err := netip.ParseAddr(s)
		if err != nil || !addr.Is4() {
			return ia, fmt.Errorf("%w: interface %q value %q", ErrTERemoteASBRv4, ifaceName, s)
		}
		ia.RemoteASBRv4 = addr.As4()
		ia.HasRemoteASBRv4 = true
	}
	if s := configString(m["remote-asbr-ipv6"]); s != "" {
		addr, err := netip.ParseAddr(s)
		if err != nil || addr.Is4() || !addr.Is6() {
			return ia, fmt.Errorf("%w: interface %q value %q", ErrTERemoteASBRv6, ifaceName, s)
		}
		ia.RemoteASBRv6 = addr.As16()
		ia.HasRemoteASBRv6 = true
	}
	if s := configString(m["scope"]); s != "" {
		switch s {
		case scopeAreaName:
			ia.Scope = OpaqueScopeArea
		case "as":
			ia.Scope = OpaqueScopeAS
		default:
			return ia, fmt.Errorf("%w: interface %q value %q", ErrTEInterASScope, ifaceName, s)
		}
	}
	return ia, nil
}

// validateTEInterface enforces the RFC 5392 inter-as cross-field requirements: a Link TLV
// advertising an inter-AS link MUST carry the Remote AS Number sub-TLV (sec 3.3.1) and
// SHOULD carry at least one Remote ASBR ID (sec 3.2.1); Ze requires the latter so an
// originated inter-AS LSA always identifies the remote end.
func validateTEInterface(ic interfaceConfig) error {
	if ic.TE == nil {
		return nil
	}
	ia := ic.TE.InterAS
	if ia == nil {
		return nil
	}
	if !ia.HasRemoteAS {
		return fmt.Errorf("%w: interface %q", ErrTEInterASRemoteAS, ic.Name)
	}
	if !ia.HasRemoteASBRv4 && !ia.HasRemoteASBRv6 {
		return fmt.Errorf("%w: interface %q", ErrTEInterASRemoteASBR, ic.Name)
	}
	return nil
}

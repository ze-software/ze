// Design: plan/learned/1313-rfcgate-1b-rfc7296-pilot.md -- operator traffic-selector policy for IKEv2 narrowing
// Related: validate.go -- the ValidateTrafficSelectors entry in the OnConfigVerify chain
// Related: config.go -- parseSiteToSitePeer, which fills these types from the config tree
// RFC: rfc/short/rfc7296.md -- Traffic Selector negotiation (Section 2.9), port encoding (Section 3.13.1)

package ipsec

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// sortedPeerNames orders the peer map so a config with two faults always reports the
// same one first. A map walk would report either, which makes a rejection message
// depend on hash order and makes its test flaky.
func sortedPeerNames(peers map[string]SiteToSitePeer) []string {
	names := make([]string, 0, len(peers))
	for name := range peers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// PortForm names how one traffic selector states its port range.
//
// RFC 7296 Section 3.13.1 gives three encodings and they are not interchangeable.
// The zero value is deliberately invalid, so a field nobody filled can never read as
// a valid form (ai/rules/fail-closed-guards.md). parseTrafficSelectorPort normalizes
// an absent leaf to PortAny, which is the RFC's own default and today's behavior.
type PortForm uint8

const (
	// PortAny is start 0, end 65535. RFC 7296 Section 3.13.1: a system that wishes to
	// indicate ANY ports MUST set the start port to 0 and the end port to 65535, and
	// ANY includes OPAQUE.
	PortAny PortForm = 1
	// PortSingle is start N, end N, for one specific port.
	PortSingle PortForm = 2
	// PortOpaque is start 65535, end 0. RFC 7296 Section 3.13.1: a system that wishes
	// to indicate OPAQUE ports, but not ANY ports, MUST use that inverted pair.
	PortOpaque PortForm = 3
)

// PortSelector is the port half of one traffic selector.
type PortSelector struct {
	Form PortForm
	// Port is the single port number, and it is read only when Form is PortSingle.
	Port uint16
}

// AnyPort returns the selector that matches every port.
func AnyPort() PortSelector { return PortSelector{Form: PortAny} }

// Wire returns the start and end port octet pair RFC 7296 Section 3.13.1 requires for
// this form. It is the single producer of that encoding, so the three MUSTs of Section
// 3.13.1 are decided in one place rather than at each call site.
func (p PortSelector) Wire() (start, end uint16) {
	switch p.Form {
	case PortSingle:
		return p.Port, p.Port
	case PortOpaque:
		return 65535, 0
	default:
		return 0, 65535
	}
}

// PortSelectorFromWire reads a start/end pair back into a form. The second result is
// false for a range Ze cannot express, which is every inclusive range that is neither
// the ANY pair, nor a single port, nor the OPAQUE pair.
//
// It fails closed. A range such as 1024..2048 has no Ze representation and no XFRM
// representation, so it must never be rounded to ANY: that would answer a peer with
// more traffic than it proposed, which RFC 7296 Section 2.9 forbids.
func PortSelectorFromWire(start, end uint16) (PortSelector, bool) {
	switch {
	case start == 0 && end == 65535:
		return PortSelector{Form: PortAny}, true
	case start == 65535 && end == 0:
		return PortSelector{Form: PortOpaque}, true
	case start == end:
		return PortSelector{Form: PortSingle, Port: start}, true
	default:
		return PortSelector{}, false
	}
}

// String renders the operator spelling of this form.
func (p PortSelector) String() string {
	switch p.Form {
	case PortSingle:
		return portDecimal(p.Port)
	case PortOpaque:
		return "opaque"
	default:
		return "any"
	}
}

func portDecimal(v uint16) string {
	if v == 0 {
		return "0"
	}
	var digits [5]byte
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
	}
	return string(digits[i:])
}

// TrafficSelectorPolicy is one configured selector pair for a site-to-site peer.
//
// It is the operator policy RFC 7296 Section 2.9 narrows a peer's proposal against.
// Before this type existed Ze had no such policy, so the responder answered every
// IKE_AUTH with a full wildcard and there was nothing to narrow to.
type TrafficSelectorPolicy struct {
	Number       string
	LocalPrefix  *net.IPNet
	LocalPort    PortSelector
	RemotePrefix *net.IPNet
	RemotePort   PortSelector
	// Protocol is the IP protocol number the selector matches. Zero is every protocol,
	// which is RFC 7296 Section 3.13.1's "protocol 0".
	Protocol uint8
}

// ErrTrafficSelectorPolicy marks a traffic-selector configuration Ze refuses.
var ErrTrafficSelectorPolicy = errors.New("ipsec traffic-selector")

// ParseChildMode reads the peer mode leaf. The second result is false for a value
// outside the enumeration, and the caller then refuses the config rather than
// defaulting: a mode nobody recognized must not silently become tunnel.
func ParseChildMode(v string) (uint8, bool) {
	switch v {
	case "tunnel":
		return dataplane.ModeTunnel, true
	case "transport":
		return dataplane.ModeTransport, true
	default:
		return 0, false
	}
}

// parseTrafficSelectors reads the peer's traffic-selector list from the config tree.
// An absent list yields no entries, which ValidateTrafficSelectors accepts and the
// narrowing engine reads as "allow everything".
func parseTrafficSelectors(peerName string, t *config.Tree) ([]TrafficSelectorPolicy, error) {
	entries := t.GetListOrdered("traffic-selector")
	if len(entries) == 0 {
		return nil, nil
	}
	out := make([]TrafficSelectorPolicy, 0, len(entries))
	for _, entry := range entries {
		ts, err := parseTrafficSelector(peerName, entry.Key, entry.Value)
		if err != nil {
			return nil, err
		}
		out = append(out, ts)
	}
	return out, nil
}

func parseTrafficSelector(peerName, number string, t *config.Tree) (TrafficSelectorPolicy, error) {
	ts := TrafficSelectorPolicy{
		Number:     number,
		LocalPort:  AnyPort(),
		RemotePort: AnyPort(),
	}

	if v, ok := t.Get("protocol"); ok {
		proto, err := strconv.ParseUint(v, 10, 8)
		if err != nil {
			return ts, fmt.Errorf("%w: peer %q traffic-selector %q protocol: %w", ErrTrafficSelectorPolicy, peerName, number, err)
		}
		ts.Protocol = uint8(proto)
	}

	for _, side := range []struct {
		name   string
		prefix **net.IPNet
		port   *PortSelector
	}{
		{"local", &ts.LocalPrefix, &ts.LocalPort},
		{"remote", &ts.RemotePrefix, &ts.RemotePort},
	} {
		sub := t.GetContainer(side.name)
		if sub == nil {
			continue
		}
		if v, ok := sub.Get("prefix"); ok {
			_, n, err := net.ParseCIDR(v)
			if err != nil {
				return ts, fmt.Errorf("%w: peer %q traffic-selector %q %s prefix %q: %w",
					ErrTrafficSelectorPolicy, peerName, number, side.name, v, err)
			}
			*side.prefix = n
		}
		if v, ok := sub.Get("port"); ok {
			p, err := parsePortSelector(peerName, number, side.name, v)
			if err != nil {
				return ts, err
			}
			*side.port = p
		}
	}

	return ts, nil
}

func parsePortSelector(peerName, number, side, v string) (PortSelector, error) {
	switch v {
	case "", "any":
		return PortSelector{Form: PortAny}, nil
	case "opaque":
		return PortSelector{Form: PortOpaque}, nil
	}
	port, err := strconv.ParseUint(v, 10, 16)
	if err != nil || port == 0 {
		return PortSelector{}, fmt.Errorf("%w: peer %q traffic-selector %q %s port %q is not a port; accepted values are any, opaque, or a port number in 1..65535",
			ErrTrafficSelectorPolicy, peerName, number, side, v)
	}
	return PortSelector{Form: PortSingle, Port: uint16(port)}, nil
}

// ValidateTrafficSelectors refuses any configured selector the dataplane cannot
// program byte for byte, and any selector that contradicts the peer's negotiated mode.
//
// ai/rules/exact-or-reject.md: a backend that cannot apply the operator's config
// EXACTLY must fail at verify, never approximate at run time. Every rejection below
// names the offending value and the accepted alternatives (ai/rules/error-messages.md).
//
// It is one of two homes for the rule. The peer's PROPOSAL never passes through here,
// because it is attacker-controlled and arrives after commit, so the narrowing engine
// applies the same programmability predicate at negotiation time (engine/ts_narrow.go).
func (c *IPsecConfig) ValidateTrafficSelectors() error {
	for _, name := range sortedPeerNames(c.Peers) {
		peer := c.Peers[name]
		if err := validatePeerSelectors(peer); err != nil {
			return err
		}
	}
	return nil
}

func validatePeerSelectors(peer SiteToSitePeer) error {
	transport := peer.Mode == dataplane.ModeTransport

	if transport && peer.VTIBind != "" {
		return fmt.Errorf("%w: peer %q sets mode transport with vti bind %q: an XFRM interface carries tunnel-mode encapsulation, so the two cannot both apply; drop the vti binding or set mode tunnel",
			ErrTrafficSelectorPolicy, peer.Name, peer.VTIBind)
	}
	if !transport && peer.TransportRequired {
		return fmt.Errorf("%w: peer %q sets transport-required true with mode tunnel: transport-required only governs a peer that proposes transport mode; set mode transport or clear transport-required",
			ErrTrafficSelectorPolicy, peer.Name)
	}

	seen := make(map[string]bool, len(peer.TrafficSelectors))
	for _, ts := range peer.TrafficSelectors {
		if seen[ts.Number] {
			return fmt.Errorf("%w: peer %q declares traffic-selector %q twice", ErrTrafficSelectorPolicy, peer.Name, ts.Number)
		}
		seen[ts.Number] = true

		if ts.LocalPrefix == nil {
			return fmt.Errorf("%w: peer %q traffic-selector %q: local prefix is required", ErrTrafficSelectorPolicy, peer.Name, ts.Number)
		}
		if ts.RemotePrefix == nil {
			return fmt.Errorf("%w: peer %q traffic-selector %q: remote prefix is required", ErrTrafficSelectorPolicy, peer.Name, ts.Number)
		}
		if err := checkPortProgrammable(peer.Name, ts.Number, "local", ts.LocalPort, ts.Protocol); err != nil {
			return err
		}
		if err := checkPortProgrammable(peer.Name, ts.Number, "remote", ts.RemotePort, ts.Protocol); err != nil {
			return err
		}
		if transport {
			if err := checkSingleHost(peer.Name, ts.Number, "local", ts.LocalPrefix); err != nil {
				return err
			}
			if err := checkSingleHost(peer.Name, ts.Number, "remote", ts.RemotePrefix); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkPortProgrammable refuses a port form no backend can express, and refuses a port
// on a protocol that has none.
//
// RFC 7296 Section 3.13.1: "For protocols for which port is undefined (including
// protocol 0) ... this field MUST be zero" and the end port MUST be 65535. So a single
// port under protocol 0 is a config that cannot be encoded conformantly, and it is
// refused rather than silently widened to ANY.
func checkPortProgrammable(peerName, number, side string, p PortSelector, proto uint8) error {
	switch p.Form {
	case PortAny:
		return nil
	case PortSingle:
		if proto == 0 {
			return fmt.Errorf("%w: peer %q traffic-selector %q: %s port %d needs a protocol, because RFC 7296 Section 3.13.1 requires the port fields to be 0 and 65535 whenever the protocol is 0; set protocol to 6 (tcp), 17 (udp), or another protocol that defines ports, or set the port to any",
				ErrTrafficSelectorPolicy, peerName, number, side, p.Port)
		}
		return nil
	case PortOpaque:
		// ai/rules/exact-or-reject.md. RFC 7296 Section 3.13.1 gives OPAQUE ports the
		// encoding start 65535 / end 0, and Ze can PARSE that from a peer. It cannot
		// PROGRAM it: the kernel policy selector derives its port mask from the port
		// value, so a request for exactly port 0 installs as any-port, which protects
		// more traffic than OPAQUE names. Refusing at commit is the honest answer;
		// accepting and widening at install time is not.
		return fmt.Errorf("%w: peer %q traffic-selector %q: %s port opaque cannot be programmed by any dataplane backend Ze has, because the kernel policy selector derives its port mask from the port value and an exact match on port 0 is not expressible; use any, or a port number in 1..65535",
			ErrTrafficSelectorPolicy, peerName, number, side)
	default:
		return fmt.Errorf("%w: peer %q traffic-selector %q: %s port form %d is not a form Ze can encode; accepted values are any, opaque, or a port number in 1..65535",
			ErrTrafficSelectorPolicy, peerName, number, side, uint8(p.Form))
	}
}

// checkSingleHost refuses a transport-mode selector wider than one address.
//
// RFC 7296 Section 2.23.1 MUST: "For transport mode, it MUST use exactly one IP address
// in the TSi and TSr payloads." A prefix wider than a host cannot satisfy that, so it is
// refused at commit rather than narrowed silently at negotiation time.
func checkSingleHost(peerName, number, side string, n *net.IPNet) error {
	ones, bits := n.Mask.Size()
	if bits == 0 || ones != bits {
		return fmt.Errorf("%w: peer %q traffic-selector %q: %s prefix %s is wider than one address, and RFC 7296 Section 2.23.1 requires transport mode to use exactly one IP address in TSi and TSr; use a /32 (IPv4) or /128 (IPv6) prefix, or set mode tunnel",
			ErrTrafficSelectorPolicy, peerName, number, side, n.String())
	}
	return nil
}

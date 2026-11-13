// Design: docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md -- operator traffic-selector policy for IKEv2 narrowing
// Related: validate.go -- the ValidateTrafficSelectors entry in the OnConfigVerify chain
// Related: config.go -- parseSiteToSitePeer, which fills these types from the config tree
// RFC: rfc/short/rfc7296.md -- Traffic Selector negotiation (Section 2.9), port encoding (Section 3.13.1)

package ipsec

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/opererr"
)

// sortedPeerNames orders the peer map so a config with two faults always reports the
// same one first. A map walk would report either, which makes a rejection message
// depend on hash order and makes its test flaky.
func sortedPeerNames(peers map[string]SiteToSitePeer) []string {
	names := make([]string, 0, len(peers))
	for name := range peers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// PortForm names how one traffic selector states its port range.
//
// RFC 7296 Section 3.13.1 gives three encodings and they are not interchangeable.
// Zero is not a parsed form and checkPortProgrammable rejects it. Wire and String
// retain the existing ANY representation for an unset selector. parsePortSelector
// normalizes an absent leaf to PortAny, which is the RFC's own default.
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

// sideLocal and sideRemote name the two ends of a selector as the config spells
// their containers, and as a refusal names the side it judged. Declared once so the
// peer traffic-selector list and the operator SPD policy list read the same words.
const (
	sideLocal  = "local"
	sideRemote = "remote"
)

// AnyPort returns the selector that matches every port.
// portAnyKeyword is the word an operator writes for "every port", in the peer
// traffic-selector list and in the operator SPD policy list alike. Declared once so
// the two surfaces cannot drift onto different spellings of one concept.
const portAnyKeyword = "any"

func AnyPort() PortSelector { return PortSelector{Form: PortAny} }

// IsAny reports whether this selector constrains nothing, which is the ANY of RFC
// 4301 Section 4.4.1.1. OPAQUE is NOT any: it says the field could not be read, and
// reading it as "every port" would widen a rule to traffic nobody named.
func (p PortSelector) IsAny() bool { return p.Form == PortAny }

// ValidForProtocol checks the wire meaning, independently of backend support.
// RFC 7296 Section 3.13.1: "For protocols for which port is undefined (including
// protocol 0), or if all ports are allowed, this field MUST be zero."
// End Port: "For protocols for which port is undefined (including protocol 0),
// or if all ports are allowed, this field MUST be 65535."
//
// Traffic Selector substructure, RFC 7296 Section 3.13.1 (byte offsets):
//
//	0       1          2                 4                 6               8
//	+-------+----------+-----------------+-----------------+---------------+
//	| Type  | Protocol | Selector Length | Start Port      | End Port      |
//	+-------+----------+-----------------+-----------------+---------------+
//	| Starting Address, then Ending Address (4 or 16 bytes each)           |
//	+--------------------------------------------------------------------+
//
// For MH, each two-byte port is [Type][0]. Port retains that IKE value.
func (p PortSelector) ValidForProtocol(proto uint8) bool {
	switch p.Form {
	case PortAny, 0:
		return true
	case PortSingle, PortOpaque:
		if protocolCarriesPorts(proto) {
			return true
		}
		if dataplane.IsICMPProto(proto) {
			return true
		}
		if proto == MobilityHeaderProtocol {
			if p.Form == PortOpaque {
				return true
			}
			// RFC 7296 Section 3.13.1: "MIPv6 MH Type values are treated as a
			// single 16-bit integer port number, with Type in the most significant
			// eight bits and the least significant eight bits set to zero."
			return p.Port&0xff == 0
		}
	}
	return false
}

// ProgrammableForProtocol additionally checks what the Linux classifier can
// select. include/net/xfrm.h, xfrm_flowi_sport/dport handle UDP-Lite but omit
// DCCP. Linux7.3 decode_session6 does not fill the MH Type either: actual
// incoming/forward probes deliver ANY MH but drop a matching exact Type.
// OPAQUE has no dataplane mapping. The legacy unset form retains the
// ANY meaning Wire gives it; config verification separately rejects that form.
func (p PortSelector) ProgrammableForProtocol(proto uint8) bool {
	// RFC 7296 Section 3.13.1
	if !p.ValidForProtocol(proto) {
		return false
	}
	switch p.Form {
	case PortAny, 0:
		return true
	case PortSingle:
		switch proto {
		case protoDCCP, MobilityHeaderProtocol:
			return false
		}
		return true
	default:
		return false
	}
}

// Wire returns the start and end port octet pair RFC 7296 Section 3.13.1 requires for
// this form. It is the single producer of that encoding, so the three MUSTs of Section
// 3.13.1 are decided in one place rather than at each call site.
func (p PortSelector) Wire() (start, end uint16) {
	switch p.Form {
	case PortSingle:
		return p.Port, p.Port
	case PortOpaque:
		return 65535, 0
	case PortAny, 0:
		return 0, 65535
	default:
		// Wire ranges are normalized by PortSelectorFromWire or rejected;
		// no received numeric code is used directly as a PortForm.
		panic("BUG: unknown traffic selector port form")
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
	case PortAny, 0:
		return portAnyKeyword
	default:
		// Parsing and wire conversion assign named forms; zero is handled
		// above for callers that have not populated a selector.
		panic("BUG: unknown traffic selector port form")
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

// parseChildMode reads the peer mode leaf. The second result is false for a value
// outside the enumeration, and the caller then refuses the config rather than
// defaulting: a mode nobody recognized must not silently become tunnel.
func parseChildMode(v string) (uint8, bool) {
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
		{sideLocal, &ts.LocalPrefix, &ts.LocalPort},
		{sideRemote, &ts.RemotePrefix, &ts.RemotePort},
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
	case "", portAnyKeyword:
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
// ai/rules/protocol.md: a backend that cannot apply the operator's config
// EXACTLY must fail at verify, never approximate at run time. Every rejection below
// names the offending value and the accepted alternatives (ai/rules/cli.md).
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
		if err := checkICMPFamily(peer.Name, ts); err != nil {
			return err
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

// checkICMPFamily refuses an ICMP type and code selector under the ICMP protocol of
// the other address family: protocol 1 with an IPv6 prefix, or 58 with an IPv4 one.
// Under ICMP the "port" of a selector carries the message type and code (RFC 4301
// Section 4.4.1.3), and the two protocols number their types apart, so such a
// selector would install a policy that matches no packet and protects nothing.
//
// RFC 4301 Section 4.4.1.1: "If the Next Layer Protocol value is ICMP, then there is a
// 16-bit selector for the ICMP message type and code."
func checkICMPFamily(peerName string, ts TrafficSelectorPolicy) error {
	if !dataplane.IsICMPProto(ts.Protocol) {
		return nil
	}
	if ts.LocalPort.IsAny() && ts.RemotePort.IsAny() {
		return nil
	}
	for _, side := range []struct {
		name   string
		prefix *net.IPNet
	}{{sideLocal, ts.LocalPrefix}, {sideRemote, ts.RemotePrefix}} {
		family := icmpFamilyOf(side.prefix)
		if ts.Protocol == family.proto {
			continue
		}
		return fmt.Errorf("%w: peer %q traffic-selector %q: the ICMP type and code in its port select %s messages "+
			"under protocol %d, but %s prefix %s is %s, which carries %s as protocol %d, so the selector would match no packet; "+
			"set protocol to %d, or use %s prefixes",
			ErrTrafficSelectorPolicy, peerName, ts.Number, family.otherMessage, ts.Protocol,
			side.name, side.prefix, family.name, family.message, family.proto, family.proto, family.otherName)
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
		fix := fmt.Sprintf("set vpn ipsec site-to-site peer %q traffic-selector %s %s port any", peerName, number, side)
		if !p.ValidForProtocol(proto) {
			why := fmt.Sprintf("protocol %d defines no port selector; RFC 7296 Section 3.13.1 requires ANY ports for protocols with undefined ports", proto)
			if proto == MobilityHeaderProtocol {
				why = "RFC 7296 Section 3.13.1 puts the MH Type in the high octet and requires the low octet to be zero; type 5 is port 1280"
			}
			return opererr.Wrap(ErrTrafficSelectorPolicy,
				fmt.Sprintf("peer %q traffic-selector %q: %s port %d is invalid for protocol %d", peerName, number, side, p.Port, proto), why, fix)
		}
		if !p.ProgrammableForProtocol(proto) {
			if proto == MobilityHeaderProtocol {
				return opererr.Wrap(ErrTrafficSelectorPolicy,
					fmt.Sprintf("peer %q traffic-selector %q: %s names a specific IPv6 Mobility Header Type", peerName, number, side),
					"Linux's receive-flow decoder does not fill the MH Type used by XFRM; a policy naming an exact Type drops even matching received and forwarded packets, so Ze cannot program this selector exactly", fix)
			}
			return opererr.Wrap(ErrTrafficSelectorPolicy,
				fmt.Sprintf("peer %q traffic-selector %q: %s names a specific DCCP port", peerName, number, side),
				"DCCP defines ports, but Linux XFRM does not copy DCCP ports into its flow selector, so the negotiated port cannot be installed exactly", fix)
		}
		return nil
	case PortOpaque:
		// ai/rules/protocol.md. RFC 7296 Section 3.13.1 gives OPAQUE ports the
		// encoding start 65535 / end 0, and Ze can PARSE that from a peer. It does not
		// PROGRAM it yet: OPAQUE names a port that is not visible in the packet, and
		// which kernel selector carries that meaning is the work of
		// plan/post-release/features/spec-ipsec-opaque-selector-port-mask.md. Refusing at
		// commit is the honest answer; installing a guess is not.
		return fmt.Errorf("%w: peer %q traffic-selector %q: %s port opaque is not programmed by Ze yet, because no dataplane mapping of a port that is not visible in the packet is implemented; use any, or a port number in 1..65535",
			ErrTrafficSelectorPolicy, peerName, number, side)
	case 0:
		return fmt.Errorf("%w: peer %q traffic-selector %q: %s port form %d is not a form Ze can encode; accepted values are any, opaque, or a port number in 1..65535",
			ErrTrafficSelectorPolicy, peerName, number, side, uint8(p.Form))
	default:
		panic("BUG: unknown traffic selector port form")
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

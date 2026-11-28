// Design: docs/architecture/diagnostics/path-mtu.md -- encapsulation overhead, derived per tunnel
// RFC: rfc/short/rfc4303.md -- the ESP packet layout these octets add up
// RFC: rfc/short/rfc2784.md -- GRE base header and optional checksum
// RFC: rfc/short/rfc2890.md -- GRE key and sequence extensions
// RFC: rfc/short/rfc2473.md -- IPv6 tunneling and the Tunnel Encapsulation Limit header
// RFC: rfc/short/rfc2003.md -- IPv4-in-IPv4 encapsulation
// RFC: rfc/short/rfc4213.md -- IPv6-in-IPv4 (SIT) encapsulation
// Related: arith.go -- the ceiling, the recommended value and the MSS built on this overhead
// Related: internal/component/ike/crypto/transform.go -- the transform ids and the ICV lengths

package cmd

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/core/ipsecinventory"
)

// The fixed header sizes the tunnel overhead arithmetic adds up, in octets.
const (
	ipv4HeaderOctets = 20
	ipv6HeaderOctets = 40
	udpHeaderOctets  = 8
	tcpHeaderOctets  = 20

	// espHeaderOctets is the SPI and the Sequence Number. RFC 4303 Section 2:
	// "The packet begins with two 4-byte fields (Security Parameters Index
	// (SPI) and Sequence Number)".
	espHeaderOctets = 8

	// espTrailerOctets is the Pad Length and the Next Header octet that end
	// every ESP plaintext. RFC 4303 Section 2: "The (transmitted) ESP trailer
	// consists of the Padding, Pad Length, and Next Header fields." The
	// padding is what the alignment below accounts for; these two are fixed.
	espTrailerOctets = 2

	// greBaseHeaderOctets is the mandatory 4-octet GRE header (flags/version
	// plus protocol type, RFC 2784 Section 2.1).
	greBaseHeaderOctets = 4

	// greOptionOctets is the size of each optional GRE extension field:
	// Checksum/Reserved1 (RFC 2784), Key (RFC 2890), and Sequence Number
	// (RFC 2890).
	greOptionOctets = 4

	// ethernetHeaderOctets is the inner 14-octet Ethernet frame header carried
	// by L2 tunnels (gretap, ip6gretap, vxlan).
	ethernetHeaderOctets = 14

	// vxlanHeaderOctets is the 8-octet VXLAN header (RFC 7348 Section 5).
	vxlanHeaderOctets = 8

	// ipv6EncapLimitOctets is the 8-octet IPv6 Destination Options header
	// carrying the Tunnel Encapsulation Limit TLV (RFC 2473 Section 4.1.1).
	ipv6EncapLimitOctets = 8

	// wireguardDataOverheadOctets is the 16-octet Type 4 transport data header
	// plus the 16-octet Poly1305 authentication tag. Linux WireGuard caps its
	// 16-octet plaintext padding at the device MTU (calculate_skb_padding), so
	// a full-MTU packet incurs 0 padding octets on the wire.
	wireguardDataOverheadOctets = 32
)

// errOverheadRefused is the answer when the overhead cannot be derived for a
// tunnel. The caller reports the tunnel as not advisable and computes nothing
// from it: a default overhead would size a tunnel from a figure nobody
// derived (ai/rules/principles.md).
var errOverheadRefused = errors.New("mtu: the encapsulation overhead cannot be derived")

// encapOverhead is what one tunnel encapsulation adds to a packet, split the
// way the ceiling arithmetic needs it (arith.go).
//
// In tunnel mode the packet handed to the encapsulation is wrapped in a new
// outer header: the expansion is outerHeader plus fixed plus any block padding
// plus trailer. In ESP transport mode the packet keeps its own IP header and
// ESP is inserted behind it (RFC 4303 Section 3.1.1), so outerHeader is the
// packet's own header rather than an addition, and only the part after it is
// padded. Non-IPsec tunnels use ModeTunnel with trailer 0 and block 1.
type encapOverhead struct {
	mode ipsecinventory.Mode
	// outerHeader is the IP header carrying the tunnel: 20 for IPv4, 40 for IPv6.
	outerHeader uint16
	// fixed is every encapsulation octet beyond outerHeader that does not
	// depend on the payload size (UDP/ESP/IV/ICV, GRE/Ethernet/VXLAN/encap-limit,
	// or UDP/WireGuard header+tag).
	fixed uint16
	// trailer is the fixed trailer inside the padded region: 2 for ESP, 0 for
	// non-IPsec encapsulations.
	trailer uint16
	// block is the alignment the plaintext, trailer included, is padded to:
	// 4, 8 or 16 for ESP; 1 for non-IPsec encapsulations.
	block uint16
}

// octets is the tunnel-mode fixed expansion before padding and trailer:
// AES-GCM-128 over IPv4 without UDP encapsulation is 20 + 8 + 8 + 16 = 52
// (AC-2), and 60 encapsulated (AC-3).
func (o encapOverhead) octets() uint16 {
	return o.outerHeader + o.fixed
}

// outerIPHeaderOctets answers the outer IP header length for addr.
func outerIPHeaderOctets(addr netip.Addr) uint16 {
	if addr.Unmap().Is6() {
		return ipv6HeaderOctets
	}
	return ipv4HeaderOctets
}

// deriveESPOverhead derives the overhead of one installed Child SA from the
// transform it negotiated, its mode and its encapsulation. It refuses, rather
// than defaults, a tunnel whose child is not installed, whose mode is not one
// ESP defines, whose transforms the IKE engine could not resolve, or whose
// cipher registry declares no installable ESP framing.
func deriveESPOverhead(t *ipsecinventory.Tunnel) (encapOverhead, error) {
	if !t.Up {
		return encapOverhead{}, fmt.Errorf("%w: peer %s has no child SA installed", errOverheadRefused, t.Peer)
	}
	// The transform fields are unset beside TransformErr, so reading them would size
	// the packet for ENCR 0.
	if t.TransformErr != nil {
		return encapOverhead{}, fmt.Errorf("%w: peer %s has unknown transforms: %w", errOverheadRefused, t.Peer, t.TransformErr)
	}
	if !t.InstalledRemote.IsValid() {
		return encapOverhead{}, fmt.Errorf("%w: peer %s has no installed endpoint", errOverheadRefused, t.Peer)
	}
	var o encapOverhead
	switch t.Mode {
	case ipsecinventory.ModeTunnel, ipsecinventory.ModeTransport:
		o.mode = t.Mode
	case ipsecinventory.ModeUnspecified:
		return encapOverhead{}, fmt.Errorf("%w: peer %s has an unspecified mode", errOverheadRefused, t.Peer)
	default:
		// The inventory mode set is open; refuse values this arithmetic does not know.
		return encapOverhead{}, fmt.Errorf("%w: peer %s has an unspecified mode", errOverheadRefused, t.Peer)
	}
	o.outerHeader = outerIPHeaderOctets(t.InstalledRemote)
	cipherID := crypto.EncryptionID(t.Encryption)
	// RFC 7634 Section 2, RFC 2451 Section 2.4 and RFC 4303 Section 2.4.
	ivOctets, blockOctets, err := cipherID.ESPWireSizes()
	if err != nil {
		return encapOverhead{}, fmt.Errorf("%w: peer %s negotiated ENCR transform %d (%s), which this arithmetic does not know", errOverheadRefused, t.Peer, cipherID, cipherID)
	}
	icv, err := espICVOctets(cipherID, crypto.IntegrityID(t.Integrity))
	if err != nil {
		return encapOverhead{}, fmt.Errorf("%w: peer %s: %w", errOverheadRefused, t.Peer, err)
	}
	o.trailer = espTrailerOctets
	o.block = blockOctets
	o.fixed = espHeaderOctets + ivOctets + icv
	if t.UDPEncap {
		o.fixed += udpHeaderOctets
	}
	return o, nil
}

// deriveTunnelOverhead derives the exact wire encapsulation overhead for one
// non-IPsec tunnel from its live dataplane state. It refuses nil info,
// unmodeled FOU/GUE outer encapsulations (OuterEncapType != 0), unknown tunnel
// kinds, and VXLAN tunnels whose remote address is invalid.
func deriveTunnelOverhead(info *iface.TunnelInfo) (encapOverhead, error) {
	if info == nil {
		return encapOverhead{}, fmt.Errorf("%w: tunnel info is absent", errOverheadRefused)
	}
	if info.OuterEncapType != 0 {
		return encapOverhead{}, fmt.Errorf("%w: outer UDP encapsulation type %d (FOU/GUE) is not modeled", errOverheadRefused, info.OuterEncapType)
	}
	var outer, fixed uint16
	switch info.Kind {
	case iface.TunnelKindGRE:
		outer = ipv4HeaderOctets
		fixed = greHeaderOctets(info)
	case iface.TunnelKindGRETap:
		outer = ipv4HeaderOctets
		fixed = greHeaderOctets(info) + ethernetHeaderOctets
	case iface.TunnelKindIP6GRE:
		outer = ipv6HeaderOctets
		fixed = greHeaderOctets(info) + ipv6EncapLimit(info)
	case iface.TunnelKindIP6GRETap:
		outer = ipv6HeaderOctets
		fixed = greHeaderOctets(info) + ethernetHeaderOctets + ipv6EncapLimit(info)
	case iface.TunnelKindVxlan:
		if !info.Remote.IsValid() {
			return encapOverhead{}, fmt.Errorf("%w: vxlan tunnel has no valid remote endpoint", errOverheadRefused)
		}
		outer = outerIPHeaderOctets(info.Remote)
		fixed = udpHeaderOctets + vxlanHeaderOctets + ethernetHeaderOctets
	case iface.TunnelKindIPIP, iface.TunnelKindSIT:
		outer = ipv4HeaderOctets
		fixed = 0
	case iface.TunnelKindIP6Tnl, iface.TunnelKindIPIP6:
		outer = ipv6HeaderOctets
		fixed = ipv6EncapLimit(info)
	case iface.TunnelKindUnknown:
		return encapOverhead{}, fmt.Errorf("%w: tunnel kind is unknown", errOverheadRefused)
	default:
		// The TunnelKind set is open; refuse kinds this arithmetic does not know.
		return encapOverhead{}, fmt.Errorf("%w: tunnel kind %d is not supported", errOverheadRefused, info.Kind)
	}
	if err := checkTunnelRemoteFamily(info.Kind, info.Remote); err != nil {
		return encapOverhead{}, err
	}
	return encapOverhead{
		mode:        ipsecinventory.ModeTunnel,
		outerHeader: outer,
		fixed:       fixed,
		trailer:     0,
		block:       1,
	}, nil
}

// checkTunnelRemoteFamily refuses a non-VXLAN tunnel whose remote endpoint
// family disagrees with its declared outer underlay family (exact-or-reject).
func checkTunnelRemoteFamily(kind iface.TunnelKind, remote netip.Addr) error {
	if kind == iface.TunnelKindVxlan {
		return nil
	}
	if !remote.IsValid() {
		return nil
	}
	if kind.IsV6Underlay() != remote.Unmap().Is6() {
		return fmt.Errorf("%w: tunnel kind %s does not match remote endpoint %s", errOverheadRefused, kind, remote.Unmap())
	}
	return nil
}

// greHeaderOctets computes the GRE header length (4 base + 4 per enabled
// checksum, key, or sequence extension).
func greHeaderOctets(info *iface.TunnelInfo) uint16 {
	octets := uint16(greBaseHeaderOctets)
	if info.GREChecksum {
		octets += greOptionOctets
	}
	if info.GREKey {
		octets += greOptionOctets
	}
	if info.GRESequence {
		octets += greOptionOctets
	}
	return octets
}

// ipv6EncapLimit returns 8 when the tunnel emits the RFC 2473 Section 4.1.1
// Tunnel Encapsulation Limit Destination Options header, else 0.
func ipv6EncapLimit(info *iface.TunnelInfo) uint16 {
	if info.IPv6EncapLimit {
		return ipv6EncapLimitOctets
	}
	return 0
}

// deriveWireguardOverhead derives the exact WireGuard overhead at full MTU for
// one peer endpoint: 60 octets over IPv4 (20 IPv4 + 8 UDP + 32 WG) and 80
// octets over IPv6 (40 IPv6 + 8 UDP + 32 WG).
func deriveWireguardOverhead(endpoint netip.Addr) (encapOverhead, error) {
	if !endpoint.IsValid() {
		return encapOverhead{}, fmt.Errorf("%w: wireguard peer has no valid endpoint address", errOverheadRefused)
	}
	return encapOverhead{
		mode:        ipsecinventory.ModeTunnel,
		outerHeader: outerIPHeaderOctets(endpoint),
		fixed:       udpHeaderOctets + wireguardDataOverheadOctets,
		trailer:     0,
		block:       1,
	}, nil
}

// espICVOctets answers the ICV one Child SA appends. An AEAD cipher carries
// its own, and the crypto package declares it per transform id; a CBC cipher
// carries its integrity transform's truncated MAC, which the same package
// declares per integrity transform. AUTH_NONE beside a non-AEAD cipher (CBC or
// the null cipher) is refused: RFC 4303 Section 2.8 makes integrity optional
// for ESP, but no live Child SA carries that pair. crypto.espProposalComplete
// refuses ENCR_NULL beside AUTH_NONE (RFC 4303 Section 3.2), and a CBC cipher
// without a hash never matches, because crypto.matchESP requires the
// peer's integrity id to equal local policy's and an esp-group proposal with
// a non-AEAD cipher and no hash is refused at commit (ipsec.parseESPProposal).
func espICVOctets(cipherID crypto.EncryptionID, integrityID crypto.IntegrityID) (uint16, error) {
	if cipherID.IsAEAD() {
		icv, err := crypto.AEADICVOctets(cipherID)
		if err != nil {
			return 0, err
		}
		return uint16(icv), nil //nolint:gosec // an ICV is at most 32 octets
	}
	if integrityID == crypto.AUTH_NONE {
		return 0, fmt.Errorf("%s carries no integrity transform", cipherID)
	}
	// RFC 2404 Sections 2 and 3; ESP integrity includes SHA-1 without an IKE PRF.
	transform, err := crypto.LookupESPIntegrity(integrityID.String())
	if err != nil {
		return 0, fmt.Errorf("INTEG transform %d (%s) is not one this build implements", integrityID, integrityID)
	}
	return transform.TruncatedLength, nil
}

// Design: docs/architecture/testing/interop.md -- raw VPNv6 family admission wire oracle.
// Related: check_raw_vpnv6_admission.go -- drives operator raw commands and native FRR checks.
// FRR: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_packet.c (bgp_update_receive, bgp_nlri_parse)
// FRR: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_attr.c (bgp_mp_reach_parse)
// FRR: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_mplsvpn.c (bgp_nlri_parse_vpn)
// FRR: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_route.c (bgp_show_table_rd, route_vty_out_detail)
package bgp

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

const (
	rawVPNv6AdmissionScenario = "bgp-raw-vpnv6-family-admission-frr"
	rawVPNv6NegotiatedCapture = "/tmp/raw-vpnv6-negotiated"
	rawVPNv6V4OnlyCapture     = "/tmp/raw-vpnv6-v4only"
	rawVPNv6RD                = "65001:100"
	rawVPNv6NextHop           = "2001:db8:ffff::1"
	rawVPNv6ASPath            = "65001"
	rawVPNv6BodyPrefix        = "2001:db8:4659::/48"
	rawVPNv6FullPrefix        = "2001:db8:465a::/48"
	rawVPNv6MixedPrefix       = "2001:db8:465b::/48"
	rawVPNv6NegotiatedFence   = "10.46.1.0/24"
	rawVPNv6MixedIPv4Prefix   = "10.46.2.0/24"
	rawVPNv6V4OnlyFence       = "10.46.255.0/24"

	rawVPNv6BodyLabel  uint32 = 1000
	rawVPNv6FullLabel  uint32 = 1001
	rawVPNv6MixedLabel uint32 = 1002
	rawVPNv6LocalASN   uint16 = 65001
	rawVPNv6RemoteASN  uint16 = 65002
	rawVPNv6LocalASNHi byte   = byte(rawVPNv6LocalASN >> 8)
	rawVPNv6LocalASNLo byte   = byte(rawVPNv6LocalASN & 0xff)

	rawVPNv6CaptureBytesMax  = 2 * 1024 * 1024
	rawVPNv6CaptureFramesMax = 256
)

var (
	rawVPNv6BodyPrefixBytes  = [6]byte{0x20, 0x01, 0x0d, 0xb8, 0x46, 0x59}
	rawVPNv6FullPrefixBytes  = [6]byte{0x20, 0x01, 0x0d, 0xb8, 0x46, 0x5a}
	rawVPNv6MixedPrefixBytes = [6]byte{0x20, 0x01, 0x0d, 0xb8, 0x46, 0x5b}
	rawVPNv6IPv4EORBody      = []byte{0, 0, 0, 0}
	rawVPNv6ExtEORBody       = []byte{0, 0, 0, 7, 0x90, 0x0f, 0, 3, 0, 2, 0x80}
	rawVPNv6ShortEORBody     = []byte{0, 0, 0, 6, 0x80, 0x0f, 3, 0, 2, 0x80}
)

// rawVPNv6CapabilityEntry decodes one address family entry under FRR's
// neighborCapabilities.multiprotocolExtensions object.
type rawVPNv6CapabilityEntry struct {
	Advertised            bool `json:"advertised"`
	Received              bool `json:"received"`
	AdvertisedAndReceived bool `json:"advertisedAndReceived"`
}

// rawVPNv6NeighborDoc decodes the capability subset of FRR's
// `show bgp neighbor <addr> json`.
type rawVPNv6NeighborDoc struct {
	NeighborCapabilities struct {
		MultiprotocolExtensions struct {
			IPv4Unicast *rawVPNv6CapabilityEntry `json:"ipv4Unicast"`
			IPv6VPN     *rawVPNv6CapabilityEntry `json:"ipv6Vpn"`
		} `json:"multiprotocolExtensions"`
	} `json:"neighborCapabilities"`
}

// rawVPNv6MPReachAttr builds a canonical MP_REACH_NLRI attribute carrying one
// labeled IPv6 VPN /48 route with a 24-octet third-party global next hop.
// RFC 4659 Section 3.2: AFI 2, SAFI 128, NH len 24 (RD 0 + global IPv6),
// reserved 0, NLRI [bitlen=136 | label(3, S=1) | RD(8) | prefix(6)].
//
// Wire layout (50 octets total):
//
//	[0:3]   Attr header: flags=0x80 (optional), code=14 (MP_REACH_NLRI), len=47 (0x2f)
//	[3:6]   AFI=2 (0x0002), SAFI=128 (0x80)
//	[6]     Next Hop length=24 (0x18)
//	[7:15]  Next Hop RD=0 (8 zero octets)
//	[15:31] Next Hop IPv6=2001:db8:ffff::1 (16 octets)
//	[31]    Reserved=0x00
//	[32]    NLRI bit length=136 (0x88: 24 label + 64 RD + 48 prefix)
//	[33:36] MPLS label=(label<<4)|1 (S bit set)
//	[36:44] RD=65001:100 (type 0, ASN 65001, value 100)
//	[44:50] IPv6 /48 prefix bytes
func rawVPNv6MPReachAttr(prefixTail [6]byte, label uint32) []byte {
	nh := netip.MustParseAddr(rawVPNv6NextHop).As16()
	encodedLabel := (label << 4) | 1
	attr := make([]byte, 0, 50)
	attr = append(attr,
		0x80, 0x0e, 0x2f, 0x00, 0x02, 0x80, 0x18,
		0, 0, 0, 0, 0, 0, 0, 0,
	)
	attr = append(attr, nh[:]...)
	attr = append(attr,
		0x00, 0x88,
		byte(encodedLabel>>16), byte(encodedLabel>>8), byte(encodedLabel),
		0x00, 0x00, rawVPNv6LocalASNHi, rawVPNv6LocalASNLo, 0x00, 0x00, 0x00, 100,
	)
	attr = append(attr, prefixTail[:]...)
	return attr
}

// rawVPNv6UpdateBody builds a valid pure VPNv6 UPDATE body with 4-octet ASN4
// AS_PATH [65001] matching Ze's local AS so FRR's enforce-first-as accepts it.
// RFC 4659 Section 3.4: "In order for two PEs to exchange labeled IPv6 VPN
// NLRIs, they MUST use BGP Capabilities Negotiation to ensure that they both
// are capable of properly processing such NLRIs."
//
// Wire layout:
//
//	[0:2]  Withdrawn Routes Length=0
//	[2:4]  Total Path Attribute Length=63
//	[4:8]  ORIGIN=IGP (0x40 0x01 0x01 0x00)
//	[8:17] AS_PATH=[65001] 4-octet (0x40 0x02 0x06 0x02 0x01 0x0000fde9)
//	[17:67] MP_REACH_NLRI (50 octets)
func rawVPNv6UpdateBody(prefixTail [6]byte, label uint32) []byte {
	attrs := make([]byte, 0, 63)
	attrs = append(attrs,
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, rawVPNv6LocalASNHi, rawVPNv6LocalASNLo,
	)
	attrs = append(attrs, rawVPNv6MPReachAttr(prefixTail, label)...)
	body := make([]byte, 4+len(attrs))
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs)))
	copy(body[4:], attrs)
	return body
}

// rawVPNv6IPv4Body builds a valid 4-octet ASN4 IPv4 unicast UPDATE body for
// 10.46.<ordinal>.0/24 with nextHop.
//
// Wire layout:
//
//	[0:2]   Withdrawn Routes Length=0
//	[2:4]   Total Path Attribute Length=20
//	[4:8]   ORIGIN=IGP
//	[8:17]  AS_PATH=[65001] 4-octet
//	[17:24] NEXT_HOP=nextHop (4 octets)
//	[24:28] NLRI=[24, 10, 46, ordinal]
func rawVPNv6IPv4Body(ordinal byte, nextHop netip.Addr) []byte {
	hop := nextHop.As4()
	attrs := make([]byte, 0, 20)
	attrs = append(attrs,
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, rawVPNv6LocalASNHi, rawVPNv6LocalASNLo,
		0x40, 0x03, 0x04,
	)
	attrs = append(attrs, hop[:]...)
	body := make([]byte, 4+len(attrs)+4)
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs)))
	copy(body[4:], attrs)
	copy(body[4+len(attrs):], []byte{24, 10, 46, ordinal})
	return body
}

// rawVPNv6MixedBody builds a valid raw UPDATE body carrying both an IPv4
// unicast NLRI (10.46.<ipv4Ordinal>.0/24) and a VPNv6 MP_REACH_NLRI. On an
// IPv4-only session Ze MUST strip the unnegotiated VPNv6 MP_REACH_NLRI and emit
// bytes equal to rawVPNv6IPv4Body(ipv4Ordinal, nextHop).
// RFC 4760 Section 8: "To have a bi-directional exchange of routing
// information for a particular <AFI, SAFI> between a pair of BGP speakers,
// each such speaker MUST advertise to the other (via the Capability
// Advertisement mechanism) the capability to support that particular
// <AFI, SAFI> route."
func rawVPNv6MixedBody(prefixTail [6]byte, label uint32, ipv4Ordinal byte, nextHop netip.Addr) []byte {
	hop := nextHop.As4()
	attrs := make([]byte, 0, 70)
	attrs = append(attrs,
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, rawVPNv6LocalASNHi, rawVPNv6LocalASNLo,
		0x40, 0x03, 0x04,
	)
	attrs = append(attrs, hop[:]...)
	attrs = append(attrs, rawVPNv6MPReachAttr(prefixTail, label)...)
	body := make([]byte, 4+len(attrs)+4)
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs)))
	copy(body[4:], attrs)
	copy(body[4+len(attrs):], []byte{24, 10, 46, ipv4Ordinal})
	return body
}

// rawVPNv6FullPacket wraps an UPDATE body in a 19-octet BGP message header for
// the full-packet raw CLI form (`send bgp <peer> raw hex <packet>`).
//
// Wire layout (RFC 4271 Section 4.1):
//
//	[0:16]  Marker (16 octets of 0xff)
//	[16:18] Total message length (19 + len(body))
//	[18]    Message type = 2 (UPDATE)
//	[19:]   UPDATE body
func rawVPNv6FullPacket(body []byte) []byte {
	packet := make([]byte, bgpHeaderLength+len(body))
	for index := range packet[:16] {
		packet[index] = 0xff
	}
	binary.BigEndian.PutUint16(packet[16:18], uint16(len(packet)))
	packet[18] = bgpUpdate
	copy(packet[bgpHeaderLength:], body)
	return packet
}

// rawVPNv6ExpectedUpdates returns the exact Ze-to-FRR UPDATE bodies expected
// after initial End-of-RIB markers on each session.
func rawVPNv6ExpectedUpdates(ze netip.Addr, negotiatedVPNv6 bool) [][]byte {
	if negotiatedVPNv6 {
		return [][]byte{
			rawVPNv6UpdateBody(rawVPNv6BodyPrefixBytes, rawVPNv6BodyLabel),
			rawVPNv6UpdateBody(rawVPNv6FullPrefixBytes, rawVPNv6FullLabel),
			rawVPNv6IPv4Body(1, ze),
		}
	}
	return [][]byte{
		rawVPNv6IPv4Body(2, ze),
		rawVPNv6IPv4Body(255, ze),
	}
}

func isRawVPNv6EORBody(body []byte) bool {
	if bytes.Equal(body, rawVPNv6ExtEORBody) {
		return true
	}
	return bytes.Equal(body, rawVPNv6ShortEORBody)
}

// judgeRawVPNv6InitialSync checks whether Ze has finished its initial handshake
// and End-of-RIB markers on the transparent relay capture before any operator
// raw commands are issued.
func judgeRawVPNv6InitialSync(text string, ze, native netip.Addr, negotiatedVPNv6 bool) (bool, error) {
	if len(text) > rawVPNv6CaptureBytesMax {
		return false, errors.New("raw VPNv6 capture exceeds its bounded fixture size")
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	opened, keepalives, seenIPv4EOR, seenVPNv6EOR := false, 0, false, false
	for ordinal := range rawVPNv6CaptureFramesMax {
		var row extendedRelayFrame
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(err, io.EOF) {
				if !opened {
					return false, nil
				}
				if keepalives == 0 {
					return false, nil
				}
				if !seenIPv4EOR {
					return false, nil
				}
				if negotiatedVPNv6 && !seenVPNv6EOR {
					return false, nil
				}
				return true, nil
			}
			return false, err
		}
		if ordinal == rawVPNv6CaptureFramesMax-1 {
			return false, errors.New("raw VPNv6 initial capture exhausted its frame bound")
		}
		frame, err := decodeRawVPNv6RelayRow(row)
		if err != nil {
			return false, err
		}
		if frame[18] == bgpOpen {
			if opened {
				return false, errors.New("raw VPNv6 initial capture changed session generation")
			}
			if err := requireRawVPNv6Open(frame, ze, native, negotiatedVPNv6, false); err != nil {
				return false, err
			}
			opened = true
			continue
		}
		if !opened {
			return false, errors.New("raw VPNv6 initial capture lacks its original OPEN")
		}
		switch frame[18] {
		case bgpKeepalive:
			keepalives++
		case bgpUpdate:
			if keepalives == 0 {
				return false, errors.New("raw VPNv6 initial UPDATE precedes handshake witness")
			}
			body := frame[bgpHeaderLength:]
			if bytes.Equal(body, rawVPNv6IPv4EORBody) {
				seenIPv4EOR = true
				continue
			}
			if negotiatedVPNv6 && isRawVPNv6EORBody(body) {
				seenVPNv6EOR = true
				continue
			}
			return false, fmt.Errorf("unexpected initial UPDATE before raw commands: %x", body)
		default:
			// Open wire type; initial sync admits no NOTIFICATION or other type.
			return false, fmt.Errorf("unexpected raw VPNv6 initial capture type %d", frame[18])
		}
	}
	return false, errors.New("raw VPNv6 initial capture has no bounded completion")
}

// judgeRawVPNv6Wire judges a declared capture interval ending at the session's
// final IPv4 fence UPDATE. FRR 10.7.1 bgp_update_receive silently skips an
// NLRI_MP_UPDATE for an unconfigured family (!peer->afc[afi][safi]), so native
// absence alone cannot detect a leaked VPNv6 frame; this transparent wire oracle
// rejects any leaked VPNv6 UPDATE or unstripped mixed UPDATE on the IPv4-only
// session and verifies exact byte delivery on the negotiated session.
func judgeRawVPNv6Wire(text string, ze, native netip.Addr, negotiatedVPNv6, fromNative bool) (bool, error) {
	if len(text) > rawVPNv6CaptureBytesMax {
		return false, errors.New("raw VPNv6 capture exceeds its bounded fixture size")
	}
	expected := rawVPNv6ExpectedUpdates(ze, negotiatedVPNv6)
	decoder := json.NewDecoder(strings.NewReader(text))
	opened, keepalives, updates := false, 0, 0
	for ordinal := range rawVPNv6CaptureFramesMax {
		var row extendedRelayFrame
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(err, io.EOF) {
				if fromNative {
					return opened && keepalives > 0, nil
				}
				return false, nil
			}
			return false, err
		}
		if ordinal == rawVPNv6CaptureFramesMax-1 {
			return false, errors.New("raw VPNv6 capture exhausted its frame bound")
		}
		frame, err := decodeRawVPNv6RelayRow(row)
		if err != nil {
			return false, err
		}
		if frame[18] == bgpOpen {
			if opened {
				return false, errors.New("raw VPNv6 capture changed session generation")
			}
			if err := requireRawVPNv6Open(frame, ze, native, negotiatedVPNv6, fromNative); err != nil {
				return false, err
			}
			opened = true
			continue
		}
		if !opened {
			return false, errors.New("raw VPNv6 capture lacks its original OPEN")
		}
		switch frame[18] {
		case bgpKeepalive:
			keepalives++
		case bgpUpdate:
			if keepalives == 0 {
				return false, errors.New("raw VPNv6 UPDATE precedes handshake witness")
			}
			body := frame[bgpHeaderLength:]
			if fromNative {
				if _, err := parsedEmptyMPUpdate(body); err != nil {
					return false, fmt.Errorf("malformed native raw VPNv6 UPDATE: %w", err)
				}
				continue
			}
			if bytes.Equal(body, rawVPNv6IPv4EORBody) {
				if updates != 0 {
					return false, errors.New("unexpected IPv4 EOR inside raw VPNv6 command sequence")
				}
				continue
			}
			if negotiatedVPNv6 && isRawVPNv6EORBody(body) {
				if updates != 0 {
					return false, errors.New("unexpected VPNv6 EOR inside raw VPNv6 command sequence")
				}
				continue
			}
			if !bytes.Equal(body, expected[updates]) {
				return false, fmt.Errorf("raw VPNv6 wire step %d differs: got %x, want %x", updates+1, body, expected[updates])
			}
			updates++
			if updates == len(expected) {
				return true, nil
			}
		default:
			// Open wire type; this scenario requests no NOTIFICATION or other traffic.
			return false, fmt.Errorf("unexpected raw VPNv6 capture type %d", frame[18])
		}
	}
	return false, errors.New("raw VPNv6 capture has no complete bounded interval")
}

func decodeRawVPNv6RelayRow(row extendedRelayFrame) ([]byte, error) {
	if row.Delivered != "" {
		return nil, errors.New("raw VPNv6 transparent relay changed a frame")
	}
	if row.Complete != nil {
		return nil, errors.New("unexpected relay completion marker")
	}
	frame, err := hex.DecodeString(row.Original)
	if err != nil {
		return nil, err
	}
	if len(frame) < bgpHeaderLength {
		return nil, errors.New("short raw VPNv6 capture frame")
	}
	size, err := validateSpeakerMessageHeader(frame[:bgpHeaderLength])
	if err != nil {
		return nil, err
	}
	if size != len(frame) {
		return nil, errors.New("raw VPNv6 capture length differs from its header")
	}
	return frame, nil
}

// requireRawVPNv6Open validates speaker identity, ASN4 capability, and the
// exact Multiprotocol capability boundary (RFC 4760 Section 8, RFC 4659
// Section 3.4) on both sides of each transparent relay.
func requireRawVPNv6Open(frame []byte, ze, native netip.Addr, negotiatedVPNv6, fromNative bool) error {
	if err := validateSpeakerOpen(frame[bgpHeaderLength:]); err != nil {
		return err
	}
	asn, address := rawVPNv6LocalASN, ze
	if fromNative {
		asn, address = rawVPNv6RemoteASN, native
	}
	router := address.As4()
	if binary.BigEndian.Uint16(frame[20:22]) != asn {
		return errors.New("raw VPNv6 capture names another speaker AS")
	}
	if !bytes.Equal(frame[24:28], router[:]) {
		return errors.New("raw VPNv6 capture names another speaker identifier")
	}
	caps, err := capability.ParseFromOptionalParams(frame[29:], false)
	if err != nil {
		return err
	}
	asn4, mpV4, mpVPNv6 := 0, 0, 0
	for _, entry := range caps {
		switch advertised := entry.(type) {
		case *capability.ASN4:
			if advertised.ASN != uint32(asn) {
				return errors.New("inconsistent raw VPNv6 session ASN4 capability")
			}
			asn4++
		case *capability.Multiprotocol:
			if advertised.AFI == family.AFIIPv4 && advertised.SAFI == family.SAFIUnicast {
				mpV4++
			}
			if advertised.AFI == family.AFIIPv6 && advertised.SAFI == family.SAFIVPN {
				mpVPNv6++
			}
		case *capability.AddPath:
			if !fromNative {
				return errors.New("the Ze speaker offered ADD-PATH in the unframed raw VPNv6 fixture")
			}
		}
	}
	if asn4 != 1 {
		return errors.New("raw VPNv6 capture lacks its negotiated ASN4 capability")
	}
	if mpV4 != 1 {
		return errors.New("raw VPNv6 capture lacks its ipv4/unicast Multiprotocol capability")
	}
	wantVPNv6 := 0
	if negotiatedVPNv6 {
		wantVPNv6 = 1
	}
	if mpVPNv6 != wantVPNv6 {
		return fmt.Errorf("raw VPNv6 capture advertised %d ipv6/mpls-vpn capabilities, want %d", mpVPNv6, wantVPNv6)
	}
	return nil
}

// requireRawVPNv6NeighborCapabilities checks FRR's `show bgp neighbor <addr>
// json` Multiprotocol capabilities against the session's expected family set.
func requireRawVPNv6NeighborCapabilities(neighborJSON, neighbor string, wantVPNv6 bool) error {
	var doc map[string]rawVPNv6NeighborDoc
	if err := json.Unmarshal([]byte(neighborJSON), &doc); err != nil {
		return fmt.Errorf("decode FRR neighbor capabilities: %w", err)
	}
	peer, ok := doc[neighbor]
	if !ok {
		return fmt.Errorf("FRR neighbor JSON has no entry for %s", neighbor)
	}
	mp := peer.NeighborCapabilities.MultiprotocolExtensions
	if mp.IPv4Unicast == nil {
		return fmt.Errorf("FRR neighbor %s reports no ipv4Unicast capability", neighbor)
	}
	if !mp.IPv4Unicast.AdvertisedAndReceived {
		return fmt.Errorf("FRR neighbor %s did not negotiate ipv4Unicast", neighbor)
	}
	if wantVPNv6 {
		if mp.IPv6VPN == nil {
			return fmt.Errorf("FRR neighbor %s reports no ipv6Vpn capability", neighbor)
		}
		if !mp.IPv6VPN.AdvertisedAndReceived {
			return fmt.Errorf("FRR neighbor %s did not negotiate ipv6Vpn", neighbor)
		}
		return nil
	}
	if mp.IPv6VPN != nil {
		if mp.IPv6VPN.Advertised {
			return fmt.Errorf("FRR IPv4-only neighbor %s unexpectedly advertised ipv6Vpn capability", neighbor)
		}
		if mp.IPv6VPN.Received {
			return fmt.Errorf("FRR IPv4-only neighbor %s unexpectedly received ipv6Vpn capability", neighbor)
		}
		if mp.IPv6VPN.AdvertisedAndReceived {
			return fmt.Errorf("FRR IPv4-only neighbor %s unexpectedly negotiated ipv6Vpn capability", neighbor)
		}
	}
	return nil
}

// requireRawVPNv6FRRRoute validates FRR 10.7.1 `show bgp ipv6 vpn rd <rd>
// <prefix> json` (route_vty_out_detail under the RD key): exact RD, prefix,
// single path from neighbor, AS_PATH 65001, remote MPLS label, and global IPv6
// next hop 2001:db8:ffff::1.
func requireRawVPNv6FRRRoute(output, rd, prefix, neighbor string, wantLabel uint32, wantNextHop netip.Addr) error {
	var byRD map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &byRD); err != nil {
		return fmt.Errorf("decode FRR VPNv6 detail JSON: %w", err)
	}
	rawDoc, ok := byRD[rd]
	if !ok {
		return fmt.Errorf("FRR VPNv6 detail JSON has no %s key", rd)
	}
	var route struct {
		Prefix string `json:"prefix"`
		Paths  []struct {
			Peer struct {
				ID string `json:"peerId"`
			} `json:"peer"`
			ASPath struct {
				Value string `json:"string"`
			} `json:"aspath"`
			RemoteLabel *uint32   `json:"remoteLabel"`
			NextHops    []nextHop `json:"nexthops"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(rawDoc, &route); err != nil {
		return fmt.Errorf("decode FRR VPNv6 route %s: %w", prefix, err)
	}
	if route.Prefix != prefix {
		return fmt.Errorf("FRR VPNv6 detail prefix is %q, want %q", route.Prefix, prefix)
	}
	if len(route.Paths) != 1 {
		return fmt.Errorf("FRR VPNv6 route %s has %d paths, want 1", prefix, len(route.Paths))
	}
	path := route.Paths[0]
	if path.Peer.ID != neighbor {
		return fmt.Errorf("FRR VPNv6 route %s came from %q, want %q", prefix, path.Peer.ID, neighbor)
	}
	if strings.TrimSpace(path.ASPath.Value) != rawVPNv6ASPath {
		return fmt.Errorf("FRR VPNv6 route %s has AS_PATH %q, want %q", prefix, path.ASPath.Value, rawVPNv6ASPath)
	}
	if path.RemoteLabel == nil {
		return fmt.Errorf("FRR VPNv6 route %s omitted remoteLabel", prefix)
	}
	if *path.RemoteLabel != wantLabel {
		return fmt.Errorf("FRR VPNv6 route %s has remoteLabel %d, want %d", prefix, *path.RemoteLabel, wantLabel)
	}
	return requireSoleNextHop(path.NextHops, wantNextHop)
}

// requireRawVPNv6FRRTable validates FRR 10.7.1 `show bgp ipv6 vpn rd all json`
// (bgp_show_table_rd). When wantPrefixes is empty, FRR MUST print an empty JSON
// object `{}` (no VPNv6 routes). Otherwise totalRoutes, totalPaths, and every
// expected prefix under rd MUST match exactly.
func requireRawVPNv6FRRTable(output, rd, neighbor string, wantPrefixes []string) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		return fmt.Errorf("decode FRR VPNv6 table JSON: %w", err)
	}
	if envelope == nil {
		return errors.New("FRR VPNv6 table JSON is null")
	}
	if len(wantPrefixes) == 0 {
		if len(envelope) != 0 {
			return fmt.Errorf("FRR IPv4-only peer VPNv6 table is non-empty: %s", strings.TrimSpace(output))
		}
		return nil
	}
	var table struct {
		TotalRoutes *int `json:"totalRoutes"`
		TotalPaths  *int `json:"totalPaths"`
		Routes      struct {
			RDs map[string]map[string][]struct {
				Peer string `json:"peerId"`
				Path string `json:"path"`
			} `json:"routeDistinguishers"`
		} `json:"routes"`
	}
	if err := json.Unmarshal([]byte(output), &table); err != nil {
		return fmt.Errorf("decode FRR VPNv6 RD table: %w", err)
	}
	if table.TotalRoutes == nil {
		return errors.New("FRR VPNv6 table has no totalRoutes")
	}
	if table.TotalPaths == nil {
		return errors.New("FRR VPNv6 table has no totalPaths")
	}
	if *table.TotalRoutes != len(wantPrefixes) {
		return fmt.Errorf("FRR VPNv6 table holds %d routes, want %d", *table.TotalRoutes, len(wantPrefixes))
	}
	if *table.TotalPaths != len(wantPrefixes) {
		return fmt.Errorf("FRR VPNv6 table holds %d paths, want %d", *table.TotalPaths, len(wantPrefixes))
	}
	rdTable := table.Routes.RDs[rd]
	if len(rdTable) != len(wantPrefixes) {
		return fmt.Errorf("FRR VPNv6 RD %s holds %d prefixes, want %d", rd, len(rdTable), len(wantPrefixes))
	}
	for _, prefix := range wantPrefixes {
		paths := rdTable[prefix]
		if len(paths) != 1 {
			return fmt.Errorf("FRR VPNv6 RD %s prefix %s holds %d paths, want 1", rd, prefix, len(paths))
		}
		if paths[0].Peer != neighbor {
			return fmt.Errorf("FRR VPNv6 RD %s prefix %s came from %q, want %q", rd, prefix, paths[0].Peer, neighbor)
		}
		if strings.TrimSpace(paths[0].Path) != rawVPNv6ASPath {
			return fmt.Errorf("FRR VPNv6 RD %s prefix %s has AS_PATH %q, want %q", rd, prefix, paths[0].Path, rawVPNv6ASPath)
		}
	}
	return nil
}

// requireRawVPNv6IPv4Route validates FRR 10.7.1 `show bgp ipv4 unicast <prefix>
// json` (route_vty_out_detail): exact prefix, valid path from neighbor, AS_PATH
// 65001, and sole next hop matching ze.
func requireRawVPNv6IPv4Route(output, prefix, neighbor string, wantNextHop netip.Addr) error {
	var route struct {
		Prefix string `json:"prefix"`
		Paths  []struct {
			Valid bool `json:"valid"`
			Peer  struct {
				ID string `json:"peerId"`
			} `json:"peer"`
			ASPath struct {
				Value string `json:"string"`
			} `json:"aspath"`
			NextHops []nextHop `json:"nexthops"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(output), &route); err != nil {
		return fmt.Errorf("decode FRR IPv4 route %s JSON: %w", prefix, err)
	}
	if route.Prefix != prefix {
		return fmt.Errorf("FRR IPv4 detail prefix is %q, want %q", route.Prefix, prefix)
	}
	if len(route.Paths) != 1 {
		return fmt.Errorf("FRR IPv4 route %s has %d paths, want 1", prefix, len(route.Paths))
	}
	path := route.Paths[0]
	if !path.Valid {
		return fmt.Errorf("FRR rejected IPv4 control route %s", prefix)
	}
	if path.Peer.ID != neighbor {
		return fmt.Errorf("FRR IPv4 route %s came from %q, want %q", prefix, path.Peer.ID, neighbor)
	}
	if strings.TrimSpace(path.ASPath.Value) != rawVPNv6ASPath {
		return fmt.Errorf("FRR IPv4 route %s has AS_PATH %q, want %q", prefix, path.ASPath.Value, rawVPNv6ASPath)
	}
	return requireSoleNextHop(path.NextHops, wantNextHop)
}

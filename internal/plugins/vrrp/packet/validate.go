// RFC: rfc/short/rfc9568.md -- Section 7.1 receive validation + errata 8298-8301
// RFC: rfc/short/rfc3768.md -- Section 7.1 receive validation + Section 5.2.3 TTL
// Design: ai/rules/performance.md -- zero-allocation decode over a lazy VIP view
//
// validate.go implements the ordered VRRP receive-validation ladder (Decode),
// the RxMeta the transport produces, the caller-supplied group Lookup, the
// typed error taxonomy, the Reason() mapping to metrics labels, and the
// IHL-aware IPv4 header strip. The ladder order is fixed and test-enforced
// (TestValidationOrder): §7.1 of both RFCs requires every check to pass but
// mandates no order; ze puts the VRID lookup before the checksum so other
// routers' multicast traffic is never checksummed, and computes the checksum
// over the FULL payload so a spurious v2 auth trailer surfaces as a length
// error, not a mystery checksum error.
package packet

import (
	"errors"
	"net/netip"
)

// RxMeta carries the IP-header facts the transport (spec-vrrp-4) extracts for a
// received VRRP datagram. It is a plain value struct, copied across boundaries.
type RxMeta struct {
	TTL     uint8      // IPv4 TTL or IPv6 hop-limit
	Src     netip.Addr // checksum pseudo-header input + FSM tie-break
	Dst     netip.Addr // checksum pseudo-header input
	Family  uint8      // V4 or V6
	IfIndex int        // informational; Decode ignores it (D-A)
}

// Local is the per-group state the engine (spec-vrrp-5) supplies through Lookup:
// the configured version (row 5) and configured interval in ms (v2 row 12).
type Local struct {
	Version         uint8
	AdverIntervalMS uint32
}

// Lookup resolves a VRID to its Local group state on the receiving
// interface+family. An unknown VRID returns (Local{}, false). The engine owns
// the group table (D-B).
type Lookup func(vrid uint8) (Local, bool)

// gtsmTTL is the mandated TTL / hop-limit for VRRP (GTSM, RFC 5082).
// RFC 9568 Section 5.1.1.3 / RFC 3768 Section 5.2.3.
const gtsmTTL uint8 = 255

// Receive-validation error taxonomy. Each maps 1:1 to a metrics reason label
// via Reason(); the transport counts them through ze_vrrp_packet_errors_total.
var (
	ErrTruncated          = errors.New("vrrp: packet shorter than 8 bytes")
	ErrVersion            = errors.New("vrrp: unsupported or mismatched version")
	ErrType               = errors.New("vrrp: type is not ADVERTISEMENT")
	ErrUnknownVRID        = errors.New("vrrp: vrid not configured on interface")
	ErrChecksum           = errors.New("vrrp: checksum verification failed")
	ErrTTL                = errors.New("vrrp: TTL/hop-limit is not 255")
	ErrCountZero          = errors.New("vrrp: address count is zero")
	ErrLength             = errors.New("vrrp: length does not match address count")
	ErrAuthType           = errors.New("vrrp: v2 auth type is not zero")
	ErrIntervalZero       = errors.New("vrrp: advertise interval is zero")
	ErrV2IntervalMismatch = errors.New("vrrp: v2 advertise interval mismatch")
	ErrFirstNotLinkLocal  = errors.New("vrrp: first IPv6 address is not link-local")
	ErrIPv4HeaderShort    = errors.New("vrrp: IPv4 header exceeds datagram")
	ErrIPv4BadIHL         = errors.New("vrrp: IPv4 IHL below 5")
)

// Accepted-outcome / engine-raised reason labels (NOT errors).
const (
	// ReasonMsgOnlyChecksum labels an ACCEPTED v3/IPv4 packet whose checksum
	// matched only the RFC 9568 message-only sum, not the RFC 5798 pseudo-header
	// form ze sends (MsgOnlyChecksum true). It marks a strict-RFC-9568 peer.
	ReasonMsgOnlyChecksum = "checksum-rfc9568-message-only"
	// ReasonAddressList is raised by the ENGINE (spec-vrrp-5) for the v2-only
	// address-list comparison; it is not a Decode ladder outcome.
	ReasonAddressList = "address-list"
)

// Reason maps a receive-validation error to its ze_vrrp_packet_errors_total
// reason label, returning "" for anything outside the receive taxonomy
// (encode-side errors, nil). The mapping is total and injective over the
// taxonomy, with ip-header intentionally shared by the two strip errors.
func Reason(err error) string {
	switch {
	case errors.Is(err, ErrTruncated):
		return "truncated"
	case errors.Is(err, ErrVersion):
		return "version"
	case errors.Is(err, ErrType):
		return "type"
	case errors.Is(err, ErrUnknownVRID):
		return "vrid"
	case errors.Is(err, ErrChecksum):
		return "checksum"
	case errors.Is(err, ErrTTL):
		return "ttl"
	case errors.Is(err, ErrCountZero):
		return "count-zero"
	case errors.Is(err, ErrLength):
		return "length"
	case errors.Is(err, ErrAuthType):
		return "auth-type"
	case errors.Is(err, ErrIntervalZero):
		return "interval-zero"
	case errors.Is(err, ErrV2IntervalMismatch):
		return "interval-mismatch"
	case errors.Is(err, ErrFirstNotLinkLocal):
		return "linklocal"
	case errors.Is(err, ErrIPv4HeaderShort), errors.Is(err, ErrIPv4BadIHL):
		return "ip-header"
	}
	return ""
}

// Decode parses and validates a received VRRP ADVERTISEMENT payload against the
// 13-row ordered ladder (TestValidationOrder). meta carries the IP-header facts
// and lookup resolves the VRID to its configured group (D-B). On success it
// returns a value Advertisement whose Virtual IP addresses are a lazy view over
// payload (valid until the next socket read, A-3). Decode never allocates on the
// happy path and never panics on malformed input.
func Decode(payload []byte, meta RxMeta, lookup Lookup) (Advertisement, error) {
	// Row 1: minimum length. RFC 9568 Section 7.1 / RFC 3768 Section 7.1:
	// "verify that the received packet contains the complete VRRP packet".
	if len(payload) < HeaderLen {
		return Advertisement{}, ErrTruncated
	}

	version := payload[0] >> 4
	msgType := payload[0] & 0x0F
	vrid := payload[1]
	priority := payload[2]
	count := payload[3]

	// Row 2: version nibble in {2,3}. RFC 9568 Section 7.1 / RFC 3768 Section 7.1.
	if version != VersionV2 && version != VersionV3 {
		return Advertisement{}, ErrVersion
	}
	// Row 3: type is ADVERTISEMENT (1). RFC 9568 Section 5.2.2 / RFC 3768
	// Section 5.3.2: "A packet with unknown type MUST be discarded".
	if msgType != TypeAdvertisement {
		return Advertisement{}, ErrType
	}
	// Row 4: VRID configured on the receiving interface. RFC 9568 Section 7.1
	// (erratum 8298 leaves owner-rx as FSM log-only, child 2).
	local, ok := lookup(vrid)
	if !ok {
		return Advertisement{}, ErrUnknownVRID
	}
	// Row 5: wire version matches the configured group version. RFC 9568
	// Section 7.1 ("verify the VRRP version"); one version per group (umbrella).
	if version != local.Version {
		return Advertisement{}, ErrVersion
	}
	// Row 6: checksum over the FULL received payload, actual src/dst from meta.
	// v3/IPv4 dual-accepts: RFC 5798 pseudo-header primary (ze's tx form and the
	// deployed base's), RFC 9568 message-only accepted+flagged / RFC 3768
	// Section 7.1 (v2).
	msgOnly, checksumOK := verifyReceived(payload, version, meta.Family, meta.Src, meta.Dst)
	if !checksumOK {
		return Advertisement{}, ErrChecksum
	}
	// Row 7: TTL / hop-limit is 255 (GTSM, RFC 5082). RFC 9568 Section 5.1.1.3,
	// Section 5.1.2.3 / RFC 3768 Section 5.2.3: "receiving a packet with the TTL
	// not equal to 255 MUST discard the packet".
	if meta.TTL != gtsmTTL {
		return Advertisement{}, ErrTTL
	}
	// Row 8: address count non-zero. RFC 9568 erratum 8299 / Section 5.2.5:
	// "If the received count is 0, the VRRP advertisement MUST be ignored".
	if count == 0 {
		return Advertisement{}, ErrCountZero
	}

	addrSize := 4
	if meta.Family == V6 {
		addrSize = 16
	}
	// Row 9: length exactness. RFC 9568 Section 7.1 / RFC 3768 Section 7.1 (v2
	// always carries the 8-byte Authentication Data trailer). Catches a v3
	// packet with a spurious auth trailer, trailing garbage, or a count lie.
	wantLen := HeaderLen + int(count)*addrSize
	if version == VersionV2 {
		wantLen += v2AuthTrailerLen
	}
	if len(payload) != wantLen {
		return Advertisement{}, ErrLength
	}

	var intervalMS uint32
	if version == VersionV2 {
		// Row 10: v2 auth type is 0. RFC 3768 Section 5.3.6, Section 7.1: types
		// 1, 2 and unknown MUST be discarded (no auth implemented, umbrella).
		if payload[4] != 0 {
			return Advertisement{}, ErrAuthType
		}
		// Row 11: interval extraction. RFC 3768 Section 5.3.7 (whole seconds).
		// Zero is stricter-than-RFC but can never match a legal local config
		// (1..255 s), so behavior equals the mandated mismatch discard.
		sec := payload[5]
		if sec == 0 {
			return Advertisement{}, ErrIntervalZero
		}
		intervalMS = v2SecondsToMS(sec)
		// Row 12: v2 MUST discard on interval mismatch. RFC 3768 Section 7.1:
		// "verify that the Adver Interval ... is the same as the locally
		// configured ... on mismatch the receiver MUST discard the packet".
		if intervalMS != local.AdverIntervalMS {
			return Advertisement{}, ErrV2IntervalMismatch
		}
	} else {
		// Row 11: v3 12-bit centisecond interval non-zero. RFC 9568 erratum
		// 8301: "a mandatory receive check that Max Advertise Interval is
		// non-zero".
		cs := uint16(payload[4]&0x0F)<<8 | uint16(payload[5])
		if cs == 0 {
			return Advertisement{}, ErrIntervalZero
		}
		intervalMS = v3CentisecondsToMS(cs)
		// Row 12: v3 NEVER errors on interval mismatch -- the value is returned
		// for FSM adoption. RFC 9568 Section 6.4.2 (Backup adopts the Active
		// Router's Max Advertise Interval); Section 7.1 is SHOULD-log only.
	}

	adv := Advertisement{
		Version:         version,
		Family:          meta.Family,
		VRID:            vrid,
		Priority:        priority,
		Count:           count,
		AdverIntervalMS: intervalMS,
		MsgOnlyChecksum: msgOnly,
		wireVIPs:        payload[HeaderLen : HeaderLen+int(count)*addrSize],
	}

	// Row 13: v3 IPv6 first VIP MUST be link-local. RFC 9568 erratum 8300 /
	// Section 5.2.9: "the first address MUST be the IPv6 link-local address".
	// (Address-family-vs-header mismatch cannot occur here: addrSize derives
	// from meta.Family and row 9 rejects any inconsistency.)
	if version == VersionV3 && meta.Family == V6 {
		if !adv.VIPAt(0).IsLinkLocalUnicast() {
			return Advertisement{}, ErrFirstNotLinkLocal
		}
	}

	return adv, nil
}

// ipv4MinHeader is the smallest valid IPv4 header (IHL 5).
const ipv4MinHeader = 20

// StripIPv4Header validates and removes the IPv4 header from a raw datagram
// delivered by an AF_INET raw socket, returning the VRRP payload and an RxMeta
// with TTL/src/dst extracted (Family V4; IfIndex left for the transport). It is
// IHL-aware so options-bearing datagrams are stripped correctly (holo's fixed
// 20-byte strip is a bug this prevents, N3). RFC 9568 Section 9 / RFC 5082: the
// TTL is extracted here so the ladder can enforce GTSM.
func StripIPv4Header(datagram []byte) ([]byte, RxMeta, error) {
	if len(datagram) < ipv4MinHeader {
		return nil, RxMeta{}, ErrIPv4HeaderShort
	}
	ihl := int(datagram[0] & 0x0F)
	if ihl < 5 {
		return nil, RxMeta{}, ErrIPv4BadIHL
	}
	headerLen := ihl * 4
	if headerLen > len(datagram) {
		return nil, RxMeta{}, ErrIPv4HeaderShort
	}
	var src, dst [4]byte
	copy(src[:], datagram[12:16])
	copy(dst[:], datagram[16:20])
	meta := RxMeta{
		TTL:    datagram[8],
		Src:    netip.AddrFrom4(src),
		Dst:    netip.AddrFrom4(dst),
		Family: V4,
	}
	return datagram[headerLen:], meta, nil
}

// Design: docs/architecture/plugin/rib-storage-design.md — Adj-RIB-In raw hex storage
// RFC: rfc/short/rfc4271.md -- Section 4.3, the [length][prefix] NLRI encoding
// RFC: rfc/short/rfc7911.md -- Section 3, the four-octet Path Identifier prepended to it
// RFC: rfc/short/rfc2545.md -- Section 3, the 16-or-32-octet IPv6 next-hop field
// Detail: rib.go — the ingest paths that call these helpers
//
// The wire-hex helpers the Adj-RIB-In ingest paths use to turn an event's bytes
// into what RawRoute stores. They are pure: they hold no manager state and take
// no lock.
//
// One rule governs all of them. A stored NLRI for a simple prefix family is the
// BARE RFC 4271 encoding, never the RFC 7911 extended one, and the path
// identifier travels beside it on RawRoute.PathID. Both storage framings existed
// before, one per ingest path, and nothing recorded which a given route carried
// -- so the relay could not emit either.

package adj_rib_in

import (
	"encoding/hex"
	"net"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// pathIDLen is the RFC 7911 Section 3 Path Identifier width: four octets,
// prepended to the NLRI when the session negotiated ADD-PATH for the family.
const pathIDLen = 4

// nhopToHex converts a next-hop IP address string to wire hex.
// IPv4: "10.0.0.1" -> "0a000001", IPv6: "::1" -> 32 hex chars.
func nhopToHex(ipStr string) string {
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return ""
	}
	if addr.Unmap().Is4() {
		b := addr.Unmap().As4()
		return hex.EncodeToString(b[:])
	}
	b := addr.As16()
	return hex.EncodeToString(b[:])
}

// splitRawNLRIHex splits a raw NLRI section into one bare RFC 4271 prefix per
// entry, in wire order. Returns nil for complex families (VPN, EVPN, FlowSpec),
// whose NLRI does not split into per-route prefix bytes.
//
// addPath says whether the source session negotiated ADD-PATH for this family.
// When it did, RFC 7911 Section 3 puts four octets of Path Identifier ahead of
// each prefix; those octets are skipped, not returned, so the caller stores the
// same bytes for an ADD-PATH source as for any other and keeps the identifier in
// RawRoute.PathID.
//
// The flag is not optional. Without it the first octet of a Path Identifier is
// read as a prefix length, and the entries returned are neither the prefixes the
// source announced nor any prefix at all -- the route is then keyed correctly
// from the parsed NLRI list and stored under the bytes of a different one.
func splitRawNLRIHex(rawHex string, fam family.Family, addPath bool) []string {
	data, err := hex.DecodeString(rawHex)
	if err != nil || len(data) == 0 {
		return nil
	}

	if !isSimplePrefixFamily(fam) {
		return nil
	}

	skip := 0
	if addPath {
		skip = pathIDLen
	}

	var result []string
	offset := 0
	// Bounded by len(data): every iteration advances offset by at least one
	// octet (the length byte), and an entry that runs past the end ends the walk.
	for offset < len(data) {
		if offset+skip >= len(data) {
			break
		}
		prefixOff := offset + skip
		prefixLen := int(data[prefixOff])
		nlriLen := 1 + (prefixLen+7)/8

		if prefixOff+nlriLen > len(data) {
			break
		}
		result = append(result, hex.EncodeToString(data[prefixOff:prefixOff+nlriLen]))
		offset = prefixOff + nlriLen
	}

	return result
}

// isSimplePrefixFamily returns true for families with simple [prefix-len][prefix-bytes] format.
// Complex families (VPN, EVPN, FlowSpec, etc.) have different NLRI structures.
func isSimplePrefixFamily(fam family.Family) bool {
	switch fam {
	case family.IPv4Unicast, family.IPv4Multicast, family.IPv6Unicast, family.IPv6Multicast:
		return true
	}
	return false
}

// prefixToWireHex converts a text prefix to bare RFC 4271 NLRI wire hex.
// Only correct for simple prefix families (IPv4/IPv6 unicast/multicast).
// Called as fallback when raw NLRI bytes are not available.
//
// It writes no Path Identifier, whatever the source session negotiated. The
// identifier is carried on RawRoute.PathID, so putting it in the bytes as well
// would give one route two framings again -- and the branch that did so wrote
// the four octets only when the identifier was non-zero, which lost the legal
// identifier 0 (RFC 7911 Section 3 reserves no value).
func prefixToWireHex(fam family.Family, prefix string) string {
	_, ipnet, err := net.ParseCIDR(prefix)
	if err != nil {
		return ""
	}

	prefixLen, _ := ipnet.Mask.Size()
	prefixBytes := (prefixLen + 7) / 8

	var ipBytes net.IP
	switch fam.AFI {
	case family.AFIIPv4:
		ipBytes = ipnet.IP.To4()
	case family.AFIIPv6:
		ipBytes = ipnet.IP.To16()
	case family.AFIL2VPN, family.AFIBGPLS:
		// Complex AFIs handled via raw blob path; prefixToWireHex not called.
	}

	if ipBytes == nil {
		return ""
	}

	wire := make([]byte, 1+prefixBytes)
	wire[0] = byte(prefixLen)
	copy(wire[1:], ipBytes[:prefixBytes])

	return hex.EncodeToString(wire)
}

// nhopHexFromWireAttr extracts next-hop from wire NEXT_HOP attribute and hex-encodes it.
func nhopHexFromWireAttr(attrs *attribute.AttributesWire) string {
	if attrs == nil {
		return ""
	}
	attr, err := attrs.Get(attribute.AttrNextHop)
	if err != nil || attr == nil {
		return ""
	}
	nhop, ok := attr.(*attribute.NextHop)
	if !ok || !nhop.Addr.IsValid() {
		return ""
	}
	return nhopHexFromAddr(nhop.Addr)
}

// nhopHexFromAddr hex-encodes a netip.Addr for RawRoute storage.
func nhopHexFromAddr(addr netip.Addr) string {
	if !addr.IsValid() {
		return ""
	}
	if addr.Unmap().Is4() {
		b := addr.Unmap().As4()
		return hex.EncodeToString(b[:])
	}
	b := addr.As16()
	return hex.EncodeToString(b[:])
}

// mpReachNextHopHex reports the MP_REACH_NLRI next-hop field of a raw attribute
// block, hex-encoded, together with the family that attribute carries.
//
// The WHOLE field, exactly as the source framed it. RFC 2545 Section 3 lets an
// IPv6 next hop be a global address followed by a link-local one. A route stored
// with the global half alone can never be re-advertised in the form the source
// sent (RFC2545-3-1, RFC2545-3-2).
//
// A relayed route IS a re-advertisement, and the live forward rail relays the
// source's own bytes. So 16 stored octets for a 32-octet field put the two rails
// on different wire.
//
// Returns ("", zero family) when the block holds no MP_REACH_NLRI, or cannot be
// indexed. The caller then falls back to the event's own next-hop string, which
// is all a legacy IPv4 unicast announcement carries.
func mpReachNextHopHex(attrHex string) (string, family.Family) {
	if attrHex == "" {
		return "", family.Family{}
	}
	packed, err := hex.DecodeString(attrHex)
	if err != nil {
		return "", family.Family{}
	}
	idx, err := attribute.BuildSpanIndex(packed)
	if err != nil {
		return "", family.Family{}
	}
	span, ok := idx.Find(attribute.AttrMPReachNLRI)
	if !ok {
		return "", family.Family{}
	}
	end := int(span.Offset) + int(span.Length)
	if end > len(packed) {
		return "", family.Family{}
	}
	mp := wireu.MPReachWire(packed[span.Offset:end])
	nh := mp.NextHopBytes()
	if len(nh) == 0 {
		return "", family.Family{}
	}
	return hex.EncodeToString(nh), mp.Family()
}

func legacyNextHop(attrs *attribute.AttributesWire) string {
	if attrs == nil {
		return ""
	}
	attr, err := attrs.Get(attribute.AttrNextHop)
	if err != nil || attr == nil {
		return ""
	}
	nhop, ok := attr.(*attribute.NextHop)
	if !ok || !nhop.Addr.IsValid() {
		return ""
	}
	return nhop.Addr.String()
}

// wireNLRIsToAny walks wire NLRI bytes and returns prefix strings as []any.
// Uses stack-allocated [16]byte buffer to avoid per-prefix heap allocation.
func wireNLRIsToAny(data []byte, addPath bool, fam family.Family) []any {
	isIPv6 := fam.AFI == family.AFIIPv6
	addrLen := 4
	if isIPv6 {
		addrLen = 16
	}

	var result []any
	var buf [16]byte // stack-allocated — large enough for IPv6
	offset := 0
	for offset < len(data) {
		if addPath {
			if offset+pathIDLen >= len(data) {
				break
			}
			offset += pathIDLen // RFC 7911 Section 3 Path Identifier
		}
		if offset >= len(data) {
			break
		}
		prefixLen := int(data[offset])
		byteCount := (prefixLen + 7) / 8
		offset++ // skip prefix-len byte
		if offset+byteCount > len(data) {
			break
		}
		// Zero and fill from wire — reuse stack buffer each iteration.
		clear(buf[:])
		copy(buf[:], data[offset:offset+byteCount])
		offset += byteCount

		addr, ok := netip.AddrFromSlice(buf[:addrLen])
		if !ok {
			continue
		}
		result = append(result, netip.PrefixFrom(addr, prefixLen).String())
	}
	return result
}

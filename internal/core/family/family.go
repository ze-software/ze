// Design: docs/architecture/wire/nlri.md -- BGP address family types
// Detail: registry.go -- runtime registration and string cache
//
// Package family defines the BGP address family types (AFI, SAFI, Family) and
// their registry. These types are core infrastructure used by BGP and other
// components -- they live in internal/core/ alongside clock, env, metrics.
//
// Wire-format definitions follow RFC 4760 (Multiprotocol Extensions for BGP-4):
//   - AFI: 2-octet Address Family Identifier (IANA Address Family Numbers)
//   - SAFI: 1-octet Subsequent Address Family Identifier (IANA SAFI registry)
//   - Family: <AFI, SAFI> tuple identifying NLRI semantics
package family

import (
	"fmt"
	"strconv"
)

// AFI represents Address Family Identifier.
// RFC 4760 Section 3: AFI is a 2-octet field in MP_REACH_NLRI/MP_UNREACH_NLRI.
// Values are assigned by IANA Address Family Numbers registry.
type AFI uint16

// Address Family Identifiers.
// RFC 4760 Section 3: "Presently defined values for the Address Family
// Identifier field are specified in the IANA's Address Family Numbers registry"
// See: https://www.iana.org/assignments/address-family-numbers/
const (
	AFIIPv4  AFI = 1     // IPv4 - RFC 4760
	AFIIPv6  AFI = 2     // IPv6 - RFC 4760
	AFIL2VPN AFI = 25    // L2VPN - RFC 4761, RFC 7432
	AFIBGPLS AFI = 16388 // BGP-LS - RFC 9552
)

// String returns the registered name for this AFI, or "afi-N" if unregistered.
func (a AFI) String() string {
	if s := lookupAFIName(a); s != "" {
		return s
	}
	return afiStringFallback(a)
}

// AppendTo appends the registered AFI name (or "afi-N" fallback) to buf and
// returns the extended slice. No intermediate allocation -- known values copy
// straight from the registered name into buf.
func (a AFI) AppendTo(buf []byte) []byte {
	if s := lookupAFIName(a); s != "" {
		return append(buf, s...)
	}
	buf = append(buf, "afi-"...)
	return strconv.AppendUint(buf, uint64(a), 10)
}

// MarshalText renders the AFI as its registered name (e.g. "ipv4"). Makes the
// type JSON-round-trippable without custom field wrappers.
func (a AFI) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}

// UnmarshalText parses a registered AFI name and stores the numeric value.
// Returns an error for unregistered names; callers typically propagate as
// parse failure.
func (a *AFI) UnmarshalText(data []byte) error {
	v, ok := LookupAFI(string(data))
	if !ok {
		return fmt.Errorf("family: unregistered AFI %q", string(data))
	}
	*a = v
	return nil
}

// SAFI represents Subsequent Address Family Identifier.
// RFC 4760 Section 3: SAFI is a 1-octet field in MP_REACH_NLRI/MP_UNREACH_NLRI.
// RFC 4760 Section 6 defines values 1 (unicast) and 2 (multicast).
// Additional values are assigned by IANA SAFI registry.
type SAFI uint8

// Subsequent Address Family Identifiers.
// RFC 4760 Section 6 defines base values. Additional values from IANA registry.
// See: https://www.iana.org/assignments/safi-namespace/
const (
	SAFIUnicast         SAFI = 1   // RFC 4760 Section 6
	SAFIMulticast       SAFI = 2   // RFC 4760 Section 6
	SAFIMPLSLabel       SAFI = 4   // RFC 8277
	SAFIMVPN            SAFI = 5   // RFC 6514
	SAFIVPLS            SAFI = 65  // RFC 4761
	SAFIEVPN            SAFI = 70  // RFC 7432
	SAFIBGPLinkState    SAFI = 71  // RFC 7752
	SAFIBGPLinkStateVPN SAFI = 72  // RFC 7752
	SAFIMUP             SAFI = 85  // draft-mpmz-bess-mup-safi
	SAFIVPN             SAFI = 128 // RFC 4364 (VPNv4), RFC 4659 (VPNv6)
	SAFIRTC             SAFI = 132 // RFC 4684
	SAFIFlowSpec        SAFI = 133 // RFC 8955
	SAFIFlowSpecVPN     SAFI = 134 // RFC 8955
)

// String returns the registered name for this SAFI, or "safi-N" if unregistered.
func (s SAFI) String() string {
	if name := lookupSAFIName(s); name != "" {
		return name
	}
	return safiStringFallback(s)
}

// AppendTo appends the registered SAFI name (or "safi-N" fallback) to buf and
// returns the extended slice. No intermediate allocation.
func (s SAFI) AppendTo(buf []byte) []byte {
	if name := lookupSAFIName(s); name != "" {
		return append(buf, name...)
	}
	buf = append(buf, "safi-"...)
	return strconv.AppendUint(buf, uint64(s), 10)
}

// MarshalText renders the SAFI as its registered name (e.g. "unicast"). Makes
// the type JSON-round-trippable without custom field wrappers.
func (s SAFI) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// UnmarshalText parses a registered SAFI name and stores the numeric value.
// Returns an error for unregistered names.
func (s *SAFI) UnmarshalText(data []byte) error {
	v, ok := LookupSAFI(string(data))
	if !ok {
		return fmt.Errorf("family: unregistered SAFI %q", string(data))
	}
	*s = v
	return nil
}

// Family combines AFI and SAFI to identify an address family.
// RFC 4760 Section 3: The combination of <AFI, SAFI> identifies the semantics
// of the Network Layer Reachability Information that follows.
type Family struct {
	AFI  AFI
	SAFI SAFI
}

// FamilyLess provides deterministic ordering for sorted iteration.
// Orders by AFI first, then SAFI. Used for consistent EOR ordering in tests.
func FamilyLess(a, b Family) bool {
	if a.AFI != b.AFI {
		return a.AFI < b.AFI
	}
	return a.SAFI < b.SAFI
}

// String returns a human-readable family name.
// Format: <afi>/<safi> (e.g., "ipv4/unicast", "l2vpn/evpn").
// Known families are served from a packed contiguous buffer (~1.3KB, L1-resident)
// via unsafe.String (zero allocation, no copy).
// Unknown families fall back to string concatenation.
func (f Family) String() string {
	if s := lookupFamilyString(f); s != "" {
		return s
	}
	return f.AFI.String() + "/" + f.SAFI.String()
}

// AppendTo appends the family name to buf and returns the extended slice.
// Known families copy straight from the packed back-store into buf (one memcpy,
// no intermediate allocation). Unknown families recurse into AFI.AppendTo +
// '/' + SAFI.AppendTo.
func (f Family) AppendTo(buf []byte) []byte {
	if s := lookupFamilyString(f); s != "" {
		return append(buf, s...)
	}
	buf = f.AFI.AppendTo(buf)
	buf = append(buf, '/')
	return f.SAFI.AppendTo(buf)
}

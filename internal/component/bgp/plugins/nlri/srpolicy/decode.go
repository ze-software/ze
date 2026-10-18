// Design: docs/architecture/wire/nlri.md -- SR-Policy NLRI decode for CLI
// RFC: rfc/short/rfc9830.md -- SR-Policy NLRI wire format
// Related: types.go -- Parse, SRPolicy struct
// Related: split.go -- the same NLRI framing for the receive path
// Related: register.go -- plugin registration

package srpolicy

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/family"
)

// errSRPolicyEmptySection refuses a section that holds no NLRI, so an empty
// answer is never mistaken for a decoded one.
var errSRPolicyEmptySection = errors.New("sr-policy: no NLRI in an empty section; give the hex of at least one NLRI, starting with its Length octet")

// DecodeNLRIHex decodes the SR-Policy NLRIs of one MP_REACH_NLRI or
// MP_UNREACH_NLRI section from hex, in their wire form, returning a
// JSON-friendly value: one object for one NLRI, a list for several.
// Registered as InProcessNLRIDecoder in the plugin registry.
//
// addPath states whether each NLRI carries a 4-octet Path Identifier ahead of it
// (RFC 7911 Section 3). The hex alone cannot say, so the flag travels with it.
func DecodeNLRIHex(familyStr, hexStr string, addPath, _ bool) (any, error) {
	afi, err := familyToAFI(familyStr)
	if err != nil {
		return nil, err
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	entries, err := decodeSRPolicySection(afi, data, addPath)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", familyStr, err)
	}

	objects := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		object := map[string]any{
			fieldColor:         e.policy.color,
			fieldDistinguisher: e.policy.distinguisher,
			fieldEndpoint:      e.policy.endpoint.String(),
		}
		if addPath {
			// RFC 7911 Section 3 reserves no Path Identifier value, so zero is an
			// identifier rather than its absence: publish it whenever ADD-PATH
			// put one on the wire.
			object["path-id"] = e.pathID
		}
		objects = append(objects, object)
	}
	if len(objects) == 1 {
		return objects[0], nil
	}
	return objects, nil
}

// decodedSRPolicy is one NLRI of a decoded section, with the Path Identifier
// that preceded it under ADD-PATH (zero and meaningless otherwise).
type decodedSRPolicy struct {
	policy *SRPolicy
	pathID uint32
}

// decodeSRPolicySection walks one MP NLRI section of SAFI 73 and parses every
// NLRI in it. A peer controls these bytes, so every defect is an error naming
// its offset, never a panic and never a partial answer.
//
// RFC 9830 Section 2.1, Figure 1 (SR Policy SAFI Format), one NLRI:
//
//	offset  size        field
//	[0..4)  4 octets    Path Identifier (RFC 7911, only when ADD-PATH is negotiated)
//	[p+0]   1 octet     NLRI Length, in bits: 96 for AFI 1, 192 for AFI 2
//	[p+1]   4 octets    Distinguisher
//	[p+5]   4 octets    Color
//	[p+9]   4 or 16     Endpoint (IPv4 for AFI 1, IPv6 for AFI 2)
//
// p is 4 under ADD-PATH and 0 otherwise.
//
// RFC 9830 Section 2.1: "NLRI Length:  1 octet indicating the length expressed in
// bits as defined in [RFC4760].  When AFI = 1, the value MUST be 96; when AFI = 2,
// the value MUST be 192."
//
// Each pass of the loop advances off by at least one octet or returns, so the
// walk is bounded by len(data).
func decodeSRPolicySection(afi family.AFI, data []byte, addPath bool) ([]decodedSRPolicy, error) {
	if len(data) == 0 {
		return nil, errSRPolicyEmptySection
	}
	body := bodyLen(afi)
	wantBits := body * 8
	var entries []decodedSRPolicy
	for off := 0; off < len(data); {
		start := off
		var pathID uint32
		if addPath {
			if len(data)-off < pathIDOctets {
				return nil, fmt.Errorf("NLRI at offset %d: path identifier is cut short (need %d octets, have %d)",
					start, pathIDOctets, len(data)-off)
			}
			pathID = binary.BigEndian.Uint32(data[off:])
			off += pathIDOctets
			if off == len(data) {
				return nil, fmt.Errorf("NLRI at offset %d: Length octet missing after the path identifier", start)
			}
		}
		bits := int(data[off])
		if bits != wantBits {
			return nil, fmt.Errorf("NLRI at offset %d: length %d bits, RFC 9830 Section 2.1 requires %d for this AFI; "+
				"the hex must be the wire form, which starts each NLRI with its Length octet (0x%02X)",
				start, bits, wantBits, wantBits)
		}
		end := off + 1 + body
		if end > len(data) {
			return nil, fmt.Errorf("NLRI at offset %d: extends past the data (need %d octets, have %d)",
				start, end-start, len(data)-start)
		}
		sp, err := Parse(afi, data[off+1:end])
		if err != nil {
			return nil, fmt.Errorf("NLRI at offset %d: %w", start, err)
		}
		entries = append(entries, decodedSRPolicy{policy: sp, pathID: pathID})
		off = end
	}
	return entries, nil
}

// familyToAFI resolves a family name to the AFI of the SR-Policy family it
// names. The name is looked up in the registry, which composed it from the AFI
// and SAFI parts register.go passed to family.MustRegister, so an unregistered
// name and a family that is not SR-Policy are both refused here.
func familyToAFI(familyStr string) (family.AFI, error) {
	fam, ok := family.LookupFamily(familyStr)
	if !ok {
		return 0, fmt.Errorf("unsupported sr-policy family: %s", familyStr)
	}
	if fam != IPv4SRPolicy && fam != IPv6SRPolicy {
		return 0, fmt.Errorf("unsupported sr-policy family: %s", familyStr)
	}
	return fam.AFI, nil
}

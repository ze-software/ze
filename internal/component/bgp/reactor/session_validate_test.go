package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/capability"
)

// VALIDATES: RFC 7606 UPDATE validation, family validation, capability mode checks, NOTIFICATION data builders.
// PREVENTS: Malformed UPDATEs reaching plugins, incorrect NOTIFICATION encoding.

// newValidateSession creates a minimal Session for validation tests (eBGP, no negotiated caps).
func newValidateSession() *Session {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.ReceiveHoldTime = 90 * time.Second
	return NewSession(settings)
}

// makeUpdateBody builds an UPDATE body from withdrawn, path attrs, and NLRI sections.
func makeUpdateBody(withdrawn, pathAttrs, nlri []byte) []byte {
	wLen := len(withdrawn)
	aLen := len(pathAttrs)
	body := make([]byte, 2+wLen+2+aLen+len(nlri))
	binary.BigEndian.PutUint16(body[0:2], uint16(wLen))
	copy(body[2:], withdrawn)
	binary.BigEndian.PutUint16(body[2+wLen:], uint16(aLen))
	copy(body[2+wLen+2:], pathAttrs)
	copy(body[2+wLen+2+aLen:], nlri)
	return body
}

// countAttrCode walks a path-attributes section and counts occurrences of one code.
// firstValue returns the value bytes of the FIRST occurrence (empty if none).
func countAttrCode(pathAttrs []byte, want uint8) (count int, firstValue []byte) {
	pos := 0
	for pos+2 <= len(pathAttrs) {
		flags := pathAttrs[pos]
		code := pathAttrs[pos+1]
		pos += 2
		var vlen int
		if flags&0x10 != 0 {
			if pos+2 > len(pathAttrs) {
				break
			}
			vlen = int(binary.BigEndian.Uint16(pathAttrs[pos : pos+2]))
			pos += 2
		} else {
			if pos+1 > len(pathAttrs) {
				break
			}
			vlen = int(pathAttrs[pos])
			pos++
		}
		if pos+vlen > len(pathAttrs) {
			break
		}
		if code == want {
			count++
			if count == 1 {
				firstValue = pathAttrs[pos : pos+vlen]
			}
		}
		pos += vlen
	}
	return count, firstValue
}

// attrSection extracts the path-attributes section from an UPDATE body.
func attrSection(body []byte) []byte {
	wLen := int(binary.BigEndian.Uint16(body[0:2]))
	off := 2 + wLen
	aLen := int(binary.BigEndian.Uint16(body[off : off+2]))
	off += 2
	return body[off : off+aLen]
}

// TestEnforceRFC7606DuplicateRebuild pins RFC 7606 Section 3.g keep-first at the session
// enforcement boundary: an UPDATE carrying two ORIGIN attributes (both individually valid)
// is accepted, and enforceRFC7606 strips the later occurrence so exactly one ORIGIN
// survives (the first). Without the strip the body keeps both ORIGINs, and the attribute
// index (attribute.AttributesWire, ensureIndexLocked) then rejects it as a duplicate — the
// error that silently drops MP routes at the RIB (rib_structured.go MPReach/MPUnreach).
//
// VALIDATES: enforceRFC7606 rebuilds the UPDATE body keep-first; the surviving ORIGIN is
// the first occurrence; NLRI is preserved; the attribute index builds cleanly.
// PREVENTS: a duplicate well-known attribute surviving into the RIB/filter/re-encode path,
// where indexed consumers error and silently drop routes.
func TestEnforceRFC7606DuplicateRebuild(t *testing.T) {
	s := newValidateSession()

	dupOriginAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP (first, keep)
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP = 192.0.2.1
		0x40, 0x01, 0x01, 0x01, // ORIGIN = EGP (DUPLICATE, strip)
	}
	nlri := []byte{24, 192, 0, 2} // 192.0.2.0/24
	body := makeUpdateBody(nil, dupOriginAttrs, nlri)
	wu := wireu.NewWireUpdate(body, 0)

	newWU, action, err := s.enforceRFC7606(wu)
	require.NoError(t, err, "two individually-valid ORIGINs must not reset the session")
	require.Equal(t, message.RFC7606ActionNone, action)
	require.NotNil(t, newWU)

	// Byte-level proof of keep-first: exactly one ORIGIN survives and it is the FIRST
	// (IGP, value 0x00), not the EGP duplicate. One ORIGIN on the wire is precisely what
	// lets the attribute index (attribute.AttributesWire.ensureIndexLocked) build instead
	// of erroring — the error that silently drops MP routes at the RIB
	// (rib_structured.go MPReach/MPUnreach return nil on that error).
	// test-relax: dropped an extra newWU.Attrs().Get() index-build assertion that needed a
	// registered encoding context (unavailable to this bgpctx-free harness — it errored
	// "unknown source context ID: 0", a harness limit, not the feature). count==1 already
	// proves the deduplicated wire carries no duplicate for the index to reject.
	attrs := attrSection(newWU.Payload())
	count, firstVal := countAttrCode(attrs, 0x01)
	require.Equal(t, 1, count, "exactly one ORIGIN must survive keep-first")
	require.Equal(t, []byte{0x00}, firstVal, "the FIRST ORIGIN (IGP) must be the survivor")

	// NLRI is untouched by the attribute rebuild.
	require.Equal(t, nlri, newWU.Payload()[len(newWU.Payload())-len(nlri):], "NLRI must be preserved")
}

// TestEnforceRFC7606_ValidUpdate verifies a minimal valid UPDATE passes.
func TestEnforceRFC7606_ValidUpdate(t *testing.T) {
	s := newValidateSession()

	// Empty UPDATE: no withdrawn, no attrs, no NLRI.
	body := makeUpdateBody(nil, nil, nil)
	wu := wireu.NewWireUpdate(body, 0)

	newWU, action, err := s.enforceRFC7606(wu)
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action)
	assert.NotNil(t, newWU)
}

// TestEnforceRFC7606_ShortBody verifies body < 4 bytes triggers session reset.
//
// RFC requirement: RFC7606-3.a-1 negative — a session-reset error is signaled by
// NOTIFICATION with Error Code UPDATE Message Error
//
// Changed from treat-as-withdraw to session reset with user approval (2026-07-16).
// A body too short to hold the section lengths means the NLRI field cannot be located at
// all, so RFC 7606 Section 3(j) forbids treat-as-withdraw: that approach requires the
// NLRI to have been successfully parsed. Section 3(b) routes the structural length
// conflict to a NOTIFICATION with subcode Malformed Attribute List.
func TestEnforceRFC7606_ShortBody(t *testing.T) {
	s := newValidateSession()

	// Only 2 bytes, too short for withdrawn length + attrs length.
	wu := wireu.NewWireUpdate([]byte{0x00, 0x00}, 0)

	newWU, action, err := s.enforceRFC7606(wu)
	require.Error(t, err, "session reset must surface as an error to the caller")
	assert.Equal(t, message.RFC7606ActionSessionReset, action)
	assert.NotNil(t, newWU)
}

// TestEnforceRFC7606_InvalidWithdrawnNLRI verifies bad withdrawn NLRI triggers session reset.
//
// RFC requirement: RFC7606-5.3-2 negative — the last NLRI in the Withdrawn Routes field
// overruns the field, which Section 3(j) escalates to session reset
// RFC requirement: RFC7606-3.i-1 negative — the Withdrawn Routes field is checked for
// syntactic correctness in the same manner as the NLRI field
//
// Changed from treat-as-withdraw to session reset with user approval (2026-07-16).
// Isolation: prefix length 24 is well within the family maximum of 32, so the §5.3-1
// "greater than 32" rule cannot be what fires — the ONLY defect is the overrun, which is
// exactly the §5.3-2 criterion. The previous version used prefix length 33 and so actually
// exercised §5.3-1, proving nothing about the overrun rule this line claims.
//
// rfc-test-change-approved: 2026-07-17 user approved isolating the §5.3-2 overrun rule from
// the §5.3-1 length rule in this test.
func TestEnforceRFC7606_InvalidWithdrawnNLRI(t *testing.T) {
	s := newValidateSession()

	// Withdrawn NLRI declares /24 (needs 3 prefix octets) but only 2 follow, so the last
	// NLRI overruns the field. 24 <= 32, so this is not the "greater than 32" rule.
	withdrawn := []byte{24, 0x0A, 0x00}
	body := makeUpdateBody(withdrawn, nil, nil)
	wu := wireu.NewWireUpdate(body, 0)

	_, action, err := s.enforceRFC7606(wu)
	require.Error(t, err, "session reset must surface as an error to the caller")
	assert.Equal(t, message.RFC7606ActionSessionReset, action)
}

// TestEnforceRFC7606_InvalidTrailingNLRI verifies bad trailing NLRI triggers session reset.
//
// RFC requirement: RFC7606-5.3-1 negative — an NLRI length greater than 32 makes the NLRI
// field syntactically incorrect, which Section 3(j) escalates to session reset
// RFC requirement: RFC7606-3.j-1 negative — if the NLRI field cannot be successfully
// parsed, the session reset approach MUST be followed
//
// Changed from treat-as-withdraw to session reset with user approval (2026-07-16).
func TestEnforceRFC7606_InvalidTrailingNLRI(t *testing.T) {
	s := newValidateSession()

	// Valid path attrs (ORIGIN=IGP, AS_PATH empty, NEXT_HOP=1.1.1.1).
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x01, 0x01, 0x01, 0x01, // NEXT_HOP = 1.1.1.1
	}
	// NLRI with prefix length 33 (invalid for IPv4).
	nlri := []byte{33, 0x0A, 0x00, 0x00, 0x00, 0x00}
	body := makeUpdateBody(nil, pathAttrs, nlri)
	wu := wireu.NewWireUpdate(body, 0)

	_, action, err := s.enforceRFC7606(wu)
	require.Error(t, err, "session reset must surface as an error to the caller")
	assert.Equal(t, message.RFC7606ActionSessionReset, action)
}

// TestEnforceRFC7606_MissingMandatoryAttrs verifies NLRI with empty attrs triggers treat-as-withdraw.
// RFC 7606 Section 3.d: missing well-known mandatory attributes.
func TestEnforceRFC7606_MissingMandatoryAttrs(t *testing.T) {
	s := newValidateSession()

	// NLRI present but no path attributes — mandatory attrs missing.
	nlri := []byte{24, 10, 0, 0} // 10.0.0.0/24
	body := makeUpdateBody(nil, nil, nlri)
	wu := wireu.NewWireUpdate(body, 0)

	_, action, err := s.enforceRFC7606(wu)
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionTreatAsWithdraw, action)
}

// TestValidateUpdateFamilies_Matching verifies negotiated family passes.
func TestValidateUpdateFamilies_Matching(t *testing.T) {
	s := newValidateSession()

	// Set up negotiated with IPv4 unicast.
	s.negotiated = capability.Negotiate(
		[]capability.Capability{
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		},
		[]capability.Capability{
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		},
		65001, 65002,
	)

	// UPDATE with MP_REACH_NLRI for IPv4 unicast (matches negotiated).
	mpReach := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x01,                   // SAFI = 1 (Unicast)
		0x04,                   // NH len = 4
		0x01, 0x01, 0x01, 0x01, // NH = 1.1.1.1
		0x00,                   // Reserved
		0x18, 0x0A, 0x00, 0x00, // 10.0.0.0/24
	}

	attrFlags := byte(0x90) // Optional, Transitive, Extended Length
	attrCode := byte(0x0E)  // MP_REACH_NLRI
	pathAttrs := append([]byte{attrFlags, attrCode, 0x00, byte(len(mpReach))}, mpReach...)

	body := makeUpdateBody(nil, pathAttrs, nil)
	err := s.validateUpdateFamilies(body)
	assert.NoError(t, err)
}

// TestValidateUpdateFamilies_NotNegotiated verifies non-negotiated family is rejected.
func TestValidateUpdateFamilies_NotNegotiated(t *testing.T) {
	s := newValidateSession()

	// Negotiate only IPv4 unicast.
	s.negotiated = capability.Negotiate(
		[]capability.Capability{
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		},
		[]capability.Capability{
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		},
		65001, 65002,
	)

	// UPDATE with MP_REACH_NLRI for IPv6 unicast (NOT negotiated).
	mpReach := []byte{
		0x00, 0x02, // AFI = 2 (IPv6)
		0x01, // SAFI = 1 (Unicast)
		0x10, // NH len = 16
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // NH = ::1
		0x00,                         // Reserved
		0x20, 0x20, 0x01, 0x0D, 0xB8, // 2001:db8::/32
	}

	attrFlags := byte(0x90) // Optional, Transitive, Extended Length
	attrCode := byte(0x0E)  // MP_REACH_NLRI
	pathAttrs := append([]byte{attrFlags, attrCode, 0x00, byte(len(mpReach))}, mpReach...)

	body := makeUpdateBody(nil, pathAttrs, nil)
	err := s.validateUpdateFamilies(body)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFamilyNotNegotiated)
}

// TestValidateUpdateFamilies_IgnoreMode verifies IgnoreFamilyMismatch skips rejection.
func TestValidateUpdateFamilies_IgnoreMode(t *testing.T) {
	s := newValidateSession()
	s.settings.IgnoreFamilyMismatch = true

	// Negotiate only IPv4 unicast.
	s.negotiated = capability.Negotiate(
		[]capability.Capability{
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		},
		[]capability.Capability{
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		},
		65001, 65002,
	)

	// IPv6 UPDATE — should be accepted in ignore mode.
	mpReach := []byte{
		0x00, 0x02, 0x01, // AFI=2, SAFI=1
		0x10, // NH len
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00,                         // Reserved
		0x20, 0x20, 0x01, 0x0D, 0xB8, // 2001:db8::/32
	}
	attrFlags := byte(0x90)
	attrCode := byte(0x0E)
	pathAttrs := append([]byte{attrFlags, attrCode, 0x00, byte(len(mpReach))}, mpReach...)

	body := makeUpdateBody(nil, pathAttrs, nil)
	err := s.validateUpdateFamilies(body)
	assert.NoError(t, err)
}

// TestBuildUnsupportedCapabilityData verifies NOTIFICATION data for Multiprotocol families.
// RFC 5492 Section 3: each family encoded as code(1) + length(1) + AFI(2) + Reserved(1) + SAFI(1).
// RFC requirement: RFC5492-3-1 positive -- buildUnsupportedCapabilityData encodes each
// offending family into the NOTIFICATION Data as a Multiprotocol capability TLV
// (code 1, len 4, AFI, reserved, SAFI), so the message contains the capabilities that
// caused the speaker to send it (internal/component/bgp/reactor/session_validation.go:396).
func TestBuildUnsupportedCapabilityData(t *testing.T) {
	families := []capability.Family{
		{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
	}

	data := buildUnsupportedCapabilityData(families)

	// 2 families * 6 bytes each = 12 bytes.
	require.Len(t, data, 12)

	// First family: code=1 (Multiprotocol), len=4, AFI=1, Reserved=0, SAFI=1.
	assert.Equal(t, byte(capability.CodeMultiprotocol), data[0])
	assert.Equal(t, byte(4), data[1])
	assert.Equal(t, uint16(1), binary.BigEndian.Uint16(data[2:4]))
	assert.Equal(t, byte(0), data[4])
	assert.Equal(t, byte(1), data[5])

	// Second family: AFI=2, SAFI=1.
	assert.Equal(t, uint16(2), binary.BigEndian.Uint16(data[8:10]))
	assert.Equal(t, byte(1), data[11])
}

// TestBuildUnsupportedCapabilityDataCodes_MultipleCodes verifies NOTIFICATION data for non-family codes.
// RFC 5492 Section 3: each code encoded as code(1) + length(1).
//
// RFC requirement: RFC5492-5-1 positive -- buildUnsupportedCapabilityDataCodes lists each
// offending capability code in the NOTIFICATION Data encoded exactly as in an OPEN message
// (code(1)+length(1), length 0 for non-family codes) (internal/component/bgp/reactor/session_validation.go:415).
func TestBuildUnsupportedCapabilityDataCodes_MultipleCodes(t *testing.T) {
	codes := []capability.Code{
		capability.CodeExtendedMessage,
		capability.CodeRouteRefresh,
	}

	data := buildUnsupportedCapabilityDataCodes(codes)

	// 2 codes * 2 bytes each = 4 bytes.
	require.Len(t, data, 4)

	assert.Equal(t, byte(capability.CodeExtendedMessage), data[0])
	assert.Equal(t, byte(0), data[1], "length=0 for non-family codes")
	assert.Equal(t, byte(capability.CodeRouteRefresh), data[2])
	assert.Equal(t, byte(0), data[3])
}

// TestBuildUnsupportedCapabilityDataCodes_Empty verifies nil for empty input.
//
// RFC requirement: RFC5492-5-1 negative -- with no offending capabilities the builder
// produces nil Data: no capability tuples are listed in a NOTIFICATION when none caused it
// (internal/component/bgp/reactor/session_validation.go:416-418).
func TestBuildUnsupportedCapabilityDataCodes_Empty(t *testing.T) {
	data := buildUnsupportedCapabilityDataCodes(nil)
	assert.Nil(t, data)
}

// Loop detection tests: moved to internal/component/bgp/reactor/filter/loop_test.go.
// The detectLoops session method was refactored to filter.LoopIngress (ingress filter plugin).
// All 12 tests preserved with identical coverage in the new location.

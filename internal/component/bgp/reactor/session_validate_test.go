package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
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

// TestEnforceRFC7606_ShortBody verifies body < 4 bytes is passed through.
func TestEnforceRFC7606_ShortBody(t *testing.T) {
	s := newValidateSession()

	// Only 2 bytes — too short for withdrawn length + attrs length.
	wu := wireu.NewWireUpdate([]byte{0x00, 0x00}, 0)

	newWU, action, err := s.enforceRFC7606(wu)
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action)
	assert.NotNil(t, newWU)
}

// TestEnforceRFC7606_InvalidWithdrawnNLRI verifies bad withdrawn NLRI triggers treat-as-withdraw.
// RFC 7606 Section 5.3: invalid prefix length in withdrawn routes.
func TestEnforceRFC7606_InvalidWithdrawnNLRI(t *testing.T) {
	s := newValidateSession()

	// Withdrawn with prefix length 33 (invalid for IPv4, max is 32).
	withdrawn := []byte{33, 0x0A, 0x00, 0x00, 0x00, 0x00}
	body := makeUpdateBody(withdrawn, nil, nil)
	wu := wireu.NewWireUpdate(body, 0)

	_, action, err := s.enforceRFC7606(wu)
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionTreatAsWithdraw, action)
}

// TestEnforceRFC7606_InvalidTrailingNLRI verifies bad trailing NLRI triggers treat-as-withdraw.
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
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionTreatAsWithdraw, action)
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
func TestBuildUnsupportedCapabilityDataCodes_Empty(t *testing.T) {
	data := buildUnsupportedCapabilityDataCodes(nil)
	assert.Nil(t, data)
}

// --- Loop Detection Tests ---
//
// VALIDATES: RFC 4271 Section 9 (AS loop), RFC 4456 Section 8 (originator-ID and cluster-list loops).
// PREVENTS: Routes looping through the local AS or reflected back to originators.

// newIBGPSession creates an iBGP session (LocalAS == PeerAS) for loop detection tests.
func newIBGPSession() *Session {
	// Router ID = 1.2.3.1 = 0x01020301
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01020301)
	return NewSession(settings)
}

// buildASPathAttr builds an AS_PATH attribute with a single AS_SEQUENCE segment.
// asn4 controls whether ASNs are 2 or 4 bytes.
func buildASPathAttr(asns []uint32, asn4 bool) []byte {
	asnSize := 2
	if asn4 {
		asnSize = 4
	}
	// Segment: type(1) + count(1) + asns
	segLen := 2 + len(asns)*asnSize
	// Attr header: flags(1) + code(1) + len(1) = 3 (non-extended)
	attr := make([]byte, 3+segLen)
	attr[0] = 0x40 // Transitive
	attr[1] = 0x02 // AS_PATH
	attr[2] = byte(segLen)
	attr[3] = 2 // AS_SEQUENCE
	attr[4] = byte(len(asns))
	off := 5
	for _, asn := range asns {
		if asn4 {
			binary.BigEndian.PutUint32(attr[off:], asn)
			off += 4
		} else {
			binary.BigEndian.PutUint16(attr[off:], uint16(asn))
			off += 2
		}
	}
	return attr
}

// buildASSetAttr builds an AS_PATH attribute with a single AS_SET segment (2-byte ASNs).
func buildASSetAttr(asns []uint32) []byte {
	segLen := 2 + len(asns)*2
	attr := make([]byte, 3+segLen)
	attr[0] = 0x40 // Transitive
	attr[1] = 0x02 // AS_PATH
	attr[2] = byte(segLen)
	attr[3] = 1 // AS_SET
	attr[4] = byte(len(asns))
	off := 5
	for _, asn := range asns {
		binary.BigEndian.PutUint16(attr[off:], uint16(asn))
		off += 2
	}
	return attr
}

// buildOriginatorIDAttr builds an ORIGINATOR_ID attribute (type 9, 4 bytes).
func buildOriginatorIDAttr(id uint32) []byte {
	attr := make([]byte, 7)
	attr[0] = 0x80 // Optional
	attr[1] = 0x09 // ORIGINATOR_ID
	attr[2] = 4    // Length
	binary.BigEndian.PutUint32(attr[3:], id)
	return attr
}

// buildClusterListAttr builds a CLUSTER_LIST attribute (type 10, 4 bytes per ID).
func buildClusterListAttr(ids []uint32) []byte {
	dataLen := len(ids) * 4
	attr := make([]byte, 3+dataLen)
	attr[0] = 0x80 // Optional
	attr[1] = 0x0A // CLUSTER_LIST
	attr[2] = byte(dataLen)
	for i, id := range ids {
		binary.BigEndian.PutUint32(attr[3+i*4:], id)
	}
	return attr
}

// --- AS Loop Detection ---

// TestDetectASLoop verifies local ASN in AS_SEQUENCE is detected.
// VALIDATES: AC-1, AC-2 — route with local ASN in AS_PATH treated as withdrawn.
func TestDetectASLoop(t *testing.T) {
	s := newIBGPSession() // LocalAS = 65001, no negotiated caps (asn4=false)
	pathAttrs := buildASPathAttr([]uint32{65002, 65001, 65003}, false)
	body := makeUpdateBody(nil, pathAttrs, nil)
	assert.True(t, s.detectLoops(body), "should detect local ASN 65001 in AS_PATH")
}

// TestDetectASLoop_ASSet verifies local ASN in AS_SET is detected.
// VALIDATES: AC-10 — AS_SET members count for loop detection.
func TestDetectASLoop_ASSet(t *testing.T) {
	s := newIBGPSession()
	pathAttrs := buildASSetAttr([]uint32{65002, 65001})
	body := makeUpdateBody(nil, pathAttrs, nil)
	assert.True(t, s.detectLoops(body), "should detect local ASN 65001 in AS_SET")
}

// TestDetectASLoop_NotPresent verifies no false positive when local ASN absent.
// VALIDATES: AC-3 — route without local ASN accepted normally.
func TestDetectASLoop_NotPresent(t *testing.T) {
	s := newIBGPSession()
	pathAttrs := buildASPathAttr([]uint32{65002, 65003, 65004}, false)
	body := makeUpdateBody(nil, pathAttrs, nil)
	assert.False(t, s.detectLoops(body), "should not detect loop when local ASN absent")
}

// TestDetectASLoop_EmptyPath verifies empty AS_PATH does not trigger.
func TestDetectASLoop_EmptyPath(t *testing.T) {
	s := newIBGPSession()
	// Empty AS_PATH: just the attr header with no segments.
	pathAttrs := []byte{0x40, 0x02, 0x00} // flags + code + len=0
	body := makeUpdateBody(nil, pathAttrs, nil)
	assert.False(t, s.detectLoops(body), "empty AS_PATH should not trigger loop")
}

// --- Originator-ID Loop Detection ---

// TestDetectOriginatorIDLoop verifies matching ORIGINATOR_ID is detected.
// VALIDATES: AC-4 — iBGP UPDATE with ORIGINATOR_ID matching local Router ID treated as withdrawn.
func TestDetectOriginatorIDLoop(t *testing.T) {
	s := newIBGPSession() // RouterID = 0x01020301
	pathAttrs := buildOriginatorIDAttr(0x01020301)
	body := makeUpdateBody(nil, pathAttrs, nil)
	assert.True(t, s.detectLoops(body), "should detect ORIGINATOR_ID matching Router ID")
}

// TestDetectOriginatorIDLoop_Different verifies non-matching ORIGINATOR_ID passes.
// VALIDATES: AC-5 — different ORIGINATOR_ID accepted normally.
func TestDetectOriginatorIDLoop_Different(t *testing.T) {
	s := newIBGPSession()
	pathAttrs := buildOriginatorIDAttr(0x0A000001) // 10.0.0.1 != 1.2.3.1
	body := makeUpdateBody(nil, pathAttrs, nil)
	assert.False(t, s.detectLoops(body), "different ORIGINATOR_ID should pass")
}

// TestDetectOriginatorIDLoop_Absent verifies missing ORIGINATOR_ID passes.
func TestDetectOriginatorIDLoop_Absent(t *testing.T) {
	s := newIBGPSession()
	// Only AS_PATH, no ORIGINATOR_ID.
	pathAttrs := buildASPathAttr([]uint32{65002}, true)
	body := makeUpdateBody(nil, pathAttrs, nil)
	assert.False(t, s.detectLoops(body), "missing ORIGINATOR_ID should pass")
}

// TestDetectOriginatorIDLoop_eBGP verifies eBGP session skips originator-ID check.
// VALIDATES: AC-6 — eBGP with ORIGINATOR_ID accepted.
func TestDetectOriginatorIDLoop_eBGP(t *testing.T) {
	s := newValidateSession() // eBGP: LocalAS=65001, PeerAS=65002
	// Same ORIGINATOR_ID as Router ID, but eBGP should skip.
	pathAttrs := buildOriginatorIDAttr(0x01020301)
	body := makeUpdateBody(nil, pathAttrs, nil)
	assert.False(t, s.detectLoops(body), "eBGP should skip ORIGINATOR_ID check")
}

// --- Cluster-List Loop Detection ---

// TestDetectClusterListLoop verifies local Router ID in CLUSTER_LIST is detected.
// VALIDATES: AC-7 — iBGP UPDATE with local Router ID in CLUSTER_LIST treated as withdrawn.
func TestDetectClusterListLoop(t *testing.T) {
	s := newIBGPSession() // RouterID = 0x01020301
	pathAttrs := buildClusterListAttr([]uint32{0x0A000001, 0x01020301, 0x0A000002})
	body := makeUpdateBody(nil, pathAttrs, nil)
	assert.True(t, s.detectLoops(body), "should detect local Router ID in CLUSTER_LIST")
}

// TestDetectClusterListLoop_NotPresent verifies CLUSTER_LIST without local ID passes.
// VALIDATES: AC-8 — CLUSTER_LIST not containing local Router ID accepted.
func TestDetectClusterListLoop_NotPresent(t *testing.T) {
	s := newIBGPSession()
	pathAttrs := buildClusterListAttr([]uint32{0x0A000001, 0x0A000002})
	body := makeUpdateBody(nil, pathAttrs, nil)
	assert.False(t, s.detectLoops(body), "CLUSTER_LIST without local ID should pass")
}

// TestDetectClusterListLoop_Absent verifies missing CLUSTER_LIST passes.
func TestDetectClusterListLoop_Absent(t *testing.T) {
	s := newIBGPSession()
	pathAttrs := buildASPathAttr([]uint32{65002}, true)
	body := makeUpdateBody(nil, pathAttrs, nil)
	assert.False(t, s.detectLoops(body), "missing CLUSTER_LIST should pass")
}

// TestDetectClusterListLoop_eBGP verifies eBGP session skips cluster-list check.
// VALIDATES: AC-9 — eBGP with CLUSTER_LIST accepted.
func TestDetectClusterListLoop_eBGP(t *testing.T) {
	s := newValidateSession() // eBGP
	pathAttrs := buildClusterListAttr([]uint32{0x01020301})
	body := makeUpdateBody(nil, pathAttrs, nil)
	assert.False(t, s.detectLoops(body), "eBGP should skip CLUSTER_LIST check")
}

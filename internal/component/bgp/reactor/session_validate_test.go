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

// TestEnforceRFC7606_ShortBody verifies body < 4 bytes triggers treat-as-withdraw.
// Truncated section headers must not reach plugins via callback dispatch.
func TestEnforceRFC7606_ShortBody(t *testing.T) {
	s := newValidateSession()

	// Only 2 bytes, too short for withdrawn length + attrs length.
	wu := wireu.NewWireUpdate([]byte{0x00, 0x00}, 0)

	newWU, action, err := s.enforceRFC7606(wu)
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionTreatAsWithdraw, action)
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

// Loop detection tests: moved to internal/component/bgp/reactor/filter/loop_test.go.
// The detectLoops session method was refactored to filter.LoopIngress (ingress filter plugin).
// All 12 tests preserved with identical coverage in the new location.

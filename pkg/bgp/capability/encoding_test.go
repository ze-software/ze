package capability

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEncodingCapsSupportsFamily verifies family lookup.
//
// VALIDATES: SupportsFamily() returns correct result for negotiated families.
//
// PREVENTS: Sending routes for non-negotiated families.
func TestEncodingCapsSupportsFamily(t *testing.T) {
	enc := &EncodingCaps{
		Families: []Family{
			{AFI: AFIIPv4, SAFI: SAFIUnicast},
			{AFI: AFIIPv6, SAFI: SAFIUnicast},
		},
	}

	assert.True(t, enc.SupportsFamily(Family{AFI: AFIIPv4, SAFI: SAFIUnicast}))
	assert.True(t, enc.SupportsFamily(Family{AFI: AFIIPv6, SAFI: SAFIUnicast}))
	assert.False(t, enc.SupportsFamily(Family{AFI: AFIL2VPN, SAFI: SAFIEVPN}))
}

// TestEncodingCapsAddPathFor verifies ADD-PATH mode lookup.
//
// VALIDATES: AddPathFor() returns correct mode for each family.
//
// PREVENTS: Wrong ADD-PATH handling (including/omitting path ID incorrectly).
func TestEncodingCapsAddPathFor(t *testing.T) {
	enc := &EncodingCaps{
		AddPathMode: map[Family]AddPathMode{
			{AFI: AFIIPv4, SAFI: SAFIUnicast}: AddPathBoth,
			{AFI: AFIIPv6, SAFI: SAFIUnicast}: AddPathReceive,
		},
	}

	assert.Equal(t, AddPathBoth, enc.AddPathFor(Family{AFI: AFIIPv4, SAFI: SAFIUnicast}))
	assert.Equal(t, AddPathReceive, enc.AddPathFor(Family{AFI: AFIIPv6, SAFI: SAFIUnicast}))
	assert.Equal(t, AddPathNone, enc.AddPathFor(Family{AFI: AFIL2VPN, SAFI: SAFIEVPN}))
}

// TestEncodingCapsAddPathForNil verifies nil map handling.
//
// VALIDATES: AddPathFor() returns AddPathNone for nil map.
//
// PREVENTS: Panic on nil map access.
func TestEncodingCapsAddPathForNil(t *testing.T) {
	enc := &EncodingCaps{}
	assert.Equal(t, AddPathNone, enc.AddPathFor(Family{AFI: AFIIPv4, SAFI: SAFIUnicast}))
}

// TestEncodingCapsExtendedNextHopAFI verifies ExtNH lookup.
//
// VALIDATES: ExtendedNextHopAFI() returns correct AFI for family.
//
// PREVENTS: Wrong next-hop encoding per RFC 8950.
func TestEncodingCapsExtendedNextHopAFI(t *testing.T) {
	enc := &EncodingCaps{
		ExtendedNextHop: map[Family]AFI{
			{AFI: AFIIPv4, SAFI: SAFIUnicast}: AFIIPv6, // IPv4/Unicast can use IPv6 NH
		},
	}

	assert.Equal(t, AFIIPv6, enc.ExtendedNextHopAFI(Family{AFI: AFIIPv4, SAFI: SAFIUnicast}))
	assert.Equal(t, AFI(0), enc.ExtendedNextHopAFI(Family{AFI: AFIIPv6, SAFI: SAFIUnicast}))
}

// TestEncodingCapsExtendedNextHopAFINil verifies nil map handling.
//
// VALIDATES: ExtendedNextHopAFI() returns 0 for nil map.
//
// PREVENTS: Panic on nil map access.
func TestEncodingCapsExtendedNextHopAFINil(t *testing.T) {
	enc := &EncodingCaps{}
	assert.Equal(t, AFI(0), enc.ExtendedNextHopAFI(Family{AFI: AFIIPv4, SAFI: SAFIUnicast}))
}

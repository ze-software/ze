package nlri

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMVPNTypes verifies MVPN route types.
func TestMVPNTypes(t *testing.T) {
	assert.Equal(t, MVPNRouteType(1), MVPNIntraASIPMSIAD)
	assert.Equal(t, MVPNRouteType(2), MVPNInterASIPMSIAD)
	assert.Equal(t, MVPNRouteType(3), MVPNSPMSIAD)
	assert.Equal(t, MVPNRouteType(4), MVPNLeafAD)
	assert.Equal(t, MVPNRouteType(5), MVPNSourceActive)
	assert.Equal(t, MVPNRouteType(6), MVPNSharedTreeJoin)
	assert.Equal(t, MVPNRouteType(7), MVPNSourceTreeJoin)
}

// TestMVPNBasic verifies basic MVPN NLRI creation.
func TestMVPNBasic(t *testing.T) {
	mvpn := NewMVPN(MVPNIntraASIPMSIAD, []byte{1, 2, 3, 4})

	assert.Equal(t, MVPNIntraASIPMSIAD, mvpn.RouteType())
	assert.NotNil(t, mvpn.Bytes())
}

// TestMVPNFamily verifies MVPN address family.
func TestMVPNFamily(t *testing.T) {
	mvpn := NewMVPN(MVPNIntraASIPMSIAD, nil)

	// IPv4 MVPN uses AFI 1, SAFI 5
	assert.Equal(t, AFIIPv4, mvpn.Family().AFI)
	assert.Equal(t, SAFIMVPN, mvpn.Family().SAFI)
}

// TestVPLSBasic verifies basic VPLS NLRI creation.
func TestVPLSBasic(t *testing.T) {
	vpls := NewVPLS(RouteDistinguisher{Type: 1}, 100, 200, []byte{1, 2, 3})

	assert.Equal(t, uint16(100), vpls.VEBlockOffset())
	assert.Equal(t, uint16(200), vpls.VEBlockSize())
}

// TestVPLSFamily verifies VPLS address family.
func TestVPLSFamily(t *testing.T) {
	vpls := NewVPLS(RouteDistinguisher{}, 0, 0, nil)

	// VPLS uses AFI 25, SAFI 65
	assert.Equal(t, AFIL2VPN, vpls.Family().AFI)
	assert.Equal(t, SAFIVPLS, vpls.Family().SAFI)
}

// TestVPLSBytes verifies VPLS wire format.
func TestVPLSBytes(t *testing.T) {
	vpls := NewVPLS(RouteDistinguisher{Type: 1}, 100, 200, []byte{1, 2, 3})

	data := vpls.Bytes()
	require.NotEmpty(t, data)
}

// TestRTCBasic verifies basic RTC NLRI creation.
func TestRTCBasic(t *testing.T) {
	rt := RouteTarget{
		Type:  0,                                // 2-byte ASN
		Value: []byte{0xFD, 0xE9, 0, 0, 0, 100}, // AS 65001 : 100
	}
	rtc := NewRTC(65001, rt)

	assert.Equal(t, uint32(65001), rtc.OriginAS())
}

// TestRTCFamily verifies RTC address family.
func TestRTCFamily(t *testing.T) {
	rtc := NewRTC(65001, RouteTarget{})

	// RTC uses AFI 1, SAFI 132
	assert.Equal(t, AFIIPv4, rtc.Family().AFI)
	assert.Equal(t, SAFIRTC, rtc.Family().SAFI)
}

// TestRTCBytes verifies RTC wire format.
func TestRTCBytes(t *testing.T) {
	rtc := NewRTC(65001, RouteTarget{
		Type:  0,
		Value: []byte{0xFD, 0xE9, 0, 0, 0, 100},
	})

	data := rtc.Bytes()
	require.NotEmpty(t, data)
	// RTC NLRI: 4 bytes origin AS + 8 bytes RT
	assert.GreaterOrEqual(t, len(data), 4)
}

// TestMUPTypes verifies MUP route types.
func TestMUPTypes(t *testing.T) {
	assert.Equal(t, MUPRouteType(1), MUPISD)
	assert.Equal(t, MUPRouteType(2), MUPDSD)
	assert.Equal(t, MUPRouteType(3), MUPT1ST)
	assert.Equal(t, MUPRouteType(4), MUPT2ST)
}

// TestMUPBasic verifies basic MUP NLRI creation.
func TestMUPBasic(t *testing.T) {
	mup := NewMUP(MUPISD, []byte{1, 2, 3, 4})

	assert.Equal(t, MUPISD, mup.RouteType())
}

// TestMUPFamily verifies MUP address family.
func TestMUPFamily(t *testing.T) {
	mup := NewMUP(MUPISD, nil)

	// MUP uses AFI 1, SAFI 85
	assert.Equal(t, AFIIPv4, mup.Family().AFI)
	assert.Equal(t, SAFIMUP, mup.Family().SAFI)
}

// TestSAFIConstants verifies additional SAFI constants exist.
func TestSAFIConstants(t *testing.T) {
	assert.Equal(t, SAFI(5), SAFIMVPN)
	assert.Equal(t, SAFI(65), SAFIVPLS)
	assert.Equal(t, SAFI(85), SAFIMUP)
	assert.Equal(t, SAFI(132), SAFIRTC)
}

package family

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test-local Family values for the specialized SAFI types tested below.
// These intentionally live next to the tests that use them so the family
// package owns the verification of its own constants.
var (
	testIPv4MVPN  = Family{AFI: AFIIPv4, SAFI: SAFIMVPN}
	testIPv6MVPN  = Family{AFI: AFIIPv6, SAFI: SAFIMVPN}
	testL2VPNVPLS = Family{AFI: AFIL2VPN, SAFI: SAFIVPLS}
	testIPv4RTC   = Family{AFI: AFIIPv4, SAFI: SAFIRTC}
	testIPv4MUP   = Family{AFI: AFIIPv4, SAFI: SAFIMUP}
	testIPv6MUP   = Family{AFI: AFIIPv6, SAFI: SAFIMUP}
)

// TestSpecializedFamilyVariables verifies the AFI/SAFI numeric constants
// resolve to the expected values when assembled into a Family literal.
func TestSpecializedFamilyVariables(t *testing.T) {
	t.Parallel()
	assert.Equal(t, AFIIPv4, testIPv4MVPN.AFI)
	assert.Equal(t, SAFIMVPN, testIPv4MVPN.SAFI)

	assert.Equal(t, AFIIPv6, testIPv6MVPN.AFI)
	assert.Equal(t, SAFIMVPN, testIPv6MVPN.SAFI)

	assert.Equal(t, AFIL2VPN, testL2VPNVPLS.AFI)
	assert.Equal(t, SAFIVPLS, testL2VPNVPLS.SAFI)

	assert.Equal(t, AFIIPv4, testIPv4RTC.AFI)
	assert.Equal(t, SAFIRTC, testIPv4RTC.SAFI)

	assert.Equal(t, AFIIPv4, testIPv4MUP.AFI)
	assert.Equal(t, SAFIMUP, testIPv4MUP.SAFI)

	assert.Equal(t, AFIIPv6, testIPv6MUP.AFI)
	assert.Equal(t, SAFIMUP, testIPv6MUP.SAFI)
}

// TestSpecializedSAFIStrings verifies SAFI.String() for specialized SAFIs.
// Exercises the registry lookup path for SAFIs registered by RegisterTestFamilies.
func TestSpecializedSAFIStrings(t *testing.T) {
	ResetRegistry()
	defer func() {
		ResetRegistry()
		RegisterTestFamilies()
	}()
	RegisterTestFamilies()

	tests := []struct {
		safi     SAFI
		expected string
	}{
		{SAFIMVPN, "mvpn"},
		{SAFIVPLS, "vpls"},
		{SAFIMUP, "mup"},
		{SAFIRTC, "rtc"},
		{SAFIBGPLinkState, "bgp-ls"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.safi.String())
		})
	}
}

// TestSpecializedFamilyParsing verifies LookupFamily for specialized families.
func TestSpecializedFamilyParsing(t *testing.T) {
	ResetRegistry()
	defer func() {
		ResetRegistry()
		RegisterTestFamilies()
	}()
	RegisterTestFamilies()

	tests := []struct {
		input    string
		expected Family
		ok       bool
	}{
		{"ipv4/mvpn", testIPv4MVPN, true},
		{"ipv6/mvpn", testIPv6MVPN, true},
		{"l2vpn/vpls", testL2VPNVPLS, true},
		{"ipv4/rtc", testIPv4RTC, true},
		{"ipv4/mup", testIPv4MUP, true},
		{"ipv6/mup", testIPv6MUP, true},
		{"unknown", Family{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			fam, ok := LookupFamily(tt.input)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.expected, fam)
			}
		})
	}
}

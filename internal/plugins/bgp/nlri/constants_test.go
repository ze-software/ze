package nlri

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSAFIConstants verifies additional SAFI constants exist.
func TestSAFIConstants(t *testing.T) {
	assert.Equal(t, SAFI(5), SAFIMVPN)
	assert.Equal(t, SAFI(65), SAFIVPLS)
	assert.Equal(t, SAFI(85), SAFIMUP)
	assert.Equal(t, SAFI(132), SAFIRTC)
}

// TestFamilyVariables verifies family variables.
func TestFamilyVariables(t *testing.T) {
	assert.Equal(t, AFIIPv4, IPv4MVPN.AFI)
	assert.Equal(t, SAFIMVPN, IPv4MVPN.SAFI)

	assert.Equal(t, AFIIPv6, IPv6MVPN.AFI)
	assert.Equal(t, SAFIMVPN, IPv6MVPN.SAFI)

	assert.Equal(t, AFIL2VPN, L2VPNVPLS.AFI)
	assert.Equal(t, SAFIVPLS, L2VPNVPLS.SAFI)

	assert.Equal(t, AFIIPv4, IPv4RTC.AFI)
	assert.Equal(t, SAFIRTC, IPv4RTC.SAFI)

	assert.Equal(t, AFIIPv4, IPv4MUP.AFI)
	assert.Equal(t, SAFIMUP, IPv4MUP.SAFI)

	assert.Equal(t, AFIIPv6, IPv6MUP.AFI)
	assert.Equal(t, SAFIMUP, IPv6MUP.SAFI)
}

// TestSAFIStrings verifies SAFI String() method for specialized types.
func TestSAFIStrings(t *testing.T) {
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

// TestFamilyParsing verifies family string parsing for specialized types.
func TestFamilyParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected Family
		ok       bool
	}{
		{"ipv4/mvpn", IPv4MVPN, true},
		{"ipv6/mvpn", IPv6MVPN, true},
		{"l2vpn/vpls", L2VPNVPLS, true},
		{"ipv4/rtc", IPv4RTC, true},
		{"ipv4/mup", IPv4MUP, true},
		{"ipv6/mup", IPv6MUP, true},
		{"unknown", Family{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			family, ok := ParseFamily(tt.input)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.expected, family)
			}
		})
	}
}

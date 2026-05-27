package scenario

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFRRParams() ConfigParams {
	return ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: "127.0.0.1",
		BasePort:  1179,
		Profiles: []PeerProfile{
			{
				Index: 0, ASN: 65001, Address: netip.MustParseAddr("127.0.0.2"),
				RouterID: netip.MustParseAddr("10.255.0.1"), HoldTime: 90, RouteCount: 100,
				Families: []string{"ipv4/unicast", "ipv6/unicast"},
			},
			{
				Index: 1, ASN: 65000, Address: netip.MustParseAddr("127.0.0.3"),
				RouterID: netip.MustParseAddr("10.255.0.2"), IsIBGP: true, HoldTime: 90, RouteCount: 100,
				Families: []string{"ipv4/unicast"},
			},
		},
	}
}

func TestFRRConfigStructure(t *testing.T) {
	config := GenerateFRRConfig(testFRRParams())

	assert.Contains(t, config, "frr defaults traditional")
	assert.Contains(t, config, "router bgp 65000")
	assert.Contains(t, config, "bgp router-id 10.0.0.1")
	assert.Contains(t, config, "no bgp ebgp-requires-policy")
}

func TestFRRConfigNeighbors(t *testing.T) {
	config := GenerateFRRConfig(testFRRParams())

	assert.Contains(t, config, "neighbor 127.0.0.2 remote-as 65001")
	assert.Contains(t, config, "neighbor 127.0.0.3 remote-as 65000")
	assert.Contains(t, config, "neighbor 127.0.0.2 description chaos-peer-0")
	assert.Contains(t, config, "neighbor 127.0.0.3 description chaos-peer-1")
	assert.Equal(t, 2, strings.Count(config, "passive"))
}

func TestFRRConfigTimers(t *testing.T) {
	config := GenerateFRRConfig(testFRRParams())

	assert.Contains(t, config, "neighbor 127.0.0.2 timers 30 90")
	assert.Contains(t, config, "neighbor 127.0.0.3 timers 30 90")
}

func TestFRRConfigAddressFamilies(t *testing.T) {
	config := GenerateFRRConfig(testFRRParams())

	assert.Contains(t, config, "address-family ipv4 unicast")
	assert.Contains(t, config, "address-family ipv6 unicast")

	// Both peers activated for ipv4 unicast.
	ipv4Section := extractFRRAddressFamily(config, "ipv4 unicast")
	require.NotEmpty(t, ipv4Section, "ipv4 unicast section should exist")
	assert.Equal(t, 2, strings.Count(ipv4Section, "activate"))
	assert.Equal(t, 2, strings.Count(ipv4Section, "route-server-client"))

	// Only peer 0 activated for ipv6 unicast.
	ipv6Section := extractFRRAddressFamily(config, "ipv6 unicast")
	require.NotEmpty(t, ipv6Section, "ipv6 unicast section should exist")
	assert.Equal(t, 1, strings.Count(ipv6Section, "activate"))
	assert.Contains(t, ipv6Section, "127.0.0.2")
	assert.NotContains(t, ipv6Section, "127.0.0.3")
}

func TestFRRConfigDeterministic(t *testing.T) {
	params := testFRRParams()
	assert.Equal(t, GenerateFRRConfig(params), GenerateFRRConfig(params))
}

func TestFRRConfigMultiplePeers(t *testing.T) {
	params := testFRRParams()
	profiles := make([]PeerProfile, 10)
	for i := range profiles {
		profiles[i] = PeerProfile{
			Index: i, ASN: uint32(65001 + i), Address: netip.AddrFrom4([4]byte{127, 0, 0, byte(2 + i)}),
			RouterID: netip.MustParseAddr("10.255.0.1"), HoldTime: 90, RouteCount: 100,
			Families: []string{"ipv4/unicast"},
		}
	}
	params.Profiles = profiles

	config := GenerateFRRConfig(params)
	assert.Equal(t, 10, strings.Count(config, "remote-as"))
	assert.Equal(t, 10, strings.Count(config, "passive"))
}

func TestFRRConfigUnsupportedFamilySkipped(t *testing.T) {
	params := testFRRParams()
	params.Profiles[0].Families = []string{"ipv4/unicast", "l2vpn/evpn"}

	config := GenerateFRRConfig(params)

	assert.Contains(t, config, "address-family ipv4 unicast")
	assert.NotContains(t, config, "l2vpn")
}

func TestFRRConfigEmptyFamiliesDefaultsToIPv4(t *testing.T) {
	params := testFRRParams()
	params.Profiles[0].Families = nil
	params.Profiles[1].Families = nil

	config := GenerateFRRConfig(params)

	assert.Contains(t, config, "address-family ipv4 unicast")
	ipv4Section := extractFRRAddressFamily(config, "ipv4 unicast")
	require.NotEmpty(t, ipv4Section)
	assert.Equal(t, 2, strings.Count(ipv4Section, "activate"))
}

func extractFRRAddressFamily(config, family string) string {
	marker := "address-family " + family
	start := strings.Index(config, marker)
	if start < 0 {
		return ""
	}
	end := strings.Index(config[start:], "exit-address-family")
	if end < 0 {
		return config[start:]
	}
	return config[start : start+end+len("exit-address-family")]
}

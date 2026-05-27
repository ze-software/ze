package scenario

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testBIRDParams() ConfigParams {
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

func TestBIRDConfigStructure(t *testing.T) {
	config := GenerateBIRDConfig(testBIRDParams())

	assert.Contains(t, config, "router id 10.0.0.1;")
	assert.Contains(t, config, "listen bgp address 127.0.0.1 port 1179;")
	assert.Contains(t, config, "protocol device {}")
	assert.Contains(t, config, "log stderr all;")
}

func TestBIRDConfigPeers(t *testing.T) {
	config := GenerateBIRDConfig(testBIRDParams())

	assert.Contains(t, config, "protocol bgp chaos_peer_0")
	assert.Contains(t, config, "protocol bgp chaos_peer_1")
	assert.Contains(t, config, "neighbor 127.0.0.2 as 65001;")
	assert.Contains(t, config, "neighbor 127.0.0.3 as 65000;")
	assert.Contains(t, config, "local as 65000;")
	assert.Equal(t, 2, strings.Count(config, "passive;"))
	assert.Equal(t, 2, strings.Count(config, "rs client;"))
}

func TestBIRDConfigHoldTime(t *testing.T) {
	config := GenerateBIRDConfig(testBIRDParams())
	assert.Equal(t, 2, strings.Count(config, "hold time 90;"))
}

func TestBIRDConfigChannels(t *testing.T) {
	config := GenerateBIRDConfig(testBIRDParams())

	// Peer 0 has ipv4 + ipv6 channels.
	peer0 := extractBIRDProtocol(config, "chaos_peer_0")
	require.NotEmpty(t, peer0)
	assert.Contains(t, peer0, "ipv4 {")
	assert.Contains(t, peer0, "ipv6 {")

	// Peer 1 has only ipv4.
	peer1 := extractBIRDProtocol(config, "chaos_peer_1")
	require.NotEmpty(t, peer1)
	assert.Contains(t, peer1, "ipv4 {")
	assert.NotContains(t, peer1, "ipv6 {")
}

func TestBIRDConfigDeterministic(t *testing.T) {
	params := testBIRDParams()
	assert.Equal(t, GenerateBIRDConfig(params), GenerateBIRDConfig(params))
}

func TestBIRDConfigMultiplePeers(t *testing.T) {
	params := testBIRDParams()
	profiles := make([]PeerProfile, 10)
	for i := range profiles {
		profiles[i] = PeerProfile{
			Index: i, ASN: uint32(65001 + i), Address: netip.AddrFrom4([4]byte{127, 0, 0, byte(2 + i)}),
			RouterID: netip.MustParseAddr("10.255.0.1"), HoldTime: 90, RouteCount: 100,
			Families: []string{"ipv4/unicast"},
		}
	}
	params.Profiles = profiles

	config := GenerateBIRDConfig(params)
	assert.Equal(t, 10, strings.Count(config, "protocol bgp chaos_peer_"))
	assert.Equal(t, 10, strings.Count(config, "passive;"))
}

func TestBIRDConfigUnsupportedFamilySkipped(t *testing.T) {
	params := testBIRDParams()
	params.Profiles[0].Families = []string{"ipv4/unicast", "l2vpn/evpn", "ipv4/multicast", "ipv6/multicast"}

	config := GenerateBIRDConfig(params)

	peer0 := extractBIRDProtocol(config, "chaos_peer_0")
	assert.Contains(t, peer0, "ipv4 {")
	assert.NotContains(t, peer0, "l2vpn")
	assert.NotContains(t, peer0, "multicast")
}

func TestBIRDConfigPortEmbedded(t *testing.T) {
	params := testBIRDParams()
	params.BasePort = 2179

	config := GenerateBIRDConfig(params)
	assert.Contains(t, config, "listen bgp address 127.0.0.1 port 2179;")
}

func extractBIRDProtocol(config, name string) string {
	marker := "protocol bgp " + name
	start := strings.Index(config, marker)
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(config); i++ {
		switch config[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return config[start : i+1]
			}
		}
	}
	return config[start:]
}

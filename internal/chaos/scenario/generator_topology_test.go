package scenario

import (
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

// TestGenerateFromTopologyProfiles turns the hand-written network into peer
// profiles and compares them with its speakers.
//
// VALIDATES: one profile per speaker, in the order of the speakers: the four
// routers as iBGP peers of the local AS, then the two transits as eBGP peers of
// their own AS. Each profile's address and router id are the speaker's address,
// and its route set is the speaker's announcements, with the route count as
// their number.
// PREVENTS: a simulated speaker known to the collector by another address than
// the one the model writes as next hop, which would break the match between a
// next hop and a router.
func TestGenerateFromTopologyProfiles(t *testing.T) {
	topology, err := NewTopology(handTopologySpec())
	require.NoError(t, err)

	profiles, err := GenerateFromTopology(topology)
	require.NoError(t, err)

	// The identities, written out: the model is not asked for them.
	type identity struct {
		address netip.Addr
		asn     uint32
		ibgp    bool
	}
	want := []identity{
		{handBorder1, handLocalAS, true},
		{handBorder2, handLocalAS, true},
		{handReflector, handLocalAS, true},
		{handClient, handLocalAS, true},
		{handTransitA, 65101, false},
		{handTransitB, 65102, false},
	}
	require.Len(t, profiles, len(want))
	speakers := topology.Speakers()
	require.Len(t, speakers, len(want))

	for index, profile := range profiles {
		assert.Equal(t, index, profile.Index)
		assert.Equal(t, want[index].address, profile.Address, "profile %d", index)
		assert.Equal(t, want[index].address, profile.RouterID, "profile %d", index)
		assert.Equal(t, want[index].asn, profile.ASN, "profile %d", index)
		assert.Equal(t, want[index].ibgp, profile.IsIBGP, "profile %d", index)
		assert.Equal(t, ModeActive, profile.Mode, "profile %d dials the daemon", index)
		assert.Equal(t, uint16(90), profile.HoldTime, "profile %d", index)
		assert.Equal(t, []string{familyIPv4Unicast}, profile.Families, "profile %d", index)
		assert.Zero(t, profile.ZePort, "profile %d: the caller chooses the port", index)

		require.NotNil(t, profile.RouteSet, "profile %d", index)
		assert.Equal(t, speakers[index].Announcements, profile.RouteSet.Announcements, "profile %d", index)
		assert.Equal(t, len(speakers[index].Announcements), profile.RouteCount, "profile %d", index)
		assert.NotEmpty(t, profile.RouteSet.Announcements, "profile %d: every speaker of this network announces a route", index)
	}

	// The second transit announces both prefixes with its own address as next
	// hop and its own AS first: the route set carries the model's routes whole.
	transitB := profiles[5].RouteSet
	assert.Equal(t, []netip.Prefix{handTwoExits, handOneExit}, transitB.Prefixes(), "ordered by prefix")
	for _, announcement := range transitB.Announcements {
		assert.Equal(t, handTransitB, announcement.NextHop)
		require.NotEmpty(t, announcement.ASPath)
		assert.Equal(t, uint32(65102), announcement.ASPath[0])
	}

	// A profile holds its own copy: changing it leaves the model as it was.
	profiles[5].RouteSet.Announcements[0].NextHop = handBorder1
	assert.Equal(t, handTransitB, topology.Speakers()[5].Announcements[0].NextHop)
}

// TestGenerateFromTopologyRefusesUnspecifiedSession gives the conversion a
// speaker whose session kind was never set.
//
// VALIDATES: the conversion answers an error naming the speaker.
// PREVENTS: a speaker of unknown kind becoming an eBGP profile because false is
// the zero value of IsIBGP.
func TestGenerateFromTopologyRefusesUnspecifiedSession(t *testing.T) {
	topology := &Topology{speakers: []Speaker{{Address: handBorder1, AS: handLocalAS}}}

	_, err := GenerateFromTopology(topology)
	require.Error(t, err)
	assert.Contains(t, err.Error(), handBorder1.String())
	assert.Contains(t, err.Error(), "neither internal nor external")
}

// peerBlock returns the body of the peer block chaos-peer-<index> of config:
// the text after its opening line, up to the next peer block or the end.
func peerBlock(t *testing.T, config string, index int) string {
	t.Helper()
	_, rest, found := strings.Cut(config, fmt.Sprintf("    peer chaos-peer-%d {\n", index))
	require.True(t, found, "peer %d", index)
	// With no later peer block, Cut answers the whole rest.
	body, _, _ := strings.Cut(rest, "    peer chaos-peer-")
	return body
}

// TestConfigGenCollectorForTopology writes the daemon config of a generated
// network and checks both sides come from the one model.
//
// VALIDATES: a collector config runs bgp-rib inside the daemon and no route
// server, every speaker of the model is a peer at its own address with its own
// AS and its own listen port, each attaching bgp-rib alone, and the looking
// glass is served over plain HTTP when asked. The text parses against the schema.
// PREVENTS: a speaker of the model with no peer in the daemon's config, whose
// session the daemon would refuse.
func TestConfigGenCollectorForTopology(t *testing.T) {
	topology, err := GenerateTopology(generatedTopologyParams(7))
	require.NoError(t, err)
	profiles, err := GenerateFromTopology(topology)
	require.NoError(t, err)
	require.Len(t, profiles, 11, "five routers and six external neighbors")
	const portFirst = 1790
	for index := range profiles {
		profiles[index].ZePort = portFirst + index
	}

	params := ConfigParams{
		LocalAS:     topology.LocalAS(),
		RouterID:    netip.MustParseAddr("192.0.2.254"),
		LocalAddr:   "127.0.0.1",
		Profiles:    profiles,
		Collector:   true,
		LGPort:      8080,
		LGPlaintext: true,
	}
	config := GenerateConfig(params)

	assert.Contains(t, config, "internal bgp-rib {\n        use bgp-rib;\n    }")
	assert.NotContains(t, config, "bgp-rs")
	assert.NotContains(t, config, "external")
	assert.Equal(t, len(profiles), strings.Count(config, "attach process bgp-rib {"))
	assert.Contains(t, config, "looking-glass {\n        enabled true;\n        tls false;\n")

	for index, speaker := range topology.Speakers() {
		assert.Contains(t, config, fmt.Sprintf("ip %s;\n                connect false;", speaker.Address), "speaker %d", index)
		assert.Contains(t, config, fmt.Sprintf("remote %d;\n                local %d;", speaker.AS, topology.LocalAS()), "speaker %d", index)
		assert.Contains(t, config, fmt.Sprintf("port %d;", portFirst+index), "speaker %d", index)
	}

	// Each of the six external neighbors carries the import policy that makes
	// Ze keep its routes (RFC 8212), the five routers carry none, and no peer
	// carries an export policy: a collector announces nothing.
	const importPolicy = "        filter {\n            import {\n                final accept;\n            }\n        }\n"
	assert.Equal(t, 6, strings.Count(config, importPolicy))
	assert.Equal(t, 6, strings.Count(config, "filter {"))
	assert.NotContains(t, config, "export")
	for index, speaker := range topology.Speakers() {
		block := peerBlock(t, config, index)
		assert.Equal(t, speaker.Session == SessionKindExternal, strings.Contains(block, importPolicy), "speaker %d (%s)", index, speaker.Address)
	}

	// The config of a chaos run against a route server is left without one.
	params.Collector = false
	assert.NotContains(t, GenerateConfig(params), "filter {")
	params.Collector = true

	// Without the plaintext option the looking glass keeps its default.
	params.LGPlaintext = false
	assert.NotContains(t, GenerateConfig(params), "tls false")

	// The schema this test links holds no looking glass module, so the shape is
	// parsed without that block: plugins, peers and their attach blocks.
	params.LGPort = 0
	_, err = zeconfig.ParseTreeWithYANG(GenerateConfig(params), nil)
	require.NoError(t, err)
}

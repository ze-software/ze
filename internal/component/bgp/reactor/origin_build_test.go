// Design: docs/architecture/api/process-protocol.md -- engine-owned local origin data.
package reactor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

func originBuildFixture() originBuildInput {
	return originBuildInput{family: family.IPv4Unicast, native: []byte{24, 10, 0, 0},
		attributes: []byte{0x40, 1, 1, 0}, attributesExplicit: true, attributeContext: bgpctx.ContextID(17),
		nextHop: bgptypes.NewNextHopSelf(), hasPathID: true}
}

// The owned value preserves original build data and legal path ID zero rather
// than borrowing mutable API buffers or inventing a received-source reference.
func TestOriginBuildValueOwnsNativeAndAttributes(t *testing.T) {
	input := originBuildFixture()
	value, err := newOriginBuildValue(input)
	require.NoError(t, err)
	input.native[1], input.attributes[3] = 99, 2
	require.Equal(t, []byte{24, 10, 0, 0}, value.native)
	require.Equal(t, []byte{0x40, 1, 1, 0}, value.attributes)
	require.True(t, value.hasPathID)
	require.Zero(t, value.pathID)
	require.True(t, value.nextHop.IsSelf())
	require.Equal(t, bgpctx.ContextID(17), value.attributeContext, "unknown original context must not become assumed ASN4")
	for _, change := range []struct {
		name  string
		apply func(*originBuildInput)
	}{
		{"empty", func(i *originBuildInput) { i.native = nil }},
		{"malformed", func(i *originBuildInput) { i.native = []byte{33, 10} }},
		{"multiple", func(i *originBuildInput) { i.native = append(i.native, 24, 10, 1, 0) }},
		{"unknown-family", func(i *originBuildInput) { i.family = family.Family{} }},
		{"path-presence", func(i *originBuildInput) { i.pathID = 9; i.hasPathID = false }},
		{"attributes", func(i *originBuildInput) { i.attributes = []byte{0x40, 1, 4, 0} }},
		{"implicit-attributes", func(i *originBuildInput) { i.attributesExplicit = false }},
		{"next-hop", func(i *originBuildInput) { i.nextHop = bgptypes.RouteNextHop{} }},
	} {
		t.Run(change.name, func(t *testing.T) {
			bad := originBuildFixture()
			change.apply(&bad)
			_, err := newOriginBuildValue(bad)
			require.Error(t, err)
		})
	}
}

// Registered opaque native identities remain independent, even with one family.
func TestOriginBuildValueKeepsOpaqueIdentity(t *testing.T) {
	input := originBuildFixture()
	input.family = family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	input.native = make([]byte, 27)
	input.native[0], input.native[1] = 1, 25
	left, err := newOriginBuildValue(input)
	require.NoError(t, err)
	input.native = bytes.Clone(input.native)
	input.native[9] = 1
	right, err := newOriginBuildValue(input)
	require.NoError(t, err)
	require.NotEqual(t, left.key, right.key)
}

// The original protocol, accepted override (including zero) and metric survive
// without consulting process-global configured distance or BGP ORIGIN bytes.
func TestOriginSourcePreservesDeclaredProvenance(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	static := redistevents.RegisterProtocol("static")
	bgp := redistevents.RegisterProtocol("bgp")
	input := originSourceInput{kind: originSourceRedistributed, protocol: static, instance: 7, metric: 9, hasDistance: true}
	source, err := newOriginSource(input)
	require.NoError(t, err)
	require.Equal(t, static, source.identity.protocol)
	require.Equal(t, uint64(7), source.identity.instance)
	require.True(t, source.hasDistance)
	require.Zero(t, source.distance)
	require.Equal(t, uint32(9), source.metric)
	local, err := newOriginSource(originSourceInput{kind: originSourceLocalBGP})
	require.NoError(t, err)
	require.Equal(t, redistevents.ProtocolUnspecified, local.identity.protocol)
	_, err = newOriginSource(originSourceInput{})
	require.Error(t, err)
	_, err = newOriginSource(originSourceInput{kind: originSourceKind(255)})
	require.Error(t, err)
	_, err = newOriginSource(originSourceInput{kind: originSourceRedistributed})
	require.Error(t, err)
	_, err = newOriginSource(originSourceInput{kind: originSourceRedistributed, protocol: bgp})
	require.Error(t, err)
	_, err = newOriginSource(originSourceInput{kind: originSourceLocalBGP, protocol: static})
	require.Error(t, err)
}

// A family without a next hop permits explicit absence, not an unknown policy.
func TestOriginBuildWithoutNextHopKeepsPolicyValidation(t *testing.T) {
	input := originBuildFixture()
	input.family = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	input.native = []byte{5, 1, 24, 10, 0, 0}
	input.nextHop = bgptypes.RouteNextHop{}
	_, err := newOriginBuildValue(input)
	require.NoError(t, err)
	input.nextHop.Policy = bgptypes.NextHopPolicy(255)
	_, err = newOriginBuildValue(input)
	require.Error(t, err)
}

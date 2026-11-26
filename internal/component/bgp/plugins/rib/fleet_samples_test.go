// Design: docs/architecture/fleet-config.md -- BGP RIB fleet sample producer tests
// Related: fleet_samples.go -- inbound path count projection
// Related: ac17_forked_config_test.go -- forked runRIBPlugin SDK runtime harness

package rib

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func insertTestInboundRoute(t *testing.T, peerRIB *storage.PeerRIB, prefix string) {
	t.Helper()
	nlriBytes, err := prefixToWire("ipv4/unicast", prefix, 0, false)
	require.NoError(t, err)
	peerRIB.Insert(family.IPv4Unicast, []byte{0x40, 0x01, 0x01, 0x00}, nlriBytes)
}

// VALIDATES: show fleet source bgp-rib reports zero when the live RIB holds no
// routes and matches status()["routes-in"] across bgpPeers and ribInPool
// without double-counting routes.
// PREVENTS: routes-held drifting from RIBManager's inbound path counts or
// counting Adj-RIB-Out / best paths alongside Adj-RIB-In.
func TestFleetSourceBGPRIBBuiltinCounts(t *testing.T) {
	r := newTestRIBManager(t)

	// Empty live RIB reports a real zero.
	status, data, err := r.handleCommand(cmdFleetSourceBGPRIB, "", nil)
	require.NoError(t, err)
	require.Equal(t, statusDone, status)

	raw, err := json.Marshal(data)
	require.NoError(t, err)
	var snap fleet.SourceSnapshot
	require.NoError(t, json.Unmarshal(raw, &snap))
	require.NoError(t, fleet.ValidateSourceSnapshot(fleet.SampleOwnerBGPRIB, &snap))
	require.Equal(t, []fleet.SourceReading{
		{Figure: fleet.FigureRoutesHeld, Unit: fleet.SampleUnitCount, Value: 0},
	}, snap.Readings)

	// Populate two routes in bgpPeers and one injected route in ribInPool.
	peer1, err := storage.NewAddressPeerRIB(netip.MustParseAddr("192.0.2.1"))
	require.NoError(t, err)
	t.Cleanup(peer1.Release)
	insertTestInboundRoute(t, peer1, "198.51.100.0/24")
	insertTestInboundRoute(t, peer1, "198.51.101.0/24")

	peer2, err := storage.NewAddressPeerRIB(netip.MustParseAddr("192.0.2.2"))
	require.NoError(t, err)
	t.Cleanup(peer2.Release)
	insertTestInboundRoute(t, peer2, "203.0.113.0/24")

	r.peerMu.Lock()
	r.bgpPeers[1] = peer1
	r.ribInPool[bgpProtocolID] = map[string]*storage.PeerRIB{
		"192.0.2.2": peer2,
	}
	r.peerMu.Unlock()

	status, data, err = r.handleCommand(cmdFleetSourceBGPRIB, "", nil)
	require.NoError(t, err)
	require.Equal(t, statusDone, status)

	raw, err = json.Marshal(data)
	require.NoError(t, err)
	snap = fleet.SourceSnapshot{}
	require.NoError(t, json.Unmarshal(raw, &snap))
	require.NoError(t, fleet.ValidateSourceSnapshot(fleet.SampleOwnerBGPRIB, &snap))
	require.Equal(t, []fleet.SourceReading{
		{Figure: fleet.FigureRoutesHeld, Unit: fleet.SampleUnitCount, Value: 3},
	}, snap.Readings)

	statusMap, ok := r.status("").(map[string]any)
	require.True(t, ok)
	require.Equal(t, 3, statusMap["routes-in"])
}

// VALIDATES: forked bgp-rib declares show fleet source bgp-rib as hidden in
// Stage 1 and answers execute-command over the external SDK transport with a
// validated fleet.SourceSnapshot.
// PREVENTS: bgp-rib source sampling working only in-process and breaking when
// bgp-rib runs in a separate process.
func TestFleetSourceBGPRIBForkedExecution(t *testing.T) {
	ctx, connection, r, declaration := ac17ForkedConfigRuntime(t)

	found := false
	for _, decl := range declaration.Commands {
		if decl.Name != cmdFleetSourceBGPRIB {
			continue
		}
		found = true
		require.True(t, decl.Hidden)
		require.Equal(t, shapeDoc, decl.Shape)
		require.NotEmpty(t, decl.ShortHelp)
		break
	}
	require.True(t, found, "Stage 1 declaration must include %s", cmdFleetSourceBGPRIB)

	peerRIB, err := storage.NewAddressPeerRIB(netip.MustParseAddr("192.0.2.10"))
	require.NoError(t, err)
	t.Cleanup(peerRIB.Release)
	insertTestInboundRoute(t, peerRIB, "198.51.100.0/24")

	r.peerMu.Lock()
	r.bgpPeers[42] = peerRIB
	r.peerMu.Unlock()

	out, err := connection.SendExecuteCommand(ctx, &rpc.ExecuteCommandInput{
		Serial:  "fleet-source-bgp-rib",
		Command: cmdFleetSourceBGPRIB,
	})
	require.NoError(t, err)
	require.Equal(t, statusDone, out.Status)

	var snap fleet.SourceSnapshot
	require.NoError(t, json.Unmarshal([]byte(out.Data), &snap))
	require.NoError(t, fleet.ValidateSourceSnapshot(fleet.SampleOwnerBGPRIB, &snap))
	require.Equal(t, []fleet.SourceReading{
		{Figure: fleet.FigureRoutesHeld, Unit: fleet.SampleUnitCount, Value: 1},
	}, snap.Readings)
}

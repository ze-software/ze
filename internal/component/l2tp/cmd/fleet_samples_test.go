// Design: docs/architecture/fleet-config.md -- L2TP fleet sample producer tests
// Related: fleet_samples.go -- hidden read-only L2TP sample handler

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	l2tppkg "github.com/ze-software/ze/internal/component/l2tp"
	_ "github.com/ze-software/ze/internal/component/l2tp/cmd/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/fleet"
)

func decodeL2TPSourceSnapshot(t *testing.T, resp *plugin.Response) fleet.SourceSnapshot {
	t.Helper()
	require.NotNil(t, resp)
	require.Equal(t, plugin.StatusDone, resp.Status)
	var snap fleet.SourceSnapshot
	require.NoError(t, json.Unmarshal([]byte(responseString(t, resp)), &snap))
	require.NoError(t, fleet.ValidateSourceSnapshot(fleet.SampleOwnerL2TP, &snap))
	return snap
}

// VALIDATES: show fleet source l2tp returns empty readings when the L2TP
// subsystem is not running, rather than reporting a synthetic zero.
// PREVENTS: an unconfigured L2TP subsystem fabricating a zero subscriber
// session count on devices that do not run L2TP.
func TestFleetSourceL2TPAbsentWhenServiceStopped(t *testing.T) {
	publishFake(t, nil)

	resp, err := handleFleetSourceL2TP(nil, commandtest.Args())
	require.NoError(t, err)
	snap := decodeL2TPSourceSnapshot(t, resp)
	require.Equal(t, fleet.SampleOwnerL2TP, snap.Owner)
	require.Empty(t, snap.Readings)
}

// VALIDATES: show fleet source l2tp reports a real zero when the L2TP service
// is live with zero sessions and reports the snapshot's SessionCount when
// sessions are active.
// PREVENTS: real zero counts being confused with absence or session counts
// drifting from Service.Snapshot().SessionCount.
func TestFleetSourceL2TPActiveCounts(t *testing.T) {
	t.Run("live-zero-sessions", func(t *testing.T) {
		publishFake(t, &fakeService{snapshot: l2tppkg.Snapshot{SessionCount: 0}})

		resp, err := handleFleetSourceL2TP(nil, commandtest.Args())
		require.NoError(t, err)
		snap := decodeL2TPSourceSnapshot(t, resp)
		require.Equal(t, []fleet.SourceReading{
			{Figure: fleet.FigureSubscriberSessions, Unit: fleet.SampleUnitCount, Value: 0},
		}, snap.Readings)
	})

	t.Run("live-active-sessions", func(t *testing.T) {
		publishFake(t, &fakeService{snapshot: l2tppkg.Snapshot{TunnelCount: 2, SessionCount: 14}})

		resp, err := handleFleetSourceL2TP(nil, commandtest.Args())
		require.NoError(t, err)
		snap := decodeL2TPSourceSnapshot(t, resp)
		require.Equal(t, []fleet.SourceReading{
			{Figure: fleet.FigureSubscriberSessions, Unit: fleet.SampleUnitCount, Value: 14},
		}, snap.Readings)
	})
}

// VALIDATES: ze-l2tp-cmd.yang declares show fleet source l2tp as hidden and
// wires it to wireFleetSourceL2TP, and the RPC registration reaches it.
// PREVENTS: hidden fleet source commands leaking into operator completion or
// drifting from their registered wire method.
func TestFleetSourceL2TPSchemaAndRegistration(t *testing.T) {
	loader := configyang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	schema, err := loader.Resolve()
	require.NoError(t, err)

	tree := configyang.BuildCommandTree(schema)
	node := command.FindNode(tree, strings.Fields(cmdFleetSourceL2TP))
	require.NotNil(t, node)
	require.True(t, node.Hidden)
	require.Equal(t, wireFleetSourceL2TP, node.WireMethod)
	require.True(t, command.HiddenAtPath(tree, strings.Fields(cmdFleetSourceL2TP)))
	require.Empty(t, command.NewTreeCompleter(tree).Complete("show fleet source "))

	found := false
	for _, reg := range pluginserver.AllBuiltinRPCs() {
		if reg.WireMethod == wireFleetSourceL2TP {
			found = true
			require.NotNil(t, reg.Handler)
			break
		}
	}
	require.True(t, found, "wire method %s must be registered", wireFleetSourceL2TP)
}

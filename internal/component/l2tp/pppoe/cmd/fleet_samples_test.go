// Design: docs/architecture/fleet-config.md -- PPPoE fleet sample producer tests
// Related: fleet_samples.go -- hidden read-only PPPoE access-concentrator sample handler

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/component/l2tp/cmd/yang"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	_ "github.com/ze-software/ze/internal/component/l2tp/pppoe/cmd/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/fleet"
)

func decodePPPoESourceSnapshot(t *testing.T, resp *plugin.Response) fleet.SourceSnapshot {
	t.Helper()
	require.NotNil(t, resp)
	require.Equal(t, plugin.StatusDone, resp.Status)
	raw, err := json.Marshal(resp.Data)
	require.NoError(t, err)
	var snap fleet.SourceSnapshot
	require.NoError(t, json.Unmarshal(raw, &snap))
	require.NoError(t, fleet.ValidateSourceSnapshot(fleet.SampleOwnerPPPoE, &snap))
	return snap
}

// VALIDATES: show fleet source pppoe returns empty readings when the PPPoE
// access-concentrator subsystem is not running, rather than reporting zero.
// PREVENTS: devices without a configured PPPoE access concentrator (or with
// only outbound PPPoE client uplinks) fabricating a zero subscriber count.
func TestFleetSourcePPPoEAbsentWhenServiceStopped(t *testing.T) {
	pppoe.PublishService(nil)
	t.Cleanup(func() { pppoe.PublishService(nil) })

	resp, err := handleFleetSourcePPPoE(nil, commandtest.Args())
	require.NoError(t, err)
	snap := decodePPPoESourceSnapshot(t, resp)
	require.Equal(t, fleet.SampleOwnerPPPoE, snap.Owner)
	require.Empty(t, snap.Readings)
}

// VALIDATES: show fleet source pppoe reports a real zero when the live PPPoE
// subsystem has zero sessions and projects non-zero SessionCount accurately.
// PREVENTS: real zero counts being confused with absence or subscriber session
// readings drifting from the access-concentrator snapshot.
func TestFleetSourcePPPoEActiveCounts(t *testing.T) {
	t.Run("live-zero-sessions", func(t *testing.T) {
		sub := pppoe.NewSubsystem(pppoe.Parameters{})
		pppoe.PublishService(sub)
		t.Cleanup(func() { pppoe.PublishService(nil) })

		resp, err := handleFleetSourcePPPoE(nil, commandtest.Args())
		require.NoError(t, err)
		snap := decodePPPoESourceSnapshot(t, resp)
		require.Equal(t, []fleet.SourceReading{
			{Figure: fleet.FigureSubscriberSessions, Unit: fleet.SampleUnitCount, Value: 0},
		}, snap.Readings)
	})

	t.Run("nonzero-snapshot-projection", func(t *testing.T) {
		snap := fleetPPPoESnapshot(pppoe.Snapshot{InterfaceCount: 2, SessionCount: 9})
		require.NoError(t, fleet.ValidateSourceSnapshot(fleet.SampleOwnerPPPoE, &snap))
		require.Equal(t, []fleet.SourceReading{
			{Figure: fleet.FigureSubscriberSessions, Unit: fleet.SampleUnitCount, Value: 9},
		}, snap.Readings)
	})
}

// VALIDATES: ze-pppoe-cmd.yang and ze-l2tp-cmd.yang merge cleanly without YANG
// augment collisions, hiding both show fleet source pppoe and show fleet source
// l2tp while wiring wireFleetSourcePPPoE to its registered handler.
// PREVENTS: duplicate-identifier schema failures between L2TP and PPPoE or
// hidden source commands leaking into operator completion.
func TestFleetSourcePPPoESchemaAndRegistration(t *testing.T) {
	loader := configyang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	schema, err := loader.Resolve()
	require.NoError(t, err)

	tree := configyang.BuildCommandTree(schema)
	node := command.FindNode(tree, strings.Fields(cmdFleetSourcePPPoE))
	require.NotNil(t, node)
	require.True(t, node.Hidden)
	require.Equal(t, wireFleetSourcePPPoE, node.WireMethod)
	require.True(t, command.HiddenAtPath(tree, strings.Fields(cmdFleetSourcePPPoE)))
	require.Empty(t, command.NewTreeCompleter(tree).Complete("show fleet source "))

	// Confirm the sibling L2TP source merged cleanly under the same hidden parent.
	l2tpNode := command.FindNode(tree, []string{"show", "fleet", "source", "l2tp"})
	require.NotNil(t, l2tpNode)
	require.True(t, l2tpNode.Hidden)

	found := false
	for _, reg := range pluginserver.AllBuiltinRPCs() {
		if reg.WireMethod == wireFleetSourcePPPoE {
			found = true
			require.NotNil(t, reg.Handler)
			break
		}
	}
	require.True(t, found, "wire method %s must be registered", wireFleetSourcePPPoE)
}

// Design: docs/architecture/ike/ipsec-dataplane-inspection.md -- kernel dataplane read surface
// Related: show_dataplane.go -- policyInfoToMap, the renderer under test
// Related: show_dataplane_test.go -- fakeDataplane, useDataplane and rowsOf, the fixtures

package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// TestShowDataplanePolicyRendersDisplacedFinalEntry pins the displaced-final-entry and
// displaced-final-action keys: a PROTECT row that stands in for an SPD's catch-all says
// which final entry and with which disposition Ze puts it back
// (PolicyInfo.Displaced), because the listing then holds no catch-all row to explain
// where the final entry went.
//
// Method: three rows through the handler: a wildcard peer's PROTECT row standing in for
// the node-wide catch-all discard, one standing in for an interface catch-all that
// comes back as a bypass, and an ordinary PROTECT row. The first two differ in both
// keys, so a constant rendering, or one that swaps role and action, is red, and the
// third must read "none" in both rather than a blank or the zero action's "protect".
func TestShowDataplanePolicyRendersDisplacedFinalEntry(t *testing.T) {
	useDataplane(t, &fakeDataplane{policies: []dataplane.PolicyInfo{
		{
			Dir: dataplane.SADirOut, Priority: 1,
			Displaced: dataplane.DisplacedFinalEntry{Role: dataplane.PolicyRoleCatchAll, Action: dataplane.SPActionDiscard},
		},
		{
			Dir: dataplane.SADirOut, Priority: 2,
			Displaced: dataplane.DisplacedFinalEntry{Role: dataplane.PolicyRoleInterfaceCatchAll, Action: dataplane.SPActionBypass},
		},
		{Dir: dataplane.SADirOut, Priority: 3},
	}})

	resp, err := handleShowVPNIPsecDataplanePolicy(nil, commandtest.Args())
	require.NoError(t, err)
	rows := rowsOf(t, resp, "policies")
	require.Len(t, rows, 3)
	byPriority := map[any]map[string]any{}
	for _, row := range rows {
		byPriority[row["priority"]] = row
	}
	node, iface, plain := byPriority[1], byPriority[2], byPriority[3]
	require.NotNil(t, node, "no row at priority 1: %v", rows)
	require.NotNil(t, iface, "no row at priority 2: %v", rows)
	require.NotNil(t, plain, "no row at priority 3: %v", rows)

	require.Equal(t, "catch-all", node["displaced-final-entry"], "the row standing in for the node-wide catch-all names it")
	require.Equal(t, "discard", node["displaced-final-action"], "and the disposition that comes back")
	require.Equal(t, "interface-catch-all", iface["displaced-final-entry"], "the row standing in for an interface catch-all names it")
	require.Equal(t, "bypass", iface["displaced-final-action"], "and the disposition that comes back")
	require.Equal(t, "none", plain["displaced-final-entry"], "a row that stands in for nothing says none")
	require.Equal(t, "none", plain["displaced-final-action"], "and never the zero action's protect")
}

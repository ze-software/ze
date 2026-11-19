// Design: docs/functional-tests.md -- a compiled driver's decisions are unit-tested, its .ci proves the path
// Related: misc_fixture_managed_fleet_divergence.go -- the divergence list decoder under test

package fixture

import (
	"encoding/json"
	"testing"
)

// TestFleetDivergenceListDecodesOnlyTheTableArray proves the drivers read the
// divergence list only in the form the CLI renders for a table, the array of
// its rows, and refuse the handler's document with its rows under `clients`.
// The method is one answer of each shape: a decoder that took both would let
// a change of the CLI's JSON form of a table pass every divergence draft.
func TestFleetDivergenceListDecodesOnlyTheTableArray(t *testing.T) {
	var rows fleetDivergenceList
	if err := json.Unmarshal([]byte(`[{"client":"edge-01","declared":true,"device-version":"0123456789abcdef"}]`), &rows); err != nil {
		t.Fatalf("the table's array of rows did not decode: %v", err)
	}
	if len(rows.Clients) != 1 || rows.Clients[0].Client != "edge-01" {
		t.Fatalf("the table's array decoded as %+v, want the one row of edge-01", rows.Clients)
	}

	var document fleetDivergenceList
	if err := json.Unmarshal([]byte(`{"clients":[{"client":"edge-01"}]}`), &document); err == nil {
		t.Fatalf("the handler's document decoded as %+v, want a refusal: the CLI renders the table as an array", document.Clients)
	}
}

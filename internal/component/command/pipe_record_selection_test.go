// Design: docs/architecture/api/commands.md -- display selection over nested records

package command

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// recordSelectionPayload has the two different uptime columns in show bgp's
// answer: the daemon's age and each peer's session age. Extra peer fields prove
// the chosen record is narrowed rather than copied as a route around selection.
const recordSelectionPayload = `{"router-id":"192.0.2.254","local-as":65000,"uptime":"2h0m0s","peers-configured":2,"peers-established":1,"peers":[{"address":"192.0.2.1","remote-as":65001,"state":"established","uptime":"1h0m0s","description":"transit"},{"address":"192.0.2.2","remote-as":65002,"state":"idle","uptime":"0s","description":"peering"}]}`

// TestDisplaySelectsTheOneRecordShapeHoldingRequestedFields reproduces the
// uptime collision through the pipe formatter. Only the peer records hold all
// four requested columns, so a shared top-level uptime must not hide the rows.
func TestDisplaySelectsTheOneRecordShapeHoldingRequestedFields(t *testing.T) {
	resetAliasTables(t)
	const selection = "show test summary | display address remote-as state uptime | "
	const want = `[{"address":"192.0.2.1","remote-as":65001,"state":"established","uptime":"1h0m0s"},{"address":"192.0.2.2","remote-as":65002,"state":"idle","uptime":"0s"}]`
	rendered := renderThroughPipes(t, selection+"json", recordSelectionPayload)
	require.False(t, IsPipeError(rendered), "a unique record shape must remain selectable: %s", rendered)
	assert.JSONEq(t, want, rendered)

	rendered = renderThroughPipes(t, selection+"text", recordSelectionPayload)
	for _, value := range []string{"192.0.2.1", "192.0.2.2", "65001", "65002", "established", "idle", "1h0m0s", "0s"} {
		assert.Contains(t, rendered, value)
	}
	for _, value := range []string{"192.0.2.254", "2h0m0s", "transit", "peering"} {
		assert.NotContains(t, rendered, value)
	}
}

// TestDisplayRefusesAColumnSharedByDifferentRecordShapes asks for the one
// column that cannot identify either record. A refusal must name that column
// instead of silently returning one age and discarding the other records.
func TestDisplayRefusesAColumnSharedByDifferentRecordShapes(t *testing.T) {
	resetAliasTables(t)
	_, format, refusal := processPipesChecked("show test summary | display uptime | json")
	require.Empty(t, refusal)
	rendered := format(recordSelectionPayload)
	require.True(t, IsPipeError(rendered), "an ambiguous column answered data: %s", rendered)
	assert.Contains(t, strings.ToLower(rendered), "uptime")
	assert.NotContains(t, rendered, "2h0m0s")
	assert.NotContains(t, rendered, "1h0m0s")
}

// TestDisplayRecordDisambiguationKeepsExplicitAliases protects the summary
// and peers alias expansions declared by peer.registerAliases. Naming aggregate
// fields selects the outer record; naming peers explicitly keeps its rows.
func TestDisplayRecordDisambiguationKeepsExplicitAliases(t *testing.T) {
	resetAliasTables(t)
	RegisterAliases([]string{"show test summary"},
		Alias{Name: "summary", Expansion: "display router-id local-as uptime peers-configured peers-established"},
		Alias{Name: "peers", Expansion: "display peers"},
	)
	for _, alias := range []string{"summary", "peers"} {
		t.Run(alias, func(t *testing.T) {
			rendered := renderThroughPipes(t, "show test summary | "+alias+" | json", recordSelectionPayload)
			require.False(t, IsPipeError(rendered), "the explicit alias must remain useful: %s", rendered)
			if alias == "summary" {
				assert.JSONEq(t, `{"router-id":"192.0.2.254","local-as":65000,"uptime":"2h0m0s","peers-configured":2,"peers-established":1}`, rendered)
				return
			}
			assert.JSONEq(t, `[{"address":"192.0.2.1","remote-as":65001,"state":"established","uptime":"1h0m0s","description":"transit"},{"address":"192.0.2.2","remote-as":65002,"state":"idle","uptime":"0s","description":"peering"}]`, rendered)
		})
	}
}

// TestRecordDisplayRefusesAmbiguousColumns drives the record-processing path,
// where a selector refusal must become a fault and stop pulling data. It must
// never return the original ambiguous item or either age as successful data.
func TestRecordDisplayRefusesAmbiguousColumns(t *testing.T) {
	resetAliasTables(t)
	produced := 0
	records := func(yield func(rpc.Record) bool) {
		for range 2 {
			produced++
			if !yield(rpc.Record{Item: json.RawMessage(recordSelectionPayload)}) {
				return
			}
		}
	}
	selected := collectRecords(applyRecordsForTest(t, "show test summary | display uptime", records))
	require.Len(t, selected, 1)
	assert.Equal(t, 1, produced)
	assert.Empty(t, selected[0].Item)
	assert.Contains(t, string(selected[0].Fault), "uptime")
	assert.NotContains(t, string(selected[0].Fault), "2h0m0s")
	assert.NotContains(t, string(selected[0].Fault), "1h0m0s")
}

// TestRecordDisplaySelectsUniqueNestedShape keeps the same peer fields through
// the record-processing path. Records retain their object shape until a later
// renderer performs the existing sole-container unwrapping.
func TestRecordDisplaySelectsUniqueNestedShape(t *testing.T) {
	resetAliasTables(t)
	records := func(yield func(rpc.Record) bool) {
		yield(rpc.Record{Item: json.RawMessage(recordSelectionPayload)})
	}
	selected := collectRecords(applyRecordsForTest(t, "show test summary | display address remote-as state uptime", records))
	require.Len(t, selected, 1)
	require.Empty(t, selected[0].Fault)
	assert.JSONEq(t, `{"peers":[{"address":"192.0.2.1","remote-as":65001,"state":"established","uptime":"1h0m0s"},{"address":"192.0.2.2","remote-as":65002,"state":"idle","uptime":"0s"}]}`, string(selected[0].Item))
}

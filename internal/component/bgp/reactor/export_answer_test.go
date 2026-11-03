// Design: docs/architecture/core-design.md -- Policy Filter Chain, the export step
// Related: filter_chain.go -- policyFilterSeam, the plugin answer this file stands in for
// Related: filter_library_external_test.go -- textBody, the raw body an edit builds

package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

// exportAnswerFilter is the one plugin filter exportAnswer binds.
const exportAnswerFilter = "probe:edit"

// exportAnswer binds each of dests to an export chain of one plugin filter,
// exportAnswerFilter, and has r answer that filter with answer, through
// policyFilterSeam (filter_chain.go). A test's egress edit then reaches the
// destination the way every egress edit does in production: from the
// destination's export chain, the chain step of forwardUpdateCore on the
// general rail and runsExportChain on the RS fast path alike. answer is given
// the destination's address. r.api is set because the chain step fails
// closed with no filter engine, and r.orderedEgressSteps is the pass
// startAPIServer builds.
func exportAnswer(t *testing.T, r *Reactor, answer func(peer string) PolicyResponse, dests ...*Peer) {
	t.Helper()
	r.api = &pluginserver.Server{}
	r.orderedEgressSteps = buildOrderedEgressSteps()
	r.policyFilterSeam = func(_, _, direction, peer string, _ uint32, _ string) PolicyResponse {
		if direction != directionExport {
			t.Errorf("%s was asked a %s verdict, want %s", exportAnswerFilter, direction, directionExport)
			return PolicyResponse{Action: PolicyReject}
		}
		return answer(peer)
	}
	for _, dest := range dests {
		dest.settings.ExportPolicy = mustPolicy(t, []routepolicy.Ref{{Name: exportAnswerFilter}}, routepolicy.FinalAccept)
		dest.refreshForwardFacts()
	}
}

// probeExportChain answers an export policy of the one plugin filter probe:f,
// compiled at bgp export with a final accept as bgp/config compiles a peer's
// chain; r.filterTransportSeam (an externalTransport) answers the filter.
func probeExportChain(t *testing.T) filterapi.Policy {
	t.Helper()
	chain, err := policychain.Compile("bgp export", nil, []policychain.Member{{Name: "f", Plugin: "probe"}},
		routepolicy.FinalAccept, policychain.Bases{LocalPreference: 100})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return mustPolicy(t, []routepolicy.Ref{{Name: "probe:f"}}, routepolicy.FinalAccept).WithLibrary(chain)
}

// rawEdit answers the modify verdict whose raw body is received with ops
// applied (textBody): the edit an in-process step once wrote into the
// destination's ModAccumulator, now made by the destination's export chain.
func rawEdit(t *testing.T, r *Reactor, received []byte, ops func(*filterapi.ModAccumulator)) PolicyResponse {
	t.Helper()
	return PolicyResponse{Action: PolicyModify, Raw: textBody(t, r, received, ops)}
}

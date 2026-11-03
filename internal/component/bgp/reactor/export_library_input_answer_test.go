// Design: docs/architecture/core-design.md -- Policy Filter Chain, the export step
// Related: export_library_answer_test.go -- exportLibraryAnswer, the per-destination form
// Related: filter_answer_transport_test.go -- answerTransport, answerPolicy

package reactor

import (
	"testing"

	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// exportLibraryInputAnswer is exportLibraryAnswer for an answer that decides
// from the route it is sent (input.Raw, input.Peer), as a plugin export
// filter deciding on the route's content does. The filter reads the raw body
// and may set the MED or the local preference, so answer can return a raw
// rewrite of input.Raw, a typed set, an accept or a reject. The library chain
// sends it each UPDATE once, whatever families the UPDATE carries.
func exportLibraryInputAnswer(t testing.TB, r *Reactor, answer func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput, dests ...*Peer) {
	t.Helper()
	transport := &answerTransport{
		reads: []string{rpc.FilterFieldRaw},
		sets:  []string{"med", "local-preference"},
		answer: func(input rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
			if input.Direction != directionExport {
				t.Errorf("%s was asked a %s verdict, want %s", exportLibraryFilter, input.Direction, directionExport)
				return rpc.FilterUpdateOutput{Action: rpc.FilterReject}
			}
			return answer(input)
		},
	}
	r.filterTransportSeam = transport
	r.orderedEgressSteps = buildOrderedEgressSteps()
	for _, dest := range dests {
		dest.settings.ExportPolicy = answerPolicy(t, routepolicy.FinalAccept, exportLibraryFilter)
		dest.refreshForwardFacts()
	}
}

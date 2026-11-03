// Design: docs/architecture/core-design.md -- Policy Filter Chain, the export step
// Related: filter_answer_transport_test.go -- answerTransport, answerPolicy
// Related: filter_library_external_test.go -- textBody, the raw body an edit builds

package reactor

import (
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// exportLibraryFilter is the one answerPlugin filter exportLibraryAnswer binds.
const exportLibraryFilter = "edit"

// exportLibraryAnswer binds each of dests to an export chain of one plugin
// filter, compiled as a library chain (answerPolicy), and has r answer it
// with answer through r.filterTransportSeam. The filter reads the raw body
// and may set the MED or the local preference, so answer can return a raw
// rewrite, a typed set, an accept or a reject. answer is given the
// destination's address. A test's egress edit then reaches the destination
// the way every egress edit does in production: from the destination's
// export chain, on the general rail and the RS fast path alike.
// r.orderedEgressSteps is the pass startAPIServer builds.
func exportLibraryAnswer(t testing.TB, r *Reactor, answer func(peer string) rpc.FilterUpdateOutput, dests ...*Peer) {
	t.Helper()
	exportLibraryInputAnswer(t, r, func(input rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
		return answer(input.Peer)
	}, dests...)
}

// medLibraryAnswer is an exportLibraryAnswer answer: the typed set a plugin
// export filter sends to set the MED to med, for every destination.
func medLibraryAnswer(med uint32) func(string) rpc.FilterUpdateOutput {
	value := strconv.FormatUint(uint64(med), 10)
	return func(string) rpc.FilterUpdateOutput { return answerSet("med", value) }
}

// rawLibraryEdit answers the modify verdict whose raw body is received with
// ops applied (textBody), as rawEdit does for the text seam.
func rawLibraryEdit(t *testing.T, r *Reactor, received []byte, ops func(*filterapi.ModAccumulator)) rpc.FilterUpdateOutput {
	t.Helper()
	return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: textBody(t, r, received, ops)}
}

// libraryReject is the reject verdict, for every destination.
func libraryReject(string) rpc.FilterUpdateOutput {
	return rpc.FilterUpdateOutput{Action: rpc.FilterReject}
}

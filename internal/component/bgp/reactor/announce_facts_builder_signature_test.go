// Design: reactor_api_batch.go -- announceFacts, the struct that is both the build-group key and the builders' per-peer input.
// Related: announce_facts_partition_test.go -- proves each existing field partitions the groups.
package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
)

// TestAnnounceBuildersTakePerPeerInputOnlyAsAnnounceFacts pins the parameter
// lists of the three group builders to the agreed shape: the per-batch
// buffers, the per-batch NLRIBatch, and announceFacts.
//
// VALIDATES: a per-peer value can reach buildBatchAnnounceUpdate,
// buildBatchWithdrawUpdate or buildWithheldWithdrawUpdate only as a field of
// announceFacts. announceFacts is
// the map key that puts peers on one build, so a field there partitions the
// groups by construction, and announce_facts_partition_test.go proves each
// field does.
// PREVENTS: a new per-peer value passed as an extra builder argument, fed from
// targets[i].peer or from the group loop. The compiler accepts that, the key
// never learns the value, and two peers that differ in it share one UPDATE: one
// peer then receives the other peer's bytes, decided by Go map iteration order.
//
// Method: pinBuilderSignatures takes each builder as a parameter of the
// agreed function type, so the call compiles only while each builder has
// exactly that signature. A changed parameter list fails this package's build
// and every test in it. The guard is the type check; the body has nothing to
// assert at run time. The Go language cannot say "per-peer input only through
// this struct", so the test suite enforces it instead (owner decision,
// 2026-10-09).
func TestAnnounceBuildersTakePerPeerInputOnlyAsAnnounceFacts(t *testing.T) {
	// If this call fails to compile, a builder's parameters changed. A new
	// per-peer value MUST be a field of announceFacts, set in announceFactsFor
	// and withdrawFactsFor, with a case in announceFactCases. It MUST NOT be a
	// new parameter: announceFacts is the grouping key, and a parameter is not.
	// Only a new PER-BATCH input (one value for every peer of the fan-out) may
	// be a parameter, and then the agreed type below changes with it.
	pinBuilderSignatures(t,
		(*reactorAPIAdapter).buildBatchAnnounceUpdate,
		(*reactorAPIAdapter).buildBatchWithdrawUpdate,
		(*reactorAPIAdapter).buildWithheldWithdrawUpdate)
}

// pinBuilderSignatures states the agreed builder signatures as parameter
// types, one for each builder whose UPDATE is built once per group and sent to
// every member. The withdraw rail groups on the same struct (withdrawFactsFor
// returns an announceFacts), so the same rule holds for its two builders:
// buildBatchWithdrawUpdate, and buildWithheldWithdrawUpdate, whose
// attributes-only UPDATE goes to every unarmed peer of the group.
func pinBuilderSignatures(t *testing.T,
	announce func(*reactorAPIAdapter, []byte, []byte, bgptypes.NLRIBatch, announceFacts) (*message.Update, error),
	withdraw func(*reactorAPIAdapter, []byte, []byte, bgptypes.NLRIBatch, announceFacts) *message.Update,
	withheld func(*reactorAPIAdapter, []byte, bgptypes.NLRIBatch, announceFacts) *message.Update,
) {
	t.Helper()
	if announce == nil {
		t.Fatal("BUG: buildBatchAnnounceUpdate method expression is nil")
	}
	if withdraw == nil {
		t.Fatal("BUG: buildBatchWithdrawUpdate method expression is nil")
	}
	if withheld == nil {
		t.Fatal("BUG: buildWithheldWithdrawUpdate method expression is nil")
	}
}

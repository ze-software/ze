// Design: docs/architecture/plugin/rib-storage-design.md -- the receive gate across the process boundary
// Related: forward_validation.go -- eligibilityBatch, the batched gate query of one section

package reactor

import (
	"bytes"
	"context"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestForwardValidationRemoteGateBatches proves the forward path reads a
// receive gate whose owner runs in another process through the owner's
// eligibility command, once per NLRI section rather than once per prefix, and
// applies its verdicts exactly as it applies an in-memory lookup's.
//
// VALIDATES: with only an announced gate (no lookup in this process), the
// eligible announcement is forwarded, the ineligible one becomes a
// withdrawal, the carried withdrawal stays, and the gate is asked once for
// the withdrawn section and once for the announced one.
// PREVENTS: a forked bgp-adj-rib-in whose validated routes the daemon never
// advertises, and a per-prefix round trip to it on the forward path.
// Method: TestForwardValidationMixedNLRI's UPDATE, its verdicts served by
// ribevents.AnswerEligibility behind a counting GateDispatch.
func TestForwardValidationRemoteGateBatches(t *testing.T) {
	api, src, dst, capture := validationForwardFixture(t, false)
	attrs := mustHex(t, storedIPv4Route("10.0.0.1").AttrHex)
	payload := buildUpdatePayload(attrs, []byte{24, 10, 0, 0, 24, 10, 0, 1})
	payload = append([]byte{0, 4, 24, 10, 0, 2}, payload[2:]...)
	update, id := newLeakTestUpdate(t, api.r.recentUpdates, payload, src.recvCtxID)
	update.WireUpdate.SetSourceID(src.SourceID())
	ownerEligible := func(key ribevents.ValidationRoute, generation uint64) bool {
		return key.Prefix == netip.MustParsePrefix("10.0.0.0/24") && (generation == 0 || generation == id)
	}
	const command = "request test gate eligibility"
	calls := 0
	release := ribevents.RegisterGateDispatch(func(_ context.Context, cmd string, args ...string) (string, json.RawMessage, error) {
		calls++
		answer, err := ribevents.AnswerEligibility(args[0], ownerEligible, func(ribevents.ValidationRoute) bool { return true })
		if err != nil {
			return "", nil, err
		}
		data, err := json.Marshal(answer)
		return "done", data, err
	})
	t.Cleanup(release)
	ribevents.RegisterValidationLookup(nil, nil)
	ribevents.NoteValidationGate(ribevents.ValidationGateState{Enabled: true, Command: command})
	t.Cleanup(func() { ribevents.NoteValidationGate(ribevents.ValidationGateState{Enabled: false}) })

	sel, err := selector.Parse(dst.Settings().Address.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := api.ForwardUpdate(sel, id, "test-plugin", plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	announced, withdrawn := validationParts(t, validationBodies(t, api, capture))
	if !bytes.Equal(announced, []byte{24, 10, 0, 0}) {
		t.Fatalf("remote-gate announcement = %x", announced)
	}
	if !bytes.Equal(withdrawn, []byte{24, 10, 0, 1, 24, 10, 0, 2}) {
		t.Fatalf("remote-gate withdrawals = %x", withdrawn)
	}
	if calls != 2 {
		t.Fatalf("gate asked %d times, want 2: one per NLRI section", calls)
	}
}

// Design: docs/architecture/plugin/rib-storage-design.md -- the receive gate across the process boundary
// Related: gate_remote.go -- remoteValidation, AnswerEligibility, RegisterGateDispatch

package ribevents

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestRemoteGateAnswersThroughOwnerCommand proves a process that does not hold
// the receive gate's lookup asks the owner through the command its
// announcement names, and fails closed when that command cannot answer.
//
// VALIDATES: RouteEligible, RoutePresent and RouteEligibleBatch answer the
// owner's verdicts through GateDispatch and AnswerEligibility, msgID included;
// a batch is one command; a dispatch error, a missing dispatcher and a short
// answer each refuse.
// PREVENTS: a forked receive store whose validated routes the daemon never
// advertises (every route refused because the lookup does not cross).
// Method: a fake dispatch answers with AnswerEligibility over a fake store.
func TestRemoteGateAnswersThroughOwnerCommand(t *testing.T) {
	t.Cleanup(func() { RegisterValidationLookup(nil, nil) })
	RegisterValidationLookup(nil, nil)
	v4 := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}
	peer := netip.MustParseAddr("192.0.2.1")
	valid := ValidationRoute{Peer: peer, Family: v4, Prefix: netip.MustParsePrefix("203.0.113.0/24")}
	invalid := ValidationRoute{Peer: peer, Family: v4, Prefix: netip.MustParsePrefix("198.51.100.0/24")}
	ownerEligible := func(route ValidationRoute, msgID uint64) bool {
		return route.Prefix == valid.Prefix && (msgID == 0 || msgID == 9)
	}
	ownerPresent := func(route ValidationRoute) bool { return true }

	const command = "request test gate eligibility"
	calls := 0
	var failWith error
	release := RegisterGateDispatch(func(_ context.Context, cmd string, args ...string) (string, json.RawMessage, error) {
		calls++
		if failWith != nil {
			return "", nil, failWith
		}
		if cmd != command || len(args) != 1 {
			return "error", nil, nil
		}
		answer, err := AnswerEligibility(args[0], ownerEligible, ownerPresent)
		if err != nil {
			return "", nil, err
		}
		data, err := json.Marshal(answer)
		return "done", data, err
	})
	t.Cleanup(release)

	NoteValidationGate(ValidationGateState{Enabled: true, Command: command})
	if !ValidationEnabled() {
		t.Fatal("an announced gate must turn validation on")
	}
	if !RouteEligible(valid, 0) || !RouteEligible(valid, 9) {
		t.Fatal("the owner's eligible verdict must reach this process")
	}
	if RouteEligible(valid, 8) {
		t.Fatal("a generation the owner does not hold must be refused")
	}
	if RouteEligible(invalid, 0) {
		t.Fatal("the owner's ineligible verdict must reach this process")
	}
	if !RoutePresent(invalid) {
		t.Fatal("the owner's present verdict must reach this process")
	}

	calls = 0
	queries := []EligibilityQuery{{Route: valid, MsgID: 9}, {Route: invalid}, {Route: invalid, Present: true}}
	answers := make([]bool, len(queries))
	RouteEligibleBatch(queries, answers)
	if calls != 1 {
		t.Fatalf("a batch must be one command, got %d", calls)
	}
	if !answers[0] || answers[1] || !answers[2] {
		t.Fatalf("batch answers = %v, want [true false true]", answers)
	}

	failWith = errors.New("owner gone")
	if RouteEligible(valid, 0) {
		t.Fatal("a command that fails must refuse the route")
	}
	failWith = nil

	release()
	if RouteEligible(valid, 0) {
		t.Fatal("no route to the dispatcher must refuse the route")
	}

	NoteValidationGate(ValidationGateState{Enabled: false})
	if ValidationEnabled() {
		t.Fatal("a disabled announcement must remove the remote gate")
	}
}

// TestAnswerEligibilityRefusesOversizedBatch proves the owner bounds a request.
//
// VALIDATES: a request above EligibilityBatchMax and an undecodable one are
// refused; the answers follow query order.
// PREVENTS: an unbounded allocation driven by another process.
// Method: call AnswerEligibility directly.
func TestAnswerEligibilityRefusesOversizedBatch(t *testing.T) {
	yes := func(ValidationRoute, uint64) bool { return true }
	no := func(ValidationRoute) bool { return false }
	if _, err := AnswerEligibility("{", yes, no); err == nil {
		t.Fatal("an undecodable request must be refused")
	}
	var b strings.Builder
	b.WriteString(`{"queries":[`)
	for i := range EligibilityBatchMax + 1 {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"route":{"peer-address":"192.0.2.1","family":"ipv4/unicast","prefix":"203.0.113.0/24","path-id":0,"nlri":""}}`)
	}
	b.WriteString(`]}`)
	if _, err := AnswerEligibility(b.String(), yes, no); err == nil {
		t.Fatal("a request above EligibilityBatchMax must be refused")
	}
	answer, err := AnswerEligibility(`{"queries":[{"route":{"peer-address":"192.0.2.1","family":"ipv4/unicast","prefix":"203.0.113.0/24","path-id":0,"nlri":""}},{"route":{"peer-address":"192.0.2.1","family":"ipv4/unicast","prefix":"203.0.113.0/24","path-id":0,"nlri":""},"present":true}]}`, yes, no)
	if err != nil {
		t.Fatal(err)
	}
	if len(answer.Answers) != 2 || !answer.Answers[0] || answer.Answers[1] {
		t.Fatalf("answers = %v, want [true false]", answer.Answers)
	}
}

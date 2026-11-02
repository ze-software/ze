// Design: docs/architecture/plugin/rib-storage-design.md -- FlowSpec authorization across the process boundary
// Related: flowspec_remote.go -- remoteFlowSpec, AnswerFlowSpec, NoteFlowSpecGate

package ribevents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRemoteFlowSpecAnswersThroughRIBCommand proves a process that does not
// hold the selecting RIB's FlowSpec lookup asks the RIB through the command
// its flowspec-gate announcement names, and fails closed when it cannot.
//
// VALIDATES: RouteEligible, RoutePresent, LookupFlowSpecPath, FlowSpecRoutes
// and FlowSpecRedirectRefused answer the RIB's verdicts through GateDispatch
// and AnswerFlowSpec; RouteEligibleBatch asks every FlowSpec rule in one
// command; a dispatch error refuses each answer; an empty command returns to
// "no RIB"; an announcement never replaces the RIB's own registration.
// PREVENTS: a forked bgp-rib (or a forked reader) whose readers withdraw every
// FlowSpec rule on replay and forward none (LookupFlowSpecPath false, nil
// provider) with no error anywhere.
// Method: the owner's registration is saved, the reader's globals cleared, and
// a fake dispatch answers with AnswerFlowSpec over the owner's registration.
func TestRemoteFlowSpecAnswersThroughRIBCommand(t *testing.T) {
	t.Cleanup(func() {
		RegisterFlowSpecLookup(nil, nil, nil, nil)
		RegisterFlowSpecRedirectLookup(nil)
	})
	// The route crosses as JSON, whose family name the registry resolves.
	family.RegisterTestFamilies()
	fs := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	peer := netip.MustParseAddr("192.0.2.1")
	authorized := ValidationRoute{Peer: peer, Family: fs, NLRI: "\x01\x18\xc0\x00\x02"}
	refused := ValidationRoute{Peer: peer, Family: fs, NLRI: "\x01\x18\xc6\x33\x64"}
	attributes := []byte{0x40, 0x01, 0x01, 0x00}

	RegisterFlowSpecLookup(
		func(route ValidationRoute, msgID uint64) bool { return route == authorized && msgID != 8 },
		func(route ValidationRoute) bool { return true },
		func(route ValidationRoute) (FlowSpecPath, bool) {
			if route != authorized {
				return FlowSpecPath{}, false
			}
			return FlowSpecPath{Attributes: attributes, MsgID: 7, SourcePolicyEpoch: 3}, true
		},
		func() []ValidationRoute { return []ValidationRoute{authorized} },
	)
	RegisterFlowSpecRedirectLookup(func(route ValidationRoute, _ uint64) bool { return route == refused })
	owner, ownerRedirect := flowSpecLookup.Load(), flowSpecRedirect.Load()
	flowSpecLookup.Store(nil)
	flowSpecRedirect.Store(nil)

	const command = "request test rib flowspec"
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
		reader, readerRedirect := flowSpecLookup.Load(), flowSpecRedirect.Load()
		flowSpecLookup.Store(owner)
		flowSpecRedirect.Store(ownerRedirect)
		answer, err := AnswerFlowSpec(args[0])
		flowSpecLookup.Store(reader)
		flowSpecRedirect.Store(readerRedirect)
		if err != nil {
			return "", nil, err
		}
		data, err := json.Marshal(answer)
		return "done", data, err
	})
	t.Cleanup(release)

	if _, found := LookupFlowSpecPath(authorized); found {
		t.Fatal("before the announcement this process holds no RIB")
	}
	NoteFlowSpecGate(FlowSpecGateState{Command: command})

	if !RouteEligible(authorized, 7) {
		t.Fatal("the RIB's authorized verdict must reach this process")
	}
	if RouteEligible(authorized, 8) || RouteEligible(refused, 0) {
		t.Fatal("the RIB's unauthorized verdicts must reach this process")
	}
	if !RoutePresent(refused) {
		t.Fatal("the RIB's present verdict must reach this process")
	}
	path, found := LookupFlowSpecPath(authorized)
	if !found || !bytes.Equal(path.Attributes, attributes) || path.MsgID != 7 || path.SourcePolicyEpoch != 3 {
		t.Fatalf("path = %+v, %v; want the RIB's owned attributes, msg 7, epoch 3", path, found)
	}
	if _, found := LookupFlowSpecPath(refused); found {
		t.Fatal("a rule the RIB holds no path for must answer none")
	}
	if routes := FlowSpecRoutes(); len(routes) != 1 || routes[0] != authorized {
		t.Fatalf("routes = %v, want the RIB's one authorized rule", routes)
	}
	if !FlowSpecRedirectRefused(refused, 0) || FlowSpecRedirectRefused(authorized, 0) {
		t.Fatal("the RIB's redirect verdicts must reach this process")
	}

	calls = 0
	queries := []EligibilityQuery{{Route: authorized, MsgID: 7}, {Route: refused}, {Route: refused, Present: true}}
	answers := make([]bool, len(queries))
	RouteEligibleBatch(queries, answers)
	if calls != 1 {
		t.Fatalf("a batch of FlowSpec rules took %d commands, want 1", calls)
	}
	if !answers[0] || answers[1] || !answers[2] {
		t.Fatalf("batch answers = %v, want [true false true]", answers)
	}

	failWith = errors.New("the RIB stopped")
	if RouteEligible(authorized, 7) || RoutePresent(authorized) {
		t.Fatal("an unanswered lookup must refuse")
	}
	if _, found := LookupFlowSpecPath(authorized); found {
		t.Fatal("an unanswered path lookup must answer none")
	}
	if FlowSpecRoutes() != nil {
		t.Fatal("an unanswered routes lookup must replay nothing")
	}
	if !FlowSpecRedirectRefused(authorized, 0) {
		t.Fatal("an unanswered redirect lookup must refuse the redirect")
	}
	failWith = nil

	NoteFlowSpecGate(FlowSpecGateState{})
	if flowSpecLookup.Load() != nil {
		t.Fatal("an empty command must return this process to no RIB")
	}
	flowSpecLookup.Store(owner)
	NoteFlowSpecGate(FlowSpecGateState{Command: command})
	if flowSpecLookup.Load() != owner {
		t.Fatal("an announcement must never replace the RIB's own registration")
	}
}

// TestAnswerFlowSpecRefusesWhatItCannotAnswer proves the RIB's side refuses a
// request it cannot answer truthfully rather than answering a default.
//
// VALIDATES: an oversized batch, an unknown operation, and a process holding
// no lookup (or only an announced one) are each refused.
// PREVENTS: a reader receiving "not eligible" from a process that never held
// the lookup, which reads as a RIB verdict.
func TestAnswerFlowSpecRefusesWhatItCannotAnswer(t *testing.T) {
	t.Cleanup(func() { RegisterFlowSpecLookup(nil, nil, nil, nil) })
	encode := func(request FlowSpecRequest) string {
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	RegisterFlowSpecLookup(nil, nil, nil, nil)
	if _, err := AnswerFlowSpec(encode(FlowSpecRequest{Routes: true})); err == nil {
		t.Fatal("a process with no lookup must refuse")
	}
	NoteFlowSpecGate(FlowSpecGateState{Command: "request test rib flowspec"})
	if _, err := AnswerFlowSpec(encode(FlowSpecRequest{Routes: true})); err == nil {
		t.Fatal("a process holding only an announced lookup must refuse")
	}
	RegisterFlowSpecLookup(func(ValidationRoute, uint64) bool { return true }, nil, nil, func() []ValidationRoute { return nil })
	if _, err := AnswerFlowSpec(encode(FlowSpecRequest{Queries: []FlowSpecQuery{{Operation: "bogus"}}})); err == nil {
		t.Fatal("an unknown operation must refuse")
	}
	if _, err := AnswerFlowSpec(encode(FlowSpecRequest{Queries: make([]FlowSpecQuery, EligibilityBatchMax+1)})); err == nil {
		t.Fatal("a batch above EligibilityBatchMax must refuse")
	}
}

// TestDecodeFlowSpecGateAndGateQuery proves both new events are told from
// every other delivered event.
func TestDecodeFlowSpecGateAndGateQuery(t *testing.T) {
	envelope := func(event, payload string) string {
		b, err := json.Marshal(rpc.EventEnvelope{Namespace: Namespace, Event: event, Payload: json.RawMessage(payload)})
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	state, matched, err := DecodeFlowSpecGate(envelope(EventFlowSpecGate, `{"command":"request bgp rib flowspec"}`))
	if err != nil || !matched || state.Command != "request bgp rib flowspec" {
		t.Fatalf("flowspec-gate = %+v, %v, %v", state, matched, err)
	}
	if _, matched, _ := DecodeFlowSpecGate(envelope(EventValidationGate, `{}`)); matched {
		t.Fatal("validation-gate matched as flowspec-gate")
	}
	if !IsGateQuery(envelope(EventGateQuery, `{}`)) {
		t.Fatal("gate-query not matched")
	}
	if IsGateQuery(envelope(EventFlowSpecGate, `{}`)) || IsGateQuery("peer 192.0.2.1 up") {
		t.Fatal("another event matched as gate-query")
	}
}

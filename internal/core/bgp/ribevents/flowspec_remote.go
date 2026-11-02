// Design: docs/architecture/plugin/rib-storage-design.md -- FlowSpec authorization across the process boundary
// Related: flowspec.go -- the in-process lookup this file reaches when it runs in another process
// Related: gate_remote.go -- the receive gate's command, whose dispatch and bounds this file shares
package ribevents

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/ze-software/ze/internal/core/events"
)

// EventFlowSpecGate announces the command the selecting RIB serves for its
// FlowSpec lookup (RegisterFlowSpecLookup and RegisterFlowSpecRedirectLookup).
// The RIB emits it once its own subscriptions are live and again on every
// EventGateQuery, on both transports, so a process that does not hold the
// lookup (a forked bgp-rib's readers, or a forked reader of an in-daemon
// bgp-rib) asks the RIB instead of reading "no FlowSpec route".
const EventFlowSpecGate = "flowspec-gate"

// FlowSpecGateState is the EventFlowSpecGate payload. Command is the command
// the RIB serves (FlowSpecRequest in, FlowSpecAnswer out); the RIB declares
// it, so no reader names the plugin. Empty means the RIB stopped serving, and
// a process without the lookup then answers as with no RIB: no FlowSpec route
// is eligible, present or replayable.
type FlowSpecGateState struct {
	Command string `json:"command,omitempty"`
}

// FlowSpecGate is the typed handle of EventFlowSpecGate.
var FlowSpecGate = events.Register[FlowSpecGateState](Namespace, EventFlowSpecGate)

// EventGateQuery asks every gate owner to announce its state again
// (EventValidationGate, EventFlowSpecGate). A subscription delivers future
// events only, so a process whose subscription went live after an owner
// announced would otherwise never learn the gate: it emits this once its own
// subscriptions are live. An owner announces after its subscription to this
// event is live, so one of the two always reaches the other.
const EventGateQuery = "gate-query"

// GateQueryRequest is the EventGateQuery payload. It carries nothing: the
// event itself is the question.
type GateQueryRequest struct{}

// GateQuery is the typed handle of EventGateQuery.
var GateQuery = events.Register[GateQueryRequest](Namespace, EventGateQuery)

// DecodeFlowSpecGate reads an enveloped delivered EventFlowSpecGate, as
// DecodeValidationGate reads its event.
func DecodeFlowSpecGate(delivered string) (state FlowSpecGateState, matched bool, err error) {
	payload, matched := envelopedPayload(delivered, EventFlowSpecGate)
	if !matched {
		return FlowSpecGateState{}, false, nil
	}
	if err := json.Unmarshal(payload, &state); err != nil {
		return FlowSpecGateState{}, true, err
	}
	return state, true, nil
}

// ApplyFlowSpecGate is a reader's whole handling of a delivered event: when
// delivered is an EventFlowSpecGate it applies it (NoteFlowSpecGate) and
// reports matched; err refuses a payload that does not decode.
func ApplyFlowSpecGate(delivered string) (matched bool, err error) {
	state, matched, err := DecodeFlowSpecGate(delivered)
	if err != nil {
		return true, err
	}
	if matched {
		NoteFlowSpecGate(state)
	}
	return matched, nil
}

// IsGateQuery reports whether delivered is an enveloped EventGateQuery.
func IsGateQuery(delivered string) bool {
	_, matched := envelopedPayload(delivered, EventGateQuery)
	return matched
}

// FlowSpecOperation names what one FlowSpecQuery asks. It is a wire string
// because the query crosses the process boundary as JSON.
type FlowSpecOperation string

// The FlowSpec lookup operations, one per in-process lookup function.
const (
	FlowSpecEligible FlowSpecOperation = "eligible" // RouteEligible's FlowSpec half
	FlowSpecPresent  FlowSpecOperation = "present"  // RoutePresent
	FlowSpecPathOf   FlowSpecOperation = "path"     // LookupFlowSpecPath
	FlowSpecRedirect FlowSpecOperation = "redirect" // FlowSpecRedirectRefused
)

// FlowSpecQuery is one question to the selecting RIB's FlowSpec lookup.
type FlowSpecQuery struct {
	Operation FlowSpecOperation `json:"operation"`
	Route     ValidationRoute   `json:"route"`
	MsgID     uint64            `json:"msg-id,omitempty"`
}

// FlowSpecRequest is the one argument of the RIB's FlowSpec command, as
// compact JSON. Routes asks for FlowSpecRoutes beside the queries.
type FlowSpecRequest struct {
	Queries []FlowSpecQuery `json:"queries,omitempty"`
	Routes  bool            `json:"routes,omitempty"`
}

// FlowSpecResult answers one FlowSpecQuery: Verdict for eligible, present
// and redirect, Verdict and Path for path (Verdict false: no current path).
type FlowSpecResult struct {
	Verdict bool         `json:"verdict"`
	Path    FlowSpecPath `json:"path"`
}

// FlowSpecAnswer is the data the FlowSpec command answers: one result per
// query in query order, and the authorized routes when the request asked.
type FlowSpecAnswer struct {
	Results []FlowSpecResult  `json:"results"`
	Routes  []ValidationRoute `json:"routes,omitempty"`
}

// FlowSpecRoutesMax bounds the routes one FlowSpec answer carries. A route
// encodes to about 200 bytes, so a full answer stays near 3 MB, below
// rpc.MaxMessageSize. An owner holding more refuses the request, and the
// reader replays nothing (logged) rather than a silently partial table.
const FlowSpecRoutesMax = 16384

// NoteFlowSpecGate applies an EventFlowSpecGate announcement to this process.
// A process that holds the RIB's own lookup (RegisterFlowSpecLookup ran here)
// keeps it. A process that holds none asks the RIB through the announced
// command (remoteFlowSpec), and returns to "no RIB" when the command is empty.
// Safe for concurrent use.
func NoteFlowSpecGate(state FlowSpecGateState) {
	current := flowSpecLookup.Load()
	if current != nil && !current.announced {
		return
	}
	if state.Command == "" {
		flowSpecLookup.CompareAndSwap(current, nil)
		return
	}
	flowSpecLookup.CompareAndSwap(current, remoteFlowSpec(state.Command))
}

// remoteFlowSpec is the FlowSpec provider of a process that does not hold the
// RIB's lookup while the RIB, in another process, serves command. Every answer
// it cannot obtain is the refusing one (fail closed): not eligible, not
// present, no path, redirect refused, no routes.
func remoteFlowSpec(command string) *flowSpecProvider {
	provider := &flowSpecProvider{announced: true}
	provider.batch = func(queries []FlowSpecQuery, results []FlowSpecResult) {
		askFlowSpecOwner(command, queries, results)
	}
	ask := func(query FlowSpecQuery) FlowSpecResult {
		results := []FlowSpecResult{{}}
		askFlowSpecOwner(command, []FlowSpecQuery{query}, results)
		return results[0]
	}
	provider.eligible = func(route ValidationRoute, msgID uint64) bool {
		return ask(FlowSpecQuery{Operation: FlowSpecEligible, Route: route, MsgID: msgID}).Verdict
	}
	provider.present = func(route ValidationRoute) bool {
		return ask(FlowSpecQuery{Operation: FlowSpecPresent, Route: route}).Verdict
	}
	provider.path = func(route ValidationRoute) (FlowSpecPath, bool) {
		result := ask(FlowSpecQuery{Operation: FlowSpecPathOf, Route: route})
		return result.Path, result.Verdict
	}
	provider.redirect = func(route ValidationRoute, msgID uint64) bool {
		results := []FlowSpecResult{{Verdict: true}}
		askFlowSpecOwner(command, []FlowSpecQuery{{Operation: FlowSpecRedirect, Route: route, MsgID: msgID}}, results)
		return results[0].Verdict
	}
	provider.routes = func() []ValidationRoute {
		answer, err := askFlowSpecRequest(command, FlowSpecRequest{Routes: true}, 0)
		if err != nil {
			logRemoteRefusal(command, err)
			return nil
		}
		return answer.Routes
	}
	return provider
}

// askFlowSpecOwner answers queries through the RIB's command,
// EligibilityBatchMax at a time. results changes only on a complete answer;
// the caller seeds each result with its refusing value. A redirect query
// whose answer is missing stays refused, which a caller seeds as Verdict true.
func askFlowSpecOwner(command string, queries []FlowSpecQuery, results []FlowSpecResult) {
	for start := 0; start < len(queries); start += EligibilityBatchMax {
		end := min(start+EligibilityBatchMax, len(queries))
		answer, err := askFlowSpecRequest(command, FlowSpecRequest{Queries: queries[start:end]}, end-start)
		if err != nil {
			logRemoteRefusal(command, err)
			continue
		}
		copy(results[start:end], answer.Results)
	}
}

// askFlowSpecRequest sends one FlowSpec command and checks it answered
// resultCount results.
func askFlowSpecRequest(command string, request FlowSpecRequest, resultCount int) (FlowSpecAnswer, error) {
	dispatch := gateDispatch.Load()
	if dispatch == nil {
		return FlowSpecAnswer{}, errors.New("this process registered no route to the engine's command dispatcher")
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return FlowSpecAnswer{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), GateLookupTimeout)
	defer cancel()
	status, data, err := (*dispatch)(ctx, command, string(payload))
	if err != nil {
		return FlowSpecAnswer{}, err
	}
	if status != gateStatusDone {
		return FlowSpecAnswer{}, errors.New("the FlowSpec command answered status " + status + ", want done")
	}
	var answer FlowSpecAnswer
	if err := json.Unmarshal(data, &answer); err != nil {
		return FlowSpecAnswer{}, err
	}
	if len(answer.Results) != resultCount {
		return FlowSpecAnswer{}, errors.New("the FlowSpec command answered a different number of results than it was asked")
	}
	return answer, nil
}

// AnswerFlowSpec is the RIB's side of the FlowSpec command: it decodes request
// (one FlowSpecRequest) and answers each query from the lookup this process
// registered. It refuses a request above EligibilityBatchMax, an unknown
// operation, a routes answer above FlowSpecRoutesMax, and a process that holds
// no registered lookup (it would answer from another process's copy).
func AnswerFlowSpec(request string) (FlowSpecAnswer, error) {
	var input FlowSpecRequest
	if err := json.Unmarshal([]byte(request), &input); err != nil {
		return FlowSpecAnswer{}, err
	}
	if len(input.Queries) > EligibilityBatchMax {
		return FlowSpecAnswer{}, errors.New("FlowSpec request carries more queries than EligibilityBatchMax")
	}
	provider := flowSpecLookup.Load()
	if provider == nil {
		return FlowSpecAnswer{}, errors.New("this process holds no FlowSpec lookup")
	}
	if provider.announced {
		return FlowSpecAnswer{}, errors.New("this process holds no FlowSpec lookup of its own")
	}
	answer := FlowSpecAnswer{Results: make([]FlowSpecResult, len(input.Queries))}
	if err := provider.answerEach(input.Queries, answer.Results); err != nil {
		return FlowSpecAnswer{}, err
	}
	if !input.Routes {
		return answer, nil
	}
	routes := provider.routes()
	if len(routes) > FlowSpecRoutesMax {
		return FlowSpecAnswer{}, errors.New("the RIB holds more authorized FlowSpec routes than FlowSpecRoutesMax")
	}
	answer.Routes = routes
	return answer, nil
}

// answerEach answers queries from this process's lookup. An operation it does
// not know refuses the whole request: the set is open, because a reader in
// another process may be newer than this RIB.
func (p *flowSpecProvider) answerEach(queries []FlowSpecQuery, results []FlowSpecResult) error {
	for i := range queries {
		query := &queries[i]
		switch query.Operation {
		case FlowSpecEligible:
			results[i].Verdict = p.eligible(query.Route, query.MsgID)
		case FlowSpecPresent:
			results[i].Verdict = p.present != nil && p.present(query.Route)
		case FlowSpecPathOf:
			if p.path == nil {
				return errors.New("this process registered no FlowSpec path lookup")
			}
			results[i].Path, results[i].Verdict = p.path(query.Route)
		case FlowSpecRedirect:
			results[i].Verdict = FlowSpecRedirectRefused(query.Route, query.MsgID)
		default:
			return errors.New("unknown FlowSpec operation " + string(query.Operation))
		}
	}
	return nil
}

// localFlowSpecBatch is the batch of a provider whose lookup runs in this
// process. answerEach refuses only an unknown operation, which no caller in
// this process builds; results then keep their seeded refusing values.
func (p *flowSpecProvider) localFlowSpecBatch(queries []FlowSpecQuery, results []FlowSpecResult) {
	if err := p.answerEach(queries, results); err != nil {
		logRemoteRefusal("in-process FlowSpec lookup", err)
	}
}

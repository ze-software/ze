// Design: docs/architecture/plugin/rib-storage-design.md -- the receive gate across the process boundary
// Related: validation.go -- the in-process lookup this file reaches when it runs in another process
package ribevents

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"time"
)

// EligibilityQuery is one question to the receive gate: RouteEligible for the
// route and msgID, or RoutePresent for the route when Present is set.
type EligibilityQuery struct {
	Route   ValidationRoute `json:"route"`
	MsgID   uint64          `json:"msg-id,omitempty"`
	Present bool            `json:"present,omitempty"`
}

// EligibilityRequest is the one argument of the gate owner's eligibility
// command (ValidationGateState.Command), as compact JSON.
type EligibilityRequest struct {
	Queries []EligibilityQuery `json:"queries"`
}

// EligibilityAnswer is the data the eligibility command answers: one answer
// per query, in query order.
type EligibilityAnswer struct {
	Answers []bool `json:"answers"`
}

// EligibilityBatchMax bounds the queries one eligibility command carries. A
// query encodes to about 200 bytes, so a full batch stays far below
// rpc.MaxMessageSize; a caller with more queries sends several commands.
const EligibilityBatchMax = 1024

// gateStatusDone is the status a gate owner's command answers on success.
const gateStatusDone = "done"

// GateLookupTimeout bounds one eligibility command. A gate owner that does not
// answer in time has its routes refused (fail closed) and logged.
const GateLookupTimeout = 5 * time.Second

// GateDispatch sends one command to the engine and returns its status and
// data: the shape of sdk.Plugin.DispatchCommandArgs without the peer argument.
type GateDispatch func(ctx context.Context, command string, args ...string) (string, json.RawMessage, error)

// gateDispatch is how this process reaches a gate owner in another process.
var gateDispatch atomic.Pointer[GateDispatch]

// RegisterGateDispatch installs this process's route to the gate owner's
// eligibility command and returns its release. Every process that hears
// EventValidationGate (the reactor, a RIB) registers one before it subscribes.
// The first registration wins: in the daemon every registrant reaches the
// same command dispatcher, so which one answers changes nothing. The caller
// MUST call the release at stop; it removes only its own registration.
func RegisterGateDispatch(fn GateDispatch) (release func()) {
	slot := &fn
	gateDispatch.CompareAndSwap(nil, slot)
	return func() { gateDispatch.CompareAndSwap(slot, nil) }
}

// remoteValidation is the provider of a process that does not hold the gate's
// lookup while its owner, in another process, serves command.
func remoteValidation(command string) *validationProvider {
	provider := &validationProvider{announced: true}
	provider.batch = func(queries []EligibilityQuery, answers []bool) {
		askGateOwner(command, queries, answers)
	}
	provider.eligible = func(route ValidationRoute, msgID uint64) bool {
		answers := []bool{false}
		askGateOwner(command, []EligibilityQuery{{Route: route, MsgID: msgID}}, answers)
		return answers[0]
	}
	provider.present = func(route ValidationRoute) bool {
		answers := []bool{false}
		askGateOwner(command, []EligibilityQuery{{Route: route, Present: true}}, answers)
		return answers[0]
	}
	return provider
}

// remoteRefusalLogged rate-limits the failure log to one line per period.
var remoteRefusalLogged atomic.Int64

// remoteRefusalPeriod is the shortest interval between two failure logs.
const remoteRefusalPeriod = time.Minute

// askGateOwner answers queries through the owner's command, EligibilityBatchMax
// at a time. Every answer it cannot obtain is false (fail closed): a route
// RPKI or ASPA never judged is never forwarded or selected.
func askGateOwner(command string, queries []EligibilityQuery, answers []bool) {
	for i := range answers {
		answers[i] = false
	}
	dispatch := gateDispatch.Load()
	if dispatch == nil {
		logRemoteRefusal(command, errors.New("this process registered no route to the engine's command dispatcher"))
		return
	}
	for start := 0; start < len(queries); start += EligibilityBatchMax {
		end := min(start+EligibilityBatchMax, len(queries))
		if err := askGateChunk(*dispatch, command, queries[start:end], answers[start:end]); err != nil {
			logRemoteRefusal(command, err)
		}
	}
}

// askGateChunk sends one command. answers changes only on a complete answer.
func askGateChunk(dispatch GateDispatch, command string, queries []EligibilityQuery, answers []bool) error {
	payload, err := json.Marshal(EligibilityRequest{Queries: queries})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), GateLookupTimeout)
	defer cancel()
	status, data, err := dispatch(ctx, command, string(payload))
	if err != nil {
		return err
	}
	if status != gateStatusDone {
		return errors.New("the eligibility command answered status " + status + ", want done")
	}
	var answer EligibilityAnswer
	if err := json.Unmarshal(data, &answer); err != nil {
		return err
	}
	if len(answer.Answers) != len(queries) {
		return errors.New("the eligibility command answered a different number of verdicts than it was asked")
	}
	copy(answers, answer.Answers)
	return nil
}

func logRemoteRefusal(command string, err error) {
	now := time.Now().UnixNano()
	last := remoteRefusalLogged.Load()
	if last != 0 && now-last < int64(remoteRefusalPeriod) {
		return
	}
	if !remoteRefusalLogged.CompareAndSwap(last, now) {
		return
	}
	gateLogger().Error("routes refused: the receive validation gate in another process did not answer",
		"command", command, "error", err.Error(),
		"fix", "check that the plugin serving "+command+" is running and answers")
}

// AnswerEligibility is the gate owner's side of the eligibility command: it
// decodes request (one EligibilityRequest), answers each query from the
// owner's own lookup, and refuses a request above EligibilityBatchMax.
func AnswerEligibility(request string, eligible ValidationLookup, present func(ValidationRoute) bool) (EligibilityAnswer, error) {
	var input EligibilityRequest
	if err := json.Unmarshal([]byte(request), &input); err != nil {
		return EligibilityAnswer{}, err
	}
	if len(input.Queries) > EligibilityBatchMax {
		return EligibilityAnswer{}, errors.New("eligibility request carries more queries than EligibilityBatchMax")
	}
	provider := validationProvider{eligible: eligible, present: present}
	answer := EligibilityAnswer{Answers: make([]bool, len(input.Queries))}
	provider.answerEach(input.Queries, answer.Answers)
	return answer, nil
}

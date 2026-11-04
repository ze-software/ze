// Design: docs/plugin-development/protocol.md -- The Sets of a filter-update answer.
package reactor

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rawBoundaryTransport supplies explicit on-error while answerTransport records
// the fields each real chain member reads.
type rawBoundaryTransport struct {
	*answerTransport
	onError rpc.OnErrorPolicy
}

func (r *rawBoundaryTransport) FilterDeclaration(plugin, filter string) ([]string, []string, rpc.OnErrorPolicy, bool) {
	reads, sets, _, ok := r.answerTransport.FilterDeclaration(plugin, filter)
	return reads, sets, r.onError, ok
}

// TestMalformedRawMemberUsesOnError proves malformed whole-body answers fail at
// the member, before terminal raw traversal: fail-open reaches the next member
// with the original fields and obeys its decision; fail-closed stops there.
// Live runs count and log the named failure, while display runs only trace it.
func TestMalformedRawMemberUsesOnError(t *testing.T) {
	badTunnel := teCarryBody(teTLV(2, teSub(6, 0, 0, 0, 0, 0, 1, 10, 0, 0)))
	for _, malformed := range []struct {
		name string
		raw  []byte
	}{
		{"withdrawn length", []byte{0xff, 0xff, 0, 0}},
		{"attribute length", []byte{0, 0, 0xff, 0xff}},
		{"mandatory tunnel refusal", badTunnel},
	} {
		for _, direction := range []string{directionImport, directionExport} {
			for _, onError := range []rpc.OnErrorPolicy{rpc.OnErrorAccept, rpc.OnErrorReject} {
				for _, next := range []rpc.FilterAction{rpc.FilterAccept, rpc.FilterReject} {
					for _, display := range []bool{false, true} {
						name := malformed.name + "/" + direction + "/" + onError.String() + "/" + next.String()
						if display {
							name += "/display"
						}
						t.Run(name, func(t *testing.T) {
							transport := &rawBoundaryTransport{onError: onError, answerTransport: &answerTransport{
								reads: []string{rpc.FilterFieldRaw, "local-preference"},
								answer: func(input rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
									if input.Filter == "malformed" {
										return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: malformed.raw}
									}
									if string(input.Fields) != `{"local-preference":100}` {
										t.Errorf("following member reads %s, want received local-preference 100", input.Fields)
									}
									return rpc.FilterUpdateOutput{Action: next}
								},
							}}
							policy := answerPolicy(t, routepolicy.FinalAccept, "malformed", "following")
							chain, ok := policy.Library().(*policychain.Chain)
							if !ok {
								t.Fatal("policy has no compiled chain")
							}
							recorder := &warnRecorder{}
							previous := fwdLogger
							fwdLogger = func() *slog.Logger { return slog.New(recorder) }
							t.Cleanup(func() { fwdLogger = previous })
							var trace []policychain.MemberTrace
							var record func(policychain.MemberTrace)
							if display {
								record = func(step policychain.MemberTrace) { trace = append(trace, step) }
							}
							body := libraryTestBody(100)
							r := &Reactor{filterTransportSeam: transport}
							got := r.runLibraryChain(chain, testWireUpdate(body), body, policyview.Session{},
								libraryIntentContext{}, direction, "raw-boundary", "192.0.2.4", record)
							wantAction := policychain.OutcomeReject
							wantCalls := 1
							if onError == rpc.OnErrorAccept {
								wantCalls = 2
								if next == rpc.FilterAccept {
									wantAction = policychain.OutcomeAccept
								}
							}
							if got.outcome.Action != wantAction || got.outcome.Failed != (onError == rpc.OnErrorReject) || got.failed {
								t.Errorf("outcome=%+v reactor-failed=%v, want action %v member-failed %v", got.outcome, got.failed, wantAction, onError == rpc.OnErrorReject)
							}
							if transport.calls() != wantCalls || got.modifiedPayload != nil {
								t.Errorf("calls=%d rewrite=%x, want calls %d and no rewrite", transport.calls(), got.modifiedPayload, wantCalls)
							}
							if display {
								wantStep := policychain.TraceFailOpen
								if onError == rpc.OnErrorReject {
									wantStep = policychain.TraceFailClosed
								}
								if len(trace) != wantCalls || trace[0].Name != "test:malformed" || trace[0].Step != wantStep || trace[0].Err == nil {
									t.Errorf("trace=%+v, want named raw failure followed according to on-error", trace)
								}
								if chain.Refused() != 0 || len(recorder.messages()) != 0 {
									t.Error("display run counted or logged a route failure")
								}
							} else {
								if chain.Refused() != 1 || len(recorder.warnedPeers()) != 1 {
									t.Fatalf("refused=%d warned peers=%v, want one counted and logged member failure", chain.Refused(), recorder.warnedPeers())
								}
								if !bytes.Contains([]byte(recorder.messages()[0]), []byte("on-error")) {
									t.Error("failure log does not identify the on-error decision")
								}
							}
						})
					}
				}
			}
		}
	}
}

// TestValidRawMemberStaysTerminal proves the positive control: a valid rewrite
// stops the following rejecting member and preserves its exact UPDATE bytes.
func TestValidRawMemberStaysTerminal(t *testing.T) {
	body, rewritten := libraryTestBody(100), libraryTestBody(200)
	transport := &answerTransport{reads: []string{rpc.FilterFieldRaw},
		answer: func(input rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
			if input.Filter == "rewrite" {
				return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rewritten}
			}
			return rpc.FilterUpdateOutput{Action: rpc.FilterReject}
		}}
	policy := answerPolicy(t, routepolicy.FinalAccept, "rewrite", "must-not-run")
	r := &Reactor{filterTransportSeam: transport}
	got := r.runLibraryChain(policy.Library(), testWireUpdate(body), body, policyview.Session{},
		libraryIntentContext{}, directionImport, "raw-boundary", "192.0.2.4", nil)
	if got.outcome.Action != policychain.OutcomeAccept || got.failed || !bytes.Equal(got.modifiedPayload, rewritten) || transport.calls() != 1 {
		t.Fatalf("outcome=%+v failed=%v body=%x calls=%d, want terminal exact rewrite %x", got.outcome, got.failed, got.modifiedPayload, transport.calls(), rewritten)
	}
}

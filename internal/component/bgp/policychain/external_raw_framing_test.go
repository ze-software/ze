// Design: docs/plugin-development/protocol.md -- The Sets of a filter-update answer.
package policychain

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestExternalRawSectionFailureUsesOnError drives the chain without a reactor
// caller, proving the member's framing guard uses the shared wire parser and
// refuses a four-octet body whose withdrawn section extends outside it.
func TestExternalRawSectionFailureUsesOnError(t *testing.T) {
	for _, onError := range []rpc.OnErrorPolicy{rpc.OnErrorAccept, rpc.OnErrorReject} {
		t.Run(onError.String(), func(t *testing.T) {
			later := 0
			caller := &fakeCaller{filters: map[string]fakeFilter{
				"ext:raw": {decl: ExternalDeclaration{Reads: []string{rpc.FilterFieldRaw}, OnError: onError},
					answer: func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
						return &rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: []byte{0xff, 0xff, 0, 0}}, nil
					}},
				"ext:later": {decl: ExternalDeclaration{Reads: []string{"local-preference"}, OnError: rpc.OnErrorReject},
					answer: func(input *rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
						later++
						if string(input.Fields) != `{"local-preference":100}` {
							t.Errorf("following member reads %s, want received local-preference", input.Fields)
						}
						return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil
					}},
			}}
			chain := compileMembers(t, testAttachment, []Member{{Plugin: "ext", Name: "raw"}, {Plugin: "ext", Name: "later"}})
			outcome, sets := runRoute(t, chain, caller, "import", localPref(100))
			want := Outcome{Action: OutcomeReject, Failed: true, Member: "ext:raw"}
			wantLater := 0
			if onError == rpc.OnErrorAccept {
				want = Outcome{Action: OutcomeAccept}
				wantLater = 1
			}
			if outcome != want || later != wantLater || sets.Raw() != nil {
				t.Errorf("outcome=%+v later=%d raw=%x, want %+v later=%d and no raw", outcome, later, sets.Raw(), want, wantLater)
			}
			failures := sets.Failures()
			if chain.Refused() != 1 || len(failures) != 1 {
				t.Fatalf("refused=%d failures=%+v, want one named member failure", chain.Refused(), failures)
			}
			if failures[0].Filter != "ext:raw" || failures[0].OnError != onError || failures[0].Err == nil {
				t.Fatalf("failure=%+v, want raw member and its explicit on-error", failures[0])
			}
		})
	}
}

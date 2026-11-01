package policychain

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rawBody is a whole UPDATE body a raw filter answers: no withdrawn routes,
// no path attributes, the NLRI 10.0.0.0/8.
var rawBody = []byte{0, 0, 0, 0, 8, 10}

// TestExternalRawRewriteTerminal proves AC-27 in a mixed chain: an external
// filter that declares raw and answers a whole-body rewrite ends the run with
// the rewrite (a later member is not judged), the chain's final still
// decides the route, and a rewrite that is undeclared, carries sets, is
// shorter than an UPDATE body, or follows an earlier member's set fails the
// route under on-error with none of its sets applied.
func TestExternalRawRewriteTerminal(t *testing.T) {
	rawDecl := ExternalDeclaration{Reads: []string{"local-preference", rpc.FilterFieldRaw}, Sets: []string{"local-preference"}}
	later := 0
	filters := func(onError rpc.OnErrorPolicy, decl ExternalDeclaration, out rpc.FilterUpdateOutput) map[string]fakeFilter {
		decl.OnError = onError
		return map[string]fakeFilter{
			"ext:raw": {decl: decl, answer: func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) { return &out, nil }},
			"ext:later": {decl: ExternalDeclaration{Reads: []string{"local-preference"}, OnError: rpc.OnErrorReject},
				answer: func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
					later++
					return &rpc.FilterUpdateOutput{Action: rpc.FilterReject}, nil
				}},
			"ext:lp": {decl: ExternalDeclaration{Reads: []string{"local-preference"}, Sets: []string{"local-preference"}, OnError: rpc.OnErrorReject},
				answer: modify(set("local-preference", rpc.FilterSetActionSet, "200"))},
		}
	}
	rewrite := rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rawBody}

	t.Run("terminal", func(t *testing.T) {
		later = 0
		chain := compileMembers(t, testAttachment, []Member{{Name: "raw", Plugin: "ext"}, {Name: "later", Plugin: "ext"}})
		outcome, sets := runRoute(t, chain, &fakeCaller{filters: filters(rpc.OnErrorReject, rawDecl, rewrite)}, "import", localPref(100))
		if outcome.Action != OutcomeAccept || !bytes.Equal(sets.Raw(), rawBody) || later != 0 {
			t.Fatalf("outcome = %+v, later judged %d times; want accept with the rewrite and no later member", outcome, later)
		}
	})
	t.Run("final reject", func(t *testing.T) {
		reg, err := policy.NewRegistry()
		if err != nil {
			t.Fatalf("registry: %v", err)
		}
		chain, err := Compile(testAttachment, reg, []Member{{Name: "raw", Plugin: "ext"}}, policy.FinalReject, Bases{})
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		outcome, sets := runRoute(t, chain, &fakeCaller{filters: filters(rpc.OnErrorReject, rawDecl, rewrite)}, "import", localPref(100))
		if outcome != (Outcome{Action: OutcomeReject}) || sets.Raw() != nil {
			t.Fatalf("outcome = %+v, want the final's reject", outcome)
		}
	})

	cases := []struct {
		name    string
		members []Member
		decl    ExternalDeclaration
		out     rpc.FilterUpdateOutput
		want    string
	}{
		{"raw undeclared", []Member{{Name: "raw", Plugin: "ext"}},
			ExternalDeclaration{Reads: []string{"local-preference"}}, rewrite, "does not declare it reads raw"},
		{"raw beside sets", []Member{{Name: "raw", Plugin: "ext"}}, rawDecl,
			rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rawBody,
				Sets: []rpc.FilterSet{set("local-preference", rpc.FilterSetActionSet, "5")}}, "beside sets"},
		{"raw too short", []Member{{Name: "raw", Plugin: "ext"}}, rawDecl,
			rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: []byte{0, 0, 0}}, "shorter than the 4 octets"},
		{"raw after an earlier set", []Member{{Name: "lp", Plugin: "ext"}, {Name: "raw", Plugin: "ext"}}, rawDecl, rewrite,
			"would discard"},
	}
	for _, tc := range cases {
		for _, onError := range []rpc.OnErrorPolicy{rpc.OnErrorReject, rpc.OnErrorAccept} {
			t.Run(tc.name+" "+onError.String(), func(t *testing.T) {
				chain := compileMembers(t, testAttachment, tc.members)
				caller := &fakeCaller{filters: filters(onError, tc.decl, tc.out)}
				outcome, sets := runRoute(t, chain, caller, "import", localPref(100))
				if chain.Refused() != 1 || sets.Raw() != nil {
					t.Fatalf("refused = %d outcome %+v, want one refusal and no rewrite", chain.Refused(), outcome)
				}
				want := Outcome{Action: OutcomeReject, Failed: true, Member: "ext:raw"}
				if onError == rpc.OnErrorAccept {
					want = Outcome{Action: OutcomeAccept}
				}
				if outcome != want {
					t.Fatalf("outcome = %+v, want %+v", outcome, want)
				}
				if len(tc.members) == 1 && !sets.Folded().Empty() {
					t.Fatalf("a refused rewrite's sets were applied: %+v", sets.Folded())
				}
				if len(tc.members) == 1 {
					_, _, err := chain.judgeExternal(&chain.members[0], viewOf(t, localPref(100)), routeSets(caller))
					if err == nil || !strings.Contains(err.Error(), tc.want) {
						t.Fatalf("err = %v, want it to say %q", err, tc.want)
					}
				}
			})
		}
	}
}

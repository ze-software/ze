package policychain

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// testAttachmentIntents reads prefix and lets a filter set the three named
// intents, so an external member answering them is judged by its reply
// alone.
const testAttachmentIntents = "policychain-test intents"

func init() {
	prefix := policy.FieldPrefix
	policy.MustRegisterAttachment("bgp", testAttachmentIntents, []policy.Field{prefix},
		[]policy.Field{policy.FieldASPath, prefix, policy.FieldFamily})
}

// runIntents runs one external member answering reply over the test UPDATE,
// whose body NLRI is 203.0.113.0/24 at offset 0.
func runIntents(t *testing.T, onError rpc.OnErrorPolicy, reply ...rpc.FilterSet) (*Chain, Outcome, *RunSets) {
	t.Helper()
	caller := &fakeCaller{filters: map[string]fakeFilter{"ext:f": {
		decl: ExternalDeclaration{Reads: []string{"prefix"}, Sets: []string{"as-path", "prefix", "family"},
			OnError: onError},
		answer: modify(reply...),
	}}}
	chain := compileMembers(t, testAttachmentIntents, []Member{{Name: "f", Plugin: "ext"}})
	outcome, sets := runRoute(t, chain, caller, "import")
	return chain, outcome, sets
}

// TestExternalIntentsFoldLikeInProcess proves the named intents an external
// filter answers in sets reach the chain's folded Sets exactly as the Sets
// methods an in-process member calls leave them: prepend count,
// remove-private mode, the body NLRI offset of the named prefix, and the
// stripped family.
func TestExternalIntentsFoldLikeInProcess(t *testing.T) {
	cases := []struct {
		name  string
		reply []rpc.FilterSet
		check func(*Sets) bool
	}{
		{"as-path prepend 2", []rpc.FilterSet{set("as-path", rpc.FilterSetActionPrepend, "2")},
			func(s *Sets) bool { return s.ASPathPrepend() == 2 }},
		{"as-path prepends add up", []rpc.FilterSet{set("as-path", rpc.FilterSetActionPrepend, "2"),
			set("as-path", rpc.FilterSetActionPrepend, "3")},
			func(s *Sets) bool { return s.ASPathPrepend() == 5 }},
		{"remove private", []rpc.FilterSet{set("as-path", rpc.FilterSetActionRemove, `"private"`)},
			func(s *Sets) bool { return s.RemovePrivate() == RemovePrivateStrip }},
		{"remove private, peer AS", []rpc.FilterSet{set("as-path", rpc.FilterSetActionRemove, `"private-peer-as"`)},
			func(s *Sets) bool { return s.RemovePrivate() == RemovePrivatePeerAS }},
		{"prefix remove", []rpc.FilterSet{set("prefix", rpc.FilterSetActionRemove, `"203.0.113.0/24"`)},
			func(s *Sets) bool { return slices.Equal(s.NLRIDrops(NLRISectionAnnounce), []uint16{0}) }},
		{"family remove", []rpc.FilterSet{set("family", rpc.FilterSetActionRemove, `"ipv6/unicast"`)},
			func(s *Sets) bool { return s.StripsFamily(family.IPv6Unicast) && !s.StripsFamily(family.IPv4Unicast) }},
	}
	family.RegisterTestFamilies()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chain, outcome, sets := runIntents(t, rpc.OnErrorReject, tc.reply...)
			if outcome.Action != OutcomeAccept || chain.Refused() != 0 {
				t.Fatalf("outcome = %+v refused %d, want accept", outcome, chain.Refused())
			}
			if !tc.check(sets.Folded()) {
				t.Fatalf("folded sets = %+v, not what the reply asked", sets.Folded())
			}
		})
	}
}

// TestExternalIntentsRefused proves AC-20 for the named intents: a count out
// of 1..32, an unknown remove-private mode, a prefix the route does not
// announce, one with host bits set, a path identifier where the session has
// no ADD-PATH, an unknown family, and an action the field does not take each
// fail the route under on-error, counted, with none of its sets applied.
func TestExternalIntentsRefused(t *testing.T) {
	cases := []struct {
		name  string
		reply rpc.FilterSet
		want  string
	}{
		{"prepend 0", set("as-path", rpc.FilterSetActionPrepend, "0"), "not a count from 1 to 32"},
		{"prepend 33", set("as-path", rpc.FilterSetActionPrepend, "33"), "not a count from 1 to 32"},
		{"prepend not a number", set("as-path", rpc.FilterSetActionPrepend, `"2"`), "not a count from 1 to 32"},
		{"remove-private mode", set("as-path", rpc.FilterSetActionRemove, `"public"`), `"private" or "private-peer-as"`},
		{"as-path set", set("as-path", rpc.FilterSetActionSet, "1"), "takes prepend or remove"},
		{"prefix not announced", set("prefix", rpc.FilterSetActionRemove, `"198.51.100.0/24"`), "announces no NLRI"},
		{"prefix host bits", set("prefix", rpc.FilterSetActionRemove, `"203.0.113.1/24"`), "host bits"},
		{"prefix path-id without ADD-PATH", set("prefix", rpc.FilterSetActionRemove,
			`{"prefix":"203.0.113.0/24","path-id":1}`), "announces no NLRI"},
		{"prefix not a prefix", set("prefix", rpc.FilterSetActionRemove, `"203.0.113.0"`), "not a prefix"},
		{"prefix add", set("prefix", rpc.FilterSetActionAdd, `"203.0.113.0/24"`), "takes remove"},
		{"family unknown", set("family", rpc.FilterSetActionRemove, `"ipv9/unicast"`), "no registered family"},
		{"family add", set("family", rpc.FilterSetActionAdd, `"ipv4/unicast"`), "takes remove"},
	}
	family.RegisterTestFamilies()
	for _, tc := range cases {
		for _, onError := range []rpc.OnErrorPolicy{rpc.OnErrorReject, rpc.OnErrorAccept} {
			t.Run(tc.name+" "+onError.String(), func(t *testing.T) {
				chain, outcome, sets := runIntents(t, onError, set("as-path", rpc.FilterSetActionPrepend, "1"), tc.reply)
				if chain.Refused() != 1 {
					t.Fatalf("refused = %d, want 1", chain.Refused())
				}
				if !sets.Folded().Empty() {
					t.Fatalf("a refused reply's sets were applied: %+v", sets.Folded())
				}
				want := Outcome{Action: OutcomeReject, Failed: true, Member: "ext:f"}
				if onError == rpc.OnErrorAccept {
					want = Outcome{Action: OutcomeAccept}
				}
				if outcome != want {
					t.Fatalf("outcome = %+v, want %+v", outcome, want)
				}
				caller := &fakeCaller{filters: map[string]fakeFilter{"ext:f": {
					decl:   ExternalDeclaration{Reads: []string{"prefix"}, Sets: []string{"as-path", "prefix", "family"}},
					answer: modify(tc.reply),
				}}}
				_, _, err := chain.judgeExternal(&chain.members[0], viewOf(t), routeSets(caller))
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("err = %v, want it to say %q", err, tc.want)
				}
			})
		}
	}
}

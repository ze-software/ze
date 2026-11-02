package policychain

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
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
// no ADD-PATH, a label stack that is empty, disagrees with its entries or
// names a unicast NLRI, an unknown family, and an action the field does not take each
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
		{"prefix labels empty", set("prefix", rpc.FilterSetActionRemove,
			`{"labels":[],"prefix":"203.0.113.0/24"}`), "empty label stack"},
		{"prefix label not its entry", set("prefix", rpc.FilterSetActionRemove,
			`{"labels":[[7,1601]],"prefix":"203.0.113.0/24"}`), "carries 100"},
		{"prefix labels on unicast", set("prefix", rpc.FilterSetActionRemove,
			`{"labels":[[100,1601]],"prefix":"203.0.113.0/24"}`), "announces no NLRI"},
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

// labeledReach is an IPv4 labeled-unicast MP_REACH_NLRI announcing
// 192.0.2.0/24 under label 100 at offset 0, then 10.0.0.0/24 under label 200.
//
// RFC 8277 Section 2.2: "The Length field consists of a single octet.  It specifies the
// length in bits of the remainder of the NLRI field."
//
//	+0 Length(1) +1 Label(3: 20-bit label, TC, S) +4 Prefix(ceil((Length-24)/8))
func labeledReach() []byte {
	value := []byte{0, 1, byte(family.SAFIMPLSLabel), 4, 192, 0, 2, 254, 0}
	value = append(value,
		48, 0x00, 0x06, 0x41, 192, 0, 2, // label 100, S set
		48, 0x00, 0x0c, 0x81, 10, 0, 0, // label 200, S set
	)
	return pathAttr(0x80, attribute.AttrMPReachNLRI, value)
}

// TestExternalPrefixRemoveLabeled proves a prefix remove reaches a
// labeled-unicast route: the value the UPDATE event writes for it
// ({"labels":[[label,entry]],"prefix":...}) drops that one entry of
// MP_REACH_NLRI at its offset, leaving the other entry and the route. A value
// whose label stack differs from the entry's, and a plain prefix string,
// which names a unicast NLRI, name no labeled entry and fail the route.
func TestExternalPrefixRemoveLabeled(t *testing.T) {
	family.RegisterTestFamilies()
	cases := []struct {
		name  string
		value string
		drops []uint16
	}{
		{"labeled shape", `{"labels":[[100,1601]],"prefix":"192.0.2.0/24"}`, []uint16{0}},
		{"second entry", `{"labels":[[200,3201]],"prefix":"10.0.0.0/24"}`, []uint16{7}},
		{"other label", `{"labels":[[101,1617]],"prefix":"192.0.2.0/24"}`, nil},
		{"plain prefix", `"192.0.2.0/24"`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caller := &fakeCaller{filters: map[string]fakeFilter{"ext:f": {
				decl: ExternalDeclaration{Reads: []string{"prefix"}, Sets: []string{"as-path", "prefix", "family"},
					OnError: rpc.OnErrorReject},
				answer: modify(set("prefix", rpc.FilterSetActionRemove, tc.value)),
			}}}
			chain := compileMembers(t, testAttachmentIntents, []Member{{Name: "f", Plugin: "ext"}})
			outcome, sets := runRoute(t, chain, caller, "import", labeledReach())
			if tc.drops == nil {
				want := Outcome{Action: OutcomeReject, Failed: true, Member: "ext:f"}
				if outcome != want || chain.Refused() != 1 {
					t.Fatalf("outcome = %+v refused %d, want %+v refused 1", outcome, chain.Refused(), want)
				}
				return
			}
			if outcome.Action != OutcomeAccept || chain.Refused() != 0 {
				t.Fatalf("outcome = %+v refused %d, want accept", outcome, chain.Refused())
			}
			if got := sets.Folded().NLRIDrops(NLRISectionMPReach); !slices.Equal(got, tc.drops) {
				t.Fatalf("MP_REACH_NLRI drops = %v, want %v", got, tc.drops)
			}
			if got := sets.Folded().NLRIDrops(NLRISectionAnnounce); len(got) != 0 {
				t.Fatalf("body NLRI drops = %v, want none", got)
			}
		})
	}
}

package policychain

import (
	"errors"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// testAttachmentReadIntents carries prefix, family and local-preference and
// lets a filter set only local-preference, as a BGP attachment lets no filter
// set prefix or family: an NLRI subset or a family strip there is admitted by
// what the member reads (D-5), never by a set declaration.
const (
	testAttachmentReadIntents = "policychain-test read-intents"
	typeReadsPrefixDrops      = "policychain-test-reads-prefix-drops"   // reads prefix, sets nothing, drops the first body entry
	typeReadsFamilyStrips     = "policychain-test-reads-family-strips"  // reads family, sets nothing, strips ipv6/unicast
	typeReadsLocalDrops       = "policychain-test-reads-local-drops"    // reads local-preference, sets nothing, drops anyway
	typeReadsLocalStrips      = "policychain-test-reads-local-strips"   // reads local-preference, sets nothing, strips anyway
	typeReadsPrefixSetsLocal  = "policychain-test-reads-prefix-no-sets" // reads prefix, writes local-preference undeclared
)

func init() {
	prefix, fam, lp := policy.FieldPrefix, policy.FieldFamily, policy.FieldLocalPreference
	policy.MustRegisterAttachment("bgp", testAttachmentReadIntents, []policy.Field{prefix, fam, lp}, []policy.Field{lp})
	policy.MustRegisterFilterType(typeReadsPrefixDrops, []policy.Field{prefix}, nil)
	policy.MustRegisterFilterType(typeReadsFamilyStrips, []policy.Field{fam}, nil)
	policy.MustRegisterFilterType(typeReadsLocalDrops, []policy.Field{lp}, nil)
	policy.MustRegisterFilterType(typeReadsLocalStrips, []policy.Field{lp}, nil)
	policy.MustRegisterFilterType(typeReadsPrefixSetsLocal, []policy.Field{prefix}, nil)
	drop := func(string, map[string]any, policy.InstanceResolver) (Evaluator, error) {
		return func(_ *policyview.View, sets *Sets) (Decision, error) {
			return DecisionPass, sets.DropNLRI(NLRISectionAnnounce, 0)
		}, nil
	}
	strip := func(string, map[string]any, policy.InstanceResolver) (Evaluator, error) {
		return func(_ *policyview.View, sets *Sets) (Decision, error) {
			return DecisionPass, sets.StripFamily(family.IPv6Unicast)
		}, nil
	}
	MustRegisterEvaluator(typeReadsPrefixDrops, rpc.OnErrorReject, drop)
	MustRegisterEvaluator(typeReadsFamilyStrips, rpc.OnErrorReject, strip)
	MustRegisterEvaluator(typeReadsLocalDrops, rpc.OnErrorReject, drop)
	MustRegisterEvaluator(typeReadsLocalStrips, rpc.OnErrorReject, strip)
	MustRegisterEvaluator(typeReadsPrefixSetsLocal, rpc.OnErrorReject, func(string, map[string]any, policy.InstanceResolver) (Evaluator, error) {
		return func(_ *policyview.View, sets *Sets) (Decision, error) {
			return DecisionPass, sets.LocalPreference(NumericActionSet, 200)
		}, nil
	})
}

// TestMemberIntentsAdmittedByReads proves an in-process member that reads
// prefix may drop NLRI, and one that reads family may strip a family, at an
// attachment that lets no filter set either field; a member that reads
// neither fails its route doing the same, and reading prefix admits no field
// set. Method: compile one-member chains at testAttachmentReadIntents and run
// the test UPDATE (one body prefix).
func TestMemberIntentsAdmittedByReads(t *testing.T) {
	family.RegisterTestFamilies()
	cases := []struct {
		name     string
		typ      string
		admitted bool
		check    func(*Sets) bool
	}{
		{"prefix reader drops", typeReadsPrefixDrops, true,
			func(s *Sets) bool { return slices.Equal(s.NLRIDrops(NLRISectionAnnounce), []uint16{0}) }},
		{"family reader strips", typeReadsFamilyStrips, true,
			func(s *Sets) bool { return s.StripsFamily(family.IPv6Unicast) }},
		{"local-preference reader drops", typeReadsLocalDrops, false, nil},
		{"local-preference reader strips", typeReadsLocalStrips, false, nil},
		{"prefix reader sets local-preference", typeReadsPrefixSetsLocal, false, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chain := compileMembers(t, testAttachmentReadIntents, []Member{{Name: "f"}},
				policy.Instance{Name: "f", Type: tc.typ})
			outcome, sets := runRoute(t, chain, nil, "import")
			if !tc.admitted {
				want := Outcome{Action: OutcomeReject, Failed: true, Member: "f"}
				if outcome != want || chain.Refused() != 1 {
					t.Fatalf("outcome = %+v refused %d, want %+v refused 1", outcome, chain.Refused(), want)
				}
				return
			}
			if outcome.Action != OutcomeAccept || chain.Refused() != 0 {
				t.Fatalf("outcome = %+v refused %d, want accept", outcome, chain.Refused())
			}
			if !tc.check(sets.Folded()) {
				t.Fatalf("folded sets = %+v, the intent was not applied", sets.Folded())
			}
		})
	}
}

// TestExternalIntentsAdmittedByReads proves a plugin filter that declares it
// reads prefix may answer a prefix remove, and one that reads family a family
// remove, with no set declaration and at an attachment that lets no filter set
// either; a filter reading neither is refused as undeclared. Method: one
// external member at testAttachmentReadIntents, judged by its reply.
func TestExternalIntentsAdmittedByReads(t *testing.T) {
	family.RegisterTestFamilies()
	cases := []struct {
		name     string
		reads    []string
		reply    rpc.FilterSet
		admitted bool
		check    func(*Sets) bool
	}{
		{"prefix reader removes a prefix", []string{"prefix"},
			set("prefix", rpc.FilterSetActionRemove, `"203.0.113.0/24"`), true,
			func(s *Sets) bool { return slices.Equal(s.NLRIDrops(NLRISectionAnnounce), []uint16{0}) }},
		{"family reader removes a family", []string{"family"},
			set("family", rpc.FilterSetActionRemove, `"ipv6/unicast"`), true,
			func(s *Sets) bool { return s.StripsFamily(family.IPv6Unicast) }},
		{"local-preference reader removes a prefix", []string{"local-preference"},
			set("prefix", rpc.FilterSetActionRemove, `"203.0.113.0/24"`), false, nil},
		{"prefix reader removes a family", []string{"prefix"},
			set("family", rpc.FilterSetActionRemove, `"ipv6/unicast"`), false, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caller := &fakeCaller{filters: map[string]fakeFilter{"ext:f": {
				decl:   ExternalDeclaration{Reads: tc.reads, OnError: rpc.OnErrorReject},
				answer: modify(tc.reply),
			}}}
			chain := compileMembers(t, testAttachmentReadIntents, []Member{{Name: "f", Plugin: "ext"}})
			outcome, sets := runRoute(t, chain, caller, "import")
			if !tc.admitted {
				if outcome.Action != OutcomeReject || chain.Refused() != 1 {
					t.Fatalf("outcome = %+v refused %d, want a refused route", outcome, chain.Refused())
				}
				_, _, err := chain.judgeExternal(&chain.members[0], viewOf(t), routeSets(caller))
				if !errors.Is(err, errUndeclaredSet) {
					t.Fatalf("err = %v, want it to wrap errUndeclaredSet", err)
				}
				return
			}
			if outcome.Action != OutcomeAccept || chain.Refused() != 0 {
				t.Fatalf("outcome = %+v refused %d, want accept", outcome, chain.Refused())
			}
			if !tc.check(sets.Folded()) {
				t.Fatalf("folded sets = %+v, the intent was not applied", sets.Folded())
			}
		})
	}
}

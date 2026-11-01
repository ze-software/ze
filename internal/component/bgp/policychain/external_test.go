package policychain

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// testAttachmentLPOnly lets a filter set local-preference and nothing else,
// so an external member that declared med is refused by the attachment.
const testAttachmentLPOnly = "policychain-test lp-only"

func init() {
	lp, med, comm := policy.FieldLocalPreference, policy.FieldMED, policy.FieldCommunity
	policy.MustRegisterAttachment("bgp", testAttachmentLPOnly, []policy.Field{lp, med, comm}, []policy.Field{lp})
}

// fakeFilter is one external filter the fake caller answers for.
type fakeFilter struct {
	decl   ExternalDeclaration
	answer func(input *rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error)
}

// fakeCaller stands for the plugin server: it answers declarations and
// replies per filter and records the fields object each filter was sent.
type fakeCaller struct {
	filters map[string]fakeFilter
	sent    map[string]string
}

func (f *fakeCaller) FilterDeclaration(plugin, filter string) (ExternalDeclaration, bool) {
	ff, ok := f.filters[plugin+":"+filter]
	return ff.decl, ok
}

func (f *fakeCaller) CallFilter(plugin string, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	inputValue, err := route.Input()
	if err != nil {
		return nil, err
	}
	input := &inputValue
	ff, ok := f.filters[plugin+":"+input.Filter]
	if !ok {
		return nil, errors.New("no such filter")
	}
	if f.sent == nil {
		f.sent = map[string]string{}
	}
	f.sent[plugin+":"+input.Filter] = string(input.Fields)
	return ff.answer(input)
}

func modify(sets ...rpc.FilterSet) func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
	return func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
		return &rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: sets}, nil
	}
}

func set(field string, action rpc.FilterSetAction, value string) rpc.FilterSet {
	return rpc.FilterSet{Field: field, Action: action, Value: json.RawMessage(value)}
}

// compileMembers compiles a chain of instance members and external members
// (a Member with Plugin set) at attachment.
func compileMembers(t *testing.T, attachment string, members []Member, instances ...policy.Instance) *Chain {
	t.Helper()
	reg, err := policy.NewRegistry(instances...)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	chain, err := Compile(attachment, reg, members, policy.FinalAccept, Bases{LocalPreference: 100, MED: 0})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return chain
}

func runRoute(t *testing.T, chain *Chain, caller ExternalCaller, direction string, attrs ...[]byte) (Outcome, *RunSets) {
	t.Helper()
	var view policyview.View
	sets := new(RunSets)
	sets.SetRoute(ExternalRoute{Caller: caller, Direction: direction, Peer: "192.0.2.1", PeerAS: 65001})
	view.Reset(update(t, attrs...), policyview.Session{})
	defer view.Clear()
	return chain.Run(&view, sets), sets
}

// TestExternalMemberReadsEarlierSets proves AC-13: an external member is sent
// the local-preference an earlier in-process member set, not the received
// one, and a later external member is sent the med an earlier external member
// set (read-after-set across both kinds).
func TestExternalMemberReadsEarlierSets(t *testing.T) {
	caller := &fakeCaller{filters: map[string]fakeFilter{
		"probe:first": {
			decl:   ExternalDeclaration{Reads: []string{"local-preference"}, Sets: []string{"med"}, OnError: rpc.OnErrorReject},
			answer: modify(set("med", rpc.FilterSetActionSet, "7")),
		},
		"probe:second": {
			decl: ExternalDeclaration{Reads: []string{"med"}, OnError: rpc.OnErrorReject},
			answer: func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
				return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil
			},
		},
	}}
	chain := compileMembers(t, testAttachment,
		[]Member{{Name: "lp200"}, {Name: "first", Plugin: "probe"}, {Name: "second", Plugin: "probe"}},
		policy.Instance{Name: "lp200", Type: typeSetLocal, Config: numericConfig(NumericActionSet, 200)})

	outcome, _ := runRoute(t, chain, caller, "import", localPref(100), med(1))
	if outcome.Action != OutcomeAccept {
		t.Fatalf("outcome = %+v, want accept", outcome)
	}
	if got := caller.sent["probe:first"]; got != `{"local-preference":200}` {
		t.Fatalf("first external member was sent %s, want the set local-preference 200", got)
	}
	if got := caller.sent["probe:second"]; got != `{"med":7}` {
		t.Fatalf("second external member was sent %s, want the med the first one set", got)
	}
}

// TestExternalSetsFoldLikeInProcess proves the op-level half of AC-4 and
// AC-25 for the external path: each set an external filter answers produces
// the same ModAccumulator operations as the in-process member that asks the
// same change, so buildModifiedPayload writes the same UPDATE bytes.
func TestExternalSetsFoldLikeInProcess(t *testing.T) {
	cases := []struct {
		name     string
		instance policy.Instance
		reply    rpc.FilterSet
		declared string
	}{
		{"local-preference set", policy.Instance{Name: "m", Type: typeSetLocal, Config: numericConfig(NumericActionSet, 200)},
			set("local-preference", rpc.FilterSetActionSet, "200"), "local-preference"},
		{"local-preference increment", policy.Instance{Name: "m", Type: typeSetLocal, Config: numericConfig(NumericActionIncrement, 5)},
			set("local-preference", rpc.FilterSetActionIncrement, "5"), "local-preference"},
		{"local-preference decrement", policy.Instance{Name: "m", Type: typeSetLocal, Config: numericConfig(NumericActionDecrement, 5)},
			set("local-preference", rpc.FilterSetActionDecrement, "5"), "local-preference"},
		{"med set", policy.Instance{Name: "m", Type: typeSetMED, Config: numericConfig(NumericActionSet, 9)},
			set("med", rpc.FilterSetActionSet, "9"), "med"},
		{"med increment", policy.Instance{Name: "m", Type: typeSetMED, Config: numericConfig(NumericActionIncrement, 9)},
			set("med", rpc.FilterSetActionIncrement, "9"), "med"},
		{"med decrement", policy.Instance{Name: "m", Type: typeSetMED, Config: numericConfig(NumericActionDecrement, 9)},
			set("med", rpc.FilterSetActionDecrement, "9"), "med"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inProcess := compile(t, policy.FinalAccept, tc.instance)
			_, inSets := run(t, inProcess, update(t, localPref(100), med(20)))
			current := Current{LocalPreference: 100, HasLocalPreference: true, MED: 20, HasMED: true}
			want := applied(t, inSets.Folded(), current)

			caller := &fakeCaller{filters: map[string]fakeFilter{"ext:m": {
				decl:   ExternalDeclaration{Reads: []string{tc.declared}, Sets: []string{tc.declared}, OnError: rpc.OnErrorReject},
				answer: modify(tc.reply),
			}}}
			chain := compileMembers(t, testAttachment, []Member{{Name: "m", Plugin: "ext"}})
			outcome, extSets := runRoute(t, chain, caller, "import", localPref(100), med(20))
			if outcome.Action != OutcomeAccept {
				t.Fatalf("outcome = %+v, want accept", outcome)
			}
			got := applied(t, extSets.Folded(), current)
			if !reflect.DeepEqual(got, want) || len(got) == 0 {
				t.Fatalf("external ops = %+v, in-process ops = %+v; want equal and non-empty", got, want)
			}
		})
	}

	t.Run("community add and remove", func(t *testing.T) {
		inProcess := compile(t, policy.FinalAccept, policy.Instance{Name: "c", Type: typeCommunity})
		_, inSets := run(t, inProcess, update(t))
		want := applied(t, inSets.Folded(), Current{})
		caller := &fakeCaller{filters: map[string]fakeFilter{"ext:c": {
			decl: ExternalDeclaration{Reads: []string{"local-preference"}, Sets: []string{"community"}, OnError: rpc.OnErrorReject},
			answer: modify(set("community", rpc.FilterSetActionAdd, `"65001:100"`),
				set("community", rpc.FilterSetActionRemove, `"65001:1"`)),
		}}}
		chain := compileMembers(t, testAttachment, []Member{{Name: "c", Plugin: "ext"}})
		_, extSets := runRoute(t, chain, caller, "import")
		got := applied(t, extSets.Folded(), Current{})
		if !reflect.DeepEqual(got, want) || len(got) == 0 {
			t.Fatalf("external ops = %+v, in-process ops = %+v; want equal and non-empty", got, want)
		}
	})
}

// TestExternalRefusedSetsFailTheRoute proves AC-6 and AC-20: a reply that
// sets a field the filter did not declare, a declared field the attachment
// does not let a filter set, a field no chain can set, an action the field
// does not take, or a value of the wrong shape fails that route under the
// filter's on-error, counted, with none of the reply's sets applied, even
// those before the refused one.
func TestExternalRefusedSetsFailTheRoute(t *testing.T) {
	cases := []struct {
		name, attachment string
		declared         []string
		reply            []rpc.FilterSet
		want             string
	}{
		{"undeclared field", testAttachment, []string{"local-preference"},
			[]rpc.FilterSet{set("local-preference", rpc.FilterSetActionSet, "1"), set("med", rpc.FilterSetActionSet, "5")}, "did not declare: med"},
		{"attachment refuses the field", testAttachmentLPOnly, []string{"local-preference", "med"},
			[]rpc.FilterSet{set("local-preference", rpc.FilterSetActionSet, "1"), set("med", rpc.FilterSetActionSet, "5")}, "may not set"},
		{"unknown field", testAttachment, []string{"tag"},
			[]rpc.FilterSet{set("tag", rpc.FilterSetActionSet, "5")}, "no BGP chain can set"},
		{"wrong action", testAttachment, []string{"local-preference"},
			[]rpc.FilterSet{set("local-preference", rpc.FilterSetActionAdd, "5")}, "takes set, increment or decrement"},
		{"wrong number shape", testAttachment, []string{"local-preference"},
			[]rpc.FilterSet{set("local-preference", rpc.FilterSetActionSet, `"200"`)}, "not a number"},
		{"number out of range", testAttachment, []string{"med"},
			[]rpc.FilterSet{set("med", rpc.FilterSetActionSet, "4294967296")}, "not a number"},
		{"wrong community shape", testAttachment, []string{"community"},
			[]rpc.FilterSet{set("community", rpc.FilterSetActionAdd, "65001")}, "not a community string"},
	}
	for _, tc := range cases {
		for _, onError := range []rpc.OnErrorPolicy{rpc.OnErrorReject, rpc.OnErrorAccept} {
			t.Run(tc.name+" "+onError.String(), func(t *testing.T) {
				caller := &fakeCaller{filters: map[string]fakeFilter{"ext:f": {
					decl:   ExternalDeclaration{Reads: []string{"local-preference"}, Sets: tc.declared, OnError: onError},
					answer: modify(tc.reply...),
				}}}
				chain := compileMembers(t, tc.attachment, []Member{{Name: "f", Plugin: "ext"}})
				outcome, sets := runRoute(t, chain, caller, "import", localPref(100))
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
				_, _, err := chain.judgeExternal(&chain.members[0], viewOf(t, localPref(100)), routeSets(caller))
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("err = %v, want it to say %q", err, tc.want)
				}
			})
		}
	}
}

func viewOf(t *testing.T, attrs ...[]byte) *policyview.View {
	t.Helper()
	view := new(policyview.View)
	view.Reset(update(t, attrs...), policyview.Session{})
	t.Cleanup(view.Clear)
	return view
}

func routeSets(caller ExternalCaller) *RunSets {
	sets := new(RunSets)
	sets.SetRoute(ExternalRoute{Caller: caller, Direction: "import"})
	return sets
}

// TestExternalAnswersThatFail proves the replies the chain cannot apply fail
// the route under on-error rather than passing it: a filter the plugin does
// not declare, a filter that reads the text update, a text delta, a raw
// rewrite, sets beside accept, an action outside the three, and a call error.
func TestExternalAnswersThatFail(t *testing.T) {
	reply := func(out *rpc.FilterUpdateOutput, err error) func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
		return func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) { return out, err }
	}
	declared := ExternalDeclaration{Reads: []string{"local-preference"}, Sets: []string{"local-preference"}, OnError: rpc.OnErrorReject}
	cases := []struct {
		name   string
		filter fakeFilter
		member string
	}{
		{"undeclared filter", fakeFilter{decl: declared}, "missing"},
		{"text reader", fakeFilter{decl: ExternalDeclaration{OnError: rpc.OnErrorReject}, answer: reply(&rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil)}, "f"},
		{"text delta", fakeFilter{decl: declared, answer: reply(&rpc.FilterUpdateOutput{Action: rpc.FilterModify, Update: "local-preference 5"}, nil)}, "f"},
		{"raw rewrite", fakeFilter{decl: declared, answer: reply(&rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: []byte{0}}, nil)}, "f"},
		{"sets beside accept", fakeFilter{decl: declared, answer: reply(&rpc.FilterUpdateOutput{Action: rpc.FilterAccept,
			Sets: []rpc.FilterSet{set("local-preference", rpc.FilterSetActionSet, "5")}}, nil)}, "f"},
		{"no action", fakeFilter{decl: declared, answer: reply(&rpc.FilterUpdateOutput{}, nil)}, "f"},
		{"call error", fakeFilter{decl: declared, answer: reply(nil, errors.New("timeout"))}, "f"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caller := &fakeCaller{filters: map[string]fakeFilter{"ext:f": tc.filter}}
			chain := compileMembers(t, testAttachment, []Member{{Name: tc.member, Plugin: "ext"}})
			outcome, _ := runRoute(t, chain, caller, "import", localPref(100))
			if outcome.Action != OutcomeReject || !outcome.Failed {
				t.Fatalf("outcome = %+v, want a failed reject", outcome)
			}
		})
	}
}

// TestExternalTeardownImportOnly proves AC-15's teardown half: a teardown
// answer closes the session on import (the outcome carries it and the codes)
// and is ignored on export, where the filter's action stands.
func TestExternalTeardownImportOnly(t *testing.T) {
	caller := &fakeCaller{filters: map[string]fakeFilter{"ext:f": {
		decl: ExternalDeclaration{Reads: []string{"local-preference"}, OnError: rpc.OnErrorReject},
		answer: func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
			return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept, Teardown: true, NotifyCode: 6, NotifySubcode: 2}, nil
		},
	}}}
	chain := compileMembers(t, testAttachment, []Member{{Name: "f", Plugin: "ext"}})
	imported, _ := runRoute(t, chain, caller, "import", localPref(100))
	if !imported.Teardown || imported.Action != OutcomeReject || imported.NotifyCode != 6 || imported.NotifySubcode != 2 {
		t.Fatalf("import outcome = %+v, want a teardown reject with 6/2", imported)
	}
	exported, _ := runRoute(t, chain, caller, "export", localPref(100))
	if exported.Teardown || exported.Action != OutcomeAccept {
		t.Fatalf("export outcome = %+v, want accept with no teardown", exported)
	}
}

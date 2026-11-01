package policy_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/family"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// The condition this file registers from outside match.go, and the
// attachments it is checked at.
const (
	testEvenTagKey    = "test-even-tag"
	testCondOwner     = "test-match-registry"
	testCondTagAttach = "test-match-registry tag"
	testCondBare      = "test-match-registry prefix-only"
)

var testEvenTag = policy.MustRegisterMatchCondition(testEvenTagKey, []policy.Field{policy.FieldTag}, true)

func init() {
	policy.RouteConditions.MustAdd(testEvenTag, func(value any, _ policy.InstanceResolver) (policy.RouteCheck, error) {
		if value != "on" {
			return nil, errors.New("want on")
		}
		return func(r *ribpolicy.Route) (bool, error) { return r.HasTag && r.Tag%2 == 0, nil }, nil
	})
	policy.MustRegisterRouteAttachment(testCondOwner, testCondTagAttach,
		[]policy.Field{policy.FieldPrefix, policy.FieldFamily, policy.FieldTag}, nil)
	policy.MustRegisterRouteAttachment(testCondOwner, testCondBare,
		[]policy.Field{policy.FieldPrefix, policy.FieldFamily}, nil)
}

// TestMatchConditionRegisteredOutside: a condition registered from another
// package is a full member of the match filter type with no edit to the
// framework (AC-38). Method: register test-even-tag from this external test
// package, then ask every reader of the registry about an instance naming
// it: the field answer, the source answer, the load-time chain check at an
// attachment that carries its field and one that does not, and an
// engine-judged chain that runs it on an even and an odd tag.
func TestMatchConditionRegisteredOutside(t *testing.T) {
	instance := map[string]any{testEvenTagKey: "on"}
	if got := policy.MatchFields(instance, false); !slices.Equal(got, []policy.Field{policy.FieldTag}) {
		t.Errorf("MatchFields = %v, want [tag]", got)
	}
	if got := policy.MatchFields(instance, true); !slices.Equal(got, []policy.Field{policy.FieldTag}) {
		t.Errorf("MatchFields source-only = %v, want [tag]", got)
	}
	if !policy.IsSourceConditionKey(testEvenTagKey) {
		t.Error("IsSourceConditionKey did not answer for the registered source condition")
	}
	if policy.IsSourceConditionKey("test-no-such-condition") {
		t.Error("IsSourceConditionKey answered true for a key nobody registered")
	}

	reg, err := policy.NewRegistry(policy.Instance{Name: "EVEN", Type: policy.MatchFilterType, Config: instance})
	if err != nil {
		t.Fatal(err)
	}
	if err := policy.CheckChain(testCondTagAttach, reg, []string{"EVEN"}); err != nil {
		t.Errorf("chain at an attachment carrying tag refused: %v", err)
	}
	err = policy.CheckChain(testCondBare, reg, []string{"EVEN"})
	if err == nil || !strings.Contains(err.Error(), "reads tag") {
		t.Errorf("chain at an attachment without tag: err = %v, want a refusal naming tag", err)
	}

	policyCfg := map[string]any{policy.MatchFilterType: map[string]any{"EVEN": instance}}
	chain, err := policy.CompileRouteChain(testCondTagAttach, []string{"EVEN"}, policy.FinalAccept, policyCfg)
	if err != nil {
		t.Fatalf("compile chain: %v", err)
	}
	even := ribpolicy.Route{Family: family.IPv4Unicast, Tag: 4, HasTag: true}
	odd := ribpolicy.Route{Family: family.IPv4Unicast, Tag: 5, HasTag: true}
	if v := chain.Evaluate(even); v.Action != ribpolicy.ActionAccept {
		t.Errorf("even tag: %v, want accept", v.Action)
	}
	if v := chain.Evaluate(odd); v.Action != ribpolicy.ActionReject {
		t.Errorf("odd tag: %v, want reject", v.Action)
	}
	_, err = policy.CompileRouteMatch(map[string]any{testEvenTagKey: "off"}, false, nil)
	if err == nil || !strings.Contains(err.Error(), "match "+testEvenTagKey) {
		t.Errorf("bad value: err = %v, want a refusal naming the condition", err)
	}
}

// TestMatchConditionRegistrationRefusals: a second registration of one key,
// a second check for one view, and the action leaf's key are Ze defects that
// panic at init (AC-38).
func TestMatchConditionRegistrationRefusals(t *testing.T) {
	mustPanic := func(name string, f func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s: no panic", name)
			}
		}()
		f()
	}
	mustPanic("second key", func() {
		policy.MustRegisterMatchCondition(testEvenTagKey, []policy.Field{policy.FieldTag}, false)
	})
	mustPanic("built-in key twice", func() {
		policy.MustRegisterMatchCondition(policy.FieldFamily.String(), []policy.Field{policy.FieldFamily}, false)
	})
	mustPanic("action key", func() {
		policy.MustRegisterMatchCondition("action", []policy.Field{policy.FieldTag}, false)
	})
	mustPanic("second check", func() {
		policy.RouteConditions.MustAdd(testEvenTag, func(any, policy.InstanceResolver) (policy.RouteCheck, error) {
			return nil, errors.New("unused")
		})
	})
}

// testView is a route view other than ribpolicy.Route, standing for the BGP
// view the BGP side owns.
type testView struct{ tag uint32 }

// TestMatchConditionPerView: one condition is judged on every view that
// added a check for it, and a view that added none refuses an instance
// naming it, naming the condition (D-7: the framework learns no view type).
func TestMatchConditionPerView(t *testing.T) {
	views := policy.NewConditions[testView]("a test view")
	if _, err := policy.CompileMatch(views, map[string]any{testEvenTagKey: "on"}, false, nil); err == nil ||
		!strings.Contains(err.Error(), "match "+testEvenTagKey) || !strings.Contains(err.Error(), "a test view") {
		t.Errorf("view with no check: err = %v, want a refusal naming the condition and the view", err)
	}
	views.MustAdd(testEvenTag, func(any, policy.InstanceResolver) (policy.ConditionCheck[testView], error) {
		return func(v *testView) (bool, error) { return v.tag%2 == 0, nil }, nil
	})
	m, err := policy.CompileMatch(views, map[string]any{testEvenTagKey: "on", "action": "reject"}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rejects, err := m.Rejects(&testView{tag: 2}); err != nil || !rejects {
		t.Errorf("even tag with action reject: rejects %v, err %v", rejects, err)
	}
	if rejects, err := m.Rejects(&testView{tag: 3}); err != nil || rejects {
		t.Errorf("odd tag with action reject: rejects %v, err %v", rejects, err)
	}
}

// TestMatchEvaluationAllocatesNothing: judging a route against a compiled
// match of several conditions allocates nothing (AC-38).
func TestMatchEvaluationAllocatesNothing(t *testing.T) {
	m, err := policy.CompileRouteMatch(map[string]any{
		testEvenTagKey: "on",
		"family":       []any{"ipv4/unicast"},
		"tag":          []any{"4", "6"},
		"metric":       map[string]any{"minimum": "1"},
	}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := ribpolicy.Route{Family: family.IPv4Unicast, Tag: 4, HasTag: true, Metric: 10}
	allocs := testing.AllocsPerRun(100, func() {
		if matches, err := m.Matches(&r); err != nil || !matches {
			t.Fatalf("did not match: %v", err)
		}
	})
	if allocs != 0 {
		t.Errorf("Matches allocated %v times per run, want 0", allocs)
	}
}

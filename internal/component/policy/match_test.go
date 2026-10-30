package policy

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// matchTestProtocol is a route source registered for the protocol condition.
var matchTestProtocol = redistevents.RegisterProtocol("test-match-source")

// TestMatchConditions: each condition, alone, passes a route carrying a
// value it names and fails one that carries another value or none. Method:
// compile one instance per condition and judge a route that matches and
// routes that do not; a route lacking the field never matches, because
// absence is never a match.
func TestMatchConditions(t *testing.T) {
	base := ribpolicy.Route{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.0.0.0/24"), Source: matchTestProtocol}
	with := func(edit func(r *ribpolicy.Route)) ribpolicy.Route {
		r := base
		edit(&r)
		return r
	}
	cases := []struct {
		name  string
		cfg   map[string]any
		match ribpolicy.Route
		miss  []ribpolicy.Route
	}{
		{"family", map[string]any{"family": []any{"ipv4/unicast"}}, base,
			[]ribpolicy.Route{with(func(r *ribpolicy.Route) { r.Family = family.IPv6Unicast })}},
		{"protocol registered", map[string]any{"protocol": []any{"test-match-source"}}, base,
			[]ribpolicy.Route{with(func(r *ribpolicy.Route) { r.Source = 0 })}},
		{"protocol ebgp", map[string]any{"protocol": []any{"ebgp"}},
			with(func(r *ribpolicy.Route) { r.IsBGP, r.IsEBGP = true, true }),
			[]ribpolicy.Route{with(func(r *ribpolicy.Route) { r.IsBGP = true }), base}},
		{"protocol ibgp", map[string]any{"protocol": []any{"ibgp"}},
			with(func(r *ribpolicy.Route) { r.IsBGP = true }),
			[]ribpolicy.Route{with(func(r *ribpolicy.Route) { r.IsBGP, r.IsEBGP = true, true }), base}},
		{"next-hop", map[string]any{"next-hop": []any{"192.0.2.0/24"}},
			with(func(r *ribpolicy.Route) { r.NextHop = netip.MustParseAddr("192.0.2.9") }),
			[]ribpolicy.Route{with(func(r *ribpolicy.Route) { r.NextHop = netip.MustParseAddr("198.51.100.1") }), base}},
		{"interface", map[string]any{"interface": []any{"eth0"}},
			with(func(r *ribpolicy.Route) { r.Interface = "eth0" }),
			[]ribpolicy.Route{with(func(r *ribpolicy.Route) { r.Interface = "eth1" }), base}},
		{"metric range", map[string]any{"metric": map[string]any{"minimum": "10", "maximum": "20"}},
			with(func(r *ribpolicy.Route) { r.Metric = 20 }),
			[]ribpolicy.Route{with(func(r *ribpolicy.Route) { r.Metric = 9 }), with(func(r *ribpolicy.Route) { r.Metric = 21 })}},
		{"metric minimum only", map[string]any{"metric": map[string]any{"minimum": "10"}},
			with(func(r *ribpolicy.Route) { r.Metric = 4294967295 }),
			[]ribpolicy.Route{with(func(r *ribpolicy.Route) { r.Metric = 9 })}},
		{"tag", map[string]any{"tag": []any{"0"}},
			with(func(r *ribpolicy.Route) { r.HasTag = true }),
			[]ribpolicy.Route{base, with(func(r *ribpolicy.Route) { r.Tag, r.HasTag = 1, true })}},
		{"level", map[string]any{"level": []any{"1"}},
			with(func(r *ribpolicy.Route) { r.Level = ribpolicy.ISISLevel1 }),
			[]ribpolicy.Route{base, with(func(r *ribpolicy.Route) { r.Level = ribpolicy.ISISLevel2 })}},
		{"route-type", map[string]any{"route-type": []any{"nssa-2"}},
			with(func(r *ribpolicy.Route) { r.OSPFRouteType = ribpolicy.OSPFNSSA2 }),
			[]ribpolicy.Route{base, with(func(r *ribpolicy.Route) { r.OSPFRouteType = ribpolicy.OSPFExternal2 })}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := CompileRouteMatch(tc.cfg, false, nil)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if !m.Matches(&tc.match) {
				t.Errorf("did not match %+v", tc.match)
			}
			for _, r := range tc.miss {
				if m.Matches(&r) {
					t.Errorf("matched %+v", r)
				}
			}
		})
	}
}

// TestMatchConditionRefusals: a condition value the matcher cannot read is
// refused at compile, naming the condition, never read as a match-all.
func TestMatchConditionRefusals(t *testing.T) {
	for _, tc := range []struct {
		key   string
		value any
	}{
		{"family", []any{"ipv9/unicast"}},
		{"protocol", []any{"no-such-source"}},
		{"next-hop", []any{"192.0.2.1"}},
		{"metric", map[string]any{"minimum": "20", "maximum": "10"}},
		{"tag", []any{"many"}},
		{"route-type", []any{"external-3"}},
		{"interface", []any{}},
	} {
		t.Run(tc.key, func(t *testing.T) {
			_, err := CompileRouteMatch(map[string]any{tc.key: tc.value}, false, nil)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), "match "+tc.key) {
				t.Errorf("err %q does not name the condition %s", err, tc.key)
			}
		})
	}
}

// TestMatchAction: the action leaf decides what happens to a route meeting
// every condition. Method: compile the match evaluator over tag 4242 with no
// action, accept and reject, and judge a tagged and an untagged route.
// accept (and the default) passes the tagged route and rejects the other;
// reject does the opposite; an unknown action is refused.
func TestMatchAction(t *testing.T) {
	tagged := ribpolicy.Route{Family: family.IPv4Unicast, Tag: 4242, HasTag: true}
	untagged := ribpolicy.Route{Family: family.IPv4Unicast}
	for _, tc := range []struct {
		action         any // nil: the leaf is left out
		rejectTagged   bool
		rejectUntagged bool
	}{
		{nil, false, true},
		{"accept", false, true},
		{"reject", true, false},
	} {
		cfg := map[string]any{"tag": []any{"4242"}}
		if tc.action != nil {
			cfg["action"] = tc.action
		}
		evaluate, err := compileMatchEvaluator("M", cfg, nil)
		if err != nil {
			t.Fatalf("action %v: %v", tc.action, err)
		}
		if got := evaluate(tagged).Reject; got != tc.rejectTagged {
			t.Errorf("action %v: tagged route rejected = %v, want %v", tc.action, got, tc.rejectTagged)
		}
		if got := evaluate(untagged).Reject; got != tc.rejectUntagged {
			t.Errorf("action %v: untagged route rejected = %v, want %v", tc.action, got, tc.rejectUntagged)
		}
	}
	_, err := compileMatchEvaluator("M", map[string]any{"tag": []any{"4242"}, "action": "permit"}, nil)
	if err == nil || !strings.Contains(err.Error(), "match action") {
		t.Fatalf("action permit: err = %v, want a refusal naming the action", err)
	}
	// modify carries the source conditions with no action: a source-only
	// compile never reads the leaf, so a match block cannot invert modify.
	m, err := CompileRouteMatch(map[string]any{"tag": []any{"4242"}, "action": "reject"}, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.Rejects(&tagged) {
		t.Error("a source-only match read the action leaf")
	}
}

package detect

import (
	"net/netip"
	"testing"
)

func polPfx(s string) netip.Prefix { return netip.MustParsePrefix(s) }
func polAddr(s string) netip.Addr  { return netip.MustParseAddr(s) }

// TestPolicyLongestPrefixMatch validates AC-7: a /24 deny inside a /16 allow wins by
// specificity, with no ordering.
func TestPolicyLongestPrefixMatch(t *testing.T) {
	p := &Policy{
		DefaultAction: actionDeny,
		Rules: []PolicyRule{
			{Prefix: polPfx("192.0.0.0/16"), Action: actionAllow, Match: matchDestination, Scope: scopeMitigation},
			{Prefix: polPfx("192.0.2.0/24"), Action: actionDeny, Match: matchDestination, Scope: scopeMitigation},
		},
	}
	// Victim in the /24 -> deny (full handling): neither flag set.
	if out := p.evaluate(polPfx("192.0.2.5/32"), nil); out.Suppress || out.SuppressMitigation {
		t.Errorf("victim in /24 deny: got %+v, want full handling", out)
	}
	// Victim elsewhere in the /16 -> allow at mitigation scope: SuppressMitigation only.
	if out := p.evaluate(polPfx("192.0.9.9/32"), nil); !out.SuppressMitigation || out.Suppress {
		t.Errorf("victim in /16 allow: got %+v, want SuppressMitigation only", out)
	}
}

// TestPolicyTieBreakDenyWins: equal-specificity rules resolve to deny (fail-safe).
func TestPolicyTieBreakDenyWins(t *testing.T) {
	p := &Policy{DefaultAction: actionAllow, Rules: []PolicyRule{
		{Prefix: polPfx("10.0.0.0/24"), Action: actionAllow, Match: matchDestination, Scope: scopeMitigation},
		{Prefix: polPfx("10.0.0.0/24"), Action: actionDeny, Match: matchDestination, Scope: scopeMitigation},
	}}
	if out := p.evaluate(polPfx("10.0.0.5/32"), nil); out.Suppress || out.SuppressMitigation {
		t.Errorf("equal-specificity tie should pick deny (full handling), got %+v", out)
	}
}

// TestPolicyMatchSides validates AC-8 (source/destination/any) and R-7 (all sources
// must be within a source rule).
func TestPolicyMatchSides(t *testing.T) {
	pSrc := &Policy{DefaultAction: actionDeny, Rules: []PolicyRule{
		{Prefix: polPfx("203.0.113.0/24"), Action: actionAllow, Match: matchSource, Scope: scopeDetection},
	}}
	allIn := []netip.Addr{polAddr("203.0.113.7"), polAddr("203.0.113.8")}
	if out := pSrc.evaluate(polPfx("198.51.100.1/32"), allIn); !out.Suppress {
		t.Errorf("all-sources-in source rule should suppress, got %+v", out)
	}
	mixed := []netip.Addr{polAddr("203.0.113.7"), polAddr("8.8.8.8")}
	if out := pSrc.evaluate(polPfx("198.51.100.1/32"), mixed); out.Suppress {
		t.Error("one out-of-prefix source must keep the attack live (no suppress)")
	}
	// At the emit stage (no sources yet) a source rule cannot match.
	if out := pSrc.evaluate(polPfx("198.51.100.1/32"), nil); out.Suppress {
		t.Error("source rule must not match at emit (sources unknown)")
	}

	pDst := &Policy{DefaultAction: actionDeny, Rules: []PolicyRule{
		{Prefix: polPfx("198.51.100.0/24"), Action: actionAllow, Match: matchDestination, Scope: scopeDetection},
	}}
	if out := pDst.evaluate(polPfx("198.51.100.1/32"), nil); !out.Suppress {
		t.Errorf("destination rule should match the victim at emit, got %+v", out)
	}
}

// TestPolicyOutcomeMapping pins the action x scope -> outcome table.
func TestPolicyOutcomeMapping(t *testing.T) {
	cases := []struct {
		action, scope         string
		suppress, suppressMit bool
	}{
		{actionAllow, scopeDetection, true, false},
		{actionAllow, scopeMitigation, false, true},
		{actionDeny, scopeDetection, false, true},
		{actionDeny, scopeMitigation, false, false},
	}
	for _, c := range cases {
		out := outcomeFor(c.action, c.scope)
		if out.Suppress != c.suppress || out.SuppressMitigation != c.suppressMit {
			t.Errorf("%s/%s: got %+v, want suppress=%v suppressMit=%v",
				c.action, c.scope, out, c.suppress, c.suppressMit)
		}
	}
}

// TestPolicyDefaultAction covers the no-match fallbacks including a nil policy.
func TestPolicyDefaultAction(t *testing.T) {
	if out := (&Policy{DefaultAction: actionAllow}).evaluate(polPfx("1.2.3.4/32"), nil); !out.Suppress {
		t.Errorf("default allow, no rules: want suppress, got %+v", out)
	}
	if out := (&Policy{DefaultAction: actionDeny}).evaluate(polPfx("1.2.3.4/32"), nil); out.Suppress || out.SuppressMitigation {
		t.Errorf("default deny, no rules: want full handling, got %+v", out)
	}
	var nilP *Policy
	if out := nilP.evaluate(polPfx("1.2.3.4/32"), nil); out.Suppress || out.SuppressMitigation {
		t.Errorf("nil policy: want full handling (fail-safe deny), got %+v", out)
	}
}

// TestParsePolicyRules validates the config-map parse (rules keyed by prefix, enum
// leaves as strings) and the enum validation.
func TestParsePolicyRules(t *testing.T) {
	m := map[string]any{
		"default-action": "deny",
		"rule": map[string]any{
			"192.0.2.0/24":    map[string]any{"action": "deny", "match": "destination", "scope": "mitigation"},
			"198.51.100.7/32": map[string]any{"action": "allow", "match": "source", "scope": "detection"},
		},
	}
	p, err := parsePolicy(m)
	if err != nil {
		t.Fatal(err)
	}
	if p.DefaultAction != actionDeny {
		t.Errorf("default-action = %q, want deny", p.DefaultAction)
	}
	if len(p.Rules) != 2 {
		t.Fatalf("rules = %d, want 2", len(p.Rules))
	}
	if err := p.validate(); err != nil {
		t.Errorf("validate: %v", err)
	}

	bad := map[string]any{"rule": map[string]any{"not-a-prefix": map[string]any{"action": "deny"}}}
	if _, err := parsePolicy(bad); err == nil {
		t.Error("expected error for invalid prefix")
	}

	badEnum := &Policy{DefaultAction: actionDeny, Rules: []PolicyRule{
		{Prefix: polPfx("10.0.0.0/8"), Action: "nope", Match: matchAny, Scope: scopeMitigation},
	}}
	if err := badEnum.validate(); err == nil {
		t.Error("expected validate error for bad action enum")
	}
}

// Related: policy.go -- the rule words this test reads

package detect

import (
	"net/netip"
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	_ "github.com/ze-software/ze/internal/plugins/ddos/detect/yang" // registers ze-ddos-detect-conf.yang, which declares the policy leaves
)

// TestPolicyWordsMatchTheModel ties the action, match and scope words the
// policy evaluates to the enumerations the rule list declares.
//
// The Go side is the declaration: each word selects an arm of evaluate,
// matches or outcomeFor, which the model cannot hold. A word only the model
// carried would pass commit and fall to a default arm, so the model is the
// copy and this test keeps it honest in both directions, through validate so
// the guard an operator's config meets is the one under test.
//
// VALIDATES: each enumeration under ddos/detect/policy and the words this
// package switches on are one set, and validate accepts every declared word.
// PREVENTS: a rule word an operator commits that the policy never matches on.
func TestPolicyWordsMatchTheModel(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		words []string
	}{
		{"default action", "ddos/detect/policy/default-action", []string{actionAllow, actionDeny}},
		{"rule action", "ddos/detect/policy/rule/action", []string{actionAllow, actionDeny}},
		{"rule match", "ddos/detect/policy/rule/match", []string{matchSource, matchDestination, matchAny}},
		{"rule scope", "ddos/detect/policy/rule/scope", []string{scopeDetection, scopeMitigation}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			declared, err := configyang.EnumValues(c.path)
			if err != nil {
				t.Fatalf("read the enumeration at %s: %v", c.path, err)
			}
			if len(declared) == 0 {
				t.Fatalf("the model declares no value at %s", c.path)
			}
			known := slices.Sorted(slices.Values(c.words))
			if !slices.Equal(declared, known) {
				t.Errorf("the model declares %v at %s, and this package switches on %v", declared, c.path, known)
			}
		})
	}

	// Every declared word, in every position, passes the guard.
	for _, action := range mustEnum(t, "ddos/detect/policy/rule/action") {
		for _, match := range mustEnum(t, "ddos/detect/policy/rule/match") {
			for _, scope := range mustEnum(t, "ddos/detect/policy/rule/scope") {
				p := &Policy{DefaultAction: actionDeny, Rules: []PolicyRule{
					{Prefix: netip.MustParsePrefix("192.0.2.0/24"), Action: action, Match: match, Scope: scope},
				}}
				if err := p.validate(); err != nil {
					t.Errorf("the model declares action %q, match %q, scope %q and validate refuses the rule: %v",
						action, match, scope, err)
				}
			}
		}
	}
}

func mustEnum(t *testing.T, path string) []string {
	t.Helper()
	values, err := configyang.EnumValues(path)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", path, err)
	}
	return values
}

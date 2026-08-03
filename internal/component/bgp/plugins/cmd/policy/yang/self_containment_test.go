package yang

import (
	"strings"
	"testing"
)

// TestPolicyCmdSchemaOwnsPolicyChainTest is the owner half of the
// self-containment invariant: the central show schema must NOT declare
// policy-chain or policy-test, and this package MUST. Removing the BGP
// policy command owner must remove the `show policy chain` and
// `show policy test` surface with no dangling node.
// See ai/rules/plugins.md.
func TestPolicyCmdSchemaOwnsPolicyChainTest(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:policy-chain"`,
		`ze:command "ze-show:policy-test"`,
		"container show",
		"container policy",
		"container chain",
		"container test",
	} {
		if !strings.Contains(ZePolicyCmdYANG, want) {
			t.Errorf("ze-policy-cmd.yang must declare %q so removing the BGP policy command owner removes that surface", want)
		}
	}
}

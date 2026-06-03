package schema

import (
	"strings"
	"testing"
)

// TestPolicyrouteCmdSchemaOwnsShowPolicyRoutes is the owner half of the
// self-containment invariant: the central show schema must NOT declare
// `show policy-routes`, and this package MUST. See ai/rules/plugin-self-containment.md.
func TestPolicyrouteCmdSchemaOwnsShowPolicyRoutes(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:policy-routes"`,
		"container policy-routes",
	} {
		if !strings.Contains(ZePolicyrouteCmdYANG, want) {
			t.Errorf("ze-policyroute-cmd.yang must declare %q so removing the policyroute plugin removes the show policy-routes surface", want)
		}
	}
}

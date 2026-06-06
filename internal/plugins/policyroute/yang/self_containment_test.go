package yang

import (
	"strings"
	"testing"
)

func TestPolicyrouteCmdSchemaOwnsShowPolicyRoutes(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:policy-routes"`,
		"container policy-routes",
	} {
		if !strings.Contains(ZePolicyrouteCmdYANG, want) {
			t.Errorf("ze-policyroute-cmd.yang must declare %q so removing policyroute removes the surface", want)
		}
	}
}

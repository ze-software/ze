package policyroute

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestPolicyRegistration(t *testing.T) {
	reg := registry.Lookup("policy-routes")
	if reg == nil {
		t.Fatal("policy-routes plugin not registered")
	}
	if reg.Name != "policy-routes" {
		t.Errorf("name = %q, want policy-routes", reg.Name)
	}
	if reg.RunEngine == nil {
		t.Error("RunEngine is nil")
	}
	// Policy routing owns only the route list under the shared policy root,
	// so a policy block holding only filter instances does not load it.
	if len(reg.ConfigRoots) != 1 || reg.ConfigRoots[0] != "policy/route" {
		t.Errorf("ConfigRoots = %v, want [policy/route]", reg.ConfigRoots)
	}
}

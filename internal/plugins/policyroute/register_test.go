package policyroute

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
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
	if len(reg.ConfigRoots) != 1 || reg.ConfigRoots[0] != "policy" {
		t.Errorf("ConfigRoots = %v, want [policy]", reg.ConfigRoots)
	}
}

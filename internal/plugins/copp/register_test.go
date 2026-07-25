package copp

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestCoppRegistration(t *testing.T) {
	reg := registry.Lookup("copp")
	if reg == nil {
		t.Fatal("copp plugin not registered")
	}
	if reg.Name != "copp" {
		t.Errorf("name = %q, want copp", reg.Name)
	}
	if reg.RunEngine == nil {
		t.Error("RunEngine is nil")
	}
	if len(reg.ConfigRoots) != 1 || reg.ConfigRoots[0] != "control-plane-protection" {
		t.Errorf("ConfigRoots = %v, want [control-plane-protection]", reg.ConfigRoots)
	}
}

func TestCoppRegistrationHasDoctorCheck(t *testing.T) {
	reg := registry.Lookup("copp")
	if reg == nil {
		t.Fatal("copp plugin not registered")
	}
	if len(reg.DoctorChecks) == 0 {
		t.Error("DoctorChecks is empty, want at least 1")
	}
	found := false
	for _, dc := range reg.DoctorChecks {
		if dc.Name == "copp-input-chain" {
			found = true
			break
		}
	}
	if !found {
		t.Error("doctor check copp-input-chain not found")
	}
}

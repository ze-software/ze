//go:build linux || darwin

// Design: docs/guide/vpp.md -- original startup-file doctor ownership
// Related: register_managed_startup.go -- live registry enrollment
package vpp

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// The production registry must retain the original-file check with the actual
// resources it reads. Removing dependencies must still be refused by the same
// registry rather than disabling the check or relaxing registration validation.
func TestManagedStartupCLIDoctorRegistryDeclaresOriginalResources(t *testing.T) {
	var registered *diagnostic.DoctorCheck
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name == "vpp-startup-cli-file" {
			registered = &check
			break
		}
	}
	if registered == nil {
		t.Fatal("production registry omitted original startup-file doctor")
	}
	if registered.Component != componentVPP {
		t.Fatal("startup-file check lost its owning component", registered.Component)
	}
	if !slices.Equal(registered.Dependencies, []string{"config-tree", "filesystem"}) {
		t.Fatal("startup-file check lost its original resource dependencies", registered.Dependencies)
	}
	if !slices.Equal(registered.Codes, []string{doctorVPPStartupCLIFileCode}) {
		t.Fatal("startup-file check lost its original diagnostic code", registered.Codes)
	}
	if diagnostic.Lookup(doctorVPPStartupCLIFileCode) == nil {
		t.Fatal("registered startup-file code has no operator explanation")
	}
	invalid := *registered
	invalid.Name = "vpp-startup-cli-file-missing-resource-control"
	invalid.Dependencies = nil
	if err := diagnostic.RegisterDoctorCheck(invalid); err == nil {
		t.Cleanup(func() { diagnostic.UnregisterDoctorCheckForTest(invalid.Name) })
		t.Fatal("registry accepted omitted resource dependencies")
	}
}

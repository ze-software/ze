// Design: docs/research/vpp-deployment-reference.md -- original NIC custody
// Related: dpdk.go -- per-device admission before another native bind attempt
package vpp

import (
	"errors"
	"maps"
	"testing"
)

// Pending original custody must refuse another admission before the original
// VFIO attachment can be mistaken for borrowed state. Exercise the actual
// per-device producer so BindAll's rollback cannot mask an admission mutation.
func TestDPDKCleanupPendingCustodyRefusesReadmission(t *testing.T) {
	fixture := newDPDKCleanupFixture(t)
	fixture.bind(t)
	fixture.unbindFailure = errors.New("original VFIO detach refused")
	if err := fixture.binder.unbindAllWithOps(fixture.ops); err == nil {
		t.Fatal("precondition: original native cleanup did not fail")
	}
	if fixture.unbindCalls != 1 || fixture.driver != "vfio-pci" {
		t.Fatal("precondition: failed detach did not retain the useful VFIO attachment")
	}
	original := maps.Clone(fixture.binder.savedDrivers)
	if len(original) != 1 {
		t.Fatal("precondition: failed cleanup lost its original custody")
	}
	ops := fixture.ops
	ops.currentDriver = func(string) (string, error) {
		t.Fatal("pending custody re-admission reached another native binding attempt")
		return "", errors.New("native admission boundary is unreachable")
	}
	if err := fixture.binder.bindPCIWithOps(dpdkCleanupPCI, ops); err == nil {
		t.Fatal("pending original custody was admitted as borrowed VFIO state")
	}
	if !maps.Equal(original, fixture.binder.savedDrivers) {
		t.Fatal("refused re-admission changed original cleanup custody")
	}
	if fixture.unbindCalls != 1 || fixture.driver != "vfio-pci" {
		t.Fatal("refused re-admission changed the retained native attachment")
	}
}

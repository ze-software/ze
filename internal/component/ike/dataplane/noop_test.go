// VALIDATES: the noop dataplane backend registers under "noop", satisfies the
// Dataplane interface with side-effect-free success, and reports no SAs --
// the backend unprivileged control-plane .ci tests select via
// ze.test.ike.dataplane (spec-test-coverage-gaps AC-3). EPERM from real XFRM
// stays fatal by design (child_test.go TestIsXFRMUnsupported); this backend
// is how tests opt out of the kernel instead of weakening that rule.
// PREVENTS: the noop backend silently vanishing from the registry.
package dataplane

import (
	"net"
	"testing"
)

func TestNoopBackendRegistered(t *testing.T) {
	factory, ok := backends["noop"]
	if !ok {
		t.Fatal("noop backend not registered")
	}
	dp, err := factory()
	if err != nil {
		t.Fatalf("noop factory: %v", err)
	}

	if err := dp.InstallSA(SAParams{}); err != nil {
		t.Fatalf("InstallSA: %v", err)
	}
	if err := dp.RemoveSA(1, net.IPv4(127, 0, 0, 1), 50); err != nil {
		t.Fatalf("RemoveSA: %v", err)
	}
	if err := dp.InstallPolicy(SPParams{}); err != nil {
		t.Fatalf("InstallPolicy: %v", err)
	}
	if err := dp.RemovePolicyParams(SPParams{}); err != nil {
		t.Fatalf("RemovePolicyParams: %v", err)
	}
	sas, err := dp.ListSAs(0)
	if err != nil {
		t.Fatalf("ListSAs: %v", err)
	}
	if len(sas) != 0 {
		t.Fatalf("ListSAs = %d entries, want none", len(sas))
	}
	if err := dp.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

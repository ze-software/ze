// VALIDATES: the noop dataplane backend registers under "noop", satisfies the
// Dataplane interface with side-effect-free success, and reports no SAs --
// the backend unprivileged control-plane .ci tests select via
// ze.test.ike.dataplane (spec-test-coverage-gaps AC-3). EPERM from real XFRM
// stays fatal by design (child_test.go TestIsXFRMUnsupported); this backend
// is how tests opt out of the kernel instead of weakening that rule.
// PREVENTS: the noop backend silently vanishing from the registry.
package dataplane

import (
	"errors"
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
	if err := dp.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestNoopListNotSupported pins the fail-closed half of the noop backend.
//
// The write methods succeed because a control-plane test needs an IKEv2
// negotiation to complete without CAP_NET_ADMIN. The READ methods cannot do the
// same: this backend installs nothing, so an empty list with a nil error is
// indistinguishable from a kernel that holds no SAs, and the operator reads
// "nothing is installed" where the truth is "nobody can tell you"
// (ai/rules/evidence.md). ErrNotSupported is the only honest answer.
func TestNoopListNotSupported(t *testing.T) {
	dp := noopDataplane{}

	sas, err := dp.ListSAs(0)
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("ListSAs error = %v, want ErrNotSupported", err)
	}
	if sas != nil {
		t.Fatalf("ListSAs entries = %v, want nil: a refusal must not also look like an answer", sas)
	}

	policies, err := dp.ListPolicies()
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("ListPolicies error = %v, want ErrNotSupported", err)
	}
	if policies != nil {
		t.Fatalf("ListPolicies entries = %v, want nil", policies)
	}
}

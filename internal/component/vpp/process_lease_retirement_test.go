// Design: docs/research/vpp-deployment-reference.md -- physical retirement validation
// Related: process_peer_test.go -- descriptor and witness ownership fixtures
package vpp

import "testing"

func TestVPPPhysicalRetirementValidationIsReadOnly(t *testing.T) {
	witness, descriptor := newPeerTestWitness(t, 801)
	other, otherDescriptor := newPeerTestWitness(t, 802)
	if err := witness.bind(31, 7, make(chan struct{}, 1)); err != nil {
		t.Fatal(err)
	}
	if err := other.bind(31, 7, make(chan struct{}, 1)); err != nil {
		t.Fatal(err)
	}
	lease, err := witness.retain()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lease.Close(); err != nil {
			t.Error(err)
		}
	})
	otherDescriptor.mu.Lock()
	otherDescriptor.dead = true
	otherDescriptor.mu.Unlock()
	wrong, err := other.exit()
	if err != nil {
		t.Fatal(err)
	}
	if !wrong.Covers(31, 7) {
		t.Fatal("negative lacks the matching logical epoch")
	}
	if err := lease.ValidateRetirement(wrong); err == nil {
		t.Fatal("logical epoch validated a different physical peer")
	}
	if err := lease.ValidateRetirement(exitedProcess{connector: 31, generation: 7}); err == nil {
		t.Fatal("logical-only proof validated physical death")
	}
	if err := lease.ValidateRetirement(nil); err == nil {
		t.Fatal("missing proof validated physical death")
	}
	if lease.closed {
		t.Fatal("failed validation released the lease")
	}
	if witness.references != 2 {
		t.Fatal("failed validation decremented physical ownership")
	}
	if descriptor.closes != 0 {
		t.Fatal("failed validation closed the descriptor")
	}
	if proof, err := lease.Exit(); err != nil || proof != nil {
		t.Fatalf("negative changed live observation: %v,%v", proof, err)
	}
	descriptor.mu.Lock()
	descriptor.dead = true
	descriptor.mu.Unlock()
	proof, err := lease.Exit()
	if err != nil {
		t.Fatal(err)
	}
	if proof == nil {
		t.Fatal("matching physical exit produced no sealed proof")
	}
	if err := lease.ValidateRetirement(proof); err != nil {
		t.Fatal(err)
	}
	if lease.closed {
		t.Fatal("successful read-only validation released the lease")
	}
	if witness.references != 2 {
		t.Fatal("successful validation consumed a reference")
	}
	if err := lease.Retire(proof); err != nil {
		t.Fatal(err)
	}
	if err := lease.ValidateRetirement(wrong); err == nil {
		t.Fatal("prior retirement allowed foreign proof")
	}
	if err := lease.ValidateRetirement(proof); err != nil {
		t.Fatal("matching closed lease lost original identity", err)
	}
}
func TestVPPPhysicalRetirementValidationRefusesNilAndZero(t *testing.T) {
	for _, lease := range []*ProcessLease{nil, {}} {
		if err := lease.ValidateRetirement(exitedProcess{connector: 31, generation: 7}); err == nil {
			t.Fatal("invalid lease accepted retirement")
		}
	}
}

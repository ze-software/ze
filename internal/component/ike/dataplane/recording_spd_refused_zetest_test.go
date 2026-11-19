//go:build zetest

package dataplane

import (
	"errors"
	"net"
	"testing"
)

// VALIDATES: the recording-spd-refused backend refuses an operator entry at order 4242
// and nothing else: an operator entry at another order, and a policy of another role
// at 4242, are recorded and listed.
// METHOD: load the backend by its registered name, as runEngine does, install the
// three policies and list what it holds.
// PREVENTS: a .ci whose refusal comes from a backend that refuses every write, which
// would also refuse the entries the test reads back as installed.
func TestRecordingSPDRefusedBackendRefusesOneOrder(t *testing.T) {
	if err := Load("recording-spd-refused"); err != nil {
		t.Fatalf("load the backend by name: %v", err)
	}
	backend := Get()
	t.Cleanup(func() {
		if err := CloseBackend(); err != nil {
			t.Errorf("close the backend: %v", err)
		}
	})
	_, local, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	_, remote, err := net.ParseCIDR("198.51.100.0/24")
	if err != nil {
		t.Fatal(err)
	}
	refused := SPParams{Src: local, Dst: remote, Dir: SADirOut, Action: SPActionDiscard,
		Priority: recordingSPDRefusedPriority, Role: PolicyRoleOperator}
	if err := backend.InstallPolicy(refused); !errors.Is(err, errRecordingSPDRefused) {
		t.Fatalf("an operator entry at order %d answered %v, want the backend's refusal", recordingSPDRefusedPriority, err)
	}
	kept := refused
	kept.Priority = 900
	if err := backend.InstallPolicy(kept); err != nil {
		t.Fatalf("an operator entry at order 900 was refused: %v", err)
	}
	other := refused
	other.Dir = SADirIn
	other.Role = PolicyRoleChildSA
	if err := backend.InstallPolicy(other); err != nil {
		t.Fatalf("a Child SA policy at priority %d was refused: %v", recordingSPDRefusedPriority, err)
	}
	listed, err := backend.ListPolicies()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 {
		t.Fatalf("listed %d policies, want the two the backend accepted: %+v", len(listed), listed)
	}
}

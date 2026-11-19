//go:build integration && linux

// Design: docs/research/vpp-deployment-reference.md -- actual cleanup-peer equality
// Related: process_managed_integration_linux_test.go -- owned child/socket fixtures
package vpp

import "testing"

// The existing fixture captures SO_PEERPIDFD from the connected socket; this
// helper binds that actual witness and retains its normal production lease.
// Test cleanup MUST close the lease before the fixture releases its witness.
func actualSocketEqualityLease(t *testing.T, socket string, connector, generation uint64) *ProcessLease {
	t.Helper()
	peer := captureManagedFixtureSocket(t, socket)
	if err := peer.bind(connector, generation, make(chan struct{}, 1)); err != nil {
		t.Fatal(err)
	}
	lease, err := peer.retain()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lease.Close(); err != nil {
			t.Error(err)
		}
	})
	return lease
}

// Two socket captures of one actual child compare equal across transport
// epochs; another live child is known different before either PID can be reused.
func TestVPPPhysicalLeaseActualSocketPeers(t *testing.T) {
	t.Chdir(t.TempDir())
	first := startManagedPeerFixture(t, "first.sock", false)
	second := startManagedPeerFixture(t, "second.sock", false)
	if first.cmd.Process.Pid == second.cmd.Process.Pid {
		t.Fatal("independent child fixture reused a live process")
	}
	original := actualSocketEqualityLease(t, "first.sock", 101, 7)
	reconnected := actualSocketEqualityLease(t, "first.sock", 102, 8)
	foreign := actualSocketEqualityLease(t, "second.sock", 101, 7)
	same, err := original.SameProcess(reconnected)
	if err != nil {
		t.Fatal(err)
	}
	if !same {
		t.Fatal("two actual sockets of one child lost physical equality")
	}
	same, err = original.SameProcess(foreign)
	if err != nil {
		t.Fatal(err)
	}
	if same {
		t.Fatal("another actual child inherited the original socket peer")
	}
	if err := reconnected.Close(); err != nil {
		t.Fatal(err)
	}
	same, err = original.SameProcess(reconnected)
	if err == nil {
		t.Fatal("closed socket lease returned a known comparison")
	}
	if same {
		t.Fatal("closed socket lease remained a cleanup witness")
	}
	// Actual child death, rather than a manual ProcessExit value, must refuse
	// further same-process observations while old socket leases remain retained.
	if err := first.stop(); err != nil {
		t.Fatal(err)
	}
	same, err = original.SameProcess(original)
	if err == nil {
		t.Fatal("exited actual peer returned a known live comparison")
	}
	if same {
		t.Fatal("exited actual peer remained live equality")
	}
}

// Namespace PIDs are not equality keys. The child is PID1 in its namespace;
// both socket captures still identify its retained kernel process object.
func TestVPPPhysicalLeaseActualPIDNamespace(t *testing.T) {
	t.Chdir(t.TempDir())
	child := startManagedPeerFixture(t, "namespace.sock", true)
	if child.reportedPID != 1 {
		t.Fatal("child did not enter its own PID namespace")
	}
	if child.cmd.Process.Pid == child.reportedPID {
		t.Fatal("fixture did not create different visible PID values")
	}
	original := actualSocketEqualityLease(t, "namespace.sock", 111, 7)
	reconnected := actualSocketEqualityLease(t, "namespace.sock", 112, 8)
	same, err := original.SameProcess(reconnected)
	if err != nil {
		t.Fatal(err)
	}
	if !same {
		t.Fatal("same physical child differed across logical epochs")
	}
}

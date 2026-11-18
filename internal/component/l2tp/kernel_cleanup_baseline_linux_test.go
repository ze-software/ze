// Design: docs/research/l2tpv2-ze-integration.md -- original worker cleanup ownership
//go:build linux

package l2tp

import (
	"errors"
	"testing"
)

var errWorkerDelete = errors.New("original kernel session delete remains unconfirmed")
var errWorkerDescriptor = errors.New("original worker descriptor close remains unconfirmed")

// This baseline drives the EXISTING real setup/teardown producer with native
// I/O substituted by its existing kernelOps seam. No authority map is seeded.
// The later typed companion changes only preparation/cleanup invocation.
func workerCleanupBaseline(t *testing.T) (*kernelWorker, *fakeKernelOps, kernelSetupEvent) {
	t.Helper()
	fake := &fakeKernelOps{}
	worker := newKernelWorker(fake.ops(), make(chan kernelSetupFailed, 4), nil, discardLogger())
	event := setupEvent(700, 701)
	event.session = &L2TPSession{localSID: event.localSID}
	return worker, fake, event
}

func TestKernelCleanupRetainsFailedSessionDeleteOwner(t *testing.T) {
	worker, fake, event := workerCleanupBaseline(t)
	fail := true
	attempts := 0
	worker.ops.sessionDelete = func(uint16, uint16) error {
		attempts++
		if fail {
			return errWorkerDelete
		}
		return nil
	}
	worker.setupSession(event)
	key := sessionKey{event.localTID, event.localSID}
	original := worker.sessions[key]
	if original == nil {
		t.Fatal("actual setup did not retain worker resources")
	}
	t.Cleanup(func() { fail = false; worker.teardownAll() })
	worker.teardownAll()
	if worker.sessions[key] != original {
		t.Fatal("failed session delete discarded original owned resources")
	}
	if worker.tunnels[event.localTID] == nil {
		t.Fatal("failed child delete discarded original tunnel")
	}
	fail = false
	worker.teardownAll()
	if len(worker.sessions) != 0 || len(worker.tunnels) != 0 {
		t.Fatal("confirmed exact retry retained completed roots")
	}
	if attempts != 2 {
		t.Fatal("session delete did not retry exactly its failed original operation")
	}
	if len(fake.closes) != 3 {
		t.Fatal("delete retry repeated already-consumed raw descriptor numbers")
	}
}

func TestKernelCleanupRetainsFailedDescriptorOwnerWithoutRawRetry(t *testing.T) {
	worker, _, event := workerCleanupBaseline(t)
	counts := map[int]int{}
	worker.ops.closeFD = func(fd int) error {
		counts[fd]++
		if fd == 11 {
			return errWorkerDescriptor
		}
		return nil
	}
	worker.setupSession(event)
	key := sessionKey{event.localTID, event.localSID}
	original := worker.sessions[key]
	if original == nil {
		t.Fatal("actual setup did not create descriptor owner")
	}
	worker.teardownAll()
	if worker.sessions[key] != original {
		t.Fatal("failed raw close was forgotten as complete cleanup")
	}
	worker.teardownAll()
	if counts[10] != 1 || counts[11] != 1 || counts[12] != 1 {
		t.Fatal("unknown raw close was retried by recyclable descriptor number")
	}
	if worker.sessions[key] != original {
		t.Fatal("a later sweep invented confirmation for unknown raw close")
	}
	// All descriptor numbers are fake; this unstarted worker owns no OS object
	// or goroutine. The exact retained object is the asserted unknown outcome.
}

func TestKernelCleanupRetainsFailedSetupRollbackOwner(t *testing.T) {
	worker, _, event := workerCleanupBaseline(t)
	setupFailure := errors.New("native PPP setup refused")
	fail := true
	worker.ops.pppSetup = func(kernelSetupEvent) (pppSessionFDs, error) { return pppSessionFDs{}, setupFailure }
	worker.ops.sessionDelete = func(uint16, uint16) error {
		if fail {
			return errWorkerDelete
		}
		return nil
	}
	t.Cleanup(func() { fail = false; worker.teardownAll() })
	worker.setupSession(event)
	key := sessionKey{event.localTID, event.localSID}
	original := worker.sessions[key]
	if original == nil {
		t.Fatal("partial setup lost the kernel session whose rollback delete failed")
	}
	if worker.tunnels[event.localTID] == nil {
		t.Fatal("partial setup lost original tunnel cleanup root")
	}
	fail = false
	worker.teardownAll()
	if len(worker.sessions) != 0 || len(worker.tunnels) != 0 {
		t.Fatal("successful rollback retry did not retire original roots")
	}
}

func TestKernelCleanupConsumesDistinctDescriptorsIncludingZeroOnce(t *testing.T) {
	worker, fake, event := workerCleanupBaseline(t)
	fake.pppSetupResult = &pppSessionFDs{pppoxFD: 0, chanFD: 0, unitFD: 0}
	worker.setupSession(event)
	worker.teardownAll()
	if len(fake.closes) != 1 || fake.closes[0] != 0 {
		t.Fatal("descriptor zero or aliased owned slots did not close exactly once")
	}
	if len(worker.sessions) != 0 || len(worker.tunnels) != 0 {
		t.Fatal("successful aliased descriptor cleanup retained roots")
	}
}

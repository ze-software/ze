// Design: docs/research/vpp-deployment-reference.md -- nonjoining exact exit retirement
// Related: process_peer.go -- descriptor finalization remains with one worker
package vpp

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// The observer releases the final resource itself. No new admission, explicit
// reconciliation, extra worker or caller Close is needed to finalize the fd.
func TestVPPProcessRetireCallbackReleasesFinalLease(t *testing.T) {
	descriptor := &peerTestDescriptor{identity: processIdentity{device: 1, inode: 201}}
	witness, err := newProcessWitness(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	releaseConstructor := sync.OnceValue(witness.release)
	t.Cleanup(func() {
		if err := releaseConstructor(); err != nil {
			t.Error(err)
		}
	})
	slots := make(chan struct{}, 1)
	if err := witness.bind(71, 3, slots); err != nil {
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
	answers := make(chan error, 1)
	name := fmt.Sprintf("test-retire-final-%d", nextConnectorIdentity())
	if err := RegisterProcessExitObserver(name, func(proof ProcessExit) {
		if proof.Covers(71, 3) {
			answers <- lease.Retire(proof)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { processObservers.Lock(); delete(processObservers.byName, name); processObservers.Unlock() })
	if err := releaseConstructor(); err != nil {
		t.Fatal(err)
	}
	descriptor.mu.Lock()
	descriptor.dead = true
	descriptor.mu.Unlock()
	select {
	case err := <-answers:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("final observer retirement joined itself")
	}
	select {
	case <-witness.done:
	case <-time.After(time.Second):
		t.Fatal("retirement did not finalize its original lifecycle worker")
	}
	if descriptor.closes != 1 {
		t.Fatalf("descriptor closes=%d", descriptor.closes)
	}
	if len(slots) != 0 {
		t.Fatal("retirement retained its process slot")
	}
}

// A join already in progress must release the lease mutex before the worker's
// callback asks to retire that same lease; neither operation creates a worker.
func TestVPPProcessCloseAndCallbackRetireDoNotDeadlock(t *testing.T) {
	descriptor := &peerTestDescriptor{identity: processIdentity{device: 1, inode: 202}, dead: true}
	witness, err := newProcessWitness(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	releaseConstructor := sync.OnceValue(witness.release)
	t.Cleanup(func() {
		if err := releaseConstructor(); err != nil {
			t.Error(err)
		}
	})
	if err := witness.bind(72, 5, make(chan struct{}, 1)); err != nil {
		t.Fatal(err)
	}
	lease, err := witness.retain()
	if err != nil {
		t.Fatal(err)
	}
	entered, allow := make(chan struct{}), make(chan struct{})
	var allowOnce sync.Once
	t.Cleanup(func() {
		allowOnce.Do(func() { close(allow) })
		if err := lease.Close(); err != nil {
			t.Error(err)
		}
	})
	answers := make(chan error, 1)
	name := fmt.Sprintf("test-retire-join-%d", nextConnectorIdentity())
	if err := RegisterProcessExitObserver(name, func(proof ProcessExit) {
		if !proof.Covers(72, 5) {
			return
		}
		close(entered)
		<-allow
		answers <- lease.Retire(proof)
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { processObservers.Lock(); delete(processObservers.byName, name); processObservers.Unlock() })
	if err := releaseConstructor(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("exit observer did not start")
	}
	closed := make(chan error, 1)
	go func() { closed <- lease.Close() }()
	// Observe the actual final release rather than depending on scheduling.
	select {
	case <-witness.stop:
	case <-time.After(time.Second):
		t.Fatal("Close did not release its reference")
	}
	allowOnce.Do(func() { close(allow) })
	select {
	case err := <-answers:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("callback blocked behind joining Close")
	}
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not join its notifier")
	}
	if descriptor.closes != 1 {
		t.Fatalf("descriptor closes=%d", descriptor.closes)
	}
}

// A kernel proof carries physical identity across logical epochs, while a
// foreign proof and a managed-wait-only proof cannot retire the new epoch.
func TestVPPProcessRetireRequiresExactPhysicalProof(t *testing.T) {
	witness, _ := newPeerTestWitness(t, 203)
	if err := witness.bind(73, 9, make(chan struct{}, 1)); err != nil {
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
	for _, wrong := range []ProcessExit{
		nil,
		exitedProcess{connector: 73, generation: 7},
		exitedProcess{connector: 73, generation: 7, physical: processIdentity{device: 1, inode: 204}},
	} {
		if err := lease.Retire(wrong); err == nil {
			t.Fatal("uncovered proof released a retained process")
		}
		if _, err := lease.Exit(); err != nil {
			t.Fatalf("refused retirement changed the lease: %v", err)
		}
	}
	proof := exitedProcess{connector: 73, generation: 7, physical: witness.identity}
	if err := lease.Retire(proof); err != nil {
		t.Fatal(err)
	}
	if err := lease.Retire(proof); err != nil {
		t.Fatal(err)
	}
	if _, err := lease.Exit(); err == nil {
		t.Fatal("retired lease remained observable")
	}
}

// Design: docs/research/vpp-deployment-reference.md -- physical process ownership
package vpp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

type peerTestDescriptor struct {
	mu       sync.Mutex
	identity processIdentity
	dead     bool
	failure  error
	closes   int
}

func (d *peerTestDescriptor) Identity() processIdentity { return d.identity }
func (d *peerTestDescriptor) Exited() (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dead, d.failure
}
func (d *peerTestDescriptor) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closes++
	return nil
}

func newPeerTestWitness(t *testing.T, inode uint64) (*processWitness, *peerTestDescriptor) {
	t.Helper()
	descriptor := &peerTestDescriptor{identity: processIdentity{device: 1, inode: inode}}
	witness, err := newProcessWitness(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := witness.release(); err != nil {
			t.Error(err)
		}
	})
	return witness, descriptor
}

// Retain the actual peer beyond a disconnected transport, observe its death,
// and prove that a live replacement cannot grant or consume the old proof.
func TestVPPProcessLeaseSurvivesTransportReplacement(t *testing.T) {
	old, descriptor := newPeerTestWitness(t, 17)
	replacement, replacementDescriptor := newPeerTestWitness(t, 18)
	slots := make(chan struct{}, processWitnessMax)
	if err := old.bind(41, 7, slots); err != nil {
		t.Fatal(err)
	}
	if err := replacement.bind(41, 9, slots); err != nil {
		t.Fatal(err)
	}
	lease, err := old.retain()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lease.Close(); err != nil {
			t.Error(err)
		}
	})
	// Release only a separately retained transport reference; the cleanup
	// retains the fixture's original constructor reference.
	transport, err := old.retain()
	if err != nil {
		t.Fatal(err)
	}
	if err := transport.Close(); err != nil {
		t.Fatal(err)
	}
	if descriptor.closes != 0 {
		t.Fatal("disconnect closed a retained peer")
	}
	if proof, err := lease.Exit(); err != nil || proof != nil {
		t.Fatalf("connection loss manufactured death: %v, %v", proof, err)
	}
	descriptor.mu.Lock()
	descriptor.dead = true
	descriptor.mu.Unlock()
	proof, err := lease.Exit()
	if err != nil {
		t.Fatal(err)
	}
	if proof == nil {
		t.Fatal("kernel-confirmed death has no proof")
	}
	if !proof.Covers(41, 7) {
		t.Fatal("proof lost the original generation")
	}
	if proof.Covers(41, 9) {
		t.Fatal("old proof covered the replacement")
	}
	if replacementDescriptor.dead {
		t.Fatal("observing old death changed replacement")
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
}

// A failed kernel observation remains unknown. It cannot be reclassified as
// process exit, even after the API transport stopped answering.
func TestVPPProcessLeaseObservationFailureIsUnknown(t *testing.T) {
	witness, descriptor := newPeerTestWitness(t, 21)
	if err := witness.bind(42, 3, make(chan struct{}, 1)); err != nil {
		t.Fatal(err)
	}
	lease, err := witness.retain()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lease.Close(); err != nil {
			t.Error(err)
		}
	}()
	want := errors.New("pidfd observation denied")
	descriptor.mu.Lock()
	descriptor.failure = want
	descriptor.mu.Unlock()
	proof, err := lease.Exit()
	if !errors.Is(err, want) {
		t.Fatalf("observation error=%v", err)
	}
	if proof != nil {
		t.Fatal("failed observation manufactured process death")
	}
}

// No generation history is accumulated: only physically retained witnesses
// occupy bounded slots, and their final release immediately frees the slot.
func TestVPPPhysicalProcessBoundReleasesExactlyOnce(t *testing.T) {
	descriptor := &peerTestDescriptor{identity: processIdentity{device: 1, inode: 31}}
	witness, err := newProcessWitness(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	slots := make(chan struct{}, 1)
	if err := witness.bind(43, 1, slots); err != nil {
		t.Fatal(err)
	}
	other, _ := newPeerTestWitness(t, 32)
	if err := other.bind(43, 2, slots); err == nil {
		t.Fatal("retained process bound was ignored")
	}
	lease, err := witness.retain()
	if err != nil {
		t.Fatal(err)
	}
	if err := witness.release(); err != nil {
		t.Fatal(err)
	}
	if descriptor.closes != 0 {
		t.Fatal("released a still-retained descriptor")
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if descriptor.closes != 1 {
		t.Fatalf("descriptor closes=%d", descriptor.closes)
	}
	if len(slots) != 0 {
		t.Fatal("retired process leaked its bounded slot")
	}
	if err := other.bind(43, 2, slots); err != nil {
		t.Fatal(err)
	}
	if _, err := lease.Exit(); err == nil {
		t.Fatal("closed lease observed a recycled descriptor")
	}
}

// Leave only a resource lease after disconnect. No new admission, reconnect,
// native API probe or explicit reconciliation drives the actual exit signal.
func TestVPPProcessExitObserverOutlivesDisconnectedManager(t *testing.T) {
	const childArgument = "vpp-process-exit-observer-child"
	if os.Args[len(os.Args)-1] != childArgument {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, executable,
			"-test.run=^TestVPPProcessExitObserverOutlivesDisconnectedManager$",
			"--", childArgument)
		command.WaitDelay = time.Second
		output, err := command.CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("physical-worker fixture exceeded its owned process bound: %v\n%s", ctx.Err(), output)
		}
		if err != nil {
			t.Fatalf("physical-worker fixture: %v\n%s", err, output)
		}
		return
	}
	descriptor := &peerTestDescriptor{identity: processIdentity{device: 1, inode: 41}}
	witness, err := newProcessWitness(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	slots := make(chan struct{}, 1)
	if err := witness.bind(61, 7, slots); err != nil {
		t.Fatal(err)
	}
	lease, err := witness.retain()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lease.Close(); err != nil {
			t.Error(err)
		}
	}()
	events := make(chan ProcessExit, 1)
	name := fmt.Sprintf("test-physical-peer-%d", nextConnectorIdentity())
	if err := RegisterProcessExitObserver(name, func(proof ProcessExit) {
		if proof.Covers(61, 7) {
			events <- proof
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { processObservers.Lock(); delete(processObservers.byName, name); processObservers.Unlock() })
	if err := witness.release(); err != nil {
		t.Fatal(err)
	}
	descriptor.mu.Lock()
	descriptor.dead = true
	descriptor.mu.Unlock()
	select {
	case proof := <-events:
		if proof.Covers(61, 8) {
			t.Fatal("automatic exit covered a new generation")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("old process death needed new subscriber activity")
	}
	closed := make(chan error, 1)
	go func() { closed <- lease.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("process-exit worker did not join after its final lease")
	}
	select {
	case <-witness.done:
	default:
		t.Fatal("Close returned before joining its physical worker")
	}
	if descriptor.closes != 1 {
		t.Fatal("final owner did not close its descriptor once")
	}
	if len(slots) != 0 {
		t.Fatal("retired manager retained its physical process slot")
	}
}

// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact stopped admission observation
package ppp

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// Identity is the existing native-accessor seam between spawnSession's two
// locked checks. Only the first real admission is paused; no map, worker,
// admitted Control or cleanup result is fabricated.
type stoppedAdmissionIdentityGate struct {
	*genericCleanupLease
	first   atomic.Bool
	entered chan struct{}
	resume  <-chan struct{}
}

func (owner *stoppedAdmissionIdentityGate) Identity() SessionIdentity {
	if owner.first.CompareAndSwap(false, true) {
		owner.entered <- struct{}{}
		<-owner.resume
	}
	return owner.SessionHandle.Identity()
}

// Production dispatch is serial. This deliberately drives the existing direct
// producer seam concurrently with actual dispatch to isolate its second locked
// check, rather than claiming a packet-only race creates two dispatcher calls.
func TestSecondStoppedAdmissionFinishesUnacquiredExistingLeaseAttempt(t *testing.T) {
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	start, _, original := frameLeaseStart(t, 96)
	resume, cancel := context.WithCancel(context.Background())
	owner := &stoppedAdmissionIdentityGate{
		genericCleanupLease: &genericCleanupLease{SessionHandle: original.SessionHandle},
		entered:             make(chan struct{}, 1), resume: resume.Done(),
	}
	owner.fail.Store(true)
	start.Dataplane = owner
	start.SubscriberID = "opaque/paused-before-second-admission-check"
	var err error
	start.Attempt, err = NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	frameLeaseProxy(&start)
	late := start.Attempt
	returned := make(chan struct{}, 1)
	t.Cleanup(func() {
		cancel()
		select {
		case <-returned:
		case <-time.After(time.Second):
			t.Error("paused admission did not finish")
		}
		driver.Stop()
		owner.fail.Store(false)
		if err := driver.RetryPendingNativeRelease(context.Background()); err != nil {
			t.Error(err)
		}
	})
	go func() { driver.spawnSession(&start); returned <- struct{}{} }()
	select {
	case <-owner.entered:
	case <-time.After(time.Second):
		t.Fatal("first transfer did not enter actual identity-accessor window")
	}
	rightful := start
	rightful.SubscriberID = "opaque/rightful-admission"
	rightful.SessionID = 97
	rightful.Attempt, err = NewSessionAttempt(rightful.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	driver.SessionsIn() <- rightful
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	opened, ok := event.(EventLCPUp)
	if !ok {
		t.Fatalf("actual rightful admission=%T", event)
	}
	select {
	case event := <-driver.AuthEventsOut():
		request, ok := event.(EventAuthRequest)
		if !ok || request.Control != opened.Control {
			t.Fatal("actual rightful worker did not retain its provider wait")
		}
	case <-time.After(time.Second):
		t.Fatal("actual rightful worker did not reach provider wait")
	}
	// Complete public Stop while the paused transfer remains before its second
	// check. Failed generic Close truthfully retains the ACTUAL old worker/lease.
	driver.Stop()
	originalStatus, err := opened.Control.CleanupStatus()
	if err != nil || !originalStatus.Retired || originalStatus.Released || originalStatus.Err == nil {
		t.Fatalf("rightful stopped retained owner=%+v,%v", originalStatus, err)
	}
	closes := owner.closes.Load()
	if closes == 0 {
		t.Fatal("public Stop never attempted actual rightful Close")
	}
	driver.mu.Lock()
	retained := driver.owners[owner] == opened.Control.worker
	driver.mu.Unlock()
	if !retained {
		t.Fatal("public Stop did not retain the actual failed rightful owner")
	}
	cancel()
	select {
	case <-returned:
		returned <- struct{}{}
	case <-time.After(time.Second):
		t.Fatal("second stopped check did not finish")
	}
	status, err := late.CleanupStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Admitted || !status.Finished || status.LeaseAcquired || status.Released || status.Err != nil {
		t.Fatalf("second stopped check left exact unacquired attempt unresolved: %+v", status)
	}
	if _, bound := late.Control(); bound {
		t.Fatal("second stopped refusal fabricated admitted Control")
	}
	if owner.closes.Load() != closes {
		t.Fatal("stopped duplicate transfer closed rightful retained lease again")
	}
	bound, ok := rightful.Attempt.Control()
	if !ok || bound != opened.Control {
		t.Fatal("refusal retargeted rightful original admission")
	}
}

// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact generic Close ownership

package ppp

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var errGenericOwnerClose = errors.New("generic session owner cleanup remains unconfirmed")

type genericCleanupLease struct {
	SessionHandle
	peer    net.Conn
	fail    atomic.Bool
	closes  atomic.Int32
	entered chan struct{}
	resume  <-chan struct{}
	once    sync.Once
}

func (owner *genericCleanupLease) Close() error {
	owner.closes.Add(1)
	err := owner.SessionHandle.Close()
	if owner.entered != nil {
		owner.once.Do(func() { close(owner.entered) })
	}
	if owner.resume != nil {
		<-owner.resume
	}
	if owner.fail.Load() {
		return errors.Join(err, errGenericOwnerClose)
	}
	return err
}

func admittedGenericCleanupOwner(t *testing.T) (*Driver, *genericCleanupLease, SessionControl) {
	t.Helper()
	driver, owner, request := admittedGenericCleanupRequest(t)
	return driver, owner, request.Control
}

func admittedGenericCleanupRequest(t *testing.T) (*Driver, *genericCleanupLease, EventAuthRequest) {
	t.Helper()
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	start, peer, original := frameLeaseStart(t, 81)
	owner := &genericCleanupLease{SessionHandle: original.SessionHandle, peer: peer}
	start.Dataplane = owner
	start.SubscriberID = "opaque/generic-cleanup"
	attempt, err := NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	start.Attempt = attempt
	frameLeaseProxy(&start)
	t.Cleanup(func() {
		owner.fail.Store(false)
		driver.Stop()
		if err := driver.RetryPendingNativeRelease(context.Background()); err != nil {
			t.Error(err)
		}
	})
	driver.SessionsIn() <- start
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	opened, valid := event.(EventLCPUp)
	if !valid {
		t.Fatalf("first admitted event=%T, want LCP-up", event)
	}
	// Hold the actual auth wait: the worker has a real admitted Control, and
	// stopping it needs no fabricated provider request or event acknowledgment.
	var observed EventAuthRequest
	select {
	case event := <-driver.AuthEventsOut():
		request, valid := event.(EventAuthRequest)
		if !valid {
			t.Fatalf("auth event=%T, want actual request", event)
		}
		if request.Control != opened.Control {
			t.Fatal("actual auth request changed worker")
		}
		observed = request
	case <-time.After(time.Second):
		t.Fatal("worker did not reach actual auth wait")
	}
	return driver, owner, observed
}

// A non-native backend can fail Close too. Its real retired worker must keep
// the exact generic owner in the EXISTING index until a successful later retry.
func TestDriverRetainsFailedGenericOwnerAfterWorkerExit(t *testing.T) {
	driver, owner, control := admittedGenericCleanupOwner(t)
	owner.fail.Store(true)
	if err := driver.StopSession(control); err != nil {
		t.Fatal(err)
	}
	driver.mu.Lock()
	retained := driver.owners[owner] == control.worker
	driver.mu.Unlock()
	if !retained {
		t.Fatal("retirement forgot the exact generic owner whose Close failed")
	}
	owner.fail.Store(false)
	if err := control.RetryNativeRelease(); err != nil {
		t.Fatal(err)
	}
	driver.mu.Lock()
	_, retained = driver.owners[owner]
	driver.mu.Unlock()
	if retained {
		t.Fatal("confirmed generic retry retained its exact old owner")
	}
}

// A successful first Close already confirmed its owner. Retirement and a
// repeated exact retry must not close that physical lease a second time.
func TestDriverSuccessfulFirstCloseIsNotRepeatedAtRetirement(t *testing.T) {
	driver, owner, control := admittedGenericCleanupOwner(t)
	if err := driver.StopSession(control); err != nil {
		t.Fatal(err)
	}
	if got := owner.closes.Load(); got != 1 {
		t.Fatalf("successful owner Close calls=%d, want one", got)
	}
	if err := control.RetryNativeRelease(); err != nil {
		t.Fatal(err)
	}
	if got := owner.closes.Load(); got != 1 {
		t.Fatalf("completed old retry closed physical owner again: %d", got)
	}
}

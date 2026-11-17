// Design: docs/architecture/l2tp/bng-5-pppoe.md -- first close and explicit retained cleanup retry
// Related: session.go -- ordinary borrowers share a cached first outcome
package pppoe

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/taskguard"
)

type fixtureLeaseRetryOwner struct {
	ppp.SessionHandle
	calls      int
	failure    error
	panicValue error
}

func (owner *fixtureLeaseRetryOwner) Close() error {
	owner.calls++
	if owner.panicValue != nil {
		panic(owner.panicValue)
	}
	return owner.failure
}

// TestSessionLeaseFailureNeedsExplicitRetry preserves one physical first
// Close for borrowers even after the backend recovers; only retryClose starts
// a later owner cleanup attempt. A confirmed success remains idempotent.
func TestSessionLeaseFailureNeedsExplicitRetry(t *testing.T) {
	cause := errors.New("exact unused owner release failed")
	owner := &fixtureLeaseRetryOwner{failure: cause}
	lease := &sessionLease{handle: owner}
	for range 2 {
		if err := lease.Close(); !errors.Is(err, cause) {
			t.Fatal("borrower lost first release failure", err)
		}
	}
	if owner.calls != 1 {
		t.Fatal("ordinary borrowers repeated physical Close", owner.calls)
	}
	owner.failure = nil
	if err := lease.Close(); !errors.Is(err, cause) {
		t.Fatal("ordinary Close hid unconfirmed initial outcome", err)
	}
	if owner.calls != 1 {
		t.Fatal("backend recovery silently authorized another Close")
	}
	if err := lease.retryClose(); err != nil {
		t.Fatal(err)
	}
	if owner.calls != 2 {
		t.Fatal("explicit later attempt did not reach exact original owner", owner.calls)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lease.retryClose(); err != nil {
		t.Fatal(err)
	}
	if owner.calls != 2 {
		t.Fatal("confirmed owner was physically closed again", owner.calls)
	}
}

// TestSessionLeasePanicCachesUnknownOutcome uses the real task boundary.
// The original panic reaches that boundary exactly once; borrowers retain its
// cause until an explicit later same-owner attempt confirms successful Close.
func TestSessionLeasePanicCachesUnknownOutcome(t *testing.T) {
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	cause := errors.New("exact unused owner Close panicked")
	owner := &fixtureLeaseRetryOwner{panicValue: cause}
	lease := &sessionLease{handle: owner}
	guard := taskguard.New("pppoe-lease-retry-test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	task := guard.Task("cleanup", "retain the original owner")
	var seen *taskguard.Panic
	func() {
		defer task.Contain(func() taskguard.Context { return taskguard.Context{Unit: "exact unused lease"} }, func(value *taskguard.Panic) { seen = value })
		if err := lease.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if seen == nil {
		t.Fatal("first Close changed or suppressed original panic", seen)
	}
	// Containment carries an exact panic payload, not an error-chain match.
	if seen.Value != any(cause) {
		t.Fatal("first Close changed or suppressed original panic", seen)
	}
	if err := lease.Close(); !errors.Is(err, cause) {
		t.Fatal("ordinary borrower lost cached panic cause", err)
	}
	if owner.calls != 1 {
		t.Fatal("ordinary borrower repeated panicking physical Close")
	}
	owner.panicValue = nil
	if err := lease.Close(); !errors.Is(err, cause) {
		t.Fatal("recovery silently changed the unknown outcome", err)
	}
	if err := lease.retryClose(); err != nil {
		t.Fatal(err)
	}
	if owner.calls != 2 {
		t.Fatal("explicit later cleanup did not use exact original owner")
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if owner.calls != 2 {
		t.Fatal("confirmed cleanup was repeated")
	}
}

package webtesting

import (
	"errors"
	"testing"
	"time"
)

// TestRetryPositiveSucceedsAfterTransientFailure covers the case the retry
// exists for: the element is not there on the first sample and appears while the
// deadline is still open.
//
// VALIDATES: retryPositive re-evaluates the check and returns nil once it passes.
// PREVENTS: a single-sample assertion reading the DOM before an asynchronous
// htmx swap has started, which is what `action=wait` cannot rule out (its
// idle predicate is true both before a request begins and after it ends).
func TestRetryPositiveSucceedsAfterTransientFailure(t *testing.T) {
	calls := 0
	err := retryPositive(func() error {
		calls++
		if calls < 3 {
			return errors.New("not yet")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retryPositive returned %v, want nil once the check passes", err)
	}
	if calls != 3 {
		t.Fatalf("check called %d times, want 3", calls)
	}
}

// TestRetryPositiveReturnsLastError pins that a genuine failure still fails, and
// that the error the caller sees is the most recent one (so its embedded
// snapshot describes the page as it finally stood, not as it was 5s earlier).
//
// VALIDATES: retryPositive gives up at the deadline and propagates the last error.
// PREVENTS: turning the retry into a way for a broken page to pass.
func TestRetryPositiveReturnsLastError(t *testing.T) {
	want := errors.New("still absent")
	calls := 0
	start := time.Now()
	err := retryPositive(func() error {
		calls++
		return want
	})
	elapsed := time.Since(start)

	if !errors.Is(err, want) {
		t.Fatalf("retryPositive returned %v, want the check's own error", err)
	}
	if calls < 2 {
		t.Fatalf("check called %d times, want it retried at least once", calls)
	}
	if elapsed < expectDeadline {
		t.Fatalf("gave up after %s, want it to poll for the full %s", elapsed, expectDeadline)
	}
}

// TestRetryFetchRetriesOnErrorOnly pins the distinction that makes retryFetch
// safe for NEGATIVE expectations: it loops on the browser command failing, never
// on what the command returned.
//
// VALIDATES: a transient `agent-browser html` exit status is retried, and the
// value it eventually returns is handed back untouched.
// PREVENTS: a tool failure ("html: exit status 1") failing a `not-contains`
// step, which says nothing about the page -- while keeping absence itself
// judged on a single answer.
func TestRetryFetchRetriesOnErrorOnly(t *testing.T) {
	calls := 0
	out, err := retryFetch(func() (string, error) {
		calls++
		if calls < 3 {
			return "", errors.New("exit status 1")
		}
		return "<html>whatever</html>", nil
	})
	if err != nil {
		t.Fatalf("retryFetch returned %v, want nil once the command succeeds", err)
	}
	if out != "<html>whatever</html>" {
		t.Fatalf("retryFetch returned %q, want the fetched value unchanged", out)
	}
	if calls != 3 {
		t.Fatalf("fetch called %d times, want 3", calls)
	}
}

// TestRetryFetchDoesNotRetryASuccessfulFetch is the other half: a command that
// succeeds is never re-run, whatever it returned. Re-running on content would
// make an absence easier to satisfy, which is exactly what negatives must not do.
//
// VALIDATES: one successful fetch, one call.
// PREVENTS: retryFetch drifting into an assertion retry.
func TestRetryFetchDoesNotRetryASuccessfulFetch(t *testing.T) {
	calls := 0
	if _, err := retryFetch(func() (string, error) { calls++; return "", nil }); err != nil {
		t.Fatalf("retryFetch returned %v, want nil", err)
	}
	if calls != 1 {
		t.Fatalf("fetch called %d times, want exactly 1 even for an empty result", calls)
	}
}

// TestRetryPositiveDoesNotRetryOnFirstSuccess keeps the happy path free: an
// expectation that is already satisfied must cost exactly one browser round trip,
// or every passing web test pays the poll interval.
//
// VALIDATES: retryPositive calls the check once when it passes immediately.
// PREVENTS: adding a fixed delay to the ~85 passing web expectations.
func TestRetryPositiveDoesNotRetryOnFirstSuccess(t *testing.T) {
	calls := 0
	if err := retryPositive(func() error { calls++; return nil }); err != nil {
		t.Fatalf("retryPositive returned %v, want nil", err)
	}
	if calls != 1 {
		t.Fatalf("check called %d times, want exactly 1", calls)
	}
}

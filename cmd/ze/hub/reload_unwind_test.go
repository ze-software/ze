package hub

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestWithUnwindKeepsFixLineLast pins where a refused reload's failed unwind
// steps go. Method: wrap an operator error the way doReload does ("reload: "
// on its first line), fail the rollback and the candidate cleanup after it,
// then compare the rendered lines and walk the chain with errors.Is and
// errors.As.
//
// VALIDATES: spec-operator-errors-name-the-fix, the operator error's fix line
// is the last line of every refusal: an unwind failure takes a line of its own
// before it, and the fix line holds the exact command and nothing else.
// PREVENTS: "fix: <command> (candidate cleanup failed: ...)", where an operator
// who copies the fix line copies the cleanup note with it.
func TestWithUnwindKeepsFixLineLast(t *testing.T) {
	evidence := errors.New("line 7: unknown keyword")
	refusal := opererr.Wrap(evidence, "the new config was refused", "the running config stays", "ze config edit")
	rollbackErr := errors.New("plugin bgp did not answer")
	clearErr := errors.New("zefs: read-only")

	err := withUnwind(fmt.Errorf("reload: %w", refusal),
		unwindStep{unwindRollback, rollbackErr}, unwindStep{unwindCandidateCleanup, clearErr})

	want := "reload: the new config was refused\n" +
		"the running config stays: line 7: unknown keyword\n" +
		"rollback failed: plugin bgp did not answer; candidate cleanup failed: zefs: read-only\n" +
		"fix: ze config edit"
	if err.Error() != want {
		t.Fatalf("withUnwind =\n%s\nwant\n%s", err, want)
	}
	lines := strings.Split(err.Error(), "\n")
	if last := lines[len(lines)-1]; last != "fix: "+refusal.Fix() {
		t.Errorf("last line = %q, want the fix line alone %q", last, "fix: "+refusal.Fix())
	}

	for _, target := range []error{refusal, evidence, rollbackErr, clearErr} {
		if !errors.Is(err, target) {
			t.Errorf("errors.Is(withUnwind, %q) = false, want true", target)
		}
	}
	var operr *opererr.Error
	if !errors.As(err, &operr) {
		t.Fatalf("withUnwind = %T, errors.As finds no *opererr.Error", err)
	}
	if operr != refusal {
		t.Errorf("errors.As found %p, want the refusal %p", operr, refusal)
	}
}

// TestWithUnwindNestedKeepsFixLineLast pins the listener path, where an unwind
// failure is reported twice over: the listener migration's own rollback
// inside, the reload's rollback outside. Method: nest withUnwind as
// reloadListeners and doReload do, then compare the rendered lines.
//
// VALIDATES: the fix line stays last however many layers add unwind failures.
// PREVENTS: the outer layer falling back to the one-line form because the
// inner layer's text no longer ends with the fix line.
func TestWithUnwindNestedKeepsFixLineLast(t *testing.T) {
	refusal := opererr.New("the web listener could not bind 0.0.0.0:443", "the port is in use", "ze show system listeners")
	listenerErr := errors.New("bind refused")
	clearErr := errors.New("zefs: read-only")

	inner := withUnwind(fmt.Errorf("reconfigure web: %w", refusal), unwindStep{unwindListenerRollback, listenerErr})
	err := withUnwind(fmt.Errorf("reload: listener migration: %w", inner), unwindStep{unwindCandidateCleanup, clearErr})

	want := "reload: listener migration: reconfigure web: the web listener could not bind 0.0.0.0:443\n" +
		"the port is in use\n" +
		"listener rollback failed: bind refused\n" +
		"candidate cleanup failed: zefs: read-only\n" +
		"fix: ze show system listeners"
	if err.Error() != want {
		t.Fatalf("withUnwind =\n%s\nwant\n%s", err, want)
	}
	for _, target := range []error{refusal, listenerErr, clearErr} {
		if !errors.Is(err, target) {
			t.Errorf("errors.Is(withUnwind, %q) = false, want true", target)
		}
	}
}

// TestWithUnwindPlainError pins the form behind an error that is not an
// operator error, and the answer when nothing failed. Method: render a plain
// refusal with two failed steps and one that succeeded, then call withUnwind
// with only succeeded steps.
//
// VALIDATES: a plain refusal keeps the one-line form, "(rollback failed: ...;
// candidate cleanup failed: ...)", naming only the steps that failed, and a
// refusal whose unwinding all succeeded comes back as itself.
// PREVENTS: a succeeded step reported as failed, and a wrapper added around a
// refusal that has nothing to add.
func TestWithUnwindPlainError(t *testing.T) {
	refusal := errors.New("subsystem reload: l2tp: tunnel table full")
	rollbackErr := errors.New("bgp rollback refused")
	clearErr := errors.New("zefs: read-only")

	err := withUnwind(refusal,
		unwindStep{unwindPKIRestore, nil},
		unwindStep{unwindRollback, rollbackErr},
		unwindStep{unwindCandidateCleanup, clearErr})
	want := "subsystem reload: l2tp: tunnel table full (rollback failed: bgp rollback refused; candidate cleanup failed: zefs: read-only)"
	if err.Error() != want {
		t.Errorf("withUnwind =\n%s\nwant\n%s", err, want)
	}
	for _, target := range []error{refusal, rollbackErr, clearErr} {
		if !errors.Is(err, target) {
			t.Errorf("errors.Is(withUnwind, %q) = false, want true", target)
		}
	}

	if got := withUnwind(refusal, unwindStep{unwindRollback, nil}, unwindStep{unwindCandidateCleanup, nil}); got != refusal { //nolint:errorlint // identity: nothing failed, so withUnwind answers the refusal itself
		t.Errorf("withUnwind with no failed step = %v, want the refusal itself", got)
	}
}

// Design: docs/architecture/cli/error-surface.md -- the operator error, whose fix line stays last
// Related: main_reload.go -- doReload, the refusals whose unwinding this reports
// Related: managed.go -- applyManagedPush, which clears the candidate a refused push staged

package hub

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Unwind step names, as each one reads before "failed" in the refusal.
const (
	unwindCandidateCleanup = "candidate cleanup"
	unwindListenerRollback = "listener rollback"
	unwindPKIRestore       = "pki restore"
	unwindRollback         = "rollback"
)

// unwindStep is one step that undid part of a refused reload: its name and
// the error it answered, nil when it succeeded or had nothing to undo.
type unwindStep struct {
	name string
	err  error
}

// unwoundError is a refused reload together with the unwind steps that failed
// after it. errors.Is and errors.As reach the refusal and every failed step.
//
// Immutable after construction and safe for concurrent use.
type unwoundError struct {
	refusal error
	failed  []unwindStep
}

// withUnwind answers refusal with the unwind steps that failed after it. It
// answers refusal itself when every step succeeded. Go evaluates the calls in
// the steps' literals in argument order, so a caller that runs each step in
// its unwindStep lists them in the order they must run.
//
// The failed steps go where the operator reads them. Behind an operator error,
// whose fix is its last line (opererr.Error.Error), they take a line of their
// own before the fix, so the fix line stays the exact text the operator copies.
// Behind any other error they trail it in parentheses on the same line.
func withUnwind(refusal error, steps ...unwindStep) error {
	var failed []unwindStep
	for _, step := range steps {
		if step.err != nil {
			failed = append(failed, step)
		}
	}
	if len(failed) == 0 {
		return refusal
	}
	return &unwoundError{refusal: refusal, failed: failed}
}

// Error renders the refusal with its failed unwind steps:
//
//	<problem>
//	<why>
//	rollback failed: <err>; candidate cleanup failed: <err>
//	fix: <fix>
//
// when the refusal is an operator error, and
//
//	<refusal> (rollback failed: <err>; candidate cleanup failed: <err>)
//
// otherwise.
func (e *unwoundError) Error() string {
	text := e.refusal.Error()
	b := textbuf.New()
	if operr, ok := errors.AsType[*opererr.Error](e.refusal); ok {
		// A wrapper above the operator error prepends to its first line and
		// never appends, so the fix line still ends the text. Insert before it
		// only when it does: a text that ends otherwise takes the one-line form.
		fixLine := "\nfix: " + operr.Fix()
		if head, found := strings.CutSuffix(text, fixLine); found {
			b.Str(head).Byte('\n')
			e.writeFailed(b)
			b.Str(fixLine)
			return b.Slice()
		}
	}
	b.Str(text).Str(" (")
	e.writeFailed(b)
	b.Byte(')')
	return b.Slice()
}

// writeFailed writes "<name> failed: <err>" for each failed step, joined by "; ".
func (e *unwoundError) writeFailed(b *textbuf.Buffer) {
	for i, step := range e.failed {
		if i > 0 {
			b.Str("; ")
		}
		b.Str(step.name).Str(" failed: ").Err(step.err)
	}
}

// Unwrap answers the refusal, then each failed step's error.
func (e *unwoundError) Unwrap() []error {
	errs := make([]error, 0, 1+len(e.failed))
	errs = append(errs, e.refusal)
	for _, step := range e.failed {
		errs = append(errs, step.err)
	}
	return errs
}

// Design: docs/architecture/cli/error-surface.md -- how an operator error reads on each surface
// Related: docs/contributing/ze-go-style.md -- "Every error is handled", the operator error convention

// Package opererr holds the operator error: a refusal an operator reads, with
// what went wrong, why it matters, and the command or config change that fixes
// it (ai/rules/user-facing-errors.md).
//
// The producer that holds the fact builds the error, once. A layer above passes
// it through unwrapped, or wraps it only to add a fact the producer did not
// hold. Each boundary renders the same three parts:
//
//   - a Go error chain, the API answer and the plugin RPC answer use Error();
//     the CLI client prints that text unchanged, so the CLI shows the same
//     lines
//   - a log line uses Problem as its message and LogAttrs as its attributes,
//     with a caller-supplied "code" passed through
//   - a web notice reads Problem, Why and Fix through the accessors
//
// The package imports the standard library and internal/core/textbuf only, so
// every tier may import it.
package opererr

import (
	"log/slog"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// fixLabel opens the fix line, so a reader who scans for the command finds it
// on a line of its own, whatever the problem line holds.
const fixLabel = "fix: "

// zeroText is what a zero Error renders. Only a composite literal or new(Error)
// in Ze code builds one, never New or Wrap, so the text names the defect
// instead of rendering an empty refusal.
const zeroText = "BUG: opererr.Error built without opererr.New or opererr.Wrap"

// Error is an operator error. Build it with New or Wrap: the fields are
// private, so no caller replaces the parts the constructor checked.
//
// The zero value is uninitialized and is not a valid operator error. Its
// Error() names that defect, and its accessors answer empty strings.
//
// An Error is immutable after construction and safe for concurrent use.
type Error struct {
	problem string
	why     string
	fix     string
	cause   error
}

// New builds an operator error from what went wrong (problem), why it matters
// (why), and the exact command or config change that fixes it (fix).
//
// fix is the command alone, the exact text the operator types (or the config
// change, stated as the statement to write), with no label: Error() adds the
// "fix: " label when it renders the line. It is not a repair id, and it is not
// a diagnostic.Diagnostic Repair or FixSafety: those name an automated repair,
// while fix is what the operator runs.
//
// All three parts are required (ai/rules/user-facing-errors.md). problem, why
// and fix MUST be text Ze's code writes: a literal, or a literal with the value
// the operator typed or configured inserted into it. They are then never empty
// unless the producer is defective, so New panics with BUG on an empty
// problem, why or fix. Text received from outside Ze (a plugin's answer, a
// file, a peer) MUST NOT be passed to New: an empty part there is an operating
// error, not a defect.
func New(problem, why, fix string) *Error {
	if problem == "" {
		panic("BUG: opererr.New called with an empty problem")
	}
	if why == "" {
		panic("BUG: opererr.New called with an empty why for: " + problem)
	}
	if fix == "" {
		panic("BUG: opererr.New called with an empty fix for: " + problem)
	}
	return &Error{problem: problem, why: why, fix: fix}
}

// Wrap builds an operator error as New does and keeps cause as its evidence:
// Unwrap answers cause, so errors.Is and errors.As see through the operator
// error to it. The rendered forms show cause after the explanation.
//
// A nil cause builds the same error New builds. Wrap panics with BUG on an
// empty problem, why or fix, under the same contract as New.
func Wrap(cause error, problem, why, fix string) *Error {
	e := New(problem, why, fix)
	e.cause = cause
	return e
}

// Error renders the operator error as plain text, one part per line:
//
//	<problem>
//	<why>: <cause>
//	fix: <fix>
//
// The second line is <why> alone when there is no cause. The fix is always
// the last line, so a layer that prepends context to the first line never moves
// it. This is the text of a Go error chain, the API answer and the plugin RPC
// answer, and the CLI prints it unchanged.
func (e *Error) Error() string {
	if e.problem == "" {
		return zeroText
	}
	b := textbuf.New()
	b.Str(e.problem)
	b.Byte('\n').Str(e.why)
	if e.cause != nil {
		b.Str(": ").Err(e.cause)
	}
	b.Byte('\n').Str(fixLabel).Str(e.fix)
	return b.Slice()
}

// Unwrap answers the cause, or nil when the error has none.
func (e *Error) Unwrap() error {
	return e.cause
}

// Problem answers what went wrong, in the operator's terms.
func (e *Error) Problem() string {
	return e.problem
}

// Why answers why the problem matters.
func (e *Error) Why() string {
	return e.why
}

// Fix answers the exact command or config change that fixes the problem: the
// text the operator types, without the "fix: " label Error() renders.
func (e *Error) Fix() string {
	return e.fix
}

// LogAttrs answers the attributes of the operator error's log line. Problem is
// the line's message; the attributes are "why", "fix", and "error"
// with the cause (when set), then the caller's own attributes unchanged. The
// attribute names "why" and "fix" are the ones the tree's log lines and the
// diagnostic "fix" field already use. The error carries no diagnostic code: a
// caller whose log line owes one passes it in, and LogAttrs never invents it:
//
//	log.LogAttrs(ctx, slog.LevelWarn, e.Problem(), e.LogAttrs(slog.String("code", code))...)
//
// A zero Error answers one "error" attribute naming the defect, then the
// caller's attributes.
func (e *Error) LogAttrs(caller ...slog.Attr) []slog.Attr {
	if e.problem == "" {
		return append([]slog.Attr{slog.String("error", zeroText)}, caller...)
	}
	attrs := make([]slog.Attr, 0, 3+len(caller))
	attrs = append(attrs, slog.String("why", e.why), slog.String("fix", e.fix))
	if e.cause != nil {
		attrs = append(attrs, slog.Any("error", e.cause))
	}
	return append(attrs, caller...)
}

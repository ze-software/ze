// Design: docs/contributing/committing.md -- what the commit actions' exit codes mean
// Related: actions.go -- the action bodies that answer these codes
//
// A commit action that fails answers one of three codes (le's table,
// internal/le/le/root/exitcode.go): 2 when the keywords the line gave are
// refused, 3 when the operation could not run because the checkout, a file or
// git could not be read, and 1 when it ran and a gate or git refused it. The
// producer marks the first two where it knows which one it met, and an
// unmarked error is the third.

package commit

import "errors"

// exitError carries the code an error answers. Error and Unwrap pass the
// wrapped error through, so the operator reads the same line and a caller's
// errors.Is still reaches the cause.
type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string { return e.err.Error() }

func (e exitError) Unwrap() error { return e.err }

// refusedKeywords marks a refusal of the keywords the line gave: exit 2.
func refusedKeywords(err error) error {
	return exitError{code: 2, err: err}
}

// couldNotRun marks an operation that could not start, because the checkout,
// a file or git could not be read: exit 3.
func couldNotRun(err error) error {
	return exitError{code: 3, err: err}
}

// exitCodeOf answers the code err carries, and 1 for an unmarked error: the
// operation ran, and a gate or git refused it.
func exitCodeOf(err error) int {
	if marked, ok := errors.AsType[exitError](err); ok {
		return marked.code
	}
	return 1
}

// foundProblem marks a refusal the operation's own check made: it ran and
// found a problem, exit 1. It is spelled out where the surrounding errors
// default to 3, so the one refusal a check owns is not read as could not run.
func foundProblem(err error) error {
	return exitError{code: 1, err: err}
}

// exitCodeOrCouldNotRun answers the code err carries, and 3 for an unmarked
// error: used where every unmarked failure is the checkout, a file or git
// that could not be read.
func exitCodeOrCouldNotRun(err error) int {
	if marked, ok := errors.AsType[exitError](err); ok {
		return marked.code
	}
	return 3
}

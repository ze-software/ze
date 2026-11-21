// Design: docs/architecture/l2tp/bng-5-pppoe.md -- native mutation submission receipts
// Related: api.go -- the only producer of an unsubmitted request error
package vpp

import "errors"

// apiRequestNotSentError proves exchange returned before calling SendMsg. It
// preserves the underlying cause, but must never wrap a send or receive error:
// a partially transmitted request already owns an uncertain native outcome.
type apiRequestNotSentError struct{ cause error }

func (e *apiRequestNotSentError) Error() string { return e.cause.Error() }
func (e *apiRequestNotSentError) Unwrap() error { return e.cause }

func requestWasNotSent(err error) bool {
	var unsent *apiRequestNotSentError
	return errors.As(err, &unsent)
}

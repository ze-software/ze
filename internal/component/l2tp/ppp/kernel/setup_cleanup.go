// Design: docs/architecture/l2tp/subscriber-session-model.md -- failed setup physical receipts

package kernel

import "errors"

// setupCleanupError retains physical uncertainty separately from the primary
// setup refusal. Its zero value is not constructed; nil cleanup returns cause.
type setupCleanupError struct {
	cause   error
	cleanup error
}

func (failure *setupCleanupError) Error() string {
	return errors.Join(failure.cause, failure.cleanup).Error()
}
func (failure *setupCleanupError) Unwrap() []error { return []error{failure.cause, failure.cleanup} }

// RecordSetupCleanup preserves every lower physical failure without claiming
// the primary setup refusal itself means a resource remains owned.
func RecordSetupCleanup(cause, cleanup error) error {
	if cleanup == nil {
		return cause
	}
	return &setupCleanupError{cause: cause, cleanup: errors.Join(SetupCleanupFailure(cause), cleanup)}
}

// SetupCleanupFailure returns the retained physical rollback failure. A nil
// result means the cause carried no unconfirmed setup cleanup operation.
func SetupCleanupFailure(cause error) error {
	if failure, found := errors.AsType[*setupCleanupError](cause); found {
		return failure.cleanup
	}
	return nil
}

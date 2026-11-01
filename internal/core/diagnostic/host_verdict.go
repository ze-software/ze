// Design: docs/features/ai-first.md — diagnostic codes
// Related: codes.go — config-host-backend, the code a host verdict is reported under

package diagnostic

import "errors"

// HostVerdictError marks an error as a verdict about the host running the
// command, not about the configuration: a subsystem with no backend this
// operating system provides, or a hardware inventory this host does not
// expose. The same configuration validates on a host that has it. The producer
// marks the error where it knows the cause (HostVerdict), and a reader asks
// IsHostVerdict, never the message text. `ze config validate` reports a marked
// verifier refusal under CodeConfigHostBackend.
type HostVerdictError struct {
	Err error
}

// Error answers the wrapped error's text unchanged.
func (e *HostVerdictError) Error() string { return e.Err.Error() }

// Unwrap answers the wrapped error, so errors.Is still finds a sentinel.
func (e *HostVerdictError) Unwrap() error { return e.Err }

// HostVerdict marks err as a verdict about this host. A nil err answers nil.
func HostVerdict(err error) error {
	if err == nil {
		return nil
	}
	return &HostVerdictError{Err: err}
}

// IsHostVerdict reports whether err, or an error it wraps, was marked by
// HostVerdict.
func IsHostVerdict(err error) bool {
	var verdict *HostVerdictError
	return errors.As(err, &verdict)
}

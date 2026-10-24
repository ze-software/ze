// Fixture for the annotation (AC-5, AC-6).
package fixture

import "errors"

var errClosed = errors.New("closed")

var handler func() error

// run answers nil when no handler is set, and Stop relies on that zero.
func run() error {
	fn := handler
	// refuse-by-default: guards Stop -- a nil handler is the stopped state Stop leaves
	if fn == nil { // accepted: miss-answered-as-value
		return nil
	}
	return fn()
}

// Stop is the guard the annotation in run names.
func Stop() error {
	handler = nil
	return errClosed
}

func reflex(kind int) (string, error) {
	switch kind {
	case 1:
		return "one", nil
	// refuse-by-default: this is fine // want: annotation-names-no-guard
	default: // want: switch-unmatched-nil-error
		return "", nil
	}
}

func unknownGuard(kind int) (string, error) {
	switch kind {
	case 1:
		return "one", nil
	// refuse-by-default: guards NoSuchThing -- somewhere else // want: annotation-names-no-guard
	default: // want: switch-unmatched-nil-error
		return "", nil
	}
}

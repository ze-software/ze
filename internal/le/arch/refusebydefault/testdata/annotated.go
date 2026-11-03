// Fixture for the annotation (AC-5, AC-6).
package fixture

import (
	"errors"
	"log/slog"
)

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

// drain drops a close error at debug, and Stop relies on that: a handler
// already closed by Stop answers errClosed, which is the state, not a fault.
func drain(log *slog.Logger) {
	fn := handler
	if fn == nil {
		return
	}
	// refuse-by-default: guards Stop -- errClosed after Stop is the stopped state, not a failure
	if err := fn(); err != nil { // accepted: error-dropped-below-warn
		log.Debug("drain after stop", "error", err)
	}
}

// selfNamed names itself as the guard, which records nothing: the guard is the
// code that relies on the zero, never the function the shape sits in.
func selfNamed(kind int) (string, error) {
	switch kind {
	case 1:
		return "one", nil
	// refuse-by-default: guards selfNamed -- the function names itself // want: annotation-names-no-guard
	default: // want: switch-unmatched-nil-error
		return "", nil
	}
}

type server struct{ handler func() error }

// close names its own method in the `Type.Method` spelling.
func (s *server) close() error {
	fn := s.handler
	// refuse-by-default: guards server.close -- the method names itself // want: annotation-names-no-guard
	if fn == nil { // want: miss-answered-as-value
		return nil
	}
	return fn()
}

// open names its own method in the bare spelling.
func (s *server) open() error {
	fn := s.handler
	// refuse-by-default: guards open -- the method names itself // want: annotation-names-no-guard
	if fn == nil { // want: miss-answered-as-value
		return nil
	}
	return fn()
}

// serve names another method of the same type, the caller that relies on the
// nil answer, so the annotation is accepted.
func (s *server) serve() error {
	fn := s.handler
	// refuse-by-default: guards server.open -- open reads a nil answer as nothing to start
	if fn == nil { // accepted: miss-answered-as-value
		return nil
	}
	return fn()
}

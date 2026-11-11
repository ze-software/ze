// Design: docs/plugin-development/README.md -- telling a startup failure from a later one
// Related: sdk.go -- Run, which marks every error it returns before stage 5 completes

package sdk

import "strconv"

// StartupError marks an error Run returned before stage 5 of the startup
// handshake completed: the plugin never started. An error Run returns after
// stage 5 carries no StartupError, because the plugin started and then
// stopped. A caller tells the two apart with errors.As (or errors.AsType),
// never by reading the message, so it can report each under its own doctor
// code: a startup failure as doctor-plugin-startup-phase-failed, a later
// failure as doctor-plugin-stopped.
//
// Stage is the handshake stage that failed, 1 to 5; Method names it as the
// message did before the mark existed ("declare-registration", "configure",
// "declare-capabilities", "share-registry", "ready"). Err is the cause, which
// errors.Is and errors.As reach through Unwrap.
type StartupError struct {
	Stage  int
	Method string
	Err    error
}

// Error reads "stage N (method): cause".
func (e *StartupError) Error() string {
	return "stage " + strconv.Itoa(e.Stage) + " (" + e.Method + "): " + e.Err.Error()
}

// Unwrap answers the cause.
func (e *StartupError) Unwrap() error {
	return e.Err
}

// startupFailure marks err as the failure of startup stage at method.
func startupFailure(stage int, method string, err error) error {
	return &StartupError{Stage: stage, Method: method, Err: err}
}

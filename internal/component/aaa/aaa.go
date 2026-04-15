// Design: .claude/patterns/registration.md -- AAA registry (VFS-like)
// Detail: types.go -- value types (AuthResult, Bundle, Contribution, ...) and BackendRegistry

// Package aaa defines the pluggable AAA (Authentication, Authorization,
// Accounting) backend layer. Backends self-register via init() and are
// composed by the hub through Build. The core never imports a specific
// backend by name.
package aaa

import "errors"

// ErrAuthRejected is returned when a backend explicitly rejects credentials.
// Distinct from connection errors: a rejected authentication MUST NOT fall
// through to the next backend in a chain.
var ErrAuthRejected = errors.New("authentication rejected")

// Authenticator verifies a user's credentials against a backend.
//
// Returns (result, nil) on success; (result, ErrAuthRejected) on explicit
// rejection; (zero, other error) on connection/infrastructure failure.
// ChainAuthenticator relies on this distinction to decide fallthrough.
type Authenticator interface {
	Authenticate(username, password string) (AuthResult, error)
}

// Authorizer decides whether a user may run a command. The bool return
// avoids a dependency from this package on authz.Action (authz imports aaa,
// not the other way around).
type Authorizer interface {
	Authorize(username, command string, isReadOnly bool) (allowed bool)
}

// Accountant records command execution. Implementations MUST NOT block
// command execution on accounting failure; errors are logged locally.
type Accountant interface {
	CommandStart(username, remoteAddr, command string) (taskID string)
	CommandStop(taskID, username, remoteAddr, command string)
}

// Backend is a factory for AAA capabilities. A backend self-registers via
// Register; the hub calls Build at startup to compose the live Bundle.
type Backend interface {
	Name() string
	Priority() int // lower number comes first in the Authenticator chain
	Build(params BuildParams) (Contribution, error)
}

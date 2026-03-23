// Design: (none -- predates documentation)

package cli

// LoginWarning is a single warning shown at CLI login.
// Message is a human-readable description of the issue.
// Command is the actionable command to resolve it (shown as "run: <command>").
type LoginWarning struct {
	Message string
	Command string
}

// Design: (none -- predates documentation)
// Overview: ssh.go -- SSH server lifecycle

package ssh

import "codeberg.org/thomas-mangin/ze/internal/component/cli/contract"

// LoginWarningsFunc returns login warnings to display when an SSH session starts.
// Returns nil when no warnings exist.
// Injected by the daemon via SetLoginWarnings after the reactor starts.
type LoginWarningsFunc func() []contract.LoginWarning

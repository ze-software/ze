// Design: (none -- predates documentation)
// Overview: ssh.go -- SSH server lifecycle

package ssh

import "codeberg.org/thomas-mangin/ze/internal/component/cli"

// LoginWarningsFunc returns login warnings to display when an SSH session starts.
// Returns nil when no warnings exist. Called during createSessionModel.
// Injected by the daemon via SetLoginWarnings after the reactor starts.
type LoginWarningsFunc func() []cli.LoginWarning

// collectWarnings calls fn with panic recovery. If fn panics, the panic
// is logged and nil is returned so the SSH session continues normally.
func (s *Server) collectWarnings(fn LoginWarningsFunc) (warnings []cli.LoginWarning) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("login warnings provider panicked", "error", r)
			warnings = nil
		}
	}()
	return fn()
}

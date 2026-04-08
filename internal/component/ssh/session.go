// Design: (none -- predates documentation)
// Overview: ssh.go -- SSH server lifecycle

package ssh

import tea "charm.land/bubbletea/v2"

// createSessionModel builds a bubbletea Model for an SSH session.
// Delegates to the SessionModelFactory injected by the hub.
// Falls back to a nil model if no factory is set (shouldn't happen in production).
func (s *Server) createSessionModel(username string) tea.Model {
	s.mu.Lock()
	factory := s.sessionModelFactory
	s.mu.Unlock()

	if factory == nil {
		s.logger.Error("no session model factory set")
		return nil
	}
	return factory(username)
}

// Design: (none -- predates documentation)
// Overview: ssh.go -- SSH server lifecycle

package ssh

import (
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
)

// createSessionModel builds a unified cli.Model for an SSH session.
// If ConfigPath and Storage are set, the model gets an Editor with session identity
// for concurrent config editing. Otherwise falls back to command-only mode.
func (s *Server) createSessionModel(username string) cli.Model {
	s.mu.Lock()
	factory := s.executorFactory
	s.mu.Unlock()

	var executor CommandExecutor
	if factory != nil {
		executor = factory(username)
	} else if s.config.Executor != nil {
		executor = s.config.Executor
	}

	// Try to create an editor-capable model for concurrent config editing.
	if s.config.ConfigPath != "" && s.config.Storage != nil {
		ed, err := cli.NewEditorWithStorage(s.config.Storage, s.config.ConfigPath)
		if err != nil {
			s.logger.Warn("SSH session editor creation failed, using command-only mode",
				"user", username, "error", err)
		} else {
			session := cli.NewEditSession(username, "ssh")
			ed.SetSession(session)

			m, modelErr := cli.NewModel(ed)
			if modelErr != nil {
				s.logger.Warn("SSH session model creation failed, using command-only mode",
					"user", username, "error", modelErr)
			} else {
				if executor != nil {
					m.SetCommandExecutor(executor)
				}
				return m
			}
		}
	}

	// Fallback: command-only model (no editor, no config editing).
	m := cli.NewCommandModel()
	if executor != nil {
		m.SetCommandExecutor(executor)
	}
	return m
}

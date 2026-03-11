// Design: (none -- new SSH server component)
// Overview: ssh.go -- SSH server lifecycle

package ssh

import (
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
)

// createSessionModel builds a unified cli.Model for an SSH session.
// The model starts in command mode with the executor wired.
// If no executor is provided, the model runs without command execution.
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

	m := cli.NewCommandModel()
	if executor != nil {
		m.SetCommandExecutor(executor)
	}
	return m
}

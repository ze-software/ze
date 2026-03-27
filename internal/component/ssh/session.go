// Design: (none -- predates documentation)
// Overview: ssh.go -- SSH server lifecycle
// Related: warnings.go -- LoginWarningsFunc type definition

package ssh

import (
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

// buildCommandTree builds a command.Node tree from YANG command modules.
// Used to wire command-mode tab completion in SSH sessions.
func buildCommandTree() *command.Node {
	loader, _ := yang.DefaultLoader()
	return yang.BuildCommandTree(loader)
}

// createSessionModel builds a unified cli.Model for an SSH session.
// If ConfigPath and Storage are set, the model gets an Editor with session identity
// for concurrent config editing. Otherwise falls back to command-only mode.
func (s *Server) createSessionModel(username string) cli.Model {
	s.mu.Lock()
	factory := s.executorFactory
	monitorFn := s.monitorFactory
	shutdownFn := s.shutdownFunc
	restartFn := s.restartFunc
	warningsFn := s.loginWarningsFunc
	s.mu.Unlock()

	// Collect login warnings (safe if warningsFn is nil or panics).
	var warnings []cli.LoginWarning
	if warningsFn != nil {
		warnings = s.collectWarnings(warningsFn)
	}

	var executor CommandExecutor
	if factory != nil {
		executor = factory(username)
	} else if s.config.Executor != nil {
		executor = s.config.Executor
	}

	// Build command tree for tab completion in command mode.
	cmdCompleter := cli.NewCommandCompleter(buildCommandTree())

	// Try to create an editor-capable model for concurrent config editing.
	if s.config.ConfigPath != "" && s.config.Storage != nil {
		ed, err := cli.NewEditorWithStorage(s.config.Storage, s.config.ConfigPath)
		if err != nil {
			s.logger.Warn("SSH session editor creation failed, using command-only mode",
				"user", username, "error", err)
		} else if err := cli.ValidateUser(username); err != nil {
			s.logger.Warn("SSH session invalid username, using command-only mode",
				"user", username, "error", err)
		} else {
			session := cli.NewEditSession(username, "ssh")
			ed.SetSession(session)

			m, modelErr := cli.NewModel(ed)
			if modelErr != nil {
				s.logger.Warn("SSH session model creation failed, using command-only mode",
					"user", username, "error", modelErr)
			} else {
				m.SetCommandCompleter(cmdCompleter)
				if executor != nil {
					m.SetCommandExecutor(executor)
				}
				if monitorFn != nil {
					m.SetMonitorFactory(monitorFn)
				}
				if shutdownFn != nil {
					m.SetShutdownFunc(shutdownFn)
				}
				if restartFn != nil {
					m.SetRestartFunc(restartFn)
				}
				m.SetLoginWarnings(warnings)
				return m
			}
		}
	}

	// Fallback: command-only model (no editor, no config editing).
	m := cli.NewCommandModel()
	m.SetCommandCompleter(cmdCompleter)
	if executor != nil {
		m.SetCommandExecutor(executor)
	}
	if monitorFn != nil {
		m.SetMonitorFactory(monitorFn)
	}
	if shutdownFn != nil {
		m.SetShutdownFunc(shutdownFn)
	}
	if restartFn != nil {
		m.SetRestartFunc(restartFn)
	}
	m.SetLoginWarnings(warnings)
	return m
}

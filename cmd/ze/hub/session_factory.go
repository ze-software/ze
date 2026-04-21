// Design: docs/architecture/hub-architecture.md -- SSH session model factory
// Related: infra_setup.go -- wires the factory into the SSH server

package hub

import (
	tea "charm.land/bubbletea/v2"

	bgpconfig "codeberg.org/thomas-mangin/ze/internal/component/bgp/config"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/cli/contract"
	"codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	zessh "codeberg.org/thomas-mangin/ze/internal/component/ssh"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// buildSessionModelFactory creates a SessionModelFactory that produces bubbletea
// models for SSH sessions. This is the logic formerly in ssh/session.go's
// createSessionModel, moved here to decouple ssh from cli.
func buildSessionModelFactory(srv *zessh.Server, params bgpconfig.InfraHookParams) contract.SessionModelFactory {
	log := slogutil.Logger("hub.session")

	return func(username, remoteAddr string) tea.Model {
		// Build command tree for tab completion.
		cmdTree := buildCommandTree()
		cmdCompleter := cli.NewCommandCompleter(cmdTree)

		// Collect login warnings.
		var warnings []contract.LoginWarning
		warningsFn := srv.LoginWarningsFunc()
		if warningsFn != nil {
			warnings = warningsFn()
		}

		// contract.LoginWarning is a type alias of cli.LoginWarning; same type.
		cliWarnings := warnings

		// Get executors from the server.
		executor := srv.ExecutorForUser(username, remoteAddr)

		// Try to create editor-capable model.
		if params.ConfigPath != "" && params.Store != nil {
			ed, err := cli.NewEditorWithStorage(params.Store, params.ConfigPath)
			if err != nil {
				log.Warn("session editor creation failed", "user", username, "error", err)
			} else if err := cli.ValidateUser(username); err != nil {
				log.Warn("session invalid username", "user", username, "error", err)
			} else {
				session := cli.NewEditSession(username, "ssh")
				ed.SetSession(session)

				m, modelErr := cli.NewModel(ed)
				if modelErr != nil {
					log.Warn("session model creation failed", "user", username, "error", modelErr)
				} else {
					m.SetCommandCompleter(cmdCompleter)
					if executor != nil {
						m.SetCommandExecutor(executor)
						m.SetDashboardFactory(dashboardFactoryFromExecutor(executor))
					}
					monitorFn := srv.MonitorFactoryFunc()
					if monitorFn != nil {
						m.SetMonitorFactory(monitorFn)
					}
					shutdownFn := srv.ShutdownFunc()
					if shutdownFn != nil {
						m.SetShutdownFunc(shutdownFn)
					}
					restartFn := srv.RestartFunc()
					if restartFn != nil {
						m.SetRestartFunc(restartFn)
					}
					m.SetLoginWarnings(cliWarnings)
					return m
				}
			}
		}

		// Fallback: command-only model.
		m := cli.NewCommandModel()
		m.SetCommandCompleter(cmdCompleter)
		if executor != nil {
			m.SetCommandExecutor(executor)
			m.SetDashboardFactory(dashboardFactoryFromExecutor(executor))
		}
		monitorFn := srv.MonitorFactoryFunc()
		if monitorFn != nil {
			m.SetMonitorFactory(monitorFn)
		}
		shutdownFn := srv.ShutdownFunc()
		if shutdownFn != nil {
			m.SetShutdownFunc(shutdownFn)
		}
		restartFn := srv.RestartFunc()
		if restartFn != nil {
			m.SetRestartFunc(restartFn)
		}
		m.SetLoginWarnings(cliWarnings)
		return m
	}
}

// buildCommandTree builds a command.Node tree from YANG command modules.
func buildCommandTree() *command.Node {
	loader, _ := yang.DefaultLoader()
	tree := yang.BuildCommandTree(loader)
	command.WireValueHints(tree)
	return tree
}

// dashboardFactoryFromExecutor creates a DashboardFactory from a CommandExecutor.
func dashboardFactoryFromExecutor(cmdExec zessh.CommandExecutor) cli.DashboardFactory {
	return func() (func() (string, error), error) {
		return func() (string, error) {
			return cmdExec("bgp summary")
		}, nil
	}
}

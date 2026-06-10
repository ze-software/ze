// Design: docs/architecture/hub-architecture.md -- SSH session model factory
// Related: infra_setup.go -- wires the factory into the SSH server

package hub

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"codeberg.org/thomas-mangin/ze/internal/component/audit"
	bgpconfig "codeberg.org/thomas-mangin/ze/internal/component/bgp/config"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/cli/contract"
	"codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	pingcmd "codeberg.org/thomas-mangin/ze/internal/component/ping/cmd"
	zessh "codeberg.org/thomas-mangin/ze/internal/component/ssh"
	traceroutecmd "codeberg.org/thomas-mangin/ze/internal/component/traceroute/cmd"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// newSessionEditor builds the storage-backed editor for one SSH session:
// validated user, session identity, and — when a reload function is given —
// the reload notifier. The notifier is what routes `commit` through the
// transactional CommitSessionCandidate + NotifyReload path so a session
// commit reaches the running daemons instead of only writing config.conf.
func newSessionEditor(store storage.Storage, configPath, username string, reloadFn func() error) (*cli.Editor, error) {
	if err := cli.ValidateUser(username); err != nil {
		return nil, fmt.Errorf("invalid username: %w", err)
	}
	ed, err := cli.NewEditorWithStorage(store, configPath)
	if err != nil {
		return nil, err
	}
	ed.SetSession(cli.NewEditSession(username, "ssh"))
	if reloadFn != nil {
		ed.SetReloadNotifier(reloadFn)
	}
	return ed, nil
}

// buildSessionModelFactory creates a SessionModelFactory that produces bubbletea
// models for SSH sessions. This is the logic formerly in ssh/session.go's
// createSessionModel, moved here to decouple ssh from cli.
// reloadFn is wired into every session editor as the reload notifier
// (see newSessionEditor); nil leaves commits non-transactional.
func buildSessionModelFactory(srv *zessh.Server, params bgpconfig.InfraHookParams, recorder audit.Recorder, reloadFn func() error) contract.SessionModelFactory {
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
			ed, err := newSessionEditor(params.Store, params.ConfigPath, username, reloadFn)
			if err != nil {
				log.Warn("session editor creation failed", "user", username, "error", err)
			} else {
				m, modelErr := cli.NewModel(ed)
				if modelErr != nil {
					log.Warn("session model creation failed", "user", username, "error", modelErr)
				} else {
					m.SetAuditRecorder(recorder, audit.SSH, username, remoteAddr)
					m.SetCommandCompleter(cmdCompleter)
					if executor != nil {
						m.SetCommandExecutor(executor)
						m.SetDashboardFactory(dashboardFactoryFromExecutor(executor))
						m.SetTracerouteFactory(streamingTracerouteFactory)
						m.SetPingFactory(streamingPingFactory)
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
			m.SetTracerouteFactory(streamingTracerouteFactory)
			m.SetPingFactory(streamingPingFactory)
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
			return cmdExec("summary")
		}, nil
	}
}

func streamingTracerouteFactory(ctx context.Context, target string, maxHops int) (<-chan map[string]any, context.CancelFunc, error) {
	return traceroutecmd.NewTracerouteSession(ctx, target, maxHops)
}

func streamingPingFactory(ctx context.Context, target string, interval, timeout time.Duration) (<-chan map[string]any, context.CancelFunc, error) {
	return pingcmd.NewPingSession(ctx, target, interval, timeout)
}

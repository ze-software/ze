// Design: docs/architecture/hub-architecture.md -- SSH session model factory
// Related: service_ssh.go -- builds the factory into the SSH server (via the seam)
//
// Compiled only under //go:build ze_ssh: the interactive ssh session model is
// ssh-only and dropped from the binary when ssh is compiled out.

//go:build ze_ssh

package hub

import (
	"context"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/component/config/infra"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/yang"
	pingcmd "github.com/ze-software/ze/internal/component/ping/cmd"
	"github.com/ze-software/ze/internal/component/plugin"
	zessh "github.com/ze-software/ze/internal/component/ssh"
	traceroutecmd "github.com/ze-software/ze/internal/component/traceroute/cmd"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/slogutil"
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
func buildSessionModelFactory(srv *zessh.Server, params infra.HookParams, recorder audit.Recorder, reloadFn func() error) zessh.SessionModelFactory {
	log := slogutil.Logger("hub.session")

	return func(username, remoteAddr string, authorizer plugin.Authorizer) tea.Model {
		// Build command tree for tab completion. The tree is rebuilt per session
		// and plugin commands are merged in lazily from the live dispatcher, so a
		// plugin registered (or gone) since the daemon started is reflected in the
		// next session's completion without a rebuild dance.
		cmdTree := buildCommandTree()
		mergePluginCommands(cmdTree, params)
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
		executor := srv.ExecutorForUser(username, remoteAddr, authorizer)

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
						m.SetCommandExecutor(cliExecutor(executor))
						injectViewFactories(&m, executor)
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
			m.SetCommandExecutor(cliExecutor(executor))
			injectViewFactories(&m, executor)
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

// mergePluginCommands injects plugin-registered commands into the completion
// tree so tab-completion offers them alongside YANG-backed commands. Entries
// come from the live dispatcher's command registry (Hidden commands excluded),
// resolved lazily via params.APIServer so this works even though the session
// factory is built before the dispatcher is wired: the per-session closure runs
// after post-start, when the registry is populated. A nil dispatcher (no plugin
// engine, e.g. early startup) is a no-op, leaving the YANG-only tree intact.
func mergePluginCommands(tree *command.Node, params infra.HookParams) {
	if params.APIServer == nil {
		return
	}
	srv := params.APIServer()
	if srv == nil {
		return
	}
	d := srv.Dispatcher()
	if d == nil {
		return
	}
	command.MergeCommandPaths(tree, d.Registry().VisibleCommandEntries())
}

// cliExecutor carries an SSH command response into the Bubble Tea writer.
func cliExecutor(executor zessh.CommandExecutor) cli.CommandExecutor {
	return func(input string) (cli.CommandOutput, error) {
		rendered, err := executor(input)
		if rendered == nil {
			return cli.CommandOutput{}, err
		}
		return cli.CommandOutput{
			Text:              rendered.Output,
			TransportComplete: rendered.TransportComplete,
		}, err
	}
}

// injectViewFactories injects each registered live view's concrete factory into
// the model by iterating cli.RegisteredViews() instead of calling per-view typed
// setters. The owner-cmd factories are built here so imports stay in the
// consumer while the model discovers views through the registry.
func injectViewFactories(m *cli.Model, executor zessh.CommandExecutor) {
	for _, v := range cli.RegisteredViews() {
		switch v.Key {
		case cli.ViewKeyDashboard:
			m.SetViewFactory(v.Key, dashboardFactoryFromExecutor(executor))
		case cli.ViewKeyTraceroute:
			m.SetViewFactory(v.Key, cli.TracerouteFactory(streamingTracerouteFactory))
		case cli.ViewKeyPing:
			m.SetViewFactory(v.Key, cli.PingFactory(streamingPingFactory))
		}
	}
}

// dashboardFactoryFromExecutor creates a DashboardFactory from a CommandExecutor.
func dashboardFactoryFromExecutor(cmdExec zessh.CommandExecutor) cli.DashboardFactory {
	return func() (func() (string, error), error) {
		return func() (string, error) {
			rendered, err := cmdExec("show bgp summary")
			if rendered != nil {
				defer rendered.TransportComplete()
				return rendered.Output, err
			}
			return "", err
		}, nil
	}
}

func streamingTracerouteFactory(ctx context.Context, target string, maxHops int) (<-chan map[string]any, context.CancelFunc, error) {
	return traceroutecmd.NewTracerouteSession(ctx, target, maxHops)
}

func streamingPingFactory(ctx context.Context, target string, interval, timeout time.Duration, count, size int) (<-chan map[string]any, context.CancelFunc, error) {
	return pingcmd.NewPingSession(ctx, target, interval, timeout, count, size)
}

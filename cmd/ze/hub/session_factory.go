// Design: docs/architecture/hub-architecture.md -- SSH session model factory
// Related: service_ssh.go -- builds the factory into the SSH server (via the seam)
// Detail: session_editor.go -- newSessionEditor, shared with the attached console
//
// Compiled only under //go:build ze_ssh: the interactive ssh session model is
// ssh-only and dropped from the binary when ssh is compiled out.

//go:build ze_ssh

package hub

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/config/infra"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	zessh "github.com/ze-software/ze/internal/component/ssh"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// buildSessionModelFactory creates a SessionModelFactory that produces bubbletea
// models for SSH sessions. This is the logic formerly in ssh/session.go's
// createSessionModel, moved here to decouple ssh from cli.
// reloadFn is wired into every session editor as the reload notifier
// (see newSessionEditor); nil leaves commits non-transactional.
func buildSessionModelFactory(srv *zessh.Server, params infra.HookParams, recorder audit.Recorder, reloadFn func() error) zessh.SessionModelFactory {
	log := slogutil.Logger("hub.session")

	return func(sessionCtx context.Context, username, remoteAddr string, authorizer plugin.Authorizer, request zessh.SessionRequest) (tea.Model, error) {
		configPath := params.ConfigPath
		commitReload := reloadFn
		if request.ConfigName != "" {
			if request.ConfigName != filepath.Base(configPath) {
				configPath = request.ConfigName
				commitReload = nil
			}
			if params.Store == nil {
				return nil, fmt.Errorf("configuration %q unavailable: no persistent store", request.ConfigName)
			}
			if !params.Store.Exists(configPath) {
				return nil, fmt.Errorf("configuration %q does not exist", request.ConfigName)
			}
		}
		// Build command tree for tab completion. The tree is rebuilt per session
		// and plugin commands are merged in lazily from the live dispatcher, so a
		// plugin registered (or gone) since the daemon started is reflected in the
		// next session's completion without a rebuild dance.
		cmdTree, err := buildCommandTree()
		if err != nil {
			return nil, fmt.Errorf("command tree: %w", err)
		}
		var cmdCompleter cli.CommandModeCompleter = cli.NewCommandCompleter(cmdTree)
		var registered func(string) bool
		if params.APIServer != nil {
			registered = serverOperationalCommand(params.APIServer())
		}

		// Collect login warnings.
		var warnings []contract.LoginWarning
		warningsFn := srv.LoginWarningsFunc()
		if warningsFn != nil {
			warnings = warningsFn()
		}

		// contract.LoginWarning is a type alias of cli.LoginWarning; same type.
		cliWarnings := warnings

		// Get executors from the server.
		executor := srv.ExecutorForUser(sessionCtx, username, remoteAddr, authorizer)
		if params.APIServer != nil {
			entries := func() []command.CommandEntry {
				server := params.APIServer()
				if server == nil {
					return nil
				}
				if server.Dispatcher() == nil {
					return nil
				}
				return server.Dispatcher().Registry().VisibleCommandEntries()
			}
			var arguments func(*command.Node) *command.Node
			if executor != nil {
				query := func(line string) (string, error) {
					answer, err := executor(line)
					if answer == nil {
						return "", err
					}
					defer answer.TransportComplete()
					return plugin.ResponseJSON(answer.Response, err)
				}
				arguments = func(base *command.Node) *command.Node {
					server := params.APIServer()
					if server == nil {
						return base
					}
					return server.WithArgCompletion(base, query)
				}
			}
			cmdCompleter = newPluginAwareCommandCompleter(cmdTree, entries, arguments, command.SurfaceSSH)
		}

		// Try to create editor-capable model.
		if configPath != "" && configPath != "-" && params.Store != nil {
			ed, err := newSessionEditor(params.Store, configPath, username, sessionOriginSSH, commitReload)
			if err != nil {
				if request.Mode == sshclient.CLIModeEdit {
					return nil, err
				}
				log.Warn("session editor creation failed", "user", username, "error", err)
			} else {
				m, modelErr := cli.NewModel(ed, cli.FilesystemAuthorityUnknown)
				if modelErr == nil {
					m.SetAuditRecorder(recorder, audit.SSH, username, remoteAddr)
					m.SetCommandCompleter(cmdCompleter)
					m.SetRegisteredOperational(registered)
					m.SetHistory(cli.NewHistory(params.Store, username))
					if executor != nil {
						m.SetCommandExecutor(cliExecutor(executor))
						injectViewFactories(&m, executor)
					}
					m.SetTranscript(sessionTranscript(username, remoteAddr))
					monitorFn := srv.MonitorFactoryFunc()
					if monitorFn != nil {
						m.SetMonitorFactory(monitorFn)
					}
					m.SetLoginWarnings(cliWarnings)
					if request.Mode == sshclient.CLIModeCommand {
						m.SetMode(cli.ModeOperational)
					}
					return m, nil
				}

				modelErr = errors.Join(modelErr, ed.Close())
				log.Warn("session model creation failed", "user", username, "error", modelErr)
				if request.Mode == sshclient.CLIModeEdit {
					return nil, modelErr
				}
			}
		}
		if request.Mode == sshclient.CLIModeEdit {
			return nil, fmt.Errorf("configuration editing unavailable: no persistent configuration source")
		}

		// Fallback: command-only model.
		m := cli.NewCommandModel(cli.FilesystemAuthorityUnknown)
		m.SetCommandCompleter(cmdCompleter)
		m.SetRegisteredOperational(registered)
		if params.Store != nil {
			m.SetHistory(cli.NewHistory(params.Store, username))
		}
		if executor != nil {
			m.SetCommandExecutor(cliExecutor(executor))
			injectViewFactories(&m, executor)
		}
		m.SetTranscript(sessionTranscript(username, remoteAddr))
		monitorFn := srv.MonitorFactoryFunc()
		if monitorFn != nil {
			m.SetMonitorFactory(monitorFn)
		}
		m.SetLoginWarnings(cliWarnings)
		return m, nil
	}
}

// buildCommandTree builds a command.Node tree from YANG command modules.
// transcriptSequence numbers the daemon's transcript files, so two sessions
// opened in the same second land in two files.
var transcriptSequence atomic.Uint64

// sessionTranscript opens the transcript for one interactive session when
// `environment { cli { transcript enabled } }` is set. The daemon owns the
// session's model (docs/guide/config-editor.md), so the file lives in the
// daemon's `$XDG_DATA_HOME/ze/transcripts/`, and the model's Close releases
// it when the session ends. The file name carries the daemon pid and the
// sequence number only: the username is authenticated, not sanitized (RADIUS
// and TACACS+ hand it back as sent), so it stays inside the header and never
// chooses a path. A nil writer records nothing.
func sessionTranscript(username, remoteAddr string) *cli.TranscriptWriter {
	tag := strconv.Itoa(os.Getpid()) + "-" + strconv.FormatUint(transcriptSequence.Add(1), 10)
	return cli.NewTranscriptWriter(cli.OpenTranscriptFile(tag), username, remoteAddr)
}

// buildCommandTree builds the operational command tree an SSH session
// completes against. A loader failure, such as a misspelled extension, is
// returned with its cause, so the session refuses to start rather than run
// with a nil tree.
func buildCommandTree() (*command.Node, error) {
	schema, err := yang.DefaultLoader()
	if err != nil {
		return nil, fmt.Errorf("YANG schema: %w", err)
	}
	tree := yang.BuildCommandTree(schema)
	command.WireValueHints(tree)
	return tree, nil
}

// mergePluginCommands injects plugin-registered commands into the completion
// tree so tab-completion offers them alongside YANG-backed commands. Entries
// come from the live dispatcher's command registry (Hidden commands excluded),
// resolved lazily via params.APIServer so this works even though the session
// factory is built before the dispatcher is wired: the per-session closure runs
// after post-start, when the registry is populated. A nil dispatcher (no plugin
// engine, e.g. early startup) is a no-op, leaving the YANG-only tree intact.
// The live session completer overlays these entries and authenticated argument
// producers at each request; this helper also serves registry-coverage probes.
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

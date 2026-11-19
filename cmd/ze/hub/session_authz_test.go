//go:build ze_ssh

// Design: docs/architecture/hub-architecture.md -- the SSH session editor authorizes each config-mode verb
// Related: session_factory.go -- buildSessionModelFactory binds the authorizer these tests drive
// Related: confirm_window.go -- the confirmed-commit revert AC-13 drives

package hub

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/sshclient"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zessh "github.com/ze-software/ze/internal/component/ssh"
)

// authzConfigPath is the daemon's own configuration; authzClientConfig is a
// stored client configuration a session names (spec-fleet-6 declares it).
const (
	authzConfigPath   = "router.conf"
	authzClientConfig = "client-edge.conf"
	authzPassword     = "authz-test-password"
	authzSeedHost     = "seed-host"
	authzSeedConfig   = "system {\n\thost " + authzSeedHost + ";\n}\n"
	// authzWait bounds every poll for screen content or stored state. It is a
	// ceiling, never a pace: each wait returns as soon as its state appears.
	authzWait = 20 * time.Second
)

// authzPolicyTemplate is the operator's system.authorization, read by the
// production extractor (infra.ExtractAuthzStore). @EDITOR@ is the edit
// default of the profile AC-13 changes while a confirmed commit is open.
const authzPolicyTemplate = `system {
	authentication {
		user admin { password "@HASH@"; profile admin; }
		user viewer { password "@HASH@"; profile read-only; }
		user setter { password "@HASH@"; profile set-commit; }
		user operator { password "@HASH@"; profile operator; }
		user fleet { password "@HASH@"; profile fleet; }
		user editor { password "@HASH@"; profile editor; }
	}
	authorization {
		profile admin {
			run { default-action allow; }
			edit { default-action allow; }
		}
		profile read-only {
			run {
				default-action allow
				entry 10 { action deny; match debug; }
				entry 20 { action deny; match clear; }
			}
			edit { default-action deny; }
		}
		profile set-commit {
			run { default-action allow; }
			edit {
				default-action deny
				entry 10 { action allow; match "config set"; }
				entry 20 { action allow; match "config commit"; }
			}
		}
		profile operator {
			run { default-action allow; }
			edit {
				default-action deny
				entry 10 { action allow; match config; }
			}
		}
		profile fleet {
			run { default-action allow; }
			edit {
				default-action allow
				entry 10 { action deny; match "config commit client-edge.conf"; }
			}
		}
		profile editor {
			run { default-action allow; }
			edit { default-action @EDITOR@; }
		}
	}
}
`

// authzUsers are the login accounts, each with the profile the policy assigns.
var authzUsers = map[string]string{
	"admin": "admin", "viewer": "read-only", "setter": "set-commit",
	"operator": "operator", "fleet": "fleet", "editor": "editor",
}

// authzDaemon is the SSH side of a daemon: the real SSH server, the real
// session factory over a real store, the live AAA indirections the daemon
// wires (infra_setup.go), and a real dispatcher for operational commands.
type authzDaemon struct {
	t       *testing.T
	store   storage.Storage
	server  *zessh.Server
	hash    string
	users   []authz.UserConfig
	reloads atomic.Int32
}

// newAuthzDaemon starts the daemon. With authorization false it has no
// system.authorization, as an unconfigured box: the bundle carries no
// authorizer, which liveAAABundleAuthorizer answers as allow (A-5).
func newAuthzDaemon(t *testing.T, authorization bool) *authzDaemon {
	t.Helper()
	resetAAABundleForTest(t)
	d := &authzDaemon{t: t, store: newTestStore(t)}
	require.NoError(t, d.store.WriteFile(authzConfigPath, []byte(authzSeedConfig), 0o600))
	require.NoError(t, d.store.WriteFile(authzClientConfig, []byte(authzSeedConfig), 0o600))
	hash, err := bcrypt.GenerateFromPassword([]byte(authzPassword), bcrypt.MinCost)
	require.NoError(t, err)
	d.hash = string(hash)
	for name, profile := range authzUsers {
		d.users = append(d.users, authz.UserConfig{Name: name, Hash: d.hash, Profiles: []string{profile}})
	}
	bundle := &aaa.Bundle{Authenticator: &authz.LocalAuthenticator{Users: d.users}}
	if authorization {
		bundle.Authorizer = authz.StoreAuthorizer{Store: d.policy("allow")}
	}
	aaaBundle.Store(bundle)

	resolved, err := yang.DefaultLoader()
	require.NoError(t, err)
	dispatcher := pluginserver.NewDispatcher()
	require.NoError(t, pluginserver.LoadBuiltins(dispatcher, yang.WireMethodToPath(resolved),
		yang.PathToDescription(resolved), yang.PathToHelp(resolved), yang.PathToArgDefs(resolved), yang.BuildCommandTree(resolved)))

	d.server, err = zessh.NewServer(zessh.Config{
		Listen:      "127.0.0.1:0",
		HostKeyPath: filepath.Join(t.TempDir(), "host-key"),
		// The daemon's own wiring (infra_setup.go): live indirections over
		// the accepted AAA bundle, never a captured chain.
		Authenticator: liveAAABundleAuthenticator{},
		Authorizer:    liveAAABundleAuthorizer{},
	})
	require.NoError(t, err)
	// The daemon's SSH executor (sshWireImpl): the dispatcher judges every
	// operational command against the connection's bound policy.
	d.server.SetExecutorFactory(func(sessionCtx context.Context, username, remoteAddr string, authorizer plugin.Authorizer) zessh.CommandExecutor {
		return func(input string) (*plugin.RenderedResponse, error) {
			resp, dispatchErr := dispatcher.Dispatch(&pluginserver.CommandContext{
				RequestContext: sessionCtx,
				Username:       username,
				RemoteAddr:     remoteAddr,
				Authorizer:     sshSessionPolicy(authorizer),
				Sender:         plugin.OperatorSender(),
			}, input)
			if resp == nil {
				return &plugin.RenderedResponse{}, dispatchErr
			}
			return &plugin.RenderedResponse{Response: resp}, dispatchErr
		}
	})
	require.NoError(t, d.server.Start(t.Context(), nil, nil))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, d.server.Stop(ctx))
	})
	// The reload a running daemon performs after a commit: it accepts the
	// staged candidate, so the store's active config is what would run.
	d.server.SetSessionModelFactory(buildSessionModelFactory(d.server,
		infra.HookParams{Store: d.store, ConfigPath: authzConfigPath}, nil, d.reload))
	return d
}

func (d *authzDaemon) reload() error {
	d.reloads.Add(1)
	return storage.PromoteCandidate(d.store, authzConfigPath)
}

// policy extracts the profiles through the production extractor, with the
// editor profile's edit default set to editorEdit.
func (d *authzDaemon) policy(editorEdit string) *authz.Store {
	d.t.Helper()
	text := strings.NewReplacer("@HASH@", d.hash, "@EDITOR@", editorEdit).Replace(authzPolicyTemplate)
	tree, err := zeconfig.ParseTreeWithYANG(text, nil)
	require.NoError(d.t, err)
	store, err := infra.ExtractAuthzStore(tree)
	require.NoError(d.t, err)
	require.NotNil(d.t, store, "the policy declares profiles, so the extractor must build a store")
	return store
}

// reloadPolicy swaps the accepted AAA bundle for one whose editor profile
// has the given edit default, as a daemon reload does (swapAAABundle).
func (d *authzDaemon) reloadPolicy(editorEdit string) {
	d.t.Helper()
	next := &aaa.Bundle{
		Authenticator: &authz.LocalAuthenticator{Users: d.users},
		Authorizer:    authz.StoreAuthorizer{Store: d.policy(editorEdit)},
	}
	if previous := aaaBundle.Swap(next); previous != nil {
		require.NoError(d.t, previous.Close())
	}
}

// holds reports whether the store's active bytes of path contain value.
func (d *authzDaemon) holds(path, value string) bool {
	d.t.Helper()
	data, err := d.store.ReadFile(path)
	require.NoError(d.t, err)
	return strings.Contains(string(data), value)
}

// edited reports whether user's uncommitted edits of path (the session's
// change file, or the shared draft) contain value: the candidate the editor
// would commit.
func (d *authzDaemon) edited(path, user, value string) bool {
	d.t.Helper()
	for _, key := range []string{cli.ChangePath(path, user), cli.DraftPath(path)} {
		if !d.store.Exists(key) {
			continue
		}
		data, err := d.store.ReadFile(key)
		require.NoError(d.t, err)
		if strings.Contains(string(data), value) {
			return true
		}
	}
	return false
}

// waitState polls cond until it holds or authzWait passes.
func (d *authzDaemon) waitState(what string, cond func() bool) {
	d.t.Helper()
	deadline := time.Now().Add(authzWait)
	for !cond() {
		if time.Now().After(deadline) {
			d.t.Fatalf("state never reached: %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// authzScreen collects what the PTY session draws.
type authzScreen struct {
	mu   sync.Mutex
	text strings.Builder
}

func (s *authzScreen) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.text.Write(p)
}

// since answers what was drawn from offset on, with the escape sequences
// removed, and the current length.
func (s *authzScreen) since(offset int) (string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all := s.text.String()
	return ansi.Strip(all[offset:]), len(all)
}

// authzTerminal is one authenticated PTY session.
type authzTerminal struct {
	t      *testing.T
	stdin  io.Writer
	screen *authzScreen
	mark   int
}

// open logs user in over SSH and starts an interactive session. env carries
// the session request (sshclient.EnvConfigName, sshclient.EnvCLIMode), as
// `ze cli` sends it.
func (d *authzDaemon) open(user string, env ...[2]string) *authzTerminal {
	d.t.Helper()
	client, err := gossh.Dial("tcp", d.server.Address(), &gossh.ClientConfig{
		User:            user,
		Auth:            []gossh.AuthMethod{gossh.Password(authzPassword)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // Per-test server key.
		Timeout:         5 * time.Second,
	})
	require.NoError(d.t, err)
	d.t.Cleanup(func() { _ = client.Close() })
	session, err := client.NewSession()
	require.NoError(d.t, err)
	d.t.Cleanup(func() { _ = session.Close() })
	term := &authzTerminal{t: d.t, screen: &authzScreen{}}
	session.Stdout, session.Stderr = term.screen, term.screen
	term.stdin, err = session.StdinPipe()
	require.NoError(d.t, err)
	for _, pair := range env {
		require.NoError(d.t, session.Setenv(pair[0], pair[1]))
	}
	// Wide enough that no answer under test wraps.
	require.NoError(d.t, session.RequestPty("xterm", 60, 240, gossh.TerminalModes{gossh.ECHO: 0}))
	require.NoError(d.t, session.Shell())
	term.expectAny("ze#", "ze>")
	return term
}

// run types line and Enter, then waits until the line's answer is drawn: every
// fragment of answer, which MUST be text only the finished command draws (its
// result or its refusal), never the typed text itself. An allowed `set`
// answers the line it set with single spaces, so a test types it with double
// spaces to tell the answer from the input field. No key is typed while a
// command runs: a key typed then races the running command over the editor
// (plan/journal/per-request-state-on-a-shared-object.md, 2026-11-19 row).
func (term *authzTerminal) run(line string, answer ...string) {
	term.t.Helper()
	require.NotEmpty(term.t, answer, "a line without an answer to wait for lets the next line race it")
	_, term.mark = term.screen.since(0)
	_, err := io.WriteString(term.stdin, line+"\r")
	require.NoError(term.t, err)
	term.expect(answer...)
}

// expect waits until everything drawn since the last line holds every
// fragment.
func (term *authzTerminal) expect(fragments ...string) {
	term.t.Helper()
	term.wait(func(drawn string) bool {
		for _, fragment := range fragments {
			if !strings.Contains(drawn, fragment) {
				return false
			}
		}
		return true
	}, fragments)
}

// expectAny waits until what is drawn since the last send holds one fragment.
func (term *authzTerminal) expectAny(fragments ...string) {
	term.t.Helper()
	term.wait(func(drawn string) bool {
		for _, fragment := range fragments {
			if strings.Contains(drawn, fragment) {
				return true
			}
		}
		return false
	}, fragments)
}

func (term *authzTerminal) wait(match func(string) bool, fragments []string) {
	term.t.Helper()
	deadline := time.Now().Add(authzWait)
	for {
		drawn, _ := term.screen.since(term.mark)
		if match(drawn) {
			return
		}
		if time.Now().After(deadline) {
			term.t.Fatalf("screen never showed %q; drawn since the last line:\n%s", fragments, drawn)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// refusal is the first line the editor's gate answers for permission.
func refusal(permission string) string {
	return plugin.UnauthorizedMessage + ": " + permission
}

// TestSSHEditorAuthorizesConfigVerbsOverPTY drives the operator's own entry
// point: a password login over a real SSH server, a PTY session built by the
// real session factory, typed lines, and a policy the production extractor
// built from system.authorization text. Each subtest asserts what the screen
// shows and what the store holds (the session's uncommitted edits, the active
// config, the reloads), never a timing.
//
// VALIDATES: spec-ssh-editor-authorization AC-1, AC-2, AC-3, AC-4, AC-6,
// AC-7, AC-10, AC-14.
// PREVENTS: a profile that denies the edit section committing a change
// through the SSH session editor (the fail-open this spec closes).
func TestSSHEditorAuthorizesConfigVerbsOverPTY(t *testing.T) {
	t.Run("AC-1 read-only set refused, show answers", func(t *testing.T) {
		d := newAuthzDaemon(t, true)
		term := d.open("viewer")
		term.run("set system host denied-host", refusal(plugin.ConfigAuthSet))
		term.run("show system", authzSeedHost)
		require.False(t, d.edited(authzConfigPath, "viewer", "denied-host"), "a refused set changed the candidate")
		require.True(t, d.holds(authzConfigPath, authzSeedHost))
		require.Zero(t, d.reloads.Load())
	})

	t.Run("AC-2 read-only commit refused", func(t *testing.T) {
		d := newAuthzDaemon(t, true)
		term := d.open("viewer")
		term.run("commit now", refusal(plugin.ConfigAuthCommit))
		data, err := d.store.ReadFile(authzConfigPath)
		require.NoError(t, err)
		require.Equal(t, authzSeedConfig, string(data), "a refused commit changed the store")
		require.Zero(t, d.reloads.Load(), "a refused commit reloaded the daemon")
	})

	t.Run("AC-3 command-mode set shortcut refused", func(t *testing.T) {
		d := newAuthzDaemon(t, true)
		term := d.open("viewer", [2]string{sshclient.EnvCLIMode, sshclient.CLIModeCommand})
		term.run("set system host denied-host", refusal(plugin.ConfigAuthSet))
		require.False(t, d.edited(authzConfigPath, "viewer", "denied-host"))
	})

	t.Run("AC-4 top set refused, top show answers", func(t *testing.T) {
		d := newAuthzDaemon(t, true)
		term := d.open("viewer")
		term.run("top set system host denied-host", refusal(plugin.ConfigAuthSet))
		term.run("top show system", authzSeedHost)
		require.False(t, d.edited(authzConfigPath, "viewer", "denied-host"))
	})

	t.Run("AC-6 admin set and commit apply", func(t *testing.T) {
		d := newAuthzDaemon(t, true)
		term := d.open("admin")
		term.run("set  system  host  admin-host", "set system host admin-host")
		d.waitState("admin's set reaches the candidate", func() bool {
			return d.edited(authzConfigPath, "admin", "admin-host")
		})
		term.run("commit now", "Session committed")
		d.waitState("admin's commit reaches the active config through a reload", func() bool {
			return d.reloads.Load() > 0 && d.holds(authzConfigPath, "admin-host")
		})
	})

	t.Run("AC-7 per-verb profile allows set and commit, refuses delete", func(t *testing.T) {
		d := newAuthzDaemon(t, true)
		term := d.open("setter")
		term.run("set  system  host  setter-host", "set system host setter-host")
		d.waitState("the allowed set reaches the candidate", func() bool {
			return d.edited(authzConfigPath, "setter", "setter-host")
		})
		term.run("commit now", "Session committed")
		d.waitState("the allowed commit reaches the active config", func() bool {
			return d.holds(authzConfigPath, "setter-host")
		})
		term.run("delete system host", refusal(plugin.ConfigAuthDelete))
		// show answers from the candidate: the refused delete left it whole.
		term.run("show system", "setter-host")
		require.True(t, d.holds(authzConfigPath, "setter-host"), "a refused delete removed the host")
	})

	t.Run("AC-10 no authorization configured stays permissive", func(t *testing.T) {
		d := newAuthzDaemon(t, false)
		term := d.open("viewer")
		term.run("set  system  host  open-host", "set system host open-host")
		d.waitState("the set reaches the candidate", func() bool {
			return d.edited(authzConfigPath, "viewer", "open-host")
		})
		term.run("commit now", "Session committed")
		d.waitState("the commit reaches the active config", func() bool {
			return d.reloads.Load() > 0 && d.holds(authzConfigPath, "open-host")
		})
	})

	t.Run("AC-14 guide operator profile edits and is refused clear", func(t *testing.T) {
		d := newAuthzDaemon(t, true)
		term := d.open("operator")
		term.run("set  system  host  operator-host", "set system host operator-host")
		d.waitState("the operator's set reaches the candidate", func() bool {
			return d.edited(authzConfigPath, "operator", "operator-host")
		})
		term.run("commit now", "Session committed")
		d.waitState("the operator's commit reaches the active config", func() bool {
			return d.holds(authzConfigPath, "operator-host")
		})
		// `clear` is not a read verb, so the dispatcher judges it in the edit
		// section, where the profile allows only `config`.
		term.run("run clear interface counters all", plugin.UnauthorizedMessage)
	})
}

// TestSSHEditorAuthorizesNamedConfigurationOverPTY opens sessions that name
// the stored client configuration, as `ze cli` does with ZE_CONFIG_NAME, on a
// hub that declares the client (spec-fleet-6's check passes). The permission
// carries the configuration's name, so an entry naming the file restricts
// that file alone.
//
// VALIDATES: spec-ssh-editor-authorization AC-8, AC-9.
// PREVENTS: a commit to a hub's client configuration that the profile denies,
// and an entry for one file spilling onto the daemon's own configuration.
func TestSSHEditorAuthorizesNamedConfigurationOverPTY(t *testing.T) {
	named := [][2]string{{sshclient.EnvConfigName, authzClientConfig}, {sshclient.EnvCLIMode, sshclient.CLIModeEdit}}
	declareEdge := func(t *testing.T) {
		storagecli.InstallDataRPC(&storagecli.DataRPCTarget{ServesClient: func(name string) bool { return name == "edge" }})
		t.Cleanup(func() { storagecli.InstallDataRPC(nil) })
	}

	t.Run("AC-8 entry naming the file refuses its commit only", func(t *testing.T) {
		d := newAuthzDaemon(t, true)
		declareEdge(t)
		term := d.open("fleet", named...)
		term.run("set  system  host  fleet-host", "set system host fleet-host")
		d.waitState("the set on the named configuration reaches its candidate", func() bool {
			return d.edited(authzClientConfig, "fleet", "fleet-host")
		})
		term.run("commit now", refusal(plugin.ConfigAuthCommit+" "+authzClientConfig))
		data, err := d.store.ReadFile(authzClientConfig)
		require.NoError(t, err)
		require.Equal(t, authzSeedConfig, string(data), "a refused commit changed the named configuration")

		own := d.open("fleet")
		own.run("set  system  host  fleet-own", "set system host fleet-own")
		d.waitState("the set on the daemon's own configuration reaches its candidate", func() bool {
			return d.edited(authzConfigPath, "fleet", "fleet-own")
		})
		own.run("commit now", "Session committed")
		d.waitState("the same user's commit of the daemon's own configuration applies", func() bool {
			return d.holds(authzConfigPath, "fleet-own")
		})
	})

	t.Run("AC-9 admin commits the named configuration", func(t *testing.T) {
		d := newAuthzDaemon(t, true)
		declareEdge(t)
		term := d.open("admin", named...)
		term.run("set  system  host  edge-host", "set system host edge-host")
		d.waitState("the set reaches the named candidate", func() bool {
			return d.edited(authzClientConfig, "admin", "edge-host")
		})
		term.run("commit now", "Session committed")
		d.waitState("the commit writes the named configuration", func() bool {
			return d.holds(authzClientConfig, "edge-host")
		})
		require.Zero(t, d.reloads.Load(), "a named configuration is store-only: it never reloads the daemon")
	})
}

// TestConfirmedRollbackSurvivesLostPermission opens the daemon's real
// confirmed-commit window (startConfirmWindow, whose revert is confirmRevert),
// commits `commit confirmed` as a user whose profile allows editing, then
// reloads the policy so that profile denies the whole edit section before the
// rollback can land: the revert's reload is held until the swap, so the order
// does not depend on the clock. The deadline MUST still revert the
// configuration: the
// revert is the window worker writing the rollback as the candidate and
// reloading, and no line passes through the editor's dispatch or its gate.
// The same session's next `set` is refused, which shows the denial is in
// force for typed verbs at the moment the daemon's own revert runs.
//
// VALIDATES: spec-ssh-editor-authorization AC-13, R-6.
// PREVENTS: a user who loses the commit permission mid-window leaving an
// unconfirmed change in place forever.
func TestConfirmedRollbackSurvivesLostPermission(t *testing.T) {
	d := newAuthzDaemon(t, true)
	// The window's reload is the revert's alone (confirmRevert); the editors
	// commit through d.reload. Holding it until the policy swap makes the
	// order of the AC hold by construction, whatever the machine's load: the
	// deadline may fire early, but the rollback cannot reach the active
	// config before the permission is lost.
	permissionLost := make(chan struct{})
	var lose sync.Once
	window := startConfirmWindow(d.store, authzConfigPath, func() error {
		<-permissionLost
		return d.reload()
	})
	require.NotNil(t, window)
	t.Cleanup(func() { stopConfirmWindow(window) })
	// Registered after the stop, so it runs first: a failed test never leaves
	// the worker waiting on the gate while Stop waits for the worker.
	t.Cleanup(func() { lose.Do(func() { close(permissionLost) }) })

	term := d.open("editor")
	term.run("set  system  host  confirmed-host", "set system host confirmed-host")
	d.waitState("the set reaches the candidate", func() bool {
		return d.edited(authzConfigPath, "editor", "confirmed-host")
	})
	// run reads the answer before the next line is typed: see run.
	term.run("commit confirmed 5", "Confirm within 5s")
	d.waitState("the confirmed commit applies", func() bool {
		return d.holds(authzConfigPath, "confirmed-host")
	})

	d.reloadPolicy("deny")
	require.True(t, d.holds(authzConfigPath, "confirmed-host"),
		"the rollback reached the active config before the permission was lost, or this test proves nothing")
	// The denial is in force for typed verbs before the rollback may land.
	term.run("set system host after-loss", refusal(plugin.ConfigAuthSet))
	lose.Do(func() { close(permissionLost) })

	d.waitState("the deadline reverts the unconfirmed commit", func() bool {
		timeouts, err := window.Timeouts()
		require.NoError(t, err)
		return timeouts == 1 && !d.holds(authzConfigPath, "confirmed-host") && d.holds(authzConfigPath, authzSeedHost)
	})
	require.GreaterOrEqual(t, d.reloads.Load(), int32(2), "the commit and the revert each reload")
}

//go:build ze_ssh && ze_bgp

// Design: docs/guide/config-editor.md -- drain SSH histories before borrowed store release.
// Related: infra_setup.go and main.go -- installed infrastructure hook and daemon defers.

package hub

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/cli"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/engine"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zessh "github.com/ze-software/ze/internal/component/ssh"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/ze"
	"github.com/ze-software/ze/pkg/zefs"
)

type infraHistoryPublication struct {
	key  string
	data string
}

type infraHistoryStore struct {
	storage.Storage
	entered chan struct{}
	release chan struct{}
	closed  chan struct{}
	first   sync.Once
	mu      sync.Mutex
	writes  []infraHistoryPublication
	atClose []infraHistoryPublication
}

func (s *infraHistoryStore) WriteFile(key string, data []byte, mode fs.FileMode) error {
	if !strings.HasPrefix(key, zefs.KeyHistory.Prefix()) {
		return s.Storage.WriteFile(key, data, mode)
	}
	s.first.Do(func() {
		close(s.entered)
		<-s.release
	})
	if err := s.Storage.WriteFile(key, data, mode); err != nil {
		return err
	}
	s.mu.Lock()
	s.writes = append(s.writes, infraHistoryPublication{key: key, data: string(data)})
	s.mu.Unlock()
	return nil
}

func (s *infraHistoryStore) Close() error {
	s.mu.Lock()
	s.atClose = slices.Clone(s.writes)
	s.mu.Unlock()
	err := s.Storage.Close()
	close(s.closed)
	return err
}

// infraHistoryStartupFailure uses the existing engine-subsystem construction
// seam to place the actual installed infra hook before controlled startup
// refusal or accepted daemon shutdown. It neither forwards nor closes SSH.
type infraHistoryStartupFailure struct {
	start func(ze.EventBus) error
}

func (*infraHistoryStartupFailure) Name() string { return "infra-history-startup-failure" }
func (s *infraHistoryStartupFailure) Start(_ context.Context, bus ze.EventBus, _ ze.ConfigProvider) error {
	return s.start(bus)
}
func (*infraHistoryStartupFailure) Stop(context.Context) error                      { return nil }
func (*infraHistoryStartupFailure) Reload(context.Context, ze.ConfigProvider) error { return nil }

// TestInfraSSHOwnerDrainsBeforeStoreCloseOnStartupFailure drives actual Run,
// its installed infra hook, the gated real sshBuildImpl, a real authenticated PTY
// and its actual Model. Engine startup then fails while the accepted history's
// real store write is held. The caller closes the real store immediately when
// Run returns, exactly as ze_core_start does. Correct ownership closes the real
// SSH transport first and cannot reach store Close until the writer is released;
// the old discarded hook handle instead reaches Close with no useful payload.
// No future owner API, fake closer, readiness poll or sleep is used.
func TestInfraSSHOwnerDrainsBeforeStoreCloseOnStartupFailure(t *testing.T) {
	testInfraSSHHistoryOwner(t, true)
}

// TestInfraSSHOwnerDrainsBeforeStoreCloseOnDaemonShutdown retains the same real
// Run/hook/authenticated-PTY/Model/publication assertions, but startup succeeds
// and the actual plugin server accepts a normal shutdown request. Both Run's
// explicit stop and deferred ownership must finish before caller store Close.
func TestInfraSSHOwnerDrainsBeforeStoreCloseOnDaemonShutdown(t *testing.T) {
	testInfraSSHHistoryOwner(t, false)
}

func testInfraSSHHistoryOwner(t *testing.T, startupFailure bool) {
	t.Helper()
	resetAAABundleForTest(t)
	dir := pinConfigDir(t)
	path := filepath.Join(dir, "startup.conf")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	previousEphemeral := env.Get("ze.ssh.ephemeral")
	if err := env.Set("ze.ssh.ephemeral", ""); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = env.Set("ze.ssh.ephemeral", previousEphemeral) }()
	store := &infraHistoryStore{
		Storage: newTestFileStore(t, path), entered: make(chan struct{}),
		release: make(chan struct{}), closed: make(chan struct{}),
	}
	// SourceMode deliberately recognizes only the outer explicit binding.
	// Preserve Run's real source guard while intercepting writes and Close.
	borrowed := storage.BindConfigSource(store, path, storage.ConfigSourceFile)
	credential := authWiringBcryptUser(t, "operator", "history-password", "admin")
	oldBuild, oldRegister := sshBuild, bngRegister
	defer func() {
		sshBuild, bngRegister = oldBuild, oldRegister
		infra.SetHook(nil)
	}()
	created := make(chan cli.Model, 1)
	var builtServer *zessh.Server
	sshBuild = func(in *sshBuildInputs) sshServer {
		// The real listener/auth/session machinery stays intact. Pin only test
		// credentials and the actual model whose accepted publication is held.
		in.Authenticator = &authz.LocalAuthenticator{Users: []authz.UserConfig{credential}}
		built := sshBuildImpl(in)
		if built == nil {
			return nil
		}
		var ok bool
		builtServer, ok = built.(*zessh.Server)
		if !ok {
			t.Fatalf("SSH builder returned %T, want *ssh.Server", built)
		}
		builtServer.SetSessionModelFactory(func(username, _ string, _ plugin.Authorizer, _ zessh.SessionRequest) (tea.Model, error) {
			model := cli.NewCommandModel(cli.FilesystemAuthorityUnknown)
			model.SetHistory(cli.NewHistory(store, username))
			initial := model
			model.SetInput("show daemon-owned")
			_, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			created <- initial
			return initial, nil
		})
		return built
	}
	ready := make(chan *zessh.Server, 1)
	failStartup := make(chan struct{})
	var fail, release sync.Once
	failure := errors.New("injected engine startup refusal after real infrastructure construction")
	bngRegister = func(_ *zeconfig.Tree, _ map[string]any, eng *engine.Engine) ([]webPortalService, error) {
		err := eng.RegisterSubsystem(&infraHistoryStartupFailure{start: func(bus ze.EventBus) error {
			infra.Run(infra.HookParams{
				Reactor: reactor.New(&reactor.Config{}), Store: borrowed,
				ConfigDir: dir, ConfigPath: path,
				SSHConfig: infra.SSHExtractedConfig{
					Listen: "127.0.0.1:0", HostKeyPath: filepath.Join(dir, "host-key"), HasConfig: true,
				},
			})
			ready <- builtServer
			<-failStartup
			if startupFailure {
				return failure
			}
			// This is the actual server the daemon's waitLoop observes, not a
			// test stop callback. Requesting before Start is supported by its
			// normal shutdownRequested channel, wired before engine startup.
			apiServer, ok := bus.(*pluginserver.Server)
			if !ok {
				return errors.New("daemon engine did not receive its actual plugin server")
			}
			apiServer.SignalShutdownRequested()
			return nil
		}})
		return nil, err
	}
	ended := make(chan struct{})
	exit := make(chan int, 1)
	var client *gossh.Client
	var initial *cli.Model
	defer func() {
		fail.Do(func() { close(failStartup) })
		release.Do(func() { close(store.release) })
		if client != nil {
			_ = client.Close()
		}
		if builtServer != nil {
			_ = builtServer.Stop(context.Background())
		}
		if initial != nil {
			_ = initial.Close()
		}
		<-ended
	}()
	go func() {
		defer close(ended)
		exit <- Run(borrowed, path, nil, 0, -1, false, "", false, "", "")
		_ = borrowed.Close()
	}()
	var server *zessh.Server
	select {
	case server = <-ready:
	case <-ended:
		t.Fatalf("Run exited before the actual infra hook was reached: %d", <-exit)
	}
	if server == nil {
		t.Fatal("the actual installed infra hook did not build the real SSH server")
	}
	var err error
	client, err = gossh.Dial("tcp", server.Address(), &gossh.ClientConfig{
		User: "operator", Auth: []gossh.AuthMethod{gossh.Password("history-password")},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // Per-test real server key.
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close() //nolint:errcheck // Stop may already have closed it.
	session.Stdout, session.Stderr = io.Discard, io.Discard
	if err := session.RequestPty("xterm", 24, 80, gossh.TerminalModes{gossh.ECHO: 0}); err != nil {
		t.Fatal(err)
	}
	if err := session.Shell(); err != nil {
		t.Fatal(err)
	}
	model := <-created
	initial = &model
	<-store.entered
	transportEnded := make(chan struct{})
	go func() {
		_ = client.Wait()
		close(transportEnded)
	}()
	fail.Do(func() { close(failStartup) })
	select {
	case <-store.closed:
		t.Error("Run released its borrowed store before the held SSH history drained")
	case <-transportEnded:
		// Actual server-owned network closure precedes the history join. The
		// held writer still cannot publish until this test releases it below.
	}
	release.Do(func() { close(store.release) })
	<-ended
	wantExit := 0
	if startupFailure {
		wantExit = 1
	}
	if got := <-exit; got != wantExit {
		t.Errorf("Run exit = %d, want %d", got, wantExit)
	}
	want := []infraHistoryPublication{{key: zefs.KeyHistory.Prefix() + "operator/" + cli.ModeOperational.String(), data: "show daemon-owned"}}
	store.mu.Lock()
	got := slices.Clone(store.atClose)
	store.mu.Unlock()
	if !slices.Equal(got, want) {
		t.Errorf("actual history publications before store Close = %v, want %v", got, want)
	}
}

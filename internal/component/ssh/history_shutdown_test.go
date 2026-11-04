// Design: docs/guide/config-editor.md -- SSH shutdown joins model publication before store release.
// Related: history_owner_test.go -- authenticated PTY/no-PTY integration positives.

package ssh

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"testing/synctest"

	tea "charm.land/bubbletea/v2"
	charmssh "charm.land/ssh"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
)

// historySessionContext supplies only transport metadata and session values;
// every model, history and close operation exercised below is production code.
type historySessionContext struct {
	context.Context
	sync.Mutex
	values sync.Map
}

func (*historySessionContext) User() string          { return "operator" }
func (*historySessionContext) SessionID() string     { return "history-session" }
func (*historySessionContext) ClientVersion() string { return "test" }
func (*historySessionContext) ServerVersion() string { return "test" }
func (*historySessionContext) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}
}
func (*historySessionContext) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2222}
}
func (*historySessionContext) Permissions() *charmssh.Permissions {
	return &charmssh.Permissions{Permissions: &gossh.Permissions{}}
}
func (c *historySessionContext) SetValue(key, value any) { c.values.Store(key, value) }
func (c *historySessionContext) Value(key any) any {
	if value, ok := c.values.Load(key); ok {
		return value
	}
	return c.Context.Value(key)
}

type historySession struct {
	charmssh.Session
	ctx     *historySessionContext
	command []string
	exit    int
}

func (s *historySession) Context() charmssh.Context    { return s.ctx }
func (s *historySession) User() string                 { return s.ctx.User() }
func (s *historySession) RemoteAddr() net.Addr         { return s.ctx.RemoteAddr() }
func (*historySession) Environ() []string              { return nil }
func (s *historySession) Command() []string            { return s.command }
func (*historySession) Stderr() io.ReadWriter          { return historyDiscard{} }
func (*historySession) Write(data []byte) (int, error) { return len(data), nil }
func (s *historySession) Exit(status int) error        { s.exit = status; return nil }

type historyDiscard struct{}

func (historyDiscard) Write(data []byte) (int, error) { return len(data), nil }
func (historyDiscard) Read([]byte) (int, error)       { return 0, io.EOF }

// observedShutdownContext marks the actual canceled branch of vendor Shutdown,
// not merely Done readiness. The test can then close its real idle connection
// and join the vendor's external-connection waiter without a select race.
type observedShutdownContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func (c *observedShutdownContext) Err() error {
	err := c.Context.Err()
	if err != nil {
		c.once.Do(func() { close(c.observed) })
	}
	return err
}

// primeHistoryShutdown creates/closes the vendor's lazy done channel outside
// synctest, where Serve receives it. The tracked idle connection stays alive,
// so only the canceled branch can complete and no model owner is stopped yet.
func primeHistoryShutdown(t *testing.T, server *Server) {
	t.Helper()
	server.mu.Lock()
	network := server.wish
	server.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := network.Shutdown(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("prime real network shutdown = %v, want context.Canceled", err)
	}
}

type historyShutdownWrite struct{ key, payload string }

type historyShutdownStore struct {
	storage.Storage
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	mu      sync.Mutex
	writes  []historyShutdownWrite
	atClose []historyShutdownWrite
}

func (s *historyShutdownStore) WriteFile(key string, data []byte, perm fs.FileMode) error {
	s.once.Do(func() { close(s.entered); <-s.release })
	err := s.Storage.WriteFile(key, data, perm)
	if err == nil {
		s.mu.Lock()
		s.writes = append(s.writes, historyShutdownWrite{key: key, payload: string(data)})
		s.mu.Unlock()
	}
	return err
}

func (s *historyShutdownStore) Close() error {
	s.mu.Lock()
	s.atClose = slices.Clone(s.writes)
	s.mu.Unlock()
	return s.Storage.Close()
}

// TestSSHStopDrainsDisconnectedModelBeforeStoreClose is compatible with the old
// Stop API. An actual sessionCloseMiddleware and actual Model are durably blocked
// draining accepted operational+config history after the inner handler returns.
// A real idle SSH connection keeps network Shutdown pending until its canceled
// context is observed. Even that network error must not let the daemon's next
// store Close overtake either final publication. No fake closer or timing poll.
func TestSSHStopDrainsDisconnectedModelBeforeStoreClose(t *testing.T) {
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	client, err := gossh.Dial("tcp", server.Address(), panicTestClientConfig("operator"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close() //nolint:errcheck // Rescue the real idle network connection.
	idle, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer idle.Close() //nolint:errcheck // Opening the channel proves the connection is tracked.
	primeHistoryShutdown(t, server)
	memory := storage.CreateMemory()
	defer memory.Close() //nolint:errcheck // Rescue if the owner path fails.
	synctest.Test(t, func(t *testing.T) {
		store := &historyShutdownStore{Storage: memory, entered: make(chan struct{}), release: make(chan struct{})}
		var release sync.Once
		unblock := func() { release.Do(func() { close(store.release) }) }
		defer unblock()
		server.SetSessionModelFactory(func(username, _ string, _ plugin.Authorizer, _ SessionRequest) (tea.Model, error) {
			model := cli.NewCommandModel(cli.FilesystemAuthorityUnknown)
			model.SetHistory(cli.NewHistory(store, username))
			initial := model
			model.SetInput("show first")
			model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			model.SetMode(cli.ModeConfig)
			model.SetInput("run show queued")
			model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			return initial, nil
		})
		session := &historySession{ctx: &historySessionContext{Context: context.Background()}}
		disconnected := make(chan struct{})
		sessionEnded := make(chan struct{})
		innerReturned := make(chan struct{})
		go func() {
			defer close(sessionEnded)
			server.sessionCloseMiddleware()(func(sess charmssh.Session) {
				server.teaHandler(sess)
				<-disconnected
				close(innerReturned)
			})(session)
		}()
		var disconnect sync.Once
		defer func() { disconnect.Do(func() { close(disconnected) }); unblock(); <-sessionEnded }()
		synctest.Wait()
		select {
		case <-store.entered:
		default:
			t.Fatal("accepted model history never reached storage")
		}
		disconnect.Do(func() { close(disconnected) })
		synctest.Wait()
		select {
		case <-innerReturned:
		default:
			t.Fatal("the actual session cleanup was not admitted")
		}
		select {
		case <-sessionEnded:
			t.Fatal("actual Model.Close returned while history remained held")
		default:
		}
		base, cancel := context.WithCancel(context.Background())
		cancel()
		ctx := &observedShutdownContext{Context: base, observed: make(chan struct{})}
		stopped := make(chan error, 1)
		joined := make(chan struct{})
		go func() {
			defer close(joined)
			stopErr := server.Stop(ctx)
			stopped <- errors.Join(stopErr, store.Close())
		}()
		defer func() { unblock(); <-joined }()
		<-ctx.observed
		client.Close() //nolint:errcheck // Release real connWg after observing the shutdown error branch.
		synctest.Wait()
		select {
		case err := <-stopped:
			t.Errorf("Server.Stop returned before disconnected model history drained: %v", err)
			stopped <- err
		default:
		}
		unblock()
		if err := <-stopped; !errors.Is(err, context.Canceled) {
			t.Errorf("network shutdown error = %v, want original context cancellation", err)
		}
		<-sessionEnded
		store.mu.Lock()
		defer store.mu.Unlock()
		want := []historyShutdownWrite{
			{key: "meta/history/operator/operational", payload: "show first"},
			{key: "meta/history/operator/config", payload: "run show queued"},
		}
		if !slices.Equal(store.atClose, want) {
			t.Errorf("publications before store Close = %v, want %v", store.atClose, want)
		}
	})
}

// TestSSHSessionCleanupDrainsBeforeTranscriptClose observes cleanup causally:
// the real inner handler has returned and synctest proves actual Model.Close
// is blocked draining history. The real transcript file must still be open at
// that point, and closed only after release and exact publication.
func TestSSHSessionCleanupDrainsBeforeTranscriptClose(t *testing.T) {
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	file, err := os.Create(filepath.Join(t.TempDir(), "causal-transcript"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // Rescue after actual owner closure.
	transcript := cli.NewTranscriptWriter(file, "operator", "test")
	memory := storage.CreateMemory()
	defer memory.Close() //nolint:errcheck // Borrowed store outlives the model owner.
	// Initialize and stop the vendor network outside the bubble; model
	// admission and its final Server.Stop join belong inside this bubble.
	if err := server.wish.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	synctest.Test(t, func(t *testing.T) {
		defer func() {
			if err := server.Stop(context.Background()); err != nil {
				t.Errorf("stop cleanup fixture: %v", err)
			}
		}()
		store := &historyShutdownStore{Storage: memory, entered: make(chan struct{}), release: make(chan struct{})}
		var release sync.Once
		unblock := func() { release.Do(func() { close(store.release) }) }
		server.SetSessionModelFactory(func(username, _ string, _ plugin.Authorizer, _ SessionRequest) (tea.Model, error) {
			model := cli.NewCommandModel(cli.FilesystemAuthorityUnknown)
			model.SetHistory(cli.NewHistory(store, username))
			model.SetTranscript(transcript)
			initial := model
			model.SetInput("show cleanup")
			model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			return initial, nil
		})
		innerReturned := make(chan struct{})
		ended := make(chan struct{})
		session := &historySession{ctx: &historySessionContext{Context: context.Background()}}
		go func() {
			defer close(ended)
			server.sessionCloseMiddleware()(func(sess charmssh.Session) {
				server.teaHandler(sess)
				close(innerReturned)
			})(session)
		}()
		defer func() { unblock(); <-ended }()
		synctest.Wait()
		select {
		case <-store.entered:
		default:
			t.Fatal("real model never submitted history")
		}
		select {
		case <-innerReturned:
		default:
			t.Fatal("session cleanup was not causally admitted")
		}
		select {
		case <-ended:
			t.Fatal("cleanup returned before held history drained")
		default:
		}
		if _, err := file.WriteString("open during admitted cleanup\n"); err != nil {
			t.Errorf("transcript closed before admitted history cleanup drained: %v", err)
		}
		unblock()
		<-ended
		if _, err := file.WriteString("must be closed"); !errors.Is(err, os.ErrClosed) {
			t.Errorf("transcript after cleanup = %v, want os.ErrClosed", err)
		}
		if got := cli.NewHistory(memory, "operator").Load(cli.ModeOperational.String()); !slices.Equal(got, []string{"show cleanup"}) {
			t.Errorf("history after causal cleanup = %v", got)
		}
	})
}

// TestSSHRestartKeepsOldModelAdmissionSealed retains the actual old generation's
// middleware across Stop/Start. A late old handler cannot acquire a model from
// the new run; the fresh middleware must still construct and close a real model.
func TestSSHRestartKeepsOldModelAdmissionSealed(t *testing.T) {
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	calls := 0
	server.SetSessionModelFactory(func(_, _ string, _ plugin.Authorizer, _ SessionRequest) (tea.Model, error) {
		calls++
		return cli.NewCommandModel(cli.FilesystemAuthorityUnknown), nil
	})
	old := server.sessionCloseMiddleware()(func(sess charmssh.Session) { server.teaHandler(sess) })
	if err := server.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	stopped := &historySession{ctx: &historySessionContext{Context: context.Background()}}
	old(stopped)
	if calls != 0 {
		t.Errorf("stopped generation constructed %d late models", calls)
	}
	if stopped.exit != 1 {
		t.Errorf("stopped generation exit = %d, want refusal 1", stopped.exit)
	}
	if err := server.Start(t.Context(), nil, nil); err != nil {
		t.Fatal(err)
	}
	stale := &historySession{ctx: &historySessionContext{Context: context.Background()}}
	old(stale)
	if calls != 0 {
		t.Errorf("old generation constructed %d new models after restart", calls)
	}
	if stale.exit != 1 {
		t.Errorf("stale generation exit = %d, want refusal 1", stale.exit)
	}
	unowned := &historySession{ctx: &historySessionContext{Context: context.Background()}}
	if model, _ := server.teaHandler(unowned); model != nil {
		t.Error("unowned callback constructed a model without a cleanup cell")
	}
	if unowned.exit != 1 {
		t.Errorf("unowned callback exit = %d, want explicit refusal 1", unowned.exit)
	}
	if calls != 0 {
		t.Errorf("unowned callback reached factory %d times", calls)
	}
	fresh := &historySession{ctx: &historySessionContext{Context: context.Background()}}
	server.sessionCloseMiddleware()(func(sess charmssh.Session) { server.teaHandler(sess) })(fresh)
	if calls != 1 {
		t.Errorf("fresh generation constructed %d models, want exactly one", calls)
	}
}

// TestSSHStopJoinsFactoryAlreadyAdmitted seals admission while the real factory
// is in flight. The factory must still be able to take s.mu through an existing
// getter, return its real Model, and drain its accepted history before Stop
// returns. Reserving ownership only after factory return loses this case.
func TestSSHStopJoinsFactoryAlreadyAdmitted(t *testing.T) {
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	client := dialPanicTest(t, server, "operator")
	idle, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer idle.Close() //nolint:errcheck // The channel proves real connection tracking.
	primeHistoryShutdown(t, server)
	memory := storage.CreateMemory()
	defer memory.Close() //nolint:errcheck // Borrowed store outlives accepted model cleanup.
	synctest.Test(t, func(t *testing.T) {
		store := &historyShutdownStore{Storage: memory, entered: make(chan struct{}), release: make(chan struct{})}
		close(store.release)
		factoryEntered := make(chan struct{})
		factoryRelease := make(chan struct{})
		var release sync.Once
		unblock := func() { release.Do(func() { close(factoryRelease) }) }
		server.SetSessionModelFactory(func(username, remote string, authorizer plugin.Authorizer, _ SessionRequest) (tea.Model, error) {
			close(factoryEntered)
			<-factoryRelease
			// This existing public getter takes s.mu. Stop MUST NOT hold that
			// mutex while waiting for admitted model construction/cleanup.
			server.ExecutorForUser(username, remote, authorizer)
			model := cli.NewCommandModel(cli.FilesystemAuthorityUnknown)
			model.SetHistory(cli.NewHistory(store, username))
			initial := model
			model.SetInput("show admitted-factory")
			model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			return initial, nil
		})
		ended := make(chan struct{})
		session := &historySession{ctx: &historySessionContext{Context: context.Background()}}
		go func() {
			defer close(ended)
			server.sessionCloseMiddleware()(func(sess charmssh.Session) { server.teaHandler(sess) })(session)
		}()
		defer func() { unblock(); <-ended }()
		synctest.Wait()
		select {
		case <-factoryEntered:
		default:
			t.Fatal("real factory was not admitted")
		}
		base, cancel := context.WithCancel(context.Background())
		cancel()
		ctx := &observedShutdownContext{Context: base, observed: make(chan struct{})}
		stopped := make(chan error, 1)
		joined := make(chan struct{})
		go func() { defer close(joined); stopped <- server.Stop(ctx) }()
		defer func() { unblock(); <-joined }()
		<-ctx.observed
		client.Close() //nolint:errcheck // Release real connWg after observing the shutdown error branch.
		synctest.Wait()
		select {
		case err := <-stopped:
			t.Errorf("Stop returned before admitted factory/real model cleanup: %v", err)
			stopped <- err
		default:
		}
		unblock()
		if err := <-stopped; !errors.Is(err, context.Canceled) {
			t.Errorf("lost network shutdown error: %v", err)
		}
		<-ended
		if got := cli.NewHistory(memory, "operator").Load(cli.ModeOperational.String()); !slices.Equal(got, []string{"show admitted-factory"}) {
			t.Errorf("admitted factory publication = %v", got)
		}
	})
}

// TestSSHExecStopCallbackDoesNotJoinItself drives the existing synchronous exec
// lifecycle callback into real Server.Stop. Only TUI/model owners may be joined;
// treating every handler as a model owner makes this callback join itself.
func TestSSHExecStopCallbackDoesNotJoinItself(t *testing.T) {
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	client := dialPanicTest(t, server, "operator")
	idle, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer idle.Close() //nolint:errcheck // Tracked connection for the real canceled network branch.
	primeHistoryShutdown(t, server)
	synctest.Test(t, func(t *testing.T) {
		base, cancel := context.WithCancel(context.Background())
		cancel()
		ctx := &observedShutdownContext{Context: base, observed: make(chan struct{})}
		stopped := make(chan error, 1)
		server.SetShutdownFunc(func() { stopped <- server.Stop(ctx) })
		ended := make(chan struct{})
		session := &historySession{
			ctx:     &historySessionContext{Context: context.Background()},
			command: []string{commandStop},
		}
		handler := server.execMiddleware()(server.sessionCloseMiddleware()(func(charmssh.Session) {
			t.Error("exec stop reached TUI/model admission")
		}))
		go func() { defer close(ended); handler(session) }()
		<-ctx.observed
		client.Close() //nolint:errcheck // Cleanup after observing the real network error branch.
		synctest.Wait()
		select {
		case <-ended:
		default:
			t.Fatal("synchronous exec stop callback joined its own handler")
		}
		if err := <-stopped; !errors.Is(err, context.Canceled) {
			t.Errorf("exec callback lost the original network error: %v", err)
		}
		if session.exit != 0 {
			t.Errorf("authorized exec stop exit = %d, want success 0", session.exit)
		}
	})
}

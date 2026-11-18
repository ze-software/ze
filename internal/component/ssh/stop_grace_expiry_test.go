// Design: docs/guide/config-editor.md -- network grace expiry does not abandon accepted history.
// Related: ssh.go -- Stop owns forced network closure and the separate model join.

package ssh

import (
	"context"
	"errors"
	"net"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	charmssh "charm.land/ssh"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
)

// stopGraceClient holds a real authenticated connection in the vendor's connWg.
// Opening a channel proves admission before a shutdown can examine that group.
func stopGraceClient(t *testing.T, server *Server) *gossh.Client {
	t.Helper()
	client, err := gossh.Dial("tcp", server.Address(), panicTestClientConfig("operator"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return client
}

// stopGraceListener closes its real socket but reports an independent transport
// failure. Actual Serve registers it before serveListener returns. Shutdown's
// first close removes it from the vendor map, so a later Close cannot recover
// a cause discarded by Shutdown's deadline branch.
type stopGraceListener struct {
	net.Listener
	failure error
}

func (l *stopGraceListener) Close() error {
	if err := l.Listener.Close(); err != nil {
		return errors.Join(err, l.failure)
	}
	return l.failure
}

func addStopGraceListener(t *testing.T, server *Server, failure error) {
	t.Helper()
	var config net.ListenConfig
	listener, err := config.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	if err := server.serveListener(server.wish, &stopGraceListener{Listener: listener, failure: failure}); err != nil {
		t.Fatal(err)
	}
}

// A live connection prevents graceful completion. The expired allowance must
// force its closure, and a completed model join makes this a successful stop.
func TestSSHStopCompletesAfterNetworkGraceExpires(t *testing.T) {
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	client := stopGraceClient(t, server)
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer cancel()
	if err := server.Stop(ctx); err != nil {
		t.Errorf("Stop after successful forced closure and model join = %v, want nil", err)
	}
	if err := client.Wait(); err == nil {
		t.Error("server-owned connection ended without a transport closure")
	}
}

// Neither native Shutdown nor its Stop owner may discard a listener-close
// failure just because a still-open connection also exhausts the allowance.
func TestSSHStopKeepsListenerFailureWhenNetworkGraceExpires(t *testing.T) {
	for _, test := range []struct {
		name string
		stop bool
	}{{name: "native-shutdown"}, {name: "stop-owner", stop: true}} {
		t.Run(test.name, func(t *testing.T) {
			server := answerServer(t, func(string) (*plugin.Response, error) {
				return plugin.NewResponse(plugin.StatusDone, nil), nil
			})
			client := stopGraceClient(t, server)
			failure := errors.New("listener transport close failed")
			addStopGraceListener(t, server, failure)
			ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
			defer cancel()
			var err error
			if test.stop {
				err = server.Stop(ctx)
			} else {
				err = server.wish.Shutdown(ctx)
				// Shutdown deliberately leaves this connection alive. Its owner
				// supplies the forced closure after observing the native result.
				if closeErr := server.wish.Close(); closeErr != nil {
					t.Errorf("forced network closure = %v", closeErr)
				}
			}
			if !errors.Is(err, failure) {
				t.Errorf("shutdown lost listener-close cause: %v", err)
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("shutdown lost accompanying deadline: %v", err)
			}
			if waitErr := client.Wait(); waitErr == nil {
				t.Error("server-owned connection ended without a transport closure")
			}
		})
	}
}

// The listener can return the native deadline sentinel before any allowance
// expires. That is a transport failure, not permission to report a clean stop.
func TestSSHStopKeepsTransportDeadlineWithoutGraceExpiry(t *testing.T) {
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	addStopGraceListener(t, server, context.DeadlineExceeded)
	if err := server.Stop(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Stop discarded a transport deadline with a healthy context: %v", err)
	}
}

// Normal network grace expiry cannot hide an independently overdue accepted
// history write. The real model's Close and publication stay held throughout
// Stop, then finish when this test releases the storage call.
func TestSSHStopKeepsModelDrainFailureWhenNetworkGraceExpires(t *testing.T) {
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	stopGraceClient(t, server)
	primeHistoryShutdown(t, server)
	memory := storage.CreateMemory()
	defer memory.Close() //nolint:errcheck // The test owns the backing store.
	synctest.Test(t, func(t *testing.T) {
		store := &historyShutdownStore{Storage: memory, entered: make(chan struct{}), release: make(chan struct{})}
		var release sync.Once
		unblock := func() { release.Do(func() { close(store.release) }) }
		server.SetSessionModelFactory(func(_ context.Context, username, _ string, _ plugin.Authorizer, _ SessionRequest) (tea.Model, error) {
			model := cli.NewCommandModel(cli.FilesystemAuthorityUnknown)
			model.SetHistory(cli.NewHistory(store, username))
			initial := model
			model.SetInput("show grace-held")
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
		case <-store.entered:
		default:
			t.Fatal("the actual accepted history never reached storage")
		}
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()
		began := time.Now()
		if err := server.Stop(ctx); !errors.Is(err, errModelDrainOverdue) {
			t.Errorf("Stop over held history after network grace = %v, want errModelDrainOverdue", err)
		}
		if waited := time.Since(began); waited != modelDrainLimitDefault {
			t.Errorf("model join waited %s, want %s", waited, modelDrainLimitDefault)
		}
		select {
		case <-ended:
			t.Error("the model finished while its history write remained held")
		default:
		}
		unblock()
		<-ended
		history, err := cli.NewHistory(memory, "operator").Load(cli.ModeOperational.String())
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(history, []string{"show grace-held"}) {
			t.Errorf("late accepted history = %v", history)
		}
	})
}

// Shutdown's listener-close result needs a distinct origin even when its cause
// spells the native deadline sentinel. Its Stop consumer can otherwise mistake
// this transport failure for grace expiry if that consumer's context also ends.
func TestSSHShutdownDistinguishesListenerDeadlineFromGrace(t *testing.T) {
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	addStopGraceListener(t, server, context.DeadlineExceeded)
	err := server.wish.Shutdown(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("native Shutdown lost the listener deadline cause: %v", err)
	}
	if err == context.DeadlineExceeded { //nolint:errorlint // Exact outer identity distinguishes grace expiry from a transport-origin deadline.
		t.Error("native Shutdown returned a listener failure as the bare grace deadline")
	}
}

package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	zepki "github.com/ze-software/ze/internal/component/pki"
	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestObserveConfiguredRequiresCompletedQuiesce drives the actual observer over
// its authenticated SDK transport. A successful scenario still owes a completed
// quiesce before it can report success: sent-event waits can precede TCP flush.
//
// MUTATION: discard DispatchCommand's quiesce error and the transport-loss case
// returns success without the failure sentinel.
func TestObserveConfiguredRequiresCompletedQuiesce(t *testing.T) {
	testObserverQuiesce(t, func(ctx context.Context, _ []string) error {
		return observeConfigured(ctx, "observer-quiesce-test", &sdk.Registration{}, nil,
			func(context.Context, *sdk.Plugin) error { return nil })
	}, 0, "observer quiesce", `observer quiesce status is "warning"`)
}

// TestP05ActiveObserversRequireCompletedQuiesce reaches the two active-plugin
// scenarios that inject UPDATEs and wait for pre-flush sent counters. Their own
// quiesce is the delivery barrier; they do not pass through Observe.
func TestP05ActiveObserversRequireCompletedQuiesce(t *testing.T) {
	for _, consumer := range []struct {
		name   string
		driver Driver
		routes int
	}{
		{"custom flowspec plugin", p05CustomFlowspecPlugin, 1},
		{"ddos flowspec announce", p05DDoSFlowspecAnnounce, 2},
	} {
		t.Run(consumer.name, func(t *testing.T) {
			testObserverQuiesce(t, consumer.driver, consumer.routes,
				"request quiesce", "request quiesce: status=warning")
		})
	}
}

// testObserverQuiesce owns the TLS engine and joins it after running the real
// consumer. The completed case must emit all requested routes, read its peer
// counters and shut down without a failure report; rejected barriers must fail.
func testObserverQuiesce(t *testing.T, driver Driver, routes int, failure, warning string) {
	t.Helper()
	cases := []struct {
		name       string
		answer     func(context.Context, *rpc.MuxConn, uint64) error
		wantError  string
		wantCause  string
		wantClosed bool
	}{
		{
			name: "completed",
			answer: func(ctx context.Context, mux *rpc.MuxConn, id uint64) error {
				return rpc.WriteDocumentAnswer(mux.AnswerWriter(ctx), id, rpc.AnswerTail{}, nil)
			},
		},
		{
			name: "transport lost before quiesce reply",
			answer: func(_ context.Context, mux *rpc.MuxConn, _ uint64) error {
				return mux.Close()
			},
			wantError:  failure,
			wantClosed: true,
		},
		{
			name: "engine refuses quiesce",
			answer: func(ctx context.Context, mux *rpc.MuxConn, id uint64) error {
				return mux.SendError(ctx, id, "peer drain refused")
			},
			wantError: failure,
			wantCause: "peer drain refused",
		},
		{
			name: "warning is not completed",
			answer: func(ctx context.Context, mux *rpc.MuxConn, id uint64) error {
				return rpc.WriteDocumentAnswer(mux.AnswerWriter(ctx), id, rpc.AnswerTail{Warning: true}, nil)
			},
			wantError: warning,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store, err := storage.Create(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := store.Close(); err != nil {
					t.Error(err)
				}
			})
			root, err := zepki.LoadOrGenerateRoot(store)
			if err != nil {
				t.Fatal(err)
			}
			acceptor, err := zeplugin.NewHubAcceptor(nil, root, clock.RealClock{})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(acceptor.Stop)
			const name = "observer-quiesce-test"
			host, port, err := net.SplitHostPort(acceptor.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			token, err := acceptor.TokenForPlugin(name)
			if err != nil {
				t.Fatal(err)
			}
			for key, value := range map[string]string{
				"ze.plugin.name":      name,
				"ze.plugin.hub.host":  host,
				"ze.plugin.hub.port":  port,
				"ze.plugin.hub.token": token,
				"ze.plugin.ca.pem":    string(root.CertificatePEM()),
			} {
				previous := env.Get(key)
				if err := env.Set(key, value); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := env.Set(key, previous); err != nil {
						t.Error(err)
					}
				})
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			t.Cleanup(cancel)
			waiter := acceptor.ExpectPlugin(name)
			engineResult := make(chan error, 1)
			engineStopped := make(chan struct{})
			// This one lifecycle worker owns the accepted transport. The test
			// MUST cancel it and join engineStopped before releasing its store.
			go func() {
				defer close(engineStopped)
				conn, err := waiter.Wait(ctx)
				if err != nil {
					engineResult <- err
					return
				}
				mux := rpc.NewMuxConn(rpc.NewConn(conn, conn))
				defer mux.Close() //nolint:errcheck // the injected close is checked at its call site
				engineResult <- serveObserverQuiesce(ctx, mux, routes, c.answer, c.wantClosed, c.wantError != "")
			}()
			t.Cleanup(func() {
				cancel()
				<-engineStopped
			})

			var observeErr error
			report := readOneReport(t, func() {
				observeErr = driver(ctx, nil)
			})
			if err := <-engineResult; err != nil {
				t.Fatalf("engine protocol: %v", err)
			}
			if c.wantError == "" {
				if observeErr != nil {
					t.Fatalf("completed quiesce returned %v", observeErr)
				}
				if report != "" {
					t.Fatalf("completed quiesce reported failure: %q", report)
				}
				return
			}
			if observeErr == nil {
				t.Fatal("observer reported success without a completed quiesce")
			}
			if !strings.Contains(observeErr.Error(), c.wantError) {
				t.Fatalf("observer error = %v, want %q", observeErr, c.wantError)
			}
			if !strings.Contains(observeErr.Error(), c.wantCause) {
				t.Fatalf("observer error = %v, want underlying cause %q", observeErr, c.wantCause)
			}
			if !strings.Contains(report, observerFailure+": ") {
				t.Fatalf("observer failure was not reported before shutdown: %q", report)
			}
		})
	}
}

// serveObserverQuiesce completes the real SDK startup and answers the lifecycle
// commands the observer sends. Closing at quiesce simulates losing the engine
// before its drain acknowledgement; every other case must still ask to stop.
func serveObserverQuiesce(
	ctx context.Context,
	mux *rpc.MuxConn,
	routes int,
	answer func(context.Context, *rpc.MuxConn, uint64) error,
	closed bool,
	wantFailure bool,
) error {
	for _, stage := range []struct {
		request  string
		response any
		callback string
	}{
		{rpc.MethodDeclareRegistration, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}, "ze-plugin-callback:configure"},
		{"ze-plugin-engine:declare-capabilities", nil, "ze-plugin-callback:share-registry"},
		{"ze-plugin-engine:ready", nil, "ze-plugin-callback:post-startup"},
	} {
		select {
		case req := <-mux.Requests():
			if req == nil {
				return fmt.Errorf("transport closed waiting for %s", stage.request)
			}
			if req.Method != stage.request {
				return fmt.Errorf("startup request = %q, want %q", req.Method, stage.request)
			}
			if err := mux.SendResult(ctx, req.ID, stage.response); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
		if _, err := mux.CallRPC(ctx, stage.callback, nil); err != nil {
			return err
		}
	}
	for range routes {
		select {
		case req := <-mux.Requests():
			if req == nil {
				return fmt.Errorf("transport closed waiting for update-route")
			}
			if req.Method != rpc.MethodUpdateRoute {
				return fmt.Errorf("route method = %q, want %q", req.Method, rpc.MethodUpdateRoute)
			}
			if err := mux.SendResult(ctx, req.ID, &rpc.UpdateRouteOutput{Announced: 1}); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if routes > 0 {
		// Each actual scenario reads updates-sent, then eor-sent, from the
		// same named peer. Both readings must succeed before its barrier.
		for range 2 {
			select {
			case req := <-mux.Requests():
				if req == nil {
					return fmt.Errorf("transport closed waiting for peer counters")
				}
				if req.Method != rpc.MethodDispatchCommand {
					return fmt.Errorf("counter method = %q, want %q", req.Method, rpc.MethodDispatchCommand)
				}
				var input rpc.DispatchCommandInput
				if err := json.Unmarshal(req.Params, &input); err != nil {
					return err
				}
				if input.Command != "show bgp peer peer1 detail" {
					return fmt.Errorf("peer counter command = %q", input.Command)
				}
				document := json.RawMessage(`{"peers":{"peer1":{"updates-sent":3,"eor-sent":1}}}`)
				if err := rpc.WriteDocumentAnswer(mux.AnswerWriter(ctx), req.ID, rpc.AnswerTail{}, document); err != nil {
					return err
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	for _, command := range []string{"request quiesce", "request shutdown"} {
		select {
		case req := <-mux.Requests():
			if req == nil {
				return fmt.Errorf("transport closed waiting for %s", command)
			}
			if req.Method != rpc.MethodDispatchCommand {
				return fmt.Errorf("command method = %q, want %q", req.Method, rpc.MethodDispatchCommand)
			}
			var input rpc.DispatchCommandInput
			if err := json.Unmarshal(req.Params, &input); err != nil {
				return err
			}
			if input.Command != command {
				return fmt.Errorf("command = %q, want %q", input.Command, command)
			}
			if command == "request quiesce" {
				if err := answer(ctx, mux, req.ID); err != nil {
					return err
				}
				if closed {
					return nil
				}
				continue
			}
			if reported := reportedOnce.Load(); reported != wantFailure {
				return fmt.Errorf("failure reported at shutdown = %t, want %t", reported, wantFailure)
			}
			if err := rpc.WriteDocumentAnswer(mux.AnswerWriter(ctx), req.ID, rpc.AnswerTail{}, nil); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	_, err := mux.CallRPC(ctx, "ze-plugin-callback:bye", nil)
	return err
}

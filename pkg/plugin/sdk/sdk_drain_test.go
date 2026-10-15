// Tests for the SDK drain worker (sdk_drain.go).
// VALIDATES: spec-daemon-drain-before-stop AC-28 (one worker per connection,
// running -> stopping, a second stop-kind drain refused while the first runs
// and after it answered), and that the handler runs off the caller (the
// event loop) so the loop is never held by a drain wait.
// PREVENTS: a second stop drain reaching the handler, and a drain wait
// freezing the plugin's command answers.

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func drainParams(t *testing.T, kind rpc.DrainKind) json.RawMessage {
	t.Helper()
	params, err := json.Marshal(rpc.DrainInput{Kind: kind, Deadline: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatalf("marshal drain input: %v", err)
	}
	return params
}

// TestDrainWorkerStoppingRefusesSecondStop admits one stop drain, then
// proves a second stop-kind drain is refused while the first handler is
// still held, and again after it answered; admission never blocks; the
// worker returns when its context ends.
func TestDrainWorkerStoppingRefusesSecondStop(t *testing.T) {
	release := make(chan struct{})
	received := make(chan rpc.DrainInput, 2)
	worker := newDrainWorker("probe", func(ctx context.Context, input rpc.DrainInput) error {
		received <- input
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	go worker.run(ctx)

	answers := make(chan error, 2)
	answer := func(err error) { answers <- err }

	if err := worker.admit(drainParams(t, rpc.DrainKindStop), answer); err != nil {
		t.Fatalf("first stop drain refused: %v", err)
	}
	first := <-received
	if first.Kind != rpc.DrainKindStop {
		t.Fatalf("handler got kind %v, want stop", first.Kind)
	}

	// The first handler is held: admission still answers at once.
	start := time.Now()
	err := worker.admit(drainParams(t, rpc.DrainKindRestart), answer)
	if !errors.Is(err, errDrainAlreadyStopping) {
		t.Fatalf("second drain while the first runs: %v, want refusal", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("refusal took %v: admission blocked on the held handler", elapsed)
	}

	close(release)
	if err := <-answers; err != nil {
		t.Fatalf("first drain answered %v", err)
	}

	if err := worker.admit(drainParams(t, rpc.DrainKindStop), answer); !errors.Is(err, errDrainAlreadyStopping) {
		t.Fatalf("second drain after the first answered: %v, want refusal", err)
	}
	select {
	case input := <-received:
		t.Fatalf("a refused drain reached the handler: %+v", input)
	default:
	}

	cancel()
	select {
	case <-worker.done:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not return when its connection context ended")
	}
}

// TestDrainWorkerRejectsUnknownKind proves the decode refuses a kind the
// wire does not define, and a refused decode leaves the worker running.
func TestDrainWorkerRejectsUnknownKind(t *testing.T) {
	worker := newDrainWorker("probe", func(context.Context, rpc.DrainInput) error { return nil })
	err := worker.admit(json.RawMessage(`{"kind":"maintenance","deadline":"2026-10-14T00:00:00Z"}`), func(error) {})
	if err == nil {
		t.Fatal("an unknown drain kind was admitted")
	}
	if worker.stopping.Load() {
		t.Fatal("a refused decode moved the worker to stopping")
	}
}

// TestOnDrainDeclaresHandler proves OnDrain records the handler that Run
// declares as DrainHandler at Stage 1, and the worker starts only with one.
func TestOnDrainDeclaresHandler(t *testing.T) {
	p := &Plugin{name: "probe"}
	if worker, stop := p.startDrainWorker(context.Background()); worker != nil {
		stop()
		t.Fatal("a drain worker started with no handler")
	}
	p.OnDrain(func(context.Context, rpc.DrainInput) error { return nil })
	worker, stop := p.startDrainWorker(context.Background())
	if worker == nil {
		t.Fatal("no drain worker started with a handler")
	}
	stop()
	select {
	case <-worker.done:
	default:
		t.Fatal("stop returned before the worker ended")
	}
}

// TestDrainKindWireNames checks the closed set of wire names both ways and
// that an unspecified kind is never sent.
func TestDrainKindWireNames(t *testing.T) {
	for _, kind := range []rpc.DrainKind{rpc.DrainKindStop, rpc.DrainKindRestart} {
		text, err := kind.MarshalText()
		if err != nil {
			t.Fatalf("marshal %v: %v", kind, err)
		}
		var back rpc.DrainKind
		if err := back.UnmarshalText(text); err != nil || back != kind {
			t.Fatalf("round trip %q gave %v, %v", text, back, err)
		}
	}
	if _, err := rpc.DrainKindUnspecified.MarshalText(); err == nil {
		t.Fatal("an unspecified drain kind was encoded")
	}
}

// TestOnDrainRunsOffEventLoop proves, over the socket event loop and over the
// DirectBridge event loop, that a held drain handler leaves the loop serving:
// an event is delivered and a second stop drain is refused while the first
// handler still waits, and the first drain is answered once released.
func TestOnDrainRunsOffEventLoop(t *testing.T) {
	type sendFunc func(ctx context.Context, method string, params json.RawMessage) error

	modes := []struct {
		name  string
		start func(t *testing.T, ctx context.Context, setup func(p *Plugin)) sendFunc
	}{
		{"socket", func(t *testing.T, ctx context.Context, setup func(p *Plugin)) sendFunc {
			p, engine := newTestPair(t)
			setup(p)
			runDone := make(chan error, 1)
			go func() { runDone <- p.Run(ctx, &Registration{DrainBudget: 5}) }()
			completeStartup(t, ctx, engine)
			return func(ctx context.Context, method string, params json.RawMessage) error {
				return callAndExpectOK(ctx, engine.mux, method, params)
			}
		}},
		{"bridge", func(t *testing.T, ctx context.Context, setup func(p *Plugin)) sendFunc {
			p, _, bridge := newBridgedTestPair(t)
			setup(p)
			loopDone := make(chan error, 1)
			go func() { loopDone <- p.bridgeEventLoop(ctx) }()
			return func(ctx context.Context, method string, params json.RawMessage) error {
				_, err := bridge.SendCallback(ctx, method, params)
				return err
			}
		}},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			release := make(chan struct{})
			entered := make(chan struct{}, 1)
			events := make(chan string, 1)
			send := mode.start(t, ctx, func(p *Plugin) {
				p.OnDrain(func(ctx context.Context, _ rpc.DrainInput) error {
					entered <- struct{}{}
					select {
					case <-release:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
				p.OnEvent(func(event string) error {
					events <- event
					return nil
				})
			})

			drainDone := make(chan error, 1)
			go func() { drainDone <- send(ctx, rpc.MethodDrain, drainParams(t, rpc.DrainKindStop)) }()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("the drain never reached the handler")
			}

			eventParams, err := json.Marshal(struct {
				Event string `json:"event"`
			}{Event: "while-draining"})
			if err != nil {
				t.Fatalf("marshal event: %v", err)
			}
			if err := send(ctx, callbackDeliverEvent, eventParams); err != nil {
				t.Fatalf("event while the drain is held: %v", err)
			}
			if got := <-events; got != "while-draining" {
				t.Fatalf("event %q, want while-draining", got)
			}

			err = send(ctx, rpc.MethodDrain, drainParams(t, rpc.DrainKindRestart))
			if err == nil || !strings.Contains(err.Error(), errDrainAlreadyStopping.Error()) {
				t.Fatalf("second drain while the first is held: %v, want the stopping refusal", err)
			}

			select {
			case err := <-drainDone:
				t.Fatalf("the held drain was answered before release: %v", err)
			default:
			}
			close(release)
			if err := <-drainDone; err != nil {
				t.Fatalf("the released drain answered %v", err)
			}
		})
	}
}

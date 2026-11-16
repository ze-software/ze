// Design: docs/architecture/command-ownership.md -- shared registered live views.
package cli

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestBrowserViewDashboard follows the same selected peer through numeric order,
// reversal, detail and back by driving the registered dashboard, not a replica.
func TestBrowserViewDashboard(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	actions := make(chan BrowserViewAction, 1)
	frames := make(chan BrowserViewFrame, 1)
	details := make(chan string, 2)
	refreshed := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- RunBrowserView(ctx, "monitor bgp", func(bound context.Context, m *Model) error {
			if bound.Err() != nil {
				t.Error("binding received canceled context")
			}
			m.SetViewFactory(ViewKeyDashboard, DashboardFactory(func() (func() (string, error), error) {
				polls := 0
				return func() (string, error) {
					polls++
					if polls == 2 {
						close(refreshed)
					}
					return `{"router-id":"192.0.2.1","peers":[{"address":"10.0.0.10","remote-as":10},{"address":"10.0.0.9","remote-as":9}]}`, nil
				}, nil
			}))
			m.SetCommandExecutor(func(line string) (CommandOutput, error) {
				details <- line
				return CommandOutput{Text: `{"peers":{"10.0.0.10":{"description":"selected-peer"}}}`}, nil
			})
			return nil
		}, actions, frames)
	}()
	frame := browserWaitFrame(t, frames, func(f BrowserViewFrame) bool { return strings.Contains(f.Text, "10.0.0.10") })
	if strings.Index(frame.Text, "10.0.0.9") > strings.Index(frame.Text, "10.0.0.10") {
		t.Fatal("address order is lexical")
	}
	actions <- BrowserViewAction{ID: "next"}
	browserWaitFrame(t, frames, func(f BrowserViewFrame) bool { return f.Sequence > frame.Sequence })
	actions <- BrowserViewAction{ID: "sort"}
	browserWaitFrame(t, frames, func(f BrowserViewFrame) bool { return strings.Contains(f.Text, "ASN") })
	actions <- BrowserViewAction{ID: "reverse"}
	reversed := browserWaitFrame(t, frames, func(f BrowserViewFrame) bool {
		return strings.Index(f.Text, "10.0.0.10") < strings.Index(f.Text, "10.0.0.9")
	})
	select {
	case <-refreshed:
	case <-ctx.Done():
		t.Fatal("view never refreshed")
	}
	browserWaitFrame(t, frames, func(f BrowserViewFrame) bool { return f.Sequence > reversed.Sequence })
	actions <- BrowserViewAction{ID: "detail"}
	select {
	case line := <-details:
		if line != "show bgp peer 10.0.0.10 detail" {
			t.Fatalf("detail target %q", line)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	detail := browserWaitFrame(t, frames, func(f BrowserViewFrame) bool { return browserHasControl(f, "back") })
	if !strings.Contains(detail.Text, "10.0.0.10") {
		t.Fatal("missing detail address")
	}
	actions <- BrowserViewAction{ID: "back"}
	browserWaitFrame(t, frames, func(f BrowserViewFrame) bool { return f.Sequence > reversed.Sequence && browserHasControl(f, "next") })
	actions <- BrowserViewAction{ID: "stop"}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stop did not join")
	}
}

type browserContextKey struct{}

// TestBrowserViewProbeParametersAndLifetime checks that registered starts keep
// the original parameters/context and release their actual producer on stop.
func TestBrowserViewProbeParametersAndLifetime(t *testing.T) {
	for _, kind := range []string{"ping", "traceroute"} {
		t.Run(kind, func(t *testing.T) {
			original := context.WithValue(t.Context(), browserContextKey{}, "caller")
			ctx, cancel := context.WithTimeout(original, 3*time.Second)
			defer cancel()
			actions := make(chan BrowserViewAction, 1)
			frames := make(chan BrowserViewFrame, 1)
			released := make(chan struct{}, 1)
			observed := make(chan context.Context, 1)
			done := make(chan error, 1)
			input := "monitor ping 192.0.2.9 interval 500ms timeout 2s count 7 size 1400"
			if kind == "traceroute" {
				input = "monitor traceroute 192.0.2.9 max-hops 11"
			}
			go func() {
				done <- RunBrowserView(ctx, input, func(_ context.Context, m *Model) error {
					if kind == "ping" {
						m.SetViewFactory(ViewKeyPing, PingFactory(func(probeCtx context.Context, target string, interval, timeout time.Duration, count, size int) (<-chan map[string]any, context.CancelFunc, error) {
							if target != "192.0.2.9" {
								t.Error("ping target changed")
							}
							if interval != 500*time.Millisecond {
								t.Error("ping interval changed")
							}
							if timeout != 2*time.Second {
								t.Error("ping timeout changed")
							}
							if count != 7 {
								t.Error("ping count changed")
							}
							if size != 1400 {
								t.Error("ping payload changed")
							}
							observed <- probeCtx
							return make(chan map[string]any), func() { released <- struct{}{} }, nil
						}))
					} else {
						m.SetViewFactory(ViewKeyTraceroute, TracerouteFactory(func(probeCtx context.Context, target string, hops int) (<-chan map[string]any, context.CancelFunc, error) {
							if target != "192.0.2.9" {
								t.Error("traceroute target changed")
							}
							if hops != 11 {
								t.Error("traceroute hop limit changed")
							}
							observed <- probeCtx
							return make(chan map[string]any), func() { released <- struct{}{} }, nil
						}))
					}
					return nil
				}, actions, frames)
			}()
			var probeCtx context.Context
			select {
			case probeCtx = <-observed:
			case <-ctx.Done():
				t.Fatal("producer never started")
			}
			if probeCtx.Value(browserContextKey{}) != "caller" {
				t.Fatal("request context lost")
			}
			deadline, ok := probeCtx.Deadline()
			if !ok {
				t.Fatal("request deadline lost")
			}
			if deadline.IsZero() {
				t.Fatal("request deadline is zero")
			}
			browserWaitFrame(t, frames, func(f BrowserViewFrame) bool { return browserHasControl(f, "stop") })
			actions <- BrowserViewAction{ID: "stop"}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("runner did not join")
			}
			if probeCtx.Err() == nil {
				t.Fatal("probe context outlived runner")
			}
			select {
			case <-released:
			default:
				t.Fatal("active view was not released")
			}
		})
	}
}

// TestBrowserViewRejectsUndeclaredAction proves a browser cannot send arbitrary
// terminal keys or commands through the action channel.
func TestBrowserViewRejectsUndeclaredAction(t *testing.T) {
	actions := make(chan BrowserViewAction, 1)
	actions <- BrowserViewAction{ID: "show secret"}
	frames := make(chan BrowserViewFrame, 1)
	err := RunBrowserView(t.Context(), "monitor bgp", func(_ context.Context, m *Model) error {
		m.SetViewFactory(ViewKeyDashboard, DashboardFactory(func() (func() (string, error), error) {
			return func() (string, error) { return `{"peers":[]}`, nil }, nil
		}))
		return nil
	}, actions, frames)
	if err == nil {
		t.Fatal("undeclared action accepted")
	}
	if !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("undeclared action: %v", err)
	}
}

// TestBrowserViewRequiresBoundedChannels refuses unbounded/oversized transport
// contracts before the caller's producer binding can run.
func TestBrowserViewRequiresBoundedChannels(t *testing.T) {
	for _, capacity := range []int{0, BrowserViewActionQueueMax + 1} {
		called := false
		err := RunBrowserView(t.Context(), "monitor bgp", func(context.Context, *Model) error { called = true; return nil }, make(chan BrowserViewAction, capacity), make(chan BrowserViewFrame, 1))
		if err == nil {
			t.Fatal("unbounded actions accepted")
		}
		if called {
			t.Fatal("producer bound before queue validation")
		}
	}
}

// TestBrowserViewOldGenerationIgnored ensures a late result cannot mutate a
// newer request's view, including its current frame and sequence.
func TestBrowserViewOldGenerationIgnored(t *testing.T) {
	b := &browserViewModel{generation: &browserViewGeneration{}}
	b.Update(browserViewResult{generation: &browserViewGeneration{}, message: dashboardDataMsg{data: `{"peers":[]}`}})
	if b.sequence != 0 {
		t.Fatal("late generation published a frame")
	}
}

// TestBrowserViewCancelJoinsPoll verifies request cancellation reaches a poll
// already running and that the driver joins it before returning.
func TestBrowserViewCancelJoinsPoll(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{})
	finished := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- RunBrowserView(ctx, "monitor bgp", func(bound context.Context, m *Model) error {
			m.SetViewFactory(ViewKeyDashboard, DashboardFactory(func() (func() (string, error), error) {
				return func() (string, error) { close(started); <-bound.Done(); close(finished); return "", bound.Err() }, nil
			}))
			return nil
		}, make(chan BrowserViewAction, 1), make(chan BrowserViewFrame, 1))
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("poll never started")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("canceled poll was not joined")
	}
	select {
	case <-finished:
	default:
		t.Fatal("runner returned before poll stopped")
	}
}

// TestBrowserViewProbeDrainBound proves one update preserves excess input for
// the next tick rather than monopolizing the headless program's action loop.
func TestBrowserViewProbeDrainBound(t *testing.T) {
	for _, drain := range []func(<-chan map[string]any) ([]map[string]any, bool){drainPingReplies, drainTracerouteHops} {
		source := make(chan map[string]any, liveViewDrainBatchMax+1)
		for range liveViewDrainBatchMax + 1 {
			source <- map[string]any{"status": "ok"}
		}
		close(source)
		rows, closed := drain(source)
		if len(rows) != liveViewDrainBatchMax {
			t.Fatalf("first drain: %d rows", len(rows))
		}
		if closed {
			t.Fatal("first drain discarded remaining input")
		}
		rows, closed = drain(source)
		if len(rows) != 1 {
			t.Fatalf("remaining drain: %d rows", len(rows))
		}
		if !closed {
			t.Fatal("remaining drain missed channel completion")
		}
	}
}

// TestBrowserViewProgramHasNoTerminalResources checks the actual dependency
// configuration rather than changing process-wide stdio or signal handlers.
func TestBrowserViewProgramHasNoTerminalResources(t *testing.T) {
	program := newBrowserViewProgram(t.Context(), &browserViewModel{})
	configuration := reflect.ValueOf(program).Elem()
	for _, name := range []string{"disableInput", "disableRenderer", "disableSignalHandler"} {
		if !configuration.FieldByName(name).Bool() {
			t.Fatalf("terminal resource enabled: %s", name)
		}
	}
	if !configuration.FieldByName("input").IsNil() {
		t.Fatal("program retains a terminal input")
	}
}

// TestBrowserViewFramesAreDetachedAndBounded holds an older frame while the
// model publishes another, and checks that an oversized frame ends the runner.
func TestBrowserViewFramesAreDetachedAndBounded(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	model := NewCommandModel(FilesystemAuthorityUnknown)
	defer func() {
		if err := model.Close(); err != nil {
			t.Error(err)
		}
	}()
	model.width, model.height = 120, 40
	model.activeView = &dashboardView{st: &dashboardState{sortAsc: true}}
	b := &browserViewModel{model: model, spec: viewSpec{browser: dashboardBrowserControls}, cancel: cancel, frames: make(chan BrowserViewFrame, 1)}
	b.publish(false)
	first := <-b.frames
	first.Controls[0].Label = "changed by receiver"
	b.publish(false)
	latest := <-b.frames
	if latest.Controls[0].Label == first.Controls[0].Label {
		t.Fatal("frames share mutable controls")
	}
	b.model.activeDashboard().detailAddr = "192.0.2.1"
	b.model.activeDashboard().snapshot = &dashboardSnapshot{Peers: []dashboardPeer{{Address: "192.0.2.1"}}}
	b.model.activeDashboard().detailData = map[string]any{"name": strings.Repeat("x", BrowserViewFrameBytesMax+1)}
	b.publish(false)
	if b.err == nil {
		t.Fatal("oversized frame accepted")
	}
	if ctx.Err() == nil {
		t.Fatal("oversized frame did not cancel view")
	}
}

// TestBrowserViewRegistryCapability checks every owner registration instead of
// keeping a list of monitor commands in the browser adapter.
func TestBrowserViewRegistryCapability(t *testing.T) {
	if gaps := browserViewCapabilityGaps(RegisteredViews()); len(gaps) != 0 {
		t.Fatalf("registered views without browser controls: %v", gaps)
	}
}

// TestBrowserViewRegistryDetectsMissingCapability proves that the same guard
// detects a newly registered view whose owner forgot the browser contract.
func TestBrowserViewRegistryDetectsMissingCapability(t *testing.T) {
	views := append(RegisteredViews(), ViewInfo{Key: "missing-browser-contract", Prefix: "monitor synthetic"})
	gaps := browserViewCapabilityGaps(views)
	if len(gaps) != 1 {
		t.Fatalf("missing registration capability gaps: %v", gaps)
	}
	if gaps[0] != "missing-browser-contract" {
		t.Fatalf("wrong capability gap: %v", gaps)
	}
}

func browserViewCapabilityGaps(views []ViewInfo) []string {
	var gaps []string
	for _, view := range views {
		if !view.Browser {
			gaps = append(gaps, view.Key)
		}
	}
	return gaps
}

// TestBrowserViewDetailFailureIsVisible proves an authorization failure from
// the existing detail executor reaches the same frame's shared error zone.
func TestBrowserViewDetailFailureIsVisible(t *testing.T) {
	model := NewCommandModel(FilesystemAuthorityUnknown)
	defer func() {
		if err := model.Close(); err != nil {
			t.Error(err)
		}
	}()
	model.width, model.height = 120, 40
	model.activeView = &dashboardView{st: &dashboardState{snapshot: &dashboardSnapshot{Peers: []dashboardPeer{{Address: "192.0.2.9"}}}, selectedAddr: "192.0.2.9"}}
	model.SetCommandExecutor(func(line string) (CommandOutput, error) {
		if line != "show bgp peer 192.0.2.9 detail" {
			t.Errorf("detail target %q", line)
		}
		return CommandOutput{}, errors.New("detail authorization refused")
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	b := &browserViewModel{model: model, spec: viewSpec{browser: dashboardBrowserControls}, cancel: cancel, frames: make(chan BrowserViewFrame, 1)}
	b.action(BrowserViewAction{ID: "detail"})
	frame := <-b.frames
	if !strings.Contains(frame.Text, "detail authorization refused") {
		t.Fatalf("detail failure hidden: %s", frame.Text)
	}
	if ctx.Err() != nil {
		t.Fatal("denied detail ended live view")
	}
}

// TestBrowserViewStopInterruptsDetail checks Stop at admission, while a detail
// RPC owns the Model loop; it must cancel that RPC and join without another tick.
func TestBrowserViewStopInterruptsDetail(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	actions := make(chan BrowserViewAction, 1)
	frames := make(chan BrowserViewFrame, 1)
	entered := make(chan struct{})
	exited := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- RunBrowserView(ctx, "monitor bgp", func(bound context.Context, m *Model) error {
			m.SetViewFactory(ViewKeyDashboard, DashboardFactory(func() (func() (string, error), error) {
				return func() (string, error) { return `{"peers":[{"address":"192.0.2.9"}]}`, nil }, nil
			}))
			m.SetCommandExecutor(func(string) (CommandOutput, error) {
				close(entered)
				<-bound.Done()
				close(exited)
				return CommandOutput{}, bound.Err()
			})
			return nil
		}, actions, frames)
	}()
	browserWaitFrame(t, frames, func(f BrowserViewFrame) bool { return strings.Contains(f.Text, "192.0.2.9") })
	actions <- BrowserViewAction{ID: "detail"}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("detail RPC did not start")
	}
	actions <- BrowserViewAction{ID: "stop"}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("Stop did not interrupt detail RPC")
	}
	select {
	case <-exited:
	default:
		t.Fatal("detail RPC outlived Stop")
	}
}

func browserHasControl(frame BrowserViewFrame, id string) bool {
	for _, control := range frame.Controls {
		if control.ID == id {
			return true
		}
	}
	return false
}

func browserWaitFrame(t *testing.T, frames <-chan BrowserViewFrame, accept func(BrowserViewFrame) bool) BrowserViewFrame {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	// A fixed timeout bounds the session observation; frames arrive on change.
	for {
		select {
		case frame := <-frames:
			if accept(frame) {
				return frame
			}
		case <-deadline.C:
			t.Fatal("expected browser frame did not arrive")
			return BrowserViewFrame{}
		}
	}
}

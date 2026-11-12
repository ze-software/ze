package server

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/show"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// enrichRestartKey is the field the restarted plugin's enricher adds.
const enrichRestartKey = "extra"

// startEnricherRestartTarget starts a running plugin that declares one show
// enricher on command and answers it with enrichRestartKey, each process the
// restart starts included. hold runs at the start of every enrich call the
// plugin receives, before it answers.
func startEnricherRestartTarget(t *testing.T, name, command string, hold func()) *Server {
	t.Helper()
	t.Cleanup(func() { forgetEnricherFailures(name) })
	return startRestartTargetWith(t, name, func(p *sdk.Plugin) *sdk.Registration {
		p.OnEnrichShow(func(_, _, _ string, _ map[string]any) (map[string]any, error) {
			hold()
			return map[string]any{enrichRestartKey: "enriched"}, nil
		})
		return &sdk.Registration{Enrichers: []rpc.EnricherDecl{{Command: command, Key: enrichRestartKey}}}
	})
}

// receiveWithin answers the next value of ch, failing the test once bound
// has passed without one.
func receiveWithin[T any](t *testing.T, ch <-chan T, bound time.Duration, failure string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(bound):
		t.Fatal(failure)
	}
	var zero T
	return zero
}

// restartOutcome is what handlePluginRestart answered on another goroutine.
type restartOutcome struct {
	resp *plugin.Response
	err  error
}

// assertRestartedEnricherHealthy checks the plugin after its restart: its
// doctor state carries no enricher failure, and a fresh `show` through the
// new process's enricher answers the field.
func assertRestartedEnricherHealthy(t *testing.T, s *Server, name, command string) {
	t.Helper()
	if state := s.pluginDoctorState(name, true); len(state.Enrichers) != 0 || state.EnrichersDropped != 0 {
		t.Fatalf("the restarted plugin's doctor state blames it for the old process's enricher call: %+v, %d past the bound",
			state.Enrichers, state.EnrichersDropped)
	}
	fresh := map[string]any{}
	show.Enrich(command, fresh)
	if fresh[enrichRestartKey] != "enriched" {
		t.Fatalf("a show after the restart did not get the field from the new process: %+v", fresh)
	}
	if state := s.pluginDoctorState(name, true); len(state.Enrichers) != 0 {
		t.Fatalf("the fresh call through the new process recorded a failure: %+v", state.Enrichers)
	}
}

// TestRestartCutsEnricherCallInFlight restarts a running plugin by the
// operator command while a `show` waits inside the plugin's enricher (R-12).
//
// VALIDATES: the stop of the old process ends the call waiting on its
// connection, well inside enrichShowTimeout and while the plugin still holds
// the request, so the `show` answers without the field; the restart
// completes; afterwards the plugin's doctor state holds no enricher failure
// from the cut call, and a fresh call through the new process's enricher
// answers.
// PREVENTS: a `show` or a restart hanging on an enricher call the old process
// never answers, a stale enricher left registered on the closed connection,
// and a failure row blaming the new process for the old one's cut call.
//
// The plugin side holds the first request on a channel until the call has
// returned, so only the stop can have ended it.
func TestRestartCutsEnricherCallInFlight(t *testing.T) {
	const (
		name    = "enrich-restart-cut"
		command = "show enrich-restart-cut"
	)
	entered := make(chan struct{})
	release := make(chan struct{})
	var held atomic.Bool
	s := startEnricherRestartTarget(t, name, command, func() {
		if held.CompareAndSwap(false, true) {
			close(entered)
			<-release
		}
	})

	// Cleanups run last-in first-out: release the held request, then join.
	var wg sync.WaitGroup
	t.Cleanup(wg.Wait)
	closeRelease := sync.OnceFunc(func() { close(release) })
	t.Cleanup(closeRelease)

	shown := make(chan map[string]any, 1)
	wg.Go(func() {
		base := map[string]any{}
		show.Enrich(command, base)
		shown <- base
	})
	receiveWithin(t, entered, 10*time.Second, "the show's enricher call never reached the plugin")

	restarted := make(chan restartOutcome, 1)
	wg.Go(func() {
		resp, err := handlePluginRestart(&CommandContext{Server: s}, commandtest.Args("name", name))
		restarted <- restartOutcome{resp: resp, err: err}
	})

	base := receiveWithin(t, shown, enrichShowTimeout/2,
		"the show waiting on the old process's enricher was not ended by the restart's stop")
	if _, answered := base[enrichRestartKey]; answered {
		t.Fatalf("the cut call answered the field: %+v", base)
	}
	closeRelease()

	out := receiveWithin(t, restarted, 10*time.Second, "the restart never completed")
	if out.err != nil {
		t.Fatalf("handlePluginRestart: %v", out.err)
	}
	if out.resp.Status != plugin.StatusDone {
		t.Fatalf("the restart was refused: %+v", out.resp)
	}
	assertRestartedEnricherHealthy(t, s, name, command)
}

// TestRestartEnricherReadBeforeRestartBlamesNoReplacement runs a `show` that
// read the plugin's enricher from the registry before the restart and calls
// it after the replacement registered its own (R-12).
//
// VALIDATES: the call through the replaced process's connection fails fast
// and answers no field, and its failure is not recorded against the plugin,
// whose new process is healthy; a fresh call answers through the new one.
// PREVENTS: a doctor-plugin-enricher-unavailable row against a healthy
// restarted plugin, which only a successful call of the same enricher would
// ever clear.
//
// show.Enrich calls a command's enrichers in key order from the list it read
// once, so an enricher with a lower key, held on a channel, keeps the `show`
// between that read and the plugin's enricher while the restart runs.
func TestRestartEnricherReadBeforeRestartBlamesNoReplacement(t *testing.T) {
	const (
		name    = "enrich-restart-stale"
		command = "show enrich-restart-stale"
		holdKey = "a-hold"
	)
	s := startEnricherRestartTarget(t, name, command, func() {})

	entered := make(chan struct{})
	release := make(chan struct{})
	var held atomic.Bool
	hold := func(map[string]any) {
		if held.CompareAndSwap(false, true) {
			close(entered)
			<-release
		}
	}
	if err := show.Register(command, holdKey, show.Enricher{Detail: hold, Brief: hold}); err != nil {
		t.Fatalf("register the holding enricher: %v", err)
	}
	t.Cleanup(func() { show.Unregister(command, holdKey) })

	var wg sync.WaitGroup
	t.Cleanup(wg.Wait)
	closeRelease := sync.OnceFunc(func() { close(release) })
	t.Cleanup(closeRelease)

	shown := make(chan map[string]any, 1)
	wg.Go(func() {
		base := map[string]any{}
		show.Enrich(command, base)
		shown <- base
	})
	receiveWithin(t, entered, 10*time.Second, "the show never reached the holding enricher")

	if resp := restartByCommand(t, &CommandContext{Server: s}, commandtest.Args("name", name)); resp.Status != plugin.StatusDone {
		t.Fatalf("the restart was refused: %+v", resp)
	}
	closeRelease()

	base := receiveWithin(t, shown, enrichShowTimeout/2,
		"the show calling the replaced process's enricher did not end fast")
	if _, answered := base[enrichRestartKey]; answered {
		t.Fatalf("the replaced process's enricher answered the field: %+v", base)
	}
	assertRestartedEnricherHealthy(t, s, name, command)
}

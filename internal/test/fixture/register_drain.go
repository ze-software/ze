// Design: docs/architecture/api/process-protocol.md — the stop drain RPC
// Overview: fixture.go — fixture registry and observer helpers
// Related: ../plugins/fakedrain/fakedrain.go — the drain responder both modes share

package fixture

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/test/plugins/fakedrain"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/drain-responder", drainResponder)
	Register("plugin/drain-budget-over-ceiling", drainBudgetOverCeiling)
	Register("plugin/drain-requester", drainRequester)
}

// drainRequesterSettle lets the .ci peer's session reach Established before
// the stop is requested, so the peer can see the Cease teardown sends.
const drainRequesterSettle = 2 * time.Second

// drainRequester is the drain responder that also stops the daemon itself,
// with the one command its argument names: `request shutdown` (default) or
// `request halt` (argument "halt"), the quit handler (handleDaemonQuit). Those
// two reach the hub through the daemon shutdown function, and the drain they
// run is the one this fixture answers.
func drainRequester(ctx context.Context, args []string) error {
	command := "request shutdown"
	if len(args) > 0 && args[0] == "halt" {
		command = "request halt"
	}
	plugin, err := newObserver("drain-requester")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	// The peer's session MUST still be up when the drain arrives: a stop that
	// tore BGP down before the hub ran the drain (the defect AC-8 removes)
	// shows here as a state event other than up before the drain. Seeing the
	// up first keeps the check from passing on a peer that never attached.
	var sessionUp, sessionDown atomic.Bool
	plugin.SetStartupSubscriptions([]string{eventState}, nil, "")
	plugin.OnEvent(func(event string) error {
		if eventIsUp02(event) {
			sessionUp.Store(true)
			return nil
		}
		if _, kind, _ := eventFacts02(event); kind == eventState {
			sessionDown.Store(true)
		}
		return nil
	})
	answered := fakedrain.ResponderWith(plugin, func() {
		if !sessionUp.Load() {
			fakedrain.Log("drain-fixture: FAIL no peer session up before the drain")
		}
		if sessionDown.Load() {
			fakedrain.Log("drain-fixture: FAIL peer session went down before the drain")
		}
	})
	plugin.OnAllPluginsReady(func() error {
		go requestStopAfterSettle(plugin, command)
		return nil
	})
	runErr := plugin.Run(ctx, sdk.Registration{DrainBudget: fakedrain.Budget})
	if !answered() {
		fakedrain.Log("drain-fixture: FAIL connection closed before the drain was answered")
		return errors.Join(errors.New("drain fixture: stopped with no drain answered"), runErr)
	}
	fakedrain.Log("drain-fixture: connection closed after the drain")
	return runErr
}

// requestStopAfterSettle waits drainRequesterSettle, then sends the stop
// command once. One goroutine for the fixture's one stop.
func requestStopAfterSettle(plugin *sdk.Plugin, command string) {
	time.Sleep(drainRequesterSettle)
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	status, _, err := plugin.DispatchCommand(stopCtx, command)
	fakedrain.Log("drain-fixture: sent " + command + " status=" + status + " error=" + errorText(err))
}

// errorText is the error's text, or "none".
func errorText(err error) string {
	if err == nil {
		return "none"
	}
	return err.Error()
}

// drainResponder is fakedrain's responder over an external connection: it
// answers the stop drain exactly as the in-process plugin does, and fails the
// run when its connection closes with no drain answered.
func drainResponder(ctx context.Context, _ []string) error {
	plugin, err := newObserver("drain-responder")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	answered := fakedrain.Responder(plugin)
	runErr := plugin.Run(ctx, sdk.Registration{DrainBudget: fakedrain.Budget})
	if !answered() {
		fakedrain.Log("drain-fixture: FAIL connection closed before the drain was answered")
		return errors.Join(errors.New("drain fixture: stopped with no drain answered"), runErr)
	}
	fakedrain.Log("drain-fixture: connection closed after the drain")
	return runErr
}

// drainBudgetOverCeiling declares a DrainBudget one second above the daemon
// drain ceiling, which Stage 1 refuses.
func drainBudgetOverCeiling(ctx context.Context, _ []string) error {
	plugin, err := newObserver("drain-over-ceiling")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	plugin.OnDrain(func(context.Context, rpc.DrainInput) error { return nil })
	return plugin.Run(ctx, sdk.Registration{DrainBudget: rpc.DrainCeilingSeconds + 1})
}

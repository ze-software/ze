package server

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// slotProcess is an internal plugin process named name whose bridge holds a
// ready filter slot answering action, counting its calls in calls.
func slotProcess(t *testing.T, name string, action rpc.FilterAction, calls *atomic.Int32) (*process.Process, *rpc.DirectBridge) {
	t.Helper()
	engineEnd, pluginEnd := net.Pipe()
	t.Cleanup(func() {
		engineEnd.Close() //nolint:errcheck // test cleanup
		pluginEnd.Close() //nolint:errcheck // test cleanup
	})
	conn := ipc.NewPluginConn(engineEnd, engineEnd)
	bridge := rpc.NewDirectBridge()
	bridge.SetFilterSlot(func(*rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
		calls.Add(1)
		return &rpc.FilterUpdateOutput{Action: action}, nil
	})
	bridge.SetReady()
	conn.SetBridge(bridge)
	proc := process.NewProcess(plugin.PluginConfig{Name: name, Internal: true})
	proc.SetConn(conn)
	return proc, bridge
}

// TestCallFilterUpdateUnavailable proves AC-33's server half through the
// server's own filter call: a plugin whose slot closed (the plugin stopped)
// answers an error naming the plugin as "unavailable" that wraps
// rpc.ErrBridgeClosed, and its evaluator is never called after the stop;
// once the respawned process (a new bridge, a new slot) replaces it, every
// route is judged by the new slot alone. Method: a process manager holding
// one hand-built process, its bridge closed, then replaced.
func TestCallFilterUpdateUnavailable(t *testing.T) {
	const name = "bgp-filter-probe"
	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	pm := process.NewProcessManager(nil)
	s.procManager.Store(pm)

	var oldCalls, newCalls atomic.Int32
	old, oldBridge := slotProcess(t, name, rpc.FilterReject, &oldCalls)
	pm.AddProcess(name, old)

	input := rpc.FilterUpdateInput{Filter: "f"}
	route := rpc.InputFilterRoute(&input)
	out, err := s.CallFilterUpdate(context.Background(), name, &route)
	if err != nil || out.Action != rpc.FilterReject {
		t.Fatalf("live slot answered %+v, %v; want its reject", out, err)
	}

	oldBridge.CloseCallbacks()
	out, err = s.CallFilterUpdate(context.Background(), name, &route)
	if !errors.Is(err, rpc.ErrBridgeClosed) {
		t.Fatalf("stopped plugin answered %+v, %v; want an error wrapping rpc.ErrBridgeClosed", out, err)
	}
	if !strings.Contains(err.Error(), `plugin "`+name+`" is unavailable`) {
		t.Fatalf("the error does not name the plugin as unavailable: %v", err)
	}
	if got := oldCalls.Load(); got != 1 {
		t.Fatalf("the stopped plugin's evaluator ran %d times, want only the call before the stop", got)
	}

	respawned, _ := slotProcess(t, name, rpc.FilterAccept, &newCalls)
	pm.AddProcess(name, respawned)
	out, err = s.CallFilterUpdate(context.Background(), name, &route)
	if err != nil || out.Action != rpc.FilterAccept {
		t.Fatalf("respawned slot answered %+v, %v; want its accept", out, err)
	}
	if oldCalls.Load() != 1 || newCalls.Load() != 1 {
		t.Fatalf("old slot ran %d times, new %d; want the route judged by the new slot alone",
			oldCalls.Load(), newCalls.Load())
	}
}

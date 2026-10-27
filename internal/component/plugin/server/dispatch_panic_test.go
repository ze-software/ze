package server

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// engineOpGuardLines counts the engine-op guard's ERROR lines in the log ring.
func engineOpGuardLines(t *testing.T) int {
	t.Helper()
	entries, err := slogutil.GlobalLogRing().Snapshot(0, "ERROR", "plugin.server")
	if err != nil {
		t.Fatalf("log ring snapshot: %v", err)
	}
	lines := 0
	for _, e := range entries {
		if strings.Contains(e.Message, "an internal Ze defect ended this unit") {
			lines++
		}
	}
	return lines
}

// TestDispatchPluginRPCPanicContained: a panic in the engine handler of one
// plugin-to-engine request (dispatchPluginRPC, through a codec RPC handler
// that panics on its first call) ends that request only. The plugin's call is
// answered with the plain "internal Ze defect" error, and a second request on
// the same connection is served. The panic is counted on the real guard,
// logged and crash-recorded. Without the boundary the panic ends the test
// binary.
func TestDispatchPluginRPCPanicContained(t *testing.T) {
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	beforeCount, declared := engineOpPanics().guard.Panics(taskEngineOp)
	if !declared {
		t.Fatalf("task %q is not declared on the engine-op guard", taskEngineOp)
	}
	beforeLines := engineOpGuardLines(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const method = "ze-plugin-engine:decode-mp-reach"
	var calls atomic.Int32
	s := &Server{ctx: ctx, rpcHandlers: map[string]func(json.RawMessage) (any, error){
		method: func(json.RawMessage) (any, error) {
			if calls.Add(1) == 1 {
				panic("boom: engine op defect")
			}
			return map[string]string{"status": "ok"}, nil
		},
	}}

	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = pluginEnd.Close(); _ = engineEnd.Close() })

	engineConn := plugipc.NewPluginConn(engineEnd, engineEnd)
	pluginConn := rpc.NewConn(pluginEnd, pluginEnd)
	proc := &process.Process{}

	call := func() error {
		done := make(chan error, 1)
		go func() {
			_, err := pluginConn.CallRPC(ctx, method, &rpc.DecodeMPReachInput{Hex: "0001"})
			done <- err
		}()
		req, err := engineConn.ReadRequest(ctx)
		if err != nil {
			t.Fatalf("engine read request: %v", err)
		}
		s.dispatchPluginRPC(proc, engineConn, req)
		return <-done
	}

	err := call()
	if err == nil {
		t.Fatal("the request whose handler panicked was answered with success")
	}
	if !strings.Contains(err.Error(), "internal Ze defect while handling "+method) {
		t.Errorf("panicked request error = %q, want the internal defect message", err)
	}
	if strings.Contains(err.Error(), "boom") {
		t.Errorf("panicked request error = %q leaks the panic value to the plugin", err)
	}

	if err := call(); err != nil {
		t.Fatalf("second request on the same connection: %v", err)
	}

	afterCount, _ := engineOpPanics().guard.Panics(taskEngineOp)
	if afterCount != beforeCount+1 {
		t.Errorf("guard count = %d, want %d", afterCount, beforeCount+1)
	}
	if got := engineOpGuardLines(t); got != beforeLines+1 {
		t.Errorf("guard ERROR lines = %d, want %d", got, beforeLines+1)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Errorf("crash records = %+v, want one %q record", crashes, crashlog.KindRecovered)
	}
}

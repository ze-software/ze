// Design: docs/architecture/ospf/ospf-8-spf-rib.md -- export chain on a failed apply.

package ospf

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	ospfredistribute "github.com/ze-software/ze/internal/plugins/ospf/redistribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// serveFailingIncrements answers each plugin request like serve, except that a
// boot-count increment answers persist-failed once failIncrements is set, so
// an engine created after that point cannot initialize its durable state.
func (e *ospfRollbackEngine) serveFailingIncrements(ctx context.Context, failIncrements *atomic.Bool) {
	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-e.mux.Requests():
			if !ok || req == nil {
				return
			}
			switch req.Method {
			case rpc.MethodStateIncrement:
				if failIncrements.Load() {
					_ = e.mux.SendResult(ctx, req.ID, rpc.StateOutput{Status: rpc.StatePersistFailed})
					break
				}
				_ = e.mux.SendResult(ctx, req.ID, rpc.StateOutput{Status: rpc.StateOK, Value: 1})
			case rpc.MethodStateGet:
				_ = e.mux.SendResult(ctx, req.ID, rpc.StateOutput{Status: rpc.StateAbsent})
			default:
				_ = e.mux.SendOK(ctx, req.ID)
			}
			select {
			case e.seen <- req.Method:
			default:
			}
		}
	}
}

// TestOSPFFailedApplyRestoresExportChain proves a config apply that fails
// after installing its export chain takes that chain back before it answers.
// Method: run the real OSPF plugin over a pipe, start it with config A
// (export final accept), offer a redistributed route, then verify and apply
// config B (export final reject plus a new OSPFv2 instance 5) while the
// engine refuses every boot-count increment, so instance 5 cannot initialize
// its durable state and the apply fails after it adopted B's chain. The SDK
// does not send the rollback to a plugin whose own apply failed
// (pkg/plugin/sdk/config_tx_gate.go), so no config-rollback is sent: the apply
// itself must leave chain A installed, and a later rollback must find nothing
// to restore.
func TestOSPFFailedApplyRestoresExportChain(t *testing.T) {
	const (
		cfgA = `{"ospf":{"router-id":"10.0.0.1","timers":{"min-ls-interval-ms":1},"filter":{"export":{"final":"accept"}}}}`
		cfgB = `{"ospf":{"router-id":"10.0.0.1","timers":{"min-ls-interval-ms":1},` +
			`"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","instance-id":"5"}}},` +
			`"filter":{"export":{"final":"reject"}}}}`
	)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	t.Cleanup(func() { ospfredistribute.SetExportChain(nil) })
	previous, hadPrevious := configredist.LookupConsumer(ospfredistribute.ConsumerName)
	t.Cleanup(func() {
		if hadPrevious {
			configredist.ReregisterConsumer(previous)
		}
	})

	var failIncrements atomic.Bool
	pluginEnd, engineEnd := net.Pipe()
	eng := &ospfRollbackEngine{mux: rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)), seen: make(chan string, 64)}
	go eng.serveFailingIncrements(ctx, &failIncrements)
	done := make(chan struct{})
	go func() {
		runOSPFEngine(pluginEnd)
		close(done)
	}()
	t.Cleanup(func() {
		_ = eng.mux.Close()
		_ = engineEnd.Close()
		_ = pluginEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("OSPF engine did not exit after its transport closed")
		}
	})

	eng.waitFor(t, ctx, "ze-plugin-engine:declare-registration")
	eng.callback(t, ctx, "ze-plugin-callback:configure",
		rpc.ConfigureInput{Sections: []rpc.ConfigSection{{Root: Namespace, Data: cfgA}}})
	eng.waitFor(t, ctx, "ze-plugin-engine:declare-capabilities")
	eng.callback(t, ctx, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{})
	eng.waitFor(t, ctx, "ze-plugin-engine:ready")

	var consumer configredist.RedistConsumer
	for consumer == nil {
		if c, ok := configredist.LookupConsumer(ospfredistribute.ConsumerName); ok && (!hadPrevious || c != previous) {
			consumer = c
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("OnStarted never registered the OSPF redistribution consumer")
		case <-time.After(10 * time.Millisecond):
		}
	}

	prefix := netip.MustParsePrefix("192.0.2.1/32")
	entry := configredist.RouteEntry{Prefix: prefix.String(), Source: "static"}
	entry.Route.Prefix = prefix
	consumer.InjectRoute(ctx, family.IPv4Unicast, entry)
	if !eng.advertises(t, ctx, prefix.Addr().String()) {
		t.Fatal("config A: chain X (final accept) did not originate the offered route")
	}

	eng.callback(t, ctx, "ze-plugin-callback:config-verify",
		rpc.ConfigVerifyInput{Sections: []rpc.ConfigSection{{Root: Namespace, Data: cfgB}}})
	failIncrements.Store(true)
	raw, err := eng.mux.CallRPC(ctx, "ze-plugin-callback:config-apply",
		rpc.ConfigApplyInput{Sections: []rpc.ConfigDiffSection{{Root: Namespace, Changed: cfgB}}})
	if err != nil {
		t.Fatalf("config-apply: %v", err)
	}
	var status struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatalf("config-apply answer %s: %v", raw, err)
	}
	if status.Status != rpc.StatusError {
		t.Fatalf("config-apply answered %s, want an error: instance 5 cannot initialize its state", raw)
	}

	chain := ospfredistribute.ExportChain()
	if chain == nil {
		t.Fatal("after the failed apply no export chain is installed, want chain X")
	}
	if got := chain.Evaluate(entry.Route).Action; got != ribpolicy.ActionAccept {
		t.Fatalf("after the failed apply the installed chain answers %v for the route, want accept (chain X)", got)
	}
}

// TestOSPFFailedApplyThatCannotUndoHandsTheRepairToRollback proves an apply
// that fails and cannot put the engines back says so with
// ErrRunningConfigLost, so the SDK sends it the rollback, and that the
// rollback tries the repair again and answers an error when it fails too,
// which is what restarts the plugin. Method: config A runs instance 5;
// config B replaces it with instance 6 while the engine refuses every
// boot-count increment, so instance 6 cannot initialize and the undo cannot
// recreate instance 5 either. The increments stay refused for the rollback.
func TestOSPFFailedApplyThatCannotUndoHandsTheRepairToRollback(t *testing.T) {
	const (
		cfgA = `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},` +
			`"interfaces":{"interface":{"ze-test-absent0":{"area":"0","instance-id":"5"}}}}}`
		cfgB = `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},` +
			`"interfaces":{"interface":{"ze-test-absent0":{"area":"0","instance-id":"6"}}}}}`
	)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	t.Cleanup(func() { ospfredistribute.SetExportChain(nil) })
	previous, hadPrevious := configredist.LookupConsumer(ospfredistribute.ConsumerName)
	t.Cleanup(func() {
		if hadPrevious {
			configredist.ReregisterConsumer(previous)
		}
	})

	var failIncrements atomic.Bool
	pluginEnd, engineEnd := net.Pipe()
	eng := &ospfRollbackEngine{mux: rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)), seen: make(chan string, 64)}
	go eng.serveFailingIncrements(ctx, &failIncrements)
	done := make(chan struct{})
	go func() {
		runOSPFEngine(pluginEnd)
		close(done)
	}()
	t.Cleanup(func() {
		_ = eng.mux.Close()
		_ = engineEnd.Close()
		_ = pluginEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("OSPF engine did not exit after its transport closed")
		}
	})

	eng.waitFor(t, ctx, "ze-plugin-engine:declare-registration")
	eng.callback(t, ctx, "ze-plugin-callback:configure",
		rpc.ConfigureInput{Sections: []rpc.ConfigSection{{Root: Namespace, Data: cfgA}}})
	eng.waitFor(t, ctx, "ze-plugin-engine:declare-capabilities")
	eng.callback(t, ctx, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{})
	eng.waitFor(t, ctx, "ze-plugin-engine:ready")

	eng.callback(t, ctx, "ze-plugin-callback:config-verify",
		rpc.ConfigVerifyInput{Sections: []rpc.ConfigSection{{Root: Namespace, Data: cfgB}}})
	failIncrements.Store(true)
	apply := callStatus(t, ctx, eng, "ze-plugin-callback:config-apply",
		rpc.ConfigApplyInput{Sections: []rpc.ConfigDiffSection{{Root: Namespace, Changed: cfgB}}})
	if apply.Status != rpc.StatusError {
		t.Fatalf("config-apply answered %q, want an error: instance 6 cannot initialize its state", apply.Status)
	}
	if !strings.Contains(apply.Error, sdk.ErrRunningConfigLost.Error()) {
		t.Fatalf("config-apply error %q does not say the running config is lost", apply.Error)
	}

	// The rollback handler's error reaches the engine as an RPC error.
	_, err := eng.mux.CallRPC(ctx, "ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "tx-b"})
	if err == nil {
		t.Fatal("config-rollback succeeded, want the error that restarts the plugin: instance 5 still cannot initialize")
	}
	if !strings.Contains(err.Error(), "rollback could not put the running config back") {
		t.Fatalf("config-rollback error %q is not the rollback's repair failure", err)
	}
}

// callStatus sends one engine-to-plugin callback and answers its status and
// error text, whichever they are.
func callStatus(t *testing.T, ctx context.Context, eng *ospfRollbackEngine, method string, input any) (answer struct {
	Status string `json:"status"`
	Error  string `json:"error"`
},
) {
	t.Helper()
	raw, err := eng.mux.CallRPC(ctx, method, input)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		t.Fatalf("%s answer %s: %v", method, raw, err)
	}
	return answer
}

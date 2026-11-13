// Design: docs/architecture/isis/isis-4-component-config.md -- runtime config rollback.
// Related: register.go -- the real SDK lifecycle callbacks under test.
package isis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	isisredistribute "github.com/ze-software/ze/internal/plugins/isis/redistribute"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// An apply accepted by IS-IS remains reversible until the whole transaction
// commits. A later participant's failure must restore the typed config, live
// circuits and own LSPs, including after removing the last NET.
func TestISISConfigRollbackRestoresRuntime(t *testing.T) {
	const before = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l2","interfaces":{"interface":{"isis-on-a":{"circuit-type":"point-to-point","hello-interval":"3600"}}}}}`
	for _, bridged := range []bool{false, true} {
		for _, tc := range []struct {
			name  string
			after string
		}{
			{"last NET removed", `{"isis":{"level":"l2","interfaces":{"interface":{"isis-on-a":{"circuit-type":"point-to-point","hello-interval":"3600"}}}}}`},
			{"L2 changed to L1", `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1","interfaces":{"interface":{"isis-on-a":{"circuit-type":"point-to-point","hello-interval":"3600"}}}}}`},
		} {
			t.Run(fmt.Sprintf("bridge=%t/%s", bridged, tc.name), func(t *testing.T) {
				useOnLinkBackend(t)
				eng, call := startISISRuntimeRollbackPlugin(t, bridged, before)
				assertISISRollbackRuntime(t, eng, before, "startup")
				verifyApply := func() {
					t.Helper()
					call("ze-plugin-callback:config-verify", rpc.ConfigVerifyInput{Sections: []rpc.ConfigSection{{Root: configRoot, Data: tc.after}}})
					call("ze-plugin-callback:config-apply", rpc.ConfigApplyInput{Sections: []rpc.ConfigDiffSection{{Root: configRoot, Changed: tc.after}}})
				}
				verifyApply()
				assertISISRollbackRuntime(t, eng, tc.after, "candidate applied")
				call("ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "later-participant-failed"})
				assertISISRollbackRuntime(t, eng, before, "rolled back")
				call("ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "duplicate-rollback"})
				assertISISRollbackRuntime(t, eng, before, "duplicate rollback")

				// Once committed, a later transaction's rollback must not replay the
				// retained candidate's undo set through the SDK gate.
				verifyApply()
				call("ze-plugin-callback:config-committed", map[string]string{"transaction-id": "accepted"})
				call("ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "another-participant-only"})
				assertISISRollbackRuntime(t, eng, tc.after, "committed candidate")
			})
		}
	}
}

// A failed circuit rebuild must refuse the callback and repair its own partial
// work. When repair also fails, the callback must identify lost running state so
// the SDK admits a compensating rollback or retries an otherwise equal config.
func TestISISConfigRuntimeFailureRecovery(t *testing.T) {
	const (
		before = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l2","interfaces":{"interface":{"isis-on-a":{"circuit-type":"point-to-point","hello-interval":"3600"}}}}}`
		after  = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1","interfaces":{"interface":{"isis-on-a":{"circuit-type":"point-to-point","hello-interval":"3600"}}}}}`
	)
	for _, bridged := range []bool{false, true} {
		for _, rollback := range []bool{false, true} {
			for _, failures := range []int{1, 2} {
				t.Run(fmt.Sprintf("bridge=%t/rollback=%t/failures=%d", bridged, rollback, failures), func(t *testing.T) {
					useOnLinkBackend(t)
					backend := &isisRuntimeFailBackend{}
					eng, call := startISISRuntimeRollbackPluginWithBackend(t, bridged, before, backend)
					assertISISRollbackRuntime(t, eng, before, "startup")
					mustCall := func(method string, input any) {
						t.Helper()
						if err := call(method, input); err != nil {
							t.Fatalf("%s: %v", method, err)
						}
					}
					verify := func() {
						t.Helper()
						mustCall("ze-plugin-callback:config-verify", rpc.ConfigVerifyInput{Sections: []rpc.ConfigSection{{Root: configRoot, Data: after}}})
					}
					apply := rpc.ConfigApplyInput{Sections: []rpc.ConfigDiffSection{{Root: configRoot, Changed: after}}}
					verify()
					if rollback {
						mustCall("ze-plugin-callback:config-apply", apply)
						assertISISRollbackRuntime(t, eng, after, "candidate applied")
					}
					backend.failNext(failures)
					var err error
					if rollback {
						err = call("ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "later-participant-failed"})
					} else {
						err = call("ze-plugin-callback:config-apply", apply)
					}
					if err == nil {
						t.Error("callback accepted an injected raw-circuit open failure")
					} else {
						if !strings.Contains(err.Error(), "injected IS-IS raw open failure") {
							t.Errorf("callback error loses the raw-circuit cause: %v", err)
						}
						lost := strings.Contains(err.Error(), sdk.ErrRunningConfigLost.Error())
						if lost != (failures == 2) {
							t.Errorf("callback lost-running-state marker = %t, want %t: %v", lost, failures == 2, err)
						}
					}
					if got := backend.failureCount(); got != failures {
						t.Errorf("raw open failures = %d, want %d including the self-repair attempt", got, failures)
					}
					if !rollback {
						if failures == 1 {
							assertISISRollbackRuntime(t, eng, before, "failed apply self-repaired")
						}
						// A lost-state apply must open the SDK rollback gate; a
						// repaired refusal leaves it closed and this is a no-op.
						mustCall("ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "failed-apply"})
						assertISISRollbackRuntime(t, eng, before, "failed apply compensated")
						return
					}
					if failures == 1 {
						assertISISRollbackRuntime(t, eng, after, "failed rollback self-repaired")
					}
					// If the undo lost its running config, an equal candidate must
					// be installed rather than skipped by ConfigTx's equality path.
					verify()
					mustCall("ze-plugin-callback:config-apply", apply)
					assertISISRollbackRuntime(t, eng, after, "candidate repaired after failed rollback")
				})
			}
		}
	}
}

// A failed added circuit must leave no transport enablement behind. A later
// link event is delivered after the SDK refusal to prove that the rejected
// interface cannot open outside the running config.
func TestISISConfigFailedAdditionDisablesTransport(t *testing.T) {
	const (
		before = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l2","interfaces":{"interface":{"isis-on-a":{"circuit-type":"point-to-point","hello-interval":"3600"}}}}}`
		after  = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l2","interfaces":{"interface":{"isis-on-a":{"circuit-type":"point-to-point","hello-interval":"3600"},"isis-on-b":{"circuit-type":"point-to-point","hello-interval":"3600"}}}}}`
	)
	for _, bridged := range []bool{false, true} {
		t.Run(fmt.Sprintf("bridge=%t", bridged), func(t *testing.T) {
			useOnLinkBackend(t)
			backend := &isisRuntimeFailBackend{}
			eng, call := startISISRuntimeRollbackPluginWithBackend(t, bridged, before, backend)
			assertISISRollbackRuntime(t, eng, before, "startup")
			if err := call("ze-plugin-callback:config-verify", rpc.ConfigVerifyInput{Sections: []rpc.ConfigSection{{Root: configRoot, Data: after}}}); err != nil {
				t.Fatal(err)
			}
			backend.failNext(1)
			err := call("ze-plugin-callback:config-apply", rpc.ConfigApplyInput{Sections: []rpc.ConfigDiffSection{{Root: configRoot, Changed: after}}})
			if err == nil || !strings.Contains(err.Error(), "injected IS-IS raw open failure") {
				t.Fatalf("failed addition response = %v, want the injected raw-open refusal", err)
			}
			if got := backend.failureCount(); got != 1 {
				t.Fatalf("candidate had %d raw-open failures, want one", got)
			}
			assertISISRollbackRuntime(t, eng, before, "failed addition repaired")
			backend.backend.mu.Lock()
			opened := len(backend.backend.opened)
			backend.backend.mu.Unlock()
			if err := eng.transport.HandleLinkUp("isis-on-b"); err != nil {
				t.Fatalf("link event for rejected interface: %v", err)
			}
			backend.backend.mu.Lock()
			openedAfter := len(backend.backend.opened)
			backend.backend.mu.Unlock()
			if openedAfter != opened {
				t.Errorf("rejected interface link event opened another raw circuit: %d -> %d", opened, openedAfter)
			}
			if got := eng.transport.OpenCircuitCount(); got != 1 {
				t.Errorf("transport has %d raw circuits after rejected-interface event, want the one running interface", got)
			}
		})
	}
}

// isisRuntimeFailBackend fails only the explicitly armed next raw opens. Startup
// and later recovery use the same fake circuit lifecycle as the success test.
type isisRuntimeFailBackend struct {
	backend   countingBackend
	mu        sync.Mutex
	remaining int
	failed    int
}

func (b *isisRuntimeFailBackend) failNext(count int) {
	b.mu.Lock()
	b.remaining, b.failed = count, 0
	b.mu.Unlock()
}

func (b *isisRuntimeFailBackend) failureCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.failed
}

func (b *isisRuntimeFailBackend) OpenCircuit(name string) (transport.CircuitHandle, error) {
	b.mu.Lock()
	if b.remaining > 0 {
		b.remaining--
		b.failed++
		b.mu.Unlock()
		return nil, errors.New("injected IS-IS raw open failure")
	}
	b.mu.Unlock()
	return b.backend.OpenCircuit(name)
}

// startISISRuntimeRollbackPlugin runs the production lifecycle with a fake raw
// transport. Startup uses the real five-stage socket protocol; runtime callbacks
// use the socket or DirectBridge selected by the caller, including the SDK gate.
func startISISRuntimeRollbackPlugin(t *testing.T, bridged bool, initial string) (*engine, func(string, any)) {
	t.Helper()
	eng, call := startISISRuntimeRollbackPluginWithBackend(t, bridged, initial, &fakeBackend{})
	return eng, func(method string, input any) {
		t.Helper()
		if err := call(method, input); err != nil {
			t.Fatalf("%s: %v", method, err)
		}
	}
}

// startISISRuntimeRollbackPluginWithBackend keeps the real callback response
// available so failure tests can assert a refusal rather than aborting on it.
func startISISRuntimeRollbackPluginWithBackend(t *testing.T, bridged bool, initial string, backend transport.Backend) (*engine, func(string, any) error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	previousClient := routeInstallClient()
	previousExport := isisredistribute.ExportChain()
	previousConsumer, hadConsumer := configredist.LookupConsumer(isisredistribute.ConsumerName)
	pluginEnd, engineEnd := net.Pipe()
	control := &isisRollbackEngine{mux: rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)), seen: make(chan string, 64)}
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		control.serve(ctx)
	}()
	var bridge *rpc.DirectBridge
	var conn net.Conn = pluginEnd
	if bridged {
		bridge = rpc.NewDirectBridge()
		bridge.SetDispatchRPC(func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			switch method {
			case rpc.MethodStateIncrement:
				return json.Marshal(rpc.StateOutput{Status: rpc.StateOK, Value: 1})
			case rpc.MethodStateGet:
				return json.Marshal(rpc.StateOutput{Status: rpc.StateAbsent})
			default:
				return json.RawMessage(`{}`), nil
			}
		})
		conn = rpc.NewBridgedConn(pluginEnd, bridge)
	}
	p := sdk.NewWithConn(componentName, conn)
	setRouteInstallClient(p)
	eng := newEngine(transport.New(backend))
	pluginDone := make(chan struct{})
	go func() {
		defer close(pluginDone)
		runISISPlugin(p, eng)
	}()
	t.Cleanup(func() {
		cancel()
		if err := p.Close(); err != nil {
			t.Errorf("close IS-IS SDK: %v", err)
		}
		if err := control.mux.Close(); err != nil {
			t.Errorf("close engine RPC: %v", err)
		}
		if err := engineEnd.Close(); err != nil {
			t.Errorf("close engine pipe: %v", err)
		}
		if err := pluginEnd.Close(); err != nil {
			t.Errorf("close plugin pipe: %v", err)
		}
		for _, done := range []<-chan struct{}{serveDone, pluginDone} {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("IS-IS rollback fixture worker did not exit")
			}
		}
		routeInstallPtr.Store(previousClient)
		isisredistribute.SetExportChain(previousExport)
		if hadConsumer {
			configredist.ReregisterConsumer(previousConsumer)
		}
	})
	control.waitFor(t, ctx, rpc.MethodDeclareRegistration)
	control.callback(t, ctx, "ze-plugin-callback:configure", rpc.ConfigureInput{Sections: []rpc.ConfigSection{{Root: configRoot, Data: initial}}})
	control.waitFor(t, ctx, "ze-plugin-engine:declare-capabilities")
	control.callback(t, ctx, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{})
	control.waitFor(t, ctx, "ze-plugin-engine:ready")
	call := func(method string, input any) error {
		t.Helper()
		var raw json.RawMessage
		var err error
		if bridge == nil {
			raw, err = control.mux.CallRPC(ctx, method, input)
		} else {
			params, marshalErr := json.Marshal(input)
			if marshalErr != nil {
				return marshalErr
			}
			raw, err = bridge.SendCallback(ctx, method, params)
		}
		if err != nil {
			return err
		}
		if len(raw) != 0 {
			var out struct {
				Status string `json:"status"`
				Error  string `json:"error"`
			}
			if err := json.Unmarshal(raw, &out); err != nil {
				return err
			}
			if out.Status == rpc.StatusError {
				return errors.New(out.Error)
			}
		}
		return nil
	}
	// A pre-apply rollback is a no-op under the SDK gate. Its receipt is also
	// a barrier proving OnStarted finished and the runtime event loop is live.
	if err := call("ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "startup-barrier"}); err != nil {
		t.Fatalf("startup barrier: %v", err)
	}
	return eng, call
}

// assertISISRollbackRuntime reads the typed engine config, the live adjacency
// circuit and the own LSP's connected prefix. The RPC receipt precedes every
// read, so assertions need no timer or polling approximation of completion.
func assertISISRollbackRuntime(t *testing.T, eng *engine, data, stage string) {
	t.Helper()
	want, err := parseISISConfig(sec(data))
	if err != nil {
		t.Fatal(err)
	}
	eng.mu.Lock()
	got := eng.cfg
	running := len(eng.running)
	eng.mu.Unlock()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: typed runtime config = %+v, want %+v", stage, got, want)
	}
	eng.circuitsMu.RLock()
	c := eng.circuitByName["isis-on-a"]
	circuitCount := len(eng.circuits)
	eng.circuitsMu.RUnlock()
	if !want.Present() {
		if running != 0 || circuitCount != 0 || c != nil {
			t.Errorf("%s: inactive config has %d running interfaces and %d circuits, named circuit %p", stage, running, circuitCount, c)
		}
		return
	}
	if running != 1 || circuitCount != 1 || c == nil {
		t.Errorf("%s: active config has %d running interfaces and %d circuits, named circuit %p", stage, running, circuitCount, c)
		return
	}
	level, lspLevel := adjacency.Level2, lsdb.Level2
	if want.Level == LevelL1 {
		level, lspLevel = adjacency.Level1, lsdb.Level1
	}
	schedules := c.HelloSchedules()
	if len(schedules) != 1 || schedules[0].Level != level || schedules[0].Period != time.Hour {
		t.Errorf("%s: live circuit Hello schedules = %+v, want one %s at 1h", stage, schedules, level)
	}
	id := types.NewLSPID(types.NewSourceID(want.SystemID, 0), 0)
	entry := eng.lsdb.Lookup(lspLevel, id)
	if entry == nil || entry.IsPurged() {
		t.Errorf("%s: own %s LSP %s missing or purged", stage, lspLevel, id)
		return
	}
	lsp, err := entry.Decode()
	if err != nil {
		t.Fatal(err)
	}
	defer packet.ReleaseTLVs(lsp.TLVs)
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	for _, tlv := range lsp.TLVs {
		if tlv.Type != packet.TLVExtendedIPReach {
			continue
		}
		reach, err := packet.DecodeExtendedIPReachTLV(tlv.Value)
		if err != nil {
			t.Fatal(err)
		}
		for _, advertised := range reach.Entries {
			if advertised.Prefix == prefix {
				return
			}
		}
	}
	t.Errorf("%s: own %s LSP does not advertise connected prefix %s", stage, lspLevel, prefix)
}

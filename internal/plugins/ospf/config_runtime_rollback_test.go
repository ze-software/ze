// Design: docs/architecture/ospf/ospf-4-component-config.md -- transactional runtime configuration.
// Related: export_chain_rollback_test.go -- the engine-side startup and callback fixture.
package ospf

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"testing"
	"time"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	ospfiface "github.com/ze-software/ze/internal/plugins/ospf/iface"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfredistribute "github.com/ze-software/ze/internal/plugins/ospf/redistribute"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	runtimeRollbackConfigA = `{"ospf":{"router-id":"10.0.0.1","timers":{"min-ls-interval-ms":1},` +
		`"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{` +
		`"eth0":{"area":"0","passive":"true","cost":"17"},` +
		`"eth1":{"area":"0","passive":"true","instance-id":"5"}}}}}`
	runtimeRollbackConfigB = `{"ospf":{"router-id":"10.0.0.1","timers":{"min-ls-interval-ms":1},` +
		`"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{` +
		`"eth0":{"area":"0","passive":"true","cost":"29"},` +
		`"eth2":{"area":"0","passive":"true","instance-id":"6"}}}}}`
)

// TestOSPFConfigRollbackRestoresRuntime proves that an accepted apply followed
// by a later participant's failure restores OSPF's engines, not only its export
// policy. Method: drive the real plugin through both SDK transports, observe B's
// changed interface cost, Router-LSA and instance set, roll it back, and observe
// A again. A committed B and a repeated rollback must retain their current state.
func TestOSPFConfigRollbackRestoresRuntime(t *testing.T) {
	// The registered interface backend and redistribution consumer are global;
	// these cases MUST NOT run in parallel, and engines stop before restoration.
	installTopologyAddressBackend(t)
	for _, mode := range []string{"socket", "direct"} {
		t.Run(mode, func(t *testing.T) {
			f := newOSPFRuntimeRollbackFixture(t, mode)
			f.assertRuntime(t, 17, 5)
			f.apply(t, runtimeRollbackConfigB)
			f.assertRuntime(t, 29, 6)

			f.callback(t, "ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "tx-b"})
			f.assertRuntime(t, 17, 5)
			f.callback(t, "ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "tx-b"})
			f.assertRuntime(t, 17, 5)

			f.apply(t, runtimeRollbackConfigB)
			f.callback(t, "ze-plugin-callback:config-committed", map[string]string{"transaction-id": "tx-c"})
			f.callback(t, "ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "tx-unapplied"})
			f.assertRuntime(t, 29, 6)
		})
	}
}

// TestOSPFConfigRollbackRestoresRemovedRoot proves that an explicit empty
// OSPF delivery stops its interfaces and child instances and remains reversible.
// Method: send the daemon's root-removal representation through each SDK
// transport, inspect the empty runtime, then roll back and inspect A again.
func TestOSPFConfigRollbackRestoresRemovedRoot(t *testing.T) {
	installTopologyAddressBackend(t)
	for _, mode := range []string{"socket", "direct"} {
		t.Run(mode, func(t *testing.T) {
			f := newOSPFRuntimeRollbackFixture(t, mode)
			f.assertRuntime(t, 17, 5)
			f.apply(t, "{}")
			var interfaces []ospfiface.Snapshot
			f.document(t, "show ospf interface", &interfaces)
			if len(interfaces) != 0 {
				t.Fatalf("removed root retained interfaces: %+v", interfaces)
			}
			var instances []instanceSummaryView
			f.document(t, "show ospf instance", &instances)
			if len(instances) != 1 {
				t.Fatalf("removed root retained child instances: %+v", instances)
			}
			if got := instances[0]; got.InstanceID != 0 || got.InterfaceCount != 0 {
				t.Fatalf("removed root base instance = %+v, want idle instance 0", got)
			}
			f.callback(t, "ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "tx-remove"})
			f.assertRuntime(t, 17, 5)
		})
	}
}

// ospfRuntimeRollbackFixture owns one real plugin lifecycle. Its cleanup MUST
// stop and join the plugin before restoring the shared redistribution consumer.
type ospfRuntimeRollbackFixture struct {
	ctx    context.Context
	engine *ospfRollbackEngine
	bridge *rpc.DirectBridge
}

func newOSPFRuntimeRollbackFixture(t *testing.T, mode string) *ospfRuntimeRollbackFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	previous, hadPrevious := configredist.LookupConsumer(ospfredistribute.ConsumerName)
	t.Cleanup(func() {
		ospfredistribute.SetExportChain(nil)
		if hadPrevious {
			configredist.ReregisterConsumer(previous)
		}
	})
	pluginEnd, engineEnd := net.Pipe()
	e := &ospfRollbackEngine{mux: rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)), seen: make(chan string, 64)}
	f := &ospfRuntimeRollbackFixture{ctx: ctx, engine: e}
	var conn net.Conn = pluginEnd
	if mode == "direct" {
		f.bridge = rpc.NewDirectBridge()
		f.bridge.SetDispatchRPC(runtimeRollbackEngineReply)
		conn = rpc.NewBridgedConn(pluginEnd, f.bridge)
	}
	go e.serve(ctx)
	done := make(chan int, 1)
	go func() { done <- runOSPFEngine(conn) }()
	t.Cleanup(func() {
		if f.bridge != nil {
			f.bridge.CloseCallbacks()
		}
		_ = e.mux.Close()     //nolint:errcheck // Closing both pipe ends also wakes an interrupted startup.
		_ = engineEnd.Close() //nolint:errcheck // Cleanup closes both ends even if one already closed.
		_ = pluginEnd.Close() //nolint:errcheck // Cleanup closes both ends even if one already closed.
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("OSPF plugin did not stop after transport shutdown")
		}
	})
	e.waitFor(t, ctx, rpc.MethodDeclareRegistration)
	e.callback(t, ctx, "ze-plugin-callback:configure",
		rpc.ConfigureInput{Sections: []rpc.ConfigSection{{Root: Namespace, Data: runtimeRollbackConfigA}}})
	e.waitFor(t, ctx, "ze-plugin-engine:declare-capabilities")
	e.callback(t, ctx, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{})
	e.waitFor(t, ctx, "ze-plugin-engine:ready")
	// This callback runs only after OnStarted completes. It is an ordering
	// barrier, not a wait based on elapsed time, and no apply is open yet.
	f.callback(t, "ze-plugin-callback:config-committed", map[string]string{"transaction-id": "startup"})
	return f
}

// runtimeRollbackEngineReply is the same absent-state/bootstrap contract as
// ospfRollbackEngine.serve, on the engine side of the direct transport.
func runtimeRollbackEngineReply(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
	switch method {
	case rpc.MethodStateIncrement:
		return json.Marshal(rpc.StateOutput{Status: rpc.StateOK, Value: 1})
	case rpc.MethodStateGet:
		return json.Marshal(rpc.StateOutput{Status: rpc.StateAbsent})
	default:
		return json.Marshal(struct {
			Status string `json:"status"`
		}{Status: rpc.StatusOK})
	}
}

func (f *ospfRuntimeRollbackFixture) callback(t *testing.T, method string, input any) {
	t.Helper()
	if f.bridge == nil {
		f.engine.callback(t, f.ctx, method, input)
		return
	}
	params, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := f.bridge.SendCallback(f.ctx, method, params)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	if len(raw) == 0 {
		return
	}
	var status struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatalf("%s response %s: %v", method, raw, err)
	}
	if status.Status == rpc.StatusError {
		t.Fatalf("%s refused: %s", method, status.Error)
	}
}

func (f *ospfRuntimeRollbackFixture) apply(t *testing.T, cfg string) {
	t.Helper()
	f.callback(t, "ze-plugin-callback:config-verify",
		rpc.ConfigVerifyInput{Sections: []rpc.ConfigSection{{Root: Namespace, Data: cfg}}})
	f.callback(t, "ze-plugin-callback:config-apply",
		rpc.ConfigApplyInput{Sections: []rpc.ConfigDiffSection{{Root: Namespace, Changed: cfg}}})
}

func (f *ospfRuntimeRollbackFixture) document(t *testing.T, command string, out any) {
	t.Helper()
	var document json.RawMessage
	if f.bridge != nil {
		answer, err := f.bridge.ExecuteCommand(f.ctx, "rollback", command, nil, "")
		if err != nil {
			t.Fatalf("%s: %v", command, err)
		}
		if answer.Status != "done" {
			t.Fatalf("%s status = %s, want done", command, answer.Status)
		}
		document = answer.Data
	} else {
		answer, err := f.engine.mux.CallAnswer(f.ctx, "ze-plugin-callback:execute-command",
			rpc.ExecuteCommandInput{Serial: "rollback", Command: command})
		if err != nil {
			t.Fatalf("%s: %v", command, err)
		}
		for record := range answer.Records {
			document = record.Item
		}
		if err := answer.Err(); err != nil {
			t.Fatalf("%s answer: %v", command, err)
		}
	}
	if err := json.Unmarshal(document, out); err != nil {
		t.Fatalf("%s document %s: %v", command, document, err)
	}
}

func (f *ospfRuntimeRollbackFixture) assertRuntime(t *testing.T, cost uint16, child uint8) {
	t.Helper()
	var interfaces []ospfiface.Snapshot
	f.document(t, "show ospf interface", &interfaces)
	if len(interfaces) != 1 {
		t.Fatalf("base interfaces = %+v, want only eth0", interfaces)
	}
	if got := interfaces[0]; got.Name != "eth0" || got.Cost != cost || !got.Passive {
		t.Fatalf("base interface = %+v, want passive eth0 cost %d", got, cost)
	}
	var instances []instanceSummaryView
	f.document(t, "show ospf instance", &instances)
	if len(instances) != 2 {
		t.Fatalf("instances = %+v, want base and instance %d", instances, child)
	}
	seen := make(map[uint8]bool)
	for _, instance := range instances {
		seen[instance.InstanceID] = true
		if instance.InterfaceCount != 1 {
			t.Fatalf("instance = %+v, want one interface", instance)
		}
	}
	if !seen[0] || !seen[child] {
		t.Fatalf("instances = %+v, want IDs 0 and %d", instances, child)
	}
	f.assertRouterMetric(t, cost)
}

// assertRouterMetric checks the checksum and length of the exact expected
// one-stub Router-LSA at its observed sequence. The public database view carries
// metadata only; encoding the known body verifies that the live LSA changed too.
func (f *ospfRuntimeRollbackFixture) assertRouterMetric(t *testing.T, cost uint16) {
	t.Helper()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	var last []ospflsdb.Snapshot
	for {
		f.document(t, cmdShowDatabaseRouter, &last)
		for _, snapshot := range last {
			for _, area := range snapshot.Areas {
				for _, row := range area.LSAs {
					if row.AdvertisingRouter != "10.0.0.1" || row.LinkStateID != "10.0.0.1" {
						continue
					}
					seq, err := strconv.ParseInt(row.Sequence, 10, 32)
					if err != nil {
						t.Fatalf("Router-LSA sequence %q: %v", row.Sequence, err)
					}
					body := packet.RouterLSA{Links: []packet.RouterLink{{
						LinkID: types.LinkStateID{192, 0, 2, 0}, LinkData: [4]byte{255, 255, 255, 0},
						Type: packet.RouterLinkTypeStub, Metric: types.Metric(cost),
					}}}
					lsa := packet.LSA{Header: packet.LSAHeader{
						Options: types.OptionE, Type: types.LSTypeRouter,
						LinkStateID: types.LinkStateID{10, 0, 0, 1}, AdvertisingRouter: types.RouterID{10, 0, 0, 1},
						Sequence: types.LSSequenceNumber(uint32(seq)),
					}, Router: &body}
					wire := make([]byte, lsa.EncodedLen())
					lsa.WriteTo(wire, 0)
					if row.Age < types.MaxAge && row.Checksum == lsa.Header.Checksum && row.Length == lsa.Header.Length {
						return
					}
				}
			}
		}
		select {
		case <-f.ctx.Done():
			t.Fatalf("Router-LSA did not advertise subnet 192.0.2.0/24 at cost %d: %+v", cost, last)
		case <-tick.C:
		}
	}
}

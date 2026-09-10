package local

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

// sweepBackend models nft ownership at the backend boundary. The real registry
// merges owners; this backend retains unclaimed state from a previous process.
// Tests inspect the resulting tables, never a sequence of registry calls.
// Safe for concurrent use by the engine and the test observer.
type sweepBackend struct {
	mu        sync.Mutex
	tables    map[string]firewall.Table
	applied   map[string]bool
	failAfter int
	failure   error
}

var sweepBackendID atomic.Uint64

func newSweepBackend(t *testing.T) (*sweepBackend, string) {
	t.Helper()
	b := &sweepBackend{tables: make(map[string]firewall.Table), applied: make(map[string]bool)}
	name := fmt.Sprintf("ddos-sweep-%d", sweepBackendID.Add(1))
	if err := firewall.RegisterBackend(name, func() (firewall.Backend, error) { return b, nil }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = firewall.RegisterTables(tableName, nil)
		_ = firewall.RegisterTables("copp", nil)
		_ = firewall.RegisterTables("firewall", nil)
		if err := firewall.CloseBackend(); err != nil {
			t.Error(err)
		}
	})
	return b, name
}

func (b *sweepBackend) Apply(desired []firewall.Table) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failAfter > 0 {
		b.failAfter--
		if b.failAfter == 0 {
			return b.failure
		}
	}
	names := make(map[string]bool, len(desired))
	for _, table := range desired {
		names[table.Name] = true
	}
	for key, table := range b.tables {
		if names[table.Name] || b.applied[table.Name] {
			delete(b.tables, key)
		}
	}
	for _, table := range desired {
		b.tables[fmt.Sprintf("%s/%d", table.Name, table.Family)] = table
	}
	b.applied = names
	return nil
}

func (b *sweepBackend) ListTables() ([]firewall.Table, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	tables := make([]firewall.Table, 0, len(b.tables))
	for _, table := range b.tables {
		tables = append(tables, table)
	}
	return tables, nil
}

func (*sweepBackend) GetCounters(string) ([]firewall.ChainCounters, error) { return nil, nil }
func (*sweepBackend) Close() error                                         { return nil }

// plant leaves a rule outside this backend instance's applied set, as a restart does.
func (b *sweepBackend) plant(table firewall.Table) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tables[fmt.Sprintf("%s/%d", table.Name, table.Family)] = table
}

func (b *sweepBackend) failReconcile(after int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failAfter, b.failure = after, err
}

func requireSwept(t *testing.T, b *sweepBackend) {
	t.Helper()
	tables, err := b.ListTables()
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range tables {
		if table.Name == "ze_ddos-local" {
			t.Fatalf("a previous process's response table survived: %+v", table)
		}
	}
}

// TestStaleDropRuleSweepClaimsTheNameThenWithdrawsIt preserves the two-reconcile
// contract through final state: both address families disappear, including the
// empty claim, while another owner's complete desired rule survives.
func TestStaleDropRuleSweepClaimsTheNameThenWithdrawsIt(t *testing.T) {
	b, name := newSweepBackend(t)
	if err := firewall.LoadBackend(name); err != nil {
		t.Fatal(err)
	}
	b.plant(firewall.Table{Name: "ze_ddos-local", Family: firewall.FamilyIP})
	b.plant(firewall.Table{Name: "ze_ddos-local", Family: firewall.FamilyIP6})
	other := firewall.Table{Name: "ze_copp", Family: firewall.FamilyInet,
		Chains: []firewall.Chain{{Name: "input", Terms: []firewall.Term{{Actions: []firewall.Action{firewall.Drop{}}}}}}}
	if err := firewall.RegisterTables("copp", []firewall.Table{other}); err != nil {
		t.Fatal(err)
	}
	if err := firewall.ApplyAll(); err != nil {
		t.Fatal(err)
	}
	if err := clearStaleDropRule(); err != nil {
		t.Fatal(err)
	}
	requireSwept(t, b)
	tables, err := b.ListTables()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tables, []firewall.Table{other}) {
		t.Fatalf("the other owner's rule changed: %+v", tables)
	}
}

// TestStaleDropRuleSweepClaimsTheTableUnderTheResponderName starts with a rule
// the actual responder installed, then forgets process ownership before the sweep.
// A sweep under another table or owner name leaves state after the next reconcile.
func TestStaleDropRuleSweepClaimsTheTableUnderTheResponderName(t *testing.T) {
	b, name := newSweepBackend(t)
	if err := firewall.LoadBackend(name); err != nil {
		t.Fatal(err)
	}
	r := newResponder(enforcing(), nil)
	r.onDetected(&ddosevent.AttackDetected{Target: floodVictim(), Direction: ddosevent.DirectionLocal})
	tables, err := b.ListTables()
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 || len(tables[0].Chains) == 0 {
		t.Fatalf("the responder installed no drop: %+v", tables)
	}
	if err := firewall.RegisterTables(tableName, nil); err != nil {
		t.Fatal(err)
	}
	b.applied = make(map[string]bool)
	if err := clearStaleDropRule(); err != nil {
		t.Fatal(err)
	}
	if err := firewall.ApplyAll(); err != nil {
		t.Fatal(err)
	}
	requireSwept(t, b)
}

// TestStaleDropRuleSweepLeavesNoClaimBehindOnAFailedReconcile checks both failure
// stages. A later owner reconcile must neither create an empty claim nor retain
// the empty table from a successful first stage.
func TestStaleDropRuleSweepLeavesNoClaimBehindOnAFailedReconcile(t *testing.T) {
	for _, stage := range []int{1, 2} {
		t.Run(fmt.Sprintf("stage-%d", stage), func(t *testing.T) {
			b, name := newSweepBackend(t)
			if err := firewall.LoadBackend(name); err != nil {
				t.Fatal(err)
			}
			wedged := errors.New("the kernel is wedged")
			b.failReconcile(stage, wedged)
			if err := clearStaleDropRule(); !errors.Is(err, wedged) {
				t.Fatalf("the failed reconcile was not reported: %v", err)
			}
			if err := firewall.ApplyAll(); err != nil {
				t.Fatal(err)
			}
			requireSwept(t, b)
		})
	}
}

// startSweepEngine runs the production engine over the SDK transport. The caller
// MUST complete its handshake; cleanup closes the transport and joins the engine.
func startSweepEngine(t *testing.T, run func(net.Conn) int) *rpc.MuxConn {
	t.Helper()
	pluginEnd, engineEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	exited := make(chan int, 1)
	go func() { exited <- run(pluginEnd) }()
	t.Cleanup(func() {
		_ = mux.Close()
		_ = engineEnd.Close()
		_ = pluginEnd.Close()
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
			t.Error("plugin engine did not exit after its transport closed")
		}
	})
	return mux
}

func sweepRequest(t *testing.T, ctx context.Context, mux *rpc.MuxConn, method string) {
	t.Helper()
	select {
	case req := <-mux.Requests():
		if req == nil || req.Method != method {
			t.Fatalf("waiting for %s, got %+v", method, req)
		}
		if err := mux.SendOK(ctx, req.ID); err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatalf("waiting for %s: %v", method, ctx.Err())
	}
}

func sweepCallback(t *testing.T, ctx context.Context, mux *rpc.MuxConn, method string, input any) {
	t.Helper()
	raw, err := mux.CallRPC(ctx, method, input)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	if len(raw) != 0 {
		var result rpc.ConfigApplyOutput
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatalf("%s response: %v", method, err)
		}
		if result.Status == rpc.StatusError {
			t.Fatalf("%s rejected: %s", method, result.Error)
		}
	}
}

func configureSweepEngine(t *testing.T, ctx context.Context, mux *rpc.MuxConn, sections []rpc.ConfigSection) {
	t.Helper()
	sweepCallback(t, ctx, mux, "ze-plugin-callback:configure", rpc.ConfigureInput{Sections: sections})
	sweepRequest(t, ctx, mux, "ze-plugin-engine:declare-capabilities")
	sweepCallback(t, ctx, mux, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{})
	sweepRequest(t, ctx, mux, "ze-plugin-engine:ready")
}

// TestLocalStartupSweepsTheConfiguredBackend exercises actual firewall and local
// engine SDK startup. Both engines exist before either gets config, as they do in
// runPluginPhase. The firewall tier configures first. A pre-handshake sweep leaves
// the configured backend's stale table intact, so the final-state assertion fails.
// The reload half proves that a new attack's live rule is never swept at apply.
func TestLocalStartupSweepsTheConfiguredBackend(t *testing.T) {
	b, name := newSweepBackend(t)
	b.plant(firewall.Table{Name: "ze_ddos-local", Family: firewall.FamilyIP})
	mux, ctx, bus := configureSweepStartup(t, b, name, nil)
	requireSwept(t, b)
	if _, err := ddosevent.Detected.Emit(bus, &ddosevent.AttackDetected{
		Target: floodVictim(), Direction: ddosevent.DirectionLocal,
	}); err != nil {
		t.Fatal(err)
	}
	before, err := b.ListTables()
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || len(before[0].Chains) == 0 {
		t.Fatalf("fresh detection installed no rule: %+v", before)
	}
	sections := []rpc.ConfigSection{{Root: configRoot, Data: `{"ddos":{"local":{"response-level":"enforce","max-mitigation-duration":"120"}}}`}}
	sweepCallback(t, ctx, mux, "ze-plugin-callback:config-verify", rpc.ConfigVerifyInput{Sections: sections})
	sweepCallback(t, ctx, mux, "ze-plugin-callback:config-apply", rpc.ConfigApplyInput{})
	after, err := b.ListTables()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("reload swept a rule installed by this engine: before=%+v after=%+v", before, after)
	}
}

// TestLocalStartupCleanupFailureStillHandlesDetection fails cleanup at each stage
// and then emits a fresh detector event. Successful mitigation proves startup
// retained the responder and its subscription despite the cleanup error.
func TestLocalStartupCleanupFailureStillHandlesDetection(t *testing.T) {
	for _, stage := range []int{1, 2} {
		t.Run(fmt.Sprintf("stage-%d", stage), func(t *testing.T) {
			b, name := newSweepBackend(t)
			_, _, bus := configureSweepStartup(t, b, name, func() {
				b.failReconcile(stage, errors.New("cleanup unavailable"))
			})
			if _, err := ddosevent.Detected.Emit(bus, &ddosevent.AttackDetected{
				Target: floodVictim(), Direction: ddosevent.DirectionLocal,
			}); err != nil {
				t.Fatal(err)
			}
			tables, err := b.ListTables()
			if err != nil {
				t.Fatal(err)
			}
			if len(tables) != 1 || len(tables[0].Chains) == 0 {
				t.Fatalf("cleanup failure prevented a fresh mitigation: %+v", tables)
			}
		})
	}
}

// configureSweepStartup starts both engines before the firewall handshake, then
// delivers config in dependency order. The transport cleanup MUST join both
// engines before the shared backend and bus are restored.
func configureSweepStartup(t *testing.T, b *sweepBackend, name string, beforeLocal func()) (*rpc.MuxConn, context.Context, ze.EventBus) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	bus, err := server.NewServer(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	previous := eventBusPtr.Load()
	var events ze.EventBus = bus
	eventBusPtr.Store(&events)
	t.Cleanup(func() { eventBusPtr.Store(previous) })
	fw := registry.Lookup("firewall")
	if fw == nil {
		t.Fatal("firewall dependency is not registered")
	}
	firewallMux := startSweepEngine(t, fw.RunEngine)
	localMux := startSweepEngine(t, runEngine)
	// Receiving this declaration proves runEngine reached the SDK. No sleep or
	// recorded helper order stands in for the pre-configuration boundary.
	sweepRequest(t, ctx, localMux, "ze-plugin-engine:declare-registration")
	sweepRequest(t, ctx, firewallMux, "ze-plugin-engine:declare-registration")
	configureSweepEngine(t, ctx, firewallMux, []rpc.ConfigSection{{Root: "firewall",
		Data: fmt.Sprintf(`{"firewall":{"backend":%q,"flush-on-shutdown":"false"}}`, name)}})
	if firewall.GetBackend() != b {
		t.Fatal("firewall configure did not select the requested backend")
	}
	if beforeLocal != nil {
		beforeLocal()
	}
	configureSweepEngine(t, ctx, localMux, []rpc.ConfigSection{{Root: configRoot,
		Data: `{"ddos":{"local":{"response-level":"enforce","max-mitigation-duration":"3600"}}}`}})
	return localMux, ctx, bus
}

// captureLog routes the package logger into a buffer for one test, and restores
// what was there. It reads at Debug so no line the responder writes is filtered
// out by level.
func captureLog(t *testing.T) (lines func() string) {
	t.Helper()
	buf := &bytes.Buffer{}
	previous := loggerPtr.Load()
	loggerPtr.Store(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { loggerPtr.Store(previous) })
	return buf.String
}

// TestRemoveMitigationDoesNotReportARemovalTheKernelRefused proves a withdrawal
// the kernel refused is reported as the failure it is.
//
// VALIDATES: the operator can tell a rule that went out from one that did not.
// PREVENTS: "drop rule removed" printed on the line after "failed to remove drop
// rule". Most callers have a later reconcile that repairs a failed withdrawal,
// because the detector re-fires about once a second. The engine's exit path has
// none. There this line is the only witness the operator gets, and it said the
// opposite of what happened (ai/rules/evidence.md).
func TestRemoveMitigationDoesNotReportARemovalTheKernelRefused(t *testing.T) {
	lines := captureLog(t)

	origReg := registerTables
	origApply := applyAll
	defer func() {
		registerTables = origReg
		applyAll = origApply
	}()
	registerTables = func(string, []firewall.Table) error { return nil }
	applyAll = func() error { return errors.New("the kernel is wedged") }

	r := newResponder(enforcing(), nil)
	r.mu.Lock()
	r.setStatus(true, floodVictim(), firewall.HookInput)
	r.removeMitigation()
	r.mu.Unlock()

	log := lines()
	if !strings.Contains(log, "failed to remove drop rule") {
		t.Errorf("a refused withdrawal must be reported, log was:\n%s", log)
	}
	if strings.Contains(log, "drop rule removed") {
		t.Errorf("a refused withdrawal must not also report a removal: the rule is still in the kernel and this line is what an operator acts on. Log was:\n%s", log)
	}
}

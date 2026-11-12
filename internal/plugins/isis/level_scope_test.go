// Design: docs/architecture/isis/isis-4-component-config.md -- instance and circuit levels.
// Related: lsdb_wiring.go -- adjacency transitions produce per-level reachability.
package isis

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	isisredistribute "github.com/ze-software/ze/internal/plugins/isis/redistribute"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The interface default and an explicit dual-level request are limited by the
// instance; single-level overrides on a dual-level instance stay independent.
func TestISISInstanceLevelScopesCircuits(t *testing.T) {
	for _, tc := range []struct {
		instance string
		circuit  string
		want     Level
	}{
		{"l1", "", LevelL1},
		{"l2", "", LevelL2},
		{"l1", `,"level":"l1-l2"`, LevelL1},
		{"l2", `,"level":"l1-l2"`, LevelL2},
		{"l1-l2", `,"level":"l1"`, LevelL1},
		{"l1-l2", `,"level":"l2"`, LevelL2},
		{"l1-l2", "", LevelL1L2},
	} {
		t.Run(tc.instance+tc.circuit, func(t *testing.T) {
			cfg, err := parseISISConfig(sec(fmt.Sprintf(`{"isis":{"net":"49.0001.0000.0000.0001.00","level":%q,"interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point"%s}}}}}`, tc.instance, tc.circuit)))
			if err != nil {
				t.Fatal(err)
			}
			circuits := cfg.EnabledCircuits()
			if len(circuits) != 1 {
				t.Fatalf("enabled circuits = %+v, want eth0", circuits)
			}
			if circuits[0].Level != tc.want {
				t.Errorf("effective level = %s, want %s", circuits[0].Level, tc.want)
			}
		})
	}
}

// A circuit with no level in common cannot honor its configuration. Verify must
// refuse it before any socket or adjacency is opened, with an operator fix.
func TestISISInstanceLevelRefusesIncompatibleCircuit(t *testing.T) {
	for _, tc := range []struct{ instance, circuit string }{{"l1", "l2"}, {"l2", "l1"}} {
		t.Run(tc.instance+"/"+tc.circuit, func(t *testing.T) {
			cfg, err := parseISISConfig(sec(fmt.Sprintf(`{"isis":{"net":"49.0001.0000.0000.0001.00","level":%q,"interfaces":{"interface":{"eth0":{"level":%q}}}}}`, tc.instance, tc.circuit)))
			if err != nil {
				t.Fatal(err)
			}
			err = validateConfig(cfg)
			if err == nil {
				t.Fatal("verify accepted a circuit whose only level the instance disables")
			}
			if !strings.Contains(err.Error(), "eth0") || !strings.Contains(err.Error(), "set isis interfaces interface eth0 level l1-l2") {
				t.Errorf("refusal lacks interface and corrective command: %v", err)
			}
		})
	}
}

// Startup must reject the same incompatible levels as commit verification.
// The real plugin receives Stage 2 over RPC; empty and partial configurations
// remain accepted, and no Stage 4 registry is delivered to start raw circuits.
func TestISISInstanceLevelStartupConfigure(t *testing.T) {
	for _, tc := range []struct {
		name    string
		config  string
		refused bool
	}{
		{name: "no sections"},
		{name: "empty", config: `{"isis":{}}`},
		{name: "partial without net", config: `{"isis":{"interfaces":{"interface":{"eth0":{}}}}}`},
		{name: "l2 default interface", config: `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l2","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point"}}}}}`},
		{name: "dual instance explicit l1", config: `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1-l2","interfaces":{"interface":{"eth0":{"level":"l1"}}}}}`},
		{name: "dual instance explicit l2", config: `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1-l2","interfaces":{"interface":{"eth0":{"level":"l2"}}}}}`},
		{name: "disabled incompatible interface", config: `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l2","interfaces":{"interface":{"eth0":{"level":"l1","enabled":"false"}}}}}`},
		{name: "l1 rejects l2", config: `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1","interfaces":{"interface":{"eth0":{"level":"l2"}}}}}`, refused: true},
		{name: "l2 rejects l1", config: `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l2","interfaces":{"interface":{"eth0":{"level":"l1"}}}}}`, refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sections []rpc.ConfigSection
			if tc.config != "" {
				sections = []rpc.ConfigSection{{Root: configRoot, Data: tc.config}}
			}
			err := configureISISStartupLevels(t, sections)
			if !tc.refused {
				if err != nil {
					t.Fatalf("startup refused accepted configuration: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("startup accepted an enabled interface with no common instance level")
			}
			if !strings.Contains(err.Error(), "eth0") || !strings.Contains(err.Error(), "set isis interfaces interface eth0 level l1-l2") {
				t.Errorf("startup refusal lacks interface and corrective command: %v", err)
			}
		})
	}
}

// A partial configuration without a NET stays acceptable only when its enabled
// interface levels agree with the instance. The real SDK config-verify callback
// must refuse incompatible pairs before config-apply can activate them.
func TestISISInstanceLevelVerifyPartialConfig(t *testing.T) {
	var previousConsumers []configredist.RedistConsumer
	for _, name := range configredist.ConsumerNames() {
		consumer, ok := configredist.LookupConsumer(name)
		if !ok {
			t.Fatalf("registered redistribution consumer %s disappeared", name)
		}
		previousConsumers = append(previousConsumers, consumer)
	}
	t.Cleanup(func() {
		configredist.ResetConsumersForTest()
		for _, consumer := range previousConsumers {
			configredist.ReregisterConsumer(consumer)
		}
	})
	eng, ctx := startISISLevelPlugin(t)
	eng.callback(t, ctx, "ze-plugin-callback:configure", rpc.ConfigureInput{})
	eng.waitFor(t, ctx, "ze-plugin-engine:declare-capabilities")
	eng.callback(t, ctx, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{})
	eng.waitFor(t, ctx, "ze-plugin-engine:ready")
	for _, tc := range []struct {
		name    string
		config  string
		refused bool
	}{
		{name: "no sections"},
		{name: "empty", config: `{"isis":{}}`},
		{name: "compatible partial l1", config: `{"isis":{"level":"l1","interfaces":{"interface":{"eth0":{}}}}}`},
		{name: "compatible partial l2", config: `{"isis":{"level":"l2","interfaces":{"interface":{"eth0":{}}}}}`},
		{name: "dual partial explicit l1", config: `{"isis":{"level":"l1-l2","interfaces":{"interface":{"eth0":{"level":"l1"}}}}}`},
		{name: "dual partial explicit l2", config: `{"isis":{"level":"l1-l2","interfaces":{"interface":{"eth0":{"level":"l2"}}}}}`},
		{name: "disabled incompatible partial", config: `{"isis":{"level":"l2","interfaces":{"interface":{"eth0":{"level":"l1","enabled":"false"}}}}}`},
		{name: "partial l1 rejects l2", config: `{"isis":{"level":"l1","interfaces":{"interface":{"eth0":{"level":"l2"}}}}}`, refused: true},
		{name: "partial l2 rejects l1", config: `{"isis":{"level":"l2","interfaces":{"interface":{"eth0":{"level":"l1"}}}}}`, refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sections []rpc.ConfigSection
			if tc.config != "" {
				sections = []rpc.ConfigSection{{Root: configRoot, Data: tc.config}}
			}
			raw, err := eng.mux.CallRPC(ctx, "ze-plugin-callback:config-verify", rpc.ConfigVerifyInput{Sections: sections})
			if err != nil {
				t.Fatalf("config-verify RPC: %v", err)
			}
			var status struct {
				Status string `json:"status"`
				Error  string `json:"error"`
			}
			if err := json.Unmarshal(raw, &status); err != nil {
				t.Fatalf("config-verify answer %s: %v", raw, err)
			}
			if !tc.refused {
				if status.Status != rpc.StatusOK {
					t.Fatalf("verify refused compatible partial configuration: %s", raw)
				}
				return
			}
			if status.Status != rpc.StatusError {
				t.Fatalf("verify accepted incompatible no-NET levels: %s", raw)
			}
			if !strings.Contains(status.Error, "eth0") || !strings.Contains(status.Error, "set isis interfaces interface eth0 level l1-l2") {
				t.Errorf("verify refusal lacks interface and corrective command: %s", status.Error)
			}
		})
	}
}

// A no-NET configuration is inactive at startup and must remain inactive on
// reload. The real reconcile/circuit lifecycle uses only the fake raw backend:
// partial -> valid NET -> NET removed while the interface stays enabled.
func TestISISInstanceLevelNoNETLifecycle(t *testing.T) {
	backend := &fakeBackend{}
	eng := newEngine(transport.New(backend))
	t.Cleanup(eng.shutdown)
	eng.setConfig(Config{})
	parse := func(data string) Config {
		t.Helper()
		cfg, err := parseISISConfig(sec(data))
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}
	partial := parse(`{"isis":{"level":"l2","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point"}}}}}`)
	active := parse(`{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l2","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point"}}}}}`)
	openCount := func() int {
		backend.mu.Lock()
		defer backend.mu.Unlock()
		return len(backend.opened)
	}
	assertCircuitCount := func(stage string, want int) {
		t.Helper()
		eng.mu.Lock()
		running := len(eng.running)
		eng.mu.Unlock()
		eng.circuitsMu.RLock()
		circuits := len(eng.circuits)
		named := len(eng.circuitByName)
		eng.circuitsMu.RUnlock()
		if running != want || circuits != want || named != want {
			t.Errorf("%s: running=%d indexed circuits=%d named circuits=%d, want %d each", stage, running, circuits, named, want)
		}
	}
	eng.reconcile(partial)
	assertCircuitCount("partial without NET", 0)
	if got := openCount(); got != 0 {
		t.Errorf("partial without NET opened %d backend circuits, want none", got)
	}

	eng.reconcile(active)
	assertCircuitCount("valid NET activation", 1)
	if got := openCount(); got != 1 {
		t.Errorf("valid activation has opened %d backend circuits, want exactly one", got)
	}
	eng.circuitsMu.RLock()
	c := eng.circuitByName["eth0"]
	eng.circuitsMu.RUnlock()
	if c == nil {
		t.Fatal("valid NET did not build the eth0 adjacency circuit")
	}
	if err := c.SendHello(adjacency.Level2); err != nil {
		t.Fatalf("valid activated circuit cannot send an L2 Hello: %v", err)
	}
	beforeRemoval := openCount()
	result := eng.reconcile(partial)
	assertCircuitCount("NET removed", 0)
	if got := openCount(); got != beforeRemoval {
		t.Errorf("NET removal opened replacement circuits: opens %d -> %d", beforeRemoval, got)
	}
	if len(result.closed) != 1 || result.closed[0] != "eth0" {
		t.Errorf("NET removal closed %v, want eth0", result.closed)
	}
	if len(result.opened) != 0 || len(result.rebuilt) != 0 {
		t.Errorf("NET removal activated circuits: opened=%v rebuilt=%v", result.opened, result.rebuilt)
	}
}

// The registered doctor check must expose the same incompatible-level refusal
// as startup and commit, while compatible and disabled interfaces remain clean.
func TestISISInstanceLevelDoctor(t *testing.T) {
	const wantCode = "doctor-isis-interface-level-mismatch"
	var check diagnostic.DoctorCheck
	for _, candidate := range diagnostic.DoctorChecks() {
		if candidate.Name == "isis-config-sanity" {
			check = candidate
		}
	}
	if check.Check == nil {
		t.Fatal("IS-IS config-sanity doctor check is not registered")
	}
	declared := false
	for _, code := range check.Codes {
		if code == wantCode {
			declared = true
		}
	}
	if !declared {
		t.Errorf("registered doctor check does not declare %s", wantCode)
	}
	if diagnostic.Lookup(wantCode) == nil {
		t.Errorf("diagnostic explanation is not registered for %s", wantCode)
	}
	for _, tc := range []struct {
		name            string
		instance        string
		interfaceConfig string
		mismatch        bool
	}{
		{"l1 default", "l1", "", false},
		{"l2 default", "l2", "", false},
		{"dual explicit l1", "l1-l2", "level l1", false},
		{"dual explicit l2", "l1-l2", "level l2", false},
		{"disabled mismatch", "l2", "level l1\n enabled false", false},
		{"l1 rejects l2", "l1", "level l2", true},
		{"l2 rejects l1", "l2", "level l1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conf := fmt.Sprintf("isis {\n net 49.0001.0000.0000.0001.00\n level %s\n interfaces {\n interface eth0 {\n %s\n }\n }\n }\n", tc.instance, tc.interfaceConfig)
			diags := check.Check(diagnostic.DoctorCheckContext{Tree: treeFrom(t, conf)})
			if !tc.mismatch {
				if len(diags) != 0 {
					t.Fatalf("compatible configuration produced diagnostics: %+v", diags)
				}
				return
			}
			if len(diags) != 1 || diags[0].Code != wantCode {
				t.Fatalf("doctor diagnostics = %+v, want one %s", diags, wantCode)
			}
			if diags[0].Severity != diagnostic.SeverityError {
				t.Errorf("level mismatch severity = %s, want error", diags[0].Severity)
			}
			if !strings.Contains(diags[0].Message, "eth0") || !strings.Contains(diags[0].Message, "set isis interfaces interface eth0 level l1-l2") {
				t.Errorf("doctor refusal lacks interface and corrective command: %s", diags[0].Message)
			}
		})
	}
}

// Missing identity must not hide an independent interface-level error from the
// registered doctor check. Compatible and disabled circuits still report only
// the missing NET, including a partial configuration with an explicit System ID.
func TestISISInstanceLevelDoctorPartialConfig(t *testing.T) {
	var check diagnostic.DoctorCheck
	for _, candidate := range diagnostic.DoctorChecks() {
		if candidate.Name == "isis-config-sanity" {
			check = candidate
		}
	}
	if check.Check == nil {
		t.Fatal("IS-IS config-sanity doctor check is not registered")
	}
	for _, tc := range []struct {
		name            string
		instance        string
		interfaceConfig string
		mismatch        bool
	}{
		{"l1 rejects l2", "l1", "level l2", true},
		{"l2 rejects l1", "l2", "level l1", true},
		{"l2 default", "l2", "", false},
		{"disabled mismatch", "l2", "level l1\n enabled false", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conf := fmt.Sprintf("isis {\n system-id 0000.0000.0001\n level %s\n interfaces {\n interface eth0 {\n %s\n }\n }\n }\n", tc.instance, tc.interfaceConfig)
			diags := check.Check(diagnostic.DoctorCheckContext{Tree: treeFrom(t, conf)})
			wantCount := 1
			if tc.mismatch {
				wantCount = 2
			}
			if len(diags) != wantCount {
				t.Fatalf("partial configuration diagnostics = %+v, want %d", diags, wantCount)
			}
			codes := make(map[string]bool, len(diags))
			for _, diag := range diags {
				if diag.Severity != diagnostic.SeverityError {
					t.Errorf("%s severity = %s, want error", diag.Code, diag.Severity)
				}
				codes[diag.Code] = true
				if diag.Code == "doctor-isis-interface-level-mismatch" {
					if diag.Fix != "set isis interfaces interface eth0 level l1-l2" {
						t.Errorf("partial-config level refusal fix = %q, want corrective command", diag.Fix)
					}
					if !strings.Contains(diag.Message, "eth0") || !strings.Contains(diag.Message, "set isis interfaces interface eth0 level l1-l2") {
						t.Errorf("partial-config level refusal lacks interface and corrective command: %s", diag.Message)
					}
				}
			}
			if !codes["doctor-isis-net-missing"] {
				t.Error("partial configuration did not report its missing NET")
			}
			if codes["doctor-isis-interface-level-mismatch"] != tc.mismatch {
				t.Errorf("level mismatch reported = %t, want %t", codes["doctor-isis-interface-level-mismatch"], tc.mismatch)
			}
		})
	}
}

// configureISISStartupLevels delivers config through the real Stage 2 callback.
func configureISISStartupLevels(t *testing.T, sections []rpc.ConfigSection) error {
	t.Helper()
	eng, ctx := startISISLevelPlugin(t)
	_, err := eng.mux.CallRPC(ctx, "ze-plugin-callback:configure", rpc.ConfigureInput{Sections: sections})
	return err
}

// startISISLevelPlugin owns the plugin and engine RPC workers until test
// cleanup closes their connection, cancels their context and joins both workers.
func startISISLevelPlugin(t *testing.T) (*isisRollbackEngine, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	previousClient := routeInstallClient()
	previousExport := isisredistribute.ExportChain()
	pluginEnd, engineEnd := net.Pipe()
	eng := &isisRollbackEngine{mux: rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)), seen: make(chan string, 64)}
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		eng.serve(ctx)
	}()
	pluginDone := make(chan struct{})
	go func() {
		defer close(pluginDone)
		runISISEngine(pluginEnd)
	}()
	t.Cleanup(func() {
		cancel()
		if err := eng.mux.Close(); err != nil {
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
				t.Error("IS-IS startup RPC worker did not exit after its connection closed")
			}
		}
		routeInstallPtr.Store(previousClient)
		isisredistribute.SetExportChain(previousExport)
	})
	eng.waitFor(t, ctx, rpc.MethodDeclareRegistration)
	return eng, ctx
}

// A parsed L2 instance with no interface override must negotiate L2, advertise
// its actual Up neighbor in the L2 LSP, and resolve a peer prefix through SPF.
func TestISISInstanceL2DefaultCircuitProducesReachability(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l2","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point","metric":"15"},"lo":{"passive":"true"}}}}}`)
	defer eng.shutdown()
	eng.circuitsMu.RLock()
	c := eng.circuitByName["eth0"]
	eng.circuitsMu.RUnlock()
	if c == nil {
		t.Fatal("eth0 circuit not built")
	}
	neighbor := types.SystemID{0, 0, 0, 0, 0, 2}
	tr := c.Receive(adjacency.SNPA{}, p2pHelloPDU(t, neighbor, eng.cfg.NETs[0].AreaID()))
	if !tr.SessionUp {
		t.Fatalf("legacy point-to-point adjacency did not reach Up: %+v", tr)
	}
	rows := c.Table().Snapshot()
	if len(rows) != 1 || rows[0].Level != "l2" || rows[0].State != "up" {
		t.Errorf("adjacency = %+v, want one Up L2 neighbor", rows)
	}
	entry := eng.lsdb.Lookup(lsdb.Level2, types.NewLSPID(types.NewSourceID(eng.cfg.SystemID, 0), 0))
	if entry == nil {
		t.Fatal("own L2 LSP missing")
	}
	lsp, err := entry.Decode()
	if err != nil {
		t.Fatal(err)
	}
	defer packet.ReleaseTLVs(lsp.TLVs)
	found := false
	for _, tlv := range lsp.TLVs {
		if tlv.Type != packet.TLVExtendedISReach {
			continue
		}
		reach, err := packet.DecodeExtendedISReachTLV(tlv.Value)
		if err != nil {
			t.Fatal(err)
		}
		for _, link := range reach.Entries {
			if link.Neighbor == types.NewSourceID(neighbor, 0) && link.Metric.Value() == 15 {
				found = true
			}
		}
	}
	if !found {
		t.Error("own L2 LSP does not advertise the Up neighbor at circuit cost 15")
	}
	for _, value := range eng.interfaceSnapshot() {
		row := value.(interfaceRow)
		if row.Level != "l2" {
			t.Errorf("interface %s reports level %s, want effective l2", row.Name, row.Level)
		}
	}
	metricReceive(t, eng, metricAdvertisement{level: lsdb.Level2, id: 2, cost: 15, metric: 7}, 1, true)
	graph := spf.BuildGraph((*lsdbSPFSource)(eng), spf.Level2)
	result := spf.Compute(graph, eng.cfg.SystemID, spf.Level2)
	routes := spf.BuildRoutes([]*spf.Result{result}, map[spf.Level]*spf.Graph{spf.Level2: graph}, (*engineNextHopResolver)(eng))
	for _, route := range routes {
		if route.Prefix != metricPrefix {
			continue
		}
		if route.Level != spf.Level2 || route.Metric != 22 || len(route.NextHops) != 1 {
			t.Fatalf("peer route = %+v, want L2 metric 22 with one next hop", route)
		}
		hop := route.NextHops[0]
		if hop.Addr != netip.MustParseAddr("192.0.2.2") || hop.Interface != "eth0" {
			t.Errorf("next hop = %+v, want 192.0.2.2 through eth0", hop)
		}
		return
	}
	t.Fatalf("peer prefix %s missing from L2 SPF routes: %+v", metricPrefix, routes)
}

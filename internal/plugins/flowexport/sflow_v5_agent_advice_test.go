// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- sFlow agent address advice
// Related: export_test.go -- RunEngine and SwapLogger, the engine entry this test drives
// Related: sflow/agent_address.go -- adviseSFlowCollectors, the advisor the engine runs
// Related: register.go -- runEngine, whose configure step logs each advice at WARN
//
// VALIDATES: sFlow v5 Section 4.3 SHOULD ("In the case of a multi-homed
// agent, this should be the loopback address of the agent.") reaches the
// operator: the flow-export engine, configured over the real SDK startup with
// an sflow collector, logs one WARN naming the routed interface when a
// multi-homed router holds the agent address there, and logs none when a
// loopback or dummy interface holds it. The config is applied either way.
// PREVENTS: the classifier being right while the engine never asks it, or
// never shows its answer: a dropped RegisterCollectorsAdvisor("sflow", ...),
// a deleted WARN loop in runEngine, or an advisor that ignores the interface
// list it read.
//
// The test package is external so it can link the sflow encoder, which
// imports flowexport to register its advisor.

package flowexport_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/flowexport"
	_ "github.com/ze-software/ze/internal/plugins/flowexport/sflow"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// agentAdviceBackendName is the iface backend this test registers once per
// test binary, so a second -count pass reuses it rather than failing.
const agentAdviceBackendName = "flowexport-sflow-agent-advice-test"

// agentAdviceBackend answers a fixed multi-homed router: the kernel loopback,
// one dummy interface used as a loopback, and two routed interfaces. The
// backend methods the engine does not call are absent.
type agentAdviceBackend struct{ iface.Backend }

func agentAdviceInterface(name, kind string, addresses ...string) iface.InterfaceInfo {
	info := iface.InterfaceInfo{Name: name, OsName: name, Type: kind, State: "up"}
	for _, a := range addresses {
		family := "ipv4"
		if strings.Contains(a, ":") {
			family = "ipv6"
		}
		info.Addresses = append(info.Addresses, iface.AddrInfo{Address: a, Family: family})
	}
	return info
}

func (*agentAdviceBackend) ListInterfaces() ([]iface.InterfaceInfo, error) {
	return []iface.InterfaceInfo{
		agentAdviceInterface("lo", "device", "127.0.0.1", "::1", "198.51.100.1"),
		agentAdviceInterface("agent0", "dummy", "198.51.100.2"),
		agentAdviceInterface("eth0", "device", "203.0.113.1", "fe80::1"),
		agentAdviceInterface("eth1", "device", "192.0.2.5", "fe80::2"),
	}, nil
}

func (*agentAdviceBackend) LinkSpeedDuplex(string) (int, string) { return 0, "" }

func (*agentAdviceBackend) Close() error { return nil }

var registerAgentAdviceBackend = sync.OnceValue(func() error {
	return iface.RegisterBackend(agentAdviceBackendName, func() (iface.Backend, error) {
		return &agentAdviceBackend{}, nil
	})
})

// installAgentAdviceBackend makes the multi-homed router the interface list
// every iface.ListInterfaces caller reads, and restores the previous backend.
// Callers MUST NOT run in parallel.
func installAgentAdviceBackend(t *testing.T) {
	t.Helper()
	if err := registerAgentAdviceBackend(); err != nil {
		t.Fatal(err)
	}
	previous := iface.ActiveBackendName()
	if err := iface.LoadBackend(agentAdviceBackendName); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := iface.CloseBackend(); err != nil {
			t.Error(err)
		}
		if previous != "" {
			if err := iface.LoadBackend(previous); err != nil {
				t.Error(err)
			}
		}
	})
}

// loggedRecord is one line the engine logged: its level and its message.
type loggedRecord struct {
	level   slog.Level
	message string
}

// recordingHandler keeps every record the plugin logger receives.
// Safe for concurrent use: the engine logs from the SDK's goroutines.
type recordingHandler struct {
	mu      sync.Mutex
	records []loggedRecord
}

func (*recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, loggedRecord{level: r.Level, message: r.Message})
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *recordingHandler) WithGroup(string) slog.Handler { return h }

func (h *recordingHandler) snapshot() []loggedRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]loggedRecord(nil), h.records...)
}

// adviceEngineHost is the engine end of the plugin's SDK connection. It
// answers the startup calls the plugin makes before and after configure.
type adviceEngineHost struct {
	ctx  context.Context
	mux  *rpc.MuxConn
	seen chan string
}

func (*adviceEngineHost) answer(method string) any {
	if method == rpc.MethodDeclareRegistration {
		return rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}
	}
	return nil
}

func (h *adviceEngineHost) serve() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case req, ok := <-h.mux.Requests():
			if !ok {
				return
			}
			// A failed answer means the transport closed: the test is over.
			if err := h.mux.SendResult(h.ctx, req.ID, h.answer(req.Method)); err != nil {
				return
			}
			select {
			case h.seen <- req.Method:
			case <-h.ctx.Done():
				return
			}
		}
	}
}

func (h *adviceEngineHost) waitFor(t *testing.T, method string) {
	t.Helper()
	for {
		select {
		case got := <-h.seen:
			if got == method {
				return
			}
		case <-h.ctx.Done():
			t.Fatalf("waiting for %s: %v", method, h.ctx.Err())
		}
	}
}

// configureSFlowAgent starts the flow-export engine as an internal plugin
// over the real SDK startup, delivers one sflow collector whose
// agent-address is agent, and answers what the engine logged by the time the
// configure callback returned. The configure callback failing fails the test,
// so every answer comes from a config the engine applied.
func configureSFlowAgent(t *testing.T, agent string) []loggedRecord {
	t.Helper()
	installAgentAdviceBackend(t)
	logs := &recordingHandler{}
	previous := flowexport.SwapLogger(slog.New(logs))
	t.Cleanup(func() { flowexport.SwapLogger(previous) })
	_, port := udpCollector(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	pluginEnd, engineEnd := net.Pipe()
	h := &adviceEngineHost{ctx: ctx, mux: rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)), seen: make(chan string, 64)}
	bridge := rpc.NewDirectBridge()
	bridge.SetDispatchRPC(func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
		return json.Marshal(h.answer(method))
	})
	go h.serve()
	done := make(chan int, 1)
	go func() { done <- flowexport.RunEngine(rpc.NewBridgedConn(pluginEnd, bridge)) }()
	t.Cleanup(func() {
		cancel()
		bridge.CloseCallbacks()
		_ = h.mux.Close()
		_ = pluginEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("flow-export did not stop after its transport closed")
		}
	})

	h.waitFor(t, rpc.MethodDeclareRegistration)
	data := `{"flow-export":{"collector":{"c1":{"address":"127.0.0.1","port":` + strconv.Itoa(port) +
		`,"protocol":"sflow","agent-address":"` + agent + `"}}}}`
	in := rpc.ConfigureInput{Sections: []rpc.ConfigSection{{Root: "flow-export", Data: data}}}
	if _, err := h.mux.CallRPC(ctx, "ze-plugin-callback:configure", in); err != nil {
		t.Fatalf("configure with agent-address %s: %v", agent, err)
	}
	records := logs.snapshot()
	if !hasRecord(records, slog.LevelInfo, "flow-export configured") {
		t.Fatalf("the engine did not apply the config; it logged %v", records)
	}
	return records
}

func hasRecord(records []loggedRecord, level slog.Level, message string) bool {
	for _, r := range records {
		if r.level == level && r.message == message {
			return true
		}
	}
	return false
}

// MUTATION: replacing `log.Warn("flow-export: " + warning)` in runEngine's
// configure (register.go) with `_ = warning` turns this test red, and every
// test under sflow/ stays green. Deleting the
// RegisterCollectorsAdvisor("sflow", ...) line in sflow/register.go does too.
//
// RFC requirement: SFLOW-V5-4.3-2 positive -- the flow-export engine, configured through its SDK configure callback on a router with two routed interfaces, applies an sflow agent-address held by eth0 and logs exactly one WARN, which names that address, the routed interface eth0 and the dummy-interface command that moves it to a loopback.
func TestSFlowV5AgentAddressMultiHomedEngineWarns(t *testing.T) {
	// Goal: the operator of a multi-homed router is told, in the log, that
	// the agent address is not a loopback address.
	// Method: run the engine with the routed eth0 address as agent-address
	// and read every record it logged at WARN.
	records := configureSFlowAgent(t, "203.0.113.1")
	var warnings []string
	for _, r := range records {
		if r.level >= slog.LevelWarn {
			warnings = append(warnings, r.message)
		}
	}
	if len(warnings) != 1 {
		t.Fatalf("WARN records = %q, want exactly one agent-address warning", warnings)
	}
	for _, part := range []string{
		"flow-export: sflow agent-address 203.0.113.1 is on routed interface eth0, not on a loopback interface.",
		"more than one routed interface",
		"create interface dummy name agent0 address 203.0.113.1/32",
	} {
		if !strings.Contains(warnings[0], part) {
			t.Errorf("warning %q does not carry %q", warnings[0], part)
		}
	}
}

// MUTATION: replacing `classifyAgentAddress(agent, ifs)` in
// adviseSFlowCollectors (sflow/agent_address.go) with
// `classifyAgentAddress(agent, ifs[:0])`, so the advisor ignores the
// interface list it read, turns this test red, and every test under sflow/
// stays green: they call classifyAgentAddress and agentAddressAdvice, never
// the advisor.
//
// RFC requirement: SFLOW-V5-4.3-2 negative -- the flow-export engine, configured through its SDK configure callback on the same multi-homed router, applies an sflow agent-address held by the kernel loopback lo or by the dummy interface agent0 and logs no record at WARN or above.
func TestSFlowV5AgentAddressOnLoopbackEngineSilent(t *testing.T) {
	// Goal: an agent address that already meets the SHOULD draws nothing.
	// Method: run the engine once per loopback-held address and read every
	// record it logged at WARN or above.
	tests := []struct {
		name  string
		agent string
	}{
		{"kernel loopback lo", "198.51.100.1"},
		{"dummy agent0", "198.51.100.2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, r := range configureSFlowAgent(t, tt.agent) {
				if r.level >= slog.LevelWarn {
					t.Errorf("agent-address %s on a loopback logged %s %q", tt.agent, r.level, r.message)
				}
			}
		})
	}
}

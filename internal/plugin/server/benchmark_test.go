package server

import (
	"bytes"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
)

// Baseline benchmarks for IPC performance.
// These capture pre-YANG-IPC metrics referenced by all subsequent IPC specs.
//
// VALIDATES: Dispatch, event writing, and connection overhead are measurable.
// PREVENTS: Performance regressions from IPC protocol changes going undetected.

// Sink variables prevent compiler from eliminating benchmark work.
var (
	benchResp *plugin.Response
	benchCmd  *Command
	benchErr  error
)

// BenchmarkDispatch measures command dispatch throughput.
// This is the hot path: tokenize input → longest-prefix match → handler execution.
func BenchmarkDispatch(b *testing.B) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address: netip.MustParseAddr("10.0.0.1"),
				PeerAS:  65001,
				LocalAS: 65000,
				State:   "established",
				Uptime:  5 * time.Minute,
			},
			{
				Address: netip.MustParseAddr("10.0.0.2"),
				PeerAS:  65002,
				LocalAS: 65000,
				State:   "established",
				Uptime:  10 * time.Minute,
			},
		},
		stats: plugin.ReactorStats{
			StartTime: time.Now().Add(-1 * time.Hour),
			Uptime:    1 * time.Hour,
			PeerCount: 2,
		},
	}

	ctx := &CommandContext{
		Server: &Server{reactor: reactor, dispatcher: d},
	}

	commands := []struct {
		name string
		cmd  string
	}{
		{"peer_list", "bgp peer list"},
		{"daemon_status", "daemon status"},
		{"system_version", "system version software"},
		{"system_command_list", "system command list"},
		{"peer_show_selector", "bgp peer 10.0.0.1 show"},
		{"rib_help", "rib help"},
	}

	for _, tc := range commands {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				benchResp, benchErr = d.Dispatch(ctx, tc.cmd)
			}
		})
	}
}

// BenchmarkDispatchLookup measures raw command lookup without handler execution.
// Isolates the longest-prefix matching cost from handler processing.
func BenchmarkDispatchLookup(b *testing.B) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	commands := []string{
		"bgp peer list",
		"daemon status",
		"system version software",
		"system command list",
		"rib help",
	}

	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		cmd := commands[i%len(commands)]
		benchCmd = d.Lookup(cmd)
	}
}

// BenchmarkTokenize measures command tokenization throughput.
// Tokenization happens on every command before dispatch.
func BenchmarkTokenize(b *testing.B) {
	inputs := []struct {
		name  string
		input string
	}{
		{"simple", "bgp peer list"},
		{"with_selector", "bgp peer 10.0.0.1 show"},
		{"quoted", `bgp peer 10.0.0.1 update text origin set igp nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24`},
		{"long_update", `update text origin set igp as-path set [ 65001 65002 65003 ] nhop set 192.168.1.1 nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24 10.0.2.0/24`},
	}

	for _, tc := range inputs {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = tokenize(tc.input)
			}
		})
	}
}

// BenchmarkEventThroughput measures event writing throughput.
// Simulates the engine writing JSON events to a process's stdin pipe.
// Uses bytes.Buffer as a stand-in for the pipe writer.
func BenchmarkEventThroughput(b *testing.B) {
	events := []struct {
		name  string
		event string
	}{
		{"keepalive", `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1","asn":65001},"message":{"id":42,"direction":"received","type":"keepalive"}}}`},
		{"update_small", `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1","asn":65001},"message":{"id":43,"direction":"received","type":"update"},"update":{"attr":{"origin":"igp","as-path":[65001],"next-hop":"10.0.0.1"},"ipv4/unicast":[{"action":"add","next-hop":"10.0.0.1","nlri":["10.0.0.0/24"]}]}}}`},
		{"update_large", generateLargeUpdateEvent(100)},
	}

	for _, tc := range events {
		b.Run(tc.name, func(b *testing.B) {
			var buf bytes.Buffer
			buf.Grow(len(tc.event) * 100)
			eventBytes := []byte(tc.event + "\n")

			b.ReportAllocs()
			b.SetBytes(int64(len(eventBytes)))
			b.ResetTimer()

			for range b.N {
				buf.Write(eventBytes) //nolint:errcheck // bytes.Buffer.Write never returns error
				if buf.Len() > 1<<20 {
					buf.Reset()
				}
			}
		})
	}
}

// BenchmarkPluginStartup measures the cost of setting up a dispatcher with all builtins.
// This represents the per-connection overhead during plugin/API initialization.
func BenchmarkPluginStartup(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		d := NewDispatcher()
		RegisterDefaultHandlers(d)
		benchCmd = d.Lookup("bgp peer list")
	}
}

// BenchmarkConnect measures the overhead of creating a CommandContext.
// Real connection benchmarks require the full server; this measures
// the minimal per-session setup cost.
func BenchmarkConnect(b *testing.B) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	reactor := &mockReactor{
		stats: plugin.ReactorStats{
			StartTime: time.Now(),
			Uptime:    0,
			PeerCount: 0,
		},
	}

	b.ReportAllocs()

	for b.Loop() {
		ctx := &CommandContext{
			Server: &Server{reactor: reactor, dispatcher: d},
			Peer:   "*",
		}
		// Simulate initial command exchange
		benchResp, benchErr = d.Dispatch(ctx, "system version software")
	}
}

// BenchmarkMemoryPerConnection measures allocation overhead per session.
// Reports bytes allocated for creating a new dispatcher + context.
func BenchmarkMemoryPerConnection(b *testing.B) {
	reactor := &mockReactor{}

	b.ReportAllocs()

	for b.Loop() {
		d := NewDispatcher()
		RegisterDefaultHandlers(d)
		ctx := &CommandContext{
			Server: &Server{reactor: reactor, dispatcher: d},
			Peer:   "*",
		}
		benchResp, benchErr = d.Dispatch(ctx, "bgp peer list")
	}
}

// generateLargeUpdateEvent creates a synthetic UPDATE event with N NLRIs.
func generateLargeUpdateEvent(nlriCount int) string {
	var buf bytes.Buffer
	buf.WriteString(`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1","asn":65001},"message":{"id":100,"direction":"received","type":"update"},"update":{"attr":{"origin":"igp","as-path":[65001,65002,65003],"next-hop":"10.0.0.1","local-preference":100},"ipv4/unicast":[{"action":"add","next-hop":"10.0.0.1","nlri":[`)
	for i := range nlriCount {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, `"10.%d.%d.0/24"`, i/256, i%256)
	}
	buf.WriteString(`]}]}}}`)
	return buf.String()
}

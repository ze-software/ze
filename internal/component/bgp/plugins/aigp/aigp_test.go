// VALIDATES: the AIGP stub plugin's real surface — the attr-26 JSON formatter,
// its registration side effects (JSON formatter + plugin registry entry), the
// logger nil-guard, and that RunAIGPPlugin actually drives the SDK loop and
// returns (rather than hanging) on a dead connection.
// PREVENTS: the AIGP attribute silently losing its JSON rendering or registry
// entry, and a plugin entry point that hangs instead of exiting on a closed conn.
//
// SCOPE: aigp is an explicit stub (aigp.go:7-8). These tests assert only what
// exists; no RFC 7311 AIGP *semantics* are invented.
package aigp

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func TestAppendAIGPJSONMetric(t *testing.T) {
	got := string(appendAIGPJSON(nil, attribute.NewAIGPMetric(42)))
	if got != "42" {
		t.Errorf("appendAIGPJSON(metric=42) = %q, want %q", got, "42")
	}
}

func TestAppendAIGPJSONNoMetric(t *testing.T) {
	// An AIGP carrying only an unknown TLV has no metric → formatter yields nil
	// so the caller falls through to hex.
	a := &attribute.AIGP{TLVs: []attribute.AIGPTLV{{Type: 99, Data: []byte{1, 2, 3}}}}
	if got := appendAIGPJSON(nil, a); got != nil {
		t.Errorf("appendAIGPJSON(no metric) = %q, want nil", got)
	}
}

func TestAppendAIGPJSONNonAIGPReturnsNil(t *testing.T) {
	// Defensive type-guard: a non-AIGP attribute must not be formatted.
	var notAIGP attribute.Attribute
	if got := appendAIGPJSON(nil, notAIGP); got != nil {
		t.Errorf("appendAIGPJSON(non-AIGP) = %q, want nil", got)
	}
}

func TestAIGPJSONFormatterRegistered(t *testing.T) {
	f := attribute.GetJSONFormatter(attribute.AttrAIGP)
	if f == nil {
		t.Fatal("no JSON formatter registered for AttrAIGP")
	}
	if f.Key != "aigp" {
		t.Errorf("formatter key = %q, want %q", f.Key, "aigp")
	}
	if got := string(f.AppendValue(nil, attribute.NewAIGPMetric(7))); got != "7" {
		t.Errorf("registered formatter output = %q, want %q", got, "7")
	}
}

func TestAIGPPluginRegistered(t *testing.T) {
	reg := registry.Lookup("bgp-aigp")
	if reg == nil {
		t.Fatal("bgp-aigp not registered in the plugin registry")
	}
	if reg.RunEngine == nil {
		t.Error("bgp-aigp registration has no RunEngine entry point")
	}
	found := false
	for _, r := range reg.RFCs {
		if r == "7311" {
			found = true
		}
	}
	if !found {
		t.Errorf("bgp-aigp RFCs = %v, want to include 7311", reg.RFCs)
	}
}

func TestSetAIGPLoggerNilGuard(t *testing.T) {
	orig := logger()
	t.Cleanup(func() { loggerPtr.Store(orig) })

	custom := slogutil.DiscardLogger()
	SetAIGPLogger(custom)
	if logger() != custom {
		t.Fatal("SetAIGPLogger did not store the provided logger")
	}
	// A nil logger must be ignored, not stored.
	SetAIGPLogger(nil)
	if logger() != custom {
		t.Error("SetAIGPLogger(nil) overwrote the logger")
	}
}

// TestRunAIGPPluginClosedConnReturns proves RunAIGPPlugin wires and drives the
// SDK loop: handed a dead connection it exits promptly with a non-zero code
// instead of hanging.
func TestRunAIGPPluginClosedConnReturns(t *testing.T) {
	client, server := net.Pipe()
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	if err := server.Close(); err != nil {
		t.Fatalf("close server: %v", err)
	}

	done := make(chan int, 1)
	go func() { done <- RunAIGPPlugin(server) }()

	select {
	case code := <-done:
		if code == 0 {
			t.Error("RunAIGPPlugin on a closed conn returned 0, want non-zero")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunAIGPPlugin did not return on a closed conn (hang)")
	}
}

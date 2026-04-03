package env

import (
	"os"
	"testing"
)

func init() {
	// Register test keys for port helper tests.
	MustRegister(EnvEntry{Key: "ze.test.port.int", Type: "int", Default: "1850", Description: "test port"})
	MustRegister(EnvEntry{Key: "ze.test.port.disabled", Type: "int", Default: "", Description: "test disabled port"})
	MustRegister(EnvEntry{Key: "ze.test.port.addr", Type: "string", Default: ":8080", Description: "test addr port"})
	MustRegister(EnvEntry{Key: "ze.test.port.addr.empty", Type: "string", Default: "", Description: "test disabled addr port"})
}

// VALIDATES: AC-1 -- PortDefault returns default value and description includes env var name
// PREVENTS: help text missing env var hint

func TestPortDefault_NoEnv(t *testing.T) {
	unsetAll(t, "ze.test.port.int")

	val, desc := PortDefault("ze.test.port.int", 1850, "Base BGP port")
	if val != 1850 {
		t.Errorf("PortDefault value: got %d, want 1850", val)
	}
	if !containsAll(desc, "Base BGP port", "default: 1850", "env: ze.test.port.int") {
		t.Errorf("PortDefault desc: got %q, missing expected parts", desc)
	}
}

// VALIDATES: AC-2 -- PortDefault shows configured value when env is set
// PREVENTS: env override not visible in help text

func TestPortDefault_EnvOverride(t *testing.T) {
	unsetAll(t, "ze.test.port.int")
	if err := os.Setenv("ze.test.port.int", "1900"); err != nil {
		t.Fatal(err)
	}
	ResetCache()

	val, desc := PortDefault("ze.test.port.int", 1850, "Base BGP port")
	if val != 1900 {
		t.Errorf("PortDefault value: got %d, want 1900", val)
	}
	if !containsAll(desc, "Base BGP port", "default: 1850", "configured: 1900", "ze.test.port.int") {
		t.Errorf("PortDefault desc: got %q, missing expected parts", desc)
	}
}

// VALIDATES: PortDefault with disabled port (default 0) shows "(disabled)"
// PREVENTS: confusing help text for optional ports

func TestPortDefault_Disabled(t *testing.T) {
	unsetAll(t, "ze.test.port.disabled")

	val, desc := PortDefault("ze.test.port.disabled", 0, "Optional SSH port")
	if val != 0 {
		t.Errorf("PortDefault value: got %d, want 0", val)
	}
	if !containsAll(desc, "Optional SSH port", "disabled", "env: ze.test.port.disabled") {
		t.Errorf("PortDefault desc: got %q, missing expected parts", desc)
	}
}

// VALIDATES: AC-1 -- AddrPortDefault returns default value and description includes env var name
// PREVENTS: addr:port flags missing env var hint

func TestAddrPortDefault_NoEnv(t *testing.T) {
	unsetAll(t, "ze.test.port.addr")

	val, desc := AddrPortDefault("ze.test.port.addr", ":8080", "Web dashboard")
	if val != ":8080" {
		t.Errorf("AddrPortDefault value: got %q, want %q", val, ":8080")
	}
	if !containsAll(desc, "Web dashboard", "default: :8080", "env: ze.test.port.addr") {
		t.Errorf("AddrPortDefault desc: got %q, missing expected parts", desc)
	}
}

// VALIDATES: AC-2 -- AddrPortDefault shows configured value when env is set
// PREVENTS: env override not visible in help text for addr:port flags

func TestAddrPortDefault_EnvOverride(t *testing.T) {
	unsetAll(t, "ze.test.port.addr")
	if err := os.Setenv("ze.test.port.addr", ":9090"); err != nil {
		t.Fatal(err)
	}
	ResetCache()

	val, desc := AddrPortDefault("ze.test.port.addr", ":8080", "Web dashboard")
	if val != ":9090" {
		t.Errorf("AddrPortDefault value: got %q, want %q", val, ":9090")
	}
	if !containsAll(desc, "Web dashboard", "default: :8080", "configured: :9090", "ze.test.port.addr") {
		t.Errorf("AddrPortDefault desc: got %q, missing expected parts", desc)
	}
}

// VALIDATES: AddrPortDefault with empty default shows "(disabled)"
// PREVENTS: confusing help text for optional addr:port flags

func TestAddrPortDefault_Disabled(t *testing.T) {
	unsetAll(t, "ze.test.port.addr.empty")

	val, desc := AddrPortDefault("ze.test.port.addr.empty", "", "Optional endpoint")
	if val != "" {
		t.Errorf("AddrPortDefault value: got %q, want %q", val, "")
	}
	if !containsAll(desc, "Optional endpoint", "disabled", "env: ze.test.port.addr.empty") {
		t.Errorf("AddrPortDefault desc: got %q, missing expected parts", desc)
	}
}

// containsAll returns true if s contains all substrings.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

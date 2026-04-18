//nolint:goconst // Test values intentionally repeated for clarity
package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultSocketPathFallback verifies /tmp/ze.socket when XDG is unset and
// the test process is non-root.
//
// VALIDATES: DefaultSocketPath returns /tmp/ze.socket as last-resort fallback.
func TestDefaultSocketPathFallback(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")

	if os.Getuid() == 0 {
		t.Skip("root fallback returns /var/run/ze.socket; skip on root")
	}

	if got := DefaultSocketPath(); got != "/tmp/ze.socket" {
		t.Errorf("DefaultSocketPath() = %q, want /tmp/ze.socket", got)
	}
}

// TestDefaultSocketPathXDG verifies XDG_RUNTIME_DIR is honored.
//
// VALIDATES: DefaultSocketPath picks XDG runtime dir over fallback.
func TestDefaultSocketPathXDG(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")

	if got := DefaultSocketPath(); got != "/run/user/1000/ze.socket" {
		t.Errorf("DefaultSocketPath() = %q, want /run/user/1000/ze.socket", got)
	}
}

// TestResolveConfigPathTraversal verifies path traversal prevention in XDG
// config search.
//
// VALIDATES: Config names with ../ do not escape the config directory.
// PREVENTS: Path traversal via crafted config filenames (CWE-22).
func TestResolveConfigPathTraversal(t *testing.T) {
	dir := t.TempDir()

	// Create a file outside the ze/ config dir
	outside := filepath.Join(dir, "secret.conf")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create the ze/ config dir (empty)
	zeDir := filepath.Join(dir, "ze")
	if err := os.MkdirAll(zeDir, 0o750); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", dir)
	// Clear XDG_CONFIG_DIRS to avoid matching real system files
	t.Setenv("XDG_CONFIG_DIRS", t.TempDir())

	// Attempt traversal — should NOT resolve to the secret file
	result := ResolveConfigPath("../secret.conf")
	if result == outside {
		t.Errorf("path traversal succeeded: resolved to %q", result)
	}
}

// TestParseCompoundListen verifies single-endpoint parsing.
//
// VALIDATES: ze.web.listen "0.0.0.0:3443" parsed into one endpoint.
func TestParseCompoundListen(t *testing.T) {
	endpoints, err := ParseCompoundListen("0.0.0.0:3443")
	if err != nil {
		t.Fatalf("ParseCompoundListen(\"0.0.0.0:3443\") error: %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("want 1 endpoint, got %d", len(endpoints))
	}
	if endpoints[0].IP != "0.0.0.0" {
		t.Errorf("IP = %q, want %q", endpoints[0].IP, "0.0.0.0")
	}
	if endpoints[0].Port != 3443 {
		t.Errorf("Port = %d, want %d", endpoints[0].Port, 3443)
	}
	if endpoints[0].String() != "0.0.0.0:3443" {
		t.Errorf("String() = %q, want %q", endpoints[0].String(), "0.0.0.0:3443")
	}
}

// TestParseCompoundListenIPv6 verifies IPv6 bracket notation parsing.
//
// VALIDATES: "[::1]:3443" parsed correctly.
// PREVENTS: IPv6 addresses failing to parse due to colons in address.
func TestParseCompoundListenIPv6(t *testing.T) {
	endpoints, err := ParseCompoundListen("[::1]:3443")
	if err != nil {
		t.Fatalf("ParseCompoundListen(\"[::1]:3443\") error: %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("want 1 endpoint, got %d", len(endpoints))
	}
	if endpoints[0].IP != "::1" {
		t.Errorf("IP = %q, want %q", endpoints[0].IP, "::1")
	}
	if endpoints[0].Port != 3443 {
		t.Errorf("Port = %d, want %d", endpoints[0].Port, 3443)
	}
	if endpoints[0].String() != "[::1]:3443" {
		t.Errorf("String() = %q, want %q", endpoints[0].String(), "[::1]:3443")
	}
}

// TestCompoundListenMulti verifies multi-endpoint parsing.
//
// VALIDATES: "0.0.0.0:3443,127.0.0.1:8080" parsed into two endpoints.
// PREVENTS: Compound format not supporting comma-separated endpoints.
func TestCompoundListenMulti(t *testing.T) {
	endpoints, err := ParseCompoundListen("0.0.0.0:3443,127.0.0.1:8080")
	if err != nil {
		t.Fatalf("ParseCompoundListen multi error: %v", err)
	}
	if len(endpoints) != 2 {
		t.Fatalf("want 2 endpoints, got %d", len(endpoints))
	}
	if endpoints[0].IP != "0.0.0.0" || endpoints[0].Port != 3443 {
		t.Errorf("endpoint[0] = %v, want 0.0.0.0:3443", endpoints[0])
	}
	if endpoints[1].IP != "127.0.0.1" || endpoints[1].Port != 8080 {
		t.Errorf("endpoint[1] = %v, want 127.0.0.1:8080", endpoints[1])
	}
}

// TestCompoundListenBoundary verifies port boundary validation.
//
// VALIDATES: Port range 1-65535 is enforced.
// PREVENTS: Port 0 or 65536 being silently accepted.
func TestCompoundListenBoundary(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"port_1_valid", "0.0.0.0:1", false},
		{"port_65535_valid", "0.0.0.0:65535", false},
		{"port_0_invalid", "0.0.0.0:0", true},
		{"port_65536_invalid", "0.0.0.0:65536", true},
		{"empty_string", "", true},
		{"missing_port", "0.0.0.0", true},
		{"missing_port_colon", "0.0.0.0:", true},
		{"port_not_number", "0.0.0.0:abc", true},
		{"negative_port", "0.0.0.0:-1", true},
		{"ipv6_port_0", "[::1]:0", true},
		{"ipv6_port_65536", "[::1]:65536", true},
		{"ipv6_no_bracket_close", "[::1:3443", true},
		{"spaces_around", " 0.0.0.0:3443 ", false},
		{"multi_with_invalid", "0.0.0.0:3443,0.0.0.0:0", true},
		{"ipv6_full_valid", "[2001:db8::1]:8443", false},
		{"only_port", ":3443", false},
		{"trailing_comma", "0.0.0.0:3443,", true},
		{"leading_comma", ",0.0.0.0:3443", true},
		{"ipv6_no_colon_after_bracket", "[::1]3443", true},
		{"empty_ipv6_brackets", "[]:3443", true},
		{"hostname_not_ip", "example.com:3443", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCompoundListen(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCompoundListen(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// TestCompoundListenValues verifies parsed values for valid inputs.
//
// VALIDATES: Parsed IP, port, and String() output for edge-case inputs.
// PREVENTS: Parser accepting input but producing wrong field values.
func TestCompoundListenValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantIP   string
		wantPort int
		wantStr  string
	}{
		{"only_port", ":3443", "", 3443, ":3443"},
		{"spaces_trimmed", " 0.0.0.0:3443 ", "0.0.0.0", 3443, "0.0.0.0:3443"},
		{"ipv6_full", "[2001:db8::1]:8443", "2001:db8::1", 8443, "[2001:db8::1]:8443"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints, err := ParseCompoundListen(tt.input)
			if err != nil {
				t.Fatalf("ParseCompoundListen(%q) unexpected error: %v", tt.input, err)
			}
			if len(endpoints) != 1 {
				t.Fatalf("want 1 endpoint, got %d", len(endpoints))
			}
			if endpoints[0].IP != tt.wantIP {
				t.Errorf("IP = %q, want %q", endpoints[0].IP, tt.wantIP)
			}
			if endpoints[0].Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", endpoints[0].Port, tt.wantPort)
			}
			if endpoints[0].String() != tt.wantStr {
				t.Errorf("String() = %q, want %q", endpoints[0].String(), tt.wantStr)
			}
		})
	}
}

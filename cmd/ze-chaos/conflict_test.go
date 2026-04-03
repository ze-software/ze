// Design: docs/architecture/chaos-web-dashboard.md -- tests for listener conflict detection

package main

import (
	"net"
	"strings"
	"testing"
)

// VALIDATES: AC-6 -- conflicting ports detected at startup
// PREVENTS: two services silently fighting for the same port

func TestChaosListenConflict_SamePort(t *testing.T) {
	// --web-ui 8443 and --lg 8443 both bind 127.0.0.1:8443
	err := validateChaosListenerConflicts(0, 8443, 8443, "", "", "", "")
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "web-ui") || !strings.Contains(err.Error(), "looking-glass") {
		t.Errorf("error should name both services, got: %v", err)
	}
}

// VALIDATES: AC-7 -- no conflict when ports differ
// PREVENTS: false positive conflict detection

func TestChaosListenConflict_NoConflict(t *testing.T) {
	err := validateChaosListenerConflicts(2222, 3443, 8443, ":8000", ":6060", ":9090", ":6061")
	if err != nil {
		t.Errorf("expected no conflict, got: %v", err)
	}
}

// VALIDATES: disabled flags (value 0 or "") are excluded from conflict check
// PREVENTS: false conflict from disabled services

func TestChaosListenConflict_DisabledExcluded(t *testing.T) {
	// SSH=0 and web-ui=0 are disabled, only lg=8443 is active -- no conflict possible
	err := validateChaosListenerConflicts(0, 0, 8443, "", "", "", "")
	if err != nil {
		t.Errorf("expected no conflict with disabled ports, got: %v", err)
	}
}

// VALIDATES: addr:port string flags conflict with int port flags
// PREVENTS: cross-type conflict going undetected

func TestChaosListenConflict_AddrVsInt(t *testing.T) {
	// --web :8443 (addr:port, wildcard) and --lg 8443 (int, localhost) conflict
	err := validateChaosListenerConflicts(0, 0, 8443, "", "", "", ":8443")
	if err == nil {
		t.Fatal("expected conflict between addr:port and int port, got nil")
	}
}

// VALIDATES: parseAddrPort handles ":port" format
// PREVENTS: bare :port format being rejected

func TestParseAddrPort_ColonPort(t *testing.T) {
	ep := parseAddrPort(":8080")
	if ep == nil {
		t.Fatal("parseAddrPort(:8080) returned nil")
	}
	if ep.port != 8080 {
		t.Errorf("port: got %d, want 8080", ep.port)
	}
}

// VALIDATES: parseAddrPort handles "host:port" format
// PREVENTS: explicit host being ignored

func TestParseAddrPort_HostPort(t *testing.T) {
	ep := parseAddrPort("127.0.0.1:6060")
	if ep == nil {
		t.Fatal("parseAddrPort(127.0.0.1:6060) returned nil")
	}
	if ep.port != 6060 {
		t.Errorf("port: got %d, want 6060", ep.port)
	}
	if !ep.ip.Equal(net.IPv4(127, 0, 0, 1)) {
		t.Errorf("ip: got %v, want 127.0.0.1", ep.ip)
	}
}

// VALIDATES: parseAddrPort rejects empty string
// PREVENTS: empty addr producing a phantom endpoint

func TestParseAddrPort_Empty(t *testing.T) {
	ep := parseAddrPort("")
	if ep != nil {
		t.Errorf("parseAddrPort('') should return nil, got %+v", ep)
	}
}

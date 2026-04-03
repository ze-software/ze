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

// VALIDATES: single-port listener inside BGP port range detected
// PREVENTS: --web-ui landing inside allocated BGP port range unnoticed

func TestRangeConflict_InsideBGPRange(t *testing.T) {
	// --port 1850, --peers 4 -> BGP range [1850, 1858)
	// --web-ui 1852 falls inside
	err := validateRangeConflicts(1850, 1950, 4, 0, 1852, 0, "", "", "", "")
	if err == nil {
		t.Fatal("expected range conflict, got nil")
	}
	if !strings.Contains(err.Error(), "web-ui") || !strings.Contains(err.Error(), "bgp port range") {
		t.Errorf("error should name service and range, got: %v", err)
	}
}

// VALIDATES: single-port listener inside listen-base range detected
// PREVENTS: --ssh landing inside allocated listen range unnoticed

func TestRangeConflict_InsideListenRange(t *testing.T) {
	// --listen-base 1950, --peers 4 -> listen range [1950, 1958)
	// --ssh 1952 falls inside
	err := validateRangeConflicts(1850, 1950, 4, 1952, 0, 0, "", "", "", "")
	if err == nil {
		t.Fatal("expected range conflict, got nil")
	}
	if !strings.Contains(err.Error(), "ssh") || !strings.Contains(err.Error(), "listen-base range") {
		t.Errorf("error should name service and range, got: %v", err)
	}
}

// VALIDATES: no false positive when ports are outside ranges
// PREVENTS: range check blocking valid configurations

func TestRangeConflict_NoConflict(t *testing.T) {
	// --port 1850, --listen-base 1950, --peers 4
	// ranges: [1850,1858) and [1950,1958)
	// --ssh 2222, --web-ui 3443, --lg 8443 all outside
	err := validateRangeConflicts(1850, 1950, 4, 2222, 3443, 8443, ":8000", ":6060", ":9090", ":6061")
	if err != nil {
		t.Errorf("expected no range conflict, got: %v", err)
	}
}

// VALIDATES: addr:port flag inside range detected
// PREVENTS: string-format ports escaping range check

func TestRangeConflict_AddrPortInsideRange(t *testing.T) {
	// --port 1850, --peers 4 -> [1850, 1858)
	// --web :1855 falls inside
	err := validateRangeConflicts(1850, 1950, 4, 0, 0, 0, ":1855", "", "", "")
	if err == nil {
		t.Fatal("expected range conflict for addr:port, got nil")
	}
}

// VALIDATES: range check works with non-default base ports
// PREVENTS: hardcoded assumptions about port 1850

func TestRangeConflict_CustomBase(t *testing.T) {
	// --port 2000, --peers 2 -> BGP range [2000, 2004)
	// --ssh 2002 falls inside
	err := validateRangeConflicts(2000, 3000, 2, 2002, 0, 0, "", "", "", "")
	if err == nil {
		t.Fatal("expected range conflict with custom base, got nil")
	}
}

// VALIDATES: disabled ports (0/"") skip range check
// PREVENTS: false conflict from disabled services

func TestRangeConflict_DisabledSkipped(t *testing.T) {
	// All single-port flags disabled -- no range conflict possible
	err := validateRangeConflicts(1850, 1950, 4, 0, 0, 0, "", "", "", "")
	if err != nil {
		t.Errorf("expected no conflict with disabled ports, got: %v", err)
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

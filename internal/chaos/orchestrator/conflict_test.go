// Design: docs/architecture/chaos-web-dashboard.md -- tests for listener conflict detection

package orchestrator

import (
	"net"
	"strings"
	"testing"
)

func TestChaosListenConflict_SamePort(t *testing.T) {
	err := ValidateChaosListenerConflicts(0, 8443, 8443, 0, "", "", "", "", "")
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "web-ui") || !strings.Contains(err.Error(), "looking-glass") {
		t.Errorf("error should name both services, got: %v", err)
	}
}

func TestChaosListenConflict_NoConflict(t *testing.T) {
	err := ValidateChaosListenerConflicts(2222, 3443, 8443, 0, ":8000", ":6060", ":9090", ":6061", "")
	if err != nil {
		t.Errorf("expected no conflict, got: %v", err)
	}
}

func TestChaosListenConflict_DisabledExcluded(t *testing.T) {
	err := ValidateChaosListenerConflicts(0, 0, 8443, 0, "", "", "", "", "")
	if err != nil {
		t.Errorf("expected no conflict with disabled ports, got: %v", err)
	}
}

func TestChaosListenConflict_AddrVsInt(t *testing.T) {
	err := ValidateChaosListenerConflicts(0, 0, 8443, 0, "", "", "", ":8443", "")
	if err == nil {
		t.Fatal("expected conflict between addr:port and int port, got nil")
	}
}

func TestRangeConflict_InsideBGPRange(t *testing.T) {
	err := ValidateRangeConflicts(1850, 1950, 4, 0, 1852, 0, 0, "", "", "", "", "")
	if err == nil {
		t.Fatal("expected range conflict, got nil")
	}
	if !strings.Contains(err.Error(), "web-ui") || !strings.Contains(err.Error(), "bgp port range") {
		t.Errorf("error should name service and range, got: %v", err)
	}
}

func TestRangeConflict_InsideListenRange(t *testing.T) {
	err := ValidateRangeConflicts(1850, 1950, 4, 1952, 0, 0, 0, "", "", "", "", "")
	if err == nil {
		t.Fatal("expected range conflict, got nil")
	}
	if !strings.Contains(err.Error(), "ssh") || !strings.Contains(err.Error(), "listen-base range") {
		t.Errorf("error should name service and range, got: %v", err)
	}
}

func TestRangeConflict_NoConflict(t *testing.T) {
	err := ValidateRangeConflicts(1850, 1950, 4, 2222, 3443, 8443, 0, ":8000", ":6060", ":9090", ":6061", "")
	if err != nil {
		t.Errorf("expected no range conflict, got: %v", err)
	}
}

func TestRangeConflict_AddrPortInsideRange(t *testing.T) {
	err := ValidateRangeConflicts(1850, 1950, 4, 0, 0, 0, 0, ":1855", "", "", "", "")
	if err == nil {
		t.Fatal("expected range conflict for addr:port, got nil")
	}
}

func TestRangeConflict_CustomBase(t *testing.T) {
	err := ValidateRangeConflicts(2000, 3000, 2, 2002, 0, 0, 0, "", "", "", "", "")
	if err == nil {
		t.Fatal("expected range conflict with custom base, got nil")
	}
}

func TestRangeConflict_DisabledSkipped(t *testing.T) {
	err := ValidateRangeConflicts(1850, 1950, 4, 0, 0, 0, 0, "", "", "", "", "")
	if err != nil {
		t.Errorf("expected no conflict with disabled ports, got: %v", err)
	}
}

func TestParseAddrPort_ColonPort(t *testing.T) {
	ep := parseAddrPort(":8080")
	if ep == nil {
		t.Fatal("parseAddrPort(:8080) returned nil")
	}
	if ep.port != 8080 {
		t.Errorf("port: got %d, want 8080", ep.port)
	}
}

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

func TestParseAddrPort_Empty(t *testing.T) {
	ep := parseAddrPort("")
	if ep != nil {
		t.Errorf("parseAddrPort('') should return nil, got %+v", ep)
	}
}

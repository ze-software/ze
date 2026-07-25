// Design: docs/architecture/chaos-web-dashboard.md -- tests for listener conflict detection

package orchestrator

import (
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/chaos/scenario"
)

func TestValidateConfigRangeConflicts_MetricsInsideBGP(t *testing.T) {
	cfg := &OrchestratorConfig{
		Profiles: []scenario.PeerProfile{
			{Index: 0, ZePort: 1790, Port: 1890},
			{Index: 1, ZePort: 1791, Port: 1891},
			{Index: 2, ZePort: 1792, Port: 1892},
		},
		MetricsAddr: "127.0.0.1:1791", // inside the derived bgp range [1790, 1796)
	}
	err := ValidateConfigRangeConflicts(cfg)
	if err == nil {
		t.Fatal("expected range conflict, got nil")
	}
	if !strings.Contains(err.Error(), "chaos-metrics") || !strings.Contains(err.Error(), "bgp port range") {
		t.Errorf("error should name the service and range, got: %v", err)
	}
}

func TestValidateConfigRangeConflicts_NoConflict(t *testing.T) {
	cfg := &OrchestratorConfig{
		Profiles: []scenario.PeerProfile{
			{Index: 0, ZePort: 1790, Port: 1890},
			{Index: 1, ZePort: 1791, Port: 1891},
		},
		MetricsAddr: ":9090",
		WebAddr:     ":8000",
	}
	if err := ValidateConfigRangeConflicts(cfg); err != nil {
		t.Errorf("expected no conflict, got: %v", err)
	}
}

func TestValidateConfigRangeConflicts_EmptyProfiles(t *testing.T) {
	// No profiles => no derivable range => nothing to validate.
	if err := ValidateConfigRangeConflicts(&OrchestratorConfig{MetricsAddr: "127.0.0.1:1790"}); err != nil {
		t.Errorf("empty profiles must not error, got: %v", err)
	}
}

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

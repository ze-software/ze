package copp

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
)

func TestCoppParseConfig(t *testing.T) {
	input := `{"control-plane-protection":{"bgp":{"rate":"100/second","burst":"20"}}}`
	policy, found, err := parseCoppConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected config to be found")
	}
	if policy.Rate != 100 {
		t.Errorf("Rate = %d, want 100", policy.Rate)
	}
	if policy.RateUnit != "second" {
		t.Errorf("RateUnit = %q, want second", policy.RateUnit)
	}
	if policy.Dimension != firewall.RateDimensionPackets {
		t.Errorf("Dimension = %d, want RateDimensionPackets", policy.Dimension)
	}
	if policy.Burst != 20 {
		t.Errorf("Burst = %d, want 20", policy.Burst)
	}
	if len(policy.ProtectedPorts) != 1 || policy.ProtectedPorts[0] != 179 {
		t.Errorf("ProtectedPorts = %v, want [179]", policy.ProtectedPorts)
	}
	if policy.OverPolicy != "accept" {
		t.Errorf("OverPolicy = %q, want accept", policy.OverPolicy)
	}
}

func TestCoppParseConfigNoBlock(t *testing.T) {
	input := `{"firewall":{}}`
	_, found, err := parseCoppConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected config not to be found")
	}
}

func TestCoppParseConfigTrustedSources(t *testing.T) {
	input := `{"control-plane-protection":{"bgp":{"rate":"50/second","trusted-source":["192.0.2.0/24","198.51.100.0/24"]}}}`
	policy, found, err := parseCoppConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected config to be found")
	}
	if len(policy.TrustedSources) != 2 {
		t.Fatalf("TrustedSources len = %d, want 2", len(policy.TrustedSources))
	}
	want0 := netip.MustParsePrefix("192.0.2.0/24")
	if policy.TrustedSources[0] != want0 {
		t.Errorf("TrustedSources[0] = %v, want %v", policy.TrustedSources[0], want0)
	}
}

func TestCoppParseConfigProtectedPort(t *testing.T) {
	input := `{"control-plane-protection":{"bgp":{"rate":"10/second","protected-port":"1790"}}}`
	policy, found, err := parseCoppConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected config to be found")
	}
	if len(policy.ProtectedPorts) != 1 || policy.ProtectedPorts[0] != 1790 {
		t.Errorf("ProtectedPorts = %v, want [1790]", policy.ProtectedPorts)
	}
}

func TestCoppParseConfigOverLimitDrop(t *testing.T) {
	input := `{"control-plane-protection":{"bgp":{"rate":"10/second","over-limit-policy":"drop"}}}`
	policy, _, err := parseCoppConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy.OverPolicy != "drop" {
		t.Errorf("OverPolicy = %q, want drop", policy.OverPolicy)
	}
}

func TestCoppParseConfigInvalidRate(t *testing.T) {
	input := `{"control-plane-protection":{"bgp":{"rate":"0/second"}}}`
	_, _, err := parseCoppConfig(input)
	if err == nil {
		t.Error("expected error for rate 0")
	}
}

func TestCoppParseConfigMissingRate(t *testing.T) {
	input := `{"control-plane-protection":{"bgp":{}}}`
	_, _, err := parseCoppConfig(input)
	if err == nil {
		t.Error("expected error for missing rate")
	}
}

func TestCoppParseConfigInvalidTrustedSource(t *testing.T) {
	input := `{"control-plane-protection":{"bgp":{"rate":"10/second","trusted-source":"not-a-prefix"}}}`
	_, _, err := parseCoppConfig(input)
	if err == nil {
		t.Error("expected error for invalid trusted-source")
	}
}

func TestCoppParseConfigInvalidOverLimitPolicy(t *testing.T) {
	input := `{"control-plane-protection":{"bgp":{"rate":"10/second","over-limit-policy":"reject"}}}`
	_, _, err := parseCoppConfig(input)
	if err == nil {
		t.Error("expected error for invalid over-limit-policy")
	}
}

func TestCoppParseConfigBoundaryRate(t *testing.T) {
	input := `{"control-plane-protection":{"bgp":{"rate":"1/second"}}}`
	policy, found, err := parseCoppConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected config to be found")
	}
	if policy.Rate != 1 {
		t.Errorf("Rate = %d, want 1 (last valid)", policy.Rate)
	}
}

func TestCoppParseConfigBoundaryPort(t *testing.T) {
	input := `{"control-plane-protection":{"bgp":{"rate":"10/second","protected-port":"65535"}}}`
	policy, _, err := parseCoppConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(policy.ProtectedPorts) != 1 || policy.ProtectedPorts[0] != 65535 {
		t.Errorf("ProtectedPorts = %v, want [65535]", policy.ProtectedPorts)
	}
}

// test-relax: port 0 now rejected by ValidatePort instead of silently using default 179
func TestCoppParseConfigPortZeroRejected(t *testing.T) {
	input := `{"control-plane-protection":{"bgp":{"rate":"10/second","protected-port":"0"}}}`
	_, _, err := parseCoppConfig(input)
	if err == nil {
		t.Fatal("expected error for port 0")
	}
}

func TestCoppParseConfigPortInvalidRejected(t *testing.T) {
	input := `{"control-plane-protection":{"bgp":{"rate":"10/second","protected-port":"abc"}}}`
	_, _, err := parseCoppConfig(input)
	if err == nil {
		t.Fatal("expected error for invalid port string")
	}
}

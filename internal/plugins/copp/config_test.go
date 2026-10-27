package copp

import (
	"errors"
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
	limit := policy.RateLimit
	if limit == nil {
		t.Fatal("RateLimit = nil, want the configured rate")
	}
	if limit.Rate != 100 {
		t.Errorf("Rate = %d, want 100", limit.Rate)
	}
	if limit.RateUnit != "second" {
		t.Errorf("RateUnit = %q, want second", limit.RateUnit)
	}
	if limit.Dimension != firewall.RateDimensionPackets {
		t.Errorf("Dimension = %d, want RateDimensionPackets", limit.Dimension)
	}
	if limit.Burst != 20 {
		t.Errorf("Burst = %d, want 20", limit.Burst)
	}
	if len(policy.ProtectedPorts) != 1 || policy.ProtectedPorts[0] != 179 {
		t.Errorf("ProtectedPorts = %v, want [179]", policy.ProtectedPorts)
	}
	if limit.OverPolicy != overPolicyDrop {
		t.Errorf("OverPolicy = %q, want drop (the default, spec-bcp194-3-session D-8)", limit.OverPolicy)
	}
}

// TestParseCoppConfigOverLimitDefaultDrop proves the over-limit-policy
// default: a limit written without the leaf drops what exceeds it, and the
// explicit `accept` spelling still admits and counts.
//
// VALIDATES: AC-10, parseRateLimit's default.
// PREVENTS: a configured limit that stops nothing (umbrella gap S2).
func TestParseCoppConfigOverLimitDefaultDrop(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"omitted", `{"control-plane-protection":{"bgp":{"rate":"10/second"}}}`, overPolicyDrop},
		{"explicit accept", `{"control-plane-protection":{"bgp":{"rate":"10/second","over-limit-policy":"accept"}}}`, overPolicyAccept},
		{"explicit drop", `{"control-plane-protection":{"bgp":{"rate":"10/second","over-limit-policy":"drop"}}}`, overPolicyDrop},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy, found, err := parseCoppConfig(tc.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if !found {
				t.Fatal("config parsed as absent")
			}
			if policy.RateLimit == nil {
				t.Fatal("RateLimit = nil, want the configured rate")
			}
			if policy.RateLimit.OverPolicy != tc.want {
				t.Errorf("OverPolicy = %q, want %q", policy.RateLimit.OverPolicy, tc.want)
			}
		})
	}
}

// TestCoppParseConfigBurstWithoutRateRefused proves a burst written alone is
// refused rather than dropped in silence: it sizes a limit that does not exist.
func TestCoppParseConfigBurstWithoutRateRefused(t *testing.T) {
	input := `{"control-plane-protection":{"bgp":{"burst":"20"}}}`
	_, _, err := parseCoppConfig(input)
	if !errors.Is(err, errBurstWithoutRate) {
		t.Fatalf("err = %v, want errBurstWithoutRate", err)
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
	if policy.RateLimit == nil {
		t.Fatal("RateLimit = nil, want the configured rate")
	}
	if policy.RateLimit.OverPolicy != "drop" {
		t.Errorf("OverPolicy = %q, want drop", policy.RateLimit.OverPolicy)
	}
}

func TestCoppParseConfigInvalidRate(t *testing.T) {
	input := `{"control-plane-protection":{"bgp":{"rate":"0/second"}}}`
	_, _, err := parseCoppConfig(input)
	if err == nil {
		t.Error("expected error for rate 0")
	}
}

// TestCoppParseConfigMissingRate proves `rate` is optional: the container
// without it is the ACL alone, parsed as present with no limiter.
//
// VALIDATES: AC-11 (spec-bcp194-3-session D-7: the ACL is RFC 7454 Section
// 4's SHOULD, the limit its MAY, so the ACL does not wait on a rate).
func TestCoppParseConfigMissingRate(t *testing.T) {
	input := `{"control-plane-protection":{"bgp":{}}}`
	policy, found, err := parseCoppConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !found {
		t.Fatal("a bgp container with no rate parsed as absent; want the ACL")
	}
	if policy.RateLimit != nil {
		t.Errorf("RateLimit = %+v, want nil: no rate was written", *policy.RateLimit)
	}
	if len(policy.ProtectedPorts) != 1 || policy.ProtectedPorts[0] != defaultPort {
		t.Errorf("ProtectedPorts = %v, want [%d]", policy.ProtectedPorts, defaultPort)
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
	if policy.RateLimit == nil {
		t.Fatal("RateLimit = nil, want the configured rate")
	}
	if policy.RateLimit.Rate != 1 {
		t.Errorf("Rate = %d, want 1 (last valid)", policy.RateLimit.Rate)
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

// ValidatePort rejects port 0 rather than silently substituting the default 179.
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

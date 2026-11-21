// Design: docs/guide/l2tp.md -- refusing pools that cannot allocate unique usable addresses

package l2tppool

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func poolValidationSection(t *testing.T, pool map[string]any) []sdk.ConfigSection {
	t.Helper()
	data, err := json.Marshal(map[string]any{"l2tp": map[string]any{"pool": pool}})
	if err != nil {
		t.Fatal(err)
	}
	return []sdk.ConfigSection{{Root: configRootL2TP, Data: string(data)}}
}

func poolValidationIPv4(start, end string) map[string]any {
	return map[string]any{"gateway": "192.0.2.1", "start": start, "end": end}
}

func poolValidationIPv6(block string) map[string]any {
	return map[string]any{"block": block, "delegation-length": "64"}
}

func poolValidationRefused(t *testing.T, pool map[string]any, details ...string) {
	t.Helper()
	err := verifyPoolConfig(poolValidationSection(t, pool))
	if err == nil {
		t.Fatal("verify accepted an invalid pool")
	}
	for _, detail := range details {
		if !strings.Contains(err.Error(), detail) {
			t.Fatalf("refusal %q does not name %q", err, detail)
		}
	}
}

// TestPoolVerifyRefusesOverlappingPools reaches the config verifier with each
// ownership ambiguity, including a single shared boundary address.
func TestPoolVerifyRefusesOverlappingPools(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pool    map[string]any
		details []string
	}{
		{"default-named", map[string]any{
			"ipv4":       poolValidationIPv4("10.0.0.10", "10.0.0.20"),
			"named-pool": map[string]any{"gold": poolValidationIPv4("10.0.0.20", "10.0.0.30")},
		}, []string{"ipv4", "named-pool gold", "10.0.0.20"}},
		{"named-named", map[string]any{
			"named-pool": map[string]any{
				"gold":   poolValidationIPv4("10.0.0.10", "10.0.0.30"),
				"silver": poolValidationIPv4("10.0.0.20", "10.0.0.40"),
			},
		}, []string{"named-pool gold", "named-pool silver", "10.0.0.20", "10.0.0.30"}},
	} {
		t.Run(tc.name, func(t *testing.T) { poolValidationRefused(t, tc.pool, tc.details...) })
	}
}

// TestPoolVerifyAcceptsDisjointPools keeps adjacent ranges available: the last
// address of one range may immediately precede the first of another.
func TestPoolVerifyAcceptsDisjointPools(t *testing.T) {
	err := verifyPoolConfig(poolValidationSection(t, map[string]any{
		"ipv4": poolValidationIPv4("10.0.0.10", "10.0.0.20"),
		"named-pool": map[string]any{
			"gold":   poolValidationIPv4("10.0.0.21", "10.0.0.30"),
			"silver": poolValidationIPv4("10.0.0.31", "10.0.0.40"),
		},
	}))
	if err != nil {
		t.Fatalf("disjoint pools refused: %v", err)
	}
}

// TestPoolVerifyRefusesOversizedRange covers the bound plus one and the uint32
// word-count wrap without making a large bitmap in the unfixed verifier.
func TestPoolVerifyRefusesOversizedRange(t *testing.T) {
	for _, tc := range []struct{ name, start, end, size string }{
		{"bound-plus-one", "10.0.0.0", "10.16.0.0", "1048577"},
		{"wrapping-word-count", "0.0.0.1", "255.255.255.255", "4294967295"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ipv4 := poolValidationIPv4(tc.start, tc.end)
			if tc.name == "wrapping-word-count" {
				ipv4["gateway"] = "0.0.0.0"
			}
			poolValidationRefused(t, map[string]any{"ipv4": ipv4},
				tc.start, tc.end, tc.size, "1048576")
		})
	}
}

// TestPoolVerifyAcceptsBoundedRange checks both the minimum range and the last
// admitted size through the same verifier that refuses the next address.
func TestPoolVerifyAcceptsBoundedRange(t *testing.T) {
	for _, tc := range []struct{ name, start, end string }{
		{"one-address", "10.0.0.10", "10.0.0.10"},
		{"bound", "10.0.0.0", "10.15.255.255"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyPoolConfig(poolValidationSection(t, map[string]any{"ipv4": poolValidationIPv4(tc.start, tc.end)}))
			if err != nil {
				t.Fatalf("bounded range refused: %v", err)
			}
		})
	}
}

// TestPoolVerifyRefusalKeepsItsFix proves the config verifier preserves the
// invalid leaf's concrete correction instead of replacing it with a generic
// validation command or a guessed config filename.
func TestPoolVerifyRefusalKeepsItsFix(t *testing.T) {
	err := verifyPoolConfig(poolValidationSection(t, map[string]any{
		"ipv4": poolValidationIPv4("10.0.0.0", "10.16.0.0"),
	}))
	if err == nil {
		t.Fatal("oversized range accepted")
	}
	refusal, isOperator := errors.AsType[*opererr.Error](err)
	if !isOperator {
		t.Fatalf("refusal has no operator correction: %v", err)
	}
	if got := refusal.Fix(); got != "set l2tp pool ipv4 end 10.15.255.255" {
		t.Fatalf("refusal fix = %q, want the bounded end command", got)
	}
}

// TestPoolVerifyRefusesZeroAndBroadcast requires an unusable boundary address
// to be named before it can enter an allocation bitmap.
func TestPoolVerifyRefusesZeroAndBroadcast(t *testing.T) {
	for _, tc := range []struct{ name, start, end, address string }{
		{"zero", "0.0.0.0", "0.0.0.1", "0.0.0.0"},
		{"broadcast", "255.255.255.254", "255.255.255.255", "255.255.255.255"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			poolValidationRefused(t, map[string]any{"ipv4": poolValidationIPv4(tc.start, tc.end)}, tc.address)
		})
	}
}

// TestPoolVerifyRefusesNonIPv4DNS exercises both leaves of default and named
// IPv4 pools, so a parser that checks only one form cannot satisfy it.
func TestPoolVerifyRefusesNonIPv4DNS(t *testing.T) {
	for _, named := range []bool{false, true} {
		for _, leaf := range []string{"dns-primary", "dns-secondary"} {
			name := "default/" + leaf
			if named {
				name = "named/" + leaf
			}
			t.Run(name, func(t *testing.T) {
				ipv4 := poolValidationIPv4("10.0.0.10", "10.0.0.20")
				ipv4[leaf] = "2001:db8::53"
				pool := map[string]any{"ipv4": ipv4}
				if named {
					pool = map[string]any{"named-pool": map[string]any{"gold": ipv4}}
				}
				poolValidationRefused(t, pool, leaf, "2001:db8::53")
			})
		}
	}
}

// TestPoolVerifyAcceptsIPv4DNS preserves explicitly configured IPv4 resolvers
// while refusing IPv6 values on the same two leaves.
func TestPoolVerifyAcceptsIPv4DNS(t *testing.T) {
	ipv4 := poolValidationIPv4("10.0.0.10", "10.0.0.20")
	ipv4["dns-primary"] = "198.51.100.53"
	ipv4["dns-secondary"] = "203.0.113.53"
	if err := verifyPoolConfig(poolValidationSection(t, map[string]any{"ipv4": ipv4})); err != nil {
		t.Fatalf("IPv4 DNS refused: %v", err)
	}
}

// TestPoolVerifyRefusesOverlappingIPv6Pools rejects intersecting ownership even
// when the two configured blocks use different delegation lengths.
func TestPoolVerifyRefusesOverlappingIPv6Pools(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pool    map[string]any
		details []string
	}{
		{"default-named", map[string]any{
			"ipv6-pd":         map[string]any{"block": "2001:db8:100::/56", "delegation-length": "60"},
			"named-ipv6-pool": map[string]any{"gold": poolValidationIPv6("2001:db8:100::/60")},
		}, []string{"ipv6-pd", "named-ipv6-pool gold", "2001:db8:100::/56", "2001:db8:100::/60"}},
		{"named-named", map[string]any{
			"named-ipv6-pool": map[string]any{
				"gold":   poolValidationIPv6("2001:db8:100::/56"),
				"silver": poolValidationIPv6("2001:db8:100:f0::/60"),
			},
		}, []string{"named-ipv6-pool gold", "named-ipv6-pool silver", "2001:db8:100:f0::/60"}},
	} {
		t.Run(tc.name, func(t *testing.T) { poolValidationRefused(t, tc.pool, tc.details...) })
	}
}

// TestPoolVerifyAcceptsDisjointIPv6Pools preserves useful disjoint delegation
// blocks when overlap validation refuses intersecting blocks.
func TestPoolVerifyAcceptsDisjointIPv6Pools(t *testing.T) {
	err := verifyPoolConfig(poolValidationSection(t, map[string]any{
		"ipv6-pd":         poolValidationIPv6("2001:db8:100::/56"),
		"named-ipv6-pool": map[string]any{"gold": poolValidationIPv6("2001:db8:101::/56")},
	}))
	if err != nil {
		t.Fatalf("disjoint IPv6 blocks refused: %v", err)
	}
}

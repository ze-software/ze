// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- local reservation authorization.
package rsvpte

import (
	"math"
	"net/netip"
	"testing"
)

// TestReservationPolicyPrecedence checks first-match numeric ordering in both
// directions, including a narrow permit before a broad deny and the reverse.
func TestReservationPolicyPrecedence(t *testing.T) {
	for _, firstAction := range []string{"permit", "deny"} {
		t.Run(firstAction, func(t *testing.T) {
			secondAction := "permit"
			if firstAction == "permit" {
				secondAction = "deny"
			}
			policy, err := parseReservationPolicy(map[string]any{
				"reservation-policy": map[string]any{
					"default-action": secondAction,
					"rule": map[string]any{
						"10": map[string]any{"sender-prefix": "192.0.2.0/24", "action": secondAction},
						"2":  map[string]any{"index": "2", "sender-prefix": "192.0.2.1/32", "action": firstAction},
					},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			endpoint := netip.MustParseAddr("198.51.100.1")
			if got := policy.allows(netip.MustParseAddr("192.0.2.1"), endpoint); got != (firstAction == "permit") {
				t.Errorf("overlapping rules: allows = %v, first action = %s", got, firstAction)
			}
			if got := policy.allows(netip.MustParseAddr("192.0.2.2"), endpoint); got != (secondAction == "permit") {
				t.Errorf("broad rule: allows = %v, second action = %s", got, secondAction)
			}
		})
	}
}

// TestReservationPolicyMatchBoundaries checks AND selectors, host boundaries,
// host-bit normalization, and a final wildcard rule through the config parser.
func TestReservationPolicyMatchBoundaries(t *testing.T) {
	policy, err := parseReservationPolicy(map[string]any{
		"reservation-policy": map[string]any{
			"rule": map[string]any{
				"0": map[string]any{
					"index": float64(0), "sender-prefix": "192.0.2.129/25",
					"endpoint-prefix": "198.51.100.9/32", "action": "permit",
				},
				"65535": map[string]any{"action": "deny"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		sender   string
		endpoint string
		allowed  bool
	}{
		{"network boundary", "192.0.2.128", "198.51.100.9", true},
		{"last address", "192.0.2.255", "198.51.100.9", true},
		{"below sender prefix", "192.0.2.127", "198.51.100.9", false},
		{"above sender prefix", "192.0.3.0", "198.51.100.9", false},
		{"endpoint below host", "192.0.2.129", "198.51.100.8", false},
		{"endpoint above host", "192.0.2.129", "198.51.100.10", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := policy.allows(netip.MustParseAddr(tc.sender), netip.MustParseAddr(tc.endpoint)); got != tc.allowed {
				t.Errorf("allows(%s, %s) = %v, want %v", tc.sender, tc.endpoint, got, tc.allowed)
			}
		})
	}
}

// TestReservationPolicyDefaults checks absent policy, configured defaults and
// omitted selectors without relying on the internal representation of a policy.
func TestReservationPolicyDefaults(t *testing.T) {
	sender := netip.MustParseAddr("192.0.2.1")
	endpoint := netip.MustParseAddr("198.51.100.9")
	if !(reservationPolicy{}).allows(sender, endpoint) {
		t.Fatal("zero policy denied an existing configuration")
	}
	for _, tc := range []struct {
		name    string
		tree    map[string]any
		allowed bool
	}{
		{"absent", nil, true},
		{"empty", map[string]any{"reservation-policy": map[string]any{}}, true},
		{"default permit", map[string]any{"reservation-policy": map[string]any{"default-action": "permit"}}, true},
		{"default deny", map[string]any{"reservation-policy": map[string]any{"default-action": "deny"}}, false},
		{"unmatched deny rule", map[string]any{"reservation-policy": map[string]any{
			"rule": map[string]any{"1": map[string]any{"sender-prefix": "203.0.113.0/24", "action": "deny"}},
		}}, true},
		{"unmatched permit rule", map[string]any{"reservation-policy": map[string]any{
			"default-action": "deny",
			"rule":           map[string]any{"1": map[string]any{"endpoint-prefix": "203.0.113.0/24", "action": "permit"}},
		}}, false},
		{"endpoint only", map[string]any{"reservation-policy": map[string]any{
			"default-action": "deny",
			"rule":           map[string]any{"1": map[string]any{"endpoint-prefix": "198.51.100.9/32", "action": "permit"}},
		}}, true},
		{"all IPv4 senders", map[string]any{"reservation-policy": map[string]any{
			"rule": map[string]any{"1": map[string]any{"sender-prefix": "0.0.0.0/0", "action": "deny"}},
		}}, false},
		{"wildcard permit", map[string]any{"reservation-policy": map[string]any{
			"default-action": "deny",
			"rule":           map[string]any{"1": map[string]any{"action": "permit"}},
		}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy, err := parseReservationPolicy(tc.tree)
			if err != nil {
				t.Fatal(err)
			}
			if got := policy.allows(sender, endpoint); got != tc.allowed {
				t.Errorf("allows = %v, want %v", got, tc.allowed)
			}
		})
	}
}

// TestParseReservationPolicyInvalid rejects malformed structures and values so
// an invalid deny rule cannot disappear or turn into a permit during parsing.
func TestParseReservationPolicyInvalid(t *testing.T) {
	for _, tc := range []struct {
		name   string
		policy any
	}{
		{"null container", nil},
		{"scalar container", "deny"},
		{"unknown setting", map[string]any{"default": "deny"}},
		{"null default", map[string]any{"default-action": nil}},
		{"unknown default", map[string]any{"default-action": "drop"}},
		{"numeric default", map[string]any{"default-action": float64(1)}},
		{"null list", map[string]any{"rule": nil}},
		{"array list", map[string]any{"rule": []any{map[string]any{"action": "deny"}}}},
		{"scalar entry", map[string]any{"rule": map[string]any{"1": "deny"}}},
		{"null entry", map[string]any{"rule": map[string]any{"1": nil}}},
		{"aliased order", map[string]any{"rule": map[string]any{
			"1": map[string]any{"action": "permit"}, "01": map[string]any{"action": "deny"},
		}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseReservationPolicy(map[string]any{"reservation-policy": tc.policy}); err == nil {
				t.Fatal("invalid policy accepted")
			}
		})
	}
	for _, key := range []string{"", "first", "-1", "65536", "+1", "01", "1.0", "1e1", "NaN", "Inf", " 1"} {
		t.Run("order "+key, func(t *testing.T) {
			_, err := parseReservationPolicy(map[string]any{"reservation-policy": map[string]any{
				"rule": map[string]any{key: map[string]any{"action": "deny"}},
			}})
			if err == nil {
				t.Fatal("invalid rule order accepted")
			}
		})
	}
	for _, tc := range []struct {
		name string
		rule map[string]any
	}{
		{"missing action", map[string]any{}},
		{"null action", map[string]any{"action": nil}},
		{"unknown action", map[string]any{"action": "reject"}},
		{"case action", map[string]any{"action": "Permit"}},
		{"numeric action", map[string]any{"action": float64(1)}},
		{"unknown selector", map[string]any{"action": "deny", "source-prefix": "192.0.2.0/24"}},
		{"index mismatch", map[string]any{"action": "deny", "index": "2"}},
		{"index alias", map[string]any{"action": "deny", "index": "01"}},
		{"fractional index", map[string]any{"action": "deny", "index": 1.5}},
		{"NaN index", map[string]any{"action": "deny", "index": math.NaN()}},
		{"infinite index", map[string]any{"action": "deny", "index": math.Inf(1)}},
		{"null index", map[string]any{"action": "deny", "index": nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseReservationPolicy(map[string]any{"reservation-policy": map[string]any{
				"rule": map[string]any{"1": tc.rule},
			}})
			if err == nil {
				t.Fatal("invalid rule accepted")
			}
		})
	}
	for _, selector := range []string{"sender-prefix", "endpoint-prefix"} {
		for _, value := range []any{nil, "", float64(1), "192.0.2.1", "192.0.2.0/33", "192.0.2.999/24", "2001:db8::/32", "::ffff:192.0.2.0/120", "192.0.2.0/24 "} {
			t.Run(selector, func(t *testing.T) {
				_, err := parseReservationPolicy(map[string]any{"reservation-policy": map[string]any{
					"rule": map[string]any{"1": map[string]any{"action": "deny", selector: value}},
				}})
				if err == nil {
					t.Fatalf("invalid selector %v accepted", value)
				}
			})
		}
	}
}

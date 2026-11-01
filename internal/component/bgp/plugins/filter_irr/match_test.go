package filter_irr

import (
	"net/netip"
	"testing"
)

// VALIDATES: AC-2 -- UPDATE with prefix in IRR-resolved list accepted
// VALIDATES: AC-3 -- UPDATE with prefix NOT in IRR-resolved list rejected
// VALIDATES: AC-12 -- IPv4 and IPv6 prefixes filtered correctly
// PREVENTS: Prefix matching logic silently accepts non-matching routes

func TestEvaluatePrefix(t *testing.T) {
	entries := []prefixEntry{
		{prefix: netip.MustParsePrefix("10.0.0.0/8"), ge: 16, le: 24},
		{prefix: netip.MustParsePrefix("172.16.0.0/12"), ge: 12, le: 32},
		{prefix: netip.MustParsePrefix("2001:db8::/32"), ge: 32, le: 48},
	}

	tests := []struct {
		name   string
		route  string
		accept bool
	}{
		{"ipv4 match exact range", "10.0.0.0/24", true},
		{"ipv4 match lower bound", "10.0.0.0/16", true},
		{"ipv4 too short", "10.0.0.0/8", false},
		{"ipv4 too long", "10.0.0.0/25", false},
		{"ipv4 outside prefix", "192.168.0.0/24", false},
		{"ipv4 second entry", "172.16.0.0/16", true},
		{"ipv6 match", "2001:db8:1::/48", true},
		{"ipv6 exact", "2001:db8::/32", true},
		{"ipv6 too long", "2001:db8:1:2::/64", false},
		{"ipv6 outside", "2001:db9::/32", false},
		{"cross family no match", "::ffff:10.0.0.0/24", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := netip.MustParsePrefix(tt.route)
			got := evaluatePrefix(entries, route)
			if got != tt.accept {
				t.Errorf("evaluatePrefix(%s) = %v, want %v", tt.route, got, tt.accept)
			}
		})
	}
}

// VALIDATES: AC-3 -- empty list rejects all (fail-closed).
// PREVENTS: Empty IRR result silently accepts routes.
func TestEvaluatePrefixEmptyList(t *testing.T) {
	route := netip.MustParsePrefix("10.0.0.0/24")
	if evaluatePrefix(nil, route) {
		t.Error("empty entries should reject all")
	}
}

// VALIDATES: AC-12 -- ge/le boundary values handled correctly.
func TestEvaluatePrefixBoundary(t *testing.T) {
	tests := []struct {
		name   string
		entry  prefixEntry
		route  string
		accept bool
	}{
		{"ge=0 le=128 ipv6 any length", prefixEntry{prefix: netip.MustParsePrefix("::/0"), ge: 0, le: 128}, "2001:db8::/32", true},
		{"ge=0 le=128 ipv6 /128", prefixEntry{prefix: netip.MustParsePrefix("::/0"), ge: 0, le: 128}, "2001:db8::1/128", true},
		{"ge=32 le=32 exact only", prefixEntry{prefix: netip.MustParsePrefix("10.0.0.0/8"), ge: 32, le: 32}, "10.0.0.1/32", true},
		{"ge=32 le=32 rejects /24", prefixEntry{prefix: netip.MustParsePrefix("10.0.0.0/8"), ge: 32, le: 32}, "10.0.0.0/24", false},
		{"ge=0 le=0 ipv4 default only", prefixEntry{prefix: netip.MustParsePrefix("0.0.0.0/0"), ge: 0, le: 0}, "0.0.0.0/0", true},
		{"ge=0 le=0 rejects /1", prefixEntry{prefix: netip.MustParsePrefix("0.0.0.0/0"), ge: 0, le: 0}, "0.0.0.0/1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := netip.MustParsePrefix(tt.route)
			got := evaluatePrefix([]prefixEntry{tt.entry}, route)
			if got != tt.accept {
				t.Errorf("evaluatePrefix(%s) = %v, want %v", tt.route, got, tt.accept)
			}
		})
	}
}

// TestJudgeNLRI checks the verdict counts judgeNLRI reads from the prefix
// field: an in-list prefix is accepted, an out-of-list one rejected, a route
// announcing nothing judges nothing, and an element naming no prefix is an
// error, never a partial judgment. It carries the cases the text evaluator's
// test held (evaluateUpdate), on the field input that replaced it.
func TestJudgeNLRI(t *testing.T) {
	list := &irrPrefixList{entries: []prefixEntry{
		{prefix: netip.MustParsePrefix("10.0.0.0/8"), ge: 16, le: 24},
	}}
	tests := []struct {
		name     string
		fields   []byte
		accepted int
		rejected int
		err      bool
	}{
		{"matching prefix", announceFields("10.0.0.0/24"), 1, 0, false},
		{"non-matching prefix", announceFields("192.168.0.0/24"), 0, 1, false},
		{"no fields", nil, 0, 0, false},
		{"a family with no NLRI", routeFields(map[string][]string{"ipv4/unicast": {}}, nil), 0, 0, false},
		{"malformed prefix", announceFields("not-a-prefix"), 0, 0, true},
		{"an object naming no prefix", routeFields(map[string][]string{"ipv4/unicast": {`{"path-id":1}`}}, nil), 0, 0, true},
		{"fields that do not decode", []byte(`{"prefix":[]}`), 0, 0, true},
		{"multiple all match", announceFields("10.0.0.0/24", "10.0.1.0/24"), 2, 0, false},
		{"multiple one no match", announceFields("10.0.0.0/24", "192.168.0.0/24"), 1, 1, false},
		{"ADD-PATH shape", routeFields(map[string][]string{"ipv4/unicast": {`{"path-id":0,"prefix":"10.0.0.0/24"}`}}, nil), 1, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			judged, err := judgeNLRI(tt.fields, list)
			if tt.err {
				if err == nil {
					t.Fatalf("judgeNLRI(%s) = %+v, want an error", tt.fields, judged)
				}
				return
			}
			if err != nil {
				t.Fatalf("judgeNLRI(%s): %v", tt.fields, err)
			}
			if judged.accepted != tt.accepted || judged.rejected != tt.rejected {
				t.Errorf("judgeNLRI(%s): accepted=%d rejected=%d, want %d and %d",
					tt.fields, judged.accepted, judged.rejected, tt.accepted, tt.rejected)
			}
		})
	}
}

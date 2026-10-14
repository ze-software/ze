package lg

import (
	"strings"
	"testing"
)

func TestIsValidPeerName(t *testing.T) {
	// VALIDATES: input validation guards API endpoints against injection.
	// PREVENTS: command injection via peer name parameter.
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid simple", "peer1", true},
		{"valid with dots", "10.0.0.1", true},
		{"valid with hyphens", "my-peer", true},
		{"valid with underscores", "peer_1", true},
		{"valid mixed", "Peer-1_test.local", true},
		{"valid ipv6", "2001:db8::1", true},
		{"valid ipv6 full", "2001:0db8:0000:0000:0000:0000:0000:0001", true},
		{"empty", "", false},
		{"too long", string(make([]byte, 256)), false},
		{"max length", string(make([]byte, 255)), false}, // all zero bytes = invalid chars
		{"space", "peer 1", false},
		{"semicolon", "peer;rm", false},
		{"newline", "peer\nname", false},
		{"slash", "peer/name", false},
		{"backslash", "peer\\name", false},
		{"pipe", "peer|name", false},
		{"backtick", "peer`name", false},
		{"dollar", "peer$name", false},
		{"angle bracket", "peer<name>", false},
		{"quotes", `peer"name`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidPeerName(tt.input)
			if got != tt.want {
				t.Errorf("isValidPeerName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestIsValidBMPPeerKey proves routes/bmp/{name} admits every key the BMP
// plugin stores routes under and nothing that could change the command.
//
// VALIDATES: the three bmpCompositeKey forms (bgp/plugins/bmp/bmp.go), IPv4 and
// bracketed IPv6 routers, monitored and Loc-RIB peers, are accepted.
// PREVENTS: a crafted name adding words to `show bgp rib protocol bmp peer
// <key>`: the command tokenizer (plugin/server/command.go tokenize) splits on
// space and tab, groups on a double quote, and the pipe parser cuts on '|'.
func TestIsValidBMPPeerKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"ipv4 router, monitored peer", "192.0.2.8:12345:10.0.0.1", true},
		{"ipv6 router, ipv6 peer", "[2001:db8::8]:4000:2001:db8::1", true},
		{"ipv6 router, loc-rib", "[2001:db8::8]:4000:loc-rib:7:3221225985", true},
		{"ipv4 router, loc-rib", "192.0.2.8:12345:loc-rib:0:16909060", true},
		{"link-local router with zone", "[fe80::8%eth0]:4000:fe80::1", true},
		{"zone with dot, dash, underscore", "[fe80::8%br-lan_0.100]:4000:fe80::1", true},
		{"rd instance peer", "192.0.2.8:12345:instance:1:4294967298:10.0.0.1", true},
		{"empty zone", "[fe80::8%]:4000:fe80::1", false},
		{"unclosed zone", "[fe80::8%eth0", false},
		{"zone with colon", "[fe80::8%eth:0]:4000:fe80::1", false},
		{"zone with semicolon", "[fe80::8%e;x]:4000:fe80::1", false},
		{"zone with space", "[fe80::8%e x]:4000:fe80::1", false},
		{"zone with pipe", "[fe80::8%e|x]:4000:fe80::1", false},
		{"zone with percent", "[fe80::8%e%x]:4000:fe80::1", false},
		{"empty", "", false},
		{"too long", "[" + strings.Repeat("1", 255) + "]", false},
		{"extra word", "192.0.2.8:12345:10.0.0.1 family ipv4/unicast", false},
		{"tab", "192.0.2.8:12345:10.0.0.1\tbest", false},
		{"pipe", "192.0.2.8:12345:10.0.0.1|count", false},
		{"quote", `[::1]:1:"x y"`, false},
		{"backslash", `[::1]:1:\x`, false},
		{"newline", "[::1]:1:a\nb", false},
		{"semicolon", "[::1]:1:a;b", false},
		{"slash", "[::1]:1:a/b", false},
		{"brace", "[::1]:1:{a}", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidBMPPeerKey(tt.input)
			if got != tt.want {
				t.Errorf("isValidBMPPeerKey(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidFamily(t *testing.T) {
	// VALIDATES: family parameter validation for routes/table endpoint.
	// PREVENTS: command injection via family parameter.
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid ipv4 unicast", "ipv4/unicast", true},
		{"valid ipv6 vpn", "ipv6/mpls-vpn", true},
		{"valid l2vpn evpn", "l2vpn/evpn", true},
		{"no slash", "ipv4unicast", false},
		{"empty afi", "/unicast", false},
		{"empty safi", "ipv4/", false},
		{"empty", "", false},
		{"double slash", "ipv4//unicast", false},
		{"space injection", "ipv4/unicast rm", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidFamily(tt.input)
			if got != tt.want {
				t.Errorf("isValidFamily(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidPrefix(t *testing.T) {
	// VALIDATES: prefix parameter validation for lookup/search endpoints.
	// PREVENTS: command injection via prefix parameter.
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid ipv4 cidr", "10.0.0.0/24", true},
		{"valid ipv4 host", "10.0.0.1", true},
		{"valid ipv6", "2001:db8::/32", true},
		{"valid ipv6 host", "2001:db8::1", true},
		{"empty", "", false},
		{"too long", string(make([]byte, 51)), false},
		{"max length", "2001:0db8:0000:0000:0000:0000:0000:0001/128aaaaaa", true}, // 50 chars, hex chars only
		{"letters outside hex", "10.0.0.0/24g", false},
		{"space injection", "10.0.0.0/24 rm", false},
		{"semicolon", "10.0.0.0;rm", false},
		{"pipe", "10.0.0.0|rm", false},
		{"newline", "10.0.0.0\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidPrefix(tt.input)
			if got != tt.want {
				t.Errorf("isValidPrefix(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidASPathPattern(t *testing.T) {
	// VALIDATES: AS path pattern validation for search endpoint.
	// PREVENTS: injection via pattern parameter.
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple numbers", "65001 65002", true},
		{"single asn", "65001", true},
		{"regex dot star", "65001.*", true},
		{"regex anchors", "^65001$", true},
		{"regex alternation", "65001|65002", true},
		{"regex grouping", "(65001)+", true},
		{"regex class", "[0-9]", true},
		{"empty", "", false},
		{"too long", string(make([]byte, 201)), false},
		{"letters", "abc", false},
		{"semicolon", "65001;rm", false},
		{"backtick", "65001`rm`", false},
		{"angle brackets", "<script>", false},
		{"newline", "65001\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidASPathPattern(tt.input)
			if got != tt.want {
				t.Errorf("isValidASPathPattern(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidCommunity(t *testing.T) {
	// VALIDATES: community string validation for search endpoint.
	// PREVENTS: injection via community parameter.
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"standard", "65000:100", true},
		{"large", "65000:100:200", true},
		{"with space", "65000:100 65001:200", true},
		{"empty", "", false},
		{"too long", string(make([]byte, 101)), false},
		{"letters", "abc:123", false},
		{"semicolon", "65000:100;rm", false},
		{"pipe", "65000:100|rm", false},
		{"slash", "65000/100", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidCommunity(tt.input)
			if got != tt.want {
				t.Errorf("isValidCommunity(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseJSON(t *testing.T) {
	// VALIDATES: JSON parsing handles all response shapes.
	// PREVENTS: panics or misinterpretation of engine responses.
	tests := []struct {
		name string
		in   string
		keys []string // expected top-level keys, nil means result should be nil
	}{
		{"empty string", "", nil},
		{"valid object", `{"router-id":"1.2.3.4"}`, []string{"router-id"}},
		{"valid array wraps to peers", `[{"name":"p1"}]`, []string{"peers"}},
		{"invalid json", `{broken`, nil},
		{"empty object", `{}`, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseJSON(tt.in)
			if tt.keys == nil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil result")
			}
			for _, k := range tt.keys {
				if _, ok := got[k]; !ok {
					t.Errorf("missing key %q", k)
				}
			}
		})
	}
}

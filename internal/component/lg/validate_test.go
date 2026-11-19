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

// TestParsePrefixLookupRefusesUnsafeInput carries the injection table the
// character check isValidPrefix answered, now answered by the parse that
// replaced it.
//
// VALIDATES: an address or a prefix is accepted, and becomes the selector
// that asks for it; every value that is neither is refused, so no text the
// user typed reaches the command line.
// PREVENTS: command injection via the prefix parameter. The old check
// accepted "max length", fifty hex characters that are no prefix, because it
// read characters rather than an address; the parse refuses it.
func TestParsePrefixLookupRefusesUnsafeInput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		keyword string // "" when the value is refused
	}{
		{"valid ipv4 cidr", "10.0.0.0/24", filterExact},
		{"valid ipv4 host", "10.0.0.1", filterCovering},
		{"valid ipv6", "2001:db8::/32", filterExact},
		{"valid ipv6 host", "2001:db8::1", filterCovering},
		{"empty", "", ""},
		{"too long", string(make([]byte, 51)), ""},
		{"max length", "2001:0db8:0000:0000:0000:0000:0000:0001/128aaaaaa", ""},
		{"letters outside hex", "10.0.0.0/24g", ""},
		{"space injection", "10.0.0.0/24 rm", ""},
		{"semicolon", "10.0.0.0;rm", ""},
		{"pipe", "10.0.0.0|rm", ""},
		{"newline", "10.0.0.0\n", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup, err := parsePrefixLookup(tt.input)
			if tt.keyword == "" {
				if err == nil {
					t.Errorf("parsePrefixLookup(%q) = %s %s, want a refusal", tt.input, lookup.keyword, lookup.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePrefixLookup(%q) refused: %v", tt.input, err)
			}
			if lookup.keyword != tt.keyword {
				t.Errorf("parsePrefixLookup(%q) keyword = %s, want %s", tt.input, lookup.keyword, tt.keyword)
			}
		})
	}
}

// TestParseASPathPattern proves the AS path field accepts the engine's
// grammar and nothing else, and writes it in the engine's form.
//
// VALIDATES: AS numbers separated by spaces, commas or both, with one
// optional leading ^ and one optional $ directly after the last number,
// become the comma form bgp-rib's validatePathPattern and matchASPath read
// (parsePathPattern, internal/component/bgp/plugins/rib/rib_pipeline.go),
// every AS number written back as asplain; the AS number bound holds at
// 4294967295. Every other value is refused with an operator error naming the
// value, the form the field takes, and a valid example on the last line.
// PREVENTS: a spaced path sent as several tokens the engine answers with
// "unknown keyword", and a regular-expression character the page accepted
// and the engine refused as an invalid ASN. The old check isValidASPathPattern
// accepted "65001.*", "65001|65002", "(65001)+" and "[0-9]": the engine
// matches none of them, so each is refused at the page.
//
// The end anchor is the engine's since 2026-11-18: one $ directly after the
// last AS number. The accepted rows carry it, and the refused rows carry every
// other place a $ can be typed, each of which the engine refuses as well
// (`$`, `^$`, `65001$$`, `$65001`, `65001$,65002`, `65001 $`, `65001,$`).
//
// The accepted outputs are the engine's input. lg imports no RIB package, so
// this table cannot be fed to validatePathPattern from here; the agreement is
// proven through a real bgp-rib by test/plugin/lg-aspath-search.ci.
func TestParseASPathPattern(t *testing.T) {
	accepted := []struct {
		name, input, engine string
	}{
		{"simple numbers", "65001 65002", "65001,65002"},
		{"single asn", "65001", "65001"},
		{"anchored", "^65001", "^65001"},
		{"anchored run", "^65001 65002", "^65001,65002"},
		{"engine comma form", "65001,65002", "65001,65002"},
		{"spaces around commas", "65001 , 65002,65003", "65001,65002,65003"},
		{"repeated spaces and surrounding spaces", "  65001   65002 ", "65001,65002"},
		{"asdot spelling", "1.10 65002", "65546,65002"},
		{"zero", "0", "0"},
		{"last valid as number", "4294967295", "4294967295"},
		{"end anchor", "65001$", "65001$"},
		{"both anchors on one number", "^65001$", "^65001$"},
		{"both anchors on a run", "^65010 65001$", "^65010,65001$"},
		{"end anchor in the engine comma form", "65010,65001$", "65010,65001$"},
		{"end anchor after an asdot number", "65002 1.10$", "65002,65546$"},
		{"end anchor with surrounding spaces", "  65001$ ", "65001$"},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			pattern, err := parseASPathPattern(tt.input)
			if err != nil {
				t.Fatalf("parseASPathPattern(%q) refused: %v", tt.input, err)
			}
			if pattern.engineText != tt.engine {
				t.Errorf("parseASPathPattern(%q) = %q, want %q", tt.input, pattern.engineText, tt.engine)
			}
		})
	}

	refused := []struct {
		name, input, problem string
	}{
		{"regex dot star", "65001.*", "holds 65001.*, which is not an AS number"},
		{"regex anchors with the end anchor doubled", "^65001$$", "holds 65001$$, which is not an AS number"},
		{"end anchor doubled", "65001$$", "holds 65001$$, which is not an AS number"},
		{"end anchor before the number", "$65001", "holds $65001, which is not an AS number"},
		{"end anchor before the last number", "65001$ 65002", "holds 65001$, which is not an AS number"},
		{"end anchor between two numbers", "65001 $ 65002", "holds $, which is not an AS number"},
		{"end anchor apart from the last number", "65001 $", "holds $, which is not an AS number"},
		{"end anchor after a comma", "65001,$", "holds $, which is not an AS number"},
		{"end anchor alone", "$", "holds $, which is not an AS number"},
		{"both anchors and no number", "^$", "holds $, which is not an AS number"},
		{"regex alternation", "65001|65002", "holds 65001|65002, which is not an AS number"},
		{"regex grouping", "(65001)+", "holds (65001)+, which is not an AS number"},
		{"grouped alternation", "(65001|65002)", "holds (65001|65002), which is not an AS number"},
		{"regex class", "[0-9]", "holds [0-9], which is not an AS number"},
		{"question mark", "65001?", "holds 65001?, which is not an AS number"},
		{"underscore", "_65001_", "holds _65001_, which is not an AS number"},
		{"dash", "65001-65002", "holds 65001-65002, which is not an AS number"},
		{"second anchor", "^65001 ^65002", "holds ^65002, which is not an AS number"},
		{"first past the last as number", "4294967296", "holds 4294967296, which is not an AS number"},
		{"empty", "", "holds no AS number"},
		{"anchor alone", "^", "holds no AS number"},
		{"separators alone", " , ", "holds no AS number"},
		{"too long", string(make([]byte, 201)), "characters is longer than the 200 the field takes"},
		{"letters", "abc", "holds abc, which is not an AS number"},
		{"semicolon", "65001;rm", "holds 65001;rm, which is not an AS number"},
		{"backtick", "65001`rm`", "holds 65001`rm`, which is not an AS number"},
		{"angle brackets", "<script>", "holds <script>, which is not an AS number"},
		{"newline", "65001\n", "holds 65001\n, which is not an AS number"},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			pattern, err := parseASPathPattern(tt.input)
			if err == nil {
				t.Fatalf("parseASPathPattern(%q) = %q, want a refusal", tt.input, pattern.engineText)
			}
			message := err.Error()
			if !strings.Contains(message, tt.problem) {
				t.Errorf("refusal %q does not say %q", message, tt.problem)
			}
			if !strings.Contains(message, "separated by spaces") || !strings.Contains(message, "a leading ^ anchors") {
				t.Errorf("refusal %q does not name the form the field takes", message)
			}
			if !strings.Contains(message, "a $ directly after the last AS number anchors it at the end") {
				t.Errorf("refusal %q does not name the end anchor the field takes", message)
			}
			lines := strings.Split(message, "\n")
			if last := lines[len(lines)-1]; last != "fix: "+asPathPatternExample {
				t.Errorf("last line = %q, want %q", last, "fix: "+asPathPatternExample)
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

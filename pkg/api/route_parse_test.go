package api

import (
	"net/netip"
	"strings"
	"testing"
)

// TestSplitPrefix tests prefix splitting for the 'split /N' syntax.
//
// VALIDATES: A prefix is correctly split into more-specific prefixes.
// For example, 10.0.0.0/21 split /23 produces 4 /23 prefixes.
//
// PREVENTS: Incorrect splitting that would cause route announcement mismatches.
func TestSplitPrefix(t *testing.T) {
	tests := []struct {
		name         string
		prefix       string
		targetLen    int
		wantPrefixes []string
		wantErr      bool
	}{
		// Valid IPv4 splits
		{
			name:      "/21 to /23 (4 prefixes)",
			prefix:    "1.0.0.0/21",
			targetLen: 23,
			wantPrefixes: []string{
				"1.0.0.0/23",
				"1.0.2.0/23",
				"1.0.4.0/23",
				"1.0.6.0/23",
			},
			wantErr: false,
		},
		{
			name:      "/24 to /25 (2 prefixes)",
			prefix:    "10.0.0.0/24",
			targetLen: 25,
			wantPrefixes: []string{
				"10.0.0.0/25",
				"10.0.0.128/25",
			},
			wantErr: false,
		},
		{
			name:         "/16 to /24 (256 prefixes)",
			prefix:       "192.168.0.0/16",
			targetLen:    24,
			wantPrefixes: nil, // too many to list, just check count
			wantErr:      false,
		},
		{
			name:         "same length (1 prefix)",
			prefix:       "10.0.0.0/24",
			targetLen:    24,
			wantPrefixes: []string{"10.0.0.0/24"},
			wantErr:      false,
		},

		// Valid IPv6 splits
		{
			name:      "/48 to /49 (2 prefixes)",
			prefix:    "2001:db8::/48",
			targetLen: 49,
			wantPrefixes: []string{
				"2001:db8::/49",
				"2001:db8:0:8000::/49",
			},
			wantErr: false,
		},

		// Invalid cases
		{
			name:         "target smaller than source",
			prefix:       "10.0.0.0/24",
			targetLen:    16,
			wantPrefixes: nil,
			wantErr:      true,
		},
		{
			name:         "target too large for IPv4",
			prefix:       "10.0.0.0/24",
			targetLen:    33,
			wantPrefixes: nil,
			wantErr:      true,
		},
		{
			name:         "target too large for IPv6",
			prefix:       "2001:db8::/48",
			targetLen:    129,
			wantPrefixes: nil,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := netip.MustParsePrefix(tt.prefix)
			got, err := splitPrefix(prefix, tt.targetLen)

			if (err != nil) != tt.wantErr {
				t.Errorf("splitPrefix(%s, %d) error = %v, wantErr %v", tt.prefix, tt.targetLen, err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// For /16 to /24, just check count
			if tt.wantPrefixes == nil {
				expectedCount := 1 << (tt.targetLen - prefix.Bits())
				if len(got) != expectedCount {
					t.Errorf("splitPrefix(%s, %d) got %d prefixes, want %d", tt.prefix, tt.targetLen, len(got), expectedCount)
				}
				return
			}

			if len(got) != len(tt.wantPrefixes) {
				t.Errorf("splitPrefix(%s, %d) got %d prefixes, want %d", tt.prefix, tt.targetLen, len(got), len(tt.wantPrefixes))
				return
			}

			for i, p := range got {
				if p.String() != tt.wantPrefixes[i] {
					t.Errorf("splitPrefix(%s, %d)[%d] = %s, want %s", tt.prefix, tt.targetLen, i, p.String(), tt.wantPrefixes[i])
				}
			}
		})
	}
}

// TestParseSplitArg tests parsing of 'split /N' argument.
//
// VALIDATES: Split argument is correctly extracted from command args.
//
// PREVENTS: Incorrect parsing of split target length.
func TestParseSplitArg(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantLen   int
		wantFound bool
	}{
		{"split /23", []string{"split", "/23"}, 23, true},
		{"split /24", []string{"split", "/24"}, 24, true},
		{"no split", []string{"next-hop", "1.2.3.4"}, 0, false},
		{"split at end", []string{"med", "100", "split", "/25"}, 25, true},
		{"split without value", []string{"split"}, 0, false},
		{"invalid split value", []string{"split", "23"}, 0, false}, // missing /
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLen, gotFound := parseSplitArg(tt.args)
			if gotLen != tt.wantLen || gotFound != tt.wantFound {
				t.Errorf("parseSplitArg(%v) = (%d, %v), want (%d, %v)",
					tt.args, gotLen, gotFound, tt.wantLen, tt.wantFound)
			}
		})
	}
}

// TestParseOrigin tests origin parsing per RFC 4271 Section 5.1.1.
// RFC 4271: ORIGIN is a well-known mandatory attribute with values:
//   - IGP (0): Network Layer Reachability Information is interior to the originating AS
//   - EGP (1): Network Layer Reachability Information learned via EGP
//   - INCOMPLETE (2): Network Layer Reachability Information learned by some other means
func TestParseOrigin(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint8
		wantErr bool
	}{
		// Valid origins
		{"igp lowercase", "igp", 0, false},
		{"IGP uppercase", "IGP", 0, false},
		{"egp lowercase", "egp", 1, false},
		{"EGP uppercase", "EGP", 1, false},
		{"incomplete lowercase", "incomplete", 2, false},
		{"INCOMPLETE uppercase", "INCOMPLETE", 2, false},
		{"? alias", "?", 2, false},

		// Invalid origins
		{"empty string", "", 0, true},
		{"invalid value", "invalid", 0, true},
		{"numeric", "0", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOrigin(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOrigin(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseOrigin(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseBracketedList tests parsing of bracketed token lists.
func TestParseBracketedList(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantTokens   []string
		wantConsumed int
	}{
		// Valid bracketed lists
		{"single token", []string{"[100]"}, []string{"100"}, 1},
		{"multiple tokens", []string{"[", "1", "2", "3", "]"}, []string{"1", "2", "3"}, 5},
		{"joined brackets", []string{"[1", "2", "3]"}, []string{"1", "2", "3"}, 3},
		{"comma separated", []string{"[1,2,3]"}, []string{"1", "2", "3"}, 1},
		{"mixed", []string{"[1,2", "3", "4,5]"}, []string{"1", "2", "3", "4", "5"}, 3},
		{"empty brackets", []string{"[]"}, nil, 1},

		// Single value without brackets (ExaBGP compatible)
		{"single no brackets", []string{"100"}, []string{"100"}, 1},
		{"single comma separated", []string{"1,2,3"}, []string{"1", "2", "3"}, 1},

		// Empty
		{"empty input", []string{}, nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, consumed := parseBracketedList(tt.args)
			if consumed != tt.wantConsumed {
				t.Errorf("parseBracketedList(%v) consumed = %d, want %d", tt.args, consumed, tt.wantConsumed)
			}
			if len(tokens) != len(tt.wantTokens) {
				t.Errorf("parseBracketedList(%v) tokens = %v, want %v", tt.args, tokens, tt.wantTokens)
				return
			}
			for i, tok := range tokens {
				if tok != tt.wantTokens[i] {
					t.Errorf("parseBracketedList(%v) tokens[%d] = %q, want %q", tt.args, i, tok, tt.wantTokens[i])
				}
			}
		})
	}
}

// TestParseASPath tests AS_PATH parsing per RFC 4271 Section 5.1.2.
func TestParseASPath(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantPath     []uint32
		wantConsumed int
		wantErr      bool
	}{
		// Valid AS paths
		{"single ASN", []string{"[100]"}, []uint32{100}, 1, false},
		{"multiple ASNs", []string{"[", "100", "200", "300", "]"}, []uint32{100, 200, 300}, 5, false},
		{"4-byte ASNs", []string{"[4200000001]"}, []uint32{4200000001}, 1, false},
		{"empty path", []string{"[]"}, nil, 1, false},

		// Single value without brackets (ExaBGP compatible)
		{"single no brackets", []string{"100"}, []uint32{100}, 1, false},

		// Invalid
		{"invalid ASN", []string{"[abc]"}, nil, 1, true},
		{"empty input", []string{}, nil, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, consumed, err := parseASPath(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseASPath(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
				return
			}
			if consumed != tt.wantConsumed {
				t.Errorf("parseASPath(%v) consumed = %d, want %d", tt.args, consumed, tt.wantConsumed)
			}
			if len(path) != len(tt.wantPath) {
				t.Errorf("parseASPath(%v) path = %v, want %v", tt.args, path, tt.wantPath)
				return
			}
			for i, asn := range path {
				if asn != tt.wantPath[i] {
					t.Errorf("parseASPath(%v) path[%d] = %d, want %d", tt.args, i, asn, tt.wantPath[i])
				}
			}
		})
	}
}

// TestParseCommunity tests single community parsing per RFC 1997.
// RFC 1997: Communities are 32-bit values encoded as ASN:value.
func TestParseCommunity(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint32
		wantErr bool
	}{
		// Valid ASN:value format
		{"simple", "2914:666", (2914 << 16) | 666, false},
		{"zero values", "0:0", 0, false},
		{"max values", "65535:65535", 0xFFFFFFFF, false},

		// Well-known communities per RFC 1997
		{"no-export", "no-export", 0xFFFFFF01, false},
		{"no-advertise", "no-advertise", 0xFFFFFF02, false},
		{"no-export-subconfed", "no-export-subconfed", 0xFFFFFF03, false},
		{"nopeer (RFC 3765)", "nopeer", 0xFFFFFF04, false},
		{"blackhole (RFC 7999)", "blackhole", 0xFFFF029A, false},

		// Case insensitivity
		{"NO-EXPORT uppercase", "NO-EXPORT", 0xFFFFFF01, false},
		{"No-Advertise mixed", "No-Advertise", 0xFFFFFF02, false},

		// Invalid
		{"missing colon", "2914666", 0, true},
		{"too many colons", "2914:666:1", 0, true},
		{"invalid ASN", "abc:666", 0, true},
		{"invalid value", "2914:abc", 0, true},
		{"ASN too large", "65536:1", 0, true},
		{"value too large", "1:65536", 0, true},
		{"empty string", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCommunity(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCommunity(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseCommunity(%q) = 0x%08X, want 0x%08X", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseCommunities tests multiple community parsing.
func TestParseCommunities(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantComms    []uint32
		wantConsumed int
		wantErr      bool
	}{
		// Valid
		{"single", []string{"[2914:666]"}, []uint32{(2914 << 16) | 666}, 1, false},
		{"multiple", []string{"[2914:1", "2914:2]"}, []uint32{(2914 << 16) | 1, (2914 << 16) | 2}, 2, false},
		{"with well-known", []string{"[no-export", "2914:1]"}, []uint32{0xFFFFFF01, (2914 << 16) | 1}, 2, false},
		{"empty", []string{"[]"}, nil, 1, false},

		// Single value without brackets (ExaBGP compatible)
		{"single no brackets", []string{"2914:666"}, []uint32{(2914 << 16) | 666}, 1, false},

		// Invalid
		{"invalid community", []string{"[invalid]"}, nil, 1, true},
		{"empty input", []string{}, nil, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comms, consumed, err := parseCommunities(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCommunities(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
				return
			}
			if consumed != tt.wantConsumed {
				t.Errorf("parseCommunities(%v) consumed = %d, want %d", tt.args, consumed, tt.wantConsumed)
			}
			if len(comms) != len(tt.wantComms) {
				t.Errorf("parseCommunities(%v) comms = %v, want %v", tt.args, comms, tt.wantComms)
				return
			}
			for i, c := range comms {
				if c != tt.wantComms[i] {
					t.Errorf("parseCommunities(%v) comms[%d] = 0x%08X, want 0x%08X", tt.args, i, c, tt.wantComms[i])
				}
			}
		})
	}
}

// TestParseLargeCommunity tests single large community parsing per RFC 8092.
// RFC 8092: Large communities are 12 octets: GlobalAdmin:LocalData1:LocalData2.
func TestParseLargeCommunity(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    LargeCommunity
		wantErr bool
	}{
		// Valid
		{"simple", "2914:100:200", LargeCommunity{GlobalAdmin: 2914, LocalData1: 100, LocalData2: 200}, false},
		{"zeros", "0:0:0", LargeCommunity{GlobalAdmin: 0, LocalData1: 0, LocalData2: 0}, false},
		{"max values", "4294967295:4294967295:4294967295", LargeCommunity{GlobalAdmin: 0xFFFFFFFF, LocalData1: 0xFFFFFFFF, LocalData2: 0xFFFFFFFF}, false},

		// Invalid
		{"missing parts", "2914:100", LargeCommunity{}, true},
		{"too many parts", "2914:100:200:300", LargeCommunity{}, true},
		{"invalid global admin", "abc:100:200", LargeCommunity{}, true},
		{"invalid local data 1", "2914:abc:200", LargeCommunity{}, true},
		{"invalid local data 2", "2914:100:abc", LargeCommunity{}, true},
		{"empty string", "", LargeCommunity{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLargeCommunity(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLargeCommunity(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseLargeCommunity(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseSAFI tests parsing of SAFI from command args.
//
// VALIDATES: SAFI is correctly extracted and validated.
//
// PREVENTS: Invalid SAFI values being accepted.
func TestParseSAFI(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantSAFI string
		wantRest []string
		wantErr  bool
	}{
		// Valid unicast SAFI
		{"unicast lowercase", []string{"unicast", "10.0.0.0/24", "next-hop", "1.2.3.4"}, "unicast", []string{"10.0.0.0/24", "next-hop", "1.2.3.4"}, false},
		{"Unicast mixed case", []string{"Unicast", "10.0.0.0/24"}, "unicast", []string{"10.0.0.0/24"}, false},
		{"UNICAST uppercase", []string{"UNICAST", "2001::/64"}, "unicast", []string{"2001::/64"}, false},

		// Valid mpls-vpn SAFI
		{"mpls-vpn lowercase", []string{"mpls-vpn", "10.0.0.0/24", "rd", "100:100"}, "mpls-vpn", []string{"10.0.0.0/24", "rd", "100:100"}, false},
		{"MPLS-VPN uppercase", []string{"MPLS-VPN", "10.0.0.0/24"}, "mpls-vpn", []string{"10.0.0.0/24"}, false},

		// Invalid
		{"empty", []string{}, "", nil, true},
		{"invalid safi", []string{"multipath", "10.0.0.0/24"}, "", nil, true},
		{"multicast unsupported", []string{"multicast", "10.0.0.0/24"}, "", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			safi, rest, err := parseSAFI(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSAFI(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if safi != tt.wantSAFI {
				t.Errorf("parseSAFI(%v) safi = %q, want %q", tt.args, safi, tt.wantSAFI)
				return
			}
			if len(rest) != len(tt.wantRest) {
				t.Errorf("parseSAFI(%v) rest = %v, want %v", tt.args, rest, tt.wantRest)
				return
			}
			for i, r := range rest {
				if r != tt.wantRest[i] {
					t.Errorf("parseSAFI(%v) rest[%d] = %q, want %q", tt.args, i, r, tt.wantRest[i])
				}
			}
		})
	}
}

// TestParseRouteAttributes_UnicastKeywords tests that only valid keywords are accepted for unicast.
//
// VALIDATES: Unicast routes accept only valid unicast keywords.
//
// PREVENTS: VPN-only keywords (rd, rt, label) being silently ignored for unicast routes.
func TestParseRouteAttributes_UnicastKeywords(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errMsg  string // substring expected in error message
	}{
		// Valid unicast keywords
		{
			name:    "valid: next-hop only",
			args:    []string{"10.0.0.0/24", "next-hop", "1.2.3.4"},
			wantErr: false,
		},
		{
			name:    "valid: all unicast keywords",
			args:    []string{"10.0.0.0/24", "next-hop", "1.2.3.4", "origin", "igp", "med", "100", "local-preference", "200", "as-path", "[65001]", "community", "[2914:666]", "large-community", "[2914:1:2]", "split", "/25"},
			wantErr: false,
		},

		// Invalid: VPN-only keywords should error for unicast
		{
			name:    "invalid: rd not valid for unicast",
			args:    []string{"10.0.0.0/24", "next-hop", "1.2.3.4", "rd", "100:100"},
			wantErr: true,
			errMsg:  "rd",
		},
		{
			name:    "invalid: rt not valid for unicast",
			args:    []string{"10.0.0.0/24", "next-hop", "1.2.3.4", "rt", "100:100"},
			wantErr: true,
			errMsg:  "rt",
		},
		{
			name:    "invalid: label not valid for unicast",
			args:    []string{"10.0.0.0/24", "next-hop", "1.2.3.4", "label", "100"},
			wantErr: true,
			errMsg:  "label",
		},

		// Invalid: unknown keywords should error
		{
			name:    "invalid: unknown keyword",
			args:    []string{"10.0.0.0/24", "next-hop", "1.2.3.4", "foo", "bar"},
			wantErr: true,
			errMsg:  "foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseRouteAttributes(tt.args, UnicastKeywords)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRouteAttributes(%v, UnicastKeywords) error = %v, wantErr %v", tt.args, err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("parseRouteAttributes(%v) error = %v, want error containing %q", tt.args, err, tt.errMsg)
				}
			}
			// Verify parsed result has valid route spec for success cases
			if !tt.wantErr && !parsed.Route.Prefix.IsValid() {
				t.Errorf("parseRouteAttributes(%v) returned invalid prefix", tt.args)
			}
		})
	}
}

// TestParseLabeledUnicastAttributes tests that labeled-unicast routes accept valid keywords.
//
// VALIDATES: Labeled-unicast routes accept unicast keywords plus label.
//
// PREVENTS: VPN-only keywords (rd, rt) being accepted for labeled-unicast.
func TestParseLabeledUnicastAttributes(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errMsg  string // substring expected in error message
	}{
		// Valid MPLS keywords (unicast + label)
		{
			name:    "valid: label and next-hop",
			args:    []string{"10.0.0.0/24", "label", "100", "next-hop", "1.2.3.4"},
			wantErr: false,
		},
		{
			name:    "valid: all MPLS keywords",
			args:    []string{"10.0.0.0/24", "label", "100", "next-hop", "1.2.3.4", "origin", "igp", "med", "100", "local-preference", "200", "as-path", "[65001]", "community", "[2914:666]"},
			wantErr: false,
		},

		// Invalid: VPN-only keywords should error for MPLS labeled-unicast
		{
			name:    "invalid: rd not valid for labeled-unicast",
			args:    []string{"10.0.0.0/24", "label", "100", "next-hop", "1.2.3.4", "rd", "100:100"},
			wantErr: true,
			errMsg:  "rd",
		},
		{
			name:    "invalid: rt not valid for labeled-unicast",
			args:    []string{"10.0.0.0/24", "label", "100", "next-hop", "1.2.3.4", "rt", "100:100"},
			wantErr: true,
			errMsg:  "rt",
		},
		{
			name:    "valid: split supported for labeled-unicast",
			args:    []string{"10.0.0.0/23", "label", "100", "next-hop", "1.2.3.4", "split", "/24"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route, err := parseLabeledUnicastAttributes(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLabeledUnicastAttributes(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("parseLabeledUnicastAttributes(%v) error = %v, want error containing %q", tt.args, err, tt.errMsg)
				}
			}
			// Verify parsed result has valid prefix for success cases
			if !tt.wantErr && !route.Prefix.IsValid() {
				t.Errorf("parseLabeledUnicastAttributes(%v) returned invalid prefix", tt.args)
			}
		})
	}
}

// TestParseLargeCommunities tests multiple large community parsing.
func TestParseLargeCommunities(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantComms    []LargeCommunity
		wantConsumed int
		wantErr      bool
	}{
		// Valid
		{"single", []string{"[2914:1:0]"}, []LargeCommunity{{GlobalAdmin: 2914, LocalData1: 1, LocalData2: 0}}, 1, false},
		{"multiple", []string{"[2914:1:0", "2914:2:0]"}, []LargeCommunity{{GlobalAdmin: 2914, LocalData1: 1, LocalData2: 0}, {GlobalAdmin: 2914, LocalData1: 2, LocalData2: 0}}, 2, false},
		{"empty", []string{"[]"}, nil, 1, false},

		// Single value without brackets (ExaBGP compatible)
		{"single no brackets", []string{"2914:1:0"}, []LargeCommunity{{GlobalAdmin: 2914, LocalData1: 1, LocalData2: 0}}, 1, false},

		// Invalid
		{"invalid format", []string{"[2914:1]"}, nil, 1, true},
		{"empty input", []string{}, nil, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lcomms, consumed, err := parseLargeCommunities(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLargeCommunities(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
				return
			}
			if consumed != tt.wantConsumed {
				t.Errorf("parseLargeCommunities(%v) consumed = %d, want %d", tt.args, consumed, tt.wantConsumed)
			}
			if len(lcomms) != len(tt.wantComms) {
				t.Errorf("parseLargeCommunities(%v) lcomms = %v, want %v", tt.args, lcomms, tt.wantComms)
				return
			}
			for i, lc := range lcomms {
				if lc != tt.wantComms[i] {
					t.Errorf("parseLargeCommunities(%v) lcomms[%d] = %+v, want %+v", tt.args, i, lc, tt.wantComms[i])
				}
			}
		})
	}
}

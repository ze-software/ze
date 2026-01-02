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

		// Bare integers (ExaBGP compatible)
		{"bare integer zero", "0", 0, false},
		{"bare integer", "2914666", 2914666, false},
		{"bare integer max", "4294967295", 0xFFFFFFFF, false},

		// Hex format (ExaBGP compatible)
		{"hex format", "0x12345678", 0x12345678, false},

		// Invalid
		{"too many colons", "2914:666:1", 0, true},
		{"invalid ASN", "abc:666", 0, true},
		{"invalid value", "2914:abc", 0, true},
		{"ASN too large", "65536:1", 0, true},
		{"value too large", "1:65536", 0, true},
		{"empty string", "", 0, true},
		{"invalid hex", "0xGGGG", 0, true},
		{"bare integer overflow", "4294967296", 0, true},
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

		// Valid MUP SAFI
		{"mup lowercase", []string{"mup", "mup-isd", "10.0.0.0/24", "rd", "100:100"}, "mup", []string{"mup-isd", "10.0.0.0/24", "rd", "100:100"}, false},
		{"MUP uppercase", []string{"MUP", "mup-dsd", "10.0.0.1"}, "mup", []string{"mup-dsd", "10.0.0.1"}, false},

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
		// ADD-PATH path-id (RFC 7911)
		{
			name:    "valid: path-id for ADD-PATH",
			args:    []string{"10.0.0.0/24", "label", "100", "next-hop", "1.2.3.4", "path-id", "42"},
			wantErr: false,
		},
		{
			name:    "invalid: path-id missing value",
			args:    []string{"10.0.0.0/24", "label", "100", "next-hop", "1.2.3.4", "path-id"},
			wantErr: true,
			errMsg:  "missing path-id",
		},
		{
			name:    "invalid: path-id not a number",
			args:    []string{"10.0.0.0/24", "label", "100", "next-hop", "1.2.3.4", "path-id", "abc"},
			wantErr: true,
			errMsg:  "invalid path-id",
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

// TestParseLabeledUnicastPathID verifies ADD-PATH path-id parsing.
//
// VALIDATES: RFC 7911 path-id is correctly parsed and stored.
//
// PREVENTS: ADD-PATH functionality not working due to parse errors.
func TestParseLabeledUnicastPathID(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantPathID uint32
	}{
		{
			name:       "path-id 0 (no path-id specified)",
			args:       []string{"10.0.0.0/24", "label", "100", "next-hop", "1.2.3.4"},
			wantPathID: 0,
		},
		{
			name:       "path-id 1",
			args:       []string{"10.0.0.0/24", "label", "100", "next-hop", "1.2.3.4", "path-id", "1"},
			wantPathID: 1,
		},
		{
			name:       "path-id 42",
			args:       []string{"10.0.0.0/24", "label", "100", "next-hop", "1.2.3.4", "path-id", "42"},
			wantPathID: 42,
		},
		{
			name:       "path-id max uint32",
			args:       []string{"10.0.0.0/24", "label", "100", "next-hop", "1.2.3.4", "path-id", "4294967295"},
			wantPathID: 4294967295,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route, err := parseLabeledUnicastAttributes(tt.args)
			if err != nil {
				t.Fatalf("parseLabeledUnicastAttributes(%v) unexpected error: %v", tt.args, err)
			}
			if route.PathID != tt.wantPathID {
				t.Errorf("parseLabeledUnicastAttributes(%v) PathID = %d, want %d", tt.args, route.PathID, tt.wantPathID)
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

// TestParseAttributesNLRI tests the ExaBGP-compatible attributes...nlri syntax.
//
// VALIDATES: Attributes are parsed correctly before the nlri keyword.
// PREVENTS: Missing nlri keyword or invalid prefix parsing.
func TestParseAttributesNLRI(t *testing.T) {
	tests := []struct {
		name                 string
		args                 []string
		wantPrefixes         int
		wantNextHop          string
		wantOrigin           *uint8
		wantLP               *uint32
		wantMED              *uint32
		wantASPath           []uint32
		wantCommunities      int
		wantLargeCommunities int
		wantErr              bool
		errContains          string
	}{
		// Valid cases
		{
			name:         "basic next-hop and nlri",
			args:         strings.Fields("next-hop 10.0.0.1 nlri 1.0.0.0/24"),
			wantPrefixes: 1,
			wantNextHop:  "10.0.0.1",
			wantErr:      false,
		},
		{
			name:         "multiple prefixes",
			args:         strings.Fields("next-hop 10.0.0.1 nlri 1.0.0.0/24 2.0.0.0/24 3.0.0.0/24"),
			wantPrefixes: 3,
			wantNextHop:  "10.0.0.1",
			wantErr:      false,
		},
		{
			name:         "with origin and local-preference",
			args:         strings.Fields("next-hop 10.0.0.1 origin igp local-preference 100 nlri 1.0.0.0/24"),
			wantPrefixes: 1,
			wantNextHop:  "10.0.0.1",
			wantOrigin:   ptr(uint8(0)),
			wantLP:       ptr(uint32(100)),
			wantErr:      false,
		},
		{
			name:         "with MED",
			args:         strings.Fields("next-hop 10.0.0.1 med 500 nlri 1.0.0.0/24"),
			wantPrefixes: 1,
			wantNextHop:  "10.0.0.1",
			wantMED:      ptr(uint32(500)),
			wantErr:      false,
		},
		{
			name:         "with AS-PATH",
			args:         strings.Fields("next-hop 10.0.0.1 as-path [ 100 200 300 ] nlri 1.0.0.0/24"),
			wantPrefixes: 1,
			wantNextHop:  "10.0.0.1",
			wantASPath:   []uint32{100, 200, 300},
			wantErr:      false,
		},
		{
			name:            "with community",
			args:            strings.Fields("next-hop 10.0.0.1 community [2914:666] nlri 1.0.0.0/24"),
			wantPrefixes:    1,
			wantNextHop:     "10.0.0.1",
			wantCommunities: 1,
			wantErr:         false,
		},
		{
			name:                 "with large-community",
			args:                 strings.Fields("next-hop 10.0.0.1 large-community [65000:1:2] nlri 1.0.0.0/24"),
			wantPrefixes:         1,
			wantNextHop:          "10.0.0.1",
			wantLargeCommunities: 1,
			wantErr:              false,
		},
		{
			name:                 "with multiple large-communities",
			args:                 strings.Fields("next-hop 10.0.0.1 large-community [65000:1:2 65001:3:4] nlri 1.0.0.0/24"),
			wantPrefixes:         1,
			wantNextHop:          "10.0.0.1",
			wantLargeCommunities: 2,
			wantErr:              false,
		},
		{
			name:                 "all attributes combined",
			args:                 strings.Fields("next-hop 10.0.0.1 origin egp local-preference 200 med 100 as-path [ 1 2 ] community [2914:666] large-community [65000:1:2] nlri 1.0.0.0/24"),
			wantPrefixes:         1,
			wantNextHop:          "10.0.0.1",
			wantOrigin:           ptr(uint8(1)), // EGP
			wantLP:               ptr(uint32(200)),
			wantMED:              ptr(uint32(100)),
			wantASPath:           []uint32{1, 2},
			wantCommunities:      1,
			wantLargeCommunities: 1,
			wantErr:              false,
		},

		// Invalid cases
		{
			name:        "missing nlri keyword",
			args:        strings.Fields("next-hop 10.0.0.1 1.0.0.0/24"),
			wantErr:     true,
			errContains: "nlri",
		},
		{
			name:         "no prefixes after nlri",
			args:         strings.Fields("next-hop 10.0.0.1 nlri"),
			wantErr:      false, // parsing succeeds, but returns 0 prefixes
			wantPrefixes: 0,
			wantNextHop:  "10.0.0.1",
		},
		{
			name:        "invalid prefix after nlri",
			args:        strings.Fields("next-hop 10.0.0.1 nlri invalid"),
			wantErr:     true,
			errContains: "invalid prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs, prefixes, err := parseAttributesNLRI(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseAttributesNLRI(%v) expected error containing %q", tt.args, tt.errContains)
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("parseAttributesNLRI(%v) error = %v, want containing %q", tt.args, err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("parseAttributesNLRI(%v) unexpected error = %v", tt.args, err)
				return
			}
			if len(prefixes) != tt.wantPrefixes {
				t.Errorf("parseAttributesNLRI(%v) prefixes = %d, want %d", tt.args, len(prefixes), tt.wantPrefixes)
			}
			if tt.wantNextHop != "" && attrs.NextHop.String() != tt.wantNextHop {
				t.Errorf("parseAttributesNLRI(%v) NextHop = %v, want %v", tt.args, attrs.NextHop, tt.wantNextHop)
			}
			if tt.wantOrigin != nil && (attrs.Origin == nil || *attrs.Origin != *tt.wantOrigin) {
				t.Errorf("parseAttributesNLRI(%v) Origin = %v, want %v", tt.args, attrs.Origin, tt.wantOrigin)
			}
			if tt.wantLP != nil && (attrs.LocalPreference == nil || *attrs.LocalPreference != *tt.wantLP) {
				t.Errorf("parseAttributesNLRI(%v) LocalPreference = %v, want %v", tt.args, attrs.LocalPreference, tt.wantLP)
			}
			if tt.wantMED != nil && (attrs.MED == nil || *attrs.MED != *tt.wantMED) {
				t.Errorf("parseAttributesNLRI(%v) MED = %v, want %v", tt.args, attrs.MED, tt.wantMED)
			}
			if tt.wantASPath != nil {
				if len(attrs.ASPath) != len(tt.wantASPath) {
					t.Errorf("parseAttributesNLRI(%v) ASPath len = %d, want %d", tt.args, len(attrs.ASPath), len(tt.wantASPath))
				} else {
					for i, asn := range tt.wantASPath {
						if attrs.ASPath[i] != asn {
							t.Errorf("parseAttributesNLRI(%v) ASPath[%d] = %d, want %d", tt.args, i, attrs.ASPath[i], asn)
						}
					}
				}
			}
			if tt.wantCommunities > 0 && len(attrs.Communities) != tt.wantCommunities {
				t.Errorf("parseAttributesNLRI(%v) Communities = %d, want %d", tt.args, len(attrs.Communities), tt.wantCommunities)
			}
			if tt.wantLargeCommunities > 0 && len(attrs.LargeCommunities) != tt.wantLargeCommunities {
				t.Errorf("parseAttributesNLRI(%v) LargeCommunities = %d, want %d", tt.args, len(attrs.LargeCommunities), tt.wantLargeCommunities)
			}
		})
	}
}

// TestParseUpdateCommand tests the ZeBGP announce update syntax.
//
// VALIDATES: AFI/SAFI is correctly parsed from the command.
// PREVENTS: Invalid AFI/SAFI or missing family specification.
func TestParseUpdateCommand(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantAFI      string
		wantSAFI     string
		wantPrefixes int
		wantNextHop  string
		wantErr      bool
		errContains  string
	}{
		// Valid cases
		{
			name:         "ipv4 unicast",
			args:         strings.Fields("next-hop 10.0.0.1 ipv4 unicast 1.0.0.0/24"),
			wantAFI:      "ipv4",
			wantSAFI:     "unicast",
			wantPrefixes: 1,
			wantNextHop:  "10.0.0.1",
			wantErr:      false,
		},
		{
			name:         "ipv6 unicast",
			args:         strings.Fields("next-hop 2001::1 ipv6 unicast 2001:db8::/32"),
			wantAFI:      "ipv6",
			wantSAFI:     "unicast",
			wantPrefixes: 1,
			wantNextHop:  "2001::1",
			wantErr:      false,
		},
		{
			name:         "with optional nlri keyword",
			args:         strings.Fields("next-hop 10.0.0.1 ipv4 unicast nlri 1.0.0.0/24 2.0.0.0/24"),
			wantAFI:      "ipv4",
			wantSAFI:     "unicast",
			wantPrefixes: 2,
			wantNextHop:  "10.0.0.1",
			wantErr:      false,
		},
		{
			name:         "with attributes before afi",
			args:         strings.Fields("next-hop 10.0.0.1 origin igp local-preference 100 ipv4 unicast 1.0.0.0/24"),
			wantAFI:      "ipv4",
			wantSAFI:     "unicast",
			wantPrefixes: 1,
			wantNextHop:  "10.0.0.1",
			wantErr:      false,
		},

		// Invalid cases
		{
			name:        "missing AFI",
			args:        strings.Fields("next-hop 10.0.0.1 1.0.0.0/24"),
			wantErr:     true,
			errContains: "AFI",
		},
		{
			name:        "missing SAFI",
			args:        strings.Fields("next-hop 10.0.0.1 ipv4"),
			wantErr:     true,
			errContains: "SAFI",
		},
		{
			name:        "invalid SAFI",
			args:        strings.Fields("next-hop 10.0.0.1 ipv4 vpn 1.0.0.0/24"),
			wantErr:     true,
			errContains: "SAFI",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs, afi, safi, prefixes, err := parseUpdateCommand(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseUpdateCommand(%v) expected error containing %q", tt.args, tt.errContains)
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("parseUpdateCommand(%v) error = %v, want containing %q", tt.args, err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("parseUpdateCommand(%v) unexpected error = %v", tt.args, err)
				return
			}
			if afi != tt.wantAFI {
				t.Errorf("parseUpdateCommand(%v) AFI = %v, want %v", tt.args, afi, tt.wantAFI)
			}
			if safi != tt.wantSAFI {
				t.Errorf("parseUpdateCommand(%v) SAFI = %v, want %v", tt.args, safi, tt.wantSAFI)
			}
			if len(prefixes) != tt.wantPrefixes {
				t.Errorf("parseUpdateCommand(%v) prefixes = %d, want %d", tt.args, len(prefixes), tt.wantPrefixes)
			}
			if tt.wantNextHop != "" && attrs.NextHop.String() != tt.wantNextHop {
				t.Errorf("parseUpdateCommand(%v) NextHop = %v, want %v", tt.args, attrs.NextHop, tt.wantNextHop)
			}
		})
	}
}

// ptr returns a pointer to the value. Helper for test cases.
func ptr[T any](v T) *T {
	return &v
}

// TestParseExtendedCommunity tests extended community parsing per RFC 4360/5575.
// Extended communities are 8 octets: Type:Subtype:Value
//
// VALIDATES: Extended community strings are correctly parsed to [8]byte.
// PREVENTS: Invalid format or incorrect byte encoding.
func TestParseExtendedCommunity(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    [8]byte
		wantErr bool
	}{
		// Origin extended communities (RFC 4360, RFC 7153)
		// Type 0x00: 2-byte ASN + 4-byte IPv4
		{
			name:  "origin with 2-byte ASN and IPv4",
			input: "origin:2345:6.7.8.9",
			want:  [8]byte{0x00, 0x03, 0x09, 0x29, 0x06, 0x07, 0x08, 0x09}, // Type=0, Subtype=3, ASN=2345 (0x0929), IP=6.7.8.9
		},
		// Type 0x01: IPv4 + 2-byte ASN
		{
			name:  "origin with IPv4 and 2-byte ASN",
			input: "origin:2.3.4.5:6789",
			want:  [8]byte{0x01, 0x03, 0x02, 0x03, 0x04, 0x05, 0x1A, 0x85}, // Type=1, Subtype=3, IP=2.3.4.5, ASN=6789 (0x1A85)
		},

		// Traffic redirect (RFC 5575, RFC 7674)
		// Type 0x80: 2-byte ASN + 4-byte target
		{
			name:  "redirect with 2-byte ASN",
			input: "redirect:65500:12345",
			want:  [8]byte{0x80, 0x08, 0xFF, 0xDC, 0x00, 0x00, 0x30, 0x39}, // Type=0x80, Subtype=8, ASN=65500 (0xFFDC), Target=12345 (0x3039)
		},
		{
			name:  "redirect with small values",
			input: "redirect:65001:119",
			want:  [8]byte{0x80, 0x08, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x77}, // ASN=65001 (0xFDE9), Target=119 (0x77)
		},

		// Traffic rate limit (RFC 5575)
		// Type 0x80, Subtype 0x06: rate limit in IEEE 754 float
		{
			name:  "rate-limit",
			input: "rate-limit:1250000000",
			want:  [8]byte{0x80, 0x06, 0x00, 0x00, 0x4E, 0x95, 0x02, 0xF9}, // Rate as IEEE 754 float
		},

		// Invalid cases
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "unknown type",
			input:   "unknown:1:2",
			wantErr: true,
		},
		{
			name:    "invalid origin format",
			input:   "origin:invalid",
			wantErr: true,
		},
		{
			name:    "missing colon",
			input:   "redirect65500:12345",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseExtendedCommunity(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseExtendedCommunity(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseExtendedCommunity(%q) = %x, want %x", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseExtendedCommunities tests parsing multiple extended communities.
func TestParseExtendedCommunities(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantCount    int
		wantConsumed int
		wantErr      bool
	}{
		// Valid
		{"single", []string{"[origin:2345:6.7.8.9]"}, 1, 1, false},
		{"multiple", []string{"[origin:2345:6.7.8.9", "redirect:65500:12345]"}, 2, 2, false},
		{"empty", []string{"[]"}, 0, 1, false},

		// Single value without brackets (ExaBGP compatible)
		{"single no brackets", []string{"redirect:65500:12345"}, 1, 1, false},

		// Invalid
		{"invalid community", []string{"[invalid:1:2]"}, 0, 1, true},
		{"empty input", []string{}, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comms, consumed, err := parseExtendedCommunities(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseExtendedCommunities(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
				return
			}
			if consumed != tt.wantConsumed {
				t.Errorf("parseExtendedCommunities(%v) consumed = %d, want %d", tt.args, consumed, tt.wantConsumed)
			}
			if len(comms) != tt.wantCount {
				t.Errorf("parseExtendedCommunities(%v) count = %d, want %d", tt.args, len(comms), tt.wantCount)
			}
		})
	}
}

// TestParseAttributesNlri tests parsing of 'announce attributes ... nlri' syntax.
//
// VALIDATES: The attributes and NLRI list are correctly parsed from the command string.
// ExaBGP syntax: 'announce attributes med 100 next-hop 1.2.3.4 nlri 10.0.0.1/32 10.0.0.2/32'
//
// PREVENTS: Incorrect parsing that would cause attribute/NLRI mismatch or missing routes.
func TestParseAttributesNlri(t *testing.T) {
	// Helper to create uint32 pointer
	u32 := func(v uint32) *uint32 { return &v }
	u8 := func(v uint8) *uint8 { return &v }

	tests := []struct {
		name        string
		args        string
		wantNextHop string
		wantMED     *uint32
		wantLP      *uint32  // LocalPreference
		wantOrigin  *uint8   // 0=IGP, 1=EGP, 2=INCOMPLETE
		wantASPath  []uint32 // nil means not checked
		wantComms   []uint32 // Communities
		wantNlris   []string
		wantErr     bool
		errContains string
	}{
		{
			name:        "med and next-hop with two nlris",
			args:        "med 100 next-hop 101.1.101.1 nlri 1.0.0.1/32 1.0.0.2/32",
			wantNextHop: "101.1.101.1",
			wantMED:     u32(100),
			wantNlris:   []string{"1.0.0.1/32", "1.0.0.2/32"},
		},
		{
			name:        "local-preference and as-path with two nlris",
			args:        "local-preference 200 as-path [ 1 2 3 4 ] next-hop 202.2.202.2 nlri 2.0.0.1/32 2.0.0.2/32",
			wantNextHop: "202.2.202.2",
			wantLP:      u32(200),
			wantASPath:  []uint32{1, 2, 3, 4},
			wantNlris:   []string{"2.0.0.1/32", "2.0.0.2/32"},
		},
		{
			name:        "origin explicit egp",
			args:        "origin egp next-hop 1.2.3.4 nlri 10.0.0.0/8",
			wantNextHop: "1.2.3.4",
			wantOrigin:  u8(1), // EGP = 1
			wantNlris:   []string{"10.0.0.0/8"},
		},
		{
			name:        "communities",
			args:        "next-hop 1.2.3.4 community [ 65000:1 65000:2 ] nlri 10.0.0.0/24",
			wantNextHop: "1.2.3.4",
			wantComms:   []uint32{0xFDE80001, 0xFDE80002}, // 65000:1, 65000:2
			wantNlris:   []string{"10.0.0.0/24"},
		},
		{
			name:        "missing nlri keyword",
			args:        "med 100 next-hop 1.2.3.4 10.0.0.1/32",
			wantErr:     true,
			errContains: "nlri", // Function returns error when 'nlri' keyword not found
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := strings.Fields(tt.args)
			attrs, nlris, err := parseAttributesNLRI(args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseAttributesNLRI(%q) expected error containing %q, got nil", tt.args, tt.errContains)
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("parseAttributesNLRI(%q) error = %q, want error containing %q", tt.args, err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("parseAttributesNLRI(%q) unexpected error: %v", tt.args, err)
				return
			}

			// Check next-hop
			wantNH := netip.MustParseAddr(tt.wantNextHop)
			if attrs.NextHop != wantNH {
				t.Errorf("parseAttributesNLRI(%q) NextHop = %v, want %v", tt.args, attrs.NextHop, wantNH)
			}

			// Check MED (pointer comparison)
			if tt.wantMED != nil {
				if attrs.MED == nil {
					t.Errorf("parseAttributesNLRI(%q) MED = nil, want %d", tt.args, *tt.wantMED)
				} else if *attrs.MED != *tt.wantMED {
					t.Errorf("parseAttributesNLRI(%q) MED = %d, want %d", tt.args, *attrs.MED, *tt.wantMED)
				}
			}

			// Check LocalPreference (pointer comparison)
			if tt.wantLP != nil {
				if attrs.LocalPreference == nil {
					t.Errorf("parseAttributesNLRI(%q) LocalPreference = nil, want %d", tt.args, *tt.wantLP)
				} else if *attrs.LocalPreference != *tt.wantLP {
					t.Errorf("parseAttributesNLRI(%q) LocalPreference = %d, want %d", tt.args, *attrs.LocalPreference, *tt.wantLP)
				}
			}

			// Check Origin (pointer comparison)
			if tt.wantOrigin != nil {
				if attrs.Origin == nil {
					t.Errorf("parseAttributesNLRI(%q) Origin = nil, want %d", tt.args, *tt.wantOrigin)
				} else if *attrs.Origin != *tt.wantOrigin {
					t.Errorf("parseAttributesNLRI(%q) Origin = %d, want %d", tt.args, *attrs.Origin, *tt.wantOrigin)
				}
			}

			// Check ASPath
			if tt.wantASPath != nil {
				if len(attrs.ASPath) != len(tt.wantASPath) {
					t.Errorf("parseAttributesNLRI(%q) ASPath = %v, want %v", tt.args, attrs.ASPath, tt.wantASPath)
				} else {
					for i, asn := range attrs.ASPath {
						if asn != tt.wantASPath[i] {
							t.Errorf("parseAttributesNLRI(%q) ASPath[%d] = %d, want %d", tt.args, i, asn, tt.wantASPath[i])
						}
					}
				}
			}

			// Check Communities
			if tt.wantComms != nil {
				if len(attrs.Communities) != len(tt.wantComms) {
					t.Errorf("parseAttributesNLRI(%q) Communities = %v, want %v", tt.args, attrs.Communities, tt.wantComms)
				} else {
					for i, comm := range attrs.Communities {
						if comm != tt.wantComms[i] {
							t.Errorf("parseAttributesNLRI(%q) Communities[%d] = %08x, want %08x", tt.args, i, comm, tt.wantComms[i])
						}
					}
				}
			}

			// Check NLRIs
			if len(nlris) != len(tt.wantNlris) {
				t.Errorf("parseAttributesNLRI(%q) nlris count = %d, want %d", tt.args, len(nlris), len(tt.wantNlris))
			} else {
				for i, prefix := range nlris {
					if prefix.String() != tt.wantNlris[i] {
						t.Errorf("parseAttributesNLRI(%q) nlris[%d] = %s, want %s", tt.args, i, prefix.String(), tt.wantNlris[i])
					}
				}
			}
		})
	}
}

// TestParseFlowSpecArgs_InvalidKeywords tests that invalid keywords are rejected.
//
// VALIDATES: FlowSpec parser rejects unknown match/then keywords.
//
// PREVENTS: Silent ignoring of typos/unknown keywords in FlowSpec rules.
func TestParseFlowSpecArgs_InvalidKeywords(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errMsg  string
	}{
		// Valid keywords should work
		{
			name:    "valid match keywords",
			args:    strings.Fields("match destination 10.0.0.0/24 then discard"),
			wantErr: false,
		},
		{
			name:    "valid match source",
			args:    strings.Fields("match source 192.168.1.0/24 then accept"),
			wantErr: false,
		},
		{
			name:    "valid match protocol",
			args:    strings.Fields("match destination 10.0.0.0/24 protocol tcp then discard"),
			wantErr: false,
		},
		{
			name:    "valid then rate-limit",
			args:    strings.Fields("match destination 10.0.0.0/24 then rate-limit 1000000"),
			wantErr: false,
		},

		// Invalid: keyword before match/then section
		{
			name:    "invalid: unknown keyword before match",
			args:    strings.Fields("bogus match destination 10.0.0.0/24 then discard"),
			wantErr: true,
			errMsg:  "unknown keyword",
		},
		{
			name:    "invalid: match keyword before match",
			args:    strings.Fields("destination 10.0.0.0/24 match then discard"),
			wantErr: true,
			errMsg:  "match keyword",
		},
		{
			name:    "invalid: then keyword before then",
			args:    strings.Fields("discard match destination 10.0.0.0/24 then"),
			wantErr: true,
			errMsg:  "then keyword",
		},

		// Invalid match keywords should error
		{
			name:    "invalid: unknown match keyword",
			args:    strings.Fields("match bogus-keyword 10.0.0.0/24 then discard"),
			wantErr: true,
			errMsg:  "bogus-keyword",
		},
		{
			name:    "invalid: typo in destination",
			args:    strings.Fields("match destnation 10.0.0.0/24 then discard"),
			wantErr: true,
			errMsg:  "destnation",
		},

		// Invalid then keywords should error
		{
			name:    "invalid: unknown then keyword",
			args:    strings.Fields("match destination 10.0.0.0/24 then unknown-action"),
			wantErr: true,
			errMsg:  "unknown-action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseFlowSpecArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFlowSpecArgs(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ParseFlowSpecArgs(%v) error = %v, want error containing %q", tt.args, err, tt.errMsg)
				}
			}
		})
	}
}

// keywordTestCase is a test case for keyword validation.
type keywordTestCase struct {
	name, errMsg string
	args         []string
	wantErr      bool
}

// runKeywordTests runs a set of keyword validation tests with the given parser.
func runKeywordTests(t *testing.T, parserName string, parser func([]string) error, tests []keywordTestCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("%s(%v) error = %v, wantErr %v", parserName, tt.args, err, tt.wantErr)
			} else if tt.wantErr && tt.errMsg != "" && (err == nil || !strings.Contains(err.Error(), tt.errMsg)) {
				t.Errorf("%s(%v) error = %v, want containing %q", parserName, tt.args, err, tt.errMsg)
			}
		})
	}
}

// TestParseVPLSArgs_InvalidKeywords tests that invalid keywords are rejected.
//
// VALIDATES: VPLS parser rejects unknown keywords.
//
// PREVENTS: Silent ignoring of typos/unknown keywords in VPLS routes.
func TestParseVPLSArgs_InvalidKeywords(t *testing.T) {
	runKeywordTests(t, "ParseVPLSArgs", func(args []string) error { _, err := ParseVPLSArgs(args); return err }, []keywordTestCase{
		{"valid minimal", "", strings.Fields("rd 100:100 next-hop 1.2.3.4"), false},
		{"valid with ve-block", "", strings.Fields("rd 100:100 ve-block-offset 1 ve-block-size 10 next-hop 1.2.3.4"), false},
		{"valid with label", "", strings.Fields("rd 100:100 label 1000 next-hop 1.2.3.4"), false},
		{"invalid: unknown keyword", "bogus-key", strings.Fields("rd 100:100 bogus-key value next-hop 1.2.3.4"), true},
		{"invalid: typo in next-hop", "nexthop", strings.Fields("rd 100:100 nexthop 1.2.3.4"), true},
	})
}

// TestParseL2VPNArgs_InvalidKeywords tests that invalid keywords are rejected.
//
// VALIDATES: L2VPN parser rejects unknown keywords.
//
// PREVENTS: Silent ignoring of typos/unknown keywords in L2VPN/EVPN routes.
func TestParseL2VPNArgs_InvalidKeywords(t *testing.T) {
	runKeywordTests(t, "parseL2VPNArgs", func(args []string) error { _, err := parseL2VPNArgs(args); return err }, []keywordTestCase{
		{"valid mac-ip", "", strings.Fields("mac-ip rd 100:100 mac 00:11:22:33:44:55 label 1000 next-hop 1.2.3.4"), false},
		{"valid ip-prefix", "", strings.Fields("ip-prefix rd 100:100 prefix 10.0.0.0/24 label 1000 next-hop 1.2.3.4"), false},
		{"valid with esi", "", strings.Fields("mac-ip rd 100:100 mac 00:11:22:33:44:55 esi 00:00:00:00:00:00:00:00:00:00 label 1000 next-hop 1.2.3.4"), false},
		{"invalid: unknown keyword", "bogus-key", strings.Fields("mac-ip rd 100:100 mac 00:11:22:33:44:55 bogus-key value next-hop 1.2.3.4"), true},
		{"invalid: typo in ethernet-tag", "ethernettag", strings.Fields("mac-ip rd 100:100 mac 00:11:22:33:44:55 ethernettag 100 next-hop 1.2.3.4"), true},
	})
}

// TestParseParenthesizedValue tests parsing of parenthesis-delimited values.
//
// VALIDATES: Parenthesized values are correctly extracted from args.
//
// PREVENTS: bgp-prefix-sid-srv6 ( ... ) not being parsed correctly.
func TestParseParenthesizedValue(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantValue    string
		wantConsumed int
		wantErr      bool
	}{
		{
			name:         "simple parenthesized",
			args:         []string{"(", "l3-service", "2001::1", "0x48", ")"},
			wantValue:    "l3-service 2001::1 0x48",
			wantConsumed: 5,
		},
		{
			name:         "with SID structure",
			args:         []string{"(", "l3-service", "2001:db8:1:1::", "0x48", "[64,24,16,0,0,0]", ")"},
			wantValue:    "l3-service 2001:db8:1:1:: 0x48 [64,24,16,0,0,0]",
			wantConsumed: 6,
		},
		{
			name:    "unclosed parenthesis",
			args:    []string{"(", "l3-service", "2001::1"},
			wantErr: true,
		},
		{
			name:    "empty args",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "not starting with paren",
			args:    []string{"l3-service", "2001::1", ")"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, consumed, err := parseParenthesizedValue(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseParenthesizedValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if value != tt.wantValue {
				t.Errorf("parseParenthesizedValue() value = %q, want %q", value, tt.wantValue)
			}
			if consumed != tt.wantConsumed {
				t.Errorf("parseParenthesizedValue() consumed = %d, want %d", consumed, tt.wantConsumed)
			}
		})
	}
}

// TestParseMUPArgs tests MUP route argument parsing.
//
// VALIDATES: MUP route types and attributes are correctly parsed.
//
// PREVENTS: MUP API commands not producing correct route specs.
func TestParseMUPArgs(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		isIPv6        bool
		wantRouteType string
		wantPrefix    string
		wantRD        string
		wantNextHop   string
		wantErr       bool
	}{
		{
			name:          "mup-isd IPv4",
			args:          strings.Fields("mup-isd 10.0.1.0/24 rd 100:100 next-hop 2001::1"),
			isIPv6:        false,
			wantRouteType: "mup-isd",
			wantPrefix:    "10.0.1.0/24",
			wantRD:        "100:100",
			wantNextHop:   "2001::1",
		},
		{
			name:          "mup-dsd IPv4",
			args:          strings.Fields("mup-dsd 10.0.0.1 rd 200:200 next-hop 2001::2"),
			isIPv6:        false,
			wantRouteType: "mup-dsd",
			wantPrefix:    "10.0.0.1",
			wantRD:        "200:200",
			wantNextHop:   "2001::2",
		},
		{
			name:          "mup-isd IPv6",
			args:          strings.Fields("mup-isd 2001:db8::/32 rd 100:100 next-hop 2001::1"),
			isIPv6:        true,
			wantRouteType: "mup-isd",
			wantPrefix:    "2001:db8::/32",
			wantRD:        "100:100",
			wantNextHop:   "2001::1",
		},
		{
			name:    "missing route type",
			args:    strings.Fields("10.0.1.0/24 rd 100:100"),
			wantErr: true,
		},
		{
			name:    "invalid route type",
			args:    strings.Fields("mup-invalid 10.0.1.0/24 rd 100:100"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := ParseMUPArgs(tt.args, tt.isIPv6)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMUPArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if spec.RouteType != tt.wantRouteType {
				t.Errorf("ParseMUPArgs() RouteType = %q, want %q", spec.RouteType, tt.wantRouteType)
			}
			if spec.Prefix != tt.wantPrefix && spec.Address != tt.wantPrefix {
				t.Errorf("ParseMUPArgs() Prefix/Address = %q/%q, want %q", spec.Prefix, spec.Address, tt.wantPrefix)
			}
			if spec.RD != tt.wantRD {
				t.Errorf("ParseMUPArgs() RD = %q, want %q", spec.RD, tt.wantRD)
			}
			if spec.NextHop != tt.wantNextHop {
				t.Errorf("ParseMUPArgs() NextHop = %q, want %q", spec.NextHop, tt.wantNextHop)
			}
		})
	}
}

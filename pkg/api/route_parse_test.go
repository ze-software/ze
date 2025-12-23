package api

import (
	"testing"
)

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

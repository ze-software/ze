package peer

import "testing"

// VALIDATES: matchRule supports exact, prefix, and contains modes.
// PREVENTS: prefix/contains matching broken or case-sensitive.
func TestMatchRule(t *testing.T) {
	tests := []struct {
		name     string
		check    string
		received string
		want     bool
	}{
		{"exact_match", "AABBCC", "AABBCC", true},
		{"exact_case_insensitive", "aabbcc", "AABBCC", true},
		{"exact_mismatch", "AABBCC", "AABBDD", false},
		{"prefix_match", "prefix:AABB", "AABBCC", true},
		{"prefix_case_insensitive", "prefix:aabb", "AABBCC", true},
		{"prefix_mismatch", "prefix:CCDD", "AABBCC", false},
		{"prefix_full", "prefix:AABBCC", "AABBCC", true},
		{"contains_match", "contains:BBCC", "AABBCCDD", true},
		{"contains_case_insensitive", "contains:bbcc", "AABBCCDD", true},
		{"contains_mismatch", "contains:EEFF", "AABBCCDD", false},
		{"contains_at_start", "contains:AABB", "AABBCCDD", true},
		{"contains_at_end", "contains:CCDD", "AABBCCDD", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchRule(tt.check, tt.received)
			if got != tt.want {
				t.Errorf("matchRule(%q, %q) = %v, want %v", tt.check, tt.received, got, tt.want)
			}
		})
	}
}

// VALIDATES: parseExpectRule handles prefix= and contains= in expect=bgp lines.
// PREVENTS: New syntax rejected by parser.
func TestParseExpectRule_PrefixContains(t *testing.T) {
	tests := []struct {
		name        string
		rule        string
		wantConn    int
		wantSeq     int
		wantContent string
		wantErr     bool
	}{
		{
			name:        "hex",
			rule:        "expect=bgp:conn=1:seq=1:hex=AABBCC",
			wantConn:    1,
			wantSeq:     1,
			wantContent: "AABBCC",
		},
		{
			name:        "prefix",
			rule:        "expect=bgp:conn=2:seq=1:prefix=AABB",
			wantConn:    2,
			wantSeq:     1,
			wantContent: "prefix:AABB",
		},
		{
			name:        "contains",
			rule:        "expect=bgp:conn=1:seq=2:contains=CCDD",
			wantConn:    1,
			wantSeq:     2,
			wantContent: "contains:CCDD",
		},
		{
			name:    "missing_all",
			rule:    "expect=bgp:conn=1:seq=1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, seq, content, err := parseExpectRule(tt.rule)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if conn != tt.wantConn {
				t.Errorf("conn = %d, want %d", conn, tt.wantConn)
			}
			if seq != tt.wantSeq {
				t.Errorf("seq = %d, want %d", seq, tt.wantSeq)
			}
			if content != tt.wantContent {
				t.Errorf("content = %q, want %q", content, tt.wantContent)
			}
		})
	}
}

package rib

import (
	"testing"
)

// TestBestPath_SingleCandidate verifies a single candidate always wins.
//
// VALIDATES: Best-path with one candidate returns it.
// PREVENTS: Nil return when exactly one candidate exists.
func TestBestPath_SingleCandidate(t *testing.T) {
	c := &Candidate{
		PeerAddr:  "10.0.0.1",
		LocalPref: 100,
	}
	best := SelectBest([]*Candidate{c})
	if best != c {
		t.Errorf("single candidate should win, got %v", best)
	}
}

// TestBestPath_Empty verifies no candidates returns nil.
//
// VALIDATES: Empty candidate list returns nil.
// PREVENTS: Panic on empty input.
func TestBestPath_Empty(t *testing.T) {
	best := SelectBest(nil)
	if best != nil {
		t.Errorf("empty candidates should return nil, got %v", best)
	}
	best = SelectBest([]*Candidate{})
	if best != nil {
		t.Errorf("zero-length candidates should return nil, got %v", best)
	}
}

// TestBestPath_LocalPref verifies higher LOCAL_PREF wins (RFC 4271 §9.1.2 step 1).
//
// VALIDATES: AC-1 — LOCAL_PREF comparison selects highest.
// PREVENTS: Inverted LOCAL_PREF comparison (lower wins instead of higher).
func TestBestPath_LocalPref(t *testing.T) {
	tests := []struct {
		name     string
		a, b     *Candidate
		wantAddr string
	}{
		{
			name:     "higher local-pref wins",
			a:        &Candidate{PeerAddr: "10.0.0.1", LocalPref: 200},
			b:        &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100},
			wantAddr: "10.0.0.1",
		},
		{
			name:     "lower local-pref loses",
			a:        &Candidate{PeerAddr: "10.0.0.1", LocalPref: 50},
			b:        &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100},
			wantAddr: "10.0.0.2",
		},
		{
			name:     "equal local-pref falls through",
			a:        &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100, ASPathLen: 1},
			b:        &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100, ASPathLen: 3},
			wantAddr: "10.0.0.1", // falls to AS_PATH step
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			best := SelectBest([]*Candidate{tt.a, tt.b})
			if best.PeerAddr != tt.wantAddr {
				t.Errorf("want %s, got %s", tt.wantAddr, best.PeerAddr)
			}
		})
	}
}

// TestBestPath_ASPathLength verifies shorter AS_PATH wins (RFC 4271 §9.1.2 step 2).
//
// VALIDATES: AC-2 — AS_PATH length comparison selects shortest.
// PREVENTS: Longer AS_PATH selected.
func TestBestPath_ASPathLength(t *testing.T) {
	tests := []struct {
		name     string
		a, b     *Candidate
		wantAddr string
	}{
		{
			name:     "shorter as-path wins",
			a:        &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100, ASPathLen: 2},
			b:        &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100, ASPathLen: 5},
			wantAddr: "10.0.0.1",
		},
		{
			name:     "empty as-path beats non-empty",
			a:        &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100, ASPathLen: 0},
			b:        &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100, ASPathLen: 1},
			wantAddr: "10.0.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			best := SelectBest([]*Candidate{tt.a, tt.b})
			if best.PeerAddr != tt.wantAddr {
				t.Errorf("want %s, got %s", tt.wantAddr, best.PeerAddr)
			}
		})
	}
}

// TestBestPath_Origin verifies lower ORIGIN wins (RFC 4271 §9.1.2 step 3).
// IGP(0) < EGP(1) < INCOMPLETE(2).
//
// VALIDATES: AC-3 — ORIGIN comparison selects lowest value.
// PREVENTS: Wrong ORIGIN ordering.
func TestBestPath_Origin(t *testing.T) {
	tests := []struct {
		name     string
		a, b     *Candidate
		wantAddr string
	}{
		{
			name:     "igp beats egp",
			a:        &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100, Origin: OriginIGP},
			b:        &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100, Origin: OriginEGP},
			wantAddr: "10.0.0.1",
		},
		{
			name:     "egp beats incomplete",
			a:        &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100, Origin: OriginEGP},
			b:        &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100, Origin: OriginIncomplete},
			wantAddr: "10.0.0.1",
		},
		{
			name:     "igp beats incomplete",
			a:        &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100, Origin: OriginIGP},
			b:        &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100, Origin: OriginIncomplete},
			wantAddr: "10.0.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			best := SelectBest([]*Candidate{tt.a, tt.b})
			if best.PeerAddr != tt.wantAddr {
				t.Errorf("want %s, got %s", tt.wantAddr, best.PeerAddr)
			}
		})
	}
}

// TestBestPath_MED_SameNeighborAS verifies lower MED wins when same neighbor AS
// (RFC 4271 §9.1.2 step 4).
//
// VALIDATES: AC-4 — MED compared only when same neighbor AS, lowest wins.
// PREVENTS: MED compared across different neighbor ASes.
func TestBestPath_MED_SameNeighborAS(t *testing.T) {
	tests := []struct {
		name     string
		a, b     *Candidate
		wantAddr string
	}{
		{
			name: "lower med wins same neighbor as",
			a: &Candidate{
				PeerAddr: "10.0.0.1", LocalPref: 100,
				MED: 100, FirstAS: 65001,
			},
			b: &Candidate{
				PeerAddr: "10.0.0.2", LocalPref: 100,
				MED: 200, FirstAS: 65001,
			},
			wantAddr: "10.0.0.1",
		},
		{
			name: "med not compared different neighbor as",
			a: &Candidate{
				PeerAddr: "10.0.0.1", LocalPref: 100,
				MED: 999, FirstAS: 65001,
			},
			b: &Candidate{
				PeerAddr: "10.0.0.2", LocalPref: 100,
				MED: 1, FirstAS: 65002,
			},
			// Different neighbor AS — MED not compared — fall to next step.
			// Both eBGP/iBGP same, peer address tiebreak.
			wantAddr: "10.0.0.1", // lower peer address wins
		},
		{
			name: "absent med treated as zero",
			a: &Candidate{
				PeerAddr: "10.0.0.1", LocalPref: 100,
				FirstAS: 65001,
			},
			b: &Candidate{
				PeerAddr: "10.0.0.2", LocalPref: 100,
				MED: 100, FirstAS: 65001,
			},
			wantAddr: "10.0.0.1", // MED=0 (default) < MED=100
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			best := SelectBest([]*Candidate{tt.a, tt.b})
			if best.PeerAddr != tt.wantAddr {
				t.Errorf("want %s, got %s", tt.wantAddr, best.PeerAddr)
			}
		})
	}
}

// TestBestPath_EBGPOverIBGP verifies eBGP preferred over iBGP
// (RFC 4271 §9.1.2 step 5).
//
// VALIDATES: AC-5 — eBGP route preferred over iBGP route.
// PREVENTS: iBGP preferred over eBGP.
func TestBestPath_EBGPOverIBGP(t *testing.T) {
	tests := []struct {
		name     string
		a, b     *Candidate
		wantAddr string
	}{
		{
			name: "ebgp beats ibgp",
			a: &Candidate{
				PeerAddr: "10.0.0.2", LocalPref: 100,
				PeerASN: 65002, LocalASN: 65001, // eBGP
			},
			b: &Candidate{
				PeerAddr: "10.0.0.1", LocalPref: 100,
				PeerASN: 65001, LocalASN: 65001, // iBGP
			},
			wantAddr: "10.0.0.2", // eBGP wins despite higher peer address
		},
		{
			name: "both ebgp falls through",
			a: &Candidate{
				PeerAddr: "10.0.0.2", LocalPref: 100,
				PeerASN: 65002, LocalASN: 65001,
			},
			b: &Candidate{
				PeerAddr: "10.0.0.1", LocalPref: 100,
				PeerASN: 65003, LocalASN: 65001,
			},
			wantAddr: "10.0.0.1", // both eBGP — peer address tiebreak
		},
		{
			name: "both ibgp falls through",
			a: &Candidate{
				PeerAddr: "10.0.0.2", LocalPref: 100,
				PeerASN: 65001, LocalASN: 65001,
			},
			b: &Candidate{
				PeerAddr: "10.0.0.1", LocalPref: 100,
				PeerASN: 65001, LocalASN: 65001,
			},
			wantAddr: "10.0.0.1", // both iBGP — peer address tiebreak
		},
		{
			name: "unknown local asn skips step",
			a: &Candidate{
				PeerAddr: "10.0.0.2", LocalPref: 100,
				PeerASN: 65002, LocalASN: 0, // unknown
			},
			b: &Candidate{
				PeerAddr: "10.0.0.1", LocalPref: 100,
				PeerASN: 65001, LocalASN: 0, // unknown
			},
			wantAddr: "10.0.0.1", // can't determine — peer address tiebreak
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			best := SelectBest([]*Candidate{tt.a, tt.b})
			if best.PeerAddr != tt.wantAddr {
				t.Errorf("want %s, got %s", tt.wantAddr, best.PeerAddr)
			}
		})
	}
}

// TestBestPath_OriginatorID verifies ORIGINATOR_ID used as Router ID tiebreak
// (RFC 4456 — when present, used instead of Router ID in step 7).
//
// VALIDATES: AC-7 — ORIGINATOR_ID used for Router ID comparison when present.
// PREVENTS: ORIGINATOR_ID ignored in tiebreak.
func TestBestPath_OriginatorID(t *testing.T) {
	tests := []struct {
		name     string
		a, b     *Candidate
		wantAddr string
	}{
		{
			name: "lower originator-id wins",
			a: &Candidate{
				PeerAddr: "10.0.0.2", LocalPref: 100,
				OriginatorID: "1.1.1.1",
			},
			b: &Candidate{
				PeerAddr: "10.0.0.1", LocalPref: 100,
				OriginatorID: "2.2.2.2",
			},
			wantAddr: "10.0.0.2", // lower originator-id wins despite higher peer addr
		},
		{
			name: "only one has originator-id skips step",
			a: &Candidate{
				PeerAddr: "10.0.0.2", LocalPref: 100,
				OriginatorID: "1.1.1.1",
			},
			b: &Candidate{
				PeerAddr: "10.0.0.1", LocalPref: 100,
			},
			wantAddr: "10.0.0.1", // can't compare — peer address tiebreak
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			best := SelectBest([]*Candidate{tt.a, tt.b})
			if best.PeerAddr != tt.wantAddr {
				t.Errorf("want %s, got %s", tt.wantAddr, best.PeerAddr)
			}
		})
	}
}

// TestBestPath_PeerAddress verifies lowest peer address wins (RFC 4271 §9.1.2 final tiebreak).
//
// VALIDATES: AC-8 — peer address is final tiebreak, lowest wins.
// PREVENTS: Higher peer address preferred.
func TestBestPath_PeerAddress(t *testing.T) {
	a := &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100}
	b := &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100}
	best := SelectBest([]*Candidate{a, b})
	if best.PeerAddr != "10.0.0.1" {
		t.Errorf("want 10.0.0.1, got %s", best.PeerAddr)
	}
}

// TestBestPath_MultipleCandidate verifies best-path across 4 candidates.
//
// VALIDATES: SelectBest works with >2 candidates.
// PREVENTS: Pairwise comparison only working for exactly 2.
func TestBestPath_MultipleCandidate(t *testing.T) {
	candidates := []*Candidate{
		{PeerAddr: "10.0.0.4", LocalPref: 100, ASPathLen: 3},
		{PeerAddr: "10.0.0.3", LocalPref: 200, ASPathLen: 5}, // highest local-pref
		{PeerAddr: "10.0.0.2", LocalPref: 100, ASPathLen: 1},
		{PeerAddr: "10.0.0.1", LocalPref: 100, ASPathLen: 2},
	}
	best := SelectBest(candidates)
	if best.PeerAddr != "10.0.0.3" {
		t.Errorf("want 10.0.0.3 (highest local-pref), got %s", best.PeerAddr)
	}
}

// TestBestPath_FullTiebreak verifies all steps are evaluated in order.
// Two candidates equal through steps 1-5, differ at step 8 (peer address).
//
// VALIDATES: All comparison steps evaluated in RFC order.
// PREVENTS: Steps skipped or evaluated out of order.
func TestBestPath_FullTiebreak(t *testing.T) {
	a := &Candidate{
		PeerAddr:  "10.0.0.2",
		PeerASN:   65002,
		LocalASN:  65001,
		LocalPref: 100,
		ASPathLen: 2,
		Origin:    OriginIGP,
		MED:       50,
		FirstAS:   65002,
	}
	b := &Candidate{
		PeerAddr:  "10.0.0.1",
		PeerASN:   65003,
		LocalASN:  65001,
		LocalPref: 100,
		ASPathLen: 2,
		Origin:    OriginIGP,
		MED:       50,
		FirstAS:   65003,
	}
	// Equal through steps 1-3. Step 4 (MED): different neighbor AS — skip.
	// Step 5: both eBGP — skip. Step 7: no ORIGINATOR_ID — skip.
	// Step 8: peer address — 10.0.0.1 wins.
	best := SelectBest([]*Candidate{a, b})
	if best.PeerAddr != "10.0.0.1" {
		t.Errorf("want 10.0.0.1 (peer address tiebreak), got %s", best.PeerAddr)
	}
}

// TestASPathLength verifies AS_PATH length counting.
// RFC 4271: AS_SET counts as 1 regardless of number of ASes.
//
// VALIDATES: AS_PATH length calculated correctly for SEQUENCE and SET.
// PREVENTS: AS_SET members counted individually.
func TestASPathLength(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want int
	}{
		{
			name: "empty path",
			data: nil,
			want: 0,
		},
		{
			name: "single as_sequence with 3 ASes",
			// Type=2(SEQUENCE), Count=3, ASN=65001,65002,65003
			data: []byte{
				2, 3,
				0, 0, 0xFD, 0xE9, // 65001
				0, 0, 0xFD, 0xEA, // 65002
				0, 0, 0xFD, 0xEB, // 65003
			},
			want: 3,
		},
		{
			name: "as_set counts as 1",
			// Type=1(SET), Count=3, ASN=65001,65002,65003
			data: []byte{
				1, 3,
				0, 0, 0xFD, 0xE9,
				0, 0, 0xFD, 0xEA,
				0, 0, 0xFD, 0xEB,
			},
			want: 1,
		},
		{
			name: "sequence then set",
			// SEQUENCE(2 ASes) + SET(3 ASes) = 2 + 1 = 3
			data: []byte{
				2, 2,
				0, 0, 0xFD, 0xE9, // 65001
				0, 0, 0xFD, 0xEA, // 65002
				1, 3,
				0, 0, 0xFD, 0xEB, // 65003
				0, 0, 0xFD, 0xEC, // 65004
				0, 0, 0xFD, 0xED, // 65005
			},
			want: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := asPathLength(tt.data)
			if got != tt.want {
				t.Errorf("asPathLength = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestFirstASInPath verifies first AS extraction from AS_PATH wire bytes.
//
// VALIDATES: First AS in path extracted correctly.
// PREVENTS: Wrong AS used for MED neighbor comparison.
func TestFirstASInPath(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want uint32
	}{
		{
			name: "empty path",
			data: nil,
			want: 0,
		},
		{
			name: "first as from sequence",
			data: []byte{
				2, 2,
				0, 0, 0xFD, 0xE9, // 65001
				0, 0, 0xFD, 0xEA, // 65002
			},
			want: 65001,
		},
		{
			name: "first as from set",
			data: []byte{
				1, 2,
				0, 0, 0xFD, 0xEB, // 65003
				0, 0, 0xFD, 0xEC, // 65004
			},
			want: 65003,
		},
		{
			name: "truncated data",
			data: []byte{2, 1, 0},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstASInPath(tt.data)
			if got != tt.want {
				t.Errorf("firstASInPath = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestComparePair verifies pairwise comparison returns correct direction.
//
// VALIDATES: ComparePair returns -1 when a is better, 1 when b is better.
// PREVENTS: Inverted comparison results.
func TestComparePair(t *testing.T) {
	a := &Candidate{PeerAddr: "10.0.0.1", LocalPref: 200}
	b := &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100}

	if got := ComparePair(a, b); got != -1 {
		t.Errorf("a has higher local-pref, want -1, got %d", got)
	}
	if got := ComparePair(b, a); got != 1 {
		t.Errorf("b has lower local-pref, want 1, got %d", got)
	}
}

// TestCompareAddrs verifies numeric IP comparison across digit-count boundaries.
// RFC 4271: BGP Identifier is a 32-bit unsigned integer, not a string.
//
// VALIDATES: IP addresses compared numerically, not lexicographically.
// PREVENTS: "9.0.0.1" > "10.0.0.1" (string comparison gives wrong result).
func TestCompareAddrs(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int // -1=a<b, 0=equal, 1=a>b
	}{
		{name: "same", a: "10.0.0.1", b: "10.0.0.1", want: 0},
		{name: "lower first octet", a: "1.0.0.1", b: "10.0.0.1", want: -1},
		{name: "higher first octet", a: "10.0.0.1", b: "1.0.0.1", want: 1},
		{name: "9 vs 10 numeric", a: "9.0.0.1", b: "10.0.0.1", want: -1},
		{name: "last octet", a: "10.0.0.1", b: "10.0.0.2", want: -1},
		{name: "unparseable fallback", a: "zzz", b: "aaa", want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareAddrs(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareAddrs(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestBestPath_PeerAddressNumeric verifies numeric IP comparison in tiebreak.
// String comparison would pick "9.0.0.1" > "10.0.0.1", but numeric picks "9.0.0.1" < "10.0.0.1".
//
// VALIDATES: Peer address tiebreak uses numeric IP comparison.
// PREVENTS: Wrong best-path when peers have different digit-count IPs.
func TestBestPath_PeerAddressNumeric(t *testing.T) {
	a := &Candidate{PeerAddr: "9.0.0.1", LocalPref: 100}
	b := &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100}
	best := SelectBest([]*Candidate{a, b})
	if best.PeerAddr != "9.0.0.1" {
		t.Errorf("want 9.0.0.1 (numerically lower), got %s", best.PeerAddr)
	}
}

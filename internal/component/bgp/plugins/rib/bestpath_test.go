package rib

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attrpool"
)

// TestSelectBestExplain_Empty verifies nil candidates yields nil explanation.
//
// VALIDATES: cmd-9 reason terminal -- the CLI must cope with "no candidates".
// PREVENTS: Nil-dereference in the reason pipeline when a prefix has no peer.
func TestSelectBestExplain_Empty(t *testing.T) {
	assert.Nil(t, SelectBestExplain(nil))
	assert.Nil(t, SelectBestExplain([]*Candidate{}))
}

// TestSelectBestExplain_Single verifies a single candidate is trivially winner
// with no comparison steps.
//
// VALIDATES: cmd-9 reason terminal for degenerate "only one path" prefixes.
// PREVENTS: Off-by-one panic when len(candidates) == 1.
func TestSelectBestExplain_Single(t *testing.T) {
	c := &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100}
	exp := SelectBestExplain([]*Candidate{c})
	require.NotNil(t, exp)
	assert.Same(t, c, exp.Winner)
	assert.Len(t, exp.Candidates, 1)
	assert.Empty(t, exp.Steps, "no comparisons required for a single candidate")
}

// TestSelectBestExplain_LocalPref verifies the reason narrative for a
// LOCAL_PREF-decided comparison.
//
// VALIDATES: cmd-9 AC -- reason terminal names the deciding step and values.
// PREVENTS: Silent wins -- the user needs to see WHY a path beat another.
func TestSelectBestExplain_LocalPref(t *testing.T) {
	lo := &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100, ASPathLen: 3}
	hi := &Candidate{PeerAddr: "10.0.0.2", LocalPref: 200, ASPathLen: 5}

	exp := SelectBestExplain([]*Candidate{lo, hi})
	require.NotNil(t, exp)
	assert.Same(t, hi, exp.Winner, "higher local-pref wins despite longer as-path")
	require.Len(t, exp.Steps, 1)

	step := exp.Steps[0]
	assert.Equal(t, BestStepLocalPref, step.Step)
	assert.Equal(t, 1, step.WinnerIdx, "candidates[1] (hi) wins")
	assert.Contains(t, step.Reason, "200")
	assert.Contains(t, step.Reason, "100")
}

// TestSelectBestExplain_ASPathLen verifies AS_PATH length tiebreak after
// equal LOCAL_PREF.
//
// VALIDATES: cmd-9 AC -- step progression through the decision process.
// PREVENTS: Misattributing the winner to a later step when an earlier one
// resolved the tie.
func TestSelectBestExplain_ASPathLen(t *testing.T) {
	short := &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100, ASPathLen: 2}
	long := &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100, ASPathLen: 5}

	exp := SelectBestExplain([]*Candidate{long, short})
	require.NotNil(t, exp)
	assert.Same(t, short, exp.Winner)
	require.Len(t, exp.Steps, 1)
	assert.Equal(t, BestStepASPathLen, exp.Steps[0].Step)
}

// TestSelectBestExplain_Origin verifies ORIGIN-based decision when LOCAL_PREF
// and AS_PATH length both tie.
//
// VALIDATES: cmd-9 AC -- ORIGIN step (IGP < EGP < INCOMPLETE) is reported.
// PREVENTS: Step reported as ASPathLen when only Origin differs.
func TestSelectBestExplain_Origin(t *testing.T) {
	igp := &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100, ASPathLen: 3, Origin: OriginIGP}
	inc := &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100, ASPathLen: 3, Origin: OriginIncomplete}

	exp := SelectBestExplain([]*Candidate{inc, igp})
	require.NotNil(t, exp)
	assert.Same(t, igp, exp.Winner)
	require.Len(t, exp.Steps, 1)
	assert.Equal(t, BestStepOrigin, exp.Steps[0].Step)
}

// TestSelectBestExplain_PeerAddrTiebreak verifies the final peer-address
// tiebreak when all earlier steps are identical.
//
// VALIDATES: cmd-9 AC -- falls through to the peer-address tiebreak.
// PREVENTS: Reason terminal reporting "equal" for a valid tiebreak.
func TestSelectBestExplain_PeerAddrTiebreak(t *testing.T) {
	a := &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100, ASPathLen: 3, Origin: OriginIGP}
	b := &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100, ASPathLen: 3, Origin: OriginIGP}

	exp := SelectBestExplain([]*Candidate{b, a})
	require.NotNil(t, exp)
	assert.Same(t, a, exp.Winner, "lower peer address wins")
	require.Len(t, exp.Steps, 1)
	assert.Equal(t, BestStepPeerAddr, exp.Steps[0].Step)
	assert.Contains(t, exp.Steps[0].Reason, "10.0.0.1")
	assert.Contains(t, exp.Steps[0].Reason, "10.0.0.2")
}

// TestSelectBestExplain_ThreeCandidates verifies the narrative for a
// non-trivial reduction with three candidates and two comparison steps.
//
// VALIDATES: cmd-9 AC -- linear reduction records N-1 pairwise steps and
// each step names the running incumbent + challenger.
// PREVENTS: Mis-indexing the incumbent after a challenger wins mid-reduction.
func TestSelectBestExplain_ThreeCandidates(t *testing.T) {
	// c0 has local-pref 100, c1 has 200 (beats c0), c2 has 150 (loses to c1).
	c0 := &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100}
	c1 := &Candidate{PeerAddr: "10.0.0.2", LocalPref: 200}
	c2 := &Candidate{PeerAddr: "10.0.0.3", LocalPref: 150}

	exp := SelectBestExplain([]*Candidate{c0, c1, c2})
	require.NotNil(t, exp)
	assert.Same(t, c1, exp.Winner)
	require.Len(t, exp.Steps, 2)

	// Step 0: c1 (challenger, idx 1) vs c0 (incumbent, idx 0); c1 wins by local-pref.
	assert.Equal(t, 0, exp.Steps[0].IncumbentIdx)
	assert.Equal(t, 1, exp.Steps[0].ChallengerIdx)
	assert.Equal(t, 1, exp.Steps[0].WinnerIdx)
	assert.Equal(t, BestStepLocalPref, exp.Steps[0].Step)

	// Step 1: c2 (challenger, idx 2) vs c1 (incumbent, idx 1); c1 still wins.
	assert.Equal(t, 1, exp.Steps[1].IncumbentIdx)
	assert.Equal(t, 2, exp.Steps[1].ChallengerIdx)
	assert.Equal(t, 1, exp.Steps[1].WinnerIdx)
	assert.Equal(t, BestStepLocalPref, exp.Steps[1].Step)
}

// TestBestStep_String verifies every defined step has a stable name.
//
// VALIDATES: cmd-9 JSON output uses the String() form directly.
// PREVENTS: A new BestStep constant added later without a name, leaving
// "unknown-step" in the reason JSON.
func TestBestStep_String(t *testing.T) {
	for step, want := range map[BestStep]string{
		BestStepStale:        "stale-level",
		BestStepLocalPref:    "local-preference",
		BestStepASPathLen:    "as-path-length",
		BestStepOrigin:       "origin",
		BestStepMED:          "med",
		BestStepEBGPOverIBGP: "ebgp-over-ibgp",
		BestStepIGPCost:      "igp-cost",
		BestStepRouterID:     "router-id",
		BestStepPeerAddr:     "peer-address",
		BestStepEqual:        "equal",
	} {
		assert.Equal(t, want, step.String())
	}
	assert.Equal(t, "unknown-step", BestStep(99).String())
}

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

// --- LLGR depreference tests (RFC 9494) ---

// TestSelectBest_LLGRStaleDepreference verifies normal beats LLGR-stale.
//
// VALIDATES: RFC 9494: any non-LLGR-stale route beats any LLGR-stale route.
// PREVENTS: LLGR-stale route winning despite having better LOCAL_PREF.
func TestSelectBest_LLGRStaleDepreference(t *testing.T) {
	t.Parallel()
	// LLGR-stale route has higher LOCAL_PREF but should still lose
	stale := &Candidate{PeerAddr: "10.0.0.1", LocalPref: 300, StaleLevel: 2}
	normal := &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100}

	best := SelectBest([]*Candidate{stale, normal})
	if best.PeerAddr != "10.0.0.2" {
		t.Errorf("normal route should beat LLGR-stale, got %s", best.PeerAddr)
	}

	// Reverse order
	best = SelectBest([]*Candidate{normal, stale})
	if best.PeerAddr != "10.0.0.2" {
		t.Errorf("normal route should beat LLGR-stale (reversed), got %s", best.PeerAddr)
	}
}

// TestSelectBest_BothLLGRStale verifies tiebreaking between two LLGR-stale routes.
//
// VALIDATES: RFC 9494: between two LLGR-stale routes, normal tiebreaking applies.
// PREVENTS: All LLGR-stale routes treated as equal.
func TestSelectBest_BothLLGRStale(t *testing.T) {
	t.Parallel()
	a := &Candidate{PeerAddr: "10.0.0.1", LocalPref: 200, StaleLevel: 2}
	b := &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100, StaleLevel: 2}

	best := SelectBest([]*Candidate{a, b})
	if best.PeerAddr != "10.0.0.1" {
		t.Errorf("higher LOCAL_PREF should win between two LLGR-stale, got %s", best.PeerAddr)
	}
}

// TestSelectBest_OnlyLLGRStale verifies best selected when all are LLGR-stale.
//
// VALIDATES: When all candidates are LLGR-stale, best among them is selected.
// PREVENTS: Nil return when all candidates are LLGR-stale.
func TestSelectBest_OnlyLLGRStale(t *testing.T) {
	t.Parallel()
	candidates := []*Candidate{
		{PeerAddr: "10.0.0.3", LocalPref: 100, ASPathLen: 3, StaleLevel: 2},
		{PeerAddr: "10.0.0.2", LocalPref: 100, ASPathLen: 1, StaleLevel: 2}, // shortest path
		{PeerAddr: "10.0.0.1", LocalPref: 100, ASPathLen: 2, StaleLevel: 2},
	}

	best := SelectBest(candidates)
	if best.PeerAddr != "10.0.0.2" {
		t.Errorf("shortest AS_PATH should win among LLGR-stale, got %s", best.PeerAddr)
	}
}

// TestComparePair_LLGRStale verifies pairwise LLGR comparison direction.
//
// VALIDATES: ComparePair returns correct values for LLGR-stale combinations.
// PREVENTS: Inverted LLGR depreference.
func TestComparePair_LLGRStale(t *testing.T) {
	t.Parallel()
	normal := &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100}
	stale := &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100, StaleLevel: 2}

	if got := ComparePair(normal, stale); got != -1 {
		t.Errorf("normal vs LLGR-stale: want -1, got %d", got)
	}
	if got := ComparePair(stale, normal); got != 1 {
		t.Errorf("LLGR-stale vs normal: want 1, got %d", got)
	}
	// Both LLGR-stale: falls through to normal comparison
	stale2 := &Candidate{PeerAddr: "10.0.0.3", LocalPref: 100, StaleLevel: 2}
	if got := ComparePair(stale, stale2); got != -1 {
		t.Errorf("both LLGR-stale, lower peer addr: want -1, got %d", got)
	}
}

// TestComparePair_GRStaleCompetesNormally verifies GR-stale (level 1) competes normally.
//
// VALIDATES: RFC 4724: GR-stale routes below DepreferenceThreshold are NOT deprioritized.
// PREVENTS: GR-stale routes losing to fresh routes when they have better attributes.
func TestComparePair_GRStaleCompetesNormally(t *testing.T) {
	t.Parallel()
	// Level 1 (GR-stale) with higher LOCAL_PREF should beat level 0 (fresh)
	grStale := &Candidate{PeerAddr: "10.0.0.1", LocalPref: 200, StaleLevel: 1}
	fresh := &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100, StaleLevel: 0}

	best := SelectBest([]*Candidate{grStale, fresh})
	if best.PeerAddr != "10.0.0.1" {
		t.Errorf("GR-stale with higher LOCAL_PREF should win, got %s", best.PeerAddr)
	}
}

// TestComparePair_GRStaleBeatsLLGRStale verifies GR-stale beats LLGR-stale across threshold.
//
// VALIDATES: Level 1 (below threshold) beats level 2 (at threshold) regardless of attributes.
// PREVENTS: LLGR-stale winning over GR-stale due to better LOCAL_PREF.
func TestComparePair_GRStaleBeatsLLGRStale(t *testing.T) {
	t.Parallel()
	// Level 1 with worse attributes should still beat level 2
	grStale := &Candidate{PeerAddr: "10.0.0.1", LocalPref: 50, StaleLevel: 1}
	llgrStale := &Candidate{PeerAddr: "10.0.0.2", LocalPref: 200, StaleLevel: 2}

	best := SelectBest([]*Candidate{grStale, llgrStale})
	if best.PeerAddr != "10.0.0.1" {
		t.Errorf("GR-stale (below threshold) should beat LLGR-stale, got %s", best.PeerAddr)
	}
}

// TestComparePair_BothDeprefDifferentLevels verifies lower level wins among deprioritized.
//
// VALIDATES: Between two routes both above threshold, lower stale level wins.
// PREVENTS: All deprioritized routes treated as equal.
func TestComparePair_BothDeprefDifferentLevels(t *testing.T) {
	t.Parallel()
	level2 := &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100, StaleLevel: 2}
	level3 := &Candidate{PeerAddr: "10.0.0.2", LocalPref: 100, StaleLevel: 3}

	best := SelectBest([]*Candidate{level2, level3})
	if best.PeerAddr != "10.0.0.1" {
		t.Errorf("lower stale level should win among deprioritized, got %s", best.PeerAddr)
	}
}

// --- SelectMultipath tests (cmd-3 phase 3) ---------------------------------

// twoEqualCostCandidates builds two candidates that tie through every
// non-tiebreaker step (LOCAL_PREF, AS_PATH len+handle, Origin, MED, eBGP/iBGP)
// and differ only by PeerAddr, so the primary-vs-sibling choice is decided by
// the final address tiebreak. Both share the same AS_PATH pool handle so the
// default (relaxASPath=false) content check passes.
func twoEqualCostCandidates(handle attrpool.Handle) (*Candidate, *Candidate) {
	a := &Candidate{
		PeerAddr: "10.0.0.1", PeerASN: 65001, LocalASN: 65000,
		LocalPref: 100, ASPathLen: 3, FirstAS: 65001, Origin: OriginIGP, MED: 50,
		ASPathHandle: handle,
	}
	b := &Candidate{
		PeerAddr: "10.0.0.2", PeerASN: 65001, LocalASN: 65000,
		LocalPref: 100, ASPathLen: 3, FirstAS: 65001, Origin: OriginIGP, MED: 50,
		ASPathHandle: handle,
	}
	return a, b
}

// TestSelectMultipath_DisabledByDefault verifies that maxPaths <= 1 never
// produces siblings, preserving single-best behavior when multipath is off.
//
// VALIDATES: cmd-3 AC-1 -- maximum-paths=1 is RFC 4271 single best-path.
// PREVENTS: Accidental ECMP when bgp/multipath is absent or maximum-paths=1.
func TestSelectMultipath_DisabledByDefault(t *testing.T) {
	t.Parallel()
	a, b := twoEqualCostCandidates(attrpool.Handle(42))

	// maxPaths = 0 and maxPaths = 1 must behave identically: single best, no siblings.
	for _, mp := range []uint32{0, 1} {
		primary, siblings := SelectMultipath([]*Candidate{a, b}, mp, false)
		assert.Same(t, a, primary, "primary must equal SelectBest winner (lower peer address)")
		assert.Nil(t, siblings, "maxPaths=%d must produce no siblings", mp)
	}
}

// TestSelectMultipath_TwoEqualCost verifies that two byte-equal paths land
// in the multipath set when maximum-paths >= 2.
//
// VALIDATES: cmd-3 AC-2 -- maximum-paths=N selects up to N equal-cost paths.
// PREVENTS: Multipath silently dropping equal-cost candidates.
func TestSelectMultipath_TwoEqualCost(t *testing.T) {
	t.Parallel()
	a, b := twoEqualCostCandidates(attrpool.Handle(42))

	primary, siblings := SelectMultipath([]*Candidate{a, b}, 4, false)
	require.NotNil(t, primary)
	assert.Same(t, a, primary, "lower peer address remains primary")
	require.Len(t, siblings, 1)
	assert.Same(t, b, siblings[0])
}

// TestSelectMultipath_FewerAvailable verifies that the siblings slice is
// capped at "available candidates" even when maximum-paths is larger.
//
// VALIDATES: cmd-3 AC-3 -- maximum-paths=4 with only 2 equal-cost available.
// PREVENTS: Out-of-bounds allocation when maxPaths exceeds candidate count.
func TestSelectMultipath_FewerAvailable(t *testing.T) {
	t.Parallel()
	a, b := twoEqualCostCandidates(attrpool.Handle(42))

	primary, siblings := SelectMultipath([]*Candidate{a, b}, 16, false)
	require.NotNil(t, primary)
	assert.Len(t, siblings, 1, "multipath set capped at available candidates")
}

// TestSelectMultipath_CappedAtMaxPaths verifies that siblings are truncated
// to maxPaths-1 when more equal-cost candidates exist.
//
// VALIDATES: cmd-3 AC-2 -- N paths selected when N > 1 are available, but
// never more than maximum-paths-1 siblings.
// PREVENTS: Multipath set exceeding the configured ECMP budget.
func TestSelectMultipath_CappedAtMaxPaths(t *testing.T) {
	t.Parallel()
	h := attrpool.Handle(42)
	cands := []*Candidate{
		{PeerAddr: "10.0.0.1", PeerASN: 65001, LocalASN: 65000, LocalPref: 100, ASPathLen: 3, FirstAS: 65001, Origin: OriginIGP, ASPathHandle: h},
		{PeerAddr: "10.0.0.2", PeerASN: 65001, LocalASN: 65000, LocalPref: 100, ASPathLen: 3, FirstAS: 65001, Origin: OriginIGP, ASPathHandle: h},
		{PeerAddr: "10.0.0.3", PeerASN: 65001, LocalASN: 65000, LocalPref: 100, ASPathLen: 3, FirstAS: 65001, Origin: OriginIGP, ASPathHandle: h},
		{PeerAddr: "10.0.0.4", PeerASN: 65001, LocalASN: 65000, LocalPref: 100, ASPathLen: 3, FirstAS: 65001, Origin: OriginIGP, ASPathHandle: h},
	}

	primary, siblings := SelectMultipath(cands, 3, false)
	require.NotNil(t, primary)
	assert.Len(t, siblings, 2, "3 total paths = 1 primary + 2 siblings")
}

// TestSelectMultipath_DifferentASPathContent verifies that two candidates
// with the same AS_PATH LENGTH but different CONTENT are NOT treated as
// equal-cost when relaxASPath is false.
//
// VALIDATES: cmd-3 AC-4 -- default behavior requires identical AS_PATH.
// PREVENTS: Incorrect multipath when routes traverse different upstream ASes.
func TestSelectMultipath_DifferentASPathContent(t *testing.T) {
	t.Parallel()
	// Same length, same FirstAS, different pool handles (= different bytes).
	a := &Candidate{
		PeerAddr: "10.0.0.1", PeerASN: 65001, LocalASN: 65000,
		LocalPref: 100, ASPathLen: 3, FirstAS: 65001, Origin: OriginIGP,
		ASPathHandle: attrpool.Handle(42),
	}
	b := &Candidate{
		PeerAddr: "10.0.0.2", PeerASN: 65001, LocalASN: 65000,
		LocalPref: 100, ASPathLen: 3, FirstAS: 65001, Origin: OriginIGP,
		ASPathHandle: attrpool.Handle(99),
	}

	primary, siblings := SelectMultipath([]*Candidate{a, b}, 4, false)
	require.NotNil(t, primary)
	assert.Empty(t, siblings, "different AS_PATH content must not multipath with relaxASPath=false")
}

// TestSelectMultipath_RelaxASPath verifies that relaxASPath=true allows
// candidates with different AS_PATH content (but same length) to share the
// multipath set.
//
// VALIDATES: cmd-3 AC-5 -- relax-as-path relaxes content match to length-only.
// PREVENTS: Cisco-style "bgp bestpath as-path multipath-relax" silently
// falling back to strict content match.
func TestSelectMultipath_RelaxASPath(t *testing.T) {
	t.Parallel()
	a := &Candidate{
		PeerAddr: "10.0.0.1", PeerASN: 65001, LocalASN: 65000,
		LocalPref: 100, ASPathLen: 3, FirstAS: 65001, Origin: OriginIGP,
		ASPathHandle: attrpool.Handle(42),
	}
	b := &Candidate{
		PeerAddr: "10.0.0.2", PeerASN: 65002, LocalASN: 65000,
		LocalPref: 100, ASPathLen: 3, FirstAS: 65002, Origin: OriginIGP,
		ASPathHandle: attrpool.Handle(99),
	}

	primary, siblings := SelectMultipath([]*Candidate{a, b}, 4, true)
	require.NotNil(t, primary)
	require.Len(t, siblings, 1, "relaxASPath allows different AS_PATH content")
	assert.Same(t, b, siblings[0])
}

// TestSelectMultipath_LocalPrefMismatch verifies that candidates with
// different LOCAL_PREF never enter the multipath set.
//
// VALIDATES: cmd-3 AC-2 guard -- LOCAL_PREF is gate zero for multipath.
// PREVENTS: Low-LP routes being promoted to ECMP alongside high-LP winners.
func TestSelectMultipath_LocalPrefMismatch(t *testing.T) {
	t.Parallel()
	a := &Candidate{PeerAddr: "10.0.0.1", PeerASN: 65001, LocalASN: 65000, LocalPref: 200, ASPathLen: 3, FirstAS: 65001, Origin: OriginIGP}
	b := &Candidate{PeerAddr: "10.0.0.2", PeerASN: 65001, LocalASN: 65000, LocalPref: 100, ASPathLen: 3, FirstAS: 65001, Origin: OriginIGP}

	primary, siblings := SelectMultipath([]*Candidate{a, b}, 4, true)
	assert.Same(t, a, primary)
	assert.Empty(t, siblings, "LOCAL_PREF mismatch must block multipath")
}

// TestSelectMultipath_EBGPvsIBGP verifies the eBGP-over-iBGP gate excludes
// iBGP siblings from an eBGP primary (and vice versa).
//
// VALIDATES: cmd-3 step-5 gate -- eBGP/iBGP protocol type must match for
// multipath membership.
// PREVENTS: Mixed eBGP+iBGP multipath, which violates vendor convention and
// causes non-deterministic FIB behavior.
func TestSelectMultipath_EBGPvsIBGP(t *testing.T) {
	t.Parallel()
	ebgp := &Candidate{PeerAddr: "10.0.0.1", PeerASN: 65001, LocalASN: 65000, LocalPref: 100, ASPathLen: 3, FirstAS: 65001, Origin: OriginIGP}
	ibgp := &Candidate{PeerAddr: "10.0.0.2", PeerASN: 65000, LocalASN: 65000, LocalPref: 100, ASPathLen: 3, FirstAS: 65001, Origin: OriginIGP}

	primary, siblings := SelectMultipath([]*Candidate{ebgp, ibgp}, 4, true)
	require.NotNil(t, primary)
	assert.Same(t, ebgp, primary, "eBGP wins over iBGP")
	assert.Empty(t, siblings, "iBGP must not enter an eBGP multipath set")
}

// TestSelectMultipath_MEDMismatchSameNeighbor verifies that MED differences
// block multipath membership when both candidates come from the same
// neighbor AS (step 4 gate).
//
// VALIDATES: cmd-3 step-4 gate -- MED matters only when FirstAS is shared.
// PREVENTS: Routes with same neighbor but different MED being multipathed.
func TestSelectMultipath_MEDMismatchSameNeighbor(t *testing.T) {
	t.Parallel()
	h := attrpool.Handle(42)
	a := &Candidate{PeerAddr: "10.0.0.1", PeerASN: 65001, LocalASN: 65000, LocalPref: 100, ASPathLen: 3, FirstAS: 65001, Origin: OriginIGP, MED: 100, ASPathHandle: h}
	b := &Candidate{PeerAddr: "10.0.0.2", PeerASN: 65001, LocalASN: 65000, LocalPref: 100, ASPathLen: 3, FirstAS: 65001, Origin: OriginIGP, MED: 200, ASPathHandle: h}

	primary, siblings := SelectMultipath([]*Candidate{a, b}, 4, false)
	require.NotNil(t, primary)
	assert.Same(t, a, primary, "lower MED wins as primary")
	assert.Empty(t, siblings, "different MED blocks multipath when FirstAS shared")
}

// TestSelectMultipath_DifferentNeighborIgnoresMED verifies that MED is NOT
// compared when the two candidates are from different neighbor ASes --
// multipath can still form if all other steps agree.
//
// VALIDATES: cmd-3 RFC 4271 §9.1.2.2(c) -- "comparison is only performed
// between routes learned from the same neighboring AS".
// PREVENTS: Multipath refusing to form across different upstreams because
// of unrelated MEDs they happen to carry.
func TestSelectMultipath_DifferentNeighborIgnoresMED(t *testing.T) {
	t.Parallel()
	h := attrpool.Handle(42)
	a := &Candidate{PeerAddr: "10.0.0.1", PeerASN: 65001, LocalASN: 65000, LocalPref: 100, ASPathLen: 3, FirstAS: 65001, Origin: OriginIGP, MED: 100, ASPathHandle: h}
	b := &Candidate{PeerAddr: "10.0.0.2", PeerASN: 65002, LocalASN: 65000, LocalPref: 100, ASPathLen: 3, FirstAS: 65002, Origin: OriginIGP, MED: 999, ASPathHandle: h}

	primary, siblings := SelectMultipath([]*Candidate{a, b}, 4, false)
	require.NotNil(t, primary)
	require.Len(t, siblings, 1, "different neighbor AS -> MED ignored -> multipath forms")
}

// TestSelectMultipath_EmptyAndSingle verifies edge cases: no candidates or
// a single candidate.
//
// VALIDATES: cmd-3 degenerate cases -- nil primary for empty, single primary
// for len-1.
// PREVENTS: Nil-dereference or out-of-bounds when the candidate set is trivial.
func TestSelectMultipath_EmptyAndSingle(t *testing.T) {
	t.Parallel()
	primary, siblings := SelectMultipath(nil, 4, false)
	assert.Nil(t, primary)
	assert.Nil(t, siblings)

	solo := &Candidate{PeerAddr: "10.0.0.1", LocalPref: 100}
	primary, siblings = SelectMultipath([]*Candidate{solo}, 4, false)
	assert.Same(t, solo, primary)
	assert.Nil(t, siblings, "single candidate has no siblings")
}

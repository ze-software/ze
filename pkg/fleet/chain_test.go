package fleet

import (
	"crypto/sha1" //nolint:gosec // independent SHA-1 verification of spec-fleet-10 chain fold
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestFoldFullDeterministicAndDiffersByContent(t *testing.T) {
	// VALIDATES: FoldFull computes SHA-1(20 zero bytes || "full\n" || config)
	// as 40 lowercase hex chars, is deterministic for identical input (including
	// empty/nil), and changes when any byte of config changes.
	// PREVENTS: Hub and client deriving different full-anchor hashes or ignoring
	// single-byte configuration differences.
	cfgA := []byte("bgp {\n    neighbor 10.0.0.1 {\n        local-as 65001;\n    }\n}\n")
	cfgB := []byte("bgp {\n    neighbor 10.0.0.2 {\n        local-as 65001;\n    }\n}\n")

	gotA1 := FoldFull(cfgA)
	gotA2 := FoldFull(cfgA)
	gotB := FoldFull(cfgB)

	if len(gotA1) != ChainHashLen {
		t.Fatalf("FoldFull len = %d, want %d (%q)", len(gotA1), ChainHashLen, gotA1)
	}
	if !ValidChainHash(gotA1) {
		t.Fatalf("FoldFull output %q is not a valid chain hash", gotA1)
	}
	if gotA1 != gotA2 {
		t.Fatalf("FoldFull not deterministic: %q != %q", gotA1, gotA2)
	}
	if gotA1 == gotB {
		t.Fatalf("FoldFull produced identical hash for distinct configs: %q", gotA1)
	}

	// Verify exact wire formula independently.
	var wantPayload []byte
	wantPayload = append(wantPayload, make([]byte, sha1.Size)...)
	wantPayload = append(wantPayload, []byte("full\n")...)
	wantPayload = append(wantPayload, cfgA...)
	sum := sha1.Sum(wantPayload) //nolint:gosec // verifying spec-fleet-10 SHA-1 fold formula
	wantA := hex.EncodeToString(sum[:])
	if gotA1 != wantA {
		t.Fatalf("FoldFull(cfgA) = %q, want %q", gotA1, wantA)
	}

	// Empty and nil configs produce the same valid 40-hex anchor.
	emptyNil := FoldFull(nil)
	emptySlice := FoldFull([]byte{})
	if !ValidChainHash(emptyNil) {
		t.Fatalf("FoldFull(nil) = %q is invalid", emptyNil)
	}
	if emptyNil != emptySlice {
		t.Fatalf("FoldFull(nil) = %q != FoldFull([]byte{}) = %q", emptyNil, emptySlice)
	}
	if emptyNil == gotA1 {
		t.Fatalf("FoldFull(empty) collided with non-empty config: %q", emptyNil)
	}

	// ChainState JSON round-trip.
	state := ChainState{Version: VersionHash(cfgA), Chain: gotA1}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal ChainState: %v", err)
	}
	var decoded ChainState
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal ChainState: %v", err)
	}
	if decoded != state {
		t.Fatalf("ChainState round-trip = %+v, want %+v", decoded, state)
	}
}

func TestFoldDiffChainsOrderAndPayload(t *testing.T) {
	// VALIDATES: FoldDiff computes SHA-1(prev20 || "diff\n" || baseVer || "\n"
	// || targetVer || "\n" || delta), depends on prevHex, baseVer, targetVer,
	// delta, and step order, and rejects invalid prevHex or empty versions.
	// PREVENTS: Swapped/out-of-order deltas or tampered version metadata
	// colliding on the same chain head.
	anchor := FoldFull([]byte("initial\n"))
	delta1 := []byte(`[{"op":"ins","lines":["v2\n"]}]`)
	delta2 := []byte(`[{"op":"ins","lines":["v3\n"]}]`)

	step1, err := FoldDiff(anchor, "1111111111111111", "2222222222222222", delta1)
	if err != nil {
		t.Fatalf("FoldDiff step1: %v", err)
	}
	if !ValidChainHash(step1) {
		t.Fatalf("FoldDiff output %q is invalid", step1)
	}

	// Verify exact formula independently.
	prevBytes, err := hex.DecodeString(anchor)
	if err != nil {
		t.Fatalf("decode anchor: %v", err)
	}
	var wantPayload []byte
	wantPayload = append(wantPayload, prevBytes...)
	wantPayload = append(wantPayload, []byte("diff\n1111111111111111\n2222222222222222\n")...)
	wantPayload = append(wantPayload, delta1...)
	sum := sha1.Sum(wantPayload) //nolint:gosec // verifying spec-fleet-10 SHA-1 fold formula
	wantStep1 := hex.EncodeToString(sum[:])
	if step1 != wantStep1 {
		t.Fatalf("FoldDiff = %q, want %q", step1, wantStep1)
	}

	// Deterministic replay produces identical output.
	step1Again, err := FoldDiff(anchor, "1111111111111111", "2222222222222222", delta1)
	if err != nil {
		t.Fatalf("FoldDiff replay: %v", err)
	}
	if step1Again != step1 {
		t.Fatalf("FoldDiff not deterministic: %q != %q", step1Again, step1)
	}

	// Changing baseVer, targetVer, delta, or prevHex changes the output.
	diffBase, err := FoldDiff(anchor, "aaaaaaaaaaaaaaaa", "2222222222222222", delta1)
	if err != nil {
		t.Fatalf("FoldDiff diffBase: %v", err)
	}
	if diffBase == step1 {
		t.Fatalf("changing baseVer did not change hash: %q", diffBase)
	}

	diffTarget, err := FoldDiff(anchor, "1111111111111111", "bbbbbbbbbbbbbbbb", delta1)
	if err != nil {
		t.Fatalf("FoldDiff diffTarget: %v", err)
	}
	if diffTarget == step1 {
		t.Fatalf("changing targetVer did not change hash: %q", diffTarget)
	}

	diffDelta, err := FoldDiff(anchor, "1111111111111111", "2222222222222222", delta2)
	if err != nil {
		t.Fatalf("FoldDiff diffDelta: %v", err)
	}
	if diffDelta == step1 {
		t.Fatalf("changing delta did not change hash: %q", diffDelta)
	}

	// Order sensitivity: (delta1 then delta2) != (delta2 then delta1).
	order12, err := FoldDiff(step1, "2222222222222222", "3333333333333333", delta2)
	if err != nil {
		t.Fatalf("FoldDiff order12: %v", err)
	}
	first2, err := FoldDiff(anchor, "1111111111111111", "2222222222222222", delta2)
	if err != nil {
		t.Fatalf("FoldDiff first2: %v", err)
	}
	order21, err := FoldDiff(first2, "2222222222222222", "3333333333333333", delta1)
	if err != nil {
		t.Fatalf("FoldDiff order21: %v", err)
	}
	if order12 == order21 {
		t.Fatalf("swapping delta order produced identical hash: %q", order12)
	}

	// Domain separation: FoldDiff never collides with FoldFull even over empty delta.
	emptyFold, err := FoldDiff(strings.Repeat("0", ChainHashLen), "v1", "v2", nil)
	if err != nil {
		t.Fatalf("FoldDiff zero prev: %v", err)
	}
	if emptyFold == FoldFull([]byte("v1\nv2\n")) {
		t.Fatalf("FoldDiff collided with FoldFull: %q", emptyFold)
	}

	// Error cases: invalid prevHex, empty baseVer, empty targetVer.
	invalidCases := []struct {
		name      string
		prevHex   string
		baseVer   string
		targetVer string
	}{
		{name: "empty prevHex", prevHex: "", baseVer: "v1", targetVer: "v2"},
		{name: "short prevHex (39)", prevHex: strings.Repeat("a", ChainHashLen-1), baseVer: "v1", targetVer: "v2"},
		{name: "long prevHex (41)", prevHex: strings.Repeat("a", ChainHashLen+1), baseVer: "v1", targetVer: "v2"},
		{name: "uppercase prevHex", prevHex: strings.ToUpper(anchor), baseVer: "v1", targetVer: "v2"},
		{name: "non-hex prevHex", prevHex: strings.Repeat("g", ChainHashLen), baseVer: "v1", targetVer: "v2"},
		{name: "empty baseVer", prevHex: anchor, baseVer: "", targetVer: "v2"},
		{name: "empty targetVer", prevHex: anchor, baseVer: "v1", targetVer: ""},
	}
	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FoldDiff(tc.prevHex, tc.baseVer, tc.targetVer, delta1)
			if err == nil {
				t.Fatalf("expected error, got hash %q", got)
			}
			if got != "" {
				t.Fatalf("expected empty hash on error, got %q", got)
			}
		})
	}
}

func TestValidChainHashBoundaries(t *testing.T) {
	// VALIDATES: ValidChainHash accepts exactly 40 lowercase hex chars and
	// rejects length 39, length 41, uppercase hex, and non-hex characters.
	// PREVENTS: Malformed or non-canonical chain hashes crossing wire or store
	// boundaries.
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "empty", in: "", want: false},
		{name: "39 chars (below min)", in: strings.Repeat("0", ChainHashLen-1), want: false},
		{name: "40 lowercase digits", in: "0123456789012345678901234567890123456789", want: true},
		{name: "40 lowercase hex", in: "0123456789abcdef0123456789abcdef01234567", want: true},
		{name: "41 chars (above max)", in: strings.Repeat("a", ChainHashLen+1), want: false},
		{name: "uppercase hex A", in: "A" + strings.Repeat("0", ChainHashLen-1), want: false},
		{name: "uppercase hex F at end", in: strings.Repeat("0", ChainHashLen-1) + "F", want: false},
		{name: "non-hex g", in: strings.Repeat("a", ChainHashLen-1) + "g", want: false},
		{name: "non-hex slash (below 0)", in: "/" + strings.Repeat("0", ChainHashLen-1), want: false},
		{name: "non-hex colon (above 9)", in: ":" + strings.Repeat("0", ChainHashLen-1), want: false},
		{name: "non-hex backtick (below a)", in: "`" + strings.Repeat("0", ChainHashLen-1), want: false},
		{name: "whitespace", in: " " + strings.Repeat("0", ChainHashLen-1), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidChainHash(tt.in); got != tt.want {
				t.Fatalf("ValidChainHash(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

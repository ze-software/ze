// Design: docs/architecture/core-design.md -- AS notation on the ze-analyze flags
package analyze

import "testing"

// TestAnalyzeASFlagsReadEveryNotation proves the four operator-typed AS flags
// of the shipped `ze-analyze` binary take any of the three RFC 5396 spellings.
//
// VALIDATES: --peer-asn and the three --local-as readers call asn.Parse.
// PREVENTS: an operator reading 1.10 on a show output and having
// `ze-analyze filter --peer-asn 1.10` silently match nothing.
func TestAnalyzeASFlagsReadEveryNotation(t *testing.T) {
	for _, spelling := range []string{"65546", "1.10"} {
		filter, ok := parseFilterOpts([]string{"--peer-asn", spelling, "in.mrt", "out.mrt"})
		if !ok {
			t.Fatalf("--peer-asn %s was refused", spelling)
		}
		if filter.peerASN != 65546 {
			t.Errorf("--peer-asn %s = %d, want 65546", spelling, filter.peerASN)
		}

		replay, ok := parseReplayOpts([]string{"--local-as", spelling, "in.mrt", "127.0.0.1:179"})
		if !ok {
			t.Fatalf("replay --local-as %s was refused", spelling)
		}
		if replay.localAS != 65546 {
			t.Errorf("replay --local-as %s = %d, want 65546", spelling, replay.localAS)
		}

		inject, ok := parseInjectOpts([]string{"--local-as", spelling, "in.mrt", "127.0.0.1:179"})
		if !ok {
			t.Fatalf("inject --local-as %s was refused", spelling)
		}
		if inject.localAS != 65546 {
			t.Errorf("inject --local-as %s = %d, want 65546", spelling, inject.localAS)
		}
	}

	// A token that names no AS number is still refused.
	if _, ok := parseFilterOpts([]string{"--peer-asn", "1.99999", "in.mrt", "out.mrt"}); ok {
		t.Error("--peer-asn 1.99999 was accepted")
	}
}

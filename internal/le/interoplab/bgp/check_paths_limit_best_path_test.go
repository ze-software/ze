package bgp

import (
	"encoding/json"
	"testing"
)

// TestPathsLimitBestPathPredicates drives both polarities of the readings
// checkPathsLimitBestPath makes of FRR's table, with no container. Arrival order
// in place of election order, a path left behind after its demotion, a demoted
// path back under a new identifier, and a path from another source must each be
// refused.
func TestPathsLimitBestPathPredicates(t *testing.T) {
	const ze = "172.30.0.2"
	type path struct {
		ID     uint32 `json:"addpathRxId"`
		Metric uint32 `json:"metric"`
		Valid  bool   `json:"valid"`
		Peer   struct {
			ID string `json:"peerId"`
		} `json:"peer"`
	}
	answer := func(peer string, medToID map[uint32]uint32) string {
		document := map[string]any{"prefix": pathsLimitBestPathPrefix}
		paths := []path{}
		for med, id := range medToID {
			entry := path{ID: id, Metric: med, Valid: true}
			entry.Peer.ID = peer
			paths = append(paths, entry)
		}
		document["paths"] = paths
		data, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	decode := func(t *testing.T, output string) map[uint32]uint32 {
		t.Helper()
		held, err := decodeFRRPathsByMED(output, ze)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		return held
	}

	t.Run("election order", func(t *testing.T) {
		best := pathsLimitBestPathSteps[2].want
		if !pathsLimitBestPathHolds(decode(t, answer(ze, map[uint32]uint32{100: 3, 200: 2})), best) {
			t.Fatal("the best two were refused")
		}
		if pathsLimitBestPathHolds(decode(t, answer(ze, map[uint32]uint32{300: 1, 200: 2})), best) {
			t.Fatal("the first two to arrive passed as the best two")
		}
		if pathsLimitBestPathHolds(decode(t, answer(ze, map[uint32]uint32{300: 1, 200: 2, 100: 3})), best) {
			t.Fatal("a demoted path left in place passed")
		}
		if pathsLimitBestPathHolds(decode(t, answer(ze, map[uint32]uint32{100: 3})), best) {
			t.Fatal("one path passed for two")
		}
	})
	t.Run("returning identifier", func(t *testing.T) {
		if err := requirePathsLimitSameID(map[uint32]uint32{100: 3, 200: 2}, 2); err != nil {
			t.Fatalf("the original identifier was refused: %v", err)
		}
		if requirePathsLimitSameID(map[uint32]uint32{100: 3, 200: 5}, 2) == nil {
			t.Fatal("a new identifier for the returning path passed")
		}
		if requirePathsLimitSameID(map[uint32]uint32{100: 3, 50: 4}, 2) == nil {
			t.Fatal("a path that never returned passed")
		}
	})
	t.Run("absent prefix is an empty table", func(t *testing.T) {
		if held := decode(t, "{}"); len(held) != 0 {
			t.Fatalf("held = %v for an absent prefix", held)
		}
	})
	t.Run("unusable answers are refused", func(t *testing.T) {
		if _, err := decodeFRRPathsByMED(answer("172.30.0.4", map[uint32]uint32{100: 1}), ze); err == nil {
			t.Fatal("a path from another source was read as relayed by Ze")
		}
		if _, err := decodeFRRPathsByMED(`{"prefix":"10.72.0.0/24","paths":[{"metric":100,"valid":true,"peer":{"peerId":"172.30.0.2"}}]}`, ze); err == nil {
			t.Fatal("a path with no received identifier was read")
		}
		repeated := `{"prefix":"10.72.0.0/24","paths":[` +
			`{"addpathRxId":1,"metric":100,"valid":true,"peer":{"peerId":"172.30.0.2"}},` +
			`{"addpathRxId":2,"metric":100,"valid":true,"peer":{"peerId":"172.30.0.2"}}]}`
		if _, err := decodeFRRPathsByMED(repeated, ze); err == nil {
			t.Fatal("two paths with one MED were read as one")
		}
		if _, err := decodeFRRPathsByMED(`{"prefix":"10.73.0.0/24","paths":[]}`, ze); err == nil {
			t.Fatal("an answer for another prefix was read")
		}
	})
}

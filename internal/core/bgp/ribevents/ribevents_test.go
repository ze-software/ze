package ribevents

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestReplayBatchJSONTagsStable pins the external JSON contract for the replay
// marker on (bgp-rib, best-change): a full-table replay batch marshals
// `"replay":true`, and an incremental batch omits the key entirely. It is
// round-trip based (decode a fixed wire literal, re-encode) so it holds both
// before and after the Replay-bool -> token-derived-marker migration
// (spec-unify-replay, A-4/AC-7). External plugin processes decode this wire, so
// the tag must never move.
func TestReplayBatchJSONTagsStable(t *testing.T) {
	// Replay batch: "replay":true survives a decode/encode round-trip.
	const replayWire = `{"protocol":"bgp","family":"ipv4/unicast","replay":true,"changes":[]}`
	var rb BestChangeBatch
	if err := json.Unmarshal([]byte(replayWire), &rb); err != nil {
		t.Fatalf("unmarshal replay: %v", err)
	}
	out, err := json.Marshal(&rb)
	if err != nil {
		t.Fatalf("marshal replay: %v", err)
	}
	if !strings.Contains(string(out), `"replay":true`) {
		t.Errorf("replay batch must marshal \"replay\":true, got %s", out)
	}

	// Incremental batch: no "replay" key on the wire.
	const incWire = `{"protocol":"bgp","family":"ipv4/unicast","changes":[]}`
	var ib BestChangeBatch
	if err := json.Unmarshal([]byte(incWire), &ib); err != nil {
		t.Fatalf("unmarshal incremental: %v", err)
	}
	out2, err := json.Marshal(&ib)
	if err != nil {
		t.Fatalf("marshal incremental: %v", err)
	}
	if strings.Contains(string(out2), `"replay"`) {
		t.Errorf("incremental batch must omit \"replay\", got %s", out2)
	}
}

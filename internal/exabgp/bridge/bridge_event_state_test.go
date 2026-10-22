package bridge

import (
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestDownReasonExabgpWords checks that a `down` state event states the
// reason in ExaBGP's words on both encoders, for every reason ze closes a
// session for.
//
// VALIDATES: exabgpDownReasons reaches the v4 text line and the v4 JSON line
// through the path a script's line takes (ReadEvent, then Event.Line).
// PREVENTS: a helper reading ze's own word (`tcp-failure`) where ExaBGP writes
// `closing connection`, or a JSON down line with no reason at all.
func TestDownReasonExabgpWords(t *testing.T) {
	cases := []struct {
		reason string
		words  string
	}{
		{rpc.ReasonTCPFailure, "closing connection"},
		{rpc.ReasonLocalClose, "closing connection"},
		{rpc.ReasonPeerRemoved, "stop, message [removed]"},
		{rpc.ReasonNotification, "notification"},
		{"a-reason-ze-adds-later", "a-reason-ze-adds-later"},
	}
	discard := slog.New(slog.DiscardHandler)
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			event := ReadEvent(map[string]any{
				"type": "bgp",
				"bgp": map[string]any{
					"message": map[string]any{"type": "state"},
					"peer":    map[string]any{"remote": map[string]any{"address": "10.0.0.2", "as": float64(65001)}},
					"state":   "down",
					"reason":  tc.reason,
				},
			})
			text := string(event.Line(FormatV4Text, discard))
			if want := "neighbor 10.0.0.2 down - " + tc.words + "\n"; text != want {
				t.Errorf("text line %q, want %q", text, want)
			}
			var line map[string]any
			if err := json.Unmarshal(event.Line(FormatV4JSON, discard), &line); err != nil {
				t.Fatalf("json line: %v", err)
			}
			neighbor, _ := line["neighbor"].(map[string]any)
			if got := neighbor["reason"]; got != tc.words {
				t.Errorf("json reason %v, want %q", got, tc.words)
			}
		})
	}
}

// TestUpCarriesNoReason checks that an `up` line has no `reason` member:
// ExaBGP's Response.JSON.up writes none.
func TestUpCarriesNoReason(t *testing.T) {
	event := ReadEvent(map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message": map[string]any{"type": "state"},
			"peer":    map[string]any{"remote": map[string]any{"address": "10.0.0.2", "as": float64(65001)}},
			"state":   "up",
		},
	})
	var line map[string]any
	if err := json.Unmarshal(event.Line(FormatV4JSON, slog.New(slog.DiscardHandler)), &line); err != nil {
		t.Fatalf("json line: %v", err)
	}
	neighbor, _ := line["neighbor"].(map[string]any)
	if _, has := neighbor["reason"]; has {
		t.Errorf("up line carries a reason: %v", neighbor)
	}
}

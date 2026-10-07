package persist

import "testing"

// Text decoding preserves all token bits and rejects malformed token fields.
func TestPersistStateReplayToken(t *testing.T) {
	for _, tc := range []struct {
		input string
		token uint64
		valid bool
	}{
		{"initial-replay 18446744073709551615", ^uint64(0), true},
		{"", 0, true},
		{"initial-replay 0", 0, true},
		{"initial-replay nope", 0, false},
		{"initial-replay 18446744073709551616", 0, false},
		{"initial-replay", 0, false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			event := parsePersistState("peer 10.0.0.1 remote as 65001 state up " + tc.input)
			if !tc.valid {
				if event != nil {
					t.Fatal("malformed replay token accepted")
				}
				return
			}
			if event == nil {
				t.Fatal("valid state event rejected")
			}
			if event.initialReplay != tc.token {
				t.Fatalf("initial replay = %d, want %d", event.initialReplay, tc.token)
			}
		})
	}
}

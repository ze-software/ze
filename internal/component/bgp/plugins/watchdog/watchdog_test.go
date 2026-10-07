package watchdog

import (
	"testing"
)

// VALIDATES: Text state event parsing extracts peer address and state
// PREVENTS: State events silently ignored or wrong peer extracted

func TestParseStateEvent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantAddr string
		wantSt   string
	}{
		{
			name:     "state up",
			input:    "peer 10.0.0.1 remote as 65001 state up\n",
			wantAddr: "10.0.0.1",
			wantSt:   "up",
		},
		{
			name:     "state down",
			input:    "peer 10.0.0.2 remote as 65002 state down",
			wantAddr: "10.0.0.2",
			wantSt:   "down",
		},
		{
			name:     "state connected",
			input:    "peer 10.0.0.1 remote as 65001 state connected\n",
			wantAddr: "10.0.0.1",
			wantSt:   "connected",
		},
		{
			name:     "empty string",
			input:    "",
			wantAddr: "",
			wantSt:   "",
		},
		{
			name:     "not a peer event",
			input:    "update direction received\n",
			wantAddr: "",
			wantSt:   "",
		},
		{
			name:     "too short",
			input:    "peer 10.0.0.1\n",
			wantAddr: "",
			wantSt:   "",
		},
		{
			name:     "no state token",
			input:    "peer 10.0.0.1 remote as 65001\n",
			wantAddr: "",
			wantSt:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, st, token := parseStateEvent(tt.input)
			if addr != tt.wantAddr {
				t.Errorf("addr = %q, want %q", addr, tt.wantAddr)
			}
			if st != tt.wantSt {
				t.Errorf("state = %q, want %q", st, tt.wantSt)
			}
			if token != 0 {
				t.Errorf("initial replay = %d, want 0 for a tokenless event", token)
			}
		})
	}
}

// Text transport preserves the full decimal token and rejects malformed values.
func TestParseStateEventReplayToken(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		token uint64
		valid bool
	}{
		{"captured", "initial-replay 18446744073709551615", ^uint64(0), true},
		{"missing", "", 0, true},
		{"zero", "initial-replay 0", 0, true},
		{"malformed", "initial-replay nope", 0, false},
		{"overflow", "initial-replay 18446744073709551616", 0, false},
		{"truncated", "initial-replay", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer, state, token := parseStateEvent("peer 192.0.2.1 remote as 65001 state up " + tc.input)
			if tc.valid {
				if peer != "192.0.2.1" || state != "up" || token != tc.token {
					t.Fatalf("parsed (%q, %q, %d), want peer up with token %d", peer, state, token, tc.token)
				}
			} else if peer != "" {
				t.Fatalf("malformed event accepted for peer %q", peer)
			}
		})
	}
}

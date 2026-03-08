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
			input:    "peer 10.0.0.1 asn 65001 state up\n",
			wantAddr: "10.0.0.1",
			wantSt:   "up",
		},
		{
			name:     "state down",
			input:    "peer 10.0.0.2 asn 65002 state down",
			wantAddr: "10.0.0.2",
			wantSt:   "down",
		},
		{
			name:     "state connected",
			input:    "peer 10.0.0.1 asn 65001 state connected\n",
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
			input:    "peer 10.0.0.1 asn 65001\n",
			wantAddr: "",
			wantSt:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, st := parseStateEvent(tt.input)
			if addr != tt.wantAddr {
				t.Errorf("addr = %q, want %q", addr, tt.wantAddr)
			}
			if st != tt.wantSt {
				t.Errorf("state = %q, want %q", st, tt.wantSt)
			}
		})
	}
}

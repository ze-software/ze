package procfs

import "testing"

func TestParseHexAddr(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		want string
	}{
		{"ipv4-loopback", "0100007F", "127.0.0.1"},
		{"ipv4-any", "00000000", "0.0.0.0"},
		{"ipv4-192.168.1.1", "0101A8C0", "192.168.1.1"},
		{"ipv6-loopback", "00000000000000000000000001000000", "::1"},
		{"invalid-length", "ABC", "ABC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseHexAddr(tt.hex)
			if got != tt.want {
				t.Errorf("ParseHexAddr(%q) = %q, want %q", tt.hex, got, tt.want)
			}
		})
	}
}

func TestParseHexPort(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		want int
	}{
		{"port-80", "0050", 80},
		{"port-443", "01BB", 443},
		{"port-179", "00B3", 179},
		{"port-0", "0000", 0},
		{"port-65535", "FFFF", 65535},
		{"invalid", "GGGG", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseHexPort(tt.hex)
			if got != tt.want {
				t.Errorf("ParseHexPort(%q) = %d, want %d", tt.hex, got, tt.want)
			}
		})
	}
}

func TestTCPStateString(t *testing.T) {
	tests := []struct {
		state int
		want  string
	}{
		{TCPEstablished, "ESTABLISHED"},
		{TCPListen, "LISTEN"},
		{TCPTimeWait, "TIME_WAIT"},
		{TCPCloseWait, "CLOSE_WAIT"},
		{TCPSynSent, "SYN_SENT"},
		{99, "UNKNOWN(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := TCPStateString(tt.state)
			if got != tt.want {
				t.Errorf("TCPStateString(%d) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

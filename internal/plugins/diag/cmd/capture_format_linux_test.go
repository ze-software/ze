// VALIDATES: the packet-capture pure formatters — formatTCPFlags decodes the TCP
// flag byte into a fixed-order name list, extractCountFilter pulls a positive
// "count N" out of the arg list, and joinFilterParts space-joins BPF parts.
// PREVENTS: a mis-decoded TCP flag set in capture output, a bad/zero count filter
// silently disabling the limit, or a malformed joined filter expression.

//go:build linux

package cmd

import "testing"

func TestFormatTCPFlags(t *testing.T) {
	for _, tc := range []struct {
		b    byte
		want string
	}{
		{0x00, "[]"},
		{0x02, "[SYN]"},
		{0x10, "[ACK]"},
		{0x12, "[SYN,ACK]"},     // SYN|ACK
		{0x11, "[ACK,FIN]"},     // ACK|FIN (0x10|0x01)
		{0x13, "[SYN,ACK,FIN]"}, // SYN|ACK|FIN (0x02|0x10|0x01)
		{0x18, "[ACK,PSH]"},     // ACK|PSH
		{0xFF, "[SYN,ACK,FIN,RST,PSH,URG]"},
	} {
		if got := formatTCPFlags(tc.b); got != tc.want {
			t.Errorf("formatTCPFlags(%#02x) = %q, want %q", tc.b, got, tc.want)
		}
	}
}

func TestExtractCountFilter(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{"present", []string{"count", "5"}, 5},
		{"present mid", []string{"host", "10.0.0.1", "count", "42"}, 42},
		{"absent", []string{"host", "10.0.0.1"}, 0},
		{"trailing keyword", []string{"count"}, 0},
		{"non-numeric", []string{"count", "abc"}, 0},
		{"zero", []string{"count", "0"}, 0},
		{"negative", []string{"count", "-3"}, 0},
	} {
		if got := extractCountFilter(tc.args); got != tc.want {
			t.Errorf("%s: extractCountFilter(%v) = %d, want %d", tc.name, tc.args, got, tc.want)
		}
	}
}

func TestJoinFilterParts(t *testing.T) {
	if got := joinFilterParts(nil); got != "" {
		t.Errorf("empty = %q, want \"\"", got)
	}
	if got := joinFilterParts([]string{"tcp", "and", "port", "179"}); got != "tcp and port 179" {
		t.Errorf("join = %q, want %q", got, "tcp and port 179")
	}
}

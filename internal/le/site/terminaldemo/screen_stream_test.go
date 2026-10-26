package siteterminaldemo

import (
	"fmt"
	"strings"
	"testing"
)

// VALIDATES: terminal control sequences retain their meaning across arbitrary
// PTY read boundaries, including a split escape introducer or OSC terminator.
func TestTerminalScreenPreservesSplitEscapes(t *testing.T) {
	cases := []struct {
		name  string
		start string
		wire  string
		want  string
	}{
		{"cursor and delete", "prefix 1-profile.conf", "\x1b[1;8H2-peer.\x1b[3P", "prefix 2-peer.conf"},
		{"title bell", "AB", "\x1b]0;not displayed\aZ", "ABZ"},
		{"title string terminator", "AB", "\x1b]0;not displayed\x1b\\Z", "ABZ"},
		{"saved cursor", "AB", "\x1b7\rX\x1b8Y", "XBY"},
		{"character set", "AB", "\x1b(BZ", "ABZ"},
	}
	for _, test := range cases {
		for split := 0; split <= len(test.wire); split++ {
			t.Run(fmt.Sprintf("%s/split-%d", test.name, split), func(t *testing.T) {
				screen := newTerminalScreen(10, 80)
				screen.settle(test.start)
				screen.settle(test.wire[:split])
				screen.settle(test.wire[split:])
				if got := strings.TrimRight(screen.text(), " "); got != test.want {
					t.Fatalf("painted text = %q, want %q", got, test.want)
				}
			})
		}
	}
}

// VALIDATES: a control-only change is visible to transcript consumers, even
// when no printable character accompanies it to mark the row as changed.
func TestTerminalScreenObservesControlOnlyEdits(t *testing.T) {
	cases := []struct {
		name string
		wire string
		want string
	}{
		{"delete", "\r\x1b[2P", "cdef"},
		{"erase", "\r\x1b[2X", "  cdef"},
		{"insert", "\r\x1b[2@", "  abcdef"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			screen := newTerminalScreen(10, 80)
			screen.settle("abcdef")
			before := len(screen.history)
			screen.settle(test.wire)
			if got := screen.text(); got != test.want {
				t.Fatalf("painted text = %q, want %q", got, test.want)
			}
			if len(screen.history) <= before || screen.history[len(screen.history)-1] != test.want {
				t.Fatalf("changed row not observable: before=%d history=%q", before, screen.history)
			}
		})
	}
}

package fixture

import (
	"strings"
	"testing"
)

// TestRSSourceSequenceParse checks that the step grammar is read before the
// daemon is driven: a valid sequence yields its steps with their typed
// operands, and every malformed one is refused with the step it names.
func TestRSSourceSequenceParse(t *testing.T) {
	steps, err := rsSourceSequenceParse([]string{"up", "127.0.0.5", "send", "127.0.0.5", "192.0.2.0/24", "sent", "2", "held", "127.0.0.1", "gone", "127.0.0.1"})
	if err != nil {
		t.Fatalf("valid sequence refused: %v", err)
	}
	if len(steps) != 5 {
		t.Fatalf("got %d steps, want 5", len(steps))
	}
	if steps[1].prefix.String() != "192.0.2.0/24" {
		t.Fatalf("send prefix = %s, want 192.0.2.0/24", steps[1].prefix)
	}
	if steps[2].count != 2 {
		t.Fatalf("sent count = %d, want 2", steps[2].count)
	}
	for _, bad := range []struct {
		args []string
		want string
	}{
		{[]string{"up"}, "has no operand"},
		{[]string{"sent", "two"}, "is not a number"},
		{[]string{"held", "nowhere"}, "is not an IP address"},
		{[]string{"send", "127.0.0.5"}, "has no prefix"},
		{[]string{"send", "127.0.0.5", "10.0.0.0"}, "is not a prefix"},
		{[]string{"wait", "1"}, "is unknown"},
	} {
		_, err := rsSourceSequenceParse(bad.args)
		if err == nil {
			t.Fatalf("%q accepted, want an error containing %q", bad.args, bad.want)
		}
		if !strings.Contains(err.Error(), bad.want) {
			t.Fatalf("%q: error %q does not contain %q", bad.args, err, bad.want)
		}
	}
}

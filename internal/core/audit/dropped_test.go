package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDroppedDiff proves DroppedDiff answers only the lines a discard
// dropped: everything on a whole discard, the dropped change alone on a path
// discard, and a repeated line once per drop. It feeds hand-written before
// and after diffs and compares the answer.
// VALIDATES: round-8 ISSUE 1, the shared config-discard detail.
// PREVENTS: a path discard's audit row naming a change the discard kept.
func TestDroppedDiff(t *testing.T) {
	cases := []struct {
		name   string
		before string
		after  string
		want   string
	}{
		{"whole discard", "+ bgp router-id 10.0.0.2\n+ description changed\n", "", "+ bgp router-id 10.0.0.2\n+ description changed\n"},
		{"path discard keeps the rest", "+ bgp router-id 10.0.0.2\n+ description changed\n", "+ bgp router-id 10.0.0.2\n", "+ description changed\n"},
		{"nothing pending", "", "", ""},
		{"nothing dropped", "+ a\n", "+ a\n", ""},
		{"repeated line counted", "- }\n- }\n+ x\n", "- }\n", "- }\n+ x\n"},
		{"last line without newline", "+ a\n+ b", "+ b\n", "+ a\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, DroppedDiff(tc.before, tc.after))
		})
	}
}

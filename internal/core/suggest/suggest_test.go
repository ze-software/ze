package suggest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// VALIDATES: Command returns closest match when within distance threshold.
// PREVENTS: wrong or missing suggestions for typos.
func TestCommand(t *testing.T) {
	commands := []string{"bgp", "config", "schema", "validate", "show", "run", "plugin", "cli", "signal"}
	tests := []struct {
		input string
		want  string
	}{
		{"bgpp", "bgp"},
		{"conifg", "config"},
		{"shcema", "schema"},
		{"validte", "validate"},
		{"shw", "show"},
		{"plgin", "plugin"},
		{"singal", "signal"},
		{"xxxxx", ""},  // too far from anything
		{"", ""},       // empty input
		{"bgp", "bgp"}, // exact match
	}

	for _, tt := range tests {
		got := Command(tt.input, commands)
		assert.Equal(t, tt.want, got, "Command(%q)", tt.input)
	}
}

// VALIDATES: Correction proposes a value only within one edit for an input of
// up to four characters, and within a third of its length beyond that.
// PREVENTS: a fix line naming a value the operator never meant, as
// `fix: color blue;` for "true", which Command's half-length bound allows.
func TestCorrection(t *testing.T) {
	tests := []struct {
		input      string
		candidates []string
		want       string
	}{
		{"requre", []string{"enable", "disable", "require", "refuse"}, "require"}, // 6 chars, 1 edit, bound 2
		{"custmer", []string{"customer", "provider", "peer"}, "customer"},         // 7 chars, 1 edit, bound 2
		{"true", []string{"red", "blue"}, ""},                                     // 4 chars, 2 edits, bound 1
		{"rad", []string{"red", "blue"}, "red"},                                   // 3 chars, 1 edit, bound 1
		{"bleu", []string{"red", "blue"}, ""},                                     // 4 chars, 2 edits (a swap), bound 1
		{"enabel", []string{"enable", "disable"}, "enable"},                       // 6 chars, 2 edits, bound 2
		{"cuxtomxrz", []string{"customer"}, "customer"},                           // 9 chars, 3 edits, bound 3
		{"cuxtomxr", []string{"customer"}, "customer"},                            // 8 chars, 2 edits, bound 2
		{"xuxtomxr", []string{"customer"}, ""},                                    // 8 chars, 3 edits, bound 2
		{"cstmr", []string{"customer"}, ""},                                       // 5 chars, 3 edits, bound 1
		{"maybe", []string{"enable", "disable", "require", "refuse"}, ""},
		{"", []string{"red", "blue"}, ""},
		{"blue", []string{"red", "blue"}, "blue"}, // exact match
		{"red", nil, ""},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, Correction(tt.input, tt.candidates), "Correction(%q, %v)", tt.input, tt.candidates)
	}

	// The same input Command corrects: the two bounds differ only where a
	// value goes into a fix line.
	assert.Equal(t, "blue", Command("true", []string{"red", "blue"}), "Command keeps its half-length bound")
}

// VALIDATES: Command returns "" for empty candidates.
// PREVENTS: panic on nil/empty slice.
func TestCommandNoCandidates(t *testing.T) {
	assert.Equal(t, "", Command("bgp", nil), "nil candidates")
	assert.Equal(t, "", Command("bgp", []string{}), "empty candidates")
}

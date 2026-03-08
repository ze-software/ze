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

// VALIDATES: Command returns "" for empty candidates.
// PREVENTS: panic on nil/empty slice.
func TestCommandNoCandidates(t *testing.T) {
	assert.Equal(t, "", Command("bgp", nil), "nil candidates")
	assert.Equal(t, "", Command("bgp", []string{}), "empty candidates")
}

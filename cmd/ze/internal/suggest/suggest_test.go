package suggest

import "testing"

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
		if got != tt.want {
			t.Errorf("Command(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// VALIDATES: Command returns "" for empty candidates.
// PREVENTS: panic on nil/empty slice.
func TestCommandNoCandidates(t *testing.T) {
	got := Command("bgp", nil)
	if got != "" {
		t.Errorf("Command with nil candidates = %q, want empty", got)
	}
	got = Command("bgp", []string{})
	if got != "" {
		t.Errorf("Command with empty candidates = %q, want empty", got)
	}
}

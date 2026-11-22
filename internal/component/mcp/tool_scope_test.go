package mcp

import "testing"

// TestIsCommandReadOnlyFollowsTheDispatcherMatch checks the scope classifier
// against how the dispatcher resolves a line: words compared case-insensitively,
// and a value bound between two words of a longer path.
//
// VALIDATES: a line the dispatcher may resolve to a mutating command is not
// read-only, even when a shorter read-only path also matches it; read-only
// lines, a peer selector and a read-only pipe stay read-only.
// PREVENTS: an mcp.read identity running a mutating command by spelling a path
// word in capitals, or by putting a value inside a longer path, both of which
// the classifier matched to the shorter read-only command.
func TestIsCommandReadOnlyFollowsTheDispatcherMatch(t *testing.T) {
	infos := []CommandInfo{
		{Name: "show config", ReadOnly: true},
		{Name: "show config migrate", ReadOnly: false},
		{Name: "show interface", ReadOnly: true},
		{Name: "show interface reset counters", ReadOnly: false, TakesSelector: true},
		{Name: "show bgp peer detail", ReadOnly: true, TakesSelector: true},
	}
	cases := []struct {
		line string
		want bool
	}{
		{"show config", true},
		{"show config | count", true},
		{"show bgp peer 192.0.2.1 detail", true},
		{"show interface eth0", true},
		// The dispatcher compares path words case-insensitively
		// (command.MatchInvocationPath), so this runs `show config migrate`.
		{"show config MIGRATE /tmp/x", false},
		// The dispatcher binds eth0 between `interface` and `reset`.
		{"show interface eth0 reset counters", false},
		{"show config | save /tmp/x", false},
		// An unfinished quote is refused by the dispatcher's scanner too.
		{`show config "unfinished`, false},
		{"show mystery", false},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			if got := isCommandReadOnly(tc.line, infos); got != tc.want {
				t.Fatalf("isCommandReadOnly(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

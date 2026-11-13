// Design: docs/guide/config-editor.md -- quoted pipes remain literal filter arguments.

package web

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestWebTerminalPipeLiteralArgument posts the operator's complete line through
// the real terminal handler, retaining the path and the exact selected lines.
func TestWebTerminalPipeLiteralArgument(t *testing.T) {
	for _, position := range []string{"root", "context", "top", "nested top"} {
		t.Run(position, func(t *testing.T) {
			_, terminal := newYANGTerminal(t)
			for _, key := range []string{"pipe|one", "pipe|two", "plain-sentinel"} {
				accepted := webTerminalLine(t, terminal, "", `set bgp group "`+key+`" session capability route-refresh enable`)
				if strings.HasPrefix(accepted.Output, "error") {
					t.Fatal(accepted.Output)
				}
			}
			path, prefix := "", ""
			if position != "root" {
				path = "bgp"
			}
			if position == "top" {
				prefix = "top "
			}
			if position == "nested top" {
				prefix = "top top "
			}
			shown := webTerminalLine(t, terminal, path, prefix+"show")
			var want []string
			for line := range strings.SplitSeq(shown.Output, "\n") {
				if strings.Contains(line, "pipe|one") || strings.Contains(line, "pipe|two") {
					want = append(want, line)
				}
			}
			if len(want) != 2 {
				t.Fatalf("need two literal-pipe lines in actual config, got %q", shown.Output)
			}
			for _, quote := range []string{`"|"`, `'|'`} {
				for _, chain := range []bool{false, true} {
					line := prefix + "show | match " + quote
					expected := strings.Join(want, "\n")
					if chain {
						line += " | head 1"
						expected = want[0]
					}
					filtered := webTerminalLine(t, terminal, path, line)
					if filtered.Output != expected {
						t.Errorf("%s: got %q, want exactly %q", line, filtered.Output, expected)
					}
					if filtered.Path != path {
						t.Fatalf("operator moved from %q to %q", path, filtered.Path)
					}
				}
			}
		})
	}
}

// TestWebTerminalUnfinishedQuoteRefusesWithoutChangingCandidate posts malformed
// secret input through the terminal while preserving the complete candidate.
func TestWebTerminalUnfinishedQuoteRefusesWithoutChangingCandidate(t *testing.T) {
	const path = "set bgp peer peer1 connection md5 password "
	for _, quote := range []string{`"`, `'`} {
		t.Run(quote, func(t *testing.T) {
			mgr, terminal := newYANGTerminal(t)
			seeded := webTerminalLine(t, terminal, "", path+"preserved-password")
			if strings.HasPrefix(seeded.Output, "error") {
				t.Fatal(seeded.Output)
			}
			before := config.Serialize(mgr.Tree("testuser"), mgr.schema)
			const secret = "private unfinished quote sentinel"
			input := path + quote + secret
			refused := webTerminalLine(t, terminal, "", input)
			if !strings.HasPrefix(refused.Output, "error: the command has an unfinished quoted value\n") {
				t.Errorf("wrong refusal: %q", refused.Output)
			}
			if lastLine(refused.Output) != "fix: help" {
				t.Errorf("wrong correction: %q", refused.Output)
			}
			if strings.Contains(refused.Output, secret) {
				t.Error("refusal exposed supplied secret")
			}
			if strings.Contains(refused.Output, input) {
				t.Error("refusal echoed command input")
			}
			if refused.Path != "" {
				t.Errorf("refusal moved operator to %q", refused.Path)
			}
			if got := config.Serialize(mgr.Tree("testuser"), mgr.schema); got != before {
				t.Error("unfinished quote changed the complete candidate")
			}
			accepted := webTerminalLine(t, terminal, "", path+quote+"accepted-password"+quote)
			if strings.HasPrefix(accepted.Output, "error") {
				t.Fatal(accepted.Output)
			}
			password, ok := mgr.Tree("testuser").GetContainer("bgp").GetList("peer")["peer1"].GetContainer("connection").GetContainer("md5").Get("password")
			if !ok {
				t.Fatal("completed quoted command did not store the password")
			}
			if password != "accepted-password" {
				t.Error("completed quoted command stored the wrong password")
			}
		})
	}
}

// Design: docs/guide/config-editor.md -- quoted pipes remain literal filter arguments.

package cli

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestModelPipeLiteralArgument exercises the actual dispatch route, including
// the root command behind top and a further structural pipe after the literal.
func TestModelPipeLiteralArgument(t *testing.T) {
	for _, position := range []string{"root", "context", "top", "nested top"} {
		t.Run(position, func(t *testing.T) {
			m, _ := topTestModel(t)
			for _, key := range []string{"pipe|one", "pipe|two", "plain-sentinel"} {
				_, err := m.dispatchCommand(`set bgp group "` + key + `" session capability route-refresh enable`)
				if err != nil {
					t.Fatal(err)
				}
			}
			prefix := ""
			if position != "root" {
				m.contextPath = []string{"bgp"}
			}
			if position == "top" {
				prefix = "top "
			}
			if position == "nested top" {
				prefix = "top top "
			}
			before := slices.Clone(m.contextPath)
			showModel := m
			shown, err := showModel.dispatchCommand(prefix + "show")
			if err != nil {
				t.Fatal(err)
			}
			if shown.configView == nil {
				t.Fatal("unfiltered show must return the config view")
			}
			var want []string
			for line := range strings.SplitSeq(shown.configView.content, "\n") {
				if strings.Contains(line, "pipe|one") || strings.Contains(line, "pipe|two") {
					want = append(want, line)
				}
			}
			if len(want) != 2 {
				t.Fatalf("need two literal-pipe lines in actual config, got %q", shown.configView.content)
			}
			for _, quote := range []string{`"|"`, `'|'`} {
				for _, chain := range []bool{false, true} {
					line := prefix + "show | match " + quote
					expected := strings.Join(want, "\n")
					if chain {
						line += " | head 1"
						expected = want[0]
					}
					// Dispatch runs on the same per-command copy as executeCommand.
					cp := m
					filtered, err := cp.dispatchCommand(line)
					if err != nil {
						t.Fatal(err)
					}
					if filtered.output != expected {
						t.Errorf("%s: got %q, want exactly %q", line, filtered.output, expected)
					}
					if !slices.Equal(before, m.contextPath) {
						t.Fatalf("operator moved from %v to %v", before, m.contextPath)
					}
				}
			}
		})
	}
}

// TestModelUnfinishedQuoteRefusesWithoutChangingCandidate exercises the strict
// command parser's refusal through dispatch without publishing secret input.
func TestModelUnfinishedQuoteRefusesWithoutChangingCandidate(t *testing.T) {
	const path = "set bgp peer peer1 connection md5 password "
	for _, quote := range []string{`"`, `'`} {
		t.Run(quote, func(t *testing.T) {
			m, _ := topTestModel(t)
			if _, err := m.dispatchCommand(path + "preserved-password"); err != nil {
				t.Fatal(err)
			}
			before := m.editor.WorkingContent()
			const secret = "private unfinished quote sentinel"
			input := path + quote + secret
			_, err := m.dispatchCommand(input)
			if err == nil {
				t.Fatal("unfinished quote was accepted")
			}
			var operator *opererr.Error
			if !errors.As(err, &operator) {
				t.Fatalf("refusal lost its operator error type: %T", err)
			}
			if !strings.HasPrefix(err.Error(), "the command has an unfinished quoted value\n") {
				t.Errorf("wrong refusal: %q", err.Error())
			}
			if lastLine(err.Error()) != "fix: help" {
				t.Errorf("wrong correction: %q", err.Error())
			}
			if strings.Contains(err.Error(), secret) {
				t.Error("refusal exposed supplied secret")
			}
			if strings.Contains(err.Error(), input) {
				t.Error("refusal echoed command input")
			}
			if got := m.editor.WorkingContent(); got != before {
				t.Error("unfinished quote changed the complete candidate")
			}
			if _, err := m.dispatchCommand(path + quote + "accepted-password" + quote); err != nil {
				t.Fatal(err)
			}
			password, ok := m.editor.tree.GetContainer("bgp").GetList("peer")["peer1"].GetContainer("connection").GetContainer("md5").Get("password")
			if !ok {
				t.Fatal("completed quoted command did not store the password")
			}
			if password != "accepted-password" {
				t.Error("completed quoted command stored the wrong password")
			}
		})
	}
}

package contract

import (
	"strings"
	"testing"
)

// TestParseCommit proves the one commit grammar every editor parses.
//
// VALIDATES: spec-session-editor-file-mode-parity AC-20, AC-27 and AC-28: each
// subcommand parses, `force` is a trailing modifier of `commit now` and
// `commit confirmed <seconds>` only, the seconds are required and bounded, and
// each refusal names what to type instead.
// PREVENTS: plain `commit`, `commit force` or a `force` on accept, abort or
// verify being accepted, and an editor parsing its own variant.
func TestParseCommit(t *testing.T) {
	accepted := []struct {
		input string
		want  CommitRequest
	}{
		{"now", CommitRequest{Action: CommitNow}},
		{"now force", CommitRequest{Action: CommitNow, Force: true}},
		{"confirmed 60", CommitRequest{Action: CommitConfirmed, Seconds: 60}},
		{"confirmed 1", CommitRequest{Action: CommitConfirmed, Seconds: 1}},
		{"confirmed 3600 force", CommitRequest{Action: CommitConfirmed, Seconds: 3600, Force: true}},
		{"accept", CommitRequest{Action: CommitAccept}},
		{"abort", CommitRequest{Action: CommitAbort}},
		{"verify", CommitRequest{Action: CommitVerify}},
	}
	for _, tc := range accepted {
		t.Run("accept "+tc.input, func(t *testing.T) {
			got, err := ParseCommit(strings.Fields(tc.input))
			if err != nil {
				t.Fatalf("ParseCommit(%q): %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("ParseCommit(%q) = %+v, want %+v", tc.input, got, tc.want)
			}
		})
	}

	refused := []struct {
		input string
		want  []string
	}{
		{"", []string{"commit now", "commit confirmed <seconds>", "commit accept", "commit abort", "commit verify", "force"}},
		{"bogus", []string{`"bogus"`, "commit now", "commit verify"}},
		{"force", []string{"modifier", "commit now", "commit confirmed <seconds>"}},
		{"force confirmed 30", []string{"modifier"}},
		{"confirmed", []string{"seconds"}},
		{"confirmed force", []string{"invalid seconds", `"force"`}},
		{"confirmed 0", []string{"at least 1"}},
		{"confirmed 3601", []string{"at most 3600"}},
		{"confirmed -5", []string{"at least 1"}},
		{"confirmed abc", []string{"invalid seconds", `"abc"`}},
		{"confirmed 60 later", []string{`"later"`, "force"}},
		{"now later", []string{`"later"`, "force"}},
		{"now force force", []string{`"force"`}},
		{"accept force", []string{"commit accept", "force", "warnings", "conflict"}},
		{"abort force", []string{"commit abort", "force"}},
		{"verify force", []string{"commit verify", "force"}},
		{"verify extra", []string{"commit verify", `"extra"`}},
	}
	for _, tc := range refused {
		t.Run("refuse "+tc.input, func(t *testing.T) {
			got, err := ParseCommit(strings.Fields(tc.input))
			if err == nil {
				t.Fatalf("ParseCommit(%q) = %+v, want an error", tc.input, got)
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("ParseCommit(%q) error %q does not name %q", tc.input, err, w)
				}
			}
		})
	}
}

// TestCommitSubcommandsListsTheGrammar proves completion and help read the same
// table the parser reads.
//
// VALIDATES: AC-27: completion offers the five subcommands and never `confirm`.
// PREVENTS: a completion list kept by hand beside the parser.
func TestCommitSubcommandsListsTheGrammar(t *testing.T) {
	var keywords []string
	for _, sub := range CommitSubcommands() {
		keywords = append(keywords, sub.Keyword)
		if _, err := ParseCommit(sub.Example()); err != nil {
			t.Errorf("subcommand %q: its own example %v does not parse: %v", sub.Keyword, sub.Example(), err)
		}
	}
	if got := strings.Join(keywords, " "); got != "now confirmed accept abort verify" {
		t.Fatalf("CommitSubcommands keywords = %q", got)
	}
}

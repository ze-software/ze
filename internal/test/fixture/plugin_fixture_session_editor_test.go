// VALIDATES: the session editor driver's script grammar and its PTY read
// boundary (spec-session-editor-file-mode-parity, Phase 1 wiring fixture).
// PREVENTS: a .ci script typo passing silently, and a status line lost because
// it arrived in the same read as the previous needle.

package fixture

import (
	"strings"
	"testing"
)

// TestParseSessionEditorScript holds the script grammar the session editor
// driver reads from each .ci: one verb and its text per line, comments and
// blank lines skipped. The method is a table of scripts, each either parsed to
// its exact steps or refused with an error naming the line.
func TestParseSessionEditorScript(t *testing.T) {
	steps, err := parseSessionEditorScript(strings.NewReader(
		"# comment\n\nsend copy bgp peer peer1 to peer2\nwait Copied peer peer1 to peer2\n" +
			"cli show bgp peer list\nhas peer2\nlacks peer3\nkey ctrl-d\nkill\nhas peer1\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []sessionEditorStep{
		{verb: sessionEditorSend, text: "copy bgp peer peer1 to peer2"},
		{verb: sessionEditorWait, text: "Copied peer peer1 to peer2"},
		{verb: sessionEditorCLI, text: "show bgp peer list"},
		{verb: sessionEditorHas, text: "peer2"},
		{verb: sessionEditorLacks, text: "peer3"},
		{verb: sessionEditorKey, text: "ctrl-d"},
		{verb: sessionEditorKill},
		{verb: sessionEditorHas, text: "peer1"},
	}
	if len(steps) != len(want) {
		t.Fatalf("steps = %+v, want %+v", steps, want)
	}
	for i := range want {
		if steps[i] != want[i] {
			t.Fatalf("step %d = %+v, want %+v", i, steps[i], want[i])
		}
	}

	refused := []struct {
		name   string
		script string
		line   string
	}{
		{"unknown verb", "send x\ntype y\n", "line 2"},
		{"verb without text", "wait\n", "line 1"},
		{"has before cli", "has peer1\n", "line 1"},
		{"lacks before cli", "send x\nlacks peer1\n", "line 2"},
		{"empty script", "# only a comment\n", "no steps"},
		{"unknown key", "key ctrl-x\n", "line 1"},
		{"key without name", "key\n", "line 1"},
		{"kill with text", "kill now\n", "line 1"},
		{"send after kill", "kill\nsend x\n", "line 2"},
		{"wait after kill", "kill\nwait x\n", "line 2"},
		{"key after kill", "kill\nkey ctrl-d\n", "line 2"},
		{"kill twice", "kill\nkill\n", "line 2"},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseSessionEditorScript(strings.NewReader(tc.script))
			if err == nil {
				t.Fatalf("script %q parsed, want refusal", tc.script)
			}
			if !strings.Contains(err.Error(), tc.line) {
				t.Fatalf("error %q does not name %q", err, tc.line)
			}
		})
	}
}

// TestSessionEditorPendingAfter holds the rule that a wait consumes output only
// up to its needle: text the PTY delivered after the needle in the same read
// stays for the next wait, so two status lines in one chunk both match.
func TestSessionEditorPendingAfter(t *testing.T) {
	rest, ok := sessionEditorPendingAfter([]byte("aaa NEEDLE bbb OTHER"), "NEEDLE")
	if !ok {
		t.Fatal("needle not found")
	}
	if string(rest) != " bbb OTHER" {
		t.Fatalf("rest = %q, want %q", rest, " bbb OTHER")
	}
	if _, ok := sessionEditorPendingAfter([]byte("aaa"), "NEEDLE"); ok {
		t.Fatal("absent needle reported found")
	}
}

// TestSessionEditorUser holds the driver's argument contract: three arguments
// drive the editor as admin, a trailing `user <name>` drives it as that user,
// and any other shape is refused, so a second SSH user (AC-6) is a script
// argument rather than a second driver. The method is a table of argument
// lists, each answered with its user or refused.
func TestSessionEditorUser(t *testing.T) {
	cases := []struct {
		args    []string
		user    string
		refused bool
	}{
		{args: []string{"2222", "c.conf", "s.script"}, user: "admin"},
		{args: []string{"2222", "c.conf", "s.script", "user", "bob"}, user: "bob"},
		{args: []string{"2222", "c.conf"}, refused: true},
		{args: []string{"2222", "c.conf", "s.script", "user"}, refused: true},
		{args: []string{"2222", "c.conf", "s.script", "name", "bob"}, refused: true},
		{args: []string{"2222", "c.conf", "s.script", "user", ""}, refused: true},
	}
	for _, tc := range cases {
		user, err := sessionEditorUser(tc.args)
		if tc.refused {
			if err == nil {
				t.Errorf("%q: answered %q, want refused", tc.args, user)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", tc.args, err)
			continue
		}
		if user != tc.user {
			t.Errorf("%q: user %q, want %q", tc.args, user, tc.user)
		}
	}
}

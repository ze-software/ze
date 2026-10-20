// VALIDATES: the group buffer, its answers and its inline split (X-16 to X-20).
// PREVENTS: a grouped route reaching ze before its group ends.

package bridge

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

// groupRunner records what a batch handed ze, and refuses a command holding
// refuse when refuse is set.
type groupRunner struct {
	dispatched []string
	flushed    []string
	refuse     string
}

func (r *groupRunner) Dispatch(_ context.Context, command string) error {
	if r.refuse != "" && strings.Contains(command, r.refuse) {
		return errors.New("refused by ze")
	}
	r.dispatched = append(r.dispatched, command)
	return nil
}

func (r *groupRunner) Flush(_ context.Context, selector string) {
	r.flushed = append(r.flushed, selector)
}

func (r *groupRunner) Status(context.Context, bool) ([]byte, BatchAnswer) {
	return nil, BatchAnswer{}
}

func (r *groupRunner) Dispatched(string, Command) {}

// groupScript is one script's command side, fed one line per batch as a
// script that waits for each answer writes them.
type groupScript struct {
	lines  ScriptLines
	runner groupRunner
	ack    AckMode
	group  Group
}

func newGroupScript() *groupScript {
	var setting atomic.Uint32
	setting.Store(uint32(APIVersionAuto))
	g := &groupScript{ack: AckMode{enabled: true}}
	log := slog.New(slog.DiscardHandler)
	g.lines = ScriptLines{
		Name: "s", Log: log, API: NewScriptVersion("s", log, EncoderText, &setting),
		Ack: &g.ack, Group: &g.group,
	}
	return g
}

// send feeds one line and answers what the script read back.
func (g *groupScript) send(t *testing.T, line string) string {
	t.Helper()
	var out bytes.Buffer
	g.lines.Batch(context.Background(), &g.runner, &out, []string{line})
	return out.String()
}

func translated(t *testing.T, line string) []string {
	t.Helper()
	translation, err := Translator{}.Line(line)
	if err != nil {
		t.Fatalf("Line(%q): %v", line, err)
	}
	texts := make([]string, 0, len(translation.Commands))
	for _, command := range translation.Commands {
		texts = append(texts, command.Text)
	}
	return texts
}

// TestGroupBlockHoldsUntilEnd checks X-16 and X-17 (AC-18): the route lines of
// a `group start` block reach ze only at `group end`, together and with one
// flush, while an End-of-RIB inside the block is sent as it comes. Method: feed
// a fake runner one line per batch and read what it was handed after each.
func TestGroupBlockHoldsUntilEnd(t *testing.T) {
	g := newGroupScript()
	if got := g.send(t, "group start"); got != "group started\ndone\n" {
		t.Fatalf("group start answered %q", got)
	}
	for _, line := range []string{
		"announce route 1.1.1.1/32 next-hop 11.11.11.11",
		"announce route 2.2.2.2/32 next-hop 11.11.11.11",
	} {
		if got := g.send(t, line); got != "done\n" {
			t.Fatalf("%q in a group answered %q, want done alone", line, got)
		}
	}
	if len(g.runner.dispatched) != 0 {
		t.Fatalf("a held line reached ze before group end: %q", g.runner.dispatched)
	}
	if got := g.send(t, "group end"); got != "group processed: 2 announced, 0 withdrawn\ndone\n" {
		t.Fatalf("group end answered %q", got)
	}
	want := translated(t, "announce attributes next-hop 11.11.11.11 nlri 1.1.1.1/32 2.2.2.2/32")
	if !slices.Equal(g.runner.dispatched, want) {
		t.Fatalf("group end dispatched %q, want %q", g.runner.dispatched, want)
	}
	if !slices.Equal(g.runner.flushed, []string{"*"}) {
		t.Fatalf("group end flushed %q, want one flush of *", g.runner.flushed)
	}

	// API 6: `peer * announce route` is held, `peer * announce eor` is not.
	v6 := newGroupScript()
	v6.send(t, "group start")
	if got := v6.send(t, "peer * announce route 3.3.3.3/32 next-hop 11.11.11.11"); got != "done\n" {
		t.Fatalf("a v6 announce in a group answered %q", got)
	}
	v6.send(t, "peer * announce eor ipv4 unicast")
	if len(v6.runner.dispatched) != 1 {
		t.Fatalf("inside the group ze was handed %q, want the End-of-RIB alone", v6.runner.dispatched)
	}
	got := v6.send(t, "group end")
	if got != `{"status": "group processed", "announced": 1, "withdrawn": 0}`+"\ndone\n" {
		t.Fatalf("v6 group end answered %q", got)
	}
	if len(v6.runner.dispatched) != 2 {
		t.Fatalf("the held v6 route never reached ze: %q", v6.runner.dispatched)
	}
}

// TestGroupAnswers checks X-16 (AC-19, AC-20): each group answer is ExaBGP's
// own (command/group.py). Method: drive one script per case and compare the
// bytes it read back.
func TestGroupAnswers(t *testing.T) {
	cases := []struct {
		name   string
		lines  []string
		refuse string
		want   string // the answer to the last line
	}{
		{"nested start", []string{"group start", "group start"}, "",
			"error: already in group block (nested groups not allowed)\nerror\n"},
		{"stray end", []string{"group end"}, "", "error: not in group block\nerror\n"},
		{"empty group", []string{"group start", "group end"}, "", "group ended (0 commands)\ndone\n"},
		{"unparsable line", []string{
			"group start",
			"announce route 1.1.1.1/32 next-hop 11.11.11.11",
			"announce route 1.1.1.2/32 next-hop 11.11.11.11 frobnicate 1",
			"group end",
		}, "", "group processed: 1 announced, 0 withdrawn, 1 errors\nerror\n"},
		{"refused by ze", []string{
			"group start",
			"announce route 1.1.1.1/32 next-hop 11.11.11.11",
			"withdraw route 1.1.1.3/32",
			"group end",
		}, "1.1.1.3", "group processed: 1 announced, 0 withdrawn, 1 errors\nerror\n"},
		{"unknown action", []string{"group teardown 2 ; withdraw route 1.1.1.3/32"}, "",
			"group processed: 0 announced, 1 withdrawn, 1 errors\nerror\n"},
		{"json errors", []string{"group announce eor ipv4 unicast json"}, "",
			`{"status": "group processed", "announced": 0, "withdrawn": 0, "errors": ["could not parse: announce eor ipv4 unicast json"]}` + "\nerror\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newGroupScript()
			g.runner.refuse = tc.refuse
			var got string
			for _, line := range tc.lines {
				got = g.send(t, line)
			}
			if got != tc.want {
				t.Fatalf("answered %q, want %q", got, tc.want)
			}
		})
	}
}

// TestGroupInlineSplitsOutsideQuotes checks X-18, X-19 and X-20 (AC-21): an
// inline group is split where ExaBGP's lexer ends a statement, applies to
// every peer when it names none, and shares the attributes of its
// `attributes` command with the routes that do not write them.
func TestGroupInlineSplitsOutsideQuotes(t *testing.T) {
	splits := []struct {
		text string
		want []string
	}{
		{"a ; b", []string{"a", "b"}},
		{`a "x ; y" ; b`, []string{`a "x ; y"`, "b"}},
		{`a 'x ; y'`, []string{`a 'x ; y'`}},
		{"a { b ; c } ; d", []string{"a { b ; c }", "d"}},
		{`a \; b`, []string{"a", "b"}}, // an escaped `;` still ends the command
		{`a \" ; b`, []string{`a \" ; b`}},
		{" ; a ;; ", []string{"a"}},
	}
	for _, tc := range splits {
		if got := SplitCommands(tc.text); !slices.Equal(got, tc.want) {
			t.Errorf("SplitCommands(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}

	g := newGroupScript()
	got := g.send(t, "group announce route 1.1.1.1/32 next-hop 11.11.11.11 ; announce route 2.2.2.2/32 next-hop 11.11.11.11")
	if got != "group processed: 2 announced, 0 withdrawn\ndone\n" {
		t.Fatalf("inline group answered %q", got)
	}
	if !slices.Equal(g.runner.flushed, []string{"*"}) {
		t.Fatalf("inline group flushed %q, want every peer", g.runner.flushed)
	}

	shared := newGroupScript()
	shared.send(t, "group attributes next-hop 9.9.9.9 med 5 ; announce route 1.1.1.1/32 ; announce route 2.2.2.2/32 med 7")
	// The two routes differ in their med, so they are two commands.
	want := append(translated(t, "announce route 1.1.1.1/32 next-hop 9.9.9.9 med 5"),
		translated(t, "announce route 2.2.2.2/32 med 7 next-hop 9.9.9.9")...)
	if !slices.Equal(shared.runner.dispatched, want) {
		t.Fatalf("shared attributes dispatched %q, want %q", shared.runner.dispatched, want)
	}
}

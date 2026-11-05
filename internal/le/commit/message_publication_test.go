// Design: docs/contributing/committing.md -- prepared messages belong to their publication blocks.
package commit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPreparedReplacementPublishesDistinctObligations executes a same-tag
// create, replacement and append through the generated script. Only the two
// surviving blocks publish, and their Git trailers alone determine debt.
// MUTATION: reuse a message path, append instead of replace, or read the retired
// debt shard; the message, history or exact obligation assertions fail.
func TestPreparedReplacementPublishesDistinctObligations(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "message-publication")
	base := revParse(t, root, "HEAD")
	// This is the predecessor's actual six-cell row shape, retained as foreign
	// evidence rather than selected for either new commit.
	const shard = "plan/verification-debt/foreign.md"
	const stale = "| 2026-09-05 | foreign | incorrect earlier debt | independent critical review | foreign predecessor row | open |\n"
	writeCommitFixture(t, root, shard, stale)
	writeCommitFixture(t, root, linuxLimitSummary, "# Original preparation\n\n"+linuxLimitRow+linuxLimitAnnotation+"\n")
	first, err := Create(root, &Options{Subject: "original preparation", Tag: "shared",
		Files: []string{linuxLimitSummary}})
	if err != nil {
		t.Fatal(err)
	}
	firstMessage, err := os.ReadFile(filepath.Join(root, first.Message))
	if err != nil {
		t.Fatal(err)
	}
	const heading = "# Replacement preparation\n\n"
	wantSummary := heading + linuxLimitRow + linuxLimitAnnotation + "\n"
	writeCommitFixture(t, root, linuxLimitSummary, wantSummary)
	replacement, err := Create(root, &Options{Subject: "replacement publication", Tag: "shared",
		Files: []string{linuxLimitSummary}, Replace: true, Script: first.Script})
	if err != nil {
		t.Fatal(err)
	}
	if replacement.Script != first.Script {
		t.Fatalf("replacement script = %q, want %q", replacement.Script, first.Script)
	}
	replacementMessage, err := os.ReadFile(filepath.Join(root, replacement.Message))
	if err != nil {
		t.Fatal(err)
	}
	writeCommitFixture(t, root, "notes.txt", "appended content\n")
	appended, err := Create(root, &Options{Subject: "appended publication", Tag: "shared",
		Files: []string{"notes.txt"}, Append: true, Script: replacement.Script})
	if err != nil {
		t.Fatal(err)
	}
	if appended.Script != first.Script {
		t.Fatalf("appended script = %q, want %q", appended.Script, first.Script)
	}
	for _, pair := range [][2]string{{first.Message, replacement.Message}, {first.Message, appended.Message}, {replacement.Message, appended.Message}} {
		if pair[0] == pair[1] {
			t.Fatalf("two preparations share message %q", pair[0])
		}
	}
	for _, message := range []struct {
		path string
		want []byte
	}{{first.Message, firstMessage}, {replacement.Message, replacementMessage}} {
		got, err := os.ReadFile(filepath.Join(root, message.path))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, message.want) {
			t.Fatalf("later preparation changed %s: %q, want %q", message.path, got, message.want)
		}
	}
	if got := revParse(t, root, "HEAD"); got != base {
		t.Fatalf("preparation published HEAD %s, want %s", got, base)
	}
	before := deriveOwed(t, root, "")
	if len(before.Items) != 0 {
		t.Fatalf("preparation or predecessor shard created debt: %+v", before.Items)
	}
	if len(before.Refusals) != 0 {
		t.Fatalf("preparation or predecessor shard refused derivation: %v", before.Refusals)
	}
	runCommitScript(t, root, first.Script)
	if got := runCommitGitOutput(t, root, "log", "--reverse", "--format=%s", base+"..HEAD"); got != "replacement publication\nappended publication\n" {
		t.Fatalf("published subjects = %q", got)
	}
	if got := runCommitGitOutput(t, root, "show", "HEAD^:"+linuxLimitSummary); got != wantSummary {
		t.Fatalf("replacement published summary %q, want %q", got, wantSummary)
	}
	if got := runCommitGitOutput(t, root, "show", "HEAD:notes.txt"); got != "appended content\n" {
		t.Fatalf("append published content %q", got)
	}
	debt := deriveOwed(t, root, "")
	assertOneItem(t, debt, revParse(t, root, "HEAD^"), owedItemLinuxLimit, linuxLimitRID)
	if len(debt.Refusals) != 0 {
		t.Fatalf("published history refused derivation: %v", debt.Refusals)
	}
	wantTrailer := trailerLinuxLimitOwed + linuxLimitRID + ": LL-EXAMPLE"
	if got := strings.Count(runCommitGitOutput(t, root, "log", "--format=%B", base+"..HEAD"), wantTrailer); got != 1 {
		t.Fatalf("published obligation occurs %d times, want once", got)
	}
	if debt.pushRefusal() == nil {
		t.Fatal("the one legitimate Linux-limit obligation did not hold the push")
	}
	gotShard, err := os.ReadFile(filepath.Join(root, shard))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotShard) != stale {
		t.Fatalf("preparation or publication changed foreign shard: %q", gotShard)
	}
	if got := runCommitGitOutput(t, root, "ls-tree", "-r", "--name-only", "HEAD", "--", shard); got != "" {
		t.Fatalf("publication selected the foreign shard: %q", got)
	}
}

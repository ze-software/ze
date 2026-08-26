// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7, AC-11 -- the architecture-list
// generator is called as a function, answers structured data, and rewrites only
// what sits between its markers.
// PREVENTS: a generator that writes different bytes from the script it
// replaces. The lists it owns are read by every agent through ai/INSTRUCTIONS.md
// and CLAUDE.md, and a wrap that moves one name to the next line makes the
// check gate red for everybody with nothing wrong in the tree.

package archmap

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture writes a tree holding the three source directories and an
// instructions file with the marker pairs, and answers its root.
func fixture(t *testing.T, instructions string, dirs ...string) string {
	t.Helper()

	root := t.TempDir()
	for _, rel := range dirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
	}
	for _, source := range sources {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(source.Path)), 0o750); err != nil {
			t.Fatalf("fixture source directory: %v", err)
		}
	}
	path := filepath.Join(root, filepath.FromSlash(instructionsFile))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(instructions), 0o600); err != nil {
		t.Fatalf("fixture file: %v", err)
	}
	return root
}

// markers is an instructions file carrying all three pairs and prose around
// them, so a test can see what a rewrite left alone.
func markers(body string) string {
	var page strings.Builder
	page.WriteString("prose before\n")
	for _, source := range sources {
		page.WriteString("<!-- BEGIN GENERATED: arch-")
		page.WriteString(source.Name)
		page.WriteString(" (scripts/dev/arch_map.py) -->\n")
		page.WriteString(body)
		page.WriteString("<!-- END GENERATED: arch-")
		page.WriteString(source.Name)
		page.WriteString(" -->\n")
		page.WriteString("prose between\n")
	}
	page.WriteString("prose after\n")
	return page.String()
}

func TestWrapKeepsWordsWhole(t *testing.T) {
	if got := Wrap("aaa bbb ccc", 7); got != "aaa bbb\nccc" {
		t.Errorf("Wrap answered %q", got)
	}
}

// The line width is the numeric input of this tool: a line exactly at the
// width stays whole and one character more moves a word down.
func TestWrapBreaksOnlyPastTheWidth(t *testing.T) {
	exact := strings.Repeat("a", 40) + " " + strings.Repeat("b", 37)
	if got := Wrap(exact, 78); strings.Contains(got, "\n") {
		t.Errorf("a line of exactly 78 characters was broken: %q", got)
	}

	over := strings.Repeat("a", 40) + " " + strings.Repeat("b", 38)
	if got := Wrap(over, 78); !strings.Contains(got, "\n") {
		t.Errorf("a line of 79 characters was not broken: %q", got)
	}
}

func TestWrapDoesNotBreakInsideAHyphenatedName(t *testing.T) {
	name := "config-archive-cmd,"
	got := Wrap(strings.Repeat("x", 70)+" "+name, 78)

	if !strings.HasSuffix(got, name) {
		t.Errorf("a hyphenated name was split: %q", got)
	}
}

func TestWrapDoesNotBreakAWordLongerThanTheWidth(t *testing.T) {
	long := strings.Repeat("z", 100)
	if got := Wrap(long, 78); got != long {
		t.Errorf("a single long word was broken: %q", got)
	}
}

func TestDirsSkipsFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o750); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.go"), []byte("x"), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	names, err := Dirs(root)
	if err != nil {
		t.Fatalf("Dirs: %v", err)
	}
	if len(names) != 1 || names[0] != "a" {
		t.Fatalf("Dirs answered %q, want only the directory", names)
	}
}

func TestDirsOverAMissingDirectoryIsAnError(t *testing.T) {
	if _, err := Dirs(filepath.Join(t.TempDir(), "gone")); err == nil {
		t.Fatal("Dirs answered no error over a directory that does not exist")
	}
}

func TestRenderRewritesOnlyBetweenTheMarkers(t *testing.T) {
	root := fixture(t, markers("stale content\n"), "internal/component/bgp")

	rendered, _, err := Render(root, markers("stale content\n"))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, kept := range []string{"prose before\n", "prose between\n", "prose after\n"} {
		if !strings.Contains(rendered, kept) {
			t.Errorf("the rewrite dropped %q", kept)
		}
	}
	if strings.Contains(rendered, "stale content") {
		t.Error("the rewrite kept the old block")
	}
}

// The BEGIN marker carries a comment naming what generates the block, and the
// rewrite starts after that comment CLOSES. A rewrite that started after the
// block's name instead would swallow the rest of the line, leaving a marker no
// later run can find -- and the prose test above would not see it, because
// what it swallows is the marker rather than the prose.
func TestTheMarkerLinesSurviveTheRewriteWhole(t *testing.T) {
	root := fixture(t, markers("stale content\n"), "internal/component/bgp")

	rendered, _, err := Render(root, markers("stale content\n"))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, source := range sources {
		begin := "<!-- BEGIN GENERATED: arch-" + source.Name + " (scripts/dev/arch_map.py) -->"
		if !strings.Contains(rendered, begin) {
			t.Errorf("the rewrite truncated the marker line for arch-%s:\n%s", source.Name, rendered)
		}
		end := "<!-- END GENERATED: arch-" + source.Name + " -->"
		if !strings.Contains(rendered, end) {
			t.Errorf("the rewrite dropped the end marker for arch-%s", source.Name)
		}
	}
}

func TestRenderNamesTheBlockThatHasNoMarker(t *testing.T) {
	root := fixture(t, "no markers here\n")

	_, _, err := Render(root, "no markers here\n")
	if err == nil {
		t.Fatal("Render answered no error over a file with no markers")
	}
	if !strings.Contains(err.Error(), "arch-components") {
		t.Errorf("the error does not name the block: %v", err)
	}
}

func TestRenderCountsWhatEachBlockHolds(t *testing.T) {
	root := fixture(t, markers(""), "internal/component/bgp", "internal/component/cli")

	_, blocks, err := Render(root, markers(""))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(blocks) != len(sources) {
		t.Fatalf("Render answered %d blocks, want %d", len(blocks), len(sources))
	}
	if blocks[0].Directories != 2 {
		t.Errorf("the components block counted %d directories, want 2", blocks[0].Directories)
	}
}

func TestCheckOverACurrentFileIsNotStale(t *testing.T) {
	root := fixture(t, markers(""), "internal/component/bgp")
	if _, err := Update(root); err != nil {
		t.Fatalf("Update: %v", err)
	}

	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Stale {
		t.Errorf("a file just written reads as stale:\n%s", report.Text())
	}
}

func TestCheckAfterADirectoryAppearsIsStale(t *testing.T) {
	root := fixture(t, markers(""), "internal/component/bgp")
	if _, err := Update(root); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "component", "later"), 0o750); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Stale {
		t.Error("a new component directory left the file reading as current")
	}
}

func TestCheckWritesNothing(t *testing.T) {
	root := fixture(t, markers("stale content\n"), "internal/component/bgp")
	before, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(instructionsFile)))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}

	if _, err := Check(root); err != nil {
		t.Fatalf("Check: %v", err)
	}

	after, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(instructionsFile)))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("check mode rewrote the file")
	}
}

func TestUpdateWritesTheListAndSaysSo(t *testing.T) {
	root := fixture(t, markers("stale content\n"), "internal/component/bgp", "internal/component/cli")

	report, err := Update(root)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !report.Written {
		t.Fatalf("Update reported no write:\n%s", report.Text())
	}

	page, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(instructionsFile)))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	if !strings.Contains(string(page), "2 directories under `internal/component/`:") {
		t.Errorf("the written block does not carry the count:\n%s", page)
	}
	if !strings.Contains(string(page), "bgp, cli") {
		t.Errorf("the written block does not carry the names:\n%s", page)
	}
}

func TestUpdateOverACurrentFileWritesNothing(t *testing.T) {
	root := fixture(t, markers(""), "internal/component/bgp")
	if _, err := Update(root); err != nil {
		t.Fatalf("first Update: %v", err)
	}

	report, err := Update(root)
	if err != nil {
		t.Fatalf("second Update: %v", err)
	}
	if report.Written {
		t.Error("a second update rewrote a file that was already current")
	}
}

func TestTextSaysWhichOfTheThreeThingsHappened(t *testing.T) {
	current := Report{File: instructionsFile}
	if !strings.Contains(current.Text(), "up to date") {
		t.Errorf("a current file renders %q", current.Text())
	}

	stale := Report{File: instructionsFile, Stale: true}
	if !strings.Contains(stale.Text(), "stale") {
		t.Errorf("a stale file renders %q", stale.Text())
	}

	written := Report{File: instructionsFile, Stale: true, Written: true}
	if !strings.Contains(written.Text(), "regenerated") {
		t.Errorf("a rewritten file renders %q", written.Text())
	}
}

func TestReportIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	raw, err := json.Marshal(Report{
		File:   instructionsFile,
		Blocks: []Block{{Name: "components", Path: "internal/component", Directories: 2}},
	})
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	for _, want := range []string{`"file"`, `"blocks"`, `"name"`, `"path"`, `"directories"`, `"stale"`, `"written"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the payload has no %s key: %s", want, raw)
		}
	}
	if strings.Contains(string(raw), "_") {
		t.Errorf("a JSON key is snake_case: %s", raw)
	}
}

func TestTheAreaHoldsBothGatesAndOnlyUpdateWrites(t *testing.T) {
	list := Actions()

	if len(list.Actions) != 2 {
		t.Fatalf("the area holds %d actions, want two", len(list.Actions))
	}
	for _, row := range list.Actions {
		switch row.Verb {
		case "check":
			if row.Writes {
				t.Error("check is marked as writing")
			}
		case "update":
			if !row.Writes {
				t.Error("update is not marked as writing")
			}
		default:
			t.Errorf("unexpected verb %q", row.Verb)
		}
	}
}

func TestAnUnknownActionAnswersTwo(t *testing.T) {
	payload, code := Answer([]string{"nonesuch"})

	if payload != nil || code != 2 {
		t.Fatalf("an unknown action answered payload=%v code=%d", payload, code)
	}
}

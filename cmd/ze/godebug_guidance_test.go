//go:build ze_core

// Design: docs/architecture/system-architecture.md -- ze unified entry point
//
// VALIDATES: nothing in the repository tells a reader to set a GODEBUG setting
// that the Go toolchain in use has removed, and every file that still names one
// says it was removed.
// PREVENTS: the failure this file was written for. cmd/ze/main.go told operators
// to set `tlsunsafeekm` to its old value and test/interop-ipsec/scenarios/eap-tls
// set it. Go 1.27 removed that setting, and the runtime raises a fatal error before
// main() when a removed setting carries its old value, so ze died at start for
// anybody who followed either one. The guidance outlived the mechanism it named,
// and no gate could see it. The next setting Go removes breaks this test instead.

package main

import (
	"bytes"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// godebugRemovedRow captures the Name of a removed setting in the toolchain's own
// godebug table. A live row carries no `Removed:` field and so does not match.
var godebugRemovedRow = regexp.MustCompile(`\{Name: "([a-z0-9]+)",[^}]*Removed: \d+`)

// subtractedTrees are the tracked paths this scan does not judge. `vendor` holds
// third-party modules ze did not write, and `testdata` holds inputs to tests,
// including this file's own defect fixture, which exists precisely BECAUSE it
// carries the forbidden text. Neither is guidance ze gives. Every other exclusion
// comes free: the population is what git tracks, so build output, scratch, and a
// downloaded module cache are outside it already.
var subtractedTrees = []string{"vendor", "testdata"}

// pathIsSubtracted answers whether rel lies in a subtracted tree, at any depth.
func pathIsSubtracted(rel string) bool {
	for part := range strings.SplitSeq(rel, "/") {
		if slices.Contains(subtractedTrees, part) {
			return true
		}
	}
	return false
}

// TestNoShippedGuidanceNamesARemovedGODEBUG refuses one shape and requires one.
//
// The refused shape is the ASSIGNABLE form: the literal text `GODEBUG=` followed
// by a value list naming a removed setting, anywhere on any line of any tracked
// file. There is no exemption for a comment, a code span, a doc, or a test.
//
// The bound is therefore stated as one sentence rather than as a list of forms,
// which is what the two earlier versions got wrong. Both enumerated the shapes
// they expected -- start of line, after `export`, outside a comment -- and both
// were then wrong about a shape nobody listed. A rule with no list has nothing to
// leave off. It costs one thing, and the cost is deliberate: a file that needs to
// discuss the setting writes its NAME (`tlsunsafeekm`) and not the form a reader
// could paste.
//
// What it still cannot see, and this list IS incomplete by construction: a value
// composed at runtime (`GODEBUG="$flags"` reads as an empty list), a name split
// across a line break, an `os.Setenv` naming the variable as a Go string, or a
// wrapper that assembles the variable itself. Those need a reader.
//
// The other way to set one, a `//go:debug <setting>=` directive, is NOT checked
// here. The Go compiler already refuses it, measured on Go 1.27.0:
//
//	./main.go:1:1: invalid //go:debug: removed GODEBUG "tlsunsafeekm" set to old
//	value "1" (https://go.dev/doc/godebug#go-127)
//
// That is a louder guard than this test and it fires at build time, so a check
// here would be machinery for a state the toolchain makes unreachable.
//
// The required shape is an explanation: a file may name a removed setting, and
// then it has to say that it was removed, so the next reader learns the mechanism
// is gone rather than reaching for it.
func TestNoShippedGuidanceNamesARemovedGODEBUG(t *testing.T) {
	removed := removedGODEBUGSettings(t)
	if len(removed) == 0 {
		t.Fatal("the toolchain's godebug table declares no removed setting, so this test would pass on any tree")
	}

	walkRepoText(t, repoRoot(t), func(rel string, content []byte) {
		var named []string
		for _, setting := range removed {
			if !bytes.Contains(content, []byte(setting)) {
				continue
			}
			named = append(named, setting)
			assertDoesNotSetGODEBUG(t, rel, content, setting)
		}
		if len(named) == 0 {
			return
		}
		if bytes.Contains(bytes.ToLower(content), []byte("removed")) {
			return
		}
		t.Errorf("%s names the removed GODEBUG setting(s) %v and never says they were removed", rel, named)
	})
}

// godebugAssignment captures the value list of a GODEBUG assignment, wherever the
// assignment sits on the line. That covers every form the repository can ship one
// in: a bare `GODEBUG=` line in an env file, `export GODEBUG=` in a shell script,
// `ENV GODEBUG=` in a Dockerfile, `env GODEBUG=... ./ze` before a command, and
// `-e GODEBUG=...` in `docker run` argument lists built by the native interop labs.
//
// The value list runs to the first whitespace, quote, or backtick, because GODEBUG
// is comma-separated and each item is its own setting.
var godebugAssignment = regexp.MustCompile("GODEBUG=([^\\s\"'`]*)")

// A note on carve-outs, because two were tried here and both passed the defect.
//
// The first required the assignment to START the line, after whitespace, a quote
// and an `export` keyword. The second skipped comment lines and Markdown code
// spans, on the true reasoning that a commented assignment sets nothing. The
// defect this test exists for is a comment containing a code span, mid-sentence
// (testdata/godebug-guidance-defect.txt), so both let it through.
//
// The lesson is about WHICH harm is being guarded. A carve-out asks "does this
// line set the variable in this process". The harm is "can a reader copy this
// string and stop their daemon", and a comment is the likeliest place a reader
// finds an instruction. So there is no carve-out: the assignable form is refused
// wherever it appears, and a file that needs to discuss the setting names it
// (`tlsunsafeekm`) without writing the form that can be pasted.

// godebugFinding is one line that assigns a removed setting.
type godebugFinding struct {
	number int
	text   string
}

// assertDoesNotSetGODEBUG reports every finding in content.
func assertDoesNotSetGODEBUG(t *testing.T, rel string, content []byte, setting string) {
	t.Helper()
	for _, found := range findGODEBUGAssignments(content, setting) {
		t.Errorf("%s line %d sets %q, which Go has removed. Its old value is a fatal error raised before main(): %s",
			rel, found.number, setting, found.text)
	}
}

// findGODEBUGAssignments answers every line of content that names setting inside
// a GODEBUG assignment, with no exemption for where the line sits or what kind of
// line it is. The whole test rests on this predicate, so it is split out and
// driven directly by the defect that motivated it
// (TestGODEBUGGuardRedsOnTheDefectItWasWrittenFor).
func findGODEBUGAssignments(content []byte, setting string) []godebugFinding {
	var found []godebugFinding
	for number, line := range strings.Split(string(content), "\n") {
		for _, at := range godebugAssignment.FindAllStringSubmatchIndex(line, -1) {
			if !godebugListNames(line[at[2]:at[3]], setting) {
				continue
			}
			found = append(found, godebugFinding{number: number + 1, text: strings.TrimSpace(line)})
		}
	}
	return found
}

// godebugListNames answers whether a GODEBUG value list assigns setting. The list
// is comma-separated and each item is `<name>=<value>`, so the name is compared
// rather than the whole item: every value of a removed setting is wrong, not only
// the old one.
func godebugListNames(list, setting string) bool {
	for item := range strings.SplitSeq(list, ",") {
		name, _, assigned := strings.Cut(item, "=")
		if assigned && name == setting {
			return true
		}
	}
	return false
}

// removedGODEBUGSettings answers every GODEBUG setting the toolchain in use has
// removed, read from that toolchain rather than from a list kept here. A list
// would go stale on the next Go release, which is the failure this test exists to
// catch.
func removedGODEBUGSettings(t *testing.T) []string {
	t.Helper()
	table := filepath.Join(goroot(t), "src", "internal", "godebugs", "table.go")
	content, err := os.ReadFile(table)
	if err != nil {
		t.Fatalf("read the toolchain godebug table: %v", err)
	}

	var settings []string
	for _, match := range godebugRemovedRow.FindAllStringSubmatch(string(content), -1) {
		settings = append(settings, match[1])
	}
	return settings
}

// goroot answers the GOROOT of the toolchain running this test. `go test` puts it
// in the environment; go/build holds the compiled-in value when it is absent.
func goroot(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("GOROOT"); root != "" {
		return root
	}
	if root := build.Default.GOROOT; root != "" {
		return root
	}
	t.Fatal("no GOROOT: this test reads the removed settings from the toolchain's own godebug table")
	return ""
}

// walkRepoText calls visit for every tracked text file under root, outside the
// vendor tree, with the path relative to root.
//
// The population is what git tracks or would accept, because that is what ze
// publishes. A walk of
// the directory tree would read a downloaded module cache instead: 163 files of
// third-party Go under gokrazy/modcache carry these settings, one of them a live
// //go:debug directive, and none of them is anything ze tells anybody to do.
//
// A file holding a NUL octet in its first 512 is binary and is not read further.
func walkRepoText(t *testing.T, root string, visit func(rel string, content []byte)) {
	t.Helper()
	// --cached --others --exclude-standard: tracked files AND untracked ones git
	// would accept. Tracked alone is the wrong population for a guard that runs
	// before a commit, because a defect introduced in a NEW file is invisible
	// until the commit that introduces it has already happened. This file was
	// itself untracked and unjudged when it first went green.
	list := exec.CommandContext(t.Context(), "git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	list.Dir = root
	out, err := list.Output()
	if err != nil {
		t.Fatalf("git ls-files in %s: %v", root, err)
	}

	seen := 0
	for rel := range strings.SplitSeq(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if rel == "" {
			continue
		}
		if pathIsSubtracted(rel) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, rel))
		if os.IsNotExist(err) {
			// Tracked in the index, deleted in the working tree. It publishes
			// nothing until somebody restores it.
			continue
		}
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		head := content
		if len(head) > 512 {
			head = head[:512]
		}
		if bytes.IndexByte(head, 0) >= 0 {
			continue
		}
		seen++
		visit(rel, content)
	}

	if seen == 0 {
		t.Fatalf("git tracks no readable text file under %s, so this scan judged nothing", root)
	}
}

// repoRoot answers the directory holding go.mod, walking up from the test's
// working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod in any parent of the test's working directory")
		}
		dir = parent
	}
}

// defectFixture is cmd/ze/main.go as it stood before this spec changed it: the
// shipped instruction that stops the daemon.
const defectFixture = "testdata/godebug-guidance-defect.txt"

// TestGODEBUGGuardRedsOnTheDefectItWasWrittenFor is the only proof this guard
// really owes.
//
// A guard built against a historical defect is worth nothing until it goes RED on
// that defect's own text. Two earlier versions of findGODEBUGAssignments went
// GREEN here and both looked defensible line by line. The first required the
// assignment to START the line, and the defect's sits mid-sentence. The second
// skipped comment lines and Markdown code spans, and the defect is a comment with
// a code span. Each carve-out was true about the shape it named and wrong about
// this file.
//
// VALIDATES: the predicate rejects the exact text that shipped.
// PREVENTS: another carve-out that reads well and passes the defect.
func TestGODEBUGGuardRedsOnTheDefectItWasWrittenFor(t *testing.T) {
	content := readFixture(t)

	found := findGODEBUGAssignments(content, "tlsunsafeekm")
	if len(found) == 0 {
		t.Fatalf("the guard passes the defect it exists to catch (%s). "+
			"Every carve-out in findGODEBUGAssignments has to answer this file.", defectFixture)
	}
	for _, f := range found {
		t.Logf("caught line %d: %s", f.number, f.text)
	}
}

// TestGODEBUGRemovedBackstopIsNotTheGuard records why the file-wide "removed"
// check cannot be relied on, so nobody restores it as the guard.
//
// The defect file already contained the words "it was written and then removed",
// about a DIFFERENT thing: the //go:debug directive deleted on 2026-08-01. The
// backstop was satisfied forty lines from the instruction it was meant to cover.
// It stays, because AC-4 asks that a surviving mention explain the removal, but
// it is a hint and findGODEBUGAssignments is the guard.
func TestGODEBUGRemovedBackstopIsNotTheGuard(t *testing.T) {
	content := bytes.ToLower(readFixture(t))
	if !bytes.Contains(content, []byte("removed")) {
		t.Fatal("the defect fixture no longer says \"removed\", so it no longer records why a file-wide check is not a guard")
	}
}

// readFixture answers the defect fixture's bytes.
func readFixture(t *testing.T) []byte {
	t.Helper()
	content, err := os.ReadFile(defectFixture)
	if err != nil {
		t.Fatalf("read %s: %v", defectFixture, err)
	}
	return content
}

// TestGODEBUGPredicateCoversEveryAssignableForm gives the predicate the fixture
// it lacked.
//
// The repository contains none of these shapes, which is the point: without a
// fixture the only evidence for the predicate's reach was a mutation restoring
// the one shape the tree had already carried. Every claim the predicate makes is
// a row in these two files, and a row that stops firing is a coverage loss a
// reader can see.
//
// The corpus is DATA under testdata, not Go literals, and that is not a style
// choice. Written as literals the rows fired against this very file, because the
// scan judges it and the rows are the forbidden form. The other way out would be
// to compose the strings at runtime, which is precisely the evasion the predicate
// documents that it cannot see: writing the test that way would shape the code to
// dodge its own guard.
//
// VALIDATES: the assignable form is found wherever it sits and whatever leads it,
// and a mention that is NOT assignable is left alone.
// PREVENTS: a narrowing of findGODEBUGAssignments that still passes the tree,
// because the tree ships none of the forms it would drop.
func TestGODEBUGPredicateCoversEveryAssignableForm(t *testing.T) {
	const setting = "tlsunsafeekm"

	for shape, line := range readShapes(t, "testdata/godebug-assignable-forms.tsv") {
		if len(findGODEBUGAssignments([]byte(line), setting)) == 0 {
			t.Errorf("%s: not found, so this form can ship: %s", shape, line)
		}
	}
	for shape, line := range readShapes(t, "testdata/godebug-inert-forms.tsv") {
		if got := findGODEBUGAssignments([]byte(line), setting); len(got) != 0 {
			t.Errorf("%s: reported %d finding(s), so the predicate is too wide: %s", shape, len(got), line)
		}
	}
}

// readShapes answers the `<shape>\t<line>` rows of a corpus file. It fails when
// the file is empty or a row has no tab, so a corpus that stopped being read
// cannot be mistaken for a corpus that passed.
func readShapes(t *testing.T, path string) map[string]string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	shapes := make(map[string]string)
	for number, row := range strings.Split(strings.TrimRight(string(content), "\n"), "\n") {
		shape, line, ok := strings.Cut(row, "\t")
		if !ok {
			t.Fatalf("%s row %d has no tab: %q", path, number+1, row)
		}
		shapes[shape] = line
	}
	if len(shapes) == 0 {
		t.Fatalf("%s holds no row, so this test judged nothing", path)
	}
	return shapes
}

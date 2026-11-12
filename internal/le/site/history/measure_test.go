package sitehistory

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// historyCommit is one commit of a fixture history: the instant it was
// authored and committed, and its whole tree.
type historyCommit struct {
	// when is an RFC 3339 instant, the author and committer time alike.
	when string
	// files maps each regular file's path to its content.
	files map[string]string
	// links maps each symbolic link's path to its target.
	links map[string]string
	// submodules names paths whose submodule entry points at the previous
	// commit of the same import, so it MUST be empty on the first one.
	submodules []string
}

// newFixtureRoot answers an empty repository whose HEAD names refs/heads/main,
// its path resolved through symbolic links so git and the tool name one root.
func newFixtureRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, root, "", nil, "init", "-q", "--initial-branch=main")
	return root
}

// buildHistory writes commits onto refs/heads/main through one git
// fast-import, the cheapest route to a history with exact trees and dates:
// one git process for the whole history. The first commit's parent is parent,
// or none when parent is empty, which replaces the branch's history. It
// answers the commit IDs in order.
func buildHistory(t *testing.T, root, parent string, commits []historyCommit) []string {
	t.Helper()
	var stream strings.Builder
	if parent == "" {
		stream.WriteString("reset refs/heads/main\n")
	}
	for index, commit := range commits {
		instant, err := time.Parse(time.RFC3339, commit.when)
		if err != nil {
			t.Fatalf("fixture commit %d: %v", index+1, err)
		}
		identity := "Fixture <fixture@example.invalid> " + strconv.FormatInt(instant.Unix(), 10) + " +0000\n"
		stream.WriteString("commit refs/heads/main\nmark :" + strconv.Itoa(index+1) + "\n")
		stream.WriteString("author " + identity + "committer " + identity)
		writeFixtureData(&stream, "commit "+strconv.Itoa(index+1)+"\n")
		if index > 0 {
			stream.WriteString("from :" + strconv.Itoa(index) + "\n")
		} else if parent != "" {
			stream.WriteString("from " + parent + "\n")
		}
		stream.WriteString("deleteall\n")
		for _, path := range slices.Sorted(maps.Keys(commit.files)) {
			stream.WriteString("M 100644 inline " + path + "\n")
			writeFixtureData(&stream, commit.files[path])
		}
		for _, path := range slices.Sorted(maps.Keys(commit.links)) {
			stream.WriteString("M 120000 inline " + path + "\n")
			writeFixtureData(&stream, commit.links[path])
		}
		for _, path := range commit.submodules {
			stream.WriteString("M 160000 :" + strconv.Itoa(index) + " " + path + "\n")
		}
	}
	for index := range commits {
		stream.WriteString("get-mark :" + strconv.Itoa(index+1) + "\n")
	}
	ids := strings.Fields(fixtureGit(t, root, stream.String(), nil, "fast-import", "--quiet", "--force"))
	if len(ids) != len(commits) {
		t.Fatalf("git fast-import answered %d commit IDs for %d commits: %v", len(ids), len(commits), ids)
	}
	return ids
}

// writeFixtureData writes one fast-import data block of content.
func writeFixtureData(stream *strings.Builder, content string) {
	stream.WriteString("data " + strconv.Itoa(len(content)) + "\n" + content + "\n")
}

// historyDays answers the recorded days of HEAD's first-parent history.
func historyDays(t *testing.T, root string) []Day {
	t.Helper()
	line, err := resolveHistory(t.Context(), root, repairExtend)
	if err != nil {
		t.Fatalf("cannot resolve the fixture history: %v", err)
	}
	return line.days
}

// assertDays compares every field of every recorded day, each section of the
// table looked up so a section a record leaves out counts as zero.
func assertDays(t *testing.T, got, want []DayRecord) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%d days recorded, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index].Date.Compare(want[index].Date) != 0 {
			t.Errorf("day %d is %s, want %s", index+1, got[index].Date, want[index].Date)
		}
		if got[index].Commit != want[index].Commit {
			t.Errorf("day %s records commit %s, want %s", want[index].Date, got[index].Commit, want[index].Commit)
		}
		if got[index].Modules != want[index].Modules {
			t.Errorf("day %s records %d modules, want %d", want[index].Date, got[index].Modules, want[index].Modules)
		}
		for _, section := range Sections() {
			if got[index].Sections[section.ID] != want[index].Sections[section.ID] {
				t.Errorf("day %s section %s holds %+v, want %+v", want[index].Date, section.ID,
					got[index].Sections[section.ID], want[index].Sections[section.ID])
			}
		}
	}
}

// The vendor/modules.txt of the AC-1 fixture: two modules, then three. A line
// "## explicit" annotates the module above it and is no module.
const (
	modulesTwo   = "# example.com/a v1.0.0\n## explicit\nexample.com/a\n# example.com/b v1.2.0\n"
	modulesThree = modulesTwo + "example.com/b\n# example.com/c v0.1.0\n"
)

// TestMeasureCountsCommittedTreeBySection asks whether each recorded day
// states the files, lines, gains and losses per section that its tree holds
// (AC-1). Method: three days, counted by hand. Day one adds main.go (3 lines:
// 2 code, 1 blank), docs/guide.md (2), plan/old.md (3) and vendor/modules.txt
// (4 lines, 2 modules). Day two rewrites main.go at the same size, deletes
// docs/guide.md, moves plan/old.md to docs/old.md and adds test/run.ci (2).
// Day three shrinks docs/old.md to 1 line and grows vendor/modules.txt to 6
// lines and 3 modules.
func TestMeasureCountsCommittedTreeBySection(t *testing.T) {
	root := newFixtureRoot(t)
	commits := buildHistory(t, root, "", []historyCommit{
		{when: "2026-02-02T10:00:00Z", files: map[string]string{
			"main.go": "package main\n\nfunc main() {}\n", "docs/guide.md": "a\nb\n",
			"plan/old.md": "x\ny\nz\n", "vendor/modules.txt": modulesTwo,
		}},
		{when: "2026-02-03T10:00:00Z", files: map[string]string{
			"main.go": "package main\n\nfunc run() {}\n", "docs/old.md": "x\ny\nz\n",
			"test/run.ci": "one\ntwo\n", "vendor/modules.txt": modulesTwo,
		}},
		{when: "2026-02-04T10:00:00Z", files: map[string]string{
			"main.go": "package main\n\nfunc run() {}\n", "docs/old.md": "x\n",
			"test/run.ci": "one\ntwo\n", "vendor/modules.txt": modulesThree,
		}},
	})
	got, err := measureDays(t.Context(), root, repairExtend, nil, historyDays(t, root))
	if err != nil {
		t.Fatal(err)
	}
	mainGo := SectionValues{Files: 1, Lines: 3, Go: GoLines{Code: 2, Blank: 1}}
	assertDays(t, got, []DayRecord{
		{Date: mustDate(t, "2026-02-02"), Commit: commits[0], Modules: 2, Sections: map[string]SectionValues{
			"go-production":     {Files: 1, Lines: 3, Gained: 3, Go: GoLines{Code: 2, Blank: 1}},
			"documentation":     {Files: 1, Lines: 2, Gained: 2},
			"plans":             {Files: 1, Lines: 3, Gained: 3},
			"third-party-other": {Files: 1, Lines: 4, Gained: 4},
		}},
		{Date: mustDate(t, "2026-02-03"), Commit: commits[1], Modules: 2, Sections: map[string]SectionValues{
			"go-production":     mainGo,
			"documentation":     {Files: 1, Lines: 3, Gained: 3, Lost: 2},
			"plans":             {Lost: 3},
			"functional-tests":  {Files: 1, Lines: 2, Gained: 2},
			"third-party-other": {Files: 1, Lines: 4},
		}},
		{Date: mustDate(t, "2026-02-04"), Commit: commits[2], Modules: 3, Sections: map[string]SectionValues{
			"go-production":     mainGo,
			"documentation":     {Files: 1, Lines: 1, Lost: 2},
			"functional-tests":  {Files: 1, Lines: 2},
			"third-party-other": {Files: 1, Lines: 6, Gained: 2},
		}},
	})
}

// nulAt answers a text of "abc\n" lines with one NUL byte at offset, the
// probe's boundary: 1,999 lines of "abc\n" fill offsets 0 to 7,995, "abc"
// fills 7,996 to 7,998, and the byte at 7,999 is "\n" unless offset names it.
func nulAt(offset int) string {
	text := []byte(strings.Repeat("abc\n", 1999) + "abc\n" + "z\n")
	return string(append(text[:offset:offset], append([]byte{0}, text[offset:]...)...))
}

// TestMeasureCountsLineBoundaries asks what the line rule counts at each
// boundary (AC-3). Method: a table over countBlob with every count written by
// hand, then one committed tree holding the same blobs, whose section sums are
// those counts added by hand.
func TestMeasureCountsLineBoundaries(t *testing.T) {
	goSource := "package main\n\n// Run runs.\nfunc Run() {}\n/* a\n b */\n"
	cases := []struct {
		name    string
		blob    string
		goSplit bool
		want    blobCount
	}{
		{"empty", "", false, blobCount{}},
		{"one unterminated line", "one", false, blobCount{lines: 1}},
		{"an unterminated last line", "one\ntwo", false, blobCount{lines: 2}},
		{"a trailing newline adds none", "one\ntwo\n", false, blobCount{lines: 2}},
		{"CRLF endings", "a\r\nb\r\n", false, blobCount{lines: 2}},
		// Offset 7,999 is inside the probe: binary, zero lines.
		{"a NUL at offset 7,999 is binary", nulAt(7999), false, blobCount{}},
		// Offset 8,000 is outside it: 2,000 lines of "abc\n", then the NUL
		// opens line 2,001, "\x00z\n".
		{"a NUL only at offset 8,000 is text", nulAt(8000), false, blobCount{lines: 2001}},
		{"a Go blob splits by hand count", goSource, true, blobCount{lines: 6, goCounted: true, goLines: GoLines{Code: 2, Blank: 1, Comment: 3}}},
		{"a Go blob with a NUL counts zero in all four", "package main\x00\n", true, blobCount{goCounted: true}},
	}
	for _, tc := range cases {
		got, err := countBlob([]byte(tc.blob), tc.goSplit)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: counts %+v, want %+v", tc.name, got, tc.want)
		}
	}

	root := newFixtureRoot(t)
	commits := buildHistory(t, root, "", []historyCommit{{when: "2026-02-09T10:00:00Z", files: map[string]string{
		"notes/empty.txt": "", "notes/one.txt": "one", "notes/crlf.txt": "a\r\nb\r\n",
		"notes/binary.dat": nulAt(7999), "notes/text.dat": nulAt(8000),
		"cmd/run.go": goSource, "cmd/nul.go": "package main\x00\n",
	}}})
	got, err := measureDays(t.Context(), root, repairExtend, nil, historyDays(t, root))
	if err != nil {
		t.Fatal(err)
	}
	// Other: 0 + 1 + 2 + 0 + 2,001 lines over five files. Go production: the
	// split Go blob's 6 lines and the binary one's 0, over two files.
	assertDays(t, got, []DayRecord{{Date: mustDate(t, "2026-02-09"), Commit: commits[0], Sections: map[string]SectionValues{
		"other":         {Files: 5, Lines: 2004, Gained: 2004},
		"go-production": {Files: 2, Lines: 6, Gained: 6, Go: GoLines{Code: 2, Blank: 1, Comment: 3}},
	}}})
}

// TestMeasurePlacesEveryPathInItsSection asks whether each regular file of a
// committed tree lands in the section its rule names, and whether links and
// submodules stay out (AC-2). Method: day two holds one file for each rule, a
// first-party package in a directory named vendor, a symbolic link and a
// submodule entry; every count is written by hand and the sections' lines sum
// to the tree's 27.
func TestMeasurePlacesEveryPathInItsSection(t *testing.T) {
	root := newFixtureRoot(t)
	commits := buildHistory(t, root, "", []historyCommit{
		{when: "2026-02-16T10:00:00Z", files: map[string]string{"README.md": "r\n"}},
		{when: "2026-02-17T10:00:00Z", files: map[string]string{
			"vendor/a/a.go":                    "package a\n",
			"third_party/x/LICENSE":            "l1\nl2\n",
			"website/assets/vendor/lib.js":     "x\n",
			"internal/le/web/vendor/v.go":      "package vendor\n\nvar V = 1\n",
			"internal/le/web/vendor/v_test.go": "package vendor\n",
			"test/plugin/x.ci":                 "a\nb\nc\n",
			"rfc/full/rfc1.txt":                "r\n",
			"reference/iso.txt":                "i\ni\n",
			"plan/spec.md":                     "p\n",
			"ai/rules/x.md":                    "a\n",
			".claude/settings.json":            "{}\n",
			"AGENTS.md":                        "g\n",
			"docs/x.md":                        "d\n",
			"features/f.md":                    "f\nf\n",
			"README.md":                        "r\n",
			"website/index.html":               "<p>\n",
			"demos/d.txt":                      "d\n",
			"internal/x/schema.yang":           "module x;\n",
			"Makefile":                         "all:\n\techo\n",
		}, links: map[string]string{"link.md": "README.md"}, submodules: []string{"sub"}},
	})
	got, err := measureDays(t.Context(), root, repairExtend, nil, historyDays(t, root))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]SectionValues{
		"third-party-go":    {Files: 1, Lines: 1, Gained: 1, Go: GoLines{Code: 1}},
		"third-party-other": {Files: 2, Lines: 3, Gained: 3},
		"go-tests":          {Files: 1, Lines: 1, Gained: 1, Go: GoLines{Code: 1}},
		"go-production":     {Files: 1, Lines: 3, Gained: 3, Go: GoLines{Code: 2, Blank: 1}},
		"functional-tests":  {Files: 1, Lines: 3, Gained: 3},
		"rfc-corpus":        {Files: 2, Lines: 3, Gained: 3},
		"plans":             {Files: 1, Lines: 1, Gained: 1},
		"agent-rules":       {Files: 3, Lines: 3, Gained: 3},
		"documentation":     {Files: 3, Lines: 4, Gained: 3},
		"website":           {Files: 2, Lines: 2, Gained: 2},
		"schemas":           {Files: 1, Lines: 1, Gained: 1},
		"other":             {Files: 1, Lines: 2, Gained: 2},
	}
	assertDays(t, got[1:], []DayRecord{{Date: mustDate(t, "2026-02-17"), Commit: commits[1], Sections: want}})
	var total uint64
	for _, values := range got[1].Sections {
		total += values.Lines
	}
	if total != 27 {
		t.Errorf("the sections hold %d lines, but the tree's regular files hold 27", total)
	}
}

// TestMeasureReadsOnlyCommittedObjects asks whether a named commit measures
// the same while the working tree is dirty, the index differs, untracked files
// exist and HEAD has moved on (AC-7). Method: measure the one day, check it by
// hand, change all four, measure the same Day again and compare.
func TestMeasureReadsOnlyCommittedObjects(t *testing.T) {
	root := newFixtureRoot(t)
	commits := buildHistory(t, root, "", []historyCommit{{when: "2026-05-04T10:00:00Z", files: map[string]string{
		"main.go": "package main\n", "docs/a.md": "a\nb\n",
	}}})
	days := historyDays(t, root)
	clean, err := measureDays(t.Context(), root, repairExtend, nil, days)
	if err != nil {
		t.Fatal(err)
	}
	assertDays(t, clean, []DayRecord{{Date: mustDate(t, "2026-05-04"), Commit: commits[0], Sections: map[string]SectionValues{
		"go-production": {Files: 1, Lines: 1, Gained: 1, Go: GoLines{Code: 1}},
		"documentation": {Files: 1, Lines: 2, Gained: 2},
	}}})

	for path, content := range map[string]string{
		"main.go": "package main\n\nfunc main() {}\n", "docs/a.md": "changed\n", "untracked.txt": "u\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fixtureGit(t, root, "", nil, "add", "main.go")
	buildHistory(t, root, commits[0], []historyCommit{{when: "2026-05-05T10:00:00Z", files: map[string]string{"main.go": "package moved\n// on\n"}}})

	dirty, err := measureDays(t.Context(), root, repairExtend, nil, days)
	if err != nil {
		t.Fatal(err)
	}
	assertDays(t, dirty, clean)
}

// TestMeasureRefusesUnreadableHistory asks what a tree naming a blob the
// repository lacks answers (R-4). Method: a commit whose tree git built with
// --missing; the whole-history update refuses, names the blob, and leaves the
// file that was there byte for byte.
func TestMeasureRefusesUnreadableHistory(t *testing.T) {
	root := newFixtureRoot(t)
	missing := "0123456789abcdef0123456789abcdef01234567"
	tree := fixtureGit(t, root, "100644 blob "+missing+"\tlost.txt\n", nil, "mktree", "--missing")
	date := []string{"GIT_AUTHOR_DATE=Wed, 01 Apr 2026 10:00:00 +0000", "GIT_COMMITTER_DATE=Wed, 01 Apr 2026 10:00:00 +0000"}
	commit := fixtureGit(t, root, "lost\n", date, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit-tree", tree)
	fixtureGit(t, root, "", nil, "update-ref", "refs/heads/main", commit)

	path := filepath.Join(root, filepath.FromSlash(HistoryFile))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	before := []byte("the file before the run\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := updateHistory(t.Context(), root, wholeRequest{})
	if err == nil {
		t.Fatal("the update measured a tree whose blob is missing")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("the refusal does not name the missing blob %s:\n%v", missing, err)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("the refused update rewrote %s:\n%s", HistoryFile, after)
	}
}

package sitehistory

import (
	"bytes"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The two commits the fixture history records, one per day.
const (
	commitDayOne = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	commitDayTwo = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// fixtureFile is the exact text WriteHistory owes for fixtureHistory, written
// by hand: a fixed key order, one section and one day per line, a trailing
// newline. On 2026-01-06 Go production went from 3 lines to 5 by gaining 3 and
// losing 1, and Documentation from 1 to 2 by gaining 1.
const fixtureFile = `{
  "schema": 1,
  "rules": "rules-fixture",
  "sections": [
    {"id":"go-production","label":"Go production","go-split":true},
    {"id":"documentation","label":"Documentation","go-split":false}
  ],
  "days": [
    {"date":"2026-01-05","commit":"` + commitDayOne + `","modules":0,"sections":{"documentation":{"files":1,"lines":1,"gained":1,"lost":0},"go-production":{"files":1,"lines":3,"gained":3,"lost":0,"go":{"code":2,"blank":1,"comment":0}}}},
    {"date":"2026-01-06","commit":"` + commitDayTwo + `","modules":2,"sections":{"documentation":{"files":1,"lines":2,"gained":1,"lost":0},"go-production":{"files":2,"lines":5,"gained":3,"lost":1,"go":{"code":3,"blank":1,"comment":1}}}}
  ]
}
`

// mustDate reads a fixture day.
func mustDate(t *testing.T, raw string) Date {
	t.Helper()
	day, err := ParseDate(raw)
	if err != nil {
		t.Fatalf("fixture day %q: %v", raw, err)
	}
	return day
}

// fixtureSections is the fixture's section list.
func fixtureSections() []Section {
	return []Section{
		{ID: "go-production", Label: "Go production", GoSplit: true},
		{ID: "documentation", Label: "Documentation"},
	}
}

// fixtureDays is the fixture's two days, the values fixtureFile states.
func fixtureDays(t *testing.T) []DayRecord {
	t.Helper()
	return []DayRecord{
		{Date: mustDate(t, "2026-01-05"), Commit: commitDayOne, Modules: 0, Sections: map[string]SectionValues{
			"go-production": {Files: 1, Lines: 3, Gained: 3, Lost: 0, Go: GoLines{Code: 2, Blank: 1, Comment: 0}},
			"documentation": {Files: 1, Lines: 1, Gained: 1, Lost: 0},
		}},
		{Date: mustDate(t, "2026-01-06"), Commit: commitDayTwo, Modules: 2, Sections: map[string]SectionValues{
			"go-production": {Files: 2, Lines: 5, Gained: 3, Lost: 1, Go: GoLines{Code: 3, Blank: 1, Comment: 1}},
			"documentation": {Files: 1, Lines: 2, Gained: 1, Lost: 0},
		}},
	}
}

// fixtureHistory is the validated fixture history.
func fixtureHistory(t *testing.T) History {
	t.Helper()
	history, err := NewHistory("rules-fixture", fixtureSections(), fixtureDays(t))
	if err != nil {
		t.Fatalf("NewHistory refused the fixture: %v", err)
	}
	return history
}

// writeFixtureFile puts text where ReadHistory looks under root.
func writeFixtureFile(t *testing.T, root, text string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(HistoryFile))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// sameDays reports the first difference between two day lists, or "".
func sameDays(got, want []DayRecord) string {
	if len(got) != len(want) {
		return "the day counts differ"
	}
	for index := range want {
		if got[index].Date.Compare(want[index].Date) != 0 {
			return "day " + want[index].Date.String() + " reads back as " + got[index].Date.String()
		}
		if got[index].Commit != want[index].Commit {
			return "day " + want[index].Date.String() + " reads back another commit"
		}
		if got[index].Modules != want[index].Modules {
			return "day " + want[index].Date.String() + " reads back another module count"
		}
		if !maps.Equal(got[index].Sections, want[index].Sections) {
			return "day " + want[index].Date.String() + " reads back other section values"
		}
	}
	return ""
}

// TestHistoryReadsBackWhatItWrote asks whether the reader answers what the
// writer was given. Method: write the fixture, read it, compare every field
// with the hand-written fixture values.
func TestHistoryReadsBackWhatItWrote(t *testing.T) {
	root := t.TempDir()
	if _, err := WriteHistory(root, fixtureHistory(t)); err != nil {
		t.Fatalf("WriteHistory: %v", err)
	}
	history, err := ReadHistory(root)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}
	if history.Rules() != "rules-fixture" {
		t.Errorf("rules read back as %q", history.Rules())
	}
	sectionList := history.Sections()
	want := fixtureSections()
	if len(sectionList) != len(want) || sectionList[0] != want[0] || sectionList[1] != want[1] {
		t.Errorf("sections read back as %+v, want %+v", sectionList, want)
	}
	if difference := sameDays(history.Days(), fixtureDays(t)); difference != "" {
		t.Errorf("%s", difference)
	}
}

// TestHistoryWritesTheSameBytesTwice asks whether one history always writes
// one file. Method: write the fixture under two roots and compare the bytes.
func TestHistoryWritesTheSameBytesTwice(t *testing.T) {
	first, err := WriteHistory(t.TempDir(), fixtureHistory(t))
	if err != nil {
		t.Fatal(err)
	}
	second, err := WriteHistory(t.TempDir(), fixtureHistory(t))
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, err := os.ReadFile(first) //nolint:gosec // a test reads the file it wrote
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(second) //nolint:gosec // a test reads the file it wrote
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Errorf("two writes of one history differ:\n%s\n---\n%s", firstBytes, secondBytes)
	}
}

// TestHistoryFileWritesOneDayPerLineInAFixedOrder asks what bytes the writer
// produces. Method: compare the written file with fixtureFile, the text
// written by hand, so key order, one day per line and the trailing newline are
// each asserted on the bytes.
func TestHistoryFileWritesOneDayPerLineInAFixedOrder(t *testing.T) {
	path, err := WriteHistory(t.TempDir(), fixtureHistory(t))
	if err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(path) //nolint:gosec // a test reads the file it wrote
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != fixtureFile {
		t.Errorf("the history file reads:\n%s\nwant:\n%s", written, fixtureFile)
	}
}

// TestHistoryFileIsReadableByAWebServer asks whether the written file carries
// the mode of published web content. Method: write the fixture over a file
// readable by its owner alone, as the temporary file used to leave it, and
// read the permission bits of the file now at that path.
func TestHistoryFileIsReadableByAWebServer(t *testing.T) {
	root := t.TempDir()
	path := writeFixtureFile(t, root, fixtureFile)
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteHistory(root, fixtureHistory(t)); err != nil {
		t.Fatalf("WriteHistory: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("the history file has mode %#o, want 0644: a web server reading as another account cannot serve it", got)
	}
}

// TestReadRefusesMalformedFile asks what the reader refuses. Method: one edit
// of fixtureFile per broken rule of the contract, each refused with the
// message that names it, never read as the missing file, and with the repair
// command on the last line.
func TestReadRefusesMalformedFile(t *testing.T) {
	for _, row := range []struct {
		name string
		from string
		into string
		want string
	}{
		{"an unknown field", `"schema": 1,`, `"schema": 1,` + "\n  " + `"extra": 1,`, `unknown field "extra"`},
		{"an unknown nested field", `"lost":0,"go":{"code":2`, `"lost":0,"bytes":9,"go":{"code":2`, `unknown field "bytes"`},
		{"days out of order", `"date":"2026-01-06"`, `"date":"2026-01-04"`, "day 2026-01-04 follows day 2026-01-05"},
		{"a repeated day", `"date":"2026-01-06"`, `"date":"2026-01-05"`, "day 2026-01-05 appears twice"},
		{"an unknown section", `"documentation":{"files":1,"lines":2,`, `"plans":{"files":1,"lines":2,`, "names section plans"},
		{
			"broken line arithmetic",
			`"files":2,"lines":5,"gained":3,"lost":1,"go":{"code":3,"blank":1,"comment":1}`,
			`"files":2,"lines":6,"gained":3,"lost":1,"go":{"code":4,"blank":1,"comment":1}`,
			"day 2026-01-06 section go-production holds 6 lines, but its 3 lines on 2026-01-05 plus 3 gained minus 1 lost make 5",
		},
		{
			"a Go split not summing to the lines",
			`"go":{"code":2,"blank":1,"comment":0}`,
			`"go":{"code":1,"blank":1,"comment":0}`,
			"day 2026-01-05 section go-production holds 3 lines, but its 1 code, 1 blank and 0 comment lines make 2",
		},
		{
			"a Go split on a section without one",
			`{"files":1,"lines":1,"gained":1,"lost":0}`,
			`{"files":1,"lines":1,"gained":1,"lost":0,"go":{"code":1,"blank":0,"comment":0}}`,
			"section documentation holds a Go split",
		},
		{"a Go section without its split", `,"go":{"code":2,"blank":1,"comment":0}`, ``, "section go-production carries the Go split and holds none"},
		{"a missing count", `{"files":1,"lines":1,"gained":1,"lost":0}`, `{"files":1,"lines":1,"gained":1}`, "day 2026-01-05 section documentation has no lost"},
		{"another schema", `"schema": 1,`, `"schema": 2,`, "the file is schema 2"},
		{"text after the object", "  ]\n}\n", "  ]\n}\n{}\n", "more than its one JSON object"},
		{"a day not written YYYY-MM-DD", `"2026-01-05"`, `"2026-1-05"`, `"2026-1-05" is not a day written YYYY-MM-DD`},
		{"a commit that is not an object ID", commitDayOne, "xyz", `records "xyz", which is not a full commit ID`},
	} {
		t.Run(row.name, func(t *testing.T) {
			if strings.Count(fixtureFile, row.from) != 1 {
				t.Fatalf("the fixture holds %q %d times; the edit needs exactly one", row.from, strings.Count(fixtureFile, row.from))
			}
			root := t.TempDir()
			writeFixtureFile(t, root, strings.Replace(fixtureFile, row.from, row.into, 1))
			_, err := ReadHistory(root)
			if err == nil {
				t.Fatalf("ReadHistory accepted a file with %s", row.name)
			}
			if !strings.Contains(err.Error(), row.want) {
				t.Errorf("the refusal does not say %q:\n%v", row.want, err)
			}
			if errors.Is(err, ErrNoHistory) {
				t.Errorf("a malformed file is reported as a missing one: %v", err)
			}
			if !strings.HasSuffix(err.Error(), "fix: ./le site history update all") {
				t.Errorf("the refusal does not end on its repair command:\n%v", err)
			}
		})
	}
}

// TestNewHistoryRefusesASectionThatVanishedUncounted asks whether a section
// absent from a day counts as zero there. Method: drop Documentation from the
// fixture's second day with no lines lost; NewHistory, the only route to the
// writer, must refuse and name the day, the section and both numbers.
func TestNewHistoryRefusesASectionThatVanishedUncounted(t *testing.T) {
	days := fixtureDays(t)
	delete(days[1].Sections, "documentation")
	_, err := NewHistory("rules-fixture", fixtureSections(), days)
	if err == nil {
		t.Fatal("NewHistory accepted a section that vanished without losing its line")
	}
	want := "day 2026-01-06 section documentation holds 0 lines, but its 1 lines on 2026-01-05 plus 0 gained minus 0 lost make 1"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("the refusal does not say %q:\n%v", want, err)
	}
}

// TestReadReportsAMissingFileApart asks how the reader answers a checkout
// with no history. Method: read an empty root; the error must carry
// ErrNoHistory, which a malformed file never does, and name the command that
// writes the file.
func TestReadReportsAMissingFileApart(t *testing.T) {
	_, err := ReadHistory(t.TempDir())
	if !errors.Is(err, ErrNoHistory) {
		t.Fatalf("a missing file is not reported as ErrNoHistory: %v", err)
	}
	if !strings.HasSuffix(err.Error(), "fix: ./le site history update all") {
		t.Errorf("the refusal does not end on the command that writes the file:\n%v", err)
	}
}

// TestWriteRefusalLeavesTheOldFileWhole asks what a refused write leaves
// behind. Method: write the fixture, then write a history no constructor
// built; the refusal must leave the old bytes and no temporary file.
func TestWriteRefusalLeavesTheOldFileWhole(t *testing.T) {
	root := t.TempDir()
	path, err := WriteHistory(root, fixtureHistory(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteHistory(root, History{}); err == nil {
		t.Fatal("WriteHistory wrote a history that holds no day")
	}
	kept, err := os.ReadFile(path) //nolint:gosec // a test reads the file it wrote
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != fixtureFile {
		t.Errorf("a refused write changed the file:\n%s", kept)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("the data directory holds %d entries after a refused write, want only the history file", len(entries))
	}
}

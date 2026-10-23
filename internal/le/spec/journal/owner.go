// Design: plan/journal/README.md -- owned rows: every open row has a spec that will fix it
// Related: validate.go -- the per-file shape check the same commit path runs first

package specjournal

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// OwnedRowsHeading is the spec section whose table lists the journal rows the
// spec owns, one line per row: `| Class file | Date | ... |`. It is the single
// declaration of ownership; a journal row carries no owner of its own.
const OwnedRowsHeading = "## Owned journal rows"

// surfaceRunesMax bounds the surface the refusal copies into its suggested
// table line, counted in runes so a cut never splits a character.
const surfaceRunesMax = 60

// UnownedRow is one open journal row a commit adds or changes that no spec's
// owned table covers. Open counts the open rows of its class carrying its date,
// and Owned the table entries that list that class and date.
type UnownedRow struct {
	Path    string
	Line    int
	Class   string
	Date    string
	Surface string
	Open    int
	Owned   int
}

type classDate struct {
	class string
	date  string
}

// Base is the tree a commit lands on, and its zero value is HEAD itself: a
// commit prepared on its own lands directly on HEAD.
//
// A block appended to a prepared script lands after that script's earlier
// blocks, which have not run yet, so HEAD is not what it lands on. Commit B of
// the two-commit spec closure is the case: it removes the spec while commit A,
// still unrun, carries the row's `fixed: ` marker or the entry moved to another
// spec. Judged against bare HEAD, B strips an owner from a row A already
// closed. Files holds the content an earlier block commits for a path and
// Removed the paths an earlier block deletes; a path is in at most one.
type Base struct {
	Files   map[string]string
	Removed map[string]bool
}

// read answers every path's content in the base, reading in one git process
// the paths no earlier block writes. A path the base does not hold is absent
// from the map.
func (base Base) read(root string, paths []string) (map[string]string, error) {
	var committed []string
	for _, path := range paths {
		if _, laid := base.Files[path]; laid {
			continue
		}
		if base.Removed[path] {
			continue
		}
		committed = append(committed, path)
	}
	contents, err := headFiles(root, committed)
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		if content, laid := base.Files[path]; laid {
			contents[path] = content
		}
	}
	return contents, nil
}

// UnownedOpenRows answers the open journal rows a commit leaves without an
// owning spec, judged on the class and date keys the commit can change.
//
// A key is judged when the commit adds or changes an open row of it against
// the base, or when a spec the commit carries or removes lists it, in the base
// or in the working tree: removing a spec, deleting a table entry or renaming a
// spec away from spec-*.md can strip an owner as surely as adding a row can. An
// open row whose five cells the base's same class file holds, marker included,
// adds no key, so a commit that merely carries a legacy row is not judged on
// it. A reopened row, an edited open row and an open row moved into another
// class are changed, and add their key.
//
// A key is owned when the owned journal rows tables list it at least as often
// as its class holds open rows of that date, so each open row needs an entry of
// its own and no surface prose is compared. An under-owned key answers every
// open row it holds, legacy rows included, so the printed fix clears the key.
// The class is read from the working tree when the commit carries it and from
// the base otherwise. The tables come from the specs the commit carries, read
// in the working tree, and from every other spec in the base, so an owner
// neither committed, nor carried by an earlier block, nor part of this commit
// owns nothing, and a removed spec owns nothing.
func UnownedOpenRows(root string, paths, removed []string, base Base) ([]UnownedRow, error) {
	classes, specs := carriedOwnerFiles(paths, removed)
	if len(classes) == 0 && len(specs) == 0 {
		return nil, nil
	}
	prior, err := base.read(root, append(slices.Clone(classes), specs...))
	if err != nil {
		return nil, err
	}
	judged, rows, err := carriedOpenRows(root, classes, prior)
	if err != nil {
		return nil, err
	}
	listed, err := listedKeys(root, specs, removed, prior)
	if err != nil {
		return nil, err
	}
	for key := range listed {
		judged[key] = true
	}
	if err := addUncarriedOpenRows(root, listed, classes, removed, base, rows); err != nil {
		return nil, err
	}
	if len(judged) == 0 {
		return nil, nil
	}
	owned, err := ownedRows(root, paths, removed, base)
	if err != nil {
		return nil, err
	}
	return underOwnedRows(judged, rows, owned), nil
}

// carriedOwnerFiles answers the class files the commit carries and the specs it
// carries or removes: the only files whose change can move an owner.
func carriedOwnerFiles(paths, removed []string) (classes, specs []string) {
	for _, path := range paths {
		if isClassFile(path) && !slices.Contains(removed, path) {
			classes = append(classes, path)
		}
		if isSpecFile(path) {
			specs = append(specs, path)
		}
	}
	for _, path := range removed {
		if isSpecFile(path) && !slices.Contains(specs, path) {
			specs = append(specs, path)
		}
	}
	return classes, specs
}

// carriedOpenRows answers the open rows of the class files the commit carries,
// by key, and the keys whose rows the commit adds or changes against prior.
func carriedOpenRows(root string, classes []string, prior map[string]string) (map[classDate]bool, map[classDate][]UnownedRow, error) {
	judged := map[classDate]bool{}
	rows := map[classDate][]UnownedRow{}
	for _, path := range classes {
		content, err := readTreeFile(root, path)
		if err != nil {
			return nil, nil, err
		}
		held := exactRows(journalRows(prior[path]))
		opens := openRowsOf(path, content)
		for index := range opens {
			open := &opens[index]
			key := classDate{class: open.row.Class, date: open.row.Date}
			rows[key] = append(rows[key], open.row)
			if held[open.cells] > 0 {
				held[open.cells]--
				continue
			}
			judged[key] = true
		}
	}
	return judged, rows, nil
}

// listedKeys counts the keys the specs the commit carries or removes list, on
// either side: their prior content and, unless removed, their working tree.
func listedKeys(root string, specs, removed []string, prior map[string]string) (map[classDate]int, error) {
	listed := map[classDate]int{}
	for _, path := range specs {
		countOwnedRows(prior[path], listed)
		if slices.Contains(removed, path) {
			continue
		}
		content, err := readSpecTable(root, path)
		if err != nil {
			return nil, err
		}
		countOwnedRows(content, listed)
	}
	return listed, nil
}

// addUncarriedOpenRows adds to rows the open rows of every listed key's class
// file the commit does not carry, read from the base.
func addUncarriedOpenRows(root string, listed map[classDate]int, classes, removed []string, base Base, rows map[classDate][]UnownedRow) error {
	var unread []string
	for key := range listed {
		path := classPath(key.class)
		if slices.Contains(classes, path) {
			continue
		}
		// A class file the commit removes holds no open row.
		if slices.Contains(removed, path) {
			continue
		}
		if slices.Contains(unread, path) {
			continue
		}
		unread = append(unread, path)
	}
	committed, err := base.read(root, unread)
	if err != nil {
		return err
	}
	for _, path := range unread {
		opens := openRowsOf(path, committed[path])
		for index := range opens {
			open := &opens[index]
			key := classDate{class: open.row.Class, date: open.row.Date}
			rows[key] = append(rows[key], open.row)
		}
	}
	return nil
}

// underOwnedRows answers every open row of each judged key whose spec tables
// list it fewer times than its class holds open rows of that date, in path and
// line order.
func underOwnedRows(judged map[classDate]bool, rows map[classDate][]UnownedRow, owned map[classDate]int) []UnownedRow {
	var unowned []UnownedRow
	for key := range judged {
		if owned[key] >= len(rows[key]) {
			continue
		}
		for _, row := range rows[key] {
			row.Open, row.Owned = len(rows[key]), owned[key]
			unowned = append(unowned, row)
		}
	}
	slices.SortFunc(unowned, func(a, b UnownedRow) int {
		if order := strings.Compare(a.Path, b.Path); order != 0 {
			return order
		}
		return a.Line - b.Line
	})
	return unowned
}

// openRow is one open row of a class file with the cells it is paired by.
type openRow struct {
	row   UnownedRow
	cells [5]string
}

// openRowsOf answers the well-formed open rows of one class file's content.
func openRowsOf(path, content string) []openRow {
	class := strings.TrimSuffix(strings.TrimPrefix(path, journalPrefix), markdownExt)
	var rows []openRow
	for number, line := range strings.Split(content, "\n") {
		row, candidate := parseJournalRow(line)
		if !candidate {
			continue
		}
		if row.malformed {
			continue
		}
		if fixMarkerOf(row.cells[4]) != fixOpen {
			continue
		}
		rows = append(rows, openRow{cells: row.cells, row: UnownedRow{Path: path, Line: number + 1,
			Class: class, Date: row.cells[0], Surface: row.cells[2]}})
	}
	return rows
}

// UnownedText renders the refusal: what is wrong, why, and the table line that
// clears each row.
func UnownedText(rows []UnownedRow) string {
	var text textbuf.Buffer
	text.Str("journal: ").Int(int64(len(rows))).Str(" open row(s) would be left with no owning spec by this commit\n")
	for _, row := range rows {
		surface := row.Surface
		if runes := []rune(surface); len(runes) > surfaceRunesMax {
			surface = string(runes[:surfaceRunesMax])
		}
		text.Str("  ").Str(row.Path).Str(" line ").Int(int64(row.Line)).Str(": the row dated ").
			Str(row.Date).Str(" is open and no spec owns it (").Int(int64(row.Open)).
			Str(" open row(s) of this class carry that date, spec tables list ").Int(int64(row.Owned)).Str(")\n").
			Str("    add to the spec that will fix it: | `").Str(row.Class).Str(markdownExt).Str("` | ").
			Str(row.Date).Str(" | ").Str(surface).Str(" |\n")
	}
	return text.Str("  Every open journal row is listed in the owned journal rows table of the spec that will fix it, ").
		Str("so no recorded defect is left without a home (plan/journal/README.md, \"Owned rows\").\n").
		Str("  Fix: add each line above under `").Str(OwnedRowsHeading).Str("` in an existing spec whose mechanism fits, ").
		Str("or in a new spec, and name that spec file in this commit; when this commit removes a spec or ").
		Str("one of its entries, move the entry to the spec that will fix the row; or, if the defect is repaired, ").
		Str("start the row's Fix cell with `fixed: `.\n").String()
}

// ownedRows counts the owned-table entries of every spec by class and date:
// the specs in paths from the working tree, every other spec from the base.
func ownedRows(root string, paths, removed []string, base Base) (map[classDate]int, error) {
	owned := map[classDate]int{}
	carried := map[string]bool{}
	for _, path := range paths {
		if !isSpecFile(path) {
			continue
		}
		carried[path] = true
		if slices.Contains(removed, path) {
			continue
		}
		content, err := readSpecTable(root, path)
		if err != nil {
			return nil, err
		}
		countOwnedRows(content, owned)
	}
	for _, path := range removed {
		carried[path] = true
	}
	specs, err := uncarriedOwners(root, carried, base)
	if err != nil {
		return nil, err
	}
	contents, err := base.read(root, specs)
	if err != nil {
		return nil, err
	}
	for _, path := range specs {
		countOwnedRows(contents[path], owned)
	}
	return owned, nil
}

// uncarriedOwners answers the specs the commit does not carry whose base
// content holds an owned journal rows heading: HEAD's, less those an earlier
// block deletes or rewrites, plus every spec an earlier block writes with the
// heading. The answer is sorted, so the base's map order never shows.
func uncarriedOwners(root string, carried map[string]bool, base Base) ([]string, error) {
	committed, err := headOwners(root)
	if err != nil {
		return nil, err
	}
	var specs []string
	for _, path := range committed {
		if carried[path] {
			continue
		}
		if base.Removed[path] {
			continue
		}
		if _, laid := base.Files[path]; laid {
			continue
		}
		specs = append(specs, path)
	}
	for path, content := range base.Files {
		if carried[path] {
			continue
		}
		if !isSpecFile(path) {
			continue
		}
		if !strings.Contains(content, OwnedRowsHeading) {
			continue
		}
		specs = append(specs, path)
	}
	slices.Sort(specs)
	return specs, nil
}

// headOwners answers the specs HEAD holds whose content names the owned journal
// rows heading. A HEAD with no commit yet holds none.
func headOwners(root string) ([]string, error) {
	unborn, err := headUnborn(root)
	if err != nil {
		return nil, err
	}
	if unborn {
		return nil, nil
	}
	answer := runGit(root, "grep", "--name-only", "--fixed-strings", "-e", OwnedRowsHeading, "HEAD", "--", "plan")
	if answer.startErr != nil {
		return nil, answer.startErr
	}
	// git grep exits 1 when nothing matches, which is an answer: no spec owns a row.
	if answer.code == 1 {
		return nil, nil
	}
	if answer.code != 0 {
		return nil, gitReadError("git grep HEAD -- plan failed: ", answer)
	}
	var specs []string
	for line := range strings.SplitSeq(string(answer.stdout), "\n") {
		path := strings.TrimPrefix(strings.TrimSpace(line), "HEAD:")
		if isSpecFile(path) {
			specs = append(specs, path)
		}
	}
	return specs, nil
}

// countOwnedRows adds the entries of the owned journal rows table in one spec.
// The section runs from its heading to the next heading; a line counts when its
// first cell names a class file and its second is the row's date.
func countOwnedRows(content string, owned map[classDate]int) {
	inside := false
	// fence is the marker of the open code fence, or empty outside one. A
	// fence opens with three backticks or three tildes and closes only with
	// the same character, so a ``` line inside a ~~~ fence is code.
	fence := ""
	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSpace(line)
		// A `#` line inside a code fence is code, not a heading, so it never
		// ends the section.
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			continue
		}
		if marker := fenceMarker(trimmed); marker != "" {
			fence = marker
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			inside = trimmed == OwnedRowsHeading
			continue
		}
		if !inside {
			continue
		}
		cells := strings.Split(strings.Trim(trimmed, "|"), "|")
		if len(cells) < 2 {
			continue
		}
		class := strings.Trim(strings.TrimSpace(cells[0]), "`")
		class = strings.TrimSuffix(strings.TrimPrefix(class, journalPrefix), markdownExt)
		date := strings.TrimSpace(cells[1])
		// The header and separator lines carry no date, so the parse skips them.
		if _, err := time.Parse(time.DateOnly, date); err != nil {
			continue
		}
		owned[classDate{class: class, date: date}]++
	}
}

// fenceMarker answers the marker a line opens a code fence with, three
// backticks or three tildes, or empty when the line opens none.
func fenceMarker(trimmed string) string {
	for _, marker := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, marker) {
			return marker
		}
	}
	return ""
}

// exactRows counts rows by their five cells, Fix marker included, so a row
// whose marker flipped is a changed row rather than a held one.
func exactRows(rows []journalRow) map[[5]string]int {
	held := make(map[[5]string]int, len(rows))
	for _, row := range rows {
		if !row.malformed {
			held[row.cells]++
		}
	}
	return held
}

func isClassFile(path string) bool {
	return strings.HasPrefix(path, journalPrefix) && strings.HasSuffix(path, markdownExt) &&
		filepath.Base(path) != journalReadme
}

// isSpecFile answers whether a path is a spec, the only file that can own a row.
func isSpecFile(path string) bool {
	if !strings.HasPrefix(path, "plan/") {
		return false
	}
	if strings.HasPrefix(path, journalPrefix) {
		return false
	}
	return strings.HasPrefix(filepath.Base(path), "spec-") && strings.HasSuffix(path, markdownExt)
}

func readTreeFile(root, path string) (string, error) {
	repository, err := os.OpenRoot(root)
	if err != nil {
		return "", err
	}
	content, readErr := repository.ReadFile(filepath.FromSlash(path))
	closeErr := repository.Close()
	if readErr != nil {
		return "", fmt.Errorf("read %s: %w", path, readErr)
	}
	if closeErr != nil {
		return "", closeErr
	}
	return string(content), nil
}

// readSpecTable reads a spec the commit carries. A spec absent from the working
// tree holds no table, so it owns nothing: the answer can only refuse a commit,
// never admit one.
func readSpecTable(root, path string) (string, error) {
	content, err := readTreeFile(root, path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return content, err
}

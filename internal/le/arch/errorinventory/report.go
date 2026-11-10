// Design: docs/contributing/error-inventory.md -- the inventory of user-visible message sites
//
// Overview: inventory.go -- the row, its surface, status, source and pre-rank
//
// report.go holds what `le arch error-inventory` ANSWERS, apart from what
// produced it: the check's refusal, the tree report with one count line per
// surface, and the change-set judgement of the sites a change adds.

package archerrorinventory

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/commit/changecheck"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// noDoor is the door column of a file no door covers.
const noDoor = "(no door)"

// CheckReport is the answer of one check: the scope it judged, how many sites
// sit on that scope's lines, and the refused ones.
type CheckReport struct {
	// Scope says what was judged, in words.
	Scope    string               `json:"scope"`
	Base     repochanged.LineBase `json:"base"`
	Judged   int                  `json:"judged"`
	Findings []Row                `json:"findings"`
}

// exitCode is 1 on a finding and 0 on none.
func (r CheckReport) exitCode() int {
	if len(r.Findings) > 0 {
		return 1
	}
	return 0
}

// ze point: user-facing-errors/directives/an-error-names-the-cause-and-the-fix
//
// Text renders the scope line, one line per refused site, why, and the fix
// on lines of its own. It ends in a newline.
func (r CheckReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("error-inventory scope: ").Str(r.Scope).Str(", ").Int(int64(r.Judged)).Str(" message site(s) judged\n")
	if len(r.Findings) == 0 {
		return tb.Str("error-inventory: OK\n").String()
	}
	tb.Str("error-inventory: ").Int(int64(len(r.Findings))).Str(" added message site(s) do not name the fix:\n")
	for i := range r.Findings {
		row := &r.Findings[i]
		tb.Str("  ").Str(row.File).Byte(':').Int(int64(row.Line)).Str(": ").Str(row.Surface.String()).
			Byte(' ').Str(row.Status.String()).Byte(' ').Str(row.Writer).
			Byte(' ').Str(strconv.Quote(row.Message)).Byte('\n')
	}
	tb.Byte('\n')
	tb.Str("Why: a message a user reads names what went wrong, why, and the exact command that fixes it (ai/rules/user-facing-errors.md). A plain string carries no fix the code declares, and a command its text names is not proven to fix the problem.\n")
	tb.Str("Fix: hand the writer an operator error that carries the fix (internal/core/opererr), rendered with .Error() where the writer takes a string:\n")
	tb.Str("  opererr.New(problem, why, fix)\n")
	tb.Str("A plugin.Response error answer is built from one with plugin.OperatorErrorResponse.\n")
	tb.Str("The sites already in the tree are listed by ./le arch error-inventory report.\n")
	return tb.String()
}

// refused reports whether the check refuses a row a change adds: only this
// package's own writers, because the operator-error and doctor-coverage
// checks judge their sources' rows, and only a status that does not meet the
// rule.
func refused(row *Row) bool {
	if row.Source != SourceWriter {
		return false
	}
	switch row.Status {
	case StatusPlain, StatusPlainNamesCommand:
		return true
	case StatusTyped, StatusUnspecified:
		return false
	}
	return false
}

// checkLines keeps the rows on changed lines, and refuses the ones refused
// answers true for.
func checkLines(rows []Row, changed repochanged.ChangedLines) CheckReport {
	report := CheckReport{Findings: []Row{}}
	for i := range rows {
		row := &rows[i]
		if !changed.Touches(row.File, row.Line, row.EndLine) {
			continue
		}
		report.Judged++
		if !refused(row) {
			continue
		}
		report.Findings = append(report.Findings, *row)
	}
	return report
}

// judgeChange is the change-set judge the commit seam runs
// (changecheck.Check.Judge). It answers the refusal text, or "" when the
// change adds no refused site.
//
// It scans the tree twice, once with the carried Go files as the commit
// leaves them and once as its parent holds them, and refuses each refused
// writer row the commit adds: a (file, writer, message) the parent holds
// fewer of (addedRows). Moving an old plain line is not an addition; a new or
// reworded one is, and so is an old typed one the commit makes plain. Files
// the commit does not carry are read from the working tree, because a
// function or constant a carried writer's argument names may sit in a file
// the commit leaves alone. A commit carrying no shipped Go adds no writer row,
// so it is admitted without a scan.
//
// An added row on a line the operator-error or doctor-coverage scan also
// names on the commit's side is theirs to judge, the line the report's merge
// gives them (mergeRows), so it is never refused here (R-6 of the spec). Those
// two scans run only when the writer scan found an addition, and only over
// the side the commit leaves: a commit adding no plain writer row costs this
// package's parse of each side, two in all, and one adding one costs two more,
// operator-error's and doctor-coverage's of the carried side. Neither of their
// sites is scanned in this package.
func judgeChange(change changecheck.Change) (string, error) {
	carried := make([]string, 0, len(change.Paths)+len(change.Removed))
	for _, path := range slices.Concat(change.Paths, change.Removed) {
		if !shippedGo(path) {
			continue
		}
		carried = append(carried, path)
	}
	if len(carried) == 0 {
		return "", nil
	}
	after, err := sideOverlay(carried, change.After)
	if err != nil {
		return "", err
	}
	before, err := sideOverlay(carried, change.Before)
	if err != nil {
		return "", err
	}
	afterRows, err := writerRows(change.Root, after)
	if err != nil {
		return "", err
	}
	if err := checkFloor(SourceWriter, len(afterRows), checkoutFloors.WriterRows); err != nil {
		return "", err
	}
	beforeRows, err := writerRows(change.Root, before)
	if err != nil {
		return "", err
	}
	report := addedRows(beforeRows, afterRows, carried)
	if len(report.Findings) == 0 {
		return "", nil
	}
	owned, err := ownedRows(change.Root, carried, change.After, checkoutFloors, noDoors)
	if err != nil {
		return "", err
	}
	report.Findings = unowned(report.Findings, owned)
	if len(report.Findings) == 0 {
		return "", nil
	}
	return report.Text(), nil
}

// unowned keeps the rows that share no file and line with an owned row: the
// writer rows the report's merge keeps as this package's (mergeRows).
func unowned(rows, owned []Row) []Row {
	ownedLines := make(map[siteKey]bool, len(owned))
	for i := range owned {
		ownedLines[siteKey{file: owned[i].File, line: owned[i].Line}] = true
	}
	kept := make([]Row, 0, len(rows))
	for i := range rows {
		if ownedLines[siteKey{file: rows[i].File, line: rows[i].Line}] {
			continue
		}
		kept = append(kept, rows[i])
	}
	return kept
}

// noDoors answers no door for any file: the commit refusal names no door, so
// the judge does not read the door pages.
func noDoors(string) []string { return nil }

// sideOverlay reads each carried path from one side of the commit.
func sideOverlay(paths []string, side changecheck.Side) (map[string]overlayFile, error) {
	overlay := make(map[string]overlayFile, len(paths))
	for _, path := range paths {
		text, present, err := side(path)
		if err != nil {
			return nil, err
		}
		overlay[path] = overlayFile{text: text, present: present}
	}
	return overlay, nil
}

// addedRows answers the refused rows in carried files that the after side
// holds more of than the before side, keyed by file, writer and message. A
// row the parent already held is the audit's, listed by report, never the
// commit's.
func addedRows(before, after []Row, carried []string) CheckReport {
	isCarried := make(map[string]bool, len(carried))
	for _, path := range carried {
		isCarried[path] = true
	}
	key := func(row *Row) string {
		return row.File + "\x00" + row.Writer + "\x00" + row.Message
	}
	held := make(map[string]int)
	for i := range before {
		row := &before[i]
		if !refused(row) {
			continue
		}
		held[key(row)]++
	}
	report := CheckReport{Findings: []Row{}}
	report.Scope = "the files this commit carries against its parent (" + strconv.Itoa(len(carried)) + " Go file(s))"
	for i := range after {
		row := &after[i]
		if !isCarried[row.File] {
			continue
		}
		report.Judged++
		if !refused(row) {
			continue
		}
		if held[key(row)] > 0 {
			held[key(row)]--
			continue
		}
		report.Findings = append(report.Findings, *row)
	}
	return report
}

// shippedGo reports whether path is a Go source file that is not a test.
func shippedGo(path string) bool {
	if !strings.HasSuffix(path, ".go") {
		return false
	}
	return !strings.HasSuffix(path, "_test.go")
}

// TreeReport is the answer of `report`: every row, sorted by pre-rank,
// surface, file and line, then one count per surface.
type TreeReport struct {
	Sites    int            `json:"sites"`
	Rows     []Row          `json:"rows"`
	Surfaces []SurfaceCount `json:"surfaces"`
}

// SurfaceCount is the rows of one surface, by status.
type SurfaceCount struct {
	Surface           Surface `json:"surface"`
	Sites             int     `json:"sites"`
	Typed             int     `json:"typed"`
	PlainNamesCommand int     `json:"plain-names-command"`
	Plain             int     `json:"plain"`
}

// treeReport sorts an inventory's rows and counts them for every surface,
// a surface with no row included, so the count lines always name each one.
func treeReport(inventory Inventory) TreeReport {
	rows := slices.Clone(inventory.Rows)
	slices.SortFunc(rows, func(a, b Row) int {
		return cmp.Or(
			cmp.Compare(a.PreRank, b.PreRank),
			cmp.Compare(a.Surface, b.Surface),
			cmp.Compare(a.File, b.File),
			cmp.Compare(a.Line, b.Line),
		)
	})
	report := TreeReport{Sites: len(rows), Rows: rows, Surfaces: make([]SurfaceCount, 0, len(surfaces))}
	for _, surface := range surfaces {
		count := SurfaceCount{Surface: surface}
		for i := range rows {
			row := &rows[i]
			if row.Surface != surface {
				continue
			}
			count.Sites++
			switch row.Status {
			case StatusTyped:
				count.Typed++
			case StatusPlainNamesCommand:
				count.PlainNamesCommand++
			case StatusPlain:
				count.Plain++
			case StatusUnspecified:
			}
		}
		report.Surfaces = append(report.Surfaces, count)
	}
	return report
}

// Text renders the total, one line per row, then one count line per surface.
// A message is printed Go-quoted, so a newline in its literal stays on the
// row's one line.
func (r TreeReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("error-inventory report: ").Int(int64(r.Sites)).Str(" message site(s)\n")
	for i := range r.Rows {
		row := &r.Rows[i]
		door := noDoor
		if len(row.Doors) > 0 {
			door = strings.Join(row.Doors, ",")
		}
		tb.Str("  ").Str(row.File).Byte(':').Int(int64(row.Line)).
			Str(" surface=").Str(row.Surface.String()).
			Str(" status=").Str(row.Status.String()).
			Str(" pre-rank=").Int(int64(row.PreRank)).
			Str(" package=").Str(row.Package).
			Str(" door=").Str(door).
			Byte(' ').Str(row.Writer).Byte(' ').Str(strconv.Quote(row.Message)).Byte('\n')
	}
	for _, count := range r.Surfaces {
		tb.Str("surface=").Str(count.Surface.String()).
			Str(" sites=").Int(int64(count.Sites)).
			Str(" typed=").Int(int64(count.Typed)).
			Str(" plain-names-command=").Int(int64(count.PlainNamesCommand)).
			Str(" plain=").Int(int64(count.Plain)).Byte('\n')
	}
	return tb.String()
}

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
	tb.Str("Fix: hand the writer the operator error type with a fix (internal/core/opererr):\n")
	tb.Str("  opererr.New(problem, why, fix)\n")
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
// It finds no site yet: the inventory lists none (Scan), so no added row
// exists to refuse. It still refuses to judge a change whose carried side
// cannot be read, because a judge that cannot look is never an admission.
func judgeChange(change changecheck.Change) (string, error) {
	report := CheckReport{Scope: "the Go files the change carries", Findings: []Row{}}
	for _, path := range change.Paths {
		if !shippedGo(path) {
			continue
		}
		if _, _, err := change.After(path); err != nil {
			return "", err
		}
	}
	if len(report.Findings) == 0 {
		return "", nil
	}
	return report.Text(), nil
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

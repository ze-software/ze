// Design: docs/architecture/doctor-and-health-checks.md -- the doctor-coverage gate
//
// Overview: doctorcoverage.go -- the site kinds and statuses, and the scan
//
// report.go holds what `le arch doctor-coverage` ANSWERS, apart from what
// produced it: the check's refusal, the tree report grouped by door area and
// owner package, and the commit seam's judgement of the sites a commit adds.

package archdoctorcoverage

import (
	pathpkg "path"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/commit/changecheck"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// noDoor is the group of a file no door covers.
const noDoor = "(no door)"

// CheckReport is the answer of one check: the scope it judged, how many
// problem sites sit on that scope's lines, and the refused ones.
type CheckReport struct {
	// Scope says what was judged, in words.
	Scope    string               `json:"scope"`
	Base     repochanged.LineBase `json:"base"`
	Judged   int                  `json:"judged"`
	Findings []Site               `json:"findings"`
}

// exitCode is 1 on a finding and 0 on none.
func (r CheckReport) exitCode() int {
	if len(r.Findings) > 0 {
		return 1
	}
	return 0
}

// ze point: user-facing-errors/directives/an-error-names-the-cause-and-the-fix
// Text renders the scope line, one line per refused site, why, and the cover
// on lines of its own. It ends in a newline.
func (r CheckReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("doctor-coverage scope: ").Str(r.Scope).Str(", ").Int(int64(r.Judged)).Str(" problem site(s) judged\n")
	if len(r.Findings) == 0 {
		return tb.Str("doctor-coverage: OK\n").String()
	}
	tb.Str("doctor-coverage: ").Int(int64(len(r.Findings))).Str(" problem site(s) name no doctor code a doctor check declares:\n")
	for i := range r.Findings {
		site := &r.Findings[i]
		tb.Str("  ").Str(site.File).Byte(':').Int(int64(site.Line)).Str(": ").Str(site.Call).
			Str(" \"").Str(site.Message).Str("\": ").Str(statusReason(*site)).Byte('\n')
	}
	tb.Byte('\n')
	tb.Str("Why: a Warn or Error log line, or a report-bus raise, is a problem the operator can hit, and ze doctor reports every problem with its cause and fix. A site whose code no registered doctor check declares has no check that answers for it. No marker comment exempts a site.\n")
	tb.Str("Fix: give the call a code attribute naming a code a registered doctor check declares (internal/core/diagnostic/codes.go):\n")
	tb.Str("  \"code\", diagnostic.CodeDoctor<Name>\n")
	tb.Str("For a transient class, also raise the event on the report bus under that code, so it is reported, not only logged:\n")
	tb.Str("  report.RaiseError(source, diagnostic.CodeDoctor<Name>, subject, message)\n")
	tb.Str("The sites already in the tree are listed by ./le arch doctor-coverage report.\n")
	return tb.String()
}

// statusReason says in words why a refused site is refused.
func statusReason(site Site) string {
	switch site.Status {
	case StatusNoCode:
		return "no code attribute"
	case StatusCodeDynamic:
		return "its code " + site.CodeExpr + " is built at run time, so no check can be matched to it; name a code constant"
	case StatusCodeNotDoctor:
		return "no doctor check in the tree declares its code " + site.Code
	case StatusCovered, StatusHealthRow, StatusUnspecified:
		return site.Status.String()
	}
	return site.Status.String()
}

// TreeReport is the answer of `report`: the counts over the whole tree, and
// every site that is not covered, grouped by door area and owner package.
type TreeReport struct {
	Sites      int `json:"sites"`
	Covered    int `json:"covered"`
	Uncovered  int `json:"uncovered"`
	HealthRows int `json:"health-rows"`
	// Codes is how many doctor codes the tree's doctor checks declared.
	Codes int         `json:"codes"`
	Doors []DoorGroup `json:"doors"`
}

// DoorGroup is the uncovered sites under one door area.
type DoorGroup struct {
	// Door names the covering door pages, joined when several tie.
	Door       string         `json:"door"`
	Uncovered  int            `json:"uncovered"`
	HealthRows int            `json:"health-rows"`
	Packages   []PackageGroup `json:"packages"`
}

// PackageGroup is the uncovered sites of one owner package.
type PackageGroup struct {
	Package    string `json:"package"`
	Uncovered  int    `json:"uncovered"`
	HealthRows int    `json:"health-rows"`
	Sites      []Site `json:"sites"`
}

// treeReport groups an inventory's sites that are not covered by door and
// package, both sorted by name, and counts every site.
func treeReport(inventory Inventory) TreeReport {
	report := TreeReport{Sites: len(inventory.Sites), Codes: inventory.Codes, Doors: []DoorGroup{}}
	doors := make(map[string]map[string][]Site)
	for i := range inventory.Sites {
		site := &inventory.Sites[i]
		if site.Status == StatusCovered {
			report.Covered++
			continue
		}
		door := doorKey(site.Doors)
		if doors[door] == nil {
			doors[door] = make(map[string][]Site)
		}
		doors[door][site.Package] = append(doors[door][site.Package], *site)
	}
	for _, door := range sortedKeys(doors) {
		group := DoorGroup{Door: door, Packages: []PackageGroup{}}
		for _, pkg := range sortedKeys(doors[door]) {
			owner := PackageGroup{Package: pkg, Sites: doors[door][pkg]}
			for i := range owner.Sites {
				if owner.Sites[i].Status == StatusHealthRow {
					owner.HealthRows++
					continue
				}
				owner.Uncovered++
			}
			group.Uncovered += owner.Uncovered
			group.HealthRows += owner.HealthRows
			group.Packages = append(group.Packages, owner)
		}
		report.Uncovered += group.Uncovered
		report.HealthRows += group.HealthRows
		report.Doors = append(report.Doors, group)
	}
	return report
}

// doorKey names a site's door group.
func doorKey(doors []string) string {
	if len(doors) == 0 {
		return noDoor
	}
	return strings.Join(doors, ", ")
}

// sortedKeys answers a map's keys, sorted.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// Text renders the counts, then each door group, its packages and their
// sites, one per line.
func (r TreeReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("doctor-coverage report: ").Int(int64(r.Sites)).Str(" problem site(s): ").
		Int(int64(r.Covered)).Str(" covered, ").
		Int(int64(r.Uncovered)).Str(" uncovered, ").
		Int(int64(r.HealthRows)).Str(" health row(s) to map; ").
		Int(int64(r.Codes)).Str(" doctor code(s) declared\n")
	for _, group := range r.Doors {
		countLine(&tb, "", group.Door, group.Uncovered, group.HealthRows)
		for _, owner := range group.Packages {
			countLine(&tb, "  ", owner.Package, owner.Uncovered, owner.HealthRows)
			for i := range owner.Sites {
				site := &owner.Sites[i]
				tb.Str("    ").Str(site.File).Byte(':').Int(int64(site.Line)).Byte(' ').
					Str(site.Kind.String()).Byte(' ').Str(site.Status.String()).Byte(' ').Str(site.Call)
				if site.CodeExpr != "" {
					tb.Str(" code=").Str(site.CodeExpr)
				}
				tb.Str(" \"").Str(site.Message).Str("\"\n")
			}
		}
	}
	return tb.String()
}

// countLine renders one group's name and counts, the health rows only when
// there are some.
func countLine(tb *textbuf.Buffer, indent, name string, uncovered, healthRows int) {
	tb.Str(indent).Str(name).Str(": ").Int(int64(uncovered)).Str(" uncovered")
	if healthRows > 0 {
		tb.Str(", ").Int(int64(healthRows)).Str(" health row(s)")
	}
	tb.Byte('\n')
}

// checkLines answers the refused sites whose call sits on a changed line. A
// site already in the tree is never refused: only its own lines changing
// make it due, so the sites the tree holds stay the audit's, in report.
func checkLines(sites []Site, changed repochanged.ChangedLines) CheckReport {
	report := CheckReport{Findings: []Site{}}
	for i := range sites {
		site := &sites[i]
		if !changed.Touches(site.File, site.Line, site.EndLine) {
			continue
		}
		report.Judged++
		if !site.Status.refused() {
			continue
		}
		report.Findings = append(report.Findings, *site)
	}
	return report
}

// judgeChange is the commit seam's judge (changecheck.Check.Judge). It scans
// the tree twice, once with the carried Go files as the commit leaves them
// and once as its parent holds them, and refuses each refusable site the
// commit adds: a (file, call, message, code expression) the parent holds
// fewer of. Moving an old uncovered line is not an addition; a new or
// reworded one is, and so is an old one whose code the commit removes.
//
// Files the commit does not carry are read from the working tree, because a
// code constant a carried site names, or the check declaring its code, may sit
// in a file the commit leaves alone. Each side's covered set is read from that
// side's own tree (scan), so a commit that adds a doctor check and its first
// use together is judged against the check it adds.
func judgeChange(change changecheck.Change) (string, error) {
	return judgeChangeAt(change, coveredCodesFloor, siteFloor)
}

// judgeChangeAt is judgeChange with the covered-set floor and the site floor
// as arguments, so a fixture tree of a few sites and codes is judged by the
// same path.
func judgeChangeAt(change changecheck.Change, codesFloor, floor int) (string, error) {
	carried := make([]string, 0, len(change.Paths)+len(change.Removed))
	judged := 0
	for _, path := range slices.Concat(change.Paths, change.Removed) {
		if !shipped(path) {
			continue
		}
		carried = append(carried, path)
		if underAny(path, siteRoots) {
			judged++
		}
	}
	// A commit that carries no shipped file under the site roots adds no
	// problem site, so it is admitted without a scan: only carried files'
	// sites are judged (addedSites).
	if judged == 0 {
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
	afterInventory, err := scan(change.Root, after, codesFloor, noDoors)
	if err != nil {
		return "", err
	}
	if err := checkSitesFloor(afterInventory, floor); err != nil {
		return "", err
	}
	beforeInventory, err := scan(change.Root, before, codesFloor, noDoors)
	if err != nil {
		return "", err
	}
	report := addedSites(beforeInventory.Sites, afterInventory.Sites, carried)
	if len(report.Findings) == 0 {
		return "", nil
	}
	return report.Text(), nil
}

// noDoors answers no door for any file: the commit refusal names no door, so
// the seam does not read the door pages.
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

// addedSites answers the refusable sites in carried files that the after
// side holds more of than the before side, keyed by file, call, message and
// code expression. A site the parent already held is the audit's, listed by
// report, never the commit's.
func addedSites(before, after []Site, carried []string) CheckReport {
	isCarried := make(map[string]bool, len(carried))
	for _, path := range carried {
		isCarried[path] = true
	}
	key := func(site *Site) string {
		return site.File + "\x00" + site.Call + "\x00" + site.Message + "\x00" + site.CodeExpr
	}
	held := make(map[string]int)
	for i := range before {
		site := &before[i]
		if !site.Status.refused() {
			continue
		}
		held[key(site)]++
	}
	report := CheckReport{Findings: []Site{}}
	report.Scope = "the files this commit carries against its parent (" + strconv.Itoa(len(carried)) + " Go file(s))"
	for i := range after {
		site := &after[i]
		if !isCarried[site.File] {
			continue
		}
		report.Judged++
		if !site.Status.refused() {
			continue
		}
		if held[key(site)] > 0 {
			held[key(site)]--
			continue
		}
		report.Findings = append(report.Findings, *site)
	}
	return report
}

// changedScope says the check judged the unpushed lines.
func changedScope(base repochanged.LineBase, changed repochanged.ChangedLines) string {
	files := 0
	for path := range changed {
		if underAny(pathpkg.Clean(path), siteRoots) {
			files++
		}
	}
	var tb textbuf.Buffer
	tb.Str("the working tree against ").Str(base.Commit)
	if base.Reason != "" {
		tb.Str(" (").Str(strings.TrimSpace(base.Reason)).Byte(')')
	}
	tb.Str(", ").Int(int64(files)).Str(" changed file(s) under the site roots, changed lines only")
	return tb.String()
}

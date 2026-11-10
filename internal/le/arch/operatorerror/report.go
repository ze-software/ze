// Design: docs/contributing/ze-go-style.md -- the operator-error gate's answer
//
// Overview: operatorerror.go -- the boundary set and the return-site statuses
//
// report.go holds what `le arch operator-error` ANSWERS, apart from what
// produced it: the check's refusal, the tree report, and the commit seam's
// judgement of the sites a commit adds.

package archoperatorerror

import (
	pathpkg "path"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/commit/changecheck"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// CheckReport is the answer of one check: the scope it judged, how many
// handler return sites sit on that scope's lines, and the plain ones.
type CheckReport struct {
	// Scope says what was judged, in words.
	Scope string `json:"scope"`
	// Base is the commit the change set was diffed against, for the verify
	// stage; the commit seam judges the carried blobs and leaves it empty.
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

// Text renders the scope line, one line per plain refusal, and the fix. It
// ends in a newline.
//
// ze point: user-facing-errors/directives/an-error-names-the-cause-and-the-fix
func (r CheckReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("operator-error scope: ").Str(r.Scope).Str(", ").Int(int64(r.Judged)).Str(" handler return site(s) judged\n")
	if len(r.Findings) == 0 {
		return tb.Str("operator-error: OK\n").String()
	}
	tb.Str("operator-error: ").Int(int64(len(r.Findings))).Str(" refusal(s) an operator reads carry no fix:\n")
	answers := false
	for _, site := range r.Findings {
		verb := ") returns "
		if site.Form == FormResponse {
			verb = ") answers an error response with "
			answers = true
		}
		tb.Str("  ").Str(site.File).Byte(':').Int(int64(site.Line)).
			Str(": ").Str(site.Handler).Str(" (").Str(site.Kind.String()).Str(" registered at ").Str(site.Registration).
			Str(verb).Str(site.Expr).Byte('\n')
	}
	tb.Byte('\n')
	tb.Str("Why: a registered handler's error, and the Error text of the error response it answers, is what the operator reads on the CLI, the API or the plugin answer. Plain text says what went wrong and nothing about why or how to fix it.\n")
	tb.Str("Fix: return an opererr.Error (internal/core/opererr) that names what went wrong, why, and the command that fixes it.\n")
	if answers {
		tb.Str("Fix (an error response): answer plugin.OperatorErrorResponse(opererr.New(problem, why, fix)) (internal/component/plugin/types.go), which writes the operator error whole into Response.Error.\n")
	}
	tb.Str("The sites already in the tree are listed by ./le arch operator-error report. See ai/rules/user-facing-errors.md.\n")
	return tb.String()
}

// TreeReport is the answer of `report`: every registration's return sites in
// the tree, and the registrations the scan could not follow.
type TreeReport struct {
	Registrations int          `json:"registrations"`
	Sites         []Site       `json:"sites"`
	Unresolved    []Unresolved `json:"unresolved"`
}

// count answers how many sites hold status.
func (r TreeReport) count(status Status) int {
	total := 0
	for _, site := range r.Sites {
		if site.Status == status {
			total++
		}
	}
	return total
}

// countForm answers how many sites of form hold status.
func (r TreeReport) countForm(form Form, status Status) int {
	total := 0
	for _, site := range r.Sites {
		if site.Form != form {
			continue
		}
		if site.Status == status {
			total++
		}
	}
	return total
}

// Text renders the counts, one line per site, and the unresolved handlers.
func (r TreeReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("operator-error report: ").Int(int64(r.Registrations)).Str(" registration(s), ").
		Int(int64(len(r.Sites))).Str(" handler return site(s): ").
		Int(int64(r.count(StatusTyped))).Str(" typed, ").
		Int(int64(r.count(StatusPlain))).Str(" plain, ").
		Int(int64(r.count(StatusPassThrough))).Str(" pass-through; ").
		Int(int64(len(r.Unresolved))).Str(" handler(s) the scan could not follow\n")
	tb.Str("of those, error responses: ").
		Int(int64(r.countForm(FormResponse, StatusTyped))).Str(" typed, ").
		Int(int64(r.countForm(FormResponse, StatusPlain))).Str(" plain, ").
		Int(int64(r.countForm(FormResponse, StatusPassThrough))).Str(" pass-through\n")
	for _, site := range r.Sites {
		tb.Str("  ").Str(site.Status.String()).Byte(' ').Str(site.File).Byte(':').Int(int64(site.Line)).
			Byte(' ').Str(site.Handler).Str(" (").Str(site.Kind.String())
		if site.Form == FormResponse {
			tb.Str(", ").Str(site.Form.String())
		}
		tb.Str(") ").Str(site.Expr).Byte('\n')
	}
	if len(r.Unresolved) > 0 {
		tb.Str("unresolved (their return sites are unknown, not clean):\n")
		for _, unresolved := range r.Unresolved {
			tb.Str("  ").Str(unresolved.File).Byte(':').Int(int64(unresolved.Line)).
				Str(" (").Str(unresolved.Kind.String()).Str(") ").Str(unresolved.Expr).Byte(' ').Str(unresolved.Reason).Byte('\n')
		}
	}
	return tb.String()
}

// checkLines answers the plain sites whose return statement sits on a changed
// line. A site already in the tree is never refused: only its own line
// changing makes it due.
func checkLines(sites []Site, changed repochanged.ChangedLines) CheckReport {
	report := CheckReport{Findings: []Site{}}
	for _, site := range sites {
		if !changed.Touches(site.File, site.Line, site.Line) {
			continue
		}
		report.Judged++
		if site.Status != StatusPlain {
			continue
		}
		report.Findings = append(report.Findings, site)
	}
	return report
}

// judgeChange is the commit seam's judge (changecheck.Check.Judge). It scans
// the tree twice, once with the carried Go files as the commit leaves them
// and once as its parent holds them, and refuses each plain site the commit
// adds: a (file, handler, expression) the parent holds fewer of. Moving an
// old plain return is not an addition; a new or reworded one is.
//
// Files the commit does not carry are read from the working tree, because a
// handler's registration may sit in a file the commit leaves alone.
func judgeChange(change changecheck.Change) (string, error) {
	return judgeChangeAt(change, registrationFloor)
}

// judgeChangeAt is judgeChange with the registration floor as an argument, so
// a fixture tree of a few registrations is judged by the same path.
func judgeChangeAt(change changecheck.Change, floor int) (string, error) {
	carried := make([]string, 0, len(change.Paths)+len(change.Removed))
	for _, path := range slices.Concat(change.Paths, change.Removed) {
		if !shipped(path) {
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
	afterInventory, err := scan(change.Root, after)
	if err != nil {
		return "", err
	}
	if err := checkFloor(afterInventory, floor); err != nil {
		return "", err
	}
	beforeInventory, err := scan(change.Root, before)
	if err != nil {
		return "", err
	}
	report := addedSites(beforeInventory.Sites, afterInventory.Sites, carried)
	if len(report.Findings) == 0 {
		return "", nil
	}
	return report.Text(), nil
}

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

// addedSites answers the plain sites in carried files that the after side
// holds more of than the before side, keyed by file, handler and expression.
// An error response's expression is its Error field, so the key tells it
// from a returned error of the same text.
func addedSites(before, after []Site, carried []string) CheckReport {
	isCarried := make(map[string]bool, len(carried))
	for _, path := range carried {
		isCarried[path] = true
	}
	key := func(site Site) string { return site.File + "\x00" + site.Handler + "\x00" + site.Expr }
	held := make(map[string]int)
	for _, site := range before {
		if site.Status != StatusPlain {
			continue
		}
		held[key(site)]++
	}
	report := CheckReport{Findings: []Site{}}
	report.Scope = "the files this commit carries against its parent (" + strconv.Itoa(len(carried)) + " Go file(s))"
	for _, site := range after {
		if !isCarried[site.File] {
			continue
		}
		report.Judged++
		if site.Status != StatusPlain {
			continue
		}
		if held[key(site)] > 0 {
			held[key(site)]--
			continue
		}
		report.Findings = append(report.Findings, site)
	}
	return report
}

// changedScope says the verify stage judged the unpushed lines.
func changedScope(base repochanged.LineBase, changed repochanged.ChangedLines) string {
	files := 0
	for path := range changed {
		if shipped(pathpkg.Clean(path)) {
			files++
		}
	}
	var tb textbuf.Buffer
	tb.Str("the working tree against ").Str(base.Commit)
	if base.Reason != "" {
		tb.Str(" (").Str(strings.TrimSpace(base.Reason)).Byte(')')
	}
	tb.Str(", ").Int(int64(files)).Str(" changed shipped Go file(s), changed lines only")
	return tb.String()
}

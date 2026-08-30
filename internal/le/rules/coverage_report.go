// Design: docs/architecture/core-design.md -- the gate map, as an answer
// Overview: coverage.go -- the join this renders
// Overview: actions.go -- the action that runs this
// Detail: hooktable.go -- the published claim the last block compares
//
// coverage_report.go renders the `le rules gate-map-report` answer. The answer
// contains a payload for every set and the page that the script printed.
//
// The script defines two streams. STDOUT contains the report and the published
// table because they are the answer. STDERR contains only the reason for a
// genuine failure. The payload also stores Diagnosis, so `| json` keeps the
// action reason that a pipe would otherwise lose.

package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// GatedPoint is one point and the checks that name it.
type GatedPoint struct {
	Ref    string   `json:"ref"`
	Checks []string `json:"checks"`
}

// DanglingBinding is a binding that resolved to nothing.
type DanglingBinding struct {
	File  string `json:"file"`
	Check string `json:"check"`
	Ref   string `json:"ref"`
	Why   string `json:"why"`
}

// UnboundCheck is a check declaring it enforces no written point, with the
// reason the declaration requires.
type UnboundCheck struct {
	Check  string `json:"check"`
	Reason string `json:"reason"`
}

// MissingLink is a declared link naming nothing: a rationale path absent from
// disk, or an exception naming no point.
type MissingLink struct {
	Ref    string `json:"ref"`
	Target string `json:"target"`
	Why    string `json:"why"`
}

// Tally is one name and how many of it there are.
type Tally struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// CoverageReport is what `le rules gate-map-report` answers.
type CoverageReport struct {
	// Refused names native roster problems that stopped the join from running.
	// Nothing else in this report is filled when it is set.
	Refused []string `json:"refused"`
	// Empty names the population the join read nothing from.
	Empty []string `json:"empty"`

	Points   int `json:"points"`
	Bindings int `json:"bindings"`
	Checks   int `json:"checks"`

	Gated    []GatedPoint      `json:"gated"`
	Dangling []DanglingBinding `json:"dangling"`
	Unbound  []UnboundCheck    `json:"unbound"`

	// Baseline says whether any hookruntime Go source existed at git HEAD.
	// Without one the ratchets cannot fire, and the report says so.
	Baseline     bool     `json:"baseline"`
	Regressed    []string `json:"regressed"`
	NoHeadFor    []string `json:"no-head-for"`
	DeclaredNone []string `json:"declared-none"`

	MissingRationale []MissingLink `json:"missing-rationale"`
	MissingException []MissingLink `json:"missing-exception"`

	Ungated       []string `json:"ungated"`
	Candidates    int      `json:"candidates"`
	Structural    int      `json:"structural"`
	Headings      int      `json:"headings"`
	Fences        int      `json:"fences"`
	UngatedByKind []Tally  `json:"ungated-by-kind"`
	MostUngated   []Tally  `json:"most-ungated"`

	Rationales int `json:"rationales"`
	Excepted   int `json:"excepted"`
	Exceptions int `json:"exceptions"`

	// Doc is the published claim, tree-relative. DocMissing says it was not
	// there, which stops the comparison rather than passing it.
	Doc        string   `json:"doc"`
	DocMissing bool     `json:"doc-missing"`
	Published  []string `json:"published"`

	// Diagnosis names each line the gate writes to stderr: the one reason a
	// reader acts on, and the published-table refusal.
	Diagnosis []string `json:"diagnosis"`
}

// Failed reports whether the gate map found anything that fails.
func (r *CoverageReport) Failed() bool {
	return len(r.Refused) > 0 || len(r.Empty) > 0 || r.DocMissing ||
		len(r.Dangling) > 0 || len(r.Regressed) > 0 || len(r.DeclaredNone) > 0 ||
		len(r.MissingRationale) > 0 || len(r.MissingException) > 0 ||
		len(r.Published) > 0
}

// writeDiagnosis sends each stderr line the script sends, in its order. Only a
// genuine failure reaches this stream.
func (r *CoverageReport) writeDiagnosis() {
	for _, line := range r.Diagnosis {
		fmt.Fprintln(os.Stderr, line) //nolint:errcheck // CLI output
	}
}

// Text renders the page the script printed on stdout.
func (r *CoverageReport) Text() string {
	var tb textbuf.Buffer
	if len(r.Refused) > 0 {
		return ""
	}
	if len(r.Empty) > 0 {
		// The join read nothing, so there are no sets to print. The published
		// claim is still compared and still printed, which is the script's own
		// control flow: an empty corpus does not excuse an unpublished check.
		for _, line := range r.Empty {
			tb.Str(line).Byte('\n')
		}
		r.writePublished(&tb)
		return tb.String()
	}

	tb.Str("gate map: ").Int(int64(r.Points)).Str(" points, ").Int(int64(r.Bindings)).
		Str(" bindings, ").Int(int64(r.Checks)).Str(" checks\n\n")

	tb.Str("GATED: ").Int(int64(len(r.Gated))).Str(" point(s) named by ").
		Int(int64(r.gatedCheckCount())).Str(" check(s)\n")
	for _, point := range r.Gated {
		tb.Str("  ").Str(point.Ref).Str("  <- ").Join(point.Checks, ", ").Byte('\n')
	}

	tb.Str("\nDANGLING: ").Int(int64(len(r.Dangling))).Byte('\n')
	for _, binding := range r.Dangling {
		tb.Str("  ").Str(binding.File).Str(": ").Str(binding.Check).Str(" -> ").
			Str(binding.Ref).Str(" (").Str(binding.Why).Str(")\n")
	}

	if r.Baseline {
		tb.Byte('\n').Str("REGRESSED: ").Int(int64(len(r.Regressed))).
			Str(" point(s) gated at HEAD, gated by nothing now\n")
		for _, ref := range r.Regressed {
			tb.Str("  ").Str(ref).Byte('\n')
		}
		r.writeNoHead(&tb)
		tb.Byte('\n').Str("DECLARED NONE: ").Int(int64(len(r.DeclaredNone))).
			Str(" check(s) named a point at HEAD and declare `none` now\n")
		for _, line := range r.DeclaredNone {
			tb.Str("  ").Str(line).Byte('\n')
		}
	} else {
		tb.Byte('\n').Str("REGRESSED: no HEAD baseline (git could not answer, or no hookruntime ").
			Str("Go source has a version at HEAD); not ratcheted\n")
		r.writeNoHead(&tb)
	}

	tb.Byte('\n').Str("UNBOUND: ").Int(int64(len(r.Unbound))).
		Str(" check(s) declare `none`, each with a reason\n")
	for _, check := range r.Unbound {
		tb.Str("  ").Str(check.Check).Str(": ").Str(check.Reason).Byte('\n')
	}

	tb.Byte('\n').Str("MISSING RATIONALE: ").Int(int64(len(r.MissingRationale))).Byte('\n')
	for _, link := range r.MissingRationale {
		tb.Str("  ").Str(link.Ref).Str(" -> ").Str(link.Target).Str(" (").Str(link.Why).Str(")\n")
	}

	tb.Byte('\n').Str("MISSING EXCEPTION: ").Int(int64(len(r.MissingException))).Byte('\n')
	for _, link := range r.MissingException {
		tb.Str("  ").Str(link.Ref).Str(" -> ").Str(link.Target).Str(" (").Str(link.Why).Str(")\n")
	}

	tb.Byte('\n').Str("UNGATED: ").Int(int64(len(r.Ungated))).Str(" of ").
		Int(int64(r.Candidates)).Str(" instruction points\n")
	tb.Str("  denominator excludes ").Int(int64(r.Structural)).Str(" structural points (").
		Int(int64(r.Headings)).Str(" heading, ").Int(int64(r.Fences)).Str(" fence)\n")
	tb.Str("  by kind: ").Str(tallyLine(r.UngatedByKind)).Byte('\n')
	tb.Str("  most ungated: ").Str(tallyLine(r.MostUngated)).Byte('\n')

	tb.Byte('\n').Str("RATIONALE: ").Int(int64(r.Rationales)).Str(" of ").
		Int(int64(r.Candidates)).Str(" instruction points name a record\n")
	tb.Str("  coverage is a measurement and exits 0 whatever the number; ").
		Str("an invented link is worse than an absent one\n")

	tb.Byte('\n').Str("EXCEPTED: ").Int(int64(r.Excepted)).Str(" of ").
		Int(int64(r.Candidates)).Str(" instruction points name an exception, naming ").
		Int(int64(r.Exceptions)).Str(" point(s)\n")
	tb.Str("  coverage is a measurement and exits 0 whatever the number; most ").
		Str("instructions state no exception\n")

	r.writePublished(&tb)
	return tb.String()
}

// writePublished renders the comparison against the published claim. A doc that
// is not there stops the comparison rather than passing it, so nothing is
// printed for it and the reason reaches stderr instead.
func (r *CoverageReport) writePublished(tb *textbuf.Buffer) {
	if r.DocMissing {
		return
	}
	tb.Byte('\n').Str("PUBLISHED: ").Int(int64(len(r.Published))).
		Str(" disagreement(s) with `").Str(r.Doc).Str("`\n")
	for _, problem := range r.Published {
		tb.Str("  ").Str(problem).Byte('\n')
	}
}

// writeNoHead names each native Go source absent at git HEAD. That absence
// removes all its bindings from the baseline.
func (r *CoverageReport) writeNoHead(tb *textbuf.Buffer) {
	for _, name := range r.NoHeadFor {
		tb.Str("  no version at HEAD: ").Str(name).Str(" (its bindings are not ratcheted)\n")
	}
}

// gatedCheckCount answers how many distinct checks name at least one point.
func (r *CoverageReport) gatedCheckCount() int {
	seen := map[string]bool{}
	for _, point := range r.Gated {
		for _, check := range point.Checks {
			seen[check] = true
		}
	}
	return len(seen)
}

// tallyLine renders a counted list the way the script's Counter did.
func tallyLine(tallies []Tally) string {
	parts := make([]string, 0, len(tallies))
	var tb textbuf.Buffer
	for _, tally := range tallies {
		tb.Reset()
		parts = append(parts, tb.Str(tally.Name).Byte(' ').Int(int64(tally.Count)).String())
	}
	tb.Reset()
	return tb.Join(parts, ", ").String()
}

// Coverage builds the gate map over one checkout and answers the report.
func Coverage(tree string) (*CoverageReport, error) {
	var report CoverageReport
	var tb textbuf.Buffer

	sources, problems := nativeHookSources(tree)
	if len(problems) > 0 {
		report.Refused = problems
		for _, problem := range problems {
			tb.Reset()
			report.Diagnosis = append(report.Diagnosis, tb.Str("rules-points: ").Str(problem).String())
		}
		return &report, nil
	}

	pointsDir := filepath.Join(tree, filepath.FromSlash(pointsRel))
	gm, err := buildGateMap(sources, pointsDir, tree)
	if err != nil {
		return nil, err
	}

	names := sortedKeys(sources)
	head, noHeadFor, gitAnswered := headSources(tree, names)
	haveBaseline := gitAnswered && len(head) > 0

	nowIDs := map[string]bool{}
	for ref := range gm.Points {
		nowIDs[ref] = true
	}
	var regressed, declaredNone []string
	if haveBaseline {
		atHead, err := bindingsAtHead(head)
		if err != nil {
			return nil, err
		}
		regressed = gatedRegressions(gm, atHead)
		declaredNone = unboundRegressions(gm, atHead, nowIDs)
	}

	report = fillCoverage(gm, haveBaseline)
	report.NoHeadFor = noHeadFor
	report.Regressed = regressed
	report.DeclaredNone = declaredNone

	reason := coverageReason(&report)
	if reason == "" && len(report.Empty) > 0 {
		// An empty join fails with no failing SET behind it, so the reason a
		// reader acts on is the emptiness itself.
		reason = report.Empty[0]
	}
	if reason != "" {
		tb.Reset()
		report.Diagnosis = append(report.Diagnosis, tb.Str("rules-points: ").Str(reason).String())
	}

	tb.Reset()
	docPath := filepath.Join(tree, filepath.FromSlash(rulesRel), tb.Str(docRule).Str(".md").String())
	report.Doc = relTo(tree, docPath)
	docText, err := os.ReadFile(docPath) // #nosec G304 -- a path derived from the checkout
	if err != nil {
		// A missing doc is this gate's VERDICT, not a run failure. The published
		// claim is missing, which is exactly what the gate reports. The report
		// carries the verdict, and the exit code identifies it.
		report.DocMissing = true
		tb.Reset()
		report.Diagnosis = append(report.Diagnosis,
			tb.Str("rules-points: ").Str(report.Doc).Str(" not found").String())
		return &report, nil //nolint:nilerr // the missing doc is reported, not raised
	}

	report.Published = hookTableProblems(gm, string(docText), sources)
	if len(report.Published) > 0 {
		tb.Reset()
		report.Diagnosis = append(report.Diagnosis,
			tb.Str("rules-points: the Hook-to-Rule Mapping table in ").Str(report.Doc).
				Str(" disagrees with the binding comments").String())
	}
	return &report, nil
}

// fillCoverage turns the join into the counted sets the page prints.
//
// An EMPTY result is never a pass: no point, or no binding at all, means the
// join read nothing and must say so.
func fillCoverage(gm gateMap, baseline bool) CoverageReport {
	report := CoverageReport{Baseline: baseline}
	if len(gm.Points) == 0 {
		report.Empty = []string{
			"no points under ai/rules/points/; the gate map read nothing and must not report success"}
		return report
	}
	if len(gm.Bindings) == 0 {
		report.Empty = []string{
			"no `// ze point:` binding on any registered native check; the gate map read nothing and must not report success"}
		return report
	}

	report.Points = len(gm.Points)
	report.Bindings = len(gm.Bindings)

	for _, ref := range sortedKeys(gm.Gated) {
		seen := map[string]bool{}
		for _, binding := range gm.Gated[ref] {
			seen[binding.Check] = true
		}
		report.Gated = append(report.Gated, GatedPoint{Ref: ref, Checks: sortedUnique(seen)})
	}
	report.Checks = report.gatedCheckCount() + len(gm.Unbound)

	for _, binding := range gm.Dangling {
		why := "names no point on disk"
		if binding.Check == noCheck {
			why = "sits above no check"
		}
		report.Dangling = append(report.Dangling, DanglingBinding{
			File: binding.File, Check: binding.Check, Ref: binding.Ref, Why: why,
		})
	}
	for _, binding := range gm.Unbound {
		report.Unbound = append(report.Unbound, UnboundCheck{Check: binding.Check, Reason: binding.Reason})
	}
	for _, pair := range gm.MissingRationale {
		report.MissingRationale = append(report.MissingRationale, MissingLink{
			Ref: pair[0], Target: pair[1], Why: "no such record: absent or empty",
		})
	}
	for _, pair := range gm.MissingException {
		why := "no such point"
		if pair[0] == pair[1] {
			why = "a point cannot except itself"
		}
		report.MissingException = append(report.MissingException, MissingLink{
			Ref: pair[0], Target: pair[1], Why: why,
		})
	}

	kinds := map[string]int{}
	for _, kind := range gm.Points {
		kinds[kind]++
	}
	report.Headings = kinds[kindHeading]
	report.Fences = kinds[kindFence]
	report.Structural = report.Headings + report.Fences

	candidates := gm.candidates()
	report.Candidates = len(candidates)
	report.Ungated = gm.Ungated
	report.UngatedByKind = countByKind(gm)
	report.MostUngated = mostUngatedRules(gm.Ungated, 5)

	instruction := map[string]bool{}
	for _, ref := range candidates {
		instruction[ref] = true
	}
	for ref := range gm.Rationales {
		if instruction[ref] {
			report.Rationales++
		}
	}
	exceptions := map[string]bool{}
	for ref, targets := range gm.Excepted {
		if instruction[ref] {
			report.Excepted++
		}
		for _, target := range targets {
			exceptions[target] = true
		}
	}
	report.Exceptions = len(exceptions)
	return report
}

// countByKind answers how many ungated points carry each kind, by kind name.
func countByKind(gm gateMap) []Tally {
	counts := map[string]int{}
	for _, ref := range gm.Ungated {
		counts[gm.Points[ref]]++
	}
	out := make([]Tally, 0, len(counts))
	for _, kind := range sortedKeys(counts) {
		out = append(out, Tally{Name: kind, Count: counts[kind]})
	}
	return out
}

// mostUngatedRules answers the n rules holding the most ungated points.
//
// Ties keep the order the ungated list gave them, which is alphabetical by
// point id, so the answer is stable across runs. That is Counter.most_common's
// own tie-break: it sorts the insertion order by count and keeps the first
// among equals.
func mostUngatedRules(ungated []string, n int) []Tally {
	counts := map[string]int{}
	var order []string
	for _, ref := range ungated {
		rule, _, _ := strings.Cut(ref, "/")
		if _, seen := counts[rule]; !seen {
			order = append(order, rule)
		}
		counts[rule]++
	}
	out := make([]Tally, 0, len(order))
	for _, rule := range order {
		out = append(out, Tally{Name: rule, Count: counts[rule]})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if len(out) > n {
		out = out[:n]
	}
	return slices.Clip(out)
}

// coverageReason answers the one line that a reader acts on. It selects the
// first failing set in the script's order because later failures usually result
// from an earlier failure.
func coverageReason(r *CoverageReport) string {
	var tb textbuf.Buffer
	switch {
	case len(r.Dangling) > 0:
		return tb.Int(int64(len(r.Dangling))).
			Str(" dangling binding(s): each names a point that does not exist, or sits above no check").String()
	case len(r.Regressed) > 0:
		return tb.Int(int64(len(r.Regressed))).
			Str(" point(s) were gated at HEAD and are gated by nothing now; restore the binding, or say which check replaces it").String()
	case len(r.DeclaredNone) > 0:
		return tb.Int(int64(len(r.DeclaredNone))).
			Str(" check(s) named a point at HEAD and declare `none` now, and each of those points is still on disk; repoint the binding at where the point moved to. A renamed point does not stop being enforced").String()
	case len(r.MissingRationale) > 0:
		return tb.Int(int64(len(r.MissingRationale))).
			Str(" point(s) name a rationale that is not on disk; repoint it at where the record moved to, or drop the field rather than leave it naming nothing").String()
	case len(r.MissingException) > 0:
		return tb.Int(int64(len(r.MissingException))).
			Str(" point(s) name an `excepted-by` point that does not exist; restore the exception, or repoint the general instruction at where the exception moved to. Do not drop the field while the general statement still needs the carve-out a reader would otherwise miss").String()
	default:
		return ""
	}
}

// Design: docs/contributing/running-commands.md -- le test redwalk check
// Overview: redwalk.go -- the walk these verdicts judge
//
// verdict.go judges the clean and the broken run of one walk through the
// overlay package's shared verdict readers, the ones `le rfc record` judges
// its observations with, and holds what the action answers.

package testredwalk

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/test/overlay"
)

// overlayArea is the session scratch area every walk's overlay is staged in.
const overlayArea = "redwalk"

// verdict is what one walk found.
type verdict uint8

const (
	verdictUnspecified verdict = iota
	// verdictRed: clean green, broken red, and the red is the unit's own.
	verdictRed
	// verdictNoBaseline: the clean run was red or did not run the unit.
	verdictNoBaseline
	// verdictGreen: the unit stayed green under the break.
	verdictGreen
	// verdictNotARed: the broken run failed, but not as the unit's red.
	verdictNotARed
	// verdictUnobserved: the broken run never gave the unit a verdict.
	verdictUnobserved
)

// name answers the verdict as the report prints it.
func (v verdict) name() string {
	switch v {
	case verdictRed:
		return "red"
	case verdictNoBaseline:
		return "no-baseline"
	case verdictGreen:
		return "green"
	case verdictNotARed:
		return "not-a-red"
	case verdictUnobserved:
		return "unobserved"
	case verdictUnspecified:
		panic("BUG: redwalk verdict unspecified: every judged run names its verdict")
	}
	panic("BUG: redwalk verdict out of range")
}

// fix answers the next step a reader of the verdict takes.
func (v verdict) fix() string {
	switch v {
	case verdictRed:
		return ""
	case verdictNoBaseline:
		return "make the test pass, and run, on the tree as it is, then run the walk again"
	case verdictGreen:
		return "make the test fail under this break, or walk a break the test is meant to catch"
	case verdictNotARed:
		return "change the break so the code still builds and the test itself fails, then run the walk again"
	case verdictUnobserved:
		return "run the walk where the test runs and finishes, then run it again"
	case verdictUnspecified:
		panic("BUG: redwalk verdict unspecified: every judged run names its verdict")
	}
	panic("BUG: redwalk verdict out of range")
}

// Report is the whole answer of one walk.
type Report struct {
	Carrier string `json:"carrier"`
	Unit    string `json:"unit"`
	Verdict string `json:"verdict"`
	// Clean is the clean run's line that gives the unit its pass.
	Clean string `json:"clean,omitempty"`
	// Broken is the broken run's line that gives the unit its verdict.
	Broken string `json:"broken,omitempty"`
	// Overlay lists the files the break replaced for the broken run.
	Overlay []string `json:"overlay"`
	Reason  string   `json:"reason,omitempty"`
	Fix     string   `json:"fix,omitempty"`
	Excerpt string   `json:"excerpt,omitempty"`
	// Leftover names an overlay directory the walk could not remove.
	Leftover string `json:"leftover,omitempty"`
	verdict  verdict
}

// Code answers the exit status: 0 only when the break reddened the unit.
func (r Report) Code() int {
	if r.verdict == verdictRed {
		return 0
	}
	return 1
}

// Text renders the report for a person. It ends in a newline.
func (r Report) Text() string {
	var tb textbuf.Buffer
	tb.Str("red walk: ").Str(r.Carrier).Byte(' ').Str(r.Unit).Str(": ").Str(r.Verdict).Byte('\n')
	tb.Str("  overlay: ").Str(strings.Join(r.Overlay, ", ")).Byte('\n')
	if r.Clean != "" {
		tb.Str("  clean:   ").Str(r.Clean).Byte('\n')
	}
	if r.Broken != "" {
		tb.Str("  broken:  ").Str(r.Broken).Byte('\n')
	}
	if r.Reason != "" {
		tb.Str("  ").Str(r.Reason).Byte('\n')
	}
	if r.Leftover != "" {
		tb.Str("  could not remove the overlay: ").Str(r.Leftover).Byte('\n')
	}
	if r.Fix != "" {
		tb.Str("  fix: ").Str(r.Fix).Byte('\n')
	}
	if r.Excerpt != "" {
		tb.Str("  output of the run:\n")
		for line := range strings.Lines(r.Excerpt) {
			tb.Str("    ").Str(strings.TrimRight(line, "\n")).Byte('\n')
		}
	}
	return tb.String()
}

// conclude sets the verdict, its fix, the reason and the excerpt.
func (r *Report) conclude(v verdict, reason, output string) {
	r.verdict = v
	r.Verdict = v.name()
	r.Fix = v.fix()
	r.Reason = reason
	if v != verdictRed {
		r.Excerpt = overlay.Excerpt(output)
	}
}

// runner runs the walk's carrier once, under the overlay at the given path, or
// clean when the path is empty, and answers whether it passed and its output.
// A build the carrier needs before the run, that fails, is a *buildError.
type runner func(overlayPath string) (bool, string, error)

// buildError is a carrier build that failed before the unit could run.
type buildError struct{ err error }

func (e *buildError) Error() string { return e.err.Error() }
func (e *buildError) Unwrap() error { return e.err }

// walk is one red walk: a carrier unit, the break's files and the runner.
type walk struct {
	tree   string
	target target
	files  []overlay.File
	run    runner
}

// check runs the carrier clean, then under the break, and judges both. The
// overlay lives in session scratch and is removed once the broken run ends;
// the shared tree is never written, so a killed walk leaves no break behind.
func (w *walk) check() (Report, error) {
	report := Report{Carrier: w.target.kind.name(), Unit: w.target.subject, Overlay: overlayList(w.files)}
	passed, output, err := w.run("")
	if err != nil {
		reason := err.Error()
		if _, built := errors.AsType[*buildError](err); built {
			report.conclude(verdictNoBaseline, "the tree does not build as it is: "+reason, output)
			return report, nil
		}
		report.conclude(verdictUnobserved, reason, output)
		return report, nil
	}
	line, why := w.target.judgeClean(passed, output)
	if why != "" {
		report.conclude(verdictNoBaseline, why, output)
		return report, nil
	}
	report.Clean = line

	path, err := overlay.Write(w.tree, overlayArea, w.files...)
	if err != nil {
		return Report{}, err
	}
	passed, output, err = w.run(path)
	if removeErr := os.RemoveAll(filepath.Dir(path)); removeErr != nil {
		report.Leftover = removeErr.Error()
	}

	if err != nil {
		if _, built := errors.AsType[*buildError](err); built {
			report.conclude(verdictNotARed, "the broken build does not compile, so the failure is not a red "+
				"of the unit: "+err.Error(), output)
			return report, nil
		}
		report.conclude(verdictUnobserved, err.Error(), output)
		return report, nil
	}
	line, judged, why := w.target.judgeBroken(passed, output)
	report.Broken = line
	report.conclude(judged, why, output)
	return report, nil
}

// overlayList answers the files a break replaces, a deleted one marked.
func overlayList(files []overlay.File) []string {
	names := make([]string, 0, len(files))
	for _, file := range files {
		if file.Absent {
			names = append(names, file.Rel+" (deleted)")
			continue
		}
		names = append(names, file.Rel)
	}
	return names
}

// judgeClean answers the clean run's line that gives the unit its pass, or
// why the run is no baseline. A run that did not run the unit is no green.
func (t target) judgeClean(passed bool, output string) (string, string) {
	var tb textbuf.Buffer
	if !passed {
		return "", tb.Str(t.subject).Str(" is already red before the break, so the walk has no baseline: ").
			Str("no red under the break could be told from it").String()
	}
	switch t.kind {
	case kindUnit:
		if at := overlay.GoTestLineNames(output, "--- SKIP: ", t.name); at >= 0 {
			return "", tb.Str(t.subject).Str(" skipped in the clean run: ").Str(overlay.SkipReason(output[:at])).
				Str(". A skip is not a green, so the walk has no baseline").String()
		}
		at := overlay.GoTestLineNames(output, "--- PASS: ", t.name)
		if at < 0 {
			return "", tb.Str("the clean run printed no `--- PASS: ").Str(t.name).
				Str("` line, so no test of that name ran in ").Str(t.pkg).Str(" and the walk has no baseline").String()
		}
		return lineAt(output, at), ""
	case kindCI:
		result := overlay.CIResult(output, t.name)
		if result != overlay.CIPass {
			return "", tb.Str(t.subject).Str(" did not pass in the clean run: ").
				Str(overlay.DescribeCIResult(t.name, result)).
				Str(". A skip is not a green, so the walk has no baseline").String()
		}
		return ciLine(output, t.name), ""
	case kindScenario:
		if why := overlay.CleanUnobserved(output, t.name, t.subject); why != "" {
			return "", why
		}
		verdictText, _ := overlay.ScenarioVerdict(output, t.name, overlay.VerdictPass)
		return t.name + verdictText, ""
	case kindUnspecified:
		panic("BUG: redwalk carrier unspecified: every target is parsed with its carrier")
	}
	panic("BUG: redwalk carrier out of range")
}

// judgeBroken answers the broken run's line, its verdict and the reason.
func (t target) judgeBroken(passed bool, output string) (string, verdict, string) {
	switch t.kind {
	case kindUnit:
		return t.judgeBrokenUnit(passed, output)
	case kindCI:
		return t.judgeBrokenCI(passed, output)
	case kindScenario:
		return t.judgeBrokenScenario(passed, output)
	case kindUnspecified:
		panic("BUG: redwalk carrier unspecified: every target is parsed with its carrier")
	}
	panic("BUG: redwalk carrier out of range")
}

// judgeBrokenUnit judges a Go unit: its own `--- FAIL:` line is the red.
func (t target) judgeBrokenUnit(passed bool, output string) (string, verdict, string) {
	var tb textbuf.Buffer
	if at := overlay.GoTestLineNames(output, "--- SKIP: ", t.name); at >= 0 {
		return lineAt(output, at), verdictUnobserved, tb.Str(t.subject).Str(" skipped under the break: ").
			Str(overlay.SkipReason(output[:at])).Str(". A skip proves nothing either way").String()
	}
	if at := overlay.GoTestLineNames(output, "--- FAIL: ", t.name); at >= 0 {
		return lineAt(output, at), verdictRed, ""
	}
	if passed {
		at := overlay.GoTestLineNames(output, "--- PASS: ", t.name)
		if at < 0 {
			return "", verdictUnobserved, tb.Str("the run under the break printed no line for ").Str(t.name).String()
		}
		return lineAt(output, at), verdictGreen, tb.Str(t.subject).
			Str(" stayed green under the break: the test does not discriminate this break").String()
	}
	if buildFailed(output) {
		return "", verdictNotARed, tb.Str("the broken build does not compile, so the failure is not a red of ").
			Str(t.name).String()
	}
	return "", verdictNotARed, tb.Str("the run failed under the break without a `--- FAIL: ").Str(t.name).
		Str("` line, so the failure is not a red of the unit: a crash in package initialization, a ").
		Str("sibling test or a build step failed instead").String()
}

// judgeBrokenCI judges a .ci through the runner's result line.
func (t target) judgeBrokenCI(passed bool, output string) (string, verdict, string) {
	result := overlay.CIResult(output, t.name)
	var tb textbuf.Buffer
	if result == "" {
		return "", verdictUnobserved, overlay.DescribeUnobservedBreak(t.subject,
			overlay.DescribeCIResult(t.name, result), output)
	}
	if result == overlay.CISkip {
		return ciLine(output, t.name), verdictUnobserved, overlay.DescribeUnobservedBreak(t.subject,
			overlay.DescribeCIResult(t.name, result), output)
	}
	if result == overlay.CIFail {
		return ciLine(output, t.name), verdictRed, ""
	}
	if result == overlay.CITimeout {
		return ciLine(output, t.name), verdictRed, ""
	}
	if passed {
		return ciLine(output, t.name), verdictGreen, tb.Str(t.subject).
			Str(" stayed green under the break: the test does not discriminate this break").String()
	}
	return ciLine(output, t.name), verdictNotARed, tb.Str("the runner reported ").Str(t.name).
		Str(" as PASS, so the failing exit came from somewhere else").String()
}

// judgeBrokenScenario judges an interop scenario. Only a FAIL in the checker
// is the scenario's red: a setup or readiness failure means the checker never
// ran (AC-2).
func (t target) judgeBrokenScenario(passed bool, output string) (string, verdict, string) {
	if why := overlay.BrokenUnobserved(output, t.name, t.subject); why != "" {
		return "", verdictUnobserved, why
	}
	var tb textbuf.Buffer
	failure, failed := overlay.ScenarioVerdict(output, t.name, overlay.VerdictFail)
	if failed {
		line := t.name + failure
		phase := overlay.FailPhase(failure)
		if phase == string(interoplab.PhaseChecker) {
			return line, verdictRed, ""
		}
		if phase == "" {
			return line, verdictNotARed, tb.Str("the FAIL line of ").Str(t.name).
				Str(" names no phase, so whether its checker ran is unknown and the red is not counted").String()
		}
		return line, verdictNotARed, tb.Str(t.name).Str(" failed in ").Str(phase).
			Str(", before its checker ran, so the red is not the scenario's").String()
	}
	pass, _ := overlay.ScenarioVerdict(output, t.name, overlay.VerdictPass)
	if passed {
		return t.name + pass, verdictGreen, tb.Str(t.subject).
			Str(" stayed green under the break: the test does not discriminate this break").String()
	}
	return t.name + pass, verdictNotARed, tb.Str(t.name).
		Str(" passed, so the failing exit came from somewhere else").String()
}

// buildFailed answers whether `go test` reported that the package did not
// build, which it marks on the package's FAIL line.
func buildFailed(output string) bool {
	if strings.Contains(output, " [build failed]") {
		return true
	}
	return strings.Contains(output, " [setup failed]")
}

// lineAt answers the line of output starting at offset, trimmed.
func lineAt(output string, offset int) string {
	line, _, _ := strings.Cut(output[offset:], "\n")
	return strings.TrimSpace(line)
}

// ciLine answers the runner's result line for the .ci named stem, the one
// overlay.CIResult reads its outcome from.
func ciLine(output, stem string) string {
	for line := range strings.Lines(output) {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[len(fields)-1] == stem {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

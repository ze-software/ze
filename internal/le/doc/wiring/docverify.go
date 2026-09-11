// Design: docs/architecture/core-design.md -- the complete documentation gate
// Overview: delegate.go -- the target callback table
//
// docverify.go ports ze-doc-verify's ordered shell sequence. Each stage calls
// the Go package that owns the check, and every stage runs even after a prior
// failure so the final page contains the complete diagnosis.

package docwiring

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/digest"
	"github.com/ze-software/ze/internal/le/docstocode"
	"github.com/ze-software/ze/internal/le/docvalid"
	"github.com/ze-software/ze/internal/le/journal"
	"github.com/ze-software/ze/internal/le/rules"
)

type docVerifyPage struct {
	text string
}

func (p docVerifyPage) Text() string { return p.text }

// docVerifyPathRE matches a "<path>:<line>" citation in a stage's prose. Every
// documentation stage reports a finding that way, so the page can name the files
// its failure is about without each stage threading a second structured list.
var docVerifyPathRE = regexp.MustCompile(`[A-Za-z0-9_][A-Za-z0-9_./-]*\.(?:md|go|html|templ|ya?ml|json):[0-9]+`)

// FailingPaths answers the files this page's findings cite, deduplicated and in
// first-seen order.
//
// Deriving them from the rendered text is deliberate. The page is what an
// operator reads, so a path that reaches the group is one the report already
// showed, and a stage cannot fail while quietly attributing the failure
// somewhere the reader never saw.
//
// A path is kept only when it resolves in the tree. A citation can name a file
// that has since moved, and charging a commit for a path that does not exist
// would be worse than not attributing it at all.
func (p docVerifyPage) FailingPaths() []string {
	seen := make(map[string]struct{})
	var out []string
	for _, match := range docVerifyPathRE.FindAllString(p.text, -1) {
		path := match[:strings.LastIndexByte(match, ':')]
		if _, dup := seen[path]; dup {
			continue
		}
		seen[path] = struct{}{}
		if _, err := os.Stat(path); err != nil {
			continue
		}
		out = append(out, path)
	}
	return out
}

type docVerifyStage struct {
	heading string
	run     func(root string) (any, int)
}

var docVerifyStages = [...]docVerifyStage{
	{"Documentation drift (docs claims vs registries and filesystem)...", docDriftStage},
	{"YANG/handler contract (validate-commands)...", docContractStage},
	{"Command usage contract (the model states every grammar, no description spells one)...", docUsageStage},
	{"Command help shape (a one-line summary a row renders, and a long text beside it)...", docHelpShapeStage},
	{"Source anchors (docs source references exist)...", docIndexStage},
	{"Rules render (ai/rules/<rule>.md matches ai/rules/points/)...", rulesRenderStage},
	{"Rules round trip (split every rendered rule, render it back, compare bytes)...", rulesRoundTripStage},
	{"Rules gate map (no hook check names a point that does not exist)...", rulesCoverageStage},
	{"Rules index (ai/rules/INDEX.md fresh, every rule has a summary)...", rulesIndexStage},
	{"Rule format (every ai/rules/*.md has the When/Severity block)...", rulesLintStage},
	{"Rules digest (ai/rules/TRIGGERS.md + CORE.md fresh)...", rulesDigestStage},
	{"Discovery indexes (package map, docs-to-code fresh)...", discoveryIndexesStage},
	{"Problem journal (classes with 2+ rows)...", journalStage},
	{"Digest anchors (ai/digests/*.md file:line references resolve)...", digestStage},
}

func answerDocVerify(root string) (any, int) {
	var out textbuf.Buffer
	out.Str("Running documentation tests...\n")
	failed := false
	for _, stage := range docVerifyStages {
		out.Str("\n  -> ").Str(stage.heading).Byte('\n')
		payload, code := stage.run(root)
		out.Str(prose(payload))
		if code != 0 {
			failed = true
		}
	}
	out.Byte('\n')
	if failed {
		out.Str("Documentation tests FAILED -- see output above.\n").
			Str("See docs/contributing/documentation-testing.md for how to fix.\n")
		return docVerifyPage{text: out.String()}, 1
	}
	out.Str("Documentation tests PASSED\n")
	return docVerifyPage{text: out.String()}, 0
}

func docDriftStage(root string) (any, int) {
	report := docvalid.Drift(root)
	if len(report.Issues) > 0 {
		return report, 1
	}
	return report, 0
}

func docContractStage(root string) (any, int) {
	report, err := docvalid.Validate(root)
	if err != nil {
		return errorPage(err), 1
	}
	if !report.Valid {
		return report, 1
	}
	return report, 0
}

// docUsageStage refuses a description that prescribes a CLI spelling, and a
// deletion that hides a grammar the model still does not state.
func docUsageStage(root string) (any, int) {
	report, err := docvalid.Usage(root)
	if err != nil {
		return errorPage(err), 1
	}
	if !report.Valid {
		return report, 1
	}
	return report, 0
}

// docHelpShapeStage refuses a summary that no one-line surface can render, and
// a declaration the commit under test wrote with no long text beside it. It
// reports how much of each of the four corpora is written.
//
// The four are the command tree, the RPCs, the offline local registry and the
// CONFIG tree. The last one is the largest and was outside every command gate
// until 2026-09-03, which is how 640 config descriptions ran past the render
// bound with nothing saying so.
//
// The root is unused: the gate reads the YANG modules this binary carries,
// which is the same population every other command gate walks, and it reads
// HEAD through git for the one rule scoped to the commit under test.
func docHelpShapeStage(_ string) (any, int) {
	report, err := docvalid.HelpShape()
	if err != nil {
		return errorPage(err), 1
	}
	if !report.Valid {
		return report, 1
	}
	return report, 0
}

func docIndexStage(root string) (any, int) { return answerDocIndex(root) }

func rulesRenderStage(root string) (any, int) {
	report, err := rules.RenderAll(root,
		filepath.Join(root, "ai", "rules"), filepath.Join(root, "ai", "rules", "points"), true)
	if err != nil {
		return errorPage(err), 2
	}
	if report.Failed() {
		return report, 1
	}
	return report, 0
}

func rulesRoundTripStage(root string) (any, int) {
	out, err := os.MkdirTemp("", "ze-rules-points-")
	if err != nil {
		return errorPage(err), 2
	}
	report, runErr := rules.RoundTrip(filepath.Join(root, "ai", "rules"), out)
	cleanupErr := os.RemoveAll(out)
	if runErr != nil {
		if cleanupErr != nil {
			return errorPage(fmt.Errorf("%w; removing round-trip scratch: %w", runErr, cleanupErr)), 2
		}
		return errorPage(runErr), 2
	}
	if cleanupErr != nil {
		return errorPage(fmt.Errorf("removing round-trip scratch: %w", cleanupErr)), 2
	}
	if report.Failed() {
		return report, 1
	}
	return report, 0
}

func rulesCoverageStage(root string) (any, int) {
	report, err := rules.Coverage(root)
	if err != nil {
		return errorPage(err), 2
	}
	page := prose(report)
	if len(report.Diagnosis) > 0 {
		var tb textbuf.Buffer
		page = tb.Str(page).Join(report.Diagnosis, "\n").Byte('\n').String()
	}
	if report.Failed() {
		return docVerifyPage{text: page}, 1
	}
	return docVerifyPage{text: page}, 0
}

func rulesIndexStage(root string) (any, int) {
	report, err := rules.Index(root, true)
	if err != nil {
		return errorPage(err), 1
	}
	if report.Failed() {
		return report, 1
	}
	return report, 0
}

func rulesLintStage(root string) (any, int) {
	report, err := rules.Lint(root)
	if err != nil {
		return errorPage(err), 1
	}
	if report.Failed() {
		return report, 1
	}
	return report, 0
}

func rulesDigestStage(root string) (any, int) {
	report, err := rules.Digest(root, true)
	if err != nil {
		return errorPage(err), 1
	}
	if report.EmptyCorpus {
		var tb textbuf.Buffer
		page := tb.Str(prose(report)).
			Str("warning: the task corpus is empty, so no blocking rule can be shown ").
			Str("unreachable and ai/rules/CORE.md loses that derivation -- check that ").
			Str("plan/spec-*.md is readable\n").String()
		if report.Failed() {
			return docVerifyPage{text: page}, 1
		}
		return docVerifyPage{text: page}, 0
	}
	if report.Failed() {
		return report, 1
	}
	return report, 0
}

// discoveryIndexesStage reports the generated documentation indexes.
//
// The five generated RFC files were judged here until 2026-09-11, by comparing
// a fresh render against the copy in the tree. They are derived and untracked
// now (internal/le/rfc/register.go), so there is no committed copy to compare
// against and nothing to report: a write to a summary removes them and a read
// rebuilds them.
func discoveryIndexesStage(root string) (any, int) {
	var out textbuf.Buffer
	failed := false

	docsToCode, err := docstocode.Check(root)
	if err != nil {
		out.Str(prose(errorPage(err)))
		failed = true
	} else {
		out.Str(prose(docsToCode))
		failed = failed || docsToCode.Stale
	}

	if failed {
		return docVerifyPage{text: out.String()}, 1
	}
	return docVerifyPage{text: out.String()}, 0
}

func journalStage(root string) (any, int) {
	var stderr bytes.Buffer
	report, code := journal.Run(root, &stderr)
	var tb textbuf.Buffer
	return docVerifyPage{text: tb.Str(prose(report)).Str(stderr.String()).String()}, code
}

func digestStage(root string) (any, int) {
	report, err := digest.Check(root)
	if err != nil {
		return errorPage(err), 2
	}
	if len(report.Errors) > 0 {
		return docVerifyPage{text: report.Diagnosis()}, 1
	}
	return report, 0
}

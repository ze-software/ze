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

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/digest"
	"github.com/ze-software/ze/internal/le/discoveryindex"
	"github.com/ze-software/ze/internal/le/docstocode"
	"github.com/ze-software/ze/internal/le/docvalid"
	"github.com/ze-software/ze/internal/le/journal"
	"github.com/ze-software/ze/internal/le/rfc"
	"github.com/ze-software/ze/internal/le/rules"
)

type docVerifyPage struct {
	text string
}

func (p docVerifyPage) Text() string { return p.text }

type docVerifyStage struct {
	heading string
	run     func(root string) (any, int)
}

var docVerifyStages = [...]docVerifyStage{
	{"Documentation drift (docs claims vs registry, Makefile, filesystem)...", docDriftStage},
	{"YANG/handler contract (validate-commands)...", docContractStage},
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

func discoveryIndexesStage(root string) (any, int) {
	var out textbuf.Buffer
	failed := false

	packageMap, err := discoveryindex.Check(root)
	if err != nil {
		out.Str(prose(errorPage(err)))
		failed = true
	} else {
		out.Str(prose(packageMap))
		failed = failed || packageMap.Stale
	}

	docsToCode, err := docstocode.Check(root)
	if err != nil {
		out.Str(prose(errorPage(err)))
		failed = true
	} else {
		out.Str(prose(docsToCode))
		failed = failed || docsToCode.Stale
	}

	rfcPage, code := rfcFreshnessStage(root)
	out.Str(prose(rfcPage))
	failed = failed || code != 0
	if failed {
		return docVerifyPage{text: out.String()}, 1
	}
	return docVerifyPage{text: out.String()}, 0
}

func rfcFreshnessStage(root string) (any, int) {
	var tb textbuf.Buffer
	collected, err := rfc.Collect(root)
	if err != nil {
		return docVerifyPage{text: tb.Str("rfc-requirements: cannot run: ").
			Err(err).Byte('\n').String()}, 2
	}
	if len(collected.ParseErrors) > 0 {
		tb.Reset()
		for _, problem := range collected.ParseErrors {
			tb.Str("* ").Str(problem).Byte('\n')
		}
		tb.Str("rfc-requirements: cannot judge freshness: a summary did not parse, so its ").
			Str("requirements are absent from this render and every page deriving from them ").
			Str("would be reported wrongly. Fix the summary, then re-run\n")
		return docVerifyPage{text: tb.String()}, 2
	}

	input, err := rfc.NewRenderInput(root, collected, nil, nil)
	if err != nil {
		return docVerifyPage{text: tb.Reset().Str("rfc-requirements: cannot run: ").
			Err(err).Byte('\n').String()}, 2
	}
	var stale []string
	index, err := rfc.RenderIndex(input)
	if err != nil {
		return docVerifyPage{text: tb.Reset().Str("rfc-requirements: cannot run: ").
			Err(err).Byte('\n').String()}, 2
	}
	indexPath := filepath.Join(root, "ai", "RFC-REQUIREMENTS.md")
	current, err := os.ReadFile(indexPath) //nolint:gosec // generated page under the named checkout
	if err != nil && !os.IsNotExist(err) {
		return docVerifyPage{text: tb.Reset().Str("rfc-requirements: cannot run: ").
			Err(err).Byte('\n').String()}, 2
	}
	if string(current) != tb.Reset().Str(index).Byte('\n').Slice() {
		stale = append(stale,
			"ai/RFC-REQUIREMENTS.md is stale vs its sources -- run: make ze-rfc-index-update")
	}

	shards := rfc.RenderShards(input)
	keep := make(map[string]bool, len(shards))
	for _, stem := range rfc.ShardStems(collected.Requirements) {
		keep[stem] = true
		fileName := tb.Reset().Str(stem).Str(".md").String()
		rel := filepath.ToSlash(filepath.Join("rfc", "requirements", fileName))
		body, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) //nolint:gosec // generated page under the named checkout
		if os.IsNotExist(readErr) {
			stale = append(stale, tb.Reset().Str(rel).
				Str(" is missing -- run: make ze-rfc-index-update").String())
			continue
		}
		if readErr != nil {
			return docVerifyPage{text: tb.Reset().Str("rfc-requirements: cannot run: ").
				Err(readErr).Byte('\n').String()}, 2
		}
		if string(body) != tb.Reset().Str(shards[stem]).Byte('\n').Slice() {
			stale = append(stale, tb.Reset().Str(rel).
				Str(" is stale vs its sources -- run: make ze-rfc-index-update").String())
		}
	}
	prunable, err := rfc.PrunableShards(root, keep)
	if err != nil {
		return docVerifyPage{text: tb.Reset().Str("rfc-requirements: cannot run: ").
			Err(err).Byte('\n').String()}, 2
	}
	for _, stem := range prunable {
		stale = append(stale, tb.Reset().Str("rfc/requirements/").Str(stem).
			Str(".md renders no requirement section and the generator no longer owns it -- ").
			Str("run: make ze-rfc-index-update").String())
	}
	if len(stale) > 0 {
		tb.Reset()
		for _, problem := range stale {
			tb.Str("* ").Str(problem).Byte('\n')
		}
		return docVerifyPage{text: tb.String()}, 1
	}
	return docVerifyPage{text: tb.Reset().Str("ai/RFC-REQUIREMENTS.md and ").
		Int(int64(len(keep))).Str(" shard(s) up to date\n").String()}, 0
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

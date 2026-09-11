// Design: docs/contributing/rfc-conformance-gates.md -- the RFC gate over a tree that holds no generated page
//
// misc_fixture_runner_rfcledger.go drives the REAL `le` binary over a scratch
// checkout and judges the RFC gate's own answer.
//
// The five generated files (ai/RFC-REQUIREMENTS.md, rfc/requirements/,
// rfc/enrolled.txt, rfc/not-enrolled.txt, docs/features/rfc-status.md) are
// derived and untracked. This scenario is the loop a session actually runs: add
// a tagged test, then run the gate, and never regenerate anything. Until
// 2026-09-11 that reported four files stale and the session charged a debt row
// for each one.
//
// The tagged test this scenario adds is a .ci, and NOT Go. Both are tag
// carriers (rfc.IsTagCarrier), so either invalidates the family through the
// same hook. What separates them is that the posttool-writeedit chain also
// formats and LINTS an edited Go file (hookruntime.postFormatGo), and that run
// waits for golangci-lint's machine-wide lock inside a 60-second budget. Several
// sessions share this machine, so the wait is what a Go write really costs here,
// and the scenario reported that queue as its own verdict. A .ci write reaches
// the linter never.
//
// Related: register_le_rfc_ledger_is_derived.go -- the registration.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The scratch corpus: one enrolled summary gating one MUST, the RFC text the
// extraction walk reads, and the two manifests the tag scanner needs.
const (
	rfcLedgerSummaryRel = "rfc/short/rfc9999.md"
	rfcLedgerIndexRel   = "ai/RFC-REQUIREMENTS.md"
	rfcLedgerShardRel   = "rfc/requirements/rfc9999.md"
	rfcLedgerTestRel    = "test/runner/widget-is-sent.ci"

	// The one MUST the summary declares. Every step of the loop names it: the
	// gate reports it untested, the shard carries a row for it, and the tag the
	// write adds binds it to a test.
	rfcLedgerRequirement = "RFC9999-2-1"

	// The tagged test, in the shape the header explains: a .ci carrier, whose
	// tag binds the fixture RFC's one MUST to this scenario file.
	rfcLedgerTest = "# A speaker sends the widget.\n#\n" +
		"# RFC requirement: RFC9999-2-1 positive - the speaker sends the widget.\n\n" +
		"cmd=foreground:seq=1:exec=true\nexpect=exit:code=0\n"

	rfcLedgerSummary = "# RFC 9999\n\n## Meta\n\n| Field | Value |\n|-------|-------|\n" +
		"| Title | Widgets |\n| Enrolment | enrolled |\n" +
		"| Enrolment reason | the fixture RFC, gated so the gate has a population |\n" +
		"| Support | bgp-base 10 |\n| Support area | Widgets |\n" +
		"| Support status | Partial |\n| Support coverage | unit tests |\n" +
		"| Support remaining | Zero MUST gaps. |\n\n" +
		"## Compliance Checklist\n\n" +
		"- [ ] [RFC9999-2-1] [MUST] A speaker MUST send the widget (§2)\n"
)

// leRFCLedgerIsDerivedDriver walks the loop of user story 1: the gate answers
// from the tree, with no generated page in it, and a read rebuilds the family.
func leRFCLedgerIsDerivedDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("rfc-ledger fixture takes no arguments")
	}
	le, err := nativeLEBinary()
	if err != nil {
		return err
	}
	repo, err := os.MkdirTemp("", "ze-rfc-ledger-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(repo) //nolint:errcheck // fixture cleanup

	if err := gitFixture(ctx, repo, map[string]string{
		fileGoMod:                       "module fixture/rfcledger\n\ngo 1.24\n",
		fileFeatureGates:                contentFeatureGate,
		fileGitIgnore:                   contentGitIgnoreTmp,
		"ai/.keep":                      "",
		"docs/features/.keep":           "",
		rfcLedgerSummaryRel:             rfcLedgerSummary,
		"rfc/full/rfc9999.txt":          "A speaker MUST send the widget.\n",
		"rfc/drain-budget.txt":          "start 2026-07-29\nrate 0\n",
		".github/workflows/nightly.yml": "on:\n  schedule:\n    - cron: '0 3 * * *'\n",
	}); err != nil {
		return err
	}
	for _, rel := range rfcLedgerGeneratedPaths() {
		if _, statErr := os.Stat(filepath.Join(repo, filepath.FromSlash(rel))); !os.IsNotExist(statErr) {
			return fmt.Errorf("%s is in the fixture tree before anything rendered it", rel)
		}
	}
	fmt.Fprintln(os.Stdout, "rfc-fixture-ready") //nolint:errcheck // progress output

	// The gate, over a tree holding none of the five. It judges the summaries,
	// the tags and the audits; a file nobody rendered is not its business.
	report, _, err := rawCommand(ctx, repo, envRootedAt(repo), le, "rfc", "check")
	if err != nil {
		return fmt.Errorf("le rfc check did not run: %w\n%s", err, report)
	}
	if strings.Contains(report, "cannot run") {
		return fmt.Errorf("le rfc check refused the fixture tree:\n%s", report)
	}
	// The gate exits non-zero over this corpus, because the one MUST has no
	// test yet, so the exit code cannot tell a gate that JUDGED from a binary
	// that never started. What tells them apart is the verdict naming the
	// requirement the summary declares. Without this, every absence check below
	// passes over an empty report.
	if !strings.Contains(report, rfcLedgerRequirement) {
		return fmt.Errorf("le rfc check judged nothing: no verdict names %s:\n%s", rfcLedgerRequirement, report)
	}
	for _, rel := range rfcLedgerGeneratedPaths() {
		if strings.Contains(report, rel) {
			return fmt.Errorf("le rfc check judges %s, which is derived and untracked:\n%s", rel, report)
		}
	}
	fmt.Fprintln(os.Stdout, "gate-judged-the-tree-not-the-pages") //nolint:errcheck // progress output

	// The read half: a command naming one of the five builds all five first.
	code, said, err := derivedReadHook(ctx, repo, le, "grep -n RFC9999 "+rfcLedgerIndexRel)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("the read hook refused a grep of a registered artifact with %d:\n%s", code, said)
	}
	index, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(rfcLedgerIndexRel))) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("the read hook built no %s: %w", rfcLedgerIndexRel, err)
	}
	if !strings.Contains(string(index), "rfc9999") {
		return fmt.Errorf("the rebuilt index does not carry the fixture's RFC:\n%s", index)
	}
	// The index is the rollup; the requirement id itself lives in the shard.
	// One run writes both, so a present index with an absent shard would mean
	// the rebuild stopped half way.
	shard, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(rfcLedgerShardRel))) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("one run writes all five and %s is absent: %w", rfcLedgerShardRel, err)
	}
	if !strings.Contains(string(shard), rfcLedgerRequirement) {
		return fmt.Errorf("the rebuilt shard does not carry the fixture's requirement:\n%s", shard)
	}
	fmt.Fprintln(os.Stdout, "read-materialized-the-family") //nolint:errcheck // progress output

	// The write half: the tagged test a session adds removes every one of them,
	// including the shard DIRECTORY, which os.Remove alone could not take.
	tagged := filepath.Join(repo, filepath.FromSlash(rfcLedgerTestRel))
	if err := os.MkdirAll(filepath.Dir(tagged), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(tagged, []byte(rfcLedgerTest), 0o600); err != nil {
		return err
	}
	code, said, err = derivedWriteHook(ctx, repo, le, tagged)
	if err != nil {
		return err
	}
	if code > 1 {
		return fmt.Errorf("the write hook refused the tagged test with %d:\n%s", code, said)
	}
	for _, rel := range rfcLedgerGeneratedPaths() {
		if _, statErr := os.Stat(filepath.Join(repo, filepath.FromSlash(rel))); !os.IsNotExist(statErr) {
			return fmt.Errorf("%s survived the write of a test that carries an RFC tag", rel)
		}
	}
	fmt.Fprintln(os.Stdout, "tagged-test-removed-the-family") //nolint:errcheck // progress output

	// The loop closes here. The first read above built the family from a tree
	// with no tagged test in it, so the shard's Positive test cell read `--`.
	// This read runs after the write, and the cell it answers with is what says
	// the rebuild took the EDIT rather than a copy of its own earlier output.
	code, said, err = derivedReadHook(ctx, repo, le, "grep -n "+rfcLedgerRequirement+" "+rfcLedgerShardRel)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("the read hook refused a grep of the rebuilt shard with %d:\n%s", code, said)
	}
	rebuilt, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(rfcLedgerShardRel))) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("the read hook rebuilt no %s: %w", rfcLedgerShardRel, err)
	}
	if !strings.Contains(string(rebuilt), rfcLedgerTestRel) {
		return fmt.Errorf("the rebuilt shard does not name the test the write added:\n%s", rebuilt)
	}
	fmt.Fprintln(os.Stdout, "rebuild-bound-the-requirement-to-the-new-test") //nolint:errcheck // progress output
	return nil
}

// rfcLedgerGeneratedPaths is the family, in the order a reader meets it: the
// index, one shard, the shard directory, and the three ledger files.
func rfcLedgerGeneratedPaths() []string {
	return []string{rfcLedgerIndexRel, rfcLedgerShardRel, "rfc/requirements",
		"rfc/enrolled.txt", "rfc/not-enrolled.txt", "docs/features/rfc-status.md"}
}

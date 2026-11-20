// Design: docs/contributing/spec-workflow.md -- what Thomas's approval of a design binds
// Related: ../approve.go -- the approval record's writer, whose spelling SeedApproval copies
// Related: ../judge.go -- the judge record's writer, whose spelling SeedJudgment copies
// Related: ../away.go -- the away record's writer, whose spelling SeedAway copies
// Related: ../admission.go -- OwedApprovalTrailer, the away trailer CommitAwayAdmission commits

// Package designtest seeds the three records of `le spec design` into a
// throwaway checkout, and the commit that carries an away admission's
// Design-approved: line, for the tests of the gates that read them: Claim,
// the ready-edit hook, the commit trailer, the closure checks and the pending
// list. Thomas writes the real records from his terminal; a test cannot, so
// it writes their spelling here. TestSeededRecordsAreRead holds this copy to
// the readers (specdesign.Approved, ActiveAway, Admit), so the two cannot
// drift apart.
package designtest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	specdesign "github.com/ze-software/ze/internal/le/spec/design"
)

// gitCommandTimeout bounds each git call a fixture makes: the hook fixtures
// run them inside ./le ai hooks unit, which must not hang on a stuck git.
const gitCommandTimeout = 30 * time.Second

// The record directory and files, as approve.go, judge.go and away.go place
// them.
const (
	recordsDir  = "tmp/spec-design"
	approvedDir = recordsDir + "/approved"
	judgedDir   = recordsDir + "/judged"
	awayRecord  = recordsDir + "/away.md"
)

// SeedApproval writes Thomas's approval of the spec at relative as it holds
// text: the record ./le spec design approve writes, approved on date with his
// words, listing every departs row of the text.
func SeedApproval(root, relative, text, date, words string) error {
	stem := stemOf(relative)
	digest := specdesign.DesignDigest(text)
	departures := specdesign.Departures(text)
	lines := make([]string, 0, 6+len(departures))
	lines = append(lines,
		"Spec: "+stem+".md", "Digest: "+digest, "Date: "+date, "Machine: fixture-host",
		"Owner words: "+words, "Judge: "+digest+" "+date+" every verdict holds",
	)
	for _, departure := range departures {
		lines = append(lines, "Departure: "+departure.Page+"\t"+departure.Section+"\t"+departure.Statement)
	}
	return writeRecord(root, approvedDir+"/"+stem+".md", lines)
}

// SeedJudgment writes the judge's verdict, approve or refuse, on the spec at
// relative as it holds text: the record ./le spec design judge writes.
func SeedJudgment(root, relative, text, verdict string) error {
	stem := stemOf(relative)
	return writeRecord(root, judgedDir+"/"+stem+".md", []string{
		"Spec: " + stem + ".md", "Digest: " + specdesign.DesignDigest(text), "Verdict: " + verdict,
		"Sentence: every verdict holds", "Judge: judge-session+judge-agent", "Date: 2026-11-20",
	})
}

// SeedAway writes Thomas's absence from from to until, both YYYY-MM-DD: the
// record ./le spec design away writes.
func SeedAway(root, from, until string) error {
	return writeRecord(root, awayRecord, []string{"From: " + from, "Until: " + until, "Owner words: away for the fixture"})
}

// The clean fix spec: one code file, the design page its header names with
// one decision row, the record the row cites, the system door, and a ready
// spec under plan/release/fixes/ whose table and presentation owe nothing.
const (
	CleanSpec     = "plan/release/fixes/spec-widget.md"
	cleanCode     = "internal/component/widget/widget.go"
	cleanPage     = "docs/architecture/widget.md"
	cleanRecord   = "plan/records/widget.md"
	cleanDoor     = "ai/doors/system.md"
	cleanDecision = "The widget answers data and the surface renders it"
)

// CleanSpecText is the ready fix spec that passes ./le spec design check: a
// row for each page and decision its file owes, the five presentation
// subsections, and no departs row.
const CleanSpecText = "# Spec: widget\n\n| Field | Value |\n|-------|-------|\n| Status | ready |\n" +
	"| Priority | P2 |\n| Updated | 2026-11-19 |\n\n## Task\n\nRender the widget in the surface.\n\n" +
	"## Acceptance Criteria\n\n| AC ID | Input | Expected |\n|---|---|---|\n| AC-1 | a widget | the surface renders it |\n\n" +
	"## Files to Modify\n- `" + cleanCode + "` - render in the surface\n\n" +
	"## Design documents\n\n| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |\n" +
	"|---|---|---|---|---|---|\n" +
	"| `" + cleanPage + "` | Design decisions | Design header of `" + cleanCode + "` | \"" + cleanDecision + "\" | agrees | The surface renders |\n" +
	"| `" + cleanDoor + "` | System door | every change | - | silent | The door names no rule on rendering |\n\n" +
	"## Design presentation\n\n### What changes\n\nThe surface renders the widget.\n\n" +
	"### Before and after\n\nBefore: the widget renders text. After: the surface renders it.\n\n" +
	"### Alternatives set aside\n\nRendering in the widget, which the decision forbids.\n\n" +
	"### Documents departed from\n\nnone\n\n### Choices for Thomas\n\nNone.\n\n## Implementation Steps\n\n1. Render.\n"

// WriteCleanTree writes the files the clean fix spec's design reads under
// root, and the spec itself at relative with text: CleanSpec and
// CleanSpecText for the clean case, another path or text for a variant.
func WriteCleanTree(root, relative, text string) error {
	files := map[string]string{
		cleanCode: "// Design: " + cleanPage + " -- the widget\n\npackage widget\n",
		cleanPage: "# Widget\n\nHow the widget answers.\n\n## Design decisions\n\n" +
			"| Decision | It forbids | Source | Date |\n|---|---|---|---|\n" +
			"| " + cleanDecision + " | Rendering text in the widget | " + cleanRecord + " | 2026-08-21 |\n",
		cleanRecord: "# Widget record\n",
		cleanDoor:   "# System door\n\n<!-- door-base: pkg cmd/ze internal -->\n",
		relative:    text,
	}
	for name, body := range files {
		if err := writeFile(root, name, body); err != nil {
			return err
		}
	}
	return nil
}

// InitRepository makes root a git repository with no commit. The approval
// lookup reads the Design-approved: lines reachable from HEAD and refuses a
// history it cannot read (AC-26), so a fixture that must read as "no
// approval" needs a history that is empty rather than absent.
func InitRepository(root string) error {
	return runGit("init", "-q", root)
}

// CommitAwayAdmission commits the spec at relative, already written under
// root as text, with the Design-approved: line of its admission while Thomas
// was away from from to until, spelled by specdesign.FormatApprovalTrailer as
// OwedApprovalTrailer spells it: the history a commit made while he was away
// leaves behind. root MUST be a repository InitRepository made.
func CommitAwayAdmission(root, relative, text, from, until string) error {
	trailer := specdesign.FormatApprovalTrailer(specdesign.Approval{Stem: stemOf(relative),
		Digest: specdesign.DesignDigest(text), Away: true, From: from, Until: until})
	if err := Git(root, "add", "--", relative); err != nil {
		return err
	}
	return Git(root, "commit", "-q", "-m", "the spec admitted while away\n\n"+trailer)
}

// Git runs one git command in root as the fixture author, unsigned and
// bounded by gitCommandTimeout: the add and commit a test's own history needs.
func Git(root string, args ...string) error {
	full := append([]string{"-C", root, "-c", "user.email=fixture@example.invalid", "-c", "user.name=fixture",
		"-c", "commit.gpgsign=false"}, args...)
	return runGit(full...)
}

// runGit runs one git command of a fixture, bounded by gitCommandTimeout.
func runGit(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed binary, the fixture's own arguments
	if out, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return nil
}

// stemOf is the spec file name of relative without .md.
func stemOf(relative string) string {
	return strings.TrimSuffix(path.Base(filepath.ToSlash(relative)), ".md")
}

// writeRecord writes lines as one `Key: value` record at relative under root.
func writeRecord(root, relative string, lines []string) error {
	return writeFile(root, relative, strings.Join(lines, "\n")+"\n")
}

// writeFile writes body at relative under root, making its directories.
func writeFile(root, relative, body string) error {
	target := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}
	return os.WriteFile(target, []byte(body), 0o600)
}

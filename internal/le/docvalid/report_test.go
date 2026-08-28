package docvalid

import (
	"regexp"
	"strings"
	"testing"
)

// ansiSGR matches a color escape, so an assertion is about the words rather
// than the palette.
var ansiSGR = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiSGR.ReplaceAllString(s, "") }

// VALIDATES: the clean drift report renders the one line the script printed.
// PREVENTS: a reader, a rule file or a grep losing the sentence they key on.
func TestDriftTextRendersTheCleanLine(t *testing.T) {
	got := DriftReport{}.Text()
	if got != "No documentation drift detected.\n" {
		t.Fatalf("the clean report renders %q", got)
	}
}

// VALIDATES: a drift report renders the heading, one line per finding, the
// detail line, and the command to run.
// PREVENTS: a port that reports the same findings in words nobody recognizes.
func TestDriftTextRendersEveryFinding(t *testing.T) {
	report := DriftReport{Issues: []Issue{
		{File: "docs/DESIGN.md", Line: 12, Message: "claims 3 address families, registry has 4"},
		{File: "README.md", Line: 0, Message: "claims 9+ fuzz targets, actual is 3", Detail: "run the counter"},
	}}

	got := stripANSI(report.Text())
	want := "\n  Documentation drift detected (2 issues)\n\n" +
		"  x docs/DESIGN.md:12: claims 3 address families, registry has 4\n" +
		"  x README.md:0: claims 9+ fuzz targets, actual is 3\n" +
		"    -> run the counter\n" +
		"\n  Run: ./le docvalid doc-drift\n\n"
	if got != want {
		t.Fatalf("the drift report renders\n%q\nwant\n%q", got, want)
	}
}

// VALIDATES: the drift report colors the heading and the finding marker.
// PREVENTS: the palette silently disappearing, which is the one thing the
// stripped comparison against the script cannot see.
func TestDriftTextColorsTheFindings(t *testing.T) {
	got := DriftReport{Issues: []Issue{{File: "a.md", Line: 1, Message: "m", Detail: "d"}}}.Text()
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("the drift report is not colored: %q", got)
	}
	if strings.Contains(stripANSI(got), "\x1b[") {
		t.Fatalf("stripping left an escape behind: %q", got)
	}
}

// VALIDATES: a clean validation result renders the counts, the verdict and the
// full command table.
// PREVENTS: a native port that drops the table from the validation payload.
func TestValidationTextRendersACleanRun(t *testing.T) {
	result := ValidationResult{
		YANGCommands: []CommandEntry{{WireMethod: "ze-show:env-list", YANGPath: "show > env > list", Module: "ze-show-cmd"}},
		Total:        1, TotalHandlers: 2, TotalLocal: 3, Valid: true,
	}

	want := "# Command Validation\n\n" +
		"YANG commands: 1\n" +
		"Registered handlers: 2\n" +
		"Registered local handlers: 3\n" +
		"Skipped (editor-internal): 0\n\n" +
		"All commands validated.\n" +
		"\n## All YANG commands (1)\n\n" +
		"| WireMethod | YANG Path | Module |\n" +
		"|------------|-----------|--------|\n" +
		"| ze-show:env-list | show > env > list | ze-show-cmd |\n"
	if got := result.Text(); got != want {
		t.Fatalf("the validation result renders\n%q\nwant\n%q", got, want)
	}
}

// VALIDATES: every orphan section, the failure verdict, the skipped table and
// the loader warnings render in the script's order.
// PREVENTS: a section that stops being printed, which reads as a tree with no
// orphans of that kind.
func TestValidationTextRendersEverySection(t *testing.T) {
	result := ValidationResult{
		YANGCommands:        []CommandEntry{{WireMethod: "a:b", YANGPath: "a > b", Module: "m-cmd"}},
		OrphanYANG:          []CommandEntry{{WireMethod: "a:b", YANGPath: "a > b", Module: "m-cmd"}},
		OrphanHandlers:      []string{"c:d"},
		OrphanLocalHandlers: []string{"show thing"},
		SkippedHandlers:     []string{"ze-editor:mode-command", "ze-editor:mode-edit"},
		Total:               1, TotalHandlers: 1, TotalLocal: 1,
		Warnings: []string{"ze-ghost-cmd"},
	}

	want := "warning: module ze-ghost-cmd not found\n" +
		"# Command Validation\n\n" +
		"YANG commands: 1\n" +
		"Registered handlers: 1\n" +
		"Registered local handlers: 1\n" +
		"Skipped (editor-internal): 2\n\n" +
		"## YANG commands with no handler (1)\n\n" +
		"  a:b  (a > b in m-cmd)\n\n" +
		"## Handlers with no YANG command (1)\n\n" +
		"  c:d\n\n" +
		"## Local handlers with no YANG command (1)\n\n" +
		"  show thing\n\n" +
		"FAILED: 2 problem(s)\n" +
		"\n## All YANG commands (1)\n\n" +
		"| WireMethod | YANG Path | Module |\n" +
		"|------------|-----------|--------|\n" +
		"| a:b | a > b | m-cmd |\n" +
		"\n## Skipped handlers (editor-internal)\n\n" +
		"| WireMethod | Reason |\n" +
		"|------------|--------|\n" +
		"| ze-editor:mode-command | run -- editor mode switch |\n" +
		"| ze-editor:mode-edit | edit -- editor mode switch |\n"
	if got := result.Text(); got != want {
		t.Fatalf("the validation result renders\n%q\nwant\n%q", got, want)
	}
}

// VALIDATES: the writing action names the file it rewrote.
// PREVENTS: a silent write, which is the one kind a developer cannot check.
func TestWriteReportNamesTheFile(t *testing.T) {
	got := WriteReport{Path: "docs/features/pipe-operators.generated.md"}.Text()
	if got != "wrote docs/features/pipe-operators.generated.md\n" {
		t.Fatalf("the write report renders %q", got)
	}
}

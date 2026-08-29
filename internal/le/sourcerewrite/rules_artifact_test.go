// Related: rules.go -- skipReformat, internal/le/rules -- IsArtifact

package sourcerewrite

import (
	"os"
	"path/filepath"
	"testing"
)

// VALIDATES: a generated aggregate under ai/rules/ is outside the reformatter's
// population, whether or not any list names it.
// PREVENTS: `./le source-rewrite rules-reformat` rewriting the files the
// repository says never to edit by hand.
//
// The rewriter kept its own three-name skip list with no shape test, so on
// 2026-08-29 a dry run over the real tree answered "WOULD migrate
// ai/rules/CORE.md" and "WOULD migrate ai/rules/TRIGGERS.md" -- the only two
// files it would have touched, every real rule already conforming.
//
// NOVEL.md is in the fixture on purpose: it is named in no list anywhere, so it
// passes only through rules.IsArtifact's all-caps stem test. A fix that added
// TRIGGERS and CORE to the list would still fail here, which is the point.
func TestReformatSkipsGeneratedAggregates(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "ai", "rules")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}

	// Content a migration would rewrite, so a file that is not skipped shows up
	// in the report rather than passing as already conforming.
	migratable := []byte("# Aggregate\n\nplain directive\n")
	for _, name := range []string{"TRIGGERS.md", "CORE.md", "NOVEL.md"} {
		if err := os.WriteFile(filepath.Join(directory, name), migratable, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	report, err := reformatRules(root, true)
	if err != nil {
		t.Fatalf("reformatRules: %v", err)
	}
	if len(report.Files) != 0 {
		t.Errorf("the reformatter would rewrite generated aggregates: %v", report.Files)
	}
	if report.Changed != 0 {
		t.Errorf("changed = %d over a directory of generated aggregates alone", report.Changed)
	}
}

// VALIDATES: rule-format.md stays outside the population.
// PREVENTS: migrating the file that DEFINES the format this migration rewrites
// toward, which is circular.
//
// rules.IsArtifact does not skip it, and should not: it is a real rule. The
// exclusion is this rewriter's own, which is why skipReformat states it beside
// the shared predicate rather than inside it.
func TestReformatSkipsTheFormatSpecItself(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "ai", "rules")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "rule-format.md"), []byte("# Format\n\nplain directive\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := reformatRules(root, true)
	if err != nil {
		t.Fatalf("reformatRules: %v", err)
	}
	if len(report.Files) != 0 {
		t.Errorf("the reformatter would rewrite the format spec: %v", report.Files)
	}
}

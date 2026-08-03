package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: ai/rules/INDEX.md is regenerated and committed, and every rule has
// a derivable one-line summary.
// PREVENTS: a new or renamed rule landing without a discoverable trigger in the
// rule overview agents scan at session start.
func TestRulesIndexCheckPasses(t *testing.T) {
	out := runCommand(t, repoRoot(t), "python3", "scripts/dev/rules_index.py", "--check")
	mustContain(t, out, "up to date")
}

// isGeneratedDigest reports whether an ai/rules/ file is a generated digest rather
// than a rule, by the repository's own convention: an ALL-CAPS stem.
//
// It mirrors the classification in scripts/dev/rules_index.py. Keep the two in
// step; a name that only one of them treats as a digest is the drift this exists
// to remove.
func isGeneratedDigest(name string) bool {
	stem := strings.TrimSuffix(name, ".md")
	return stem != "" && stem == strings.ToUpper(stem)
}

// VALIDATES: the generated index references every ai/rules/*.md file.
// PREVENTS: the generator silently skipping rules so they never appear in the overview.
func TestRulesIndexCoversEveryRule(t *testing.T) {
	root := repoRoot(t)
	rulesDir := filepath.Join(root, "ai", "rules")
	indexBytes, err := os.ReadFile(filepath.Join(rulesDir, "INDEX.md"))
	if err != nil {
		t.Fatalf("read INDEX.md: %v", err)
	}
	index := string(indexBytes)

	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		t.Fatalf("read rules dir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		// A generated digest is not a rule. rules_index.py classifies one by its
		// ALL-CAPS stem -- INDEX.md, CONDENSED.md, TRIGGERS.md, CORE.md -- and its
		// comment records that the two-name list this mirrors is exactly what made
		// TRIGGERS.md and CORE.md land in the index by mistake. The same rule is
		// applied here rather than a second list of names, so the next digest needs
		// no edit in either place (ai/rules/derive-not-hardcode.md).
		if e.IsDir() || !strings.HasSuffix(name, ".md") || isGeneratedDigest(name) {
			continue
		}
		if !strings.Contains(index, "ai/rules/"+name) {
			t.Errorf("rule %s is not listed in ai/rules/INDEX.md", name)
		}
	}
}

// VALIDATES: the summariser derives a trigger from a `**When:**` line and flags a
// rule that has no derivable summary.
// PREVENTS: the drift gate passing on rules whose overview cell would be empty.
func TestRulesIndexFlagsMissingSummary(t *testing.T) {
	root := repoRoot(t)
	fixture := filepath.Join(root, "tmp", "rules-index-test-"+strings.ReplaceAll(t.Name(), "/", "-"))
	if err := os.RemoveAll(fixture); err != nil {
		t.Fatalf("clean fixture: %v", err)
	}
	if err := os.MkdirAll(fixture, 0o755); err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(fixture) })

	writeRule(t, fixture, "alpha.md", "# Alpha\n\n**When:** read for alpha things.\n\nbody\n")
	writeRule(t, fixture, "bravo.md", "# Bravo\n\nRationale: `somewhere.md`\n")

	script := "import sys, pathlib\n" +
		"sys.path.insert(0, 'scripts/dev')\n" +
		"import rules_index\n" +
		"content, missing = rules_index.build(pathlib.Path(sys.argv[1]))\n" +
		"print('MISSING:', ','.join(missing))\n" +
		"print('ALPHA_OK:', 'read for alpha things.' in content)\n"

	out := runCommand(t, root, "python3", "-c", script, fixture)
	mustContain(t, out, "MISSING: bravo.md")
	mustContain(t, out, "ALPHA_OK: True")
	if strings.Contains(out, "MISSING:") && strings.Contains(out, "alpha.md") {
		t.Errorf("alpha.md has a **When:** line and must not be flagged missing:\n%s", out)
	}
}

func writeRule(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write rule %s: %v", name, err)
	}
}

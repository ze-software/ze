package ai

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// VALIDATES: The skill mirror is generated from ONE source set, and every
// target is written. A missing source set produces an ERROR instead of a
// successful empty run.
// PREVENTS: Two regressions measured in internal/le/ai/actions.go on 2026-08-26.
// An empty ai/skills answers "synced 0 skill(s) + 0 agent(s) + CLAUDE.md +
// AGENTS.md" and exits 0. That answer names two files the script did not write.
// Also, any unrecognized argument enters the script's SYNC branch. Thus, a
// mistyped --check writes the tree instead of reading it. This occurs in a hook
// whose sole task is to read the tree (.claude/hooks/session-start.sh:135).

// fixture builds a checkout with two skills, one agent and an instructions
// file.
func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "ai/skills/ze-close.md", "---\nname: ze-close\n---\nsee .claude/rules/x.md\n")
	write(t, root, "ai/skills/ze-spec.md", "---\nname: ze-spec\n---\nbody\n")
	write(t, root, "ai/agents/ze-work.md", "---\nname: ze-work\n---\nagent\n")
	write(t, root, "ai/INSTRUCTIONS.md", "instructions for {{TOOL}}\n")
	return root
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(body)
}

// Every generated path, so a case can assert the ABSOLUTE number of files a
// sync wrote rather than that two runs wrote the same number.
func generatedPaths(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, tree := range []string{claudeSkills, codexSkills, agentsSkills, claudeAgents} {
		err := filepath.WalkDir(filepath.Join(root, tree), func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				// A tree that is not there yet contributes no path, which is
				// what a case asserting "nothing was written" reads.
				return nil //nolint:nilerr // an absent tree is an empty tree here
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			out = append(out, rel)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", tree, err)
		}
	}
	for _, name := range []string{claudeInstructions, codexInstructions} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func TestASyncWritesEveryMirrorOfEverySource(t *testing.T) {
	root := fixture(t)

	report, err := Mirror{Root: root}.Sync()
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(report.Skills) != 2 || len(report.Agents) != 1 {
		t.Fatalf("report names %d skills and %d agents, want 2 and 1", len(report.Skills), len(report.Agents))
	}

	want := []string{
		".agents/skills/ze-close/SKILL.md",
		".agents/skills/ze-spec/SKILL.md",
		".claude/agents/ze-work.md",
		".claude/skills/ze-close/SKILL.md",
		".claude/skills/ze-spec/SKILL.md",
		".codex/skills/ze-close/SKILL.md",
		".codex/skills/ze-spec/SKILL.md",
		"AGENTS.md",
		"CLAUDE.md",
	}
	got := generatedPaths(t, root)
	if len(got) != len(want) {
		t.Fatalf("%d generated files, want exactly %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("generated file %d is %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTheAgentsMirrorRepointsClaudePathsAndTheOthersDoNot(t *testing.T) {
	root := fixture(t)
	if _, err := (Mirror{Root: root}).Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if body := read(t, root, filepath.Join(".agents", "skills", "ze-close", "SKILL.md")); !strings.Contains(body, ".agents/rules/x.md") {
		t.Errorf("the .agents mirror did not repoint the .claude path: %q", body)
	}
	if body := read(t, root, filepath.Join(".claude", "skills", "ze-close", "SKILL.md")); !strings.Contains(body, ".claude/rules/x.md") {
		t.Errorf("the .claude mirror is not a verbatim copy: %q", body)
	}
	if body := read(t, root, filepath.Join(".codex", "skills", "ze-close", "SKILL.md")); !strings.Contains(body, ".claude/rules/x.md") {
		t.Errorf("the .codex mirror is not a verbatim copy: %q", body)
	}
}

func TestTheTwoInstructionFilesNameTheirOwnTool(t *testing.T) {
	root := fixture(t)
	if _, err := (Mirror{Root: root}).Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if body := read(t, root, claudeInstructions); body != "instructions for Claude\n" {
		t.Errorf("CLAUDE.md is %q", body)
	}
	if body := read(t, root, codexInstructions); body != "instructions for Codex\n" {
		t.Errorf("AGENTS.md is %q", body)
	}
}

// A source set that is not there is a caller in the wrong tree, not a tree with
// nothing to sync. The shell half reports success and names two files it never
// wrote.
func TestASyncWithNoSkillAtAllIsAnErrorRatherThanASuccessfulRunOverNothing(t *testing.T) {
	root := t.TempDir()
	write(t, root, "ai/INSTRUCTIONS.md", "x {{TOOL}}\n")

	if _, err := (Mirror{Root: root}).Sync(); err == nil {
		t.Fatal("a checkout holding no skill synced successfully")
	}
}

// The message the shell prints always ends "+ CLAUDE.md + AGENTS.md", whether
// or not ai/INSTRUCTIONS.md was there to generate them from.
func TestASyncWithoutTheInstructionsSourceIsAnError(t *testing.T) {
	root := t.TempDir()
	write(t, root, "ai/skills/x.md", "body\n")

	if _, err := (Mirror{Root: root}).Sync(); err == nil {
		t.Fatal("a checkout holding no instructions source synced successfully")
	}
}

func TestACheckOverAFreshlySyncedTreeIsClean(t *testing.T) {
	root := fixture(t)
	if _, err := (Mirror{Root: root}).Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	report, err := Mirror{Root: root}.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Fresh() {
		t.Errorf("a freshly synced tree reads stale: %v", report.Stale)
	}
}

func TestACheckNamesEveryKindOfDrift(t *testing.T) {
	root := fixture(t)
	if _, err := (Mirror{Root: root}).Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// An EDITED mirror, an ORPHAN mirror whose source is gone, and a MISSING
	// mirror. Each is a separate way for the tree to be wrong, and a check
	// that finds only the first passes two of them.
	write(t, root, ".claude/skills/ze-close/SKILL.md", "edited by hand\n")
	write(t, root, ".codex/skills/orphan/SKILL.md", "no source\n")
	if err := os.Remove(filepath.Join(root, ".agents", "skills", "ze-spec", "SKILL.md")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	report, err := Mirror{Root: root}.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	want := []string{
		".agents/skills/ze-spec/SKILL.md",
		".claude/skills/ze-close/SKILL.md",
		".codex/skills/orphan/SKILL.md",
	}
	if len(report.Stale) != len(want) {
		t.Fatalf("%d stale paths, want exactly %d: %v", len(report.Stale), len(want), report.Stale)
	}
	for i := range want {
		if report.Stale[i] != want[i] {
			t.Errorf("stale path %d is %q, want %q", i, report.Stale[i], want[i])
		}
	}
}

// The check must not touch the tree that it judges. A hook runs the check at
// every session start. Nobody can trust a check that writes to be read-only.
func TestACheckWritesNothing(t *testing.T) {
	root := fixture(t)
	if _, err := (Mirror{Root: root}).Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	before := generatedPaths(t, root)
	stamps := stamp(t, root, before)

	if _, err := (Mirror{Root: root}).Check(); err != nil {
		t.Fatalf("Check: %v", err)
	}

	after := generatedPaths(t, root)
	if len(after) != len(before) {
		t.Fatalf("the check changed the file count from %d to %d", len(before), len(after))
	}
	for path, when := range stamps {
		info, err := os.Stat(filepath.Join(root, path))
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if !info.ModTime().Equal(when) {
			t.Errorf("the check rewrote %s", path)
		}
	}
}

func stamp(t *testing.T, root string, paths []string) map[string]time.Time {
	t.Helper()
	out := make(map[string]time.Time, len(paths))
	for _, path := range paths {
		info, err := os.Stat(filepath.Join(root, path))
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		out[path] = info.ModTime()
	}
	return out
}

func TestAPreviewNamesEverySkillAndWritesNothing(t *testing.T) {
	root := fixture(t)

	report, err := Mirror{Root: root}.Preview()
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(report.Skills) != 2 {
		t.Fatalf("%d skills previewed, want exactly 2: %v", len(report.Skills), report.Skills)
	}
	if paths := generatedPaths(t, root); len(paths) != 0 {
		t.Errorf("the preview wrote %d files: %v", len(paths), paths)
	}
}

// A word this area does not hold must be refused. The shell's `case` has no
// default branch, so an unknown flag reaches the write.
func TestAnUnknownVerbIsRefusedRatherThanTreatedAsASync(t *testing.T) {
	payload, code := Answer([]string{"chekc"})
	if code == 0 {
		t.Errorf("an unknown verb answered code 0 with payload %v", payload)
	}
}

func TestEveryVerbOfTheAreaIsReachable(t *testing.T) {
	list := Actions()
	if len(list.Actions) != 3 {
		t.Fatalf("%d actions, want exactly 3: %v", len(list.Actions), list.Actions)
	}
	want := map[string]bool{"skills-sync": false, "sync-check": false, "sync-preview": false}
	for _, row := range list.Actions {
		if _, known := want[row.Verb]; !known {
			t.Errorf("unexpected verb %q", row.Verb)
			continue
		}
		want[row.Verb] = true
	}
	for verb, seen := range want {
		if !seen {
			t.Errorf("verb %q is not in the listing", verb)
		}
	}
}

package aisync

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// VALIDATES: The skill mirror is generated from ONE source set, and every
// target is written. A missing source set produces an ERROR instead of a
// successful empty run.
// PREVENTS: Two regressions measured in internal/le/ai/actions.go on 2026-08-26.
// An empty ai/skills answers "synced 0 skill(s) + 0 agent(s) + AGENTS.md"
// and exits 0. That answer names a file the script did not write.
// Also, any unrecognized argument enters the script's SYNC branch. Thus, a
// mistyped --check writes the tree instead of reading it. This occurs in a hook
// whose sole task is to read the tree (.claude/hooks/session-start.sh:135).

// The body lines of the fixture's skills. No stub holds either one, so a case
// can assert that no line of a body reached a generated file.
const (
	closeBody = "see .claude/rules/x.md"
	specBody  = "only the source holds this line"
)

// The fixture's frontmatter blocks, byte for byte.
const (
	closeFrontmatter = "---\nname: ze-close\ndescription: close a spec\n---\n"
	specFrontmatter  = "---\nname: ze-spec\ndescription: write a spec\n---\n"
)

// fixture builds a checkout with two skills, one agent and an instructions
// file.
func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "ai/skills/ze-close.md", closeFrontmatter+closeBody+"\n")
	write(t, root, "ai/skills/ze-spec.md", specFrontmatter+specBody+"\n")
	write(t, root, "ai/agents/ze-work.md", "---\nname: ze-work\n---\nagent\n")
	write(t, root, "ai/INSTRUCTIONS.md", "instructions for every agent\n")
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
	for _, name := range []string{agentsInstructions, shadowInstructions} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			out = append(out, name)
		}
	}
	slices.Sort(out)
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

// syncTree runs one sync over root and fails the test on an error.
func syncTree(t *testing.T, root string) Report {
	t.Helper()
	report, err := Mirror{Root: root}.Sync()
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	return report
}

// stubPaths answers the three generated paths of one skill.
func stubPaths(name string) []string {
	return []string{
		filepath.Join(claudeSkills, name, skillFile),
		filepath.Join(codexSkills, name, skillFile),
		filepath.Join(agentsSkills, name, skillFile),
	}
}

// VALIDATES: No harness gets text of its own, so the three files of a skill
// hold the same bytes (AC-2). It takes the place of the .agents path rewrite,
// which is deleted. read fails the test on a missing file, and the content
// must be non-empty, so three absent or empty files cannot pass.
func TestTheThreeFilesOfASkillAreByteIdentical(t *testing.T) {
	root := fixture(t)
	syncTree(t, root)

	for _, name := range []string{"ze-close", "ze-spec"} {
		paths := stubPaths(name)
		first := read(t, root, paths[0])
		if first == "" {
			t.Fatalf("%s is empty", paths[0])
		}
		for _, path := range paths[1:] {
			if body := read(t, root, path); body != first {
				t.Errorf("%s differs from %s:\n%q\n%q", path, paths[0], body, first)
			}
		}
	}
}

// VALIDATES: A stub opens with its source's frontmatter block, byte for byte,
// in all three trees (AC-1). A harness lists the skill by it.
func TestAStubCarriesTheSourcesFrontmatterByteForByte(t *testing.T) {
	root := fixture(t)
	syncTree(t, root)

	for name, block := range map[string]string{"ze-close": closeFrontmatter, "ze-spec": specFrontmatter} {
		for _, path := range stubPaths(name) {
			if body := read(t, root, path); !strings.HasPrefix(body, block) {
				t.Errorf("%s does not open with its source's frontmatter %q:\n%s", path, block, body)
			}
		}
	}
}

// VALIDATES: No line of a skill's body reaches any of its three files (AC-1),
// because a copied line goes stale when the source changes.
func TestAStubCarriesNoLineOfTheSourcesBody(t *testing.T) {
	root := fixture(t)
	syncTree(t, root)

	for name, line := range map[string]string{"ze-close": closeBody, "ze-spec": specBody} {
		for _, path := range stubPaths(name) {
			if body := read(t, root, path); strings.Contains(body, line) {
				t.Errorf("%s carries the body line %q:\n%s", path, line, body)
			}
		}
	}
}

// VALIDATES: The stub's marker is the first line after the frontmatter, and
// its instruction gives the five orders of AC-9. The stub holds no argument
// placeholder, even for a source that uses one, because a harness would
// substitute it in the stub's prose.
func TestTheStubOrdersAFullReadOfItsSourceAndHoldsNoArgumentPlaceholder(t *testing.T) {
	root := fixture(t)
	const argsFrontmatter = "---\nname: ze-args\ndescription: take arguments\n---\n"
	write(t, root, "ai/skills/ze-args.md", argsFrontmatter+"work on $ARGUMENTS\n")
	syncTree(t, root)

	body := read(t, root, filepath.Join(claudeSkills, "ze-args", skillFile))
	after, opened := strings.CutPrefix(body, argsFrontmatter)
	if !opened {
		t.Fatalf("the stub does not open with its frontmatter:\n%s", body)
	}
	marker, _, _ := strings.Cut(after, "\n")
	wantMarker := "<!-- Generated by ./le ai sync update from ai/skills/ze-args.md. Do not edit this file: edit that one. -->"
	if marker != wantMarker {
		t.Errorf("the first line after the frontmatter is %q, want %q", marker, wantMarker)
	}
	for _, order := range []string{
		"Read `ai/skills/ze-args.md` in full now, in one read with no offset and no limit.",
		"relative to the repository root",
		"Follow that file as this skill's instructions.",
		"The argument placeholder in that file stands for the arguments given to this skill.",
		"Read that file again after a context compaction",
		"If that file cannot be read, stop and say so. Do not work from this skill's description.",
	} {
		if !strings.Contains(body, order) {
			t.Errorf("the stub lacks the order %q:\n%s", order, body)
		}
	}
	if strings.Contains(body, "$ARGUMENTS") {
		t.Errorf("the stub holds the argument placeholder:\n%s", body)
	}
}

// VALIDATES: The frontmatter is the one copied part, so an edited description
// names that skill's three files stale, and a sync clears them (AC-4).
func TestAChangedDescriptionIsStaleUntilSynced(t *testing.T) {
	root := fixture(t)
	syncTree(t, root)
	write(t, root, "ai/skills/ze-spec.md", "---\nname: ze-spec\ndescription: write a spec, better\n---\n"+specBody+"\n")

	report, err := Mirror{Root: root}.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	want := stubPaths("ze-spec")
	slices.Sort(want)
	if !slices.Equal(report.Stale, want) {
		t.Errorf("stale paths are %v, want %v", report.Stale, want)
	}

	syncTree(t, root)
	report, err = Mirror{Root: root}.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Fresh() {
		t.Errorf("the tree is still stale after a sync: %v", report.Stale)
	}
}

// VALIDATES: A source that cannot make a stub refuses the sync and the check,
// with a message naming the file and ending in the fix (AC-5). The bad source
// sorts after both valid ones, and no generated file exists afterwards, so an
// implementation that writes as it goes is red.
func TestASourceThatCannotMakeAStubIsRefused(t *testing.T) {
	cases := []struct {
		defect string
		source string
		fix    string
	}{
		{"no frontmatter block", "just a body\n", "name: ze-zz"},
		{"an unclosed block", "---\nname: ze-zz\ndescription: d\n", "name: ze-zz"},
		{"no name", "---\ndescription: d\n---\nbody\n", "name: ze-zz"},
		{"no description", "---\nname: ze-zz\n---\nbody\n", "description:"},
		{"an empty description", "---\nname: ze-zz\ndescription:\n---\nbody\n", "description:"},
		{"a name that is not the stem", "---\nname: other\ndescription: d\n---\nbody\n", "name: ze-zz"},
	}
	for _, tc := range cases {
		t.Run(tc.defect, func(t *testing.T) {
			root := fixture(t)
			write(t, root, "ai/skills/ze-zz.md", tc.source)

			for mode, run := range map[string]func() (Report, error){
				"sync":  Mirror{Root: root}.Sync,
				"check": Mirror{Root: root}.Check,
			} {
				_, err := run()
				if err == nil {
					t.Fatalf("the %s accepted a source with %s", mode, tc.defect)
				}
				if !errors.Is(err, ErrBadSkill) {
					t.Errorf("the %s refused with %v, want ErrBadSkill", mode, err)
				}
				message := err.Error()
				if !strings.Contains(message, "ai/skills/ze-zz.md") {
					t.Errorf("the refusal does not name the file: %s", message)
				}
				lines := strings.Split(message, "\n")
				if last := lines[len(lines)-1]; !strings.Contains(last, tc.fix) {
					t.Errorf("the refusal's last line %q does not give the fix %q", last, tc.fix)
				}
			}
			if paths := generatedPaths(t, root); len(paths) != 0 {
				t.Errorf("a refused run wrote %d files: %v", len(paths), paths)
			}
		})
	}
}

// VALIDATES: A stub whose source is gone is removed by a sync, with its
// emptied directory, and the answer names each removed path; the other
// skill's files stay (AC-6).
func TestASyncRemovesTheStubOfADeletedSkill(t *testing.T) {
	root := fixture(t)
	syncTree(t, root)
	if err := os.Remove(filepath.Join(root, skillSources, "ze-spec.md")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	report := syncTree(t, root)
	want := stubPaths("ze-spec")
	slices.Sort(want)
	if !slices.Equal(report.Removed, want) {
		t.Errorf("removed is %v, want %v", report.Removed, want)
	}
	if len(report.Left) != 0 {
		t.Errorf("left is %v, want none", report.Left)
	}
	for _, tree := range skillTrees {
		if _, err := os.Lstat(filepath.Join(root, tree, "ze-spec")); !os.IsNotExist(err) {
			t.Errorf("the emptied directory %s/ze-spec survived: %v", tree, err)
		}
	}
	for _, path := range stubPaths("ze-close") {
		read(t, root, path)
	}
}

// VALIDATES: The check names a leftover stub stale until a sync removes it,
// then answers clean (AC-6).
func TestACheckIsCleanOnceTheLeftoverStubIsRemoved(t *testing.T) {
	root := fixture(t)
	syncTree(t, root)
	if err := os.Remove(filepath.Join(root, skillSources, "ze-spec.md")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	report, err := Mirror{Root: root}.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	want := stubPaths("ze-spec")
	slices.Sort(want)
	if !slices.Equal(report.Stale, want) {
		t.Errorf("stale before the sync is %v, want %v", report.Stale, want)
	}

	syncTree(t, root)
	report, err = Mirror{Root: root}.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Fresh() {
		t.Errorf("stale after the sync: %v", report.Stale)
	}
}

// VALIDATES: A file in a skill tree that le cannot show it wrote is left byte
// for byte, and the sync's answer names it with the reason and the fix (AC-7).
func TestASyncNamesAFileItLeftInASkillTree(t *testing.T) {
	cases := []struct {
		kind string
		path string
		body string
	}{
		{"an unmarked SKILL.md with no source", ".claude/skills/hand/SKILL.md",
			"---\nname: hand\ndescription: mine\n---\nmy own skill\n"},
		{"a stub whose marker line was edited", ".codex/skills/gone/SKILL.md",
			"---\nname: gone\ndescription: d\n---\n<!-- Generated by ./le ai sync update from ai/skills/gone.md. Please do not edit this file: edit that one. -->\n"},
		{"a stub carrying another skill's marker", ".agents/skills/other/SKILL.md",
			"---\nname: other\ndescription: d\n---\n<!-- Generated by ./le ai sync update from ai/skills/gone.md. Do not edit this file: edit that one. -->\n"},
		{"a sibling file", ".agents/skills/ze-close/notes.md", "notes\n"},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			root := fixture(t)
			syncTree(t, root)
			write(t, root, tc.path, tc.body)

			report := syncTree(t, root)
			if body := read(t, root, tc.path); body != tc.body {
				t.Errorf("the sync changed %s to %q", tc.path, body)
			}
			if !slices.Equal(report.Left, []string{tc.path}) {
				t.Errorf("left is %v, want [%s]", report.Left, tc.path)
			}
			if len(report.Removed) != 0 {
				t.Errorf("removed is %v, want none", report.Removed)
			}
			text := report.Text()
			for _, want := range []string{tc.path, "not generated by le", "delete it: rm " + tc.path} {
				if !strings.Contains(text, want) {
					t.Errorf("the answer lacks %q:\n%s", want, text)
				}
			}
		})
	}
}

// VALIDATES: The marker counts only as the first line after the frontmatter.
// A hand-written file that quotes it further down is not a leftover stub.
func TestAMarkerElsewhereInTheFileDoesNotMakeItALeftoverStub(t *testing.T) {
	root := fixture(t)
	syncTree(t, root)
	const path = ".claude/skills/hand/SKILL.md"
	body := "---\nname: hand\ndescription: mine\n---\nmy notes\n" +
		"<!-- Generated by ./le ai sync update from ai/skills/hand.md. Do not edit this file: edit that one. -->\n"
	write(t, root, path, body)

	report := syncTree(t, root)
	if got := read(t, root, path); got != body {
		t.Errorf("the sync changed %s to %q", path, got)
	}
	if !slices.Equal(report.Left, []string{path}) {
		t.Errorf("left is %v, want [%s]", report.Left, path)
	}
}

// VALIDATES: A subagent definition whose source is gone is never removed: the
// sync names it as left and the check still calls it stale (AC-12).
func TestASyncNamesALeftoverAgentDefinitionAndTheCheckStillCallsItStale(t *testing.T) {
	root := fixture(t)
	syncTree(t, root)
	if err := os.Remove(filepath.Join(root, agentSources, "ze-work.md")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	const path = ".claude/agents/ze-work.md"

	report := syncTree(t, root)
	if body := read(t, root, path); body != "---\nname: ze-work\n---\nagent\n" {
		t.Errorf("the sync changed %s to %q", path, body)
	}
	if !slices.Equal(report.Left, []string{path}) {
		t.Errorf("left is %v, want [%s]", report.Left, path)
	}

	checked, err := Mirror{Root: root}.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !slices.Contains(checked.Stale, path) {
		t.Errorf("the check does not call %s stale: %v", path, checked.Stale)
	}
}

// VALIDATES: A sync's answer carries the removed and the left paths in its
// text and its JSON, and the JSON omits both keys when they are empty.
func TestTheSyncReportNamesWhatItRemovedAndWhatItLeft(t *testing.T) {
	full := Report{Mode: modeSync, Skills: []string{"a"}, Agents: []string{},
		Removed: []string{".claude/skills/gone/SKILL.md"}, Left: []string{".codex/skills/hand/SKILL.md"}}
	text := full.Text()
	for _, want := range []string{
		"removed: .claude/skills/gone/SKILL.md\n",
		"left: .codex/skills/hand/SKILL.md was not generated by le, so the sync left it in place\n",
		"if it is a leftover, delete it: rm .codex/skills/hand/SKILL.md\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the text lacks %q:\n%s", want, text)
		}
	}
	encoded, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"removed":[".claude/skills/gone/SKILL.md"]`, `"left":[".codex/skills/hand/SKILL.md"]`} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("the JSON lacks %s: %s", want, encoded)
		}
	}

	empty := Report{Mode: modeSync, Skills: []string{"a"}, Agents: []string{}}
	encoded, err = json.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{`"removed"`, `"left"`} {
		if strings.Contains(string(encoded), key) {
			t.Errorf("the JSON of a sync with nothing removed or left carries %s: %s", key, encoded)
		}
	}
	if text := empty.Text(); strings.Contains(text, "removed:") || strings.Contains(text, "left:") {
		t.Errorf("the text of a sync with nothing removed or left names some: %s", text)
	}
}

// PRESERVES: A sync deletes nothing it cannot show it wrote, in any of the
// four trees. Green before the stub change, and kept green by it.
func TestASyncLeavesAFileItDidNotGenerateByteForByte(t *testing.T) {
	root := fixture(t)
	syncTree(t, root)
	extras := map[string]string{
		".claude/skills/ze-close/notes.md": "notes\n",
		".codex/skills/hand/SKILL.md":      "---\nname: hand\ndescription: mine\n---\nmy own skill\n",
		".agents/skills/deep/nested/x.md":  "nested\n",
		".claude/agents/hand.md":           "---\nname: hand\n---\nmine\n",
	}
	for path, body := range extras {
		write(t, root, path, body)
	}

	syncTree(t, root)
	for path, body := range extras {
		if got := read(t, root, path); got != body {
			t.Errorf("the sync changed %s to %q", path, got)
		}
	}
}

// VALIDATES: A SKILL.md that is a symlink to its source is replaced by a
// regular file holding the stub, and the source it pointed at keeps its
// bytes. os.WriteFile follows a symlink, so writing through it would
// overwrite the source with the stub.
func TestASyncReplacesASymlinkedSkillFileAndLeavesItsTargetAlone(t *testing.T) {
	root := fixture(t)
	source := filepath.Join(root, skillSources, "ze-close.md")
	const path = ".claude/skills/ze-close/SKILL.md"
	link := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(link), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(source, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	syncTree(t, root)
	if body := read(t, root, filepath.Join(skillSources, "ze-close.md")); body != closeFrontmatter+closeBody+"\n" {
		t.Errorf("the sync changed the source through the symlink to %q", body)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Errorf("%s is %v after the sync, want a regular file", path, info.Mode())
	}
	got := read(t, root, path)
	if want := read(t, root, ".codex/skills/ze-close/SKILL.md"); got != want {
		t.Errorf("%s holds %q, want the stub %q", path, got, want)
	}
	if !strings.Contains(got, "ai/skills/ze-close.md") {
		t.Errorf("%s does not hold the stub naming its source: %q", path, got)
	}
}

// PRESERVES: Whatever sits at a current source's path is overwritten by what
// the generation writes there (AC-8), so a pre-change full copy is replaced.
func TestASyncOverwritesAnUnmarkedFileAtASourcesPath(t *testing.T) {
	root := fixture(t)
	syncTree(t, root)
	const path = ".claude/skills/ze-close/SKILL.md"
	write(t, root, path, "an old full copy\n")

	syncTree(t, root)
	got := read(t, root, path)
	if got == "an old full copy\n" {
		t.Fatalf("the sync left the unmarked file at %s", path)
	}
	if want := read(t, root, ".codex/skills/ze-close/SKILL.md"); got != want {
		t.Errorf("%s holds %q, want what the generation wrote, %q", path, got, want)
	}
}

func TestAgentsMDIsAVerbatimCopyOfTheInstructions(t *testing.T) {
	root := fixture(t)
	if _, err := (Mirror{Root: root}).Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if body := read(t, root, agentsInstructions); body != "instructions for every agent\n" {
		t.Errorf("AGENTS.md is %q", body)
	}
}

// VALIDATES: A root CLAUDE.md is removed by a sync and named by a check.
// PREVENTS: Claude Code reads AGENTS.md only when no CLAUDE.md exists, so a
// leftover CLAUDE.md from the earlier two-file layout silently replaces the
// current rules with a stale copy.
func TestALeftoverClaudeMDIsRemovedBySyncAndNamedByCheck(t *testing.T) {
	root := fixture(t)
	if _, err := (Mirror{Root: root}).Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	write(t, root, shadowInstructions, "old rules\n")

	report, err := Mirror{Root: root}.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !slices.Equal(report.Stale, []string{shadowInstructions}) {
		t.Errorf("stale paths are %v, want only %s", report.Stale, shadowInstructions)
	}

	if _, err := (Mirror{Root: root}).Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, shadowInstructions)); !os.IsNotExist(err) {
		t.Errorf("CLAUDE.md survived a sync: %v", err)
	}
}

// A source set that is not there is a caller in the wrong tree, not a tree with
// nothing to sync. The shell half reports success and names two files it never
// wrote.
func TestASyncWithNoSkillAtAllIsAnErrorRatherThanASuccessfulRunOverNothing(t *testing.T) {
	root := t.TempDir()
	write(t, root, "ai/INSTRUCTIONS.md", "x\n")

	if _, err := (Mirror{Root: root}).Sync(); err == nil {
		t.Fatal("a checkout holding no skill synced successfully")
	}
}

// The message the shell prints always ends "+ AGENTS.md", whether or not
// ai/INSTRUCTIONS.md was there to generate it from.
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
	want := map[string]bool{"update": false, "check": false, "preview": false}
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

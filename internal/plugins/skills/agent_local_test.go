// Design: docs/features/ai-first.md — setting an operator's agent up to use Ze
// Related: agent_skill.go — the plan producer and the shell route under test
// Related: agent_test.go — the daemon route, from the external test package
//
// These tests sit in the package itself because they call the shell route's
// producer, localAnswer, and read its rows as values. agent_test.go is an
// external package, because it links plugin/all, which imports this plugin.

package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/skillstub"
)

// isolatedHome points HOME, CLAUDE_CONFIG_DIR and CODEX_HOME at a directory of
// the test's own, creates the configuration directories named in harnesses
// (home-relative), and answers the home. Nothing a test runs can reach the
// home of whoever runs it.
func isolatedHome(t *testing.T, harnessDirs ...string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	for _, dir := range harnessDirs {
		require.NoError(t, os.MkdirAll(filepath.Join(home, filepath.FromSlash(dir)), 0o700))
	}
	return home
}

// runSetup runs one of the three commands' shell route through the model's
// validator, as `ze <command>` does, and answers its rows.
func runSetup(t *testing.T, verb setupVerb, path string, tokens ...string) agentSkillAnswer {
	t.Helper()
	validated, err := command.ValidateModelArgs(path, tokens, nil)
	require.NoError(t, err, "the model refused %s %q", path, tokens)
	result := localAnswer(verb, path, validated)
	answer, isAnswer := result.Payload.(agentSkillAnswer)
	require.True(t, isAnswer, "%s %q answered no rows: %s", path, tokens, result.Diagnostic)
	return answer
}

// rowOf answers the row of one harness and item.
func rowOf(t *testing.T, answer agentSkillAnswer, agent, item string) setupRow {
	t.Helper()
	for _, row := range answer.Setup.Rows {
		if row.Agent == agent && row.Item == item {
			return row
		}
	}
	require.Failf(t, "no row", "the answer has no %s %s row: %+v", agent, item, answer.Setup.Rows)
	return setupRow{}
}

// claudeStub and sharedStub answer the two stub files under home.
func claudeStub(home string) string {
	return filepath.Join(home, ".claude", "skills", "ze", "SKILL.md")
}
func sharedStub(home string) string {
	return filepath.Join(home, ".agents", "skills", "ze", "SKILL.md")
}

// writeFile writes data at path, creating its directory.
func writeFile(t *testing.T, path, data string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
}

// readFile answers the bytes at path.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // a path the test built
	require.NoError(t, err)
	return string(data)
}

// TestCreateAgentSkillWritesTheStubForEachHarnessFound proves AC-1.
//
// Goal: with Claude Code and Codex found, create writes Claude Code's own
// stub and the shared one Codex reads, answers both rows created with the
// absolute path, and leaves another skill in the same directory. Method: a
// home holding both configuration directories and a sibling skill, the shell
// route, then the files and the rows.
func TestCreateAgentSkillWritesTheStubForEachHarnessFound(t *testing.T) {
	home := isolatedHome(t, ".claude", ".codex")
	sibling := filepath.Join(home, ".claude", "skills", "other", "SKILL.md")
	writeFile(t, sibling, "another skill\n")

	answer := runSetup(t, verbCreate, pathCreateAgentSkill)

	assert.FileExists(t, claudeStub(home))
	assert.FileExists(t, sharedStub(home))
	claude := rowOf(t, answer, "claude", itemSkill)
	assert.Equal(t, stateCreated, claude.State)
	assert.Equal(t, claudeStub(home), claude.Path)
	codex := rowOf(t, answer, "codex", itemSkill)
	assert.Equal(t, stateCreated, codex.State)
	assert.Equal(t, sharedStub(home), codex.Path)
	assert.Equal(t, "another skill\n", readFile(t, sibling), "another skill was changed")
}

// TestTheStubCarriesTheZeGuidesNameAndDescriptionAndTheMarker proves the
// stub's text, AC-1.
//
// Goal: the frontmatter is the ze inventory row's name and description, the
// marker is the first line after it, and the body orders `ze show skill ze`.
// Method: create in a home with Claude Code found, then read the file.
func TestTheStubCarriesTheZeGuidesNameAndDescriptionAndTheMarker(t *testing.T) {
	home := isolatedHome(t, ".claude")
	runSetup(t, verbCreate, pathCreateAgentSkill)

	block, rest, found := skillstub.Frontmatter([]byte(readFile(t, claudeStub(home))))
	require.True(t, found, "the stub opens with no frontmatter")
	guide := zeGuide()
	assert.Contains(t, string(block), "name: "+guide.Name+"\n")
	assert.Contains(t, string(block), "description: "+guide.Description+"\n")
	first, body, _ := strings.Cut(string(rest), "\n")
	assert.Equal(t, stubMarker, first, "the marker is not the first line after the frontmatter")
	assert.Contains(t, body, "ze show skill ze")
}

// TestASecondCreateLeavesABackdatedStubUntouched proves AC-2.
//
// Goal: a second create of a current stub answers unchanged and does not
// rewrite it. Method: create, set the stub's modification time back an hour,
// create again, then compare the bytes and the time. The time set back
// cannot be matched by a rewrite, whatever the clock's resolution.
func TestASecondCreateLeavesABackdatedStubUntouched(t *testing.T) {
	home := isolatedHome(t, ".claude")
	runSetup(t, verbCreate, pathCreateAgentSkill)
	before := readFile(t, claudeStub(home))
	backdated := time.Now().Add(-time.Hour).Truncate(time.Second)
	require.NoError(t, os.Chtimes(claudeStub(home), backdated, backdated))

	answer := runSetup(t, verbCreate, pathCreateAgentSkill)

	assert.Equal(t, stateUnchanged, rowOf(t, answer, "claude", itemSkill).State)
	assert.Equal(t, before, readFile(t, claudeStub(home)))
	info, err := os.Stat(claudeStub(home))
	require.NoError(t, err)
	assert.True(t, info.ModTime().Equal(backdated), "the stub was rewritten: %s", info.ModTime())
}

// outdatedStub answers a stub carrying Ze's marker and another body.
func outdatedStub() string {
	return "---\nname: ze\ndescription: an older ze\n---\n" + stubMarker + "\nRun something else.\n"
}

// TestShowAgentSkillNamesAnOutdatedStub proves AC-3, show half.
//
// Goal: a stub with Ze's marker and other bytes is reported outdated, and is
// left. Method: write such a stub, run show, read the row and the file.
func TestShowAgentSkillNamesAnOutdatedStub(t *testing.T) {
	home := isolatedHome(t, ".claude")
	writeFile(t, claudeStub(home), outdatedStub())

	answer := runSetup(t, verbShow, pathShowAgentSkill, "agent", "claude")

	assert.Equal(t, stateOutdated, rowOf(t, answer, "claude", itemSkill).State)
	assert.Equal(t, outdatedStub(), readFile(t, claudeStub(home)))
}

// TestCreateRefreshesAnOutdatedStub proves AC-3, create half.
//
// Goal: create rewrites an outdated stub of Ze's and answers updated. Method:
// write such a stub, run create, compare the file with this binary's stub.
func TestCreateRefreshesAnOutdatedStub(t *testing.T) {
	home := isolatedHome(t, ".claude")
	writeFile(t, claudeStub(home), outdatedStub())

	answer := runSetup(t, verbCreate, pathCreateAgentSkill, "agent", "claude")

	assert.Equal(t, stateUpdated, rowOf(t, answer, "claude", itemSkill).State)
	assert.Equal(t, stubText(formLocal), readFile(t, claudeStub(home)))
}

// TestAForeignZeSkillIsLeftByCreateShowAndDelete proves AC-4, one row per
// command.
//
// Goal: a SKILL.md without Ze's marker at the stub's place is answered
// foreign and left byte for byte by every command, and create says how to
// replace it. Method: write such a file, run the command, read the row and
// the file.
func TestAForeignZeSkillIsLeftByCreateShowAndDelete(t *testing.T) {
	const foreign = "---\nname: ze\ndescription: the operator's own\n---\nMy own notes.\n"
	cases := []struct {
		verb   setupVerb
		path   string
		tokens []string
	}{
		{verbCreate, pathCreateAgentSkill, []string{"agent", "claude"}},
		{verbShow, pathShowAgentSkill, []string{"agent", "claude"}},
		{verbDelete, pathDeleteAgentSkill, []string{"agent", "claude"}},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			home := isolatedHome(t, ".claude")
			writeFile(t, claudeStub(home), foreign)

			row := rowOf(t, runSetup(t, tc.verb, tc.path, tc.tokens...), "claude", itemSkill)

			assert.Equal(t, stateForeign, row.State)
			assert.Equal(t, foreign, readFile(t, claudeStub(home)))
			if tc.verb == verbCreate {
				assert.Contains(t, row.Note, "rm "+claudeStub(home)+" && ze create agent skill")
			}
		})
	}
}

// TestCreateReplacesALinkedSkillDirectoryOrFileAndLeavesItsTarget proves
// AC-5, one row per link position.
//
// Goal: a link at the ze directory or at its SKILL.md is replaced by a real
// directory or file holding the stub, and the link's target keeps its bytes.
// Method: place the link, run create, then Lstat the place and read the
// target.
func TestCreateReplacesALinkedSkillDirectoryOrFileAndLeavesItsTarget(t *testing.T) {
	const kept = "the target's own bytes\n"
	cases := []struct {
		name  string
		place func(home string) string
	}{
		{"the ze directory is a link", func(home string) string {
			return filepath.Join(home, ".claude", "skills", "ze")
		}},
		{"SKILL.md is a link", claudeStub},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := isolatedHome(t, ".claude")
			target := filepath.Join(t.TempDir(), "elsewhere")
			writeFile(t, filepath.Join(target, "SKILL.md"), kept)
			link := tc.place(home)
			pointsAt := target
			if filepath.Base(link) == "SKILL.md" {
				pointsAt = filepath.Join(target, "SKILL.md")
			}
			require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o700))
			require.NoError(t, os.Symlink(pointsAt, link))

			runSetup(t, verbCreate, pathCreateAgentSkill, "agent", "claude")

			info, err := os.Lstat(link)
			require.NoError(t, err)
			assert.Zero(t, info.Mode()&os.ModeSymlink, "the link is still at %s", link)
			assert.Equal(t, stubText(formLocal), readFile(t, claudeStub(home)))
			assert.Equal(t, kept, readFile(t, filepath.Join(target, "SKILL.md")), "the link's target was changed")
		})
	}
}

// TestCreateWritesInsideALinkedSkillsDirectory proves AC-6.
//
// Goal: a linked skills directory is the operator's placement: the stub goes
// inside the directory it points at, and the link stays. Method: link
// ~/.claude/skills to another directory, run create, read both.
func TestCreateWritesInsideALinkedSkillsDirectory(t *testing.T) {
	home := isolatedHome(t, ".claude")
	dotfiles := filepath.Join(t.TempDir(), "dotfiles-skills")
	require.NoError(t, os.MkdirAll(dotfiles, 0o700))
	link := filepath.Join(home, ".claude", "skills")
	require.NoError(t, os.Symlink(dotfiles, link))

	answer := runSetup(t, verbCreate, pathCreateAgentSkill, "agent", "claude")

	assert.Equal(t, stateCreated, rowOf(t, answer, "claude", itemSkill).State)
	assert.Equal(t, stubText(formLocal), readFile(t, filepath.Join(dotfiles, "ze", "SKILL.md")))
	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink, "the operator's link was replaced")
}

// TestHarnessesWithoutHooksAnswerUnsupportedAndShareOneStub proves AC-15.
//
// Goal: with OpenCode and Pi found, both stub rows name the one shared file,
// it is acted on once, and their hook rows answer unsupported with no hook
// file written. Method: a home with both directories, create, then the rows
// and the directories.
func TestHarnessesWithoutHooksAnswerUnsupportedAndShareOneStub(t *testing.T) {
	home := isolatedHome(t, ".config/opencode", ".pi")

	answer := runSetup(t, verbCreate, pathCreateAgentSkill)

	opencode := rowOf(t, answer, "opencode", itemSkill)
	pi := rowOf(t, answer, "pi", itemSkill)
	assert.Equal(t, sharedStub(home), opencode.Path)
	assert.Equal(t, opencode.Path, pi.Path)
	assert.Equal(t, stateCreated, opencode.State)
	assert.Equal(t, opencode.State, pi.State, "the second row saw a second write")
	for _, agent := range []string{"opencode", "pi"} {
		hook := rowOf(t, answer, agent, itemHook)
		assert.Equal(t, stateUnsupported, hook.State, agent)
		assert.NotEmpty(t, hook.Note, agent)
	}
	for _, dir := range []string{".config/opencode", ".pi"} {
		entries, err := os.ReadDir(filepath.Join(home, filepath.FromSlash(dir)))
		require.NoError(t, err)
		assert.Empty(t, entries, "a file was written in %s", dir)
	}
}

// TestANamedAgentIsWrittenEvenWhenNotFoundAndNoOtherIs proves AC-16.
//
// Goal: `agent claude` writes Claude Code's stub though its directory does
// not exist, and touches no other harness. Method: a home where only Codex
// is found, create agent claude, then the files and the rows.
func TestANamedAgentIsWrittenEvenWhenNotFoundAndNoOtherIs(t *testing.T) {
	home := isolatedHome(t, ".codex")

	answer := runSetup(t, verbCreate, pathCreateAgentSkill, "agent", "claude")

	assert.FileExists(t, claudeStub(home))
	assert.NoDirExists(t, filepath.Join(home, ".agents"), "the shared stub was written for a harness not named")
	for _, row := range answer.Setup.Rows {
		assert.Equal(t, "claude", row.Agent, "a row for a harness not named: %+v", row)
	}
}

// TestWithNoHarnessFoundTheSharedStubIsWritten proves AC-17.
//
// Goal: with no harness found and none named, create writes the shared stub,
// says no harness was found and who reads the shared directory, and answers
// no hook. Method: an empty home, create, then the file, the note and rows.
func TestWithNoHarnessFoundTheSharedStubIsWritten(t *testing.T) {
	home := isolatedHome(t)

	answer := runSetup(t, verbCreate, pathCreateAgentSkill)

	assert.FileExists(t, sharedStub(home))
	assert.Contains(t, answer.Setup.Note, "No agent harness was found")
	assert.Contains(t, answer.Setup.Note, titlesOf(sharedReaders()))
	for _, row := range answer.Setup.Rows {
		assert.Equal(t, itemSkill, row.Item, "a hook row with no harness found: %+v", row)
	}
	assert.NoDirExists(t, filepath.Join(home, ".claude"))
	assert.NoDirExists(t, filepath.Join(home, ".codex"))
}

// TestShowAgentSkillCreatesAndChangesNoFile proves AC-18.
//
// Goal: show reports every harness the plugin knows, per item, with whether
// it was found and a state from show's closed set, and changes no file.
// Method: a home with an install for Claude Code only, a snapshot of every
// entry, show, then the rows and a second snapshot.
func TestShowAgentSkillCreatesAndChangesNoFile(t *testing.T) {
	home := isolatedHome(t, ".claude")
	runSetup(t, verbCreate, pathCreateAgentSkill, "agent", "claude")
	before := snapshotOf(t, home)

	answer := runSetup(t, verbShow, pathShowAgentSkill)

	assert.Equal(t, before, snapshotOf(t, home), "show changed the home")
	allowed := map[string]bool{stateCurrent: true, stateOutdated: true, stateMissing: true,
		stateForeign: true, stateRefused: true, stateUnsupported: true}
	for i := range harnesses {
		for _, item := range []string{itemSkill, itemHook} {
			row := rowOf(t, answer, harnesses[i].name, item)
			assert.True(t, allowed[row.State], "%s %s answered %q", row.Agent, item, row.State)
		}
	}
	claude := rowOf(t, answer, "claude", itemSkill)
	assert.Equal(t, stateCurrent, claude.State)
	assert.Equal(t, harnessFound, claude.Harness)
	assert.Equal(t, claudeStub(home), claude.Path)
	assert.Equal(t, harnessNotFound, rowOf(t, answer, "codex", itemSkill).Harness)
	assert.Equal(t, stateMissing, rowOf(t, answer, "codex", itemSkill).State)
}

// snapshotOf answers every entry under root with its mode and, for a regular
// file, its bytes.
func snapshotOf(t *testing.T, root string) map[string]string {
	t.Helper()
	entries := make(map[string]string)
	require.NoError(t, filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if info.Mode().IsRegular() {
			value += "\n" + readFile(t, path)
		}
		entries[path] = value
		return nil
	}))
	return entries
}

// TestDeleteRemovesWhatZeWroteAndASecondDeleteAnswersAbsent proves AC-19,
// the stub half.
//
// Goal: `delete agent skill agent all` removes each stub Ze wrote and its
// emptied directory, answers removed, leaves another skill, and a second run
// answers absent. Method: create for Claude Code and Codex beside a sibling
// skill, delete twice, then the files and the rows.
func TestDeleteRemovesWhatZeWroteAndASecondDeleteAnswersAbsent(t *testing.T) {
	home := isolatedHome(t, ".claude", ".codex")
	sibling := filepath.Join(home, ".claude", "skills", "other", "SKILL.md")
	writeFile(t, sibling, "another skill\n")
	runSetup(t, verbCreate, pathCreateAgentSkill)

	first := runSetup(t, verbDelete, pathDeleteAgentSkill, "agent", agentAll)

	for _, agent := range []string{"claude", "codex"} {
		assert.Equal(t, stateRemoved, rowOf(t, first, agent, itemSkill).State, agent)
	}
	assert.NoDirExists(t, filepath.Dir(claudeStub(home)))
	assert.NoDirExists(t, filepath.Dir(sharedStub(home)))
	assert.Equal(t, "another skill\n", readFile(t, sibling))

	second := runSetup(t, verbDelete, pathDeleteAgentSkill, "agent", agentAll)

	for _, agent := range []string{"claude", "codex"} {
		assert.Equal(t, stateAbsent, rowOf(t, second, agent, itemSkill).State, agent)
	}
}

// TestDeleteKeepsASharedStubAnotherFoundHarnessReads proves AC-20.
//
// Goal: `delete agent skill agent pi` keeps the shared stub while Codex,
// found, still reads it, and names Codex. Method: create with Codex and Pi
// found, delete agent pi, then the file and the row.
func TestDeleteKeepsASharedStubAnotherFoundHarnessReads(t *testing.T) {
	home := isolatedHome(t, ".codex", ".pi")
	runSetup(t, verbCreate, pathCreateAgentSkill)

	answer := runSetup(t, verbDelete, pathDeleteAgentSkill, "agent", "pi")

	assert.FileExists(t, sharedStub(home))
	row := rowOf(t, answer, "pi", itemSkill)
	assert.Equal(t, stateKept, row.State)
	assert.Contains(t, row.Note, "Codex")
	assert.NotContains(t, row.Note, "Pi,", "the harness being removed is named as a reader")
}

// TestTheRemoteStubSendsTheAgentToTheRouterAndTheRemotePlanHasNoHook proves
// AC-21 and R-7 at the producer.
//
// Goal: the remote form's stub orders no local ze command and its plan holds
// no hook item, while the local form has both. Method: agentPlan in each form
// over the whole table.
func TestTheRemoteStubSendsTheAgentToTheRouterAndTheRemotePlanHasNoHook(t *testing.T) {
	remote := agentPlan(formRemote, everyHarness(), nil)
	local := agentPlan(formLocal, everyHarness(), nil)

	for _, item := range remote {
		assert.NotEqual(t, itemHook, item.item, "the remote plan holds a hook for %s", item.harness.name)
		assert.NotContains(t, item.content, "ze show skill", "the remote stub orders a local ze command")
		assert.NotContains(t, item.content, "`ze ", "the remote stub names a local ze command")
		assert.Contains(t, item.content, "show skill ze")
	}
	hooks := 0
	for _, item := range local {
		if item.item == itemHook {
			hooks++
			continue
		}
		assert.Contains(t, item.content, "ze show skill ze")
	}
	assert.Equal(t, len(harnesses), hooks, "the local plan has no hook item for some harness")
}

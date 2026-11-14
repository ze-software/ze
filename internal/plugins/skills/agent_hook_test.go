// Design: docs/features/ai-first.md — setting an operator's agent up to use Ze
// Related: agent_hook.go — the settings file splice under test
// Related: agent_local_test.go — the home isolation and shell route helpers
//
// Every test sets HOME, CLAUDE_CONFIG_DIR and CODEX_HOME to a directory of its
// own through isolatedHome, so no test reaches the settings file of whoever
// runs it.

package skills

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The hook groups the fixtures hold: Rex's, as it stands on the owner's
// machine; Ze's, as an entry another ze could have written; another one with
// a matcher; and the empty group Codex's own file carries.
const (
	rexGroup   = `{"hooks": [{"type": "command", "command": "/bin/sh -c 'rex __internal session-start --agent claude 2>/dev/null || true'", "timeout": 5}]}`
	zeInline   = `{"hooks": [{"type": "command", "command": "/bin/sh -c 'ze show agent context 2>/dev/null || true'", "timeout": 5}]}`
	otherGroup = `{"matcher": "startup", "hooks": [{"type": "command", "command": "echo hello", "timeout": 1}]}`
	emptyGroup = `{"hooks": []}`
)

// The command lines the shell route names in a refusal.
const (
	rerunCreateClaude = "ze create agent skill agent claude"
)

// wantZeGroup is the group Ze installs, written here independently of the
// producer: no matcher, one command entry with a five-second timeout.
func wantZeGroup() any {
	return map[string]any{"hooks": []any{map[string]any{
		"type":    "command",
		"command": "/bin/sh -c 'ze show agent context 2>/dev/null || true'",
		"timeout": float64(5),
	}}}
}

// claudeSettings and codexHooks answer the two settings files under home.
func claudeSettings(home string) string {
	return filepath.Join(home, ".claude", "settings.json")
}
func codexHooks(home string) string {
	return filepath.Join(home, ".codex", "hooks.json")
}

// sessionStartList wraps groups in a settings file's hook list, two spaces
// deep, one group per line.
func sessionStartList(groups ...string) string {
	return "{\n  \"hooks\": {\n    \"SessionStart\": [\n      " + strings.Join(groups, ",\n      ") + "\n    ]\n  }\n}\n"
}

// sessionStartOf decodes the SessionStart list of a settings file's text.
func sessionStartOf(t *testing.T, text string) []any {
	t.Helper()
	var settings map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &settings), "the file is not valid JSON:\n%s", text)
	hooks, isObject := settings["hooks"].(map[string]any)
	require.True(t, isObject, "the file holds no hooks object:\n%s", text)
	list, isList := hooks["SessionStart"].([]any)
	require.True(t, isList, "the file holds no SessionStart list:\n%s", text)
	return list
}

// lastNoteLine answers the last line of a row's note.
func lastNoteLine(row setupRow) string {
	lines := strings.Split(row.Note, "\n")
	return lines[len(lines)-1]
}

// TestCreateSplicesOneHookGroupAndLeavesEveryOtherByte proves AC-7.
//
// Goal: create appends one group at the end of hooks.SessionStart, one step
// deeper than the list's own line, and every byte before and after the
// inserted text is the original's. Method: a settings file with unrelated
// keys, another event and Rex's group, in three indentations, with and
// without a final newline; the shell route; then the file against the
// original split where the group goes.
func TestCreateSplicesOneHookGroupAndLeavesEveryOtherByte(t *testing.T) {
	for _, tc := range []struct {
		name  string
		unit  string
		final string
	}{
		{name: "two spaces, final newline", unit: "  ", final: "\n"},
		{name: "tabs, no final newline", unit: "\t", final: ""},
		{name: "four spaces, final newline", unit: "    ", final: "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := isolatedHome(t, ".claude")
			u := tc.unit
			before := "{\n" + u + `"model": "opus",` + "\n" + u + `"hooks": {` + "\n" +
				u + u + `"Stop": [],` + "\n" + u + u + `"SessionStart": [` + "\n" + u + u + u + rexGroup
			after := "\n" + u + u + "]\n" + u + "},\n" + u + `"permissions": {"allow": ["Bash(ls)"]}` + "\n}" + tc.final
			writeFile(t, claudeSettings(home), before+after)

			answer := runSetup(t, verbCreate, pathCreateAgentSkill, "agent", "claude")

			assert.Equal(t, stateCreated, rowOf(t, answer, "claude", itemHook).State)
			result := readFile(t, claudeSettings(home))
			require.True(t, strings.HasPrefix(result, before), "the bytes before the group changed:\n%s", result)
			require.True(t, strings.HasSuffix(result, after), "the bytes after the group changed:\n%s", result)
			inserted := result[len(before) : len(result)-len(after)]
			assert.True(t, strings.HasPrefix(inserted, ",\n"+u+u+u+"{\n"+u+u+u+u+`"hooks"`),
				"the group is not one step deeper than its list: %q", inserted)
			list := sessionStartOf(t, result)
			require.Len(t, list, 2)
			assert.Equal(t, wantZeGroup(), list[1])
		})
	}
}

// TestCreateInsertsAMissingHooksOrSessionStartKey proves AC-8.
//
// Goal: a file with no hooks key, a hooks object with no SessionStart, or an
// empty object gains the missing keys with Ze's group, and every byte outside
// the inserted text is the original's. Method: one file per shape, the shell
// route, then the file against the original split where the text goes.
func TestCreateInsertsAMissingHooksOrSessionStartKey(t *testing.T) {
	for _, tc := range []struct {
		name   string
		before string
		after  string
	}{
		{name: "no hooks key", before: "{\n  \"model\": \"opus\"", after: "\n}\n"},
		{name: "hooks without SessionStart", before: "{\n  \"hooks\": {\n    \"Stop\": []", after: "\n  }\n}\n"},
		{name: "an empty hooks object", before: "{\n  \"hooks\": {", after: "}\n}\n"},
		{name: "an empty file object", before: "{", after: "}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := isolatedHome(t, ".claude")
			writeFile(t, claudeSettings(home), tc.before+tc.after)

			answer := runSetup(t, verbCreate, pathCreateAgentSkill, "agent", "claude")

			assert.Equal(t, stateCreated, rowOf(t, answer, "claude", itemHook).State)
			result := readFile(t, claudeSettings(home))
			assert.True(t, strings.HasPrefix(result, tc.before), "the bytes before the insert changed:\n%s", result)
			assert.True(t, strings.HasSuffix(result, tc.after), "the bytes after the insert changed:\n%s", result)
			assert.Equal(t, []any{wantZeGroup()}, sessionStartOf(t, result))
		})
	}
}

// TestCreateMakesAMissingSettingsFile proves AC-9.
//
// Goal: with no settings file, create writes one, owner-only, holding only
// the hook list, whether the harness was found or only named. Method: a home
// with Claude Code's directory and one without it, the shell route naming
// Claude Code, then the file's mode and content.
func TestCreateMakesAMissingSettingsFile(t *testing.T) {
	for _, tc := range []struct {
		name string
		dirs []string
	}{
		{name: "the harness directory exists", dirs: []string{".claude"}},
		{name: "the harness is named and its directory does not exist", dirs: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := isolatedHome(t, tc.dirs...)

			answer := runSetup(t, verbCreate, pathCreateAgentSkill, "agent", "claude")

			row := rowOf(t, answer, "claude", itemHook)
			assert.Equal(t, stateCreated, row.State)
			assert.Equal(t, claudeSettings(home), row.Path)
			info, err := os.Stat(claudeSettings(home))
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
			var settings any
			require.NoError(t, json.Unmarshal([]byte(readFile(t, claudeSettings(home))), &settings))
			assert.Equal(t, map[string]any{"hooks": map[string]any{"SessionStart": []any{wantZeGroup()}}}, settings)
		})
	}
}

// TestASettingsFileZeDoesNotUnderstandIsLeftAndRefused proves AC-10.
//
// Goal: each shape Ze does not fully understand leaves the file byte for
// byte, answers the hook row refused with what was found and where, and ends
// the note with the command to run again; the stub is still written. Method:
// one file per shape, the shell route, then the file, the row and the stub.
func TestASettingsFileZeDoesNotUnderstandIsLeftAndRefused(t *testing.T) {
	for _, tc := range []struct {
		name  string
		file  string
		found string
	}{
		{name: "not valid JSON", file: "{\n  \"hooks\": \n}\n", found: "not valid JSON"},
		{name: "a byte-order mark", file: "\xef\xbb\xbf{}\n", found: "byte-order mark"},
		{name: "a duplicate hooks key", file: `{"hooks": {}, "hooks": {}}`, found: `a second "hooks" key`},
		{name: "a duplicate SessionStart key", file: `{"hooks": {"SessionStart": [], "SessionStart": []}}`, found: `a second "SessionStart" key`},
		{name: "hooks is not an object", file: `{"hooks": []}`, found: `"hooks" value that is not an object`},
		{name: "SessionStart is not a list", file: `{"hooks": {"SessionStart": {}}}`, found: `"SessionStart" value that is not a list`},
		{name: "a group is not an object", file: `{"hooks": {"SessionStart": ["x"]}}`, found: "group that is not an object"},
		{name: "the top level is not an object", file: `[]`, found: "top level that is not a JSON object"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := isolatedHome(t, ".claude")
			writeFile(t, claudeSettings(home), tc.file)

			answer := runSetup(t, verbCreate, pathCreateAgentSkill, "agent", "claude")

			assert.Equal(t, tc.file, readFile(t, claudeSettings(home)), "the file was changed")
			row := rowOf(t, answer, "claude", itemHook)
			assert.Equal(t, stateRefused, row.State)
			assert.Contains(t, row.Note, tc.found)
			assert.Contains(t, row.Note, " at line ")
			assert.Equal(t, rerunCreateClaude, lastNoteLine(row))
			assert.Equal(t, stateCreated, rowOf(t, answer, "claude", itemSkill).State)
		})
	}
}

// TestALinkedSettingsFileIsWrittenThroughToItsTarget proves AC-11.
//
// Goal: a settings file that is a link, directly or through a chain, stays a
// link, and the regular file it finally names holds the new content with its
// permission bits unchanged. Method: a target in a dotfiles directory with
// mode 0640, one link or two in front of it, the shell route, then the links,
// the target's mode and its content.
func TestALinkedSettingsFileIsWrittenThroughToItsTarget(t *testing.T) {
	for _, tc := range []struct {
		name  string
		chain bool
	}{
		{name: "a direct link", chain: false},
		{name: "a chain of two links", chain: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := isolatedHome(t, ".claude")
			target := filepath.Join(home, "dotfiles", "claude-settings.json")
			writeFile(t, target, "{\n  \"model\": \"opus\"\n}\n")
			require.NoError(t, os.Chmod(target, 0o640))
			pointsAt := target
			if tc.chain {
				pointsAt = filepath.Join(home, "dotfiles", "current.json")
				require.NoError(t, os.Symlink(target, pointsAt))
			}
			require.NoError(t, os.Symlink(pointsAt, claudeSettings(home)))

			answer := runSetup(t, verbCreate, pathCreateAgentSkill, "agent", "claude")

			row := rowOf(t, answer, "claude", itemHook)
			assert.Equal(t, stateCreated, row.State)
			assert.Contains(t, row.Note, "claude-settings.json")
			link, err := os.Readlink(claudeSettings(home))
			require.NoError(t, err, "the settings file is no longer a link")
			assert.Equal(t, pointsAt, link)
			info, err := os.Lstat(target)
			require.NoError(t, err)
			assert.True(t, info.Mode().IsRegular())
			assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
			assert.Equal(t, []any{wantZeGroup()}, sessionStartOf(t, readFile(t, target)))
		})
	}
}

// TestASettingsFileThatCannotBeWrittenIsRefused proves AC-12.
//
// Goal: a dangling link, a link to a directory, a directory at the file's
// place, a read-only file and a file in a read-only directory each answer the
// hook row refused with the fix last, and nothing is created or changed.
// Method: one case per row, the shell route, then the row and the place. The
// two read-only rows are skipped as root, which writes whatever the mode says.
func TestASettingsFileThatCannotBeWrittenIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name     string
		readOnly bool
		set      func(t *testing.T, home string)
		left     func(t *testing.T, home string)
	}{
		{name: "a dangling link",
			set: func(t *testing.T, home string) {
				require.NoError(t, os.Symlink(filepath.Join(home, "nowhere.json"), claudeSettings(home)))
			},
			left: func(t *testing.T, home string) {
				assert.NoFileExists(t, filepath.Join(home, "nowhere.json"))
				_, err := os.Readlink(claudeSettings(home))
				assert.NoError(t, err, "the link is gone")
			}},
		{name: "a link to a directory",
			set: func(t *testing.T, home string) {
				require.NoError(t, os.Mkdir(filepath.Join(home, "adir"), 0o700))
				require.NoError(t, os.Symlink(filepath.Join(home, "adir"), claudeSettings(home)))
			},
			left: func(t *testing.T, home string) {
				entries, err := os.ReadDir(filepath.Join(home, "adir"))
				require.NoError(t, err)
				assert.Empty(t, entries)
			}},
		{name: "a directory at the file's place",
			set: func(t *testing.T, home string) {
				require.NoError(t, os.Mkdir(claudeSettings(home), 0o700))
			},
			left: func(t *testing.T, home string) {
				assert.DirExists(t, claudeSettings(home))
			}},
		{name: "a read-only file", readOnly: true,
			set: func(t *testing.T, home string) {
				writeFile(t, claudeSettings(home), "{}\n")
				require.NoError(t, os.Chmod(claudeSettings(home), 0o400))
			},
			left: func(t *testing.T, home string) {
				assert.Equal(t, "{}\n", readFile(t, claudeSettings(home)))
				assert.Equal(t, []string{"settings.json", "skills"}, namesIn(t, filepath.Join(home, ".claude")))
			}},
		{name: "a file in a read-only directory", readOnly: true,
			set: func(t *testing.T, home string) {
				shut := filepath.Join(home, "shut")
				writeFile(t, filepath.Join(shut, "settings.json"), "{}\n")
				require.NoError(t, os.Chmod(shut, 0o500))
				t.Cleanup(func() { _ = os.Chmod(shut, 0o700) })
				require.NoError(t, os.Symlink(filepath.Join(shut, "settings.json"), claudeSettings(home)))
			},
			left: func(t *testing.T, home string) {
				assert.Equal(t, "{}\n", readFile(t, filepath.Join(home, "shut", "settings.json")))
				assert.Equal(t, []string{"settings.json"}, namesIn(t, filepath.Join(home, "shut")))
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.readOnly && os.Geteuid() == 0 {
				t.Skip("root writes a file whatever its mode says")
			}
			home := isolatedHome(t, ".claude")
			tc.set(t, home)

			answer := runSetup(t, verbCreate, pathCreateAgentSkill, "agent", "claude")

			row := rowOf(t, answer, "claude", itemHook)
			assert.Equal(t, stateRefused, row.State)
			assert.Equal(t, claudeSettings(home), row.Path)
			assert.Equal(t, rerunCreateClaude, lastNoteLine(row))
			tc.left(t, home)
		})
	}
}

// namesIn answers the names of the entries in directory, sorted.
func namesIn(t *testing.T, directory string) []string {
	t.Helper()
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// TestASettingsFileChangedSinceTheReadIsNotReplaced proves AC-13.
//
// Goal: when the file changes between Ze's read and its replace, Ze leaves
// the other writer's content, removes its temporary file, and answers the row
// refused, saying the file changed, with the command to run again last.
// Method: the editor's seam rewrites the file in that moment, then the file,
// the directory and the row.
func TestASettingsFileChangedSinceTheReadIsNotReplaced(t *testing.T) {
	home := isolatedHome(t, ".claude")
	writeFile(t, claudeSettings(home), "{\n  \"model\": \"opus\"\n}\n")
	other := "{\n  \"model\": \"sonnet\",\n  \"theme\": \"dark\"\n}\n"
	claude, known := harnessNamed("claude")
	require.True(t, known)
	editor := hookEditor{beforeReplace: func(target string) {
		require.NoError(t, os.WriteFile(target, []byte(other), 0o600))
	}}

	result := editor.create(claude, claudeSettings(home), rerunCreateClaude)

	assert.Equal(t, stateRefused, result.state)
	assert.Contains(t, result.note, "changed while Ze was editing it")
	assert.Equal(t, rerunCreateClaude, lastNoteLine(setupRow{Note: result.note}))
	assert.Equal(t, other, readFile(t, claudeSettings(home)))
	assert.Equal(t, []string{"settings.json"}, namesIn(t, filepath.Join(home, ".claude")))
}

// TestTheCodexHookRowSaysItNeedsApproval proves AC-14.
//
// Goal: with Codex found, create installs Ze's group in hooks.json and the
// row's note says Codex skips it until it is trusted through /hooks. Method:
// a home with Codex's directory, the shell route, then the file and the row.
func TestTheCodexHookRowSaysItNeedsApproval(t *testing.T) {
	home := isolatedHome(t, ".codex")

	answer := runSetup(t, verbCreate, pathCreateAgentSkill)

	row := rowOf(t, answer, "codex", itemHook)
	assert.Equal(t, stateCreated, row.State)
	assert.Contains(t, row.Note, "until you trust it")
	assert.Contains(t, row.Note, "/hooks")
	assert.Equal(t, []any{wantZeGroup()}, sessionStartOf(t, readFile(t, codexHooks(home))))
}

// TestShowSaysItCannotSeeCodexsTrust proves AC-14's show half.
//
// Goal: show answers Codex's installed hook current, and its note says Ze
// cannot see whether Codex trusts it and how to check. Method: create, then
// show, then the row.
func TestShowSaysItCannotSeeCodexsTrust(t *testing.T) {
	isolatedHome(t, ".codex")
	runSetup(t, verbCreate, pathCreateAgentSkill)

	answer := runSetup(t, verbShow, pathShowAgentSkill)

	row := rowOf(t, answer, "codex", itemHook)
	assert.Equal(t, stateCurrent, row.State)
	assert.Contains(t, row.Note, "cannot tell whether it runs this one")
	assert.Contains(t, row.Note, "/hooks")
}

// TestDeleteSplicesOutOnlyZesHookGroup proves AC-19, the settings half.
//
// Goal: delete removes Ze's group with its separating comma and leaves every
// other byte; in Codex's file a group that is not the last is emptied in
// place, so every group after it keeps its index. Method: one file per case,
// the shell route naming the harness, then the file against the exact text
// expected and the row.
func TestDeleteSplicesOutOnlyZesHookGroup(t *testing.T) {
	for _, tc := range []struct {
		name     string
		agent    string
		file     func(home string) string
		original string
		want     string
		emptied  bool
	}{
		{name: "Claude Code, a group between two others", agent: "claude", file: claudeSettings,
			original: sessionStartList(rexGroup, zeInline, otherGroup),
			want:     sessionStartList(rexGroup, otherGroup)},
		{name: "Claude Code, Ze's group alone", agent: "claude", file: claudeSettings,
			original: sessionStartList(zeInline),
			want:     "{\n  \"hooks\": {\n    \"SessionStart\": []\n  }\n}\n"},
		{name: "Codex, a group that is not the last", agent: "codex", file: codexHooks,
			original: sessionStartList(emptyGroup, zeInline, rexGroup),
			want:     sessionStartList(emptyGroup, emptyGroup, rexGroup), emptied: true},
		{name: "Codex, the last group", agent: "codex", file: codexHooks,
			original: sessionStartList(emptyGroup, rexGroup, zeInline),
			want:     sessionStartList(emptyGroup, rexGroup)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := isolatedHome(t, ".claude", ".codex")
			writeFile(t, tc.file(home), tc.original)

			answer := runSetup(t, verbDelete, pathDeleteAgentSkill, "agent", tc.agent)

			assert.Equal(t, tc.want, readFile(t, tc.file(home)))
			row := rowOf(t, answer, tc.agent, itemHook)
			assert.Equal(t, stateRemoved, row.State)
			if tc.emptied {
				assert.Contains(t, row.Note, "emptied its group's hook list")
			}
		})
	}
}

// TestCreateRefreshesAnOutdatedHookGroup proves AC-3's hook counterpart.
//
// Goal: a group of Ze's that differs from what this binary writes is shown
// outdated, rewritten in place by create (answering updated) with every byte
// outside it unchanged, and then shown current. Method: a file holding Rex's
// group and Ze's with another timeout, show, create, then the file and show.
func TestCreateRefreshesAnOutdatedHookGroup(t *testing.T) {
	home := isolatedHome(t, ".claude")
	outdated := strings.Replace(zeInline, `"timeout": 5`, `"timeout": 10`, 1)
	original := sessionStartList(rexGroup, outdated)
	writeFile(t, claudeSettings(home), original)
	require.Equal(t, stateOutdated, rowOf(t, runSetup(t, verbShow, pathShowAgentSkill), "claude", itemHook).State)

	answer := runSetup(t, verbCreate, pathCreateAgentSkill, "agent", "claude")

	assert.Equal(t, stateUpdated, rowOf(t, answer, "claude", itemHook).State)
	result := readFile(t, claudeSettings(home))
	at := strings.Index(original, outdated)
	require.True(t, strings.HasPrefix(result, original[:at]), "the bytes before the group changed:\n%s", result)
	require.True(t, strings.HasSuffix(result, original[at+len(outdated):]), "the bytes after the group changed:\n%s", result)
	list := sessionStartOf(t, result)
	require.Len(t, list, 2)
	assert.Equal(t, wantZeGroup(), list[1])
	assert.Equal(t, stateCurrent, rowOf(t, runSetup(t, verbShow, pathShowAgentSkill), "claude", itemHook).State)
}

// TestAnotherInstallersGroupIsNeverTouched proves R-2 for the hook.
//
// Goal: Rex's group survives create, show and delete byte for byte, and after
// the delete the file is exactly what it was before the create. Method: a
// file holding Rex's group alone, the three commands, the file after each.
func TestAnotherInstallersGroupIsNeverTouched(t *testing.T) {
	home := isolatedHome(t, ".claude")
	original := sessionStartList(rexGroup)
	writeFile(t, claudeSettings(home), original)
	upToRex := original[:strings.Index(original, rexGroup)+len(rexGroup)]

	runSetup(t, verbCreate, pathCreateAgentSkill, "agent", "claude")
	created := readFile(t, claudeSettings(home))
	assert.True(t, strings.HasPrefix(created, upToRex), "create changed Rex's group:\n%s", created)

	runSetup(t, verbShow, pathShowAgentSkill)
	assert.Equal(t, created, readFile(t, claudeSettings(home)), "show changed the file")

	runSetup(t, verbDelete, pathDeleteAgentSkill, "agent", "claude")
	assert.Equal(t, original, readFile(t, claudeSettings(home)), "delete did not give back the file as it was")
}

// TestASecondCreateLeavesACurrentHookGroupUnchanged proves the hook's
// unchanged state.
//
// Goal: a create that finds Ze's current group answers unchanged and does not
// rewrite the file. Method: create, set the file's modification time back an
// hour, create again, then the row, the bytes and the time.
func TestASecondCreateLeavesACurrentHookGroupUnchanged(t *testing.T) {
	home := isolatedHome(t, ".claude")
	runSetup(t, verbCreate, pathCreateAgentSkill, "agent", "claude")
	before := readFile(t, claudeSettings(home))
	backdated := time.Now().Add(-time.Hour).Truncate(time.Second)
	require.NoError(t, os.Chtimes(claudeSettings(home), backdated, backdated))

	answer := runSetup(t, verbCreate, pathCreateAgentSkill, "agent", "claude")

	assert.Equal(t, stateUnchanged, rowOf(t, answer, "claude", itemHook).State)
	assert.Equal(t, before, readFile(t, claudeSettings(home)))
	info, err := os.Stat(claudeSettings(home))
	require.NoError(t, err)
	assert.True(t, info.ModTime().Equal(backdated), "the settings file was rewritten: %s", info.ModTime())
}

// TestTheInstalledHookCommandSucceedsSilentlyWithoutZe proves AC-25.
//
// Goal: the command Ze installs prints nothing and exits 0 where no ze is on
// the path. Method: run the command string with /bin/sh, as a harness does,
// with PATH naming an empty directory, then its output and exit status.
func TestTheInstalledHookCommandSucceedsSilentlyWithoutZe(t *testing.T) {
	command := exec.CommandContext(t.Context(), "/bin/sh", "-c", hookCommand)
	command.Env = []string{"PATH=" + t.TempDir()}

	output, err := command.CombinedOutput()

	require.NoError(t, err, "the hook command failed without ze")
	assert.Empty(t, string(output))
}

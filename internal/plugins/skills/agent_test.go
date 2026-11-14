// Design: docs/features/ai-first.md — setting an operator's agent up to use Ze
// Related: agent.go — the four commands under test
// Related: register.go — their local and daemon registrations

package skills_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	// Trigger every module and RPC registration, so the dispatcher under test
	// holds the handlers and the schema the daemon holds. An external test
	// package, because plugin/all imports this plugin.
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)

// agentPaths are the four commands that set an operator's agent up.
var agentPaths = []string{
	"create agent skill",
	"show agent skill",
	"delete agent skill",
	"show agent context",
}

// isolateHome points HOME, CLAUDE_CONFIG_DIR and CODEX_HOME at directories of
// the test's own, so no command under test can reach the home of whoever runs
// it, and answers the home.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	return home
}

// homeSnapshot answers every entry under root with its mode and, for a
// regular file, its bytes, so two snapshots differ when anything was created,
// removed or rewritten.
func homeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	entries := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += "\n" + string(data)
		}
		entries[path] = value
		return nil
	})
	require.NoError(t, err)
	return entries
}

// daemonDispatcher answers a dispatcher loaded the way the daemon loads its
// builtins: every registered RPC at the path its YANG declares, with the
// argument definitions the same schema declares.
func daemonDispatcher(t *testing.T) *pluginserver.Dispatcher {
	t.Helper()
	loader, err := yang.DefaultLoader()
	require.NoError(t, err, "load the embedded YANG modules")
	d := pluginserver.NewDispatcher()
	require.NoError(t, pluginserver.LoadBuiltins(d, yang.WireMethodToPath(loader),
		yang.PathToDescription(loader), yang.PathToHelp(loader), yang.PathToArgDefs(loader)))
	return d
}

// TestCreateAgentSkillThroughTheDaemonAnswersThePlanAndTouchesNoFile proves
// the daemon form of create agent skill answers the stub a remote agent places
// and writes nothing.
//
// Goal: AC-21, the wiring row for the daemon. Method: a home holding a Claude
// Code and a Codex directory with content, the command dispatched through a
// dispatcher loaded as the daemon loads its builtins, then the answer read for
// the default home-relative stub path and the not-inspected state, and the
// home compared byte for byte with its state before the call.
//
// VALIDATES: AC-21 (create through the daemon).
// PREVENTS: the daemon writing into the home of the account it runs as, or
// answering the daemon host's own paths.
func TestCreateAgentSkillThroughTheDaemonAnswersThePlanAndTouchesNoFile(t *testing.T) {
	home := isolateHome(t)
	for _, dir := range []string{".claude", ".codex"} {
		require.NoError(t, os.MkdirAll(filepath.Join(home, dir), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(home, dir, "settings.json"), []byte("{}\n"), 0o600))
	}
	before := homeSnapshot(t, home)

	resp, err := daemonDispatcher(t).Dispatch(&pluginserver.CommandContext{}, "create agent skill")
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, plugin.StatusDone, resp.Status, "the daemon refused: %s", resp.Error)
	encoded, err := json.Marshal(resp.Data)
	require.NoError(t, err)
	answer := string(encoded)
	assert.Contains(t, answer, ".claude/skills/ze/SKILL.md", "the answer names no default home-relative stub path")
	assert.Contains(t, answer, "not-inspected", "the answer claims to know the caller's files")
	assert.NotContains(t, answer, home, "the answer names the daemon host's home")

	assert.Equal(t, before, homeSnapshot(t, home), "the daemon command changed the home")
}

// TestTheFourCommandsArePlainLocalWithADaemonCommand proves how each agent
// command is served.
//
// Goal: AC-27 and AC-28, and R-6. A plain local handler answers `ze <command>`
// at a shell; no local-data handler covers the path, so neither `ze cli -c`
// (command.ServeLocal) nor the daemon bridge runs it against a home; a builtin
// RPC is registered for the wire method the path's YANG node declares, so
// every daemon surface and `ze cli` reach the plugin's own command. Method:
// read the local registry, the local-data lookup, the schema's wire methods
// and the builtin RPC list for each path.
//
// VALIDATES: AC-27, AC-28 (wiring).
// PREVENTS: a local-data registration that would make `ze cli -c` or the
// bridge act on a home.
func TestTheFourCommandsArePlainLocalWithADaemonCommand(t *testing.T) {
	isolateHome(t)
	loader, err := yang.DefaultLoader()
	require.NoError(t, err, "load the embedded YANG modules")
	pathToWire := make(map[string]string)
	for wire, path := range yang.WireMethodToPath(loader) {
		pathToWire[path] = wire
	}
	builtin := make(map[string]bool)
	for _, reg := range pluginserver.AllBuiltinRPCs() {
		builtin[reg.WireMethod] = true
	}
	local := make(map[string]bool)
	for _, entry := range command.ListLocal() {
		local[entry.Path] = true
	}

	for _, path := range agentPaths {
		assert.True(t, local[path], "%s has no local handler", path)
		handler, _ := command.LookupLocalData(strings.Fields(path))
		assert.Nil(t, handler, "%s is covered by a local-data handler", path)
		wire, declared := pathToWire[path]
		if !assert.True(t, declared, "%s has no YANG ze:command node", path) {
			continue
		}
		assert.True(t, builtin[wire], "%s: no builtin RPC for %s", path, wire)
	}
}

// remoteSetup is the daemon route's answer as a remote caller decodes it.
type remoteSetup struct {
	Setup struct {
		Note string `json:"note"`
		Rows []struct {
			Agent   string `json:"agent"`
			Item    string `json:"item"`
			Harness string `json:"harness"`
			State   string `json:"state"`
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"rows"`
	} `json:"agent-skill"`
}

// dispatchSetup dispatches line through a dispatcher loaded as the daemon
// loads its builtins, and answers the decoded answer.
func dispatchSetup(t *testing.T, line string) remoteSetup {
	t.Helper()
	resp, err := daemonDispatcher(t).Dispatch(&pluginserver.CommandContext{}, line)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, plugin.StatusDone, resp.Status, "the daemon refused %q: %s", line, resp.Error)
	encoded, err := json.Marshal(resp.Data)
	require.NoError(t, err)
	var answer remoteSetup
	require.NoError(t, json.Unmarshal(encoded, &answer))
	return answer
}

// TestTheDaemonPlanUsesDefaultPathsWhateverTheDaemonsEnvironment proves
// AC-21 and R-9.
//
// Goal: the daemon command answers the harness defaults relative to the home
// directory, whatever CLAUDE_CONFIG_DIR and CODEX_HOME say in the daemon's
// own process, so it leaks none of the daemon host's paths. Method: set both
// variables to directories of the test's own, dispatch create, read the paths.
func TestTheDaemonPlanUsesDefaultPathsWhateverTheDaemonsEnvironment(t *testing.T) {
	home := isolateHome(t)
	elsewhere := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(elsewhere, "claude-config"))
	t.Setenv("CODEX_HOME", filepath.Join(elsewhere, "codex-home"))

	answer := dispatchSetup(t, "create agent skill")

	paths := make(map[string]string)
	for _, row := range answer.Setup.Rows {
		paths[row.Agent] = row.Path
		assert.NotContains(t, row.Path, elsewhere, "%s: the daemon's environment reached the answer", row.Agent)
		assert.NotContains(t, row.Path, home, "%s: the daemon's home reached the answer", row.Agent)
	}
	assert.Equal(t, "~/.claude/skills/ze/SKILL.md", paths["claude"])
	assert.Equal(t, "~/.agents/skills/ze/SKILL.md", paths["codex"])
}

// TestDeleteThroughTheDaemonAnswersWhatToRemove proves AC-21 for delete.
//
// Goal: `delete agent skill agent all` through the daemon removes nothing on
// the daemon's host and answers, per harness, the file the caller removes,
// not-inspected, with no content. Method: a home holding a stub, the daemon
// command, then the rows and the home compared before and after.
func TestDeleteThroughTheDaemonAnswersWhatToRemove(t *testing.T) {
	home := isolateHome(t)
	stub := filepath.Join(home, ".claude", "skills", "ze", "SKILL.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(stub), 0o700))
	require.NoError(t, os.WriteFile(stub, []byte("a stub on the daemon's host\n"), 0o600))
	before := homeSnapshot(t, home)

	answer := dispatchSetup(t, "delete agent skill agent all")

	assert.Equal(t, before, homeSnapshot(t, home), "the daemon command changed the home")
	require.NotEmpty(t, answer.Setup.Rows)
	for _, row := range answer.Setup.Rows {
		assert.Equal(t, "not-inspected", row.State, row.Agent)
		assert.Equal(t, "skill", row.Item, "%s: a hook in the remote plan", row.Agent)
		assert.True(t, strings.HasPrefix(row.Path, "~/"), "%s: %s is not home-relative", row.Agent, row.Path)
		assert.Empty(t, row.Content, "%s: delete carries content", row.Agent)
	}
}

package cli

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/config/storage"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/zefs"
)

// repairArgs answers tokens judged against the leaf ze-data-cmd.yang declares
// for `request data repair`, as the dispatcher judges them.
func repairArgs(t *testing.T, tokens ...string) command.ValidatedArgs {
	t.Helper()
	defs := []command.ArgDef{commandtest.Must(command.NewStringArg(keywordPath,
		[]command.UintRange{{Min: 1, Max: 4096}}, nil, command.ArgOptions{Mandatory: true}))}
	validated, err := command.ValidateArgs(tokens, defs, nil)
	require.NoError(t, err)
	return validated
}

// repairCall runs the repair handler on tokens judged by repairArgs.
func repairCall(t *testing.T, tokens ...string) *zePlugin.Response {
	t.Helper()
	response, err := handleDataRepair(&pluginserver.CommandContext{}, repairArgs(t, tokens...))
	require.NoError(t, err)
	return response
}

// TestRepairRPCPathRules verifies AC-11 through the RPC entry point. Method:
// a relative path, a ".." element, a symlink, an existing path, a name the
// store owns in its folder and a path inside the live tree are each refused
// with the rule named; an absolute new path is written as a new store that
// holds every key of the live one, whose own check is clean, and the answer
// carries the path and the recovered and skipped counts. The live store still
// reads its key afterwards.
// VALIDATES: AC-11.
// PREVENTS: a salvage written where the operator did not mean, or into the store's own entries.
func TestRepairRPCPathRules(t *testing.T) {
	store, configPath := dataRPCStore(t, func(context.Context) error { return nil })
	require.NoError(t, store.WriteKey("meta/test/state", []byte("runtime state")))
	storeDir := filepath.Dir(configPath)
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing")
	require.NoError(t, os.Mkdir(existing, 0o700))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(existing, link))

	for _, refused := range []struct {
		output string
		rule   string
	}{
		{"salvage", "absolute"},
		{dir + "/a/../salvage", `".."`},
		{link, "symlink"},
		{existing, "already exists"},
		{filepath.Join(storeDir, "salvage.lock"), "a lock sidecar"},
		{filepath.Join(storeDir, storage.TreeName, "salvage"), "inside the live tree"},
	} {
		response := repairCall(t, "path", refused.output)
		assert.Equal(t, zePlugin.StatusError, response.Status, refused.output)
		assert.Contains(t, response.Error, "request data repair", refused.output)
		assert.Contains(t, response.Error, refused.rule, refused.output)
	}

	output := filepath.Join(dir, "salvage")
	response := repairCall(t, "path", output)
	require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
	data, ok := response.Data.(zePlugin.Map)
	require.True(t, ok, "answer %T is not a map", response.Data)
	assert.Equal(t, output, data[keywordPath])
	recovered, ok := data["recovered"].(int)
	require.True(t, ok)
	assert.Positive(t, recovered)
	assert.Equal(t, 0, data["skipped"])

	salvaged, err := storage.OpenTree(output, false)
	require.NoError(t, err)
	value, err := salvaged.ReadKey("meta/test/state")
	require.NoError(t, err)
	assert.Equal(t, "runtime state", string(value))
	require.NoError(t, salvaged.Close())
	verdict, err := integrityVerdict(output)
	require.NoError(t, err)
	assert.Zero(t, verdict.errorCount(), "the salvage checks clean: %+v", verdict.findings)

	live, err := store.ReadKey("meta/test/state")
	require.NoError(t, err)
	assert.Equal(t, "runtime state", string(live))
}

// TestRepairShowsKeysRemovedDuringTheWalk verifies that a key a writer removed
// from the source while the salvage walked (zefs RepairReport.Vanished) is
// listed by `ze data repair` and counted on its summary line, and answered by
// `request data repair` as a count and a row per key, without changing the
// exit code or the status: the store no longer holds the key. Method: a real
// salvage of a one-key store, whose report gains a removed file key and a
// removed directory key, is printed and answered.
// PREVENTS: a salvage that leaves keys out without saying which.
func TestRepairShowsKeysRemovedDuringTheWalk(t *testing.T) {
	source := checkStore(t, map[string]string{"meta/test/state": "runtime state"})
	output := filepath.Join(t.TempDir(), "salvage")
	report, err := zefs.RepairPath(source, output)
	require.NoError(t, err)
	require.Empty(t, report.Vanished, "nothing writes the source")
	report.Vanished = append(report.Vanished, "meta/test/gone", "meta/test/dir/")

	code, stdout, stderr := captureCatStdio(t, func() int { return printRepair(report, output) })
	assert.Zero(t, code, stderr)
	assert.Contains(t, stdout, "removed: meta/test/gone\nremoved: meta/test/dir/\n")
	assert.True(t, strings.HasSuffix(stdout,
		strconv.Itoa(report.RecoveredCount)+" recovered, 0 skipped, 2 removed while the repair ran -> "+output+"\n"), stdout)

	response := repairAnswer(output, report, nil)
	assert.Equal(t, zePlugin.StatusDone, response.Status, response.Warnings)
	data, ok := response.Data.(zePlugin.Map)
	require.True(t, ok, "answer %T is not a map", response.Data)
	assert.Equal(t, 2, data[repairVanished])
	assert.Equal(t, []map[string]any{{keywordKey: "meta/test/gone"}, {keywordKey: "meta/test/dir/"}}, data[repairVanishedKeys])
}

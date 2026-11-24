package cli

import (
	"context"
	"encoding/base64"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	_ "github.com/ze-software/ze/internal/component/config/storage/yang" // registers ze-data-cmd.yang
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/redact"
)

// updateTestKey is the runtime-state key the update tests write.
const updateTestKey = "meta/test/written"

// The content bound of update data key, as ze-data-cmd.yang declares it: one
// SSH packet with the command around it.
const (
	updateContentEncodedMax = 261120
	updateContentBytesMax   = 195840
)

// updateDefs answers the leaves the shipped model declares for update data
// key, as the dispatcher's validator reads them.
func updateDefs(t *testing.T) []command.ArgDef {
	t.Helper()
	loader, err := configyang.DefaultLoader()
	require.NoError(t, err, "load the embedded YANG modules")
	defs := configyang.PathToArgDefs(loader)[pathUpdateDataKey]
	require.NotEmpty(t, defs, "the model declares the leaves of update data key")
	return defs
}

// updateArgs answers tokens judged against the shipped leaves of update data
// key, requiring the validator to accept them.
func updateArgs(t *testing.T, tokens ...string) command.ValidatedArgs {
	t.Helper()
	validated, err := command.ValidateArgs(tokens, updateDefs(t), nil)
	require.NoError(t, err)
	return validated
}

// looseUpdateArgs binds key and content to leaves that judge nothing about
// content, so a test reaches the handler's own refusals with a value the
// shipped leaf would refuse first.
func looseUpdateArgs(t *testing.T, key, content string) command.ValidatedArgs {
	t.Helper()
	defs := []command.ArgDef{
		commandtest.Must(command.NewStringArg(keywordKey, []command.UintRange{{Min: 1, Max: 512}}, nil,
			command.ArgOptions{Mandatory: true})),
		commandtest.Must(command.NewStringArg(keywordContent, nil, nil, command.ArgOptions{})),
	}
	validated, err := command.ValidateArgs([]string{keywordKey, key, keywordContent, content}, defs, nil)
	require.NoError(t, err)
	return validated
}

// updateCall runs the key write handler on validated.
func updateCall(t *testing.T, validated command.ValidatedArgs) *zePlugin.Response {
	t.Helper()
	response, err := handleDataKeyUpdate(&pluginserver.CommandContext{}, validated)
	require.NoError(t, err)
	return response
}

// TestUpdateKeyWritesDecodedContent proves the daemon write through its
// handler, on tokens the shipped leaves accepted: a key that does not exist is
// created and one that exists is replaced, each holding exactly the decoded
// bytes when read back from the live store, binary bytes included; the answer
// names the key and the byte count.
// VALIDATES: AC-7 (daemon half).
// PREVENTS: a write that stores the base64 text, keeps the old value, or
// answers a count it did not write.
func TestUpdateKeyWritesDecodedContent(t *testing.T) {
	store, _ := dataRPCStore(t, func(context.Context) error { return nil })
	for _, value := range [][]byte{[]byte("first value"), {0x00, 0xff, 0x10, '\n', 0x80}} {
		encoded := base64.StdEncoding.EncodeToString(value)
		response := updateCall(t, updateArgs(t, updateTestKey, keywordContent, encoded))
		require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
		data, ok := response.Data.(zePlugin.Map)
		require.True(t, ok)
		assert.Equal(t, updateTestKey, data[keywordKey])
		assert.Equal(t, len(value), data[keywordBytes])
		stored, err := store.ReadKey(updateTestKey)
		require.NoError(t, err)
		assert.Equal(t, value, stored, "the key holds exactly the decoded bytes")
	}
}

// TestUpdateKeyRefusesBadBase64 proves a content that is not base64 is
// refused naming the argument and the encoding and never the value: by the
// shipped leaf's pattern before the handler, and by the handler itself for a
// value that reached it undecodable. The key is not written.
// VALIDATES: AC-7 (survivor), AC-8 (a refusal that echoes an argument).
// PREVENTS: the base64 text of a credential printed in a refusal, or bytes
// that do not decode written as the key.
func TestUpdateKeyRefusesBadBase64(t *testing.T) {
	store, _ := dataRPCStore(t, func(context.Context) error { return nil })
	const bad = "bm90-YmFzZTY0"

	_, err := command.ValidateArgs([]string{updateTestKey, keywordContent, bad}, updateDefs(t), nil)
	require.Error(t, err, "the leaf's pattern refuses a value outside the base64 alphabet")
	assert.Contains(t, err.Error(), "content is base64")
	assert.NotContains(t, err.Error(), bad)

	response := updateCall(t, looseUpdateArgs(t, updateTestKey, "%%%%"))
	assert.Equal(t, zePlugin.StatusError, response.Status)
	assert.Contains(t, response.Error, "update data key: content is not base64")
	assert.Contains(t, response.Error, "fix: "+usageUpdateDataKey)
	assert.NotContains(t, response.Error, "%%%%")
	_, err = store.ReadKey(updateTestKey)
	assert.ErrorIs(t, err, fs.ErrNotExist, "nothing was written")
}

// TestUpdateKeyRefusesEmptyContent proves an empty content is refused: by the
// shipped leaf's length, and by the handler for a value that decodes to no
// byte, naming the removal of the key as the fix.
// VALIDATES: the Boundary table (0 bytes is invalid below).
// PREVENTS: a key emptied by a write where the operator meant to remove it.
func TestUpdateKeyRefusesEmptyContent(t *testing.T) {
	store, _ := dataRPCStore(t, func(context.Context) error { return nil })

	_, err := command.ValidateArgs([]string{updateTestKey, keywordContent, ""}, updateDefs(t), nil)
	require.Error(t, err, "the leaf refuses an empty content")

	response := updateCall(t, looseUpdateArgs(t, updateTestKey, ""))
	assert.Equal(t, zePlugin.StatusError, response.Status)
	assert.Contains(t, response.Error, "update data key: the content is empty")
	assert.Contains(t, response.Error, "fix: delete data key "+updateTestKey)
	_, err = store.ReadKey(updateTestKey)
	assert.ErrorIs(t, err, fs.ErrNotExist, "nothing was written")
}

// TestUpdateKeyContentBound proves the Boundary table through the shipped
// leaf: a content of 261,120 characters is accepted and written, 195,840
// bytes; one character more, and one byte past the validator's byte cap, are
// refused before the handler with the bound in bytes of content and the
// offline command for a larger value, never with the value and never with the
// generic byte count.
// VALIDATES: AC-12 (update data key), the Boundary table.
// PREVENTS: a bound refusal that says only "too long", or names a size to
// shorten to that the bound still refuses.
func TestUpdateKeyContentBound(t *testing.T) {
	store, _ := dataRPCStore(t, func(context.Context) error { return nil })
	defs := updateDefs(t)

	atBound := strings.Repeat("A", updateContentEncodedMax)
	response := updateCall(t, updateArgs(t, updateTestKey, keywordContent, atBound))
	require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
	stored, err := store.ReadKey(updateTestKey)
	require.NoError(t, err)
	assert.Len(t, stored, updateContentBytesMax)

	for name, content := range map[string]string{
		"one character over": atBound + "A",
		"past the byte cap":  strings.Repeat("A", 4*updateContentEncodedMax+1),
	} {
		_, err := command.ValidateArgs([]string{updateTestKey, keywordContent, content}, defs, nil)
		require.Error(t, err, name)
		text := err.Error()
		assert.Contains(t, text, "195,840 bytes", name)
		assert.Contains(t, text, "ze data write <key> <file> while ze is stopped", name)
		assert.Contains(t, text, redact.Placeholder, name)
		assert.NotContains(t, text, strings.Repeat("A", 64), name)
		assert.NotContains(t, text, "argument too long", name)
	}
}

// TestUpdateKeyRefusesConfigurationKey is the update half of AC-4: a key that
// is not runtime state is refused before the store is touched, naming the
// command that manages a config. No key the store held changes.
// VALIDATES: AC-4 (update).
// PREVENTS: a raw write of a config file or pointer that bypasses versions.
func TestUpdateKeyRefusesConfigurationKey(t *testing.T) {
	store, _ := dataRPCStore(t, func(context.Context) error { return nil })
	before, err := store.ListKeys("")
	require.NoError(t, err)

	for _, key := range []string{"file/active/ze.conf", "meta/config/ze.conf", "ze.conf"} {
		response := updateCall(t, updateArgs(t, key, keywordContent, "c3RhdGU="))
		assert.Equal(t, zePlugin.StatusError, response.Status, key)
		assert.Contains(t, response.Error, "update data key: "+key+" is configuration", key)
		assert.Contains(t, response.Error, "fix: update data config name <name> content <base64>", key)
	}

	after, err := store.ListKeys("")
	require.NoError(t, err)
	assert.Equal(t, before, after, "no key was written")
	for _, key := range before {
		_, err := store.ReadKey(key)
		require.NoError(t, err, key)
	}
}

// TestUpdateKeyRefusesKeyOfRunningPlugin is the update half of AC-5: with the
// grants and the plugin snapshot stood in, a key whose granted plugin runs is
// refused and keeps its value, naming the plugin and its restart; the same key
// with its owner stopped is written.
// VALIDATES: AC-5 (update).
// PREVENTS: runtime state rewritten under the plugin that manages it.
func TestUpdateKeyRefusesKeyOfRunningPlugin(t *testing.T) {
	granted := map[string][]string{updateTestKey: {"test-owner"}}
	running := pluginserver.DoctorSnapshot{Plugins: []pluginserver.PluginDoctorState{
		{Name: "test-owner", RegistrationName: "test-owner", Configured: true, HasProcess: true, Running: true},
	}}
	stopped := pluginserver.DoctorSnapshot{Plugins: []pluginserver.PluginDoctorState{
		{Name: "test-owner", RegistrationName: "test-owner", Configured: true, HasProcess: true, Running: false, Down: true},
	}}
	encoded := base64.StdEncoding.EncodeToString([]byte("new state"))

	t.Run("owner runs", func(t *testing.T) {
		store, _ := dataRPCStore(t, func(context.Context) error { return nil })
		standInPluginState(t, granted, running, nil)
		require.NoError(t, store.WriteKey(updateTestKey, []byte("live state")))

		response := updateCall(t, updateArgs(t, updateTestKey, keywordContent, encoded))
		assert.Equal(t, zePlugin.StatusError, response.Status)
		assert.Contains(t, response.Error, "update data key: plugin test-owner manages "+updateTestKey+" while it runs")
		assert.Contains(t, response.Error, "fix: ze request plugin restart test-owner")
		stored, err := store.ReadKey(updateTestKey)
		require.NoError(t, err)
		assert.Equal(t, "live state", string(stored), "the key is unchanged")
	})

	t.Run("owner stopped", func(t *testing.T) {
		store, _ := dataRPCStore(t, func(context.Context) error { return nil })
		standInPluginState(t, granted, stopped, nil)
		require.NoError(t, store.WriteKey(updateTestKey, []byte("stale state")))

		response := updateCall(t, updateArgs(t, updateTestKey, keywordContent, encoded))
		require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
		stored, err := store.ReadKey(updateTestKey)
		require.NoError(t, err)
		assert.Equal(t, "new state", string(stored))
	})
}

// TestUpdateKeyRefusesWithNoStore is the update half of R-10: while the
// daemon publishes no store, the write refuses and names the offline command.
// VALIDATES: R-10 (update).
// PREVENTS: a nil dereference for a write that arrives during shutdown.
func TestUpdateKeyRefusesWithNoStore(t *testing.T) {
	InstallDataRPC(nil)

	response := updateCall(t, updateArgs(t, updateTestKey, keywordContent, "c3RhdGU="))
	assert.Equal(t, zePlugin.StatusError, response.Status)
	assert.Contains(t, response.Error, "update data key: this daemon serves no store")
	assert.Contains(t, response.Error, "fix: ze data write <key> <file>")
}

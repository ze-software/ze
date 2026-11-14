package cli

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/config/storage"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/pkg/zefs"
)

// The configs the import tests name: the daemon's own (dataRPCStore serves
// ze.conf, holding importActiveText) and another the daemon does not run.
const (
	importOwnName    = "ze.conf"
	importOtherName  = "other.conf"
	importActiveText = "current"
	importedText     = "imported"
	importOtherFirst = "other first"
)

// configDefs answers the leaves the shipped model declares for update data
// config, as the dispatcher's validator reads them.
func configDefs(t *testing.T) []command.ArgDef {
	t.Helper()
	loader, err := configyang.DefaultLoader()
	require.NoError(t, err, "load the embedded YANG modules")
	defs := configyang.PathToArgDefs(loader)[pathUpdateDataConfig]
	require.NotEmpty(t, defs, "the model declares the leaves of update data config")
	return defs
}

// configArgs answers `name <name> content <base64 of text>` judged against the
// shipped leaves, requiring the validator to accept them.
func configArgs(t *testing.T, name, text string) command.ValidatedArgs {
	t.Helper()
	tokens := []string{keywordName, name, keywordContent, base64.StdEncoding.EncodeToString([]byte(text))}
	validated, err := command.ValidateArgs(tokens, configDefs(t), nil)
	require.NoError(t, err)
	return validated
}

// looseConfigArgs binds name and content to leaves that judge nothing about
// either, so a test reaches the handler's own refusals with a value the
// shipped leaves would refuse first.
func looseConfigArgs(t *testing.T, name, content string) command.ValidatedArgs {
	t.Helper()
	defs := []command.ArgDef{
		commandtest.Must(command.NewStringArg(keywordName, nil, nil, command.ArgOptions{})),
		commandtest.Must(command.NewStringArg(keywordContent, nil, nil, command.ArgOptions{})),
	}
	validated, err := command.ValidateArgs([]string{keywordName, name, keywordContent, content}, defs, nil)
	require.NoError(t, err)
	return validated
}

// configCall runs the config import handler on validated.
func configCall(t *testing.T, validated command.ValidatedArgs) *zePlugin.Response {
	t.Helper()
	response, err := handleDataConfigUpdate(&pluginserver.CommandContext{}, validated)
	require.NoError(t, err)
	return response
}

// requireActive requires the active config of name to read text.
func requireActive(t *testing.T, store storage.Storage, name, text string) {
	t.Helper()
	active, err := storage.ReadActiveConfig(store, name)
	require.NoError(t, err)
	assert.Equal(t, text, string(active))
}

// TestUpdateConfigStagesAndReloads proves the import of the daemon's own
// config through the handler: the content is staged as the candidate while
// the previous config is still active, the reload runs and promotes it, and
// the answer carries the name, a version stamp and the byte count; the
// active config then reads the imported bytes.
// VALIDATES: AC-10 (own config).
// PREVENTS: an import that promotes before the reload accepts, writes only the
// file/active mirror its active pointer shadows, or answers no version.
func TestUpdateConfigStagesAndReloads(t *testing.T) {
	var store storage.Storage
	var configPath string
	reloaded := false
	store, configPath = dataRPCStore(t, func(context.Context) error {
		candidate, _, staged, err := storage.ReadCandidateConfig(store, configPath)
		require.NoError(t, err)
		require.True(t, staged, "the reload runs with the candidate staged")
		assert.Equal(t, importedText, string(candidate))
		requireActive(t, store, configPath, importActiveText)
		reloaded = true
		return storage.PromoteCandidate(store, configPath)
	})

	response := configCall(t, configArgs(t, importOwnName, importedText))
	require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
	assert.True(t, reloaded, "the import of the daemon's own config runs the reload")
	data, ok := response.Data.(zePlugin.Map)
	require.True(t, ok)
	assert.Equal(t, importOwnName, data[keywordName])
	assert.NotEmpty(t, data[answerVersion])
	assert.Equal(t, len(importedText), data[keywordBytes])
	requireActive(t, store, configPath, importedText)
}

// TestUpdateConfigRejectedLeavesPointers proves a config the reload refuses
// leaves the active config as it was, no candidate staged, and a refusal that
// carries the reload's reason and says nothing changed.
// VALIDATES: AC-10 (a refused reload), R-9.
// PREVENTS: a refused import reported as success, or a config left active or
// staged that the reload refused.
func TestUpdateConfigRejectedLeavesPointers(t *testing.T) {
	var store storage.Storage
	var configPath string
	store, configPath = dataRPCStore(t, func(context.Context) error {
		require.NoError(t, storage.ClearCandidate(store, configPath))
		return errors.New("verify refused the peer")
	})

	response := configCall(t, configArgs(t, importOwnName, importedText))
	assert.Equal(t, zePlugin.StatusError, response.Status)
	assert.Contains(t, response.Error, "update data config: ze.conf was not imported")
	assert.Contains(t, response.Error, "verify refused the peer")
	assert.Contains(t, response.Error, "the active config, its pointers and the running daemon are unchanged")
	requireActive(t, store, configPath, importActiveText)
	_, _, staged, err := storage.ReadCandidateConfig(store, configPath)
	require.NoError(t, err)
	assert.False(t, staged, "no candidate is left staged")
}

// TestUpdateConfigRefusesWhenChangeStaged proves an import over a change
// somebody else staged is refused naming commit or discard, for the daemon's
// own config and for another, and leaves the staged change and the active
// config as they were, without a reload.
// VALIDATES: AC-10 (survivor: a change already staged).
// PREVENTS: an import that discards an edit somebody staged.
func TestUpdateConfigRefusesWhenChangeStaged(t *testing.T) {
	reloaded := false
	store, configPath := dataRPCStore(t, func(context.Context) error {
		reloaded = true
		return nil
	})
	_, err := storage.WriteCandidateVersion(store, configPath, []byte("staged"), time.Now())
	require.NoError(t, err)
	_, err = storage.RestoreConfig(store, importOtherName, []byte(importOtherFirst))
	require.NoError(t, err)
	_, err = storage.WriteCandidateVersion(store, importOtherName, []byte("other staged"), time.Now())
	require.NoError(t, err)

	for _, name := range []string{importOwnName, importOtherName} {
		response := configCall(t, configArgs(t, name, importedText))
		assert.Equal(t, zePlugin.StatusError, response.Status, name)
		assert.Contains(t, response.Error, "commit or discard it first", name)
		assert.Contains(t, response.Error, "fix: "+fixEditStagedChange, name)
	}
	assert.False(t, reloaded, "a refused import runs no reload")
	requireActive(t, store, configPath, importActiveText)
	requireActive(t, store, importOtherName, importOtherFirst)
	candidate, _, staged, err := storage.ReadCandidateConfig(store, configPath)
	require.NoError(t, err)
	require.True(t, staged)
	assert.Equal(t, "staged", string(candidate), "the staged change is kept")
}

// TestUpdateConfigOtherNameCommitsVersion proves a name that is not the
// daemon's config is committed as a new active version with no reload, and a
// second import of it keeps the first as rollback.
// VALIDATES: AC-10 (another name).
// PREVENTS: an import of another config that reloads the daemon, or a raw
// write its active pointer shadows.
func TestUpdateConfigOtherNameCommitsVersion(t *testing.T) {
	reloaded := false
	store, configPath := dataRPCStore(t, func(context.Context) error {
		reloaded = true
		return nil
	})

	for _, text := range []string{importOtherFirst, "other second"} {
		response := configCall(t, configArgs(t, importOtherName, text))
		require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
		data, ok := response.Data.(zePlugin.Map)
		require.True(t, ok)
		assert.Equal(t, importOtherName, data[keywordName])
		assert.NotEmpty(t, data[answerVersion])
		requireActive(t, store, importOtherName, text)
	}
	assert.False(t, reloaded, "another config imports with no reload")
	requireActive(t, store, configPath, importActiveText)
	_, err := store.ReadKey(zefs.KeyConfigRollback.Key(importOtherName))
	require.NoError(t, err, "the first import is kept as rollback")
}

// TestUpdateConfigClientConfigReachesAServedClientOnly proves the import of a
// managed client's config (client-<name>.conf) on the three daemons that can
// receive one: a daemon that serves no managed client and a hub without that
// client entry both refuse, naming the client, write no key and push nothing;
// a hub that serves the client commits the version with no reload, and the
// write observer reports the client, which is what pushes config-changed.
// VALIDATES: AC-10 (another name, a managed client's config).
// PREVENTS: a client config imported as success that no client ever fetches,
// where request data restore ... client refuses the same case.
func TestUpdateConfigClientConfigReachesAServedClientOnly(t *testing.T) {
	reloads := 0
	reload := func(context.Context) error {
		reloads++
		return nil
	}
	store, configPath := dataRPCStore(t, reload)
	var pushed []string
	store.SetWriteObserver(func(key string) {
		if name, ok := pluginserver.ClientNameFromConfigKey(key); ok {
			pushed = append(pushed, name)
		}
	})
	const clientConfig = "client-edge.conf"

	response := configCall(t, configArgs(t, clientConfig, importedText))
	assert.Equal(t, zePlugin.StatusError, response.Status)
	assert.Contains(t, response.Error, "update data config: client-edge.conf was not imported")
	assert.Contains(t, response.Error, "this daemon serves no managed client")
	assert.Contains(t, response.Error, "fix: "+fixAddHubClient)

	InstallDataRPC(&DataRPCTarget{Store: store, ConfigPath: configPath, Reload: reload,
		ServesClient: func(name string) bool { return name == "core" }})
	response = configCall(t, configArgs(t, clientConfig, importedText))
	assert.Equal(t, zePlugin.StatusError, response.Status)
	assert.Contains(t, response.Error, "no client entry named edge")
	assert.False(t, store.Exists(clientConfig), "a refused client import writes no config")
	assert.Empty(t, pushed, "a refused client import pushes nothing")

	InstallDataRPC(&DataRPCTarget{Store: store, ConfigPath: configPath, Reload: reload,
		ServesClient: func(name string) bool { return name == "edge" }})
	response = configCall(t, configArgs(t, clientConfig, importedText))
	require.Equal(t, zePlugin.StatusDone, response.Status, response.Error)
	requireActive(t, store, clientConfig, importedText)
	assert.Contains(t, pushed, "edge", "the write observer hears the served client's config")
	assert.Zero(t, reloads, "a client config import MUST NOT reload the hub")
	requireActive(t, store, configPath, importActiveText)
}

// TestUpdateConfigRefusesBadName proves a name that is not a plain config name
// is refused before the store is touched, naming the rule, whatever the
// validator let through.
// VALIDATES: AC-10 (survivor: a path, . and ..).
// PREVENTS: an import that writes outside the config namespace.
func TestUpdateConfigRefusesBadName(t *testing.T) {
	store, _ := dataRPCStore(t, func(context.Context) error { return nil })
	before, err := store.ListKeys("")
	require.NoError(t, err)

	for _, name := range []string{".", "..", "../ze.conf", "etc/ze.conf", `dir\ze.conf`, "-"} {
		response := configCall(t, configArgs(t, name, importedText))
		assert.Equal(t, zePlugin.StatusError, response.Status, name)
		assert.Contains(t, response.Error, "is not a plain config name", name)
		assert.Contains(t, response.Error, "fix: "+usageUpdateDataConfig, name)
	}
	after, err := store.ListKeys("")
	require.NoError(t, err)
	assert.Equal(t, before, after, "no key was written")
}

// TestUpdateConfigRefusesBadBase64 proves a content that is not base64 is
// refused naming the argument and the encoding and never the value: by the
// shipped leaf's pattern before the handler, and by the handler for a value
// that reached it undecodable. No config changes.
// VALIDATES: AC-10, AC-8 (a refusal that echoes an argument).
// PREVENTS: the text of a config printed in a refusal, or bytes that do not
// decode imported as a config.
func TestUpdateConfigRefusesBadBase64(t *testing.T) {
	store, configPath := dataRPCStore(t, func(context.Context) error { return nil })
	const bad = "bm90-YmFzZTY0"

	_, err := command.ValidateArgs([]string{keywordName, importOwnName, keywordContent, bad}, configDefs(t), nil)
	require.Error(t, err, "the leaf's pattern refuses a value outside the base64 alphabet")
	assert.Contains(t, err.Error(), "content is base64")
	assert.NotContains(t, err.Error(), bad)

	response := configCall(t, looseConfigArgs(t, importOwnName, "%%%%"))
	assert.Equal(t, zePlugin.StatusError, response.Status)
	assert.Contains(t, response.Error, "update data config: content is not base64")
	assert.Contains(t, response.Error, "fix: "+usageUpdateDataConfig)
	assert.NotContains(t, response.Error, "%%%%")
	requireActive(t, store, configPath, importActiveText)
}

// TestUpdateConfigContentBound proves AC-12 for a config through the shipped
// leaf: 261,120 characters are accepted; one more is refused before the
// handler with the bound in bytes and the restore route for a larger config,
// never with the value.
// VALIDATES: AC-12 (update data config).
// PREVENTS: a bound refusal that names no route for the config it refuses.
func TestUpdateConfigContentBound(t *testing.T) {
	defs := configDefs(t)
	atBound := strings.Repeat("A", updateContentEncodedMax)
	_, err := command.ValidateArgs([]string{keywordName, importOwnName, keywordContent, atBound}, defs, nil)
	require.NoError(t, err, "a content at the bound is accepted")

	_, err = command.ValidateArgs([]string{keywordName, importOwnName, keywordContent, atBound + "A"}, defs, nil)
	require.Error(t, err)
	text := err.Error()
	assert.Contains(t, text, "195,840 bytes")
	assert.Contains(t, text, "request data restore path <file> config")
	assert.Contains(t, text, redact.Placeholder)
	assert.NotContains(t, text, strings.Repeat("A", 64))
}

// TestUpdateConfigRefusesWithNoStore is the import half of R-10: while the
// daemon publishes no store, the import refuses and names the local command.
// VALIDATES: R-10 (update data config).
// PREVENTS: a nil dereference for an import that arrives during shutdown.
func TestUpdateConfigRefusesWithNoStore(t *testing.T) {
	InstallDataRPC(nil)

	response := configCall(t, configArgs(t, importOwnName, importedText))
	assert.Equal(t, zePlugin.StatusError, response.Status)
	assert.Contains(t, response.Error, "update data config: this daemon serves no store")
	assert.Contains(t, response.Error, "fix: ze data import <file>")
}

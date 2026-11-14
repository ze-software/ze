// Design: docs/architecture/hub-architecture.md -- request data RPCs beside request reload.
// Related: register_data_rpc.go -- registers the handlers.
// Related: cmd/ze/hub/main.go -- the daemon installs the target, with the reload a SIGHUP runs (main_reload.go).

package cli

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// DataRPCTarget is what the request data RPCs act on: the daemon's own live
// store, the config it serves, the reload that accepts and promotes a staged
// candidate (the SIGHUP chain), and the managed clients it serves as a hub.
type DataRPCTarget struct {
	Store      storage.Storage
	ConfigPath string
	Reload     func(context.Context) error
	// ServesClient reports whether this hub serves the named managed client. It
	// is nil when the daemon serves no managed client at all.
	ServesClient func(name string) bool
	// Window returns the daemon's confirmed-commit window, nil when it has
	// none. A config restore is refused while one is open, because the
	// window's revert would wipe it (confirm.WriteOutside). Window itself is
	// nil when the daemon runs no window at all.
	Window func() *confirm.Window
}

// dataRPC holds the target once the daemon has built its reload. Until then,
// and in a daemon with no store, it is nil and both RPCs refuse.
var dataRPC atomic.Pointer[DataRPCTarget]

// InstallDataRPC publishes the target the RPCs act on. The daemon MUST call it
// after its reload function exists, and MUST call it with nil before it closes
// the store.
func InstallDataRPC(target *DataRPCTarget) { dataRPC.Store(target) }

// keywordPath names the artifact path keyword of the backup and restore RPCs,
// and the answer key that echoes it. keywordClient does the same for the
// restore's client target, keywordName names the source config a config restore
// reads, and keywordKey names the store key leaf of `delete data key` and the
// answer key that echoes it, and the key column of every answer row that names
// a store key. keywordContent names the base64 content leaf of `update data
// key`, and keywordBytes the answer key that counts the bytes a command wrote.
const (
	keywordPath    = "path"
	keywordClient  = "client"
	keywordName    = "name"
	keywordKey     = "key"
	keywordContent = "content"
	keywordBytes   = "bytes"
)

// pathUpdateDataKey is the command `update data key` is typed as, which its
// refusals name first and the local `ze data write` sends, and
// usageUpdateDataKey the whole command, the fix of a refusal that says what to
// type.
const (
	pathUpdateDataKey  = "update data key"
	usageUpdateDataKey = "update data key <key> content <base64>"
)

// findingSeverity, findingCheck, findingKind and findingDetail are answer keys
// of one integrity finding row: how bad it is, the check that found it, its
// kind and what it found. `show data check` answers its rows with all four and
// orders its columns by them, and `request data repair` answers its history
// findings with the same kind and detail.
const (
	findingSeverity = "severity"
	findingCheck    = "check"
	findingKind     = "kind"
	findingDetail   = "detail"
)

// dataBackupArgs is one parsed `request data backup` line.
type dataBackupArgs struct {
	path  string
	spare int
	force bool
}

// handleDataBackup answers `request data backup path <abs> [spare <n>] [force]`.
func handleDataBackup(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*zePlugin.Response, error) {
	args := validated.Tokens()
	target := dataRPC.Load()
	if target == nil {
		return dataRefusal(errors.New("request data backup: this daemon serves no store")), nil
	}
	parsed, err := parseDataBackupArgs(args)
	if err != nil {
		return dataRefusal(fmt.Errorf("request data backup: %w", err)), nil
	}
	if err := storage.CheckArtifactPath(parsed.path); err != nil {
		return dataRefusal(fmt.Errorf("request data backup: %w", err)), nil
	}
	result, err := storage.Backup(target.Store, parsed.path, parsed.force, zefs.Spare(parsed.spare))
	if err != nil {
		return dataRefusal(fmt.Errorf("request data backup: %w", err)), nil
	}
	return &zePlugin.Response{
		Status: zePlugin.StatusDone,
		Data: zePlugin.Map{
			keywordPath:  result.Path,
			"keys":       result.Keys,
			keywordBytes: result.Bytes,
			"warning":    "the file holds credentials and private keys; it is mode 0600",
		},
	}, nil
}

// parseDataBackupArgs reads `path <abs> [spare <n>] [force]`, keywords in any
// order, each at most once.
func parseDataBackupArgs(args []string) (dataBackupArgs, error) {
	parsed := dataBackupArgs{}
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		keyword := args[i]
		if seen[keyword] {
			return dataBackupArgs{}, fmt.Errorf("%s given twice", keyword)
		}
		seen[keyword] = true
		if keyword == "force" {
			parsed.force = true
			continue
		}
		if keyword != keywordPath && keyword != "spare" {
			return dataBackupArgs{}, fmt.Errorf("unknown keyword %q: the keywords are path, spare and force", keyword)
		}
		if i+1 >= len(args) {
			return dataBackupArgs{}, fmt.Errorf("%s needs a value", keyword)
		}
		i++
		if keyword == keywordPath {
			parsed.path = args[i]
			continue
		}
		spare, err := strconv.Atoi(args[i])
		if err != nil {
			return dataBackupArgs{}, fmt.Errorf("spare %q is not an integer in the range 0 to 100", args[i])
		}
		parsed.spare = spare
	}
	if parsed.path == "" {
		return dataBackupArgs{}, errors.New("path <absolute-file> is required")
	}
	return parsed, nil
}

// dataRestoreArgs is one parsed `request data restore` line.
type dataRestoreArgs struct {
	path       string
	sourceName string
	client     string
}

// handleDataRestore answers `request data restore path <abs> config [name <n>]
// [client <c>]`. Without client, the artifact's config is staged as the
// candidate FIRST and promoted LAST, by the same reload a SIGHUP runs, so a
// config the reload refuses never becomes active and the active pointer and its
// rollback stay as they were. It is refused while a confirmed-commit window is
// open, with the window's own refusal naming its owner. With client, see
// restoreClientConfig: the client's config is not the one a window reverts.
func handleDataRestore(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*zePlugin.Response, error) {
	args := validated.Tokens()
	target := dataRPC.Load()
	if target == nil {
		return dataRefusal(errors.New("request data restore: this daemon serves no store")), nil
	}
	parsed, err := parseDataRestoreArgs(args)
	if err != nil {
		return dataRefusal(fmt.Errorf("request data restore: %w", err)), nil
	}
	if err := storage.CheckArtifactPath(parsed.path); err != nil {
		return dataRefusal(fmt.Errorf("request data restore: %w", err)), nil
	}
	if parsed.client != "" {
		return restoreClientConfig(target, parsed), nil
	}
	deviceName := filepath.Base(target.ConfigPath)
	selected, err := storage.ReadRestoreSource(parsed.path, parsed.sourceName, deviceName)
	if err != nil {
		return dataRefusal(fmt.Errorf("request data restore: %w", err)), nil
	}
	var window *confirm.Window
	if target.Window != nil {
		window = target.Window()
	}
	var stamp string
	writeErr := confirm.WriteOutside(window, func() error {
		var stageErr error
		stamp, stageErr = restoreDaemonConfig(ctx.Context(), target, selected.Data)
		return stageErr
	})
	if writeErr != nil {
		return dataRefusal(fmt.Errorf("request data restore: %w", writeErr)), nil
	}
	return &zePlugin.Response{
		Status: zePlugin.StatusDone,
		Data: zePlugin.Map{
			keywordPath:   parsed.path,
			"source-name": selected.Name,
			"config-name": deviceName,
			answerVersion: stamp,
		},
	}, nil
}

// errConfigChangeStaged is restoreDaemonConfig's refusal of a stage over a
// candidate somebody else staged, so a caller can tell it from a reload that
// refused the config.
var errConfigChangeStaged = errors.New("a config change is already staged; commit or discard it first")

// restoreDaemonConfig stages data as the daemon's candidate and promotes it by
// the reload, returning the version stamp. It refuses with
// errConfigChangeStaged when another change is already staged.
// `request data restore ... config` and `update data config` for the daemon's
// own config both take this path.
func restoreDaemonConfig(ctx context.Context, target *DataRPCTarget, data []byte) (string, error) {
	stamp, err := storage.WriteCandidateVersion(target.Store, target.ConfigPath, data, time.Now())
	if errors.Is(err, storage.ErrCandidateExists) {
		return "", errConfigChangeStaged
	}
	if err != nil {
		return "", fmt.Errorf("stage candidate: %w", err)
	}
	if err := target.Reload(ctx); err != nil {
		return "", fmt.Errorf("the reload refused the config, the active config is unchanged: %w", err)
	}
	return stamp, nil
}

// restoreClientConfig writes the artifact's config as the config this hub serves
// to one managed client, file/active/client-<name>.conf, as a new version
// promoted under the store guard. The hub does NOT reload: the config is the
// client's, not the hub's. The storage write observer sees the key and pushes
// config-changed to the client, which fetches the new config and applies it
// through its own reload. A daemon that is not a hub serving that client
// refuses, because the key would be written and never served.
func restoreClientConfig(target *DataRPCTarget, parsed dataRestoreArgs) *zePlugin.Response {
	if target.ServesClient == nil {
		return dataRefusal(fmt.Errorf("request data restore: client %s: this daemon serves no managed client; only a hub with a client entry under plugin hub server serves one", parsed.client))
	}
	if !target.ServesClient(parsed.client) {
		return dataRefusal(fmt.Errorf("request data restore: client %s: this hub has no client entry named %s under plugin hub server", parsed.client, parsed.client))
	}
	clientName := pluginserver.ClientConfigKey(parsed.client)
	selected, err := storage.ReadRestoreSource(parsed.path, parsed.sourceName, clientName)
	if err != nil {
		return dataRefusal(fmt.Errorf("request data restore: %w", err))
	}
	stamp, err := storage.RestoreConfig(target.Store, clientName, selected.Data)
	if errors.Is(err, storage.ErrCandidateExists) {
		return dataRefusal(fmt.Errorf("request data restore: a change to %s is already staged; commit or discard it first", clientName))
	}
	if err != nil {
		return dataRefusal(fmt.Errorf("request data restore: %w", err))
	}
	return &zePlugin.Response{
		Status: zePlugin.StatusDone,
		Data: zePlugin.Map{
			keywordPath:   parsed.path,
			"source-name": selected.Name,
			"config-name": clientName,
			keywordClient: parsed.client,
			answerVersion: stamp,
		},
	}
}

// parseDataRestoreArgs reads `path <abs> config [name <n>] [client <c>]`. The config
// keyword is required: a full restore replaces the store under the daemon,
// so it runs offline only.
func parseDataRestoreArgs(args []string) (dataRestoreArgs, error) {
	parsed := dataRestoreArgs{}
	configMode := false
	for i := 0; i < len(args); i++ {
		keyword := args[i]
		if keyword == "config" {
			configMode = true
			continue
		}
		if keyword != keywordPath && keyword != keywordName && keyword != keywordClient {
			return dataRestoreArgs{}, fmt.Errorf("unknown keyword %q: the keywords are path, config, name and client", keyword)
		}
		if i+1 >= len(args) {
			return dataRestoreArgs{}, fmt.Errorf("%s needs a value", keyword)
		}
		i++
		switch keyword {
		case keywordPath:
			parsed.path = args[i]
		case keywordName:
			parsed.sourceName = args[i]
		case keywordClient:
			parsed.client = args[i]
		}
	}
	if parsed.path == "" {
		return dataRestoreArgs{}, errors.New("path <absolute-file> is required")
	}
	if !configMode {
		return dataRestoreArgs{}, errors.New("the config keyword is required: a live restore replaces the config only")
	}
	return parsed, nil
}

// handleDataKeyDelete answers `delete data key <key>`. It removes one
// runtime-state key through the store handle the daemon owns: opening the store
// afresh here would be a second writer, which the owner lock refuses. Every
// refusal runs before the store is touched. The answer names the key removed.
func handleDataKeyDelete(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*zePlugin.Response, error) {
	target := dataRPC.Load()
	if target == nil {
		return zePlugin.OperatorErrorResponse(opererr.New(
			"delete data key: this daemon serves no store",
			"the daemon publishes its store to this command only while the store is open, so it is starting, stopping, or runs with no store",
			"ze data rm <key>")), nil
	}
	key, found := validated.Value(keywordKey)
	if !found {
		return zePlugin.OperatorErrorResponse(opererr.New(
			"delete data key: no key was given",
			"the command removes exactly one store key, the value typed after key",
			"delete data key <key>")), nil
	}
	if !storage.RuntimeKey(key) {
		return zePlugin.OperatorErrorResponse(configurationKeyError("delete data key", key)), nil
	}
	if refusal := runningOwnerError("delete data key", key); refusal != nil {
		return zePlugin.OperatorErrorResponse(refusal), nil
	}
	if err := target.Store.RemoveKey(key); err != nil {
		return zePlugin.OperatorErrorResponse(removeKeyError(key, err)), nil
	}
	return &zePlugin.Response{
		Status: zePlugin.StatusDone,
		Data:   zePlugin.Map{keywordKey: key},
	}, nil
}

// handleDataKeyUpdate answers `update data key <key> content <base64>`. It
// writes one runtime-state key, creating it or replacing it, through the store
// handle the daemon owns, as handleDataKeyDelete removes one. The validator has
// already judged content against its leaf (the length and the base64 pattern),
// and the handler decodes it again rather than trust that: a decode it cannot
// do is refused. The decoded bytes are the key's value and nothing else, never
// a path. Every refusal runs before the store is touched, and none quotes the
// content. The answer names the key and the bytes written.
func handleDataKeyUpdate(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*zePlugin.Response, error) {
	target := dataRPC.Load()
	if target == nil {
		return zePlugin.OperatorErrorResponse(opererr.New(
			pathUpdateDataKey+": this daemon serves no store",
			"the daemon publishes its store to this command only while the store is open, so it is starting, stopping, or runs with no store",
			"ze data write <key> <file>")), nil
	}
	key, found := validated.Value(keywordKey)
	if !found {
		return zePlugin.OperatorErrorResponse(opererr.New(
			pathUpdateDataKey+": no key was given",
			"the command writes exactly one store key, the value typed after key",
			usageUpdateDataKey)), nil
	}
	encoded, found := validated.Value(keywordContent)
	if !found {
		return zePlugin.OperatorErrorResponse(opererr.New(
			pathUpdateDataKey+": no content was given",
			"the command writes the bytes typed after content, base64-encoded",
			usageUpdateDataKey)), nil
	}
	if !storage.RuntimeKey(key) {
		return zePlugin.OperatorErrorResponse(configurationKeyError(pathUpdateDataKey, key)), nil
	}
	if refusal := runningOwnerError(pathUpdateDataKey, key); refusal != nil {
		return zePlugin.OperatorErrorResponse(refusal), nil
	}
	content, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// The decoder's error names a byte offset, never the input.
		return zePlugin.OperatorErrorResponse(opererr.Wrap(err,
			pathUpdateDataKey+": content is not base64",
			"content carries the key's bytes in base64 (RFC 4648 Section 4, with = padding), the encoding ze data write sends a file in, so the key is unchanged",
			usageUpdateDataKey)), nil
	}
	if len(content) == 0 {
		var fix textbuf.Buffer
		fix.Str("delete data key ").Str(configtext.QuoteCommandWord(key))
		return zePlugin.OperatorErrorResponse(opererr.New(
			pathUpdateDataKey+": the content is empty",
			"a key is written with at least one byte; a key that should hold nothing is removed, so "+key+" is unchanged",
			fix.String())), nil
	}
	if err := target.Store.WriteKey(key, content); err != nil {
		return zePlugin.OperatorErrorResponse(opererr.Wrap(err,
			pathUpdateDataKey+": the store refused to write "+key,
			"the key holds what it held before the command",
			"ze doctor")), nil
	}
	return &zePlugin.Response{
		Status: zePlugin.StatusDone,
		Data:   zePlugin.Map{keywordKey: key, keywordBytes: len(content)},
	}, nil
}

// configurationKeyError refuses a key the daemon's key commands do not reach:
// any key storage.RuntimeKey does not call runtime state. commandPath is the
// command the operator typed, which the refusal names first.
func configurationKeyError(commandPath, key string) *opererr.Error {
	return opererr.New(
		commandPath+": "+key+" is configuration, not runtime state",
		commandPath+" reaches runtime state only, a key under meta/ outside meta/config/, because a raw change to a configuration key bypasses its versions, pointers and history",
		usageUpdateDataConfig)
}

// keyOwners answers the plugin process names granted a store key, and
// pluginStates answers the running plugin server's state. They are variables
// only so a test can stand in for the grants and the server; the daemon reads
// the real registries. Both answer value types, so the guard below holds no
// handle on a plugin and works the same for a plugin that runs in another
// process.
var (
	keyOwners    = statestore.PluginKeyOwners
	pluginStates = pluginserver.PluginDoctorSnapshot
)

// runningOwnerError refuses a change to a key while a plugin granted that key
// runs: the plugin manages the key while it runs, so a change under it leaves
// the plugin's view and the store disagreeing. The owner comes from the key
// grants and the running state from the plugin snapshot, both by the process
// name the daemon configured, which is the identity a plugin's state calls
// carry, so the guard names no plugin, key or feature. commandPath is the
// command the operator typed, which the refusal names first.
//
// A nil answer means the key may change: no plugin holds a grant for it, or no
// plugin holding one runs. A key no plugin is granted needs no snapshot, so it
// is never refused because the plugin state is unavailable. A key with an owner
// is refused while the snapshot is unavailable (plugins not started yet, or a
// server that is stopping), because nothing then says whether the owner runs.
func runningOwnerError(commandPath, key string) *opererr.Error {
	owners := keyOwners(key)
	if len(owners) == 0 {
		return nil
	}
	snapshot, err := pluginStates()
	if err != nil {
		return opererr.Wrap(err,
			commandPath+": cannot tell yet whether the plugin that owns "+key+" runs",
			"the key is granted to "+strings.Join(owners, ", ")+", and the daemon has not started its plugins yet or is stopping them, so the key is unchanged",
			"ze show health")
	}
	for i := range snapshot.Plugins {
		state := &snapshot.Plugins[i]
		if !state.Running {
			continue
		}
		if slices.Contains(owners, state.Name) {
			return opererr.New(
				commandPath+": plugin "+state.Name+" manages "+key+" while it runs",
				"a change made under the running plugin leaves its own view of the key and the store disagreeing, so the key is unchanged; a restart makes the plugin take up its own state again",
				process.RestartCommand(state.Name))
		}
	}
	return nil
}

// removeKeyError explains a removal the store refused. A key the store does
// not hold is named as such, as the local ze data rm reports it, and is never
// answered as a success.
func removeKeyError(key string, err error) *opererr.Error {
	if errors.Is(err, fs.ErrNotExist) {
		return opererr.Wrap(err,
			"delete data key: the store holds no key "+key,
			"nothing was removed: the key was already removed, or its name is mistyped",
			"show data list")
	}
	return opererr.Wrap(err,
		"delete data key: the store refused to remove "+key,
		"the key is unchanged",
		"ze doctor")
}

// dataRefusal is the error answer the backup and restore RPCs give, carrying
// the reason.
func dataRefusal(err error) *zePlugin.Response {
	return &zePlugin.Response{Status: zePlugin.StatusError, Error: err.Error()}
}

// Design: docs/architecture/hub-architecture.md -- request data RPCs beside request reload.
// Related: register_data_rpc.go -- registers the two handlers.
// Related: cmd/ze/hub/main.go -- the daemon installs the target, with the reload a SIGHUP runs (main_reload.go).

package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
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

// keywordPath names the artifact path keyword of both RPCs, and the answer key
// that echoes it. keywordClient does the same for the restore's client target, and
// keywordName names the source config a config restore reads.
const (
	keywordPath   = "path"
	keywordClient = "client"
	keywordName   = "name"
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
			keywordPath: result.Path,
			"keys":      result.Keys,
			"bytes":     result.Bytes,
			"warning":   "the file holds credentials and private keys; it is mode 0600",
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
			"version":     stamp,
		},
	}, nil
}

// restoreDaemonConfig stages data as the daemon's candidate and promotes it by
// the reload, returning the version stamp. It refuses when another change is
// already staged.
func restoreDaemonConfig(ctx context.Context, target *DataRPCTarget, data []byte) (string, error) {
	stamp, err := storage.WriteCandidateVersion(target.Store, target.ConfigPath, data, time.Now())
	if errors.Is(err, storage.ErrCandidateExists) {
		return "", errors.New("a config change is already staged; commit or discard it first")
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
			"version":     stamp,
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

// dataRefusal is the error answer both RPCs give, carrying the reason.
func dataRefusal(err error) *zePlugin.Response {
	return &zePlugin.Response{Status: zePlugin.StatusError, Error: err.Error()}
}

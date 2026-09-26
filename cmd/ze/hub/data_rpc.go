// Design: docs/architecture/hub-architecture.md -- request data RPCs beside request reload.
// Related: register_data_rpc.go -- registers the two handlers.
// Related: main_reload.go -- the candidate and reload sequence a live restore runs.

package hub

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/zefs"
)

// dataRPCTarget is what the request data RPCs act on: the daemon's own live
// store, the config it serves, and the reload that accepts and promotes a
// staged candidate (the SIGHUP chain).
type dataRPCTarget struct {
	store      storage.Storage
	configPath string
	reload     func(context.Context) error
}

// dataRPC holds the target once the daemon has built its reload. Until then,
// and in a daemon with no store, it is nil and both RPCs refuse.
var dataRPC atomic.Pointer[dataRPCTarget]

// installDataRPC publishes the target the RPCs act on. The daemon MUST call it
// after its reload function exists, and MUST call it with nil before it closes
// the store.
func installDataRPC(target *dataRPCTarget) { dataRPC.Store(target) }

// keywordPath names the artifact path keyword of both RPCs, and the answer key
// that echoes it.
const keywordPath = "path"

// dataBackupArgs is one parsed `request data backup` line.
type dataBackupArgs struct {
	path  string
	spare int
	force bool
}

// handleDataBackup answers `request data backup path <abs> [spare <n>] [force]`.
func handleDataBackup(_ *pluginserver.CommandContext, args []string) (*zePlugin.Response, error) {
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
	result, err := storage.Backup(target.store, parsed.path, parsed.force, zefs.Spare(parsed.spare))
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
}

// handleDataRestore answers `request data restore path <abs> config [name <n>]`.
// The artifact's config is staged as the candidate FIRST and promoted LAST, by
// the same reload a SIGHUP runs, so a config the reload refuses never becomes
// active and the active pointer and its rollback stay as they were.
func handleDataRestore(ctx *pluginserver.CommandContext, args []string) (*zePlugin.Response, error) {
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
	deviceName := filepath.Base(target.configPath)
	selected, err := storage.ReadRestoreSource(parsed.path, parsed.sourceName, deviceName)
	if err != nil {
		return dataRefusal(fmt.Errorf("request data restore: %w", err)), nil
	}
	stamp, err := storage.WriteCandidateVersion(target.store, target.configPath, selected.Data, time.Now())
	if errors.Is(err, storage.ErrCandidateExists) {
		return dataRefusal(errors.New("request data restore: a config change is already staged; commit or discard it first")), nil
	}
	if err != nil {
		return dataRefusal(fmt.Errorf("request data restore: stage candidate: %w", err)), nil
	}
	if err := target.reload(ctx.Context()); err != nil {
		return dataRefusal(fmt.Errorf("request data restore: the reload refused the config, the active config is unchanged: %w", err)), nil
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

// parseDataRestoreArgs reads `path <abs> config [name <n>]`. The config
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
		if keyword != keywordPath && keyword != "name" {
			return dataRestoreArgs{}, fmt.Errorf("unknown keyword %q: the keywords are path, config and name", keyword)
		}
		if i+1 >= len(args) {
			return dataRestoreArgs{}, fmt.Errorf("%s needs a value", keyword)
		}
		i++
		if keyword == keywordPath {
			parsed.path = args[i]
			continue
		}
		parsed.sourceName = args[i]
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

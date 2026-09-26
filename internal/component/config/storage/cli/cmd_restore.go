// Design: docs/architecture/storage-backends.md -- restore modes.
// Related: main.go -- subcommandHandlers dispatches here.
// Related: cmd_backup.go -- the artifact a restore reads.

package cli

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// restoreMode is the part of the store a restore replaces.
type restoreMode uint8

const (
	restoreModeUnspecified restoreMode = iota
	restoreModeConfig                  // the artifact's config becomes a new active version
	restoreModeFull                    // the artifact's keys replace the whole tree
)

// restoreArgs is one parsed `ze data restore` command line.
type restoreArgs struct {
	file       string
	mode       restoreMode
	sourceName string // "name <source-name>", "" when not given
}

// The two mode keywords of `ze data restore <file> <mode>`.
const (
	restoreKeywordConfig = "config"
	restoreKeywordFull   = "full"
)

const restoreUsage = "usage: ze data restore <file> config [name <source-name>]\n       ze data restore <file> full\n"

// cmdRestore handles `ze data restore <file> config [name <n>]` and
// `ze data restore <file> full`. Both take writer ownership of the store, so
// they refuse while a daemon owns it.
func cmdRestore(storePath string, args []string) int {
	var out textbuf.Buffer
	parsed, err := parseRestoreArgs(args)
	if err != nil {
		out.Str("error: restore: ").Err(err).Byte('\n').Str(restoreUsage)
		out.StdErr() //nolint:errcheck // error output
		return 1
	}
	if parsed.mode == restoreModeFull {
		return cmdRestoreFull(storePath, parsed)
	}
	return cmdRestoreConfig(storePath, parsed)
}

// cmdRestoreFull replaces the live tree with the artifact's keys through the
// keep-source import protocol. The canonical seed name is refused before the
// lock: detect refuses it as a live store, so restoring from it would leave a
// store no opener accepts.
func cmdRestoreFull(storePath string, parsed restoreArgs) int {
	var out textbuf.Buffer
	dir, err := restoreDir(storePath)
	if err == nil {
		err = refuseCanonicalSeed(parsed.file, dir)
	}
	if err != nil {
		out.Str("error: restore: ").Err(err).Byte('\n')
		out.StdErr() //nolint:errcheck // error output
		return 1
	}
	restored, err := storage.RestoreBlob(parsed.file, dir)
	if err != nil {
		out.Str("error: restore: ").Err(err).Byte('\n')
		if errors.Is(err, storage.ErrBusy) {
			out.Str("hint: a daemon owns the store").Str(daemonAddress(storePath))
			out.Str("; a full restore replaces the tree the daemon serves: stop the daemon, then run ze data restore ")
			out.Str(parsed.file).Str(" full; to restore only its config live use: request data restore path <absolute-file> config\n")
		}
		out.StdErr() //nolint:errcheck // error output
		return 1
	}
	keys, err := restored.ListKeys("")
	if closeErr := restored.Close(); closeErr != nil {
		err = errors.Join(err, closeErr)
	}
	if err != nil {
		out.Str("error: restore: ").Err(err).Byte('\n')
		out.StdErr() //nolint:errcheck // error output
		return 1
	}
	out.Str("restore: ").Str(parsed.file).Str(": full: ").Int(int64(len(keys))).Str(" keys now form ").Str(storePath).Byte('\n')
	out.Str("note: a previous tree is kept as database.replaced-<stamp> and an unrelated seed as database.zefs.replaced-<stamp> in ")
	out.Str(dir).Str("; the artifact is unchanged\n")
	out.StdOut() //nolint:errcheck // status output
	return 0
}

// restoreDir answers the configuration folder whose live tree a full restore
// replaces. Only the live tree name is a target: a full restore into a blob or
// into another tree path would publish a store nothing opens.
func restoreDir(storePath string) (string, error) {
	dir, name := filepath.Split(filepath.Clean(storePath))
	if name != storage.TreeName {
		return "", fmt.Errorf("a full restore replaces a live tree named %s; %s is not one", storage.TreeName, storePath)
	}
	if dir == "" {
		return ".", nil
	}
	return filepath.Clean(dir), nil
}

// refuseCanonicalSeed refuses <dir>/database.zefs as a restore source, naming
// the two ways out.
func refuseCanonicalSeed(file, dir string) error {
	source, err := filepath.Abs(file)
	if err != nil {
		return err
	}
	folder, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	seed := filepath.Join(folder, storage.BlobName)
	if source != seed {
		return nil
	}
	return fmt.Errorf("%s is the canonical seed name, which the live store refuses as \"not a live store\": import it with ze init --from %s, or move it to another name and run ze data restore <new-name> full", seed, seed)
}

// cmdRestoreConfig commits the artifact's selected config as a new active
// version of the device's config.
func cmdRestoreConfig(storePath string, parsed restoreArgs) int {
	var out textbuf.Buffer
	target, err := openStore(storePath, true)
	if err != nil {
		out.Str("error: restore: ").Err(err).Byte('\n')
		if errors.Is(err, storage.ErrBusy) {
			out.Str("hint: a daemon owns the store").Str(daemonAddress(storePath))
			out.Str("; restore its config live with: request data restore path <absolute-file> config\n")
		}
		out.StdErr() //nolint:errcheck // error output
		return 1
	}
	deviceName := resolve.DefaultConfig(target)
	stamp, source, err := restoreConfig(target, parsed, deviceName)
	if closeErr := target.Close(); closeErr != nil {
		err = errors.Join(err, closeErr)
	}
	if err != nil {
		out.Str("error: ").Err(err).Byte('\n')
		out.StdErr() //nolint:errcheck // error output
		return 1
	}
	out.Str("restore: ").Str(parsed.file).Str(": config ").Str(source).Str(" committed as ").Str(deviceName)
	out.Str(" version ").Str(stamp).Byte('\n')
	if source != deviceName {
		out.Str("note: the artifact's config ").Str(source).Str(" was renamed ").Str(deviceName).Byte('\n')
	}
	out.StdOut() //nolint:errcheck // status output
	return 0
}

// restoreConfig selects the artifact's config and commits it; the selection
// is refused before anything is written.
func restoreConfig(target storage.Storage, parsed restoreArgs, deviceName string) (stamp, source string, err error) {
	selected, err := storage.ReadRestoreSource(parsed.file, parsed.sourceName, deviceName)
	if err != nil {
		return "", "", err
	}
	stamp, err = storage.RestoreConfig(target, deviceName, selected.Data)
	if err != nil {
		return "", "", err
	}
	return stamp, selected.Name, nil
}

// parseRestoreArgs reads `<file> config [name <source-name>]`.
func parseRestoreArgs(args []string) (restoreArgs, error) {
	if len(args) < 2 {
		return restoreArgs{}, errors.New("missing <file> or mode")
	}
	parsed := restoreArgs{file: args[0]}
	switch args[1] {
	case restoreKeywordConfig:
		parsed.mode = restoreModeConfig
	case restoreKeywordFull:
		parsed.mode = restoreModeFull
	default:
		return restoreArgs{}, fmt.Errorf("unknown mode %q: the modes are config and full", args[1])
	}
	rest := args[2:]
	for len(rest) > 0 {
		if rest[0] != keywordName {
			return restoreArgs{}, fmt.Errorf("unknown keyword %q", rest[0])
		}
		if parsed.mode == restoreModeFull {
			return restoreArgs{}, errors.New("name selects a config and applies to config mode only; full restores every key")
		}
		if len(rest) < 2 {
			return restoreArgs{}, errors.New("name needs a config name")
		}
		parsed.sourceName = rest[1]
		rest = rest[2:]
	}
	return parsed, nil
}

// Design: docs/architecture/storage-backends.md -- restore modes.
// Related: main.go -- subcommandHandlers dispatches here.
// Related: cmd_backup.go -- the artifact a restore reads.

package cli

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// restoreMode is the part of the store a restore replaces.
type restoreMode uint8

const (
	restoreModeUnspecified restoreMode = iota
	restoreModeConfig                  // the artifact's config becomes a new active version
)

// restoreArgs is one parsed `ze data restore` command line.
type restoreArgs struct {
	file       string
	mode       restoreMode
	sourceName string // "name <source-name>", "" when not given
}

const restoreUsage = "usage: ze data restore <file> config [name <source-name>]\n"

// cmdRestore handles `ze data restore <file> config [name <n>]`. It takes
// writer ownership of the store, so it refuses while a daemon owns it and
// names the live route instead.
func cmdRestore(storePath string, args []string) int {
	var out textbuf.Buffer
	parsed, err := parseRestoreArgs(args)
	if err != nil {
		out.Str("error: restore: ").Err(err).Byte('\n').Str(restoreUsage)
		out.StdErr() //nolint:errcheck // error output
		return 1
	}
	return cmdRestoreConfig(storePath, parsed)
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
	case "config":
		parsed.mode = restoreModeConfig
	default:
		return restoreArgs{}, fmt.Errorf("unknown mode %q: the mode is config", args[1])
	}
	rest := args[2:]
	for len(rest) > 0 {
		if rest[0] != "name" {
			return restoreArgs{}, fmt.Errorf("unknown keyword %q", rest[0])
		}
		if len(rest) < 2 {
			return restoreArgs{}, errors.New("name needs a config name")
		}
		parsed.sourceName = rest[1]
		rest = rest[2:]
	}
	return parsed, nil
}

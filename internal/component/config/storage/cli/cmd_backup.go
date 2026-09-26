// Design: docs/architecture/storage-backends.md -- backup artifact.
// Related: main.go -- subcommandHandlers dispatches here.

package cli

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// backupSpareDefault is the spare an artifact is written with when the
// operator names none: a backup is copied whole, never edited in place.
const backupSpareDefault = 0

// cmdBackup handles `ze data backup <file> [spare <n>]`. It takes writer
// ownership of the store, so it refuses while a daemon owns it and names the
// live route instead.
func cmdBackup(storePath string, args []string) int {
	var out textbuf.Buffer
	file, spare, err := parseBackupArgs(args)
	if err != nil {
		out.Str("error: backup: ").Err(err).Byte('\n').Str("usage: ze data backup <file> [spare <0-100>]\n")
		out.StdErr() //nolint:errcheck // error output
		return 1
	}
	source, err := openStore(storePath, true)
	if err != nil {
		out.Str("error: backup: ").Err(err).Byte('\n')
		if errors.Is(err, storage.ErrBusy) {
			out.Str("hint: a daemon owns the store").Str(daemonAddress(storePath))
			out.Str("; back it up live with: request data backup path <absolute-file>\n")
		}
		out.StdErr() //nolint:errcheck // error output
		return 1
	}
	result, err := storage.Backup(source, file, zefs.Spare(spare))
	if closeErr := source.Close(); closeErr != nil {
		err = errors.Join(err, closeErr)
	}
	if err != nil {
		out.Str("error: ").Err(err).Byte('\n')
		out.StdErr() //nolint:errcheck // error output
		return 1
	}
	out.Str("backup: ").Str(result.Path).Str(": ").Int(int64(result.Keys)).Str(" keys, ").Int(result.Bytes).Str(" bytes\n")
	out.Str("warning: ").Str(result.Path).Str(" holds secrets (credentials and private keys); it is mode 0600\n")
	out.StdOut() //nolint:errcheck // status output
	return 0
}

// parseBackupArgs reads `<file> [spare <n>]`. The range itself is zefs.Spare's
// to enforce; this refuses what is not an integer.
func parseBackupArgs(args []string) (file string, spare int, err error) {
	if len(args) == 0 {
		return "", 0, errors.New("missing <file>")
	}
	file = args[0]
	spare = backupSpareDefault
	rest := args[1:]
	for len(rest) > 0 {
		if rest[0] != "spare" {
			return "", 0, fmt.Errorf("unknown keyword %q", rest[0])
		}
		if len(rest) < 2 {
			return "", 0, errors.New("spare needs a value in the range 0 to 100")
		}
		spare, err = strconv.Atoi(rest[1])
		if err != nil {
			return "", 0, fmt.Errorf("spare %q is not an integer in the range 0 to 100", rest[1])
		}
		rest = rest[2:]
	}
	return file, spare, nil
}

// daemonAddress returns " at <host/port>" for the SSH endpoint the store
// records, read through a reader handle, which a running daemon permits. It
// returns "" when the store records none or cannot be read, so the hint
// still names the live route without inventing an address.
func daemonAddress(storePath string) string {
	reader, err := openStore(storePath, false)
	if err != nil {
		return ""
	}
	defer reader.Close() //nolint:errcheck // read-only hint lookup.
	endpoint, err := reader.ReadKey(zefs.KeySSHDefault.Pattern)
	if err != nil {
		return ""
	}
	return " at " + string(endpoint)
}

// Design: docs/architecture/zefs-format.md -- managed store and blob artifact CLI
//
// Package cli provides key operations over managed stores and blob artifacts:
// offline on a store no process owns, and through the daemon's own store
// handle while a daemon owns it.
package cli

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/storage/daemonroute"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/suggest"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

const defaultStoreName = storage.TreeName

// subcommandCheck is the name of the `ze data check` subcommand, which the
// handler table and the usage page both spell. It is not findingCheck, the
// answer column of an integrity finding, which only shares its spelling.
const subcommandCheck = "check"

// subcommandHandlers maps subcommand names to their handler functions.
// Each handler receives the store path and remaining args.
var subcommandHandlers = map[string]func(string, []string) int{
	"backup":        cmdBackup,
	"restore":       cmdRestore,
	"import":        cmdImport,
	"write":         cmdWrite,
	"rm":            cmdRm,
	"list":          cmdList,
	"cat":           cmdCat,
	"registered":    cmdRegistered,
	subcommandCheck: cmdCheck,
	"repair":        cmdRepair,
	"encode":        cmdEncode,
}

// Run executes the data subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	storePath, remaining := extractPathFlag(args)

	if len(remaining) == 0 {
		usage()
		return 1
	}

	subcmd := remaining[0]
	subArgs := remaining[1:]

	if subcmd == "help" || subcmd == "-h" || subcmd == "--help" {
		usage()
		return 0
	}

	if handler, ok := subcommandHandlers[subcmd]; ok {
		return handler(storePath, subArgs)
	}

	fmt.Fprintf(os.Stderr, "unknown data subcommand: %s\n", subcmd)
	candidates := make([]string, 0, len(subcommandHandlers))
	for k := range subcommandHandlers {
		candidates = append(candidates, k)
	}
	if s := suggest.Command(subcmd, candidates); s != "" {
		fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
	}
	usage()
	return 1
}

// extractPathFlag parses --path <path> and returns the store path and remaining args.
func extractPathFlag(args []string) (string, []string) {
	storePath := ""
	remaining := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--path" && i+1 < len(args):
			storePath = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--path="):
			storePath = args[i][len("--path="):]
		default:
			remaining = append(remaining, args[i])
		}
	}

	if storePath == "" {
		storePath = filepath.Join(resolve.StoreDir(""), defaultStoreName)
	}

	return storePath, remaining
}

// openStore opens a database directory or an explicitly named blob artifact.
// The caller MUST close the returned handle, releasing ownership for writers.
func openStore(storePath string, writable bool) (storage.Storage, error) {
	info, err := os.Stat(storePath)
	if errors.Is(err, os.ErrNotExist) && filepath.Base(storePath) == storage.TreeName {
		// An absent live tree is the store's to explain: no store at all, or
		// an unfinished import or restore whose recovery command it names.
		return storage.OpenTree(storePath, writable)
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", storePath, err)
	}
	if !info.IsDir() {
		return storage.OpenBlob(storePath, writable)
	}
	return storage.OpenTree(storePath, writable)
}

// openOrCreateStore only creates explicitly named blob artifacts. Live stores
// are initialized by ze init, never by a data command.
func openOrCreateStore(storePath string) (storage.Storage, error) {
	if _, err := os.Stat(storePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if strings.HasSuffix(storePath, ".zefs") {
				return storage.CreateBlob(storePath)
			}
		}
		return nil, fmt.Errorf("open %s: %w; run ze init to create a live store", storePath, err)
	}
	return openStore(storePath, true)
}

// filePathToKey converts a filesystem path to a blob key under the file/active/ namespace.
// Only the base filename is used as the key (not the full path).
func filePathToKey(path string) string {
	return zefs.KeyFileActive.Key(filepath.Base(path))
}

// commandDataWrite is the local command the refusals of the write route name,
// and errorLinePrefix opens every error line the write and removal routes
// print.
const (
	commandDataWrite = "ze data write"
	errorLinePrefix  = "error: "
)

// cmdWrite is `ze data write <key> <file>`. With the store free it writes the
// key itself. While a daemon owns the store, the open answers storage.ErrBusy
// and the bytes go to that daemon as `update data key <key> content <base64>`
// (daemonroute). Both routes print `wrote <key> (<n> bytes) into <store>`
// and exit 0; a file that cannot be read, a store that cannot be reached and a
// refused write exit 2, a usage error 1.
func cmdWrite(storePath string, args []string) int {
	return writeKey(storePath, args, daemonroute.OverSSH, os.Stdout, os.Stderr)
}

// writeKey is cmdWrite over one daemon ask and two writers, so a test stubs the
// daemon and reads the lines.
func writeKey(storePath string, args []string, ask daemonroute.Ask, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		writeText(stderr, "usage: ze data write <key> <file>\n")
		return 1
	}
	key, srcPath := args[0], args[1]

	data, err := cliio.ReadFile(srcPath) // "-" reads stdin
	if err != nil {
		var line textbuf.Buffer
		writeText(stderr, line.Str("error: read ").Str(srcPath).Str(": ").Err(err).Byte('\n').String())
		return 2
	}

	s, openErr := openOrCreateStore(storePath)
	if errors.Is(openErr, storage.ErrBusy) {
		return writeKeyThroughDaemon(storePath, key, data, openErr, ask, stdout, stderr)
	}
	if openErr != nil {
		writeErrorLine(stderr, openErr)
		return 2
	}
	defer s.Close() //nolint:errcheck // best-effort close

	if writeErr := s.WriteKey(key, data); writeErr != nil {
		var line textbuf.Buffer
		writeText(stderr, line.Str("error: write ").Str(key).Str(": ").Err(writeErr).Byte('\n').String())
		return 2
	}

	writeWrote(stdout, key, len(data), storePath)
	return 0
}

// writeKeyThroughDaemon sends the bytes to the daemon that owns the store, as
// `update data key <key> content <base64>`. The line is first judged by the
// model, as the daemon's validator judges it, so a content over the bound one
// SSH packet carries is refused here, before anything connects, with the
// daemon's own refusal: the bound and the route for a larger value. busy is
// the store's refusal of the local open, which a refusal for no answer names
// first. The decoded answer must name the key and the byte count written.
func writeKeyThroughDaemon(storePath, key string, data []byte, busy error, ask daemonroute.Ask, stdout, stderr io.Writer) int {
	encoded := base64.StdEncoding.EncodeToString(data)
	tokens := []string{keywordKey, key, keywordContent, encoded}
	if _, err := command.ValidateModelArgs(pathUpdateDataKey, tokens, nil); err != nil {
		var line textbuf.Buffer
		writeText(stderr, line.Str(errorLinePrefix).Str(commandDataWrite).Str(": ").Err(err).Byte('\n').String())
		return 2
	}

	var line textbuf.Buffer
	line.Str(pathUpdateDataKey).Byte(' ').Str(configtext.QuoteCommandWord(key)).
		Byte(' ').Str(keywordContent).Byte(' ').Str(encoded)
	var answer writeAnswer
	err := daemonroute.Send(ask, storePath, line.String(), &answer)
	if unanswered, ok := errors.AsType[*daemonroute.Unanswered](err); ok {
		writeErrorLine(stderr, daemonroute.UnansweredRefusal(commandDataWrite, busy, unanswered))
		return 2
	}
	if err != nil {
		writeErrorLine(stderr, err)
		return 2
	}
	if answer.Key != key {
		writeErrorLine(stderr, writeMismatch(key, len(data), answer))
		return 2
	}
	if answer.Bytes != len(data) {
		writeErrorLine(stderr, writeMismatch(key, len(data), answer))
		return 2
	}
	writeWrote(stdout, key, len(data), storePath)
	return 0
}

// writeAnswer is the answer of `update data key`: the key it wrote and the
// bytes it wrote into it.
type writeAnswer struct {
	Key   string `json:"key"`
	Bytes int    `json:"bytes"`
}

// writeMismatch refuses an answer that names another key or another byte
// count than the write sent: nothing then says the key holds the file.
func writeMismatch(key string, octets int, answer writeAnswer) *opererr.Error {
	var problem, fix textbuf.Buffer
	problem.Str(commandDataWrite).Str(": the daemon answered the write of ").Str(strconv.Quote(answer.Key)).
		Str(" (").Int(int64(answer.Bytes)).Str(" bytes), not of ").Str(key).
		Str(" (").Int(int64(octets)).Str(" bytes)")
	fix.Str("ze data cat ").Str(configtext.QuoteCommandWord(key))
	return opererr.New(problem.String(), "whether "+key+" holds the file is not known", fix.String())
}

// writeWrote prints the line a written key gets, on both routes.
func writeWrote(stdout io.Writer, key string, octets int, storePath string) {
	var line textbuf.Buffer
	writeText(stdout, line.Str("wrote ").Str(key).Str(" (").Int(int64(octets)).Str(" bytes) into ").
		Str(storePath).Byte('\n').String())
}

// writeErrorLine prints err as one error of the write route.
func writeErrorLine(stderr io.Writer, err error) {
	var line textbuf.Buffer
	writeText(stderr, line.Str(errorLinePrefix).Err(err).Byte('\n').String())
}

// commandDataImport is the local command the refusals of the import route name.
const commandDataImport = "ze data import"

// cmdImport is `ze data import <file>...`. Each file lands in the config named
// by its base name, under the key filePathToKey gives it. With the store free,
// a config the store already holds is committed as a new active version, the
// previous one kept as rollback (storage.RestoreConfig), as `ze config import`
// replaces one: a raw write of its file/active key is shadowed by its active
// pointer (storage.ReadActiveConfig), so the daemon would go on reading the old
// config. A config the store does not hold yet is written as its key. While a
// daemon owns the store, the open answers storage.ErrBusy and each file goes to
// that daemon as `update data config name <name> content <base64>`, one
// command per file (daemonroute). Both routes print `imported <key> (<n>
// bytes)` per file and `<n> files imported into <store>`, and exit 0 when at
// least one file was imported, 2 when none was or when the store could not be
// reached, 1 on usage. Stdin ("-") is refused on both routes: it has no file
// name to name the config (unnamedImportRefusal).
func cmdImport(storePath string, args []string) int {
	return importConfigs(storePath, args, daemonroute.OverSSH, os.Stdout, os.Stderr)
}

// importConfigs is cmdImport over one daemon ask and two writers, so a test
// stubs the daemon and reads the lines.
func importConfigs(storePath string, paths []string, ask daemonroute.Ask, stdout, stderr io.Writer) int {
	if len(paths) == 0 {
		writeText(stderr, "usage: ze data import <file>...\n")
		return 1
	}

	s, err := openOrCreateStore(storePath)
	if errors.Is(err, storage.ErrBusy) {
		return importConfigsThroughDaemon(storePath, paths, err, ask, stdout, stderr)
	}
	if err != nil {
		writeErrorLine(stderr, err)
		return 2
	}
	defer s.Close() //nolint:errcheck // best-effort close

	imported := 0
	for _, path := range paths {
		if refusal := unnamedImportRefusal(path); refusal != nil {
			writeErrorLine(stderr, refusal)
			continue
		}
		data, readErr := cliio.ReadFile(path)
		if readErr != nil {
			writeReadError(stderr, path, readErr)
			continue
		}
		key := filePathToKey(path)
		if writeErr := ImportConfig(s, path, data); writeErr != nil {
			var line textbuf.Buffer
			writeText(stderr, line.Str("error: write ").Str(key).Str(": ").Err(writeErr).Byte('\n').String())
			continue
		}
		writeImported(stdout, key, len(data))
		imported++
	}
	return importedSummary(stdout, storePath, imported)
}

// ImportConfig lands data in the config named by the base name of path, on
// the store the caller owns: a new active version of a config the store holds,
// the previous one kept as rollback, or the key of a config it does not hold.
//
// What decides is the active pointer, because the daemon reads a config
// through it: a raw write under a pointer is shadowed, whether or not the file
// key exists. A config with a file key and no pointer yet is versioned too, as
// `ze config import` replaces it.
//
// s is the store's owner, so no daemon runs and none can open a confirmed
// commit window during the write; a window record found here was left by a
// daemon that stopped during that window. Its next start reverts the window,
// which would undo the import, so the import is refused with the window's own
// refusal (confirm.WriteOutsideRecorded), as `ze config rollback` is.
// The caller MUST own this store for the call and MUST use the daemon's reload
// path instead for its live configuration.
func ImportConfig(s storage.Storage, path string, data []byte) error {
	name := filepath.Base(path)
	return confirm.WriteOutsideRecorded(s, name, func() error {
		return storage.ImportConfig(s, name, data)
	})
}

// importConfigsThroughDaemon sends each file to the daemon that owns the
// store, as `update data config name <name> content <base64>`, with the local
// route's rules: a file that cannot be read or that the daemon refuses is an
// error line and the walk goes on, so a refusal of the third file leaves the
// first two imported; a daemon that does not answer ends the walk, because no
// later file can reach it. daemonroute.ConfigLine judges each line by the
// model first, as the daemon's validator judges it, so a file over the bound
// one SSH packet carries is refused here, before anything connects, with the
// daemon's own refusal. busy is the store's refusal of the local open, which a
// refusal for no answer names first. The decoded answer must name the config
// and the byte count imported.
func importConfigsThroughDaemon(storePath string, paths []string, busy error, ask daemonroute.Ask, stdout, stderr io.Writer) int {
	imported := 0
	for _, path := range paths {
		if refusal := unnamedImportRefusal(path); refusal != nil {
			writeErrorLine(stderr, refusal)
			continue
		}
		data, readErr := cliio.ReadFile(path)
		if readErr != nil {
			writeReadError(stderr, path, readErr)
			continue
		}
		name := filepath.Base(path)
		line, err := daemonroute.ConfigLine(name, data)
		if err != nil {
			var refusal textbuf.Buffer
			writeText(stderr, refusal.Str(errorLinePrefix).Str(commandDataImport).Str(": ").Err(err).Byte('\n').String())
			continue
		}

		var answer daemonroute.ConfigAnswer
		err = daemonroute.Send(ask, storePath, line, &answer)
		if unanswered, ok := errors.AsType[*daemonroute.Unanswered](err); ok {
			writeErrorLine(stderr, daemonroute.UnansweredRefusal(commandDataImport, busy, unanswered))
			if imported == 0 {
				return 2
			}
			break
		}
		if err != nil {
			writeErrorLine(stderr, err)
			continue
		}
		if mismatch := answer.Mismatch(commandDataImport, name, len(data)); mismatch != nil {
			writeErrorLine(stderr, mismatch)
			continue
		}
		writeImported(stdout, filePathToKey(path), len(data))
		imported++
	}
	return importedSummary(stdout, storePath, imported)
}

// unnamedImportRefusal refuses an input whose base name is no plain config
// name (storage.PlainConfigName), before it is read. `ze data import` names
// the config after the file, and stdin ("-") has no file name: read, it would
// be committed as a config named "-", which no daemon ever loads and which
// `update data config` refuses. Both routes ask here, so they refuse the same
// inputs. It answers nil for an input that names a config.
func unnamedImportRefusal(path string) *opererr.Error {
	if storage.PlainConfigName(filepath.Base(path)) {
		return nil
	}
	var problem textbuf.Buffer
	problem.Str(commandDataImport).Str(": ").Str(strconv.Quote(path)).Str(" was not imported")
	return opererr.New(problem.String(),
		"the config is named after the file it is read from, and this input has no file name",
		"ze config import --name <name> -")
}

// writeReadError prints the line a file that cannot be read gets.
func writeReadError(stderr io.Writer, path string, err error) {
	var line textbuf.Buffer
	writeText(stderr, line.Str("error: read ").Str(path).Str(": ").Err(err).Byte('\n').String())
}

// writeImported prints the line each imported file gets, on both routes.
func writeImported(stdout io.Writer, key string, octets int) {
	var line textbuf.Buffer
	writeText(stdout, line.Str("imported ").Str(key).Str(" (").Int(int64(octets)).Str(" bytes)\n").String())
}

// importedSummary prints the count line both routes end with and answers the
// exit code: 0 when at least one file was imported, 2 when none was.
func importedSummary(stdout io.Writer, storePath string, imported int) int {
	var line textbuf.Buffer
	writeText(stdout, line.Int(int64(imported)).Str(" files imported into ").Str(storePath).Byte('\n').String())
	if imported == 0 {
		return 2
	}
	return 0
}

// cmdRm is `ze data rm <key>...`. With the store free it removes each key
// itself. While a daemon owns the store, the open answers storage.ErrBusy and
// each key goes to that daemon as `delete data key <key>`, one command per
// key (daemonroute). Both routes print `removed <key>` per key and
// `<n> entries removed from <store>`, and exit 0 when at least one key was
// removed, 2 when none was or the store could not be reached, 1 on usage.
func cmdRm(storePath string, args []string) int {
	return removeKeys(storePath, args, daemonroute.OverSSH, os.Stdout, os.Stderr)
}

// removeKeys is cmdRm over one daemon ask and two writers, so a test stubs the
// daemon and reads the lines.
func removeKeys(storePath string, keys []string, ask daemonroute.Ask, stdout, stderr io.Writer) int {
	if len(keys) == 0 {
		writeText(stderr, "usage: ze data rm <key>...\n")
		return 1
	}

	s, err := openStore(storePath, true)
	if errors.Is(err, storage.ErrBusy) {
		return removeKeysThroughDaemon(storePath, keys, err, ask, stdout, stderr)
	}
	if err != nil {
		var line textbuf.Buffer
		writeText(stderr, line.Str(errorLinePrefix).Err(err).Byte('\n').String())
		return 2
	}
	defer s.Close() //nolint:errcheck // best-effort close

	removed := 0
	for _, key := range keys {
		if rmErr := s.RemoveKey(key); rmErr != nil {
			var line textbuf.Buffer
			writeText(stderr, line.Str("error: remove ").Str(key).Str(": ").Err(rmErr).Byte('\n').String())
			continue
		}
		writeRemoved(stdout, key)
		removed++
	}
	return removedSummary(stdout, storePath, removed)
}

// removeKeysThroughDaemon sends each key to the daemon that owns the store, as
// `delete data key <key>`, with the local route's rules: a key the daemon
// refuses is an error line and the walk goes on; a daemon that does not answer
// ends the walk, because no later key can reach it. busy is the store's
// refusal of the local open, which the refusal names first.
func removeKeysThroughDaemon(storePath string, keys []string, busy error, ask daemonroute.Ask, stdout, stderr io.Writer) int {
	removed := 0
	for _, key := range keys {
		var line textbuf.Buffer
		line.Str("delete data key ").Str(configtext.QuoteCommandWord(key))
		var answer keyAnswer
		err := daemonroute.Send(ask, storePath, line.String(), &answer)
		if unanswered, ok := errors.AsType[*daemonroute.Unanswered](err); ok {
			var refusal textbuf.Buffer
			writeText(stderr, refusal.Str(errorLinePrefix).Err(daemonroute.UnansweredRefusal("ze data rm", busy, unanswered)).Byte('\n').String())
			if removed == 0 {
				return 2
			}
			break
		}
		if err != nil {
			var refusal textbuf.Buffer
			writeText(stderr, refusal.Str(errorLinePrefix).Err(err).Byte('\n').String())
			continue
		}
		if answer.Key != key {
			var problem, fix, refusal textbuf.Buffer
			problem.Str("ze data rm: the daemon answered the removal of ").Str(strconv.Quote(answer.Key)).
				Str(", not of ").Str(key)
			fix.Str("ze data cat ").Str(configtext.QuoteCommandWord(key))
			mismatch := opererr.New(problem.String(), "whether "+key+" was removed is not known, so it is not counted", fix.String())
			writeText(stderr, refusal.Str(errorLinePrefix).Err(mismatch).Byte('\n').String())
			continue
		}
		writeRemoved(stdout, key)
		removed++
	}
	return removedSummary(stdout, storePath, removed)
}

// keyAnswer is the answer of a daemon key command: the key it acted on.
type keyAnswer struct {
	Key string `json:"key"`
}

// writeRemoved prints the line each removed key gets, on both routes.
func writeRemoved(stdout io.Writer, key string) {
	var line textbuf.Buffer
	writeText(stdout, line.Str("removed ").Str(key).Byte('\n').String())
}

// removedSummary prints the count line both routes end with and answers the
// exit code: 0 when at least one key was removed, 2 when none was.
func removedSummary(stdout io.Writer, storePath string, removed int) int {
	var line textbuf.Buffer
	writeText(stdout, line.Int(int64(removed)).Str(" entries removed from ").Str(storePath).Byte('\n').String())
	if removed == 0 {
		return 2
	}
	return 0
}

// writeText writes one piece of command output. A failed write to the
// operator's terminal has nowhere left to be reported.
func writeText(w io.Writer, text string) {
	io.WriteString(w, text) //nolint:errcheck // command output; see the comment above
}

func cmdList(storePath string, args []string) int {
	s, err := openStore(storePath, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	defer s.Close() //nolint:errcheck // best-effort close

	prefix := ""
	if len(args) > 0 {
		prefix = args[0]
	}

	keys, err := s.ListKeys(prefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: list %s: %v\n", storePath, err)
		return 2
	}
	for _, key := range keys {
		fmt.Println(key)
	}
	return 0
}

// cmdCat is `ze data cat`: the answer of `show data cat`, printed.
func cmdCat(storePath string, args []string) int {
	return command.PrintLocalAnswer("show data cat", catAnswer(storePath, args))
}

func cmdRegistered(_ string, args []string) int {
	validated, err := command.ValidateModelArgs("show data registered", args, nil)
	if err != nil {
		return command.PrintLocalAnswer("show data registered", storageRefusal(err))
	}
	return command.PrintLocalAnswer("show data registered", dataRegistered(validated))
}

func usage() {
	p := helpfmt.Page{
		Command:   "ze data",
		ShortHelp: "Manage store keys and blob artifacts",
		Usage:     []string{"ze data [--path <store>] <command> [args...]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: []helpfmt.HelpEntry{
				{Name: "backup <file> [spare <n>]", Desc: "Copy every key to one blob artifact, exact-fit unless spare names a percentage (0-100); offline only"},
				{Name: "restore <file> config [name <source-name>]", Desc: "Commit the artifact's config as a new active version of this device's config; offline only"},
				{Name: "restore <file> full", Desc: "Replace the whole tree with the artifact's keys, keeping the previous tree as database.replaced-<stamp>; offline only"},
				{Name: "write <key> <file>", Desc: "Write a file to an explicit key; while ze runs, through the daemon, which writes runtime-state keys only, up to 195,840 bytes"},
				{Name: "import <file>...", Desc: "Import each file as the config named by its file name, as a new active version when that config exists; while ze runs, through the daemon, which reloads for its own config, up to 195,840 bytes per file"},
				{Name: "rm <key>...", Desc: "Remove entries from the store; while ze runs, through the daemon, which removes runtime-state keys only"},
				{Name: "list [prefix]", Desc: "List all matching keys recursively"},
				{Name: "cat <key>", Desc: "Print entry content to stdout"},
				{Name: "registered", Desc: "List all registered key patterns"},
				{Name: subcommandCheck, Desc: "Verify store integrity (CRC32c)"},
				{Name: "repair --output <path>", Desc: "Recover valid entries to a new store"},
				{Name: "encode [--crc|--header] [--cap N] <string>", Desc: "Show netcapstring encoding"},
			}},
			{Title: "Flags", Entries: []helpfmt.HelpEntry{
				{Name: "--path <store>", Desc: "Database directory or blob artifact (default: {configDir}/database)"},
			}},
		},
		Examples: []string{
			"ze data import /etc/ze/router.conf /etc/ze/site-b.conf",
			"ze data list",
			"ze data list file/active/",
			"ze data list meta/",
			"ze data cat file/active/router.conf",
			"ze data rm meta/ddos-flowspec/announcement",
			"ze data registered",
			"ze data backup /var/backups/router-1.zefs",
			"ze data restore /var/backups/router-1.zefs config",
			"ze data restore /var/backups/router-1.zefs full",
			"ze data --path /tmp/test.zefs import router.conf",
		},
	}
	p.WriteErr()
}

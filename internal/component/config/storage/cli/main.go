// Design: docs/architecture/zefs-format.md -- managed store and blob artifact CLI
//
// Package cli provides key operations over managed stores and blob artifacts:
// offline on a store no process owns, and through the daemon's own store
// handle while a daemon owns it.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
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

func cmdWrite(storePath string, args []string) int {
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: ze data write <key> <file>\n")
		return 1
	}
	key, srcPath := args[0], args[1]

	data, err := cliio.ReadFile(srcPath) // "-" reads stdin
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read %s: %v\n", srcPath, err)
		return 2
	}

	s, openErr := openOrCreateStore(storePath)
	if openErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", openErr)
		return 2
	}
	defer s.Close() //nolint:errcheck // best-effort close

	if writeErr := s.WriteKey(key, data); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: write %s: %v\n", key, writeErr)
		return 2
	}

	fmt.Printf("wrote %s (%d bytes) into %s\n", key, len(data), storePath)
	return 0
}

func cmdImport(storePath string, args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: ze data import <file>...\n")
		return 1
	}

	s, err := openOrCreateStore(storePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	defer s.Close() //nolint:errcheck // best-effort close

	imported := 0
	stdinConflict := false
	for _, path := range args {
		data, readErr := cliio.ReadFile(path)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "error: read %s: %v\n", path, readErr)
			// Stdin can be read once; a second "-" fails closed after any
			// preceding successful imports, as with config import.
			if errors.Is(readErr, cliio.ErrStdinClaimed) {
				stdinConflict = true
				break
			}
			continue
		}

		key := filePathToKey(path)

		if writeErr := s.WriteKey(key, data); writeErr != nil {
			fmt.Fprintf(os.Stderr, "error: write %s: %v\n", key, writeErr)
			continue
		}

		fmt.Printf("imported %s (%d bytes)\n", key, len(data))
		imported++
	}

	if stdinConflict {
		return 2
	}

	fmt.Printf("%d files imported into %s\n", imported, storePath)
	if imported == 0 {
		return 2
	}
	return 0
}

// cmdRm is `ze data rm <key>...`. With the store free it removes each key
// itself. While a daemon owns the store, the open answers storage.ErrBusy and
// each key goes to that daemon as `delete data key <key>`, one command per
// key (daemon_route.go). Both routes print `removed <key>` per key and
// `<n> entries removed from <store>`, and exit 0 when at least one key was
// removed, 2 when none was or the store could not be reached, 1 on usage.
func cmdRm(storePath string, args []string) int {
	return removeKeys(storePath, args, askDaemonOverSSH, os.Stdout, os.Stderr)
}

// removeKeys is cmdRm over one daemon ask and two writers, so a test stubs the
// daemon and reads the lines.
func removeKeys(storePath string, keys []string, ask daemonAsk, stdout, stderr io.Writer) int {
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
		writeText(stderr, line.Str("error: ").Err(err).Byte('\n').String())
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
func removeKeysThroughDaemon(storePath string, keys []string, busy error, ask daemonAsk, stdout, stderr io.Writer) int {
	removed := 0
	for _, key := range keys {
		var line textbuf.Buffer
		line.Str("delete data key ").Str(configtext.QuoteCommandWord(key))
		var answer keyAnswer
		err := sendToDaemon(ask, storePath, line.String(), &answer)
		if unanswered, ok := errors.AsType[*daemonUnanswered](err); ok {
			var refusal textbuf.Buffer
			writeText(stderr, refusal.Str("error: ").Err(unansweredRefusal("ze data rm", busy, unanswered)).Byte('\n').String())
			if removed == 0 {
				return 2
			}
			break
		}
		if err != nil {
			var refusal textbuf.Buffer
			writeText(stderr, refusal.Str("error: ").Err(err).Byte('\n').String())
			continue
		}
		if answer.Key != key {
			var problem, fix, refusal textbuf.Buffer
			problem.Str("ze data rm: the daemon answered the removal of ").Str(strconv.Quote(answer.Key)).
				Str(", not of ").Str(key)
			fix.Str("ze data cat ").Str(configtext.QuoteCommandWord(key))
			mismatch := opererr.New(problem.String(), "whether "+key+" was removed is not known, so it is not counted", fix.String())
			writeText(stderr, refusal.Str("error: ").Err(mismatch).Byte('\n').String())
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
	if len(args) > 0 {
		return showRegisteredKey(args[0])
	}
	return listRegisteredKeys()
}

func listRegisteredKeys() int {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	printKeyRow(w, "PATTERN", "DESCRIPTION")
	printKeyRow(w, "-------", "-----------")
	for _, e := range zefs.Entries() {
		printKeyRow(w, e.Pattern, e.Description)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func showRegisteredKey(key string) int {
	for _, e := range zefs.Entries() {
		if e.Pattern != key {
			continue
		}
		fmt.Printf("Pattern:     %s\n", e.Pattern)
		fmt.Printf("Description: %s\n", e.Description)
		return 0
	}
	fmt.Fprintf(os.Stderr, "error: unknown key %q\n", key)
	return 1
}

// printKeyRow writes a tab-separated row to w.
func printKeyRow(w *tabwriter.Writer, cols ...string) {
	if _, err := fmt.Fprintln(w, textbuf.Join(cols, "\t")); err != nil { //nolint:errcheck // output
		return
	}
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
				{Name: "write <key> <file>", Desc: "Write a file to an explicit key"},
				{Name: "import <file>...", Desc: "Import files into the store"},
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
			"ze data cat file/active/etc/ze/router.conf",
			"ze data rm file/active/etc/ze/old-router.conf",
			"ze data registered",
			"ze data backup /var/backups/router-1.zefs",
			"ze data restore /var/backups/router-1.zefs config",
			"ze data restore /var/backups/router-1.zefs full",
			"ze data --path /tmp/test.zefs import router.conf",
		},
	}
	p.WriteErr()
}

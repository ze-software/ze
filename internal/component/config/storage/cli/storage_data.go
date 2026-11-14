// Design: docs/architecture/api/commands.md — where a command is served
// Related: docs/architecture/api/commands.md — the operator model this serves
//
// storage_data.go answers `show data list`, `show data registered` and `show
// data check` with structured data, so they reach the pipe layer and the
// daemon serves them under the same path. list and registered printed a table
// and returned an exit code, while YANG declared a wire method for each that
// no daemon handler implements.
//
// `show data cat` answers the BYTES of one stored file, which may be YAML,
// JSON, a certificate or a binary blob. Those bytes are the answer, so it
// answers a command.Blob: the local routes print the bytes unchanged, and a
// route that carries the answer as JSON names the encoding they travel in.

package cli

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// answerVanishedEntries is the `show data check` answer key counting the keys
// a writer removed while the check ran (storeVerdict.removedKeys).
const answerVanishedEntries = "vanished-entries"

// storageRefusal is the answer when the store could not be opened or read. It
// carries the line the printers write for the same failure, so both spellings
// of the failure agree. The handler writes nothing itself
// (command.LocalDataHandler).
func storageRefusal(err error) command.LocalAnswer {
	return command.LocalRefusal(2, command.LocalErrorLine(err))
}

// dataList answers `show data list [prefix]`: the keys the store holds.
func dataList(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	storePath, remaining := extractPathFlag(args)
	s, err := openStore(storePath, false)
	if err != nil {
		return storageRefusal(err)
	}
	defer s.Close() //nolint:errcheck // best-effort close

	prefix := ""
	if len(remaining) > 0 {
		prefix = remaining[0]
	}
	keys, err := s.ListKeys(prefix)
	if err != nil {
		return storageRefusal(err)
	}
	rows := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, map[string]any{keywordKey: key})
	}
	return command.LocalAnswer{Payload: map[string]any{"keys": rows}}
}

// dataCheck answers `show data check`: the store's integrity verdict as rows,
// the rows `ze data check` prints (integrityVerdict), with exit 1 when any row
// is an error. In the caller's own process it checks the store the path
// resolves to, as `ze data check` does. On the daemon it checks the tree the
// daemon commits to with the daemon's writers held (storage.HoldWrites): the
// walk opens one directory entry after another, so a key the daemon removed
// between the listing and the open would be a key the check did not read
// (a removed-during-check row, as on the local route), where under the guard
// the removal waits. A daemon that has not published its store (starting or
// stopping) has no writers to hold, and checks the resolved path unguarded.
func dataCheck(validated command.ValidatedArgs) command.LocalAnswer {
	storePath, _ := extractPathFlag(validated.Tokens())
	target := dataRPC.Load()
	if validated.Route().CallerLocal() || target == nil {
		verdict, err := integrityVerdict(storePath)
		if err != nil {
			return storageRefusal(err)
		}
		return verdict.answer()
	}
	// Under HoldWrites no writer of this daemon changes the tree, and while
	// the daemon owns the store no other process opens it as a writer, so
	// no key vanishes from this walk. The verdict still shows any the
	// reports name, as on the local route, rather than assume none.
	var verdict storeVerdict
	err := storage.HoldWrites(target.Store, func(tree string) error {
		var walkErr error
		verdict, walkErr = integrityVerdict(tree)
		return walkErr
	})
	if err != nil {
		return storageRefusal(err)
	}
	return verdict.answer()
}

// answer is the verdict as `show data check` answers it: the entry counts, the
// count of keys a writer removed while the check ran (vanished-entries), and
// one row per finding, a removed key included as a warning row of kind
// removed-during-check, exit 1 with the closing line when any row is an error.
// A removed key is not an error: it was not read, and the count says so.
func (v storeVerdict) answer() command.LocalAnswer {
	rows := make([]map[string]any, 0, len(v.findings))
	for _, finding := range v.findings {
		rows = append(rows, map[string]any{
			findingSeverity: finding.severity, findingCheck: finding.check, findingKind: finding.kind,
			keywordKey: finding.key, findingDetail: finding.detail,
		})
	}
	payload := map[string]any{
		"path": v.path, "entries": v.entries, "corrupt-entries": v.corrupt,
		answerVanishedEntries: len(v.removedKeys()),
		"history-checked":     v.historyChecked, "findings": rows,
	}
	errorCount := v.errorCount()
	if errorCount == 0 {
		return command.LocalAnswer{Payload: payload}
	}
	var tb textbuf.Buffer
	tb.Str("error: ")
	switch {
	case v.containerFault:
		tb.Str("the store container does not parse; run ze data repair --output <path>")
	case v.corrupt > 0:
		tb.Str(v.corruptLine())
	default:
		tb.Int(int64(errorCount)).Str(" history errors; run ze data repair --output <path>")
	}
	tb.Byte('\n')
	return command.LocalAnswer{Payload: payload, Code: 1, Diagnostic: tb.String()}
}

// dataRegistered answers `show data registered [pattern]`: the key patterns the
// code declares, and what each one holds.
func dataRegistered(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	entries := zefs.Entries()
	rows := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		if len(args) > 0 && e.Pattern != args[0] {
			continue
		}
		rows = append(rows, map[string]any{
			"pattern": e.Pattern, "description": e.Description,
		})
	}
	return command.LocalAnswer{Payload: map[string]any{"patterns": rows}}
}

// catAnswer answers `show data cat [--path <store>] <key>`: the bytes of the
// stored file as a Blob, or the refusal in Diagnostic. It writes nothing itself
// (command.LocalDataHandler).
func catAnswer(storePath string, args []string) command.LocalAnswer {
	if len(args) != 1 {
		return command.LocalRefusal(1, "usage: ze data cat <key>\n")
	}

	s, err := openStore(storePath, false)
	if err != nil {
		return storageRefusal(err)
	}
	defer s.Close() //nolint:errcheck // best-effort close

	data, err := s.ReadKey(args[0])
	if err != nil {
		return storageRefusal(err)
	}
	return command.LocalAnswer{Payload: command.Blob(data), Code: 0}
}

// dataCat answers `show data cat`. The store path may be given as --path
// anywhere among the words, as `ze data --path <store> cat <key>` takes it.
//
// On the daemon route the answer is bounded (command.BlobRemoteOctetsMax): a
// value larger than one response carries is refused with its size and the
// local command that reads it on the host holding the store.
func dataCat(validated command.ValidatedArgs) command.LocalAnswer {
	storePath, remaining := extractPathFlag(validated.Tokens())
	answer := catAnswer(storePath, remaining)
	if validated.Route().CallerLocal() {
		return answer
	}
	blob, isBlob := answer.Payload.(command.Blob)
	if !isBlob {
		return answer
	}
	if len(blob) <= command.BlobRemoteOctetsMax {
		return answer
	}
	var tb textbuf.Buffer
	tb.Str("error: ").Str(remaining[0]).Str(" holds ").Int(int64(len(blob))).
		Str(" bytes, more than the ").Int(command.BlobRemoteOctetsMax).
		Str(" a remote answer carries; run 'ze data cat ").Str(remaining[0]).
		Str("' on the host that holds the store\n")
	return command.LocalRefusal(1, tb.String())
}

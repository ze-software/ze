// Design: docs/architecture/api/commands.md — where a command is served
// Related: docs/architecture/api/commands.md — the operator model this serves
//
// storage_data.go answers `show data list` and `show data registered` with
// structured data, so they reach the pipe layer. They printed a table and
// returned an exit code, while YANG declared a wire method for each that no
// daemon handler implements.
//
// `show data cat` answers the BYTES of one stored file, which may be YAML,
// JSON, a certificate or a binary blob. Those bytes are the answer, so it
// answers a command.Blob: the local routes print the bytes unchanged, and a
// route that carries the answer as JSON names the encoding they travel in.

package cli

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/pkg/zefs"
)

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
		rows = append(rows, map[string]any{"key": key})
	}
	return command.LocalAnswer{Payload: map[string]any{"keys": rows}}
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
func dataCat(validated command.ValidatedArgs) command.LocalAnswer {
	storePath, remaining := extractPathFlag(validated.Tokens())
	return catAnswer(storePath, remaining)
}

// Design: docs/architecture/api/commands.md — where a command is served
// Related: docs/architecture/api/commands.md — the operator model this serves
//
// storage_data.go answers `show data list` and `show data registered` with
// structured data, so they reach the pipe layer. They printed a table and
// returned an exit code, while YANG declared a wire method for each that no
// daemon handler implements.
//
// `show data cat` is deliberately NOT converted. It answers the BYTES of one
// stored file, which may be YAML, JSON, a certificate or a binary blob. Those
// bytes are the answer; wrapping them in a record would corrupt the one use the
// command has, and no pipe operator has anything to do with them. It keeps its
// plain handler, and the published page says it reaches no pipe layer, which is
// the truth rather than an omission.

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

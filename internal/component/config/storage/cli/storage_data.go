// Design: docs/architecture/api/commands.md — where a command is served
// Related: plan/spec-cli-pipe-operator-coverage.md — AC-10
//
// storage_data.go answers `show data ls` and `show data registered` with
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
	"fmt"
	"os"

	"github.com/ze-software/ze/pkg/zefs"
)

// writeStorageError reports why the store could not be opened, the same way the
// printers do, so both spellings of the failure agree.
func writeStorageError(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
}

// dataLs answers `show data ls [prefix]`: the keys the store holds.
func dataLs(args []string) (any, int) {
	storePath, remaining := extractPathFlag(args)
	s, err := openStore(storePath)
	if err != nil {
		writeStorageError(err)
		return nil, 2
	}
	defer s.Close() //nolint:errcheck // best-effort close

	prefix := ""
	if len(remaining) > 0 {
		prefix = remaining[0]
	}
	keys := s.List(prefix)
	rows := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, map[string]any{"key": key})
	}
	return map[string]any{"keys": rows}, 0
}

// dataRegistered answers `show data registered [pattern]`: the key patterns the
// code declares, and what each one holds.
func dataRegistered(args []string) (any, int) {
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
	return map[string]any{"patterns": rows}, 0
}

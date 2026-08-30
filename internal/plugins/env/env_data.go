// Design: docs/architecture/api/commands.md — where a command is served
// Related: docs/architecture/api/commands.md — the operator model this serves
//
// env_data.go answers `show env list`, `show env get` and `show env registered`
// with structured data, so they reach the pipe layer.
//
// They used to print a tabwriter table and return an exit code. YANG declared a
// wire method for each and no daemon handler implemented one, so the published
// catalog said `global-pipes: true` and the daemon answered `unknown command`:
//
//	$ ze cli -c "show env list | json"
//	error: unknown command
//
// The root `ze env` command keeps its own printing: it is a developer tool with
// its own layout, and its `.ci` tests assert that layout.

package env

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
)

// envRow is one environment variable as the CLI answers it.
type envRow struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Default     string `json:"default,omitempty"`
	Current     string `json:"current,omitempty"`
	Description string `json:"description,omitempty"`
}

// envRows answers every registered variable, in key order.
func envRows(withCurrent bool) []envRow {
	entries := env.Entries()
	slices.SortFunc(entries, func(a, b env.EnvEntry) int {
		return strings.Compare(a.Key, b.Key)
	})
	rows := make([]envRow, 0, len(entries))
	for _, e := range entries {
		row := envRow{Key: e.Key, Type: e.Type, Default: e.Default, Description: e.Description}
		if withCurrent {
			row.Current = currentValue(e.Key)
		}
		rows = append(rows, row)
	}
	return rows
}

// keyVariables is the payload key each of the three answers carries its rows
// under, so a caller parses one shape whichever command it ran.
const keyVariables = "variables"

// dataList answers `show env list`. The rows carry their effective values,
// because a reader asking a machine for the list wants what is in force.
func dataList(_ []string) (any, int) {
	return map[string]any{keyVariables: envRows(true)}, 0
}

// dataRegistered answers `show env registered`: what the code declares, with no
// effective value, which is the difference between the two commands.
func dataRegistered(_ []string) (any, int) {
	return map[string]any{keyVariables: envRows(false)}, 0
}

// dataGet answers `show env get <key>` with the one variable, or refuses by
// name. It answers the same row shape as the list, so a caller parses one thing.
func dataGet(args []string) (any, int) {
	if len(args) == 0 {
		writeErr("error: show env get requires a key")
		return nil, 1
	}
	key := args[0]
	for _, row := range envRows(true) {
		if row.Key == key {
			return map[string]any{keyVariables: []envRow{row}}, 0
		}
	}
	var tb strings.Builder
	tb.WriteString("error: no environment variable named ")
	tb.WriteString(key)
	writeErr(tb.String())
	return nil, 1
}

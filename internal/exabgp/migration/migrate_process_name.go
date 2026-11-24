// Design: docs/exabgp/exabgp-migration.md -- deterministic endpoint name sanitization for migrated ExaBGP processes
// Related: migrate.go -- migrateProcesses, which names each bus > endpoint entry with these names
// Related: migrate_bus.go -- builds each external bus endpoint from migrated process metadata

package migration

import (
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// endpointNameMax is the maximum length of a bus endpoint name accepted by
// ipc.CheckPluginName (^[a-zA-Z0-9][a-zA-Z0-9-]{0,63}$).
const endpointNameMax = 64

// endpointProcessNames maps each ExaBGP process name to a valid bus endpoint
// name accepted by ipc.CheckPluginName.
//
// Names that already satisfy ipc.ValidPluginName are reserved unchanged first.
// Any remaining name is sanitized deterministically in input order: every
// character outside [a-zA-Z0-9-] (including '_' and '.') is replaced with '-',
// leading '-' characters are stripped (falling back to "p" when nothing
// remains), and the result is truncated to 64 characters. Collisions with any
// reserved or earlier name append -2, -3, ... while truncating the base to fit
// within 64 characters.
func endpointProcessNames(exabgpNames []string) map[string]string {
	names := make(map[string]string, len(exabgpNames))
	taken := make(map[string]bool, len(exabgpNames))
	for _, name := range exabgpNames {
		if ipc.ValidPluginName(name) {
			names[name] = name
			taken[name] = true
		}
	}
	for _, name := range exabgpNames {
		if _, kept := names[name]; kept {
			continue
		}
		base := sanitizeEndpointBase(name)
		candidate := truncateASCII(base, endpointNameMax)
		// Each suffix yields a distinct candidate and taken holds at most
		// len(exabgpNames) entries, so this loop ends within that many steps.
		for suffix := 2; taken[candidate]; suffix++ {
			mark := "-" + strconv.Itoa(suffix)
			prefixLen := max(1, endpointNameMax-len(mark))
			candidate = truncateASCII(base, prefixLen) + mark
		}
		if err := ipc.CheckPluginName(candidate); err != nil {
			panic("BUG: migration: sanitized endpoint name invalid: " + err.Error())
		}
		names[name] = candidate
		taken[candidate] = true
	}
	return names
}

// sanitizeEndpointBase converts an arbitrary ExaBGP process name into a
// non-empty ASCII [a-zA-Z0-9][a-zA-Z0-9-]* base suitable for truncation.
func sanitizeEndpointBase(name string) string {
	var tb textbuf.Buffer
	tb.Grow(len(name))
	for _, r := range name {
		if isEndpointNameChar(r) {
			tb.Byte(byte(r))
		} else {
			tb.Byte('-')
		}
	}
	base := strings.TrimLeft(tb.String(), "-")
	if base == "" {
		return "p"
	}
	return base
}

func isEndpointNameChar(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	return r == '-'
}

// truncateASCII returns s truncated to at most limit bytes.
// Callers pass pure ASCII strings produced by sanitizeEndpointBase.
func truncateASCII(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit]
}

// renamedProcessMark is the part of a warning that says renamedProcessWarning
// wrote it, so a reader of the warnings can tell a rename from other notes.
const renamedProcessMark = ": renamed "

// renamedProcessWarning is the migration's note for a process whose bus >
// endpoint entry carries another name than the ExaBGP config gave it.
func renamedProcessWarning(exabgpName, endpointName string) string {
	var tb textbuf.Buffer
	tb.Str("process ").Str(exabgpName).Str(renamedProcessMark).Str(endpointName).
		Str(" in bus > endpoint (endpoint names must be ").Str(ipc.PluginNameForm).
		Str("); the script, its run line and its peers are unchanged")
	return tb.String()
}

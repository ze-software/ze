// Design: docs/exabgp/exabgp-migration.md -- the name a migrated process keeps in the bridge
// Related: migrate.go -- migrateProcesses, which names each bridge process block with these names

package migration

import (
	"strconv"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// bridgeProcessNameMax is the most characters a bridge process name holds:
// ze-exabgp-bridge-conf.yang, list process, leaf name, `length "1..64"`.
// The config counts a length in characters, not bytes (validateLengths,
// internal/component/config/leaf_value.go), so this does too.
const bridgeProcessNameMax = 64

// bridgeProcessNames answers, for each ExaBGP process name, the name its
// block carries in `exabgp { bridge { process <name> } }`.
//
// The bridge keeps the ExaBGP name (the YANG asks for it, and the bridge
// puts it in its log lines), so a name the bridge can hold is answered
// unchanged. ExaBGP sets no length on a process name and the bridge holds 64
// characters, so a longer name would migrate into a config ze refuses at
// load; such a name is cut to its first 64 characters. Two long names can cut
// to the same text, and a cut can equal a name kept whole, so a cut already
// taken is cut shorter and given the suffix -2, then -3, until it is free.
// Every name kept whole is reserved before any cut is chosen, and the cuts
// are chosen in the order the names come, so the answer depends only on the
// ExaBGP config. The bridge's process key is otherwise a free string, so no
// other character needs mapping. Answers a map with exactly the names given.
func bridgeProcessNames(exabgpNames []string) map[string]string {
	names := make(map[string]string, len(exabgpNames))
	taken := make(map[string]bool, len(exabgpNames))
	for _, name := range exabgpNames {
		if utf8.RuneCountInString(name) <= bridgeProcessNameMax {
			names[name] = name
			taken[name] = true
		}
	}
	for _, name := range exabgpNames {
		if _, kept := names[name]; kept {
			continue
		}
		bridgeName := firstRunes(name, bridgeProcessNameMax)
		// Each suffix gives a different candidate and taken holds at most
		// len(exabgpNames) names, so this loop ends within that many steps.
		for suffix := 2; taken[bridgeName]; suffix++ {
			mark := "-" + strconv.Itoa(suffix)
			bridgeName = firstRunes(name, bridgeProcessNameMax-len(mark)) + mark
		}
		names[name] = bridgeName
		taken[bridgeName] = true
	}
	return names
}

// firstRunes answers the first count characters of s, or s when it is no
// longer.
func firstRunes(s string, count int) string {
	seen := 0
	for offset := range s {
		if seen == count {
			return s[:offset]
		}
		seen++
	}
	return s
}

// renamedProcessMark is the part of a warning that says renamedProcessWarning
// wrote it, so a reader of the warnings can tell a rename from other notes.
const renamedProcessMark = ": renamed "

// renamedProcessWarning is the migration's note for a process whose bridge
// block carries another name than the ExaBGP config gave it.
func renamedProcessWarning(exabgpName, bridgeName string) string {
	var tb textbuf.Buffer
	tb.Str("process ").Str(exabgpName).Str(renamedProcessMark).Str(bridgeName).
		Str(" in the exabgp bridge block, which holds a name of at most ").
		Str(strconv.Itoa(bridgeProcessNameMax)).
		Str(" characters; the script, its run line and its peers are unchanged")
	return tb.String()
}

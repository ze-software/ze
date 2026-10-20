// Design: docs/architecture/exabgp-bridge.md -- the destination keyword is `neighbor` or `peer`
// Related: bridge_command.go -- Translator.Line, the caller
// Related: bridge_version.go -- CommandAPIVersion, which decides a script's version from the same words

package bridge

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// v6ControlSpelling is one ExaBGP 6.0 control word sequence and the v4 words
// ExaBGP itself maps onto it.
type v6ControlSpelling struct {
	v6 string // the v6 words, space separated
	v4 string
}

// v6ControlSpellings is the inverse of ExaBGP's `translate_v4_to_v6`
// (src/exabgp/reactor/api/dispatch/v4.py at the pin): its
// V4_SIMPLE_TRANSLATIONS table and its `show`, `flush`, `clear`, `create` and
// `delete` branches, read from the v6 side. ExaBGP translates the v4 line into
// the v6 one and dispatches that; the bridge translates the v4 spelling, so it
// reads a v6 line in its v4 spelling and from then on both take one path. A v6
// word whose v4 spelling the bridge does not translate is therefore refused in
// both spellings with the same reason.
//
// `system api version` is not here: apiVersionRequest reads it before this
// table is consulted. `peer list` has no v4 spelling and is refused as written.
// The selector forms (`peer <sel> announce ...`) are the `neighbor` selector's
// other spelling and are read by splitNeighborSelector.
var v6ControlSpellings = []v6ControlSpelling{
	{"daemon shutdown", shutdownWord},
	{"daemon reload", "reload"},
	{"daemon restart", "restart"},
	{"daemon status", statusWord},
	{"session ack enable", "enable-ack"},
	{"session ack disable", "disable-ack"},
	{"session ack silence", "silence-ack"},
	{"session sync enable", "enable-sync"},
	{"session sync disable", "disable-sync"},
	{"session reset", "reset"},
	{"session ping", pingWord},
	{"session bye", "bye"},
	{"system help", bridgePassthrough},
	{"system version", "version"},
	{"system crash", "crash"},
	{"system queue-status", "queue-status"},
	{"rib show", "show adj-rib"},
	{"rib flush", "flush adj-rib"},
	{"rib clear", "clear adj-rib"},
	{"peer show", "show neighbor"},
	{"peer create", "create neighbor"},
	{"peer delete", "delete neighbor"},
}

// v4ControlSpelling answers line, whose runs of spaces are already collapsed,
// in its v4 spelling when it opens with a v6 control word sequence, and line
// unchanged otherwise. The words match without regard to case, as ExaBGP's
// dispatcher lowers the line before it walks its tree; the words after them are
// kept as written.
func v4ControlSpelling(line string) string {
	fields := strings.Fields(line)
	for _, spelling := range v6ControlSpellings {
		words := strings.Fields(spelling.v6)
		if !opensWith(fields, words) {
			continue
		}
		var tb textbuf.Buffer
		tb.Str(spelling.v4)
		for _, word := range fields[len(words):] {
			tb.Byte(' ').Str(word)
		}
		return tb.String()
	}
	return line
}

// opensWith reports whether fields begin with words, ignoring case.
func opensWith(fields, words []string) bool {
	if len(fields) < len(words) {
		return false
	}
	for i, word := range words {
		if !strings.EqualFold(fields[i], word) {
			return false
		}
	}
	return true
}

// Design: docs/architecture/testing/ci-format.md -- per-daemon config file naming
//
// A .ci test may spawn more than one long-lived `ze -` daemon (e.g. an IKE
// responder + initiator pair, one cmd=background and one cmd=foreground). Each
// daemon's config is written into the shared per-test tmpfs directory. Writing
// them all to one fixed filename makes the second daemon clobber the first, so
// the pair loads a single config and can never negotiate. zeConfigFileName keys
// the filename on the daemon's stdin block so distinct daemons get distinct
// files, while the first block keeps the ze-bgp.conf name that
// action=rewrite:dest=ze-bgp.conf and restart tests read.

package runner

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// zeDefaultConfigName is the filename the first ze daemon's config is written
// to; action=rewrite:dest=ze-bgp.conf and restart tests read this file.
const zeDefaultConfigName = "ze-bgp.conf"

// zeConfigFileName returns the tmpfs config filename for a ze daemon spawned
// from the given stdin block. The first distinct block in a test gets
// ze-bgp.conf (the name single-daemon and action=rewrite:dest=ze-bgp.conf tests
// read); each additional distinct block gets its own ze-<block>.conf. Reusing a
// block (a restart) returns its already assigned file. The rec map is created
// lazily on first use.
func zeConfigFileName(rec *Record, block string) string {
	if rec.zeConfigFiles == nil {
		rec.zeConfigFiles = make(map[string]string)
	}
	if name, ok := rec.zeConfigFiles[block]; ok {
		return name
	}
	name := zeDefaultConfigName
	if len(rec.zeConfigFiles) > 0 {
		// A distinct additional concurrent daemon: give it a per-block file so
		// it does not overwrite the first daemon's ze-bgp.conf.
		var tb textbuf.Buffer
		name = tb.Str("ze-").Str(sanitizeConfigBlock(block)).Str(".conf").String()
	}
	rec.zeConfigFiles[block] = name
	return name
}

// sanitizeConfigBlock reduces a stdin block name to a filesystem-safe token,
// mapping anything outside [A-Za-z0-9_-] to '-' and falling back to "daemon"
// for an empty result.
func sanitizeConfigBlock(block string) string {
	s := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, block)
	if s == "" {
		return "daemon"
	}
	return s
}

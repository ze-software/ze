// Design: docs/architecture/testing/ci-format.md -- per-daemon config file naming
//
// A .ci test may spawn more than one long-lived `ze -` daemon (e.g. an IKE
// responder + initiator pair, one cmd=background and one cmd=foreground). Each
// daemon needs its own store directory, not merely a distinct config filename.
// The first uses WorkDir so bare-name fixtures still rewrite the actual source.
// Further blocks use numbered subdirectories; restarts reuse their assignment.

package runner

import (
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// zeDefaultConfigName is the filename the first ze daemon's config is written
// to; action=rewrite:dest=ze-bgp.conf and restart tests read this file.
const zeDefaultConfigName = "ze-bgp.conf"

// zeConfigFileName returns a path relative to WorkDir. The first block retains
// ze-bgp.conf; further blocks get distinct directories even if their sanitized
// names collide. Reusing a block reuses its source and storage directory.
func zeConfigFileName(rec *Record, block string) string {
	if rec.zeConfigFiles == nil {
		rec.zeConfigFiles = make(map[string]string)
	}
	if name, ok := rec.zeConfigFiles[block]; ok {
		return name
	}
	name := zeDefaultConfigName
	if len(rec.zeConfigFiles) > 0 {
		// A sequence number distinguishes blocks whose sanitized names collide.
		var tb textbuf.Buffer
		dir := tb.Str("daemon-").Int(int64(len(rec.zeConfigFiles) + 1)).String()
		name = filepath.Join(dir, "ze-"+sanitizeConfigBlock(block)+".conf")
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

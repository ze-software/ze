// Design: docs/architecture/testing/interop.md -- bgp-panic-cease, the ze_le daemon
// Related: session_panic_inject.go -- the shipped half, which injects nothing
// Related: session_read.go -- processMessage, the read-path call
// Related: session_connection.go -- ceaseAfterPanic, what the injected panic proves

//go:build ze_le

package reactor

import (
	"errors"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// readPanicTriggerDir holds the trigger files. A file named
// readPanicTriggerPrefix plus a peer address makes the next UPDATE received
// from that peer panic on the session's read task. The interop scenario
// bgp-panic-cease creates the file from outside the daemon with
// `docker exec`; a test points this at its own temporary directory.
//
// Not safe for concurrent writes: a test sets it before any session runs.
var readPanicTriggerDir = "/run/ze-inject"

// readPanicTriggerPrefix is the trigger file name before the peer address.
const readPanicTriggerPrefix = "bgp-read-panic-"

// injectReadPanic panics when a trigger file exists for peer, after removing
// it, so one trigger raises exactly one panic. It exists only in a build that
// sets the ze_le tag, which no shipped binary does: the shipped half in
// session_panic_inject.go is empty, so a peer can never reach this panic.
//
// It is called for every received UPDATE from processValidatedMessage, the
// function both readers dispatch an UPDATE through, before the UPDATE reaches
// any consumer. That puts the panic inside the read task's boundary
// (Session.Run), the boundary that sends the peer a Cease / Unspecific. A
// coalesced batch meets it once, when the batch is flushed.
//
// Safe for concurrent use: each session asks for its own peer's file, and the
// remove decides which caller fires when two race on one file.
func injectReadPanic(peer netip.Addr) {
	var name textbuf.Buffer
	path := filepath.Join(readPanicTriggerDir, name.Str(readPanicTriggerPrefix).Addr(peer).String())
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		sessionLogger().Warn("the read-panic trigger file exists but cannot be removed, so no panic is injected",
			"peer", peer, "file", path, "error", err)
		return
	}
	panic("BUG: injected read-path panic (ze_le test seam, trigger " + path + ")")
}

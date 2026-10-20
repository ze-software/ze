// Design: docs/architecture/exabgp-bridge.md — the bridge answers `ping` itself
// Related: bridge_ack.go — AnswerLocal writes the pong before the ack
// Related: bridge_command.go — convertControl reads the ping line

package bridge

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// daemonUUID is the identity a ping answers, one per process.
//
// ExaBGP draws its daemon_uuid once when the reactor starts
// (src/exabgp/reactor/loop.py, `str(uuid.uuid4())`), so every helper of one
// daemon reads the same value and a restart reads a new one. The process that
// runs the bridge is the daemon a script talks to, so the value is drawn once
// when this package initializes.
var daemonUUID = newDaemonUUID()

// newDaemonUUID draws a version 4 UUID and prints it as Python's str() does.
func newDaemonUUID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// crypto/rand.Read crashes the program rather than return an error
		// since Go 1.24, so this branch states the contract it relies on.
		panic("BUG: crypto/rand.Read returned an error: " + err.Error())
	}
	// Python's uuid4() sets the same two fields over its random bytes: the
	// version nibble to 4 and the variant bits to 10.
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80

	var text [36]byte
	hex.Encode(text[0:8], raw[0:4])
	text[8] = '-'
	hex.Encode(text[9:13], raw[4:6])
	text[13] = '-'
	hex.Encode(text[14:18], raw[6:8])
	text[18] = '-'
	hex.Encode(text[19:23], raw[8:10])
	text[23] = '-'
	hex.Encode(text[24:36], raw[10:16])
	return string(text[:])
}

// pingAction reads which form of the answer a ping line asked for.
//
// ExaBGP answers JSON unless a word of the line is `text`, compared without
// case (src/exabgp/reactor/api/command/reactor.py, ping). The CLI's own
// `ping <client_uuid> <client_start_time>` arguments are accepted and unused,
// as ExaBGP accepts them. fields is the line already lowercased.
func pingAction(fields []string) LocalAction {
	if slices.Contains(fields, "text") {
		return LocalPingText
	}
	return LocalPing
}

// writePong writes the answer line a ping owes, before its done. Every client
// is active: ExaBGP keeps no list of clients any more and answers true.
//
// JSON:  {"pong": "<uuid>", "active": true}   (json.dumps's separators)
// text:  pong <uuid> active=true
//
// It writes nothing for an action that is not a ping.
func writePong(w io.Writer, action LocalAction) {
	var tb textbuf.Buffer
	//exhaustive:ignore // only the two ping forms carry an answer line; every other local action owes its ack alone
	switch action {
	case LocalPing:
		tb.Str(`{"pong": "`).Str(daemonUUID).Str(`", "active": true}`).Byte('\n')
	case LocalPingText:
		tb.Str("pong ").Str(daemonUUID).Str(" active=true").Byte('\n')
	default:
		return
	}
	if _, err := w.Write(tb.Bytes()); err != nil {
		slog.Debug("bridge ping: write pong failed", "error", err)
	}
}

// Design: docs/architecture/core-design.md — ExaBGP bridge command ack
// Overview: bridge.go — bridge runtime
// Related: bridge_muxconn.go — MuxConn wire format parsing
//
// When the `exabgp.api.ack` OS env var is truthy (default true), the bridge
// emits `done\n` or `error <msg>\n` on the plugin's stdin after each
// dispatched command so the plugin can synchronize on Ze's outcome. When
// false, the bridge stays silent. The ExaBGP convention is the plain text
// name of the env var: the bridge subprocess reads via os.Getenv because it
// runs before Ze's env registry is initialized.

package bridge

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"unicode/utf8"
)

// ackEnvKey is the OS env var the bridge subprocess reads at construction
// time. The parent Ze process writes it via config.ApplyEnvConfig when the
// operator sets `environment { exabgp { api { ack <bool>; } } }`.
const ackEnvKey = "exabgp.api.ack"

// ackMode is a snapshot of exabgp.api.ack captured at bridge construction.
// `true` means emit done/error lines on plugin stdin after each dispatched
// command. Default is true to match ExaBGP's historical behavior.
type ackMode struct {
	enabled bool
}

// newAckMode reads the env once at bridge construction. Later changes to
// the env var are ignored -- operators reload the daemon to pick them up.
func newAckMode() ackMode {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(ackEnvKey)))
	if raw == "" {
		return ackMode{enabled: true}
	}
	if raw == "false" || raw == "0" || raw == "no" || raw == "off" || raw == "disable" || raw == "disabled" {
		return ackMode{enabled: false}
	}
	return ackMode{enabled: true}
}

// writeAck emits `done\n` on the plugin's stdin. No-op when ack mode is
// disabled. Write errors are logged but not returned: a broken pipe means
// the plugin exited, which the bridge's wait loop handles independently.
func (m ackMode) writeAck(pluginW io.Writer) {
	if !m.enabled {
		return
	}
	if _, err := fmt.Fprintln(pluginW, "done"); err != nil {
		slog.Debug("bridge ack: write done failed", "error", err)
	}
}

// writeError emits `error <sanitized message>\n` on the plugin's stdin.
// The message is newline-stripped and length-bounded so a malformed Ze
// error cannot inject additional framing.
func (m ackMode) writeError(pluginW io.Writer, msg string) {
	if !m.enabled {
		return
	}
	clean := sanitizeErrorMessage(msg)
	if _, err := fmt.Fprintf(pluginW, "error %s\n", clean); err != nil {
		slog.Debug("bridge ack: write error failed", "error", err)
	}
}

const maxAckMessageLen = 512

// sanitizeErrorMessage strips \r and \n and truncates to maxAckMessageLen
// bytes on a rune boundary so the emitted ack cannot contain multi-line
// framing or split a multi-byte UTF-8 sequence.
func sanitizeErrorMessage(msg string) string {
	clean := strings.ReplaceAll(msg, "\n", " ")
	clean = strings.ReplaceAll(clean, "\r", " ")
	if len(clean) <= maxAckMessageLen {
		return clean
	}
	end := maxAckMessageLen
	for end > 0 && !utf8.RuneStart(clean[end]) {
		end--
	}
	return clean[:end]
}

// emitAck is the bridge dispatch ack dispatcher: called once per command after
// waiting for ze's response. Keeps the pluginToZebgp hot loop free of
// branching over the (ok, err, timeout) tri-state.
func (b *Bridge) emitAck(pluginW io.Writer, reqID uint64, result pendingResult, err error) {
	if err != nil {
		b.ack.writeError(pluginW, "ze dispatch timeout")
		slog.Warn("plugin->zebgp: dispatch ack wait error", "error", err, "id", reqID)
		return
	}
	if result.ok {
		b.ack.writeAck(pluginW)
		return
	}
	b.ack.writeError(pluginW, result.errText)
}

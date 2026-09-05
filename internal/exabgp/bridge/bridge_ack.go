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

// AckMode is exabgp.api.ack as the bridge currently holds it. `true` means emit
// done/error lines on plugin stdin after each dispatched command. Default is
// true to match ExaBGP's historical behavior.
//
// It is not a constant snapshot: the ExaBGP API lets a script turn acks off and
// on with `disable-ack`, `silence-ack` and `enable-ack`, so Apply moves it.
type AckMode struct {
	enabled bool
}

// AnswerLocal moves the ack state for one local action and writes the ack that
// action still owes, in ONE place.
//
// The decision cannot be split between a caller and WriteAck. `disable-ack` is
// itself acked and silences what follows, so applying it sets enabled false
// while this command still owes a `done`; a caller that asked apply and then
// called WriteAck got silence, because WriteAck re-read the flag apply had just
// cleared. That is a decision made twice: both halves were right and the answer
// was wrong.
func (m *AckMode) AnswerLocal(pluginW io.Writer, action LocalAction) {
	if !m.apply(action) {
		return
	}
	if _, err := fmt.Fprintln(pluginW, "done"); err != nil { //nolint:errcheck // output
		slog.Debug("bridge ack: write done failed", "error", err)
	}
}

// apply moves the ack state for one local action and answers whether THIS
// command is still acked.
//
// The three words differ only in when the silence starts, and that difference
// is the whole of what api-ack-control and api-silence-ack assert:
// `disable-ack` is acked and silences what follows, `silence-ack` is silent
// immediately, and `enable-ack` resumes and is acked.
func (m *AckMode) apply(action LocalAction) bool {
	switch action {
	case LocalAckEnable:
		m.enabled = true
		return true
	case LocalAckDisableAfter:
		acked := m.enabled
		m.enabled = false
		return acked
	case LocalAckDisableNow:
		m.enabled = false
		return false
	case LocalNone:
		return m.enabled
	}
	return m.enabled
}

// NewAckMode reads the env once at bridge construction. Later changes to
// the env var are ignored -- operators reload the daemon to pick them up.
//
// It is exported because the bridge has THREE runners and the ack is owed by
// all of them. Only the standalone MuxConn runner sent one until 2026-09-05, so
// a script driven by the internal or the SDK runner sent its first command,
// waited two seconds for `done`, and gave up: 29 of the 40 ported ExaBGP API
// tests stopped after one frame for that reason alone.
func NewAckMode() AckMode {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(ackEnvKey)))
	if raw == "" {
		return AckMode{enabled: true}
	}
	if raw == "false" || raw == "0" || raw == "no" || raw == "off" || raw == "disable" || raw == "disabled" {
		return AckMode{enabled: false}
	}
	return AckMode{enabled: true}
}

// WriteAck emits `done\n` on the plugin's stdin. No-op when ack mode is
// disabled. Write errors are logged but not returned: a broken pipe means
// the plugin exited, which the bridge's wait loop handles independently.
func (m AckMode) WriteAck(pluginW io.Writer) {
	if !m.enabled {
		return
	}
	if _, err := fmt.Fprintln(pluginW, "done"); err != nil { //nolint:errcheck // output
		slog.Debug("bridge ack: write done failed", "error", err)
	}
}

// WriteError emits `error <sanitized message>\n` on the plugin's stdin.
// The message is newline-stripped and length-bounded so a malformed Ze
// error cannot inject additional framing.
func (m AckMode) WriteError(pluginW io.Writer, msg string) {
	if !m.enabled {
		return
	}
	clean := sanitizeErrorMessage(msg)
	if _, err := fmt.Fprintf(pluginW, "error %s\n", clean); err != nil { //nolint:errcheck // output
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
		b.ack.WriteError(pluginW, "ze dispatch timeout")
		slog.Warn("plugin->zebgp: dispatch ack wait error", "error", err, "id", reqID)
		return
	}
	if result.ok {
		b.ack.WriteAck(pluginW)
		return
	}
	b.ack.WriteError(pluginW, result.errText)
}

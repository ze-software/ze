package reactor

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// warnSink points sessionLogger() at a WARN-level text handler, the level an
// operator runs at by default, and returns the sink.
func warnSink(t *testing.T) *syncBuffer {
	t.Helper()
	sink := &syncBuffer{}
	lg := slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelWarn}))
	t.Cleanup(swapSessionLogger(func() *slog.Logger { return lg }))
	return sink
}

// receivedLines returns the "notification received" lines in out.
func receivedLines(out string) []string {
	var lines []string
	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(line, `msg="notification received"`) {
			lines = append(lines, line)
		}
	}
	return lines
}

// TestHandleNotificationLogsWarn pins the one line a received NOTIFICATION
// leaves at the default log level.
//
// VALIDATES: AC-3 and AC-3n: one WARN line per received NOTIFICATION naming the
// peer, the FSM state, and the code and subcode as names and numbers; an RFC 8203
// shutdown communication rides in that line, and no second line is written for it.
// PREVENTS: a peer refusing Ze's OPEN leaving nothing in the log at WARN, so the
// session never comes up and no line says why (plan/journal/silent-fall-through.md).
func TestHandleNotificationLogsWarn(t *testing.T) {
	t.Run("open refused in OpenSent", func(t *testing.T) {
		sink := warnSink(t)
		s := newOpenSentSession(t)

		// OPEN Message Error (2) / Unspecific (0).
		require.ErrorIs(t, s.handleNotification([]byte{0x02, 0x00}), ErrNotificationRecv)

		lines := receivedLines(sink.String())
		require.Len(t, lines, 1, "one WARN line per received NOTIFICATION:\n%s", sink.String())
		line := lines[0]
		assert.Contains(t, line, "level=WARN")
		assert.Contains(t, line, "peer=192.0.2.1")
		assert.Contains(t, line, "state=OPENSENT")
		assert.Contains(t, line, `code="OPEN Message Error"`)
		assert.Contains(t, line, "code-number=2")
		assert.Contains(t, line, "subcode=Unspecific")
		assert.Contains(t, line, "subcode-number=0")
	})

	t.Run("shutdown communication in Established", func(t *testing.T) {
		sink := warnSink(t)
		s, _ := newEstablishedFakeClockSession(t, 90*time.Second)

		// Cease (6) / Administrative Shutdown (2), RFC 8203 message "maintenance".
		body := []byte{0x06, 0x02, 0x0B, 'm', 'a', 'i', 'n', 't', 'e', 'n', 'a', 'n', 'c', 'e'}
		require.ErrorIs(t, s.handleNotification(body), ErrNotificationRecv)

		out := sink.String()
		lines := receivedLines(out)
		require.Len(t, lines, 1, "one WARN line per received NOTIFICATION:\n%s", out)
		line := lines[0]
		assert.Contains(t, line, "state=ESTABLISHED")
		assert.Contains(t, line, "code=Cease")
		assert.Contains(t, line, "code-number=6")
		assert.Contains(t, line, `subcode="Administrative Shutdown"`)
		assert.Contains(t, line, "subcode-number=2")
		assert.Contains(t, line, "message=maintenance")
		assert.Equal(t, 1, strings.Count(out, "maintenance"), "the message is logged once:\n%s", out)
	})

	t.Run("invalid shutdown communication", func(t *testing.T) {
		sink := warnSink(t)
		s := newOpenSentSession(t)

		// Cease (6) / Administrative Shutdown (2), length 3, invalid UTF-8.
		require.ErrorIs(t, s.handleNotification([]byte{0x06, 0x02, 0x03, 0xFF, 0xFE, 0xFD}), ErrNotificationRecv)

		out := sink.String()
		lines := receivedLines(out)
		require.Len(t, lines, 1, "one WARN line per received NOTIFICATION:\n%s", out)
		assert.Contains(t, lines[0], "message-error=")
		assert.NotContains(t, out, "invalid shutdown communication")
	})
}

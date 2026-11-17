// Design: docs/architecture/wire/messages.md -- emitted OPEN Hold Time range.
// Related: session_negotiate.go -- sendOpen; message/open.go -- writeFixedFields.
package reactor

import (
	"bufio"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// Goal: inspect the Hold Time octets actually emitted by the session OPEN writer.
// Method: keep NewPeerSettings' default unchanged, then exercise legal overrides.
// VALIDATES: one complete OPEN carries a legal Hold Time and the selected value.
// PREVENTS: a serializer emitting forbidden one/two while settings still look valid.
//
// RFC requirement: RFC4271-4.2-1 positive -- actual sendOpen output with unchanged
// default settings carries Hold Time 90, which is zero or at least three seconds.
// Configured zero and three are supplementary option-preservation controls. This
// observes flushed connection-writer bytes, not remote acceptance or negotiation;
// RFC 4271 Section 6.2 permits rejecting any proposed Hold Time.
func TestRFC4271OpenHoldTimeOnWire(t *testing.T) {
	tests := []struct {
		name       string
		configured bool
		seconds    uint16
	}{
		{"default", false, 90},
		{"configured zero", true, 0},
		{"configured minimum", true, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
			if tc.configured {
				settings.ReceiveHoldTime = time.Duration(tc.seconds) * time.Second
			}
			session := NewSession(settings)
			t.Cleanup(session.timers.StopAll)
			conn := &recordingConn{}
			session.bufWriter = bufio.NewWriter(conn)
			require.NoError(t, session.sendOpen(conn))
			stream := conn.written()
			require.GreaterOrEqual(t, len(stream), message.HeaderLen+10)
			require.Equal(t, len(stream), int(binary.BigEndian.Uint16(stream[16:18])), "one complete OPEN is the captured output")
			frames := splitMessages(t, stream)
			require.Len(t, frames, 1)
			require.Equal(t, byte(msgtype.TypeOPEN), frames[0][18])
			hold := binary.BigEndian.Uint16(frames[0][message.HeaderLen+3 : message.HeaderLen+5])
			require.True(t, hold == 0 || hold >= 3, "serialized OPEN Hold Time %d must be zero or at least three", hold)
			require.Equal(t, tc.seconds, hold, "OPEN carries the default or explicitly configured Hold Time")
		})
	}
}

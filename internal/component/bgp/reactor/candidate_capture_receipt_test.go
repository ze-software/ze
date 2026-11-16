// Design: docs/architecture/bgp/protocol-event-capture.md -- deferred identity and close ordering.
// Related: capture_pending.go -- the one-shot prepared-OPEN receipt.
package reactor

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/capture"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestPreparedCaptureReceiptBeforeCloseDrainsOriginalInput queues selection,
// successful proposal and retirement before starting the worker. Every signal
// is then ready at once; Close must still drain the selected original input
// with that proposal's identity and the input's original start timestamp.
func TestPreparedCaptureReceiptBeforeCloseDrainsOriginalInput(t *testing.T) {
	for range 20 {
		settings := testCapturePeer(t.TempDir())
		fc := sim.NewFakeClock(time.Unix(1700000000, 0))
		pending, publisher := newProposalSessionCapture(settings, false, fc, nil, nil)
		wire := message.PackTo(&message.Keepalive{}, nil)
		pending.recordConfig(fc.Now(), capture.OpCommit, "before-prepared-receipt", []byte(`{"hold-time":90}`))
		fc.Add(time.Nanosecond)
		pending.recordMessage(fc.Now(), 4, wire, 7, 9)
		settings.PeerAS = 65002
		session := NewSession(settings)
		require.NoError(t, session.prepareOpen())
		proposal, err := session.openProposal()
		require.NoError(t, err)
		session.stopSessionTimers()
		pending.selectForPublication()
		publisher.publish(proposal)
		pending.Close()
		header, events := readEvents(t, pending.Path())
		require.Equal(t, uint32(65002), header.PeerAS)
		require.Equal(t, time.Unix(1700000000, 0).UTC().Format(capture.TimeFormat), header.Started)
		incomingCaptureConfig(t, events, "before-prepared-receipt")
		messages := 0
		for _, event := range events {
			if event.Type != capture.EventMessage {
				continue
			}
			messages++
			require.True(t, bytes.Equal(event.Data, wire))
			require.Equal(t, uint32(7), event.SourceID)
			require.Equal(t, uint16(9), event.CtxID)
			require.Equal(t, fc.Now().UTC().Format(capture.TimeFormat), event.TS)
		}
		require.Equal(t, 1, messages)
		require.Equal(t, capture.SessionCaptureStop, events[len(events)-1].Event)
	}
}

// Design: docs/architecture/bgp/protocol-event-capture.md -- replay identity follows the actual OPEN proposal.
// Related: candidate_router_id_test.go -- real factory Sessions and dynamic AS publication.
package reactor

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/capture"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestCaptureIdentityFollowsPreparedDynamicProposal creates the capture input
// before the incumbent publishes its learned AS, then prepares the candidate
// on either side of that publication. Selection may precede preparation, but
// the file must describe the candidate's actual immutable proposal and retain
// its earlier input timestamps and configuration operation.
func TestCaptureIdentityFollowsPreparedDynamicProposal(t *testing.T) {
	for _, prepareAfterPublication := range []bool{false, true} {
		name := "prepared-before-publication"
		if prepareAfterPublication {
			name = "prepared-after-publication"
		}
		t.Run(name, func(t *testing.T) {
			settings := NewPeerSettings(mustParseAddr("127.0.0.1"), 65000, 0, 0x01010101)
			settings.Connection = ConnectionPassive
			settings.IsDynamic = true
			peer := NewPeer(settings)
			incumbent, incumbentRemote := candidateRouterIDSession(t, peer, 0x02020202)
			peer.mu.Lock()
			peer.session = incumbent
			peer.settings.Capture = testCaptureSettings(t.TempDir())
			peer.mu.Unlock()
			fc := sim.NewFakeClock(time.Now())
			peer.SetClock(fc)
			candidate := peer.newSession()
			pending, ok := candidate.captureWriter.(*pendingSessionCapture)
			require.True(t, ok, "the real factory installs its immutable pending input")
			t.Cleanup(func() {
				candidate.closeConn()
				candidate.stopSessionTimers()
				peer.stopCapture(pending)
			})
			started := fc.Now()
			pending.recordConfig(fc.Now(), capture.OpCommit, "before-proposal", []byte(`{"hold-time":90}`))
			fc.Add(time.Nanosecond)
			require.NoError(t, candidate.Start())
			remote, local := collisionProofTCPPair(t)
			if prepareAfterPublication {
				// Ordinary publication can select an input before TCP prepares OPEN.
				pending.selectForPublication()
			} else {
				require.NoError(t, candidate.Accept(local))
				delayOpenReadType(t, remote, 1)
			}
			delayOpenWrite(t, incumbentRemote, &message.Keepalive{})
			require.NoError(t, incumbent.ReadAndProcess())
			require.Equal(t, fsm.StateEstablished, incumbent.State())
			peer.resolveDynamicPeerSettings(incumbent)
			require.Equal(t, uint32(65001), peer.PeerAS(), "production dynamic resolution actually published the AS")
			if prepareAfterPublication {
				require.NoError(t, candidate.Accept(local))
				delayOpenReadType(t, remote, 1)
			} else {
				pending.selectForPublication()
			}
			remoteOpen := delayOpenRemote(0x03030303)
			wire := message.PackTo(remoteOpen, nil)
			observed := fc.Now()
			delayOpenWrite(t, remote, remoteOpen)
			require.NoError(t, candidate.ReadAndProcess())
			delayOpenReadType(t, remote, 4)
			peer.stopCapture(pending)
			header, events := readEvents(t, pending.Path())
			require.Equal(t, uint32(65000), header.LocalAS)
			require.Equal(t, uint32(0x01010101), header.RouterID)
			var expectedPeerAS uint32
			if prepareAfterPublication {
				expectedPeerAS = 65001
			}
			require.Equal(t, expectedPeerAS, header.PeerAS, "replay identity must follow preparation rather than factory construction or later publication")
			require.Equal(t, started.UTC().Format(capture.TimeFormat), header.Started)
			incomingCaptureConfig(t, events, "before-proposal")
			messages := 0
			for _, event := range events {
				if event.Type != capture.EventMessage {
					continue
				}
				messages++
				require.True(t, bytes.Equal(wire, event.Data), "activation must retain the original remote OPEN")
				require.Equal(t, observed.UTC().Format(capture.TimeFormat), event.TS)
			}
			require.Equal(t, 1, messages)
			require.Equal(t, capture.SessionCaptureStop, events[len(events)-1].Event)
			incomingCaptureFiles(t, filepath.Dir(pending.Path()), 1)
		})
	}
}

// TestSelectedUnpreparedCaptureClosesWithoutFile selects a factory input before
// TCP starts, then retires its Session. Close must join the waiting worker and
// preserve the absence of a file whose replay identity was never prepared.
func TestSelectedUnpreparedCaptureClosesWithoutFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-prepared")
	settings := testCapturePeer(dir)
	peer := NewPeer(settings)
	session := peer.newSession()
	pending, ok := session.captureWriter.(*pendingSessionCapture)
	require.True(t, ok)
	pending.recordConfig(peer.clock.Now(), capture.OpCommit, "before-tcp", []byte(`{"hold-time":90}`))
	pending.selectForPublication()
	peer.stopCapture(pending)
	session.stopSessionTimers()
	_, err := os.Stat(dir)
	require.True(t, errors.Is(err, os.ErrNotExist), "an unprepared Session must not create or rotate a capture: %v", err)
}

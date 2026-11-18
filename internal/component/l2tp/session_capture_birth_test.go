// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact selection across reusable labels

package l2tp

import (
	"crypto/rand"
	"io"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// TestL2TPSessionCaptureRejectsOriginalBirthAfterRealSIDReuse allocates both
// rows through the real ICRQ producer, forcing only its two-byte wire-ID draw.
// The first capture and its old guard must never observe or clear the second
// row after the real removal and reallocation reuse its numeric ID.
func TestL2TPSessionCaptureRejectsOriginalBirthAfterRealSIDReuse(t *testing.T) {
	captureBirthSIDDraw(t)
	reactor := newReactorForSnapshot(t)
	insertEstablishedTunnel(t, reactor, 100, 200, netip.MustParseAddrPort("192.0.2.1:1701"))
	tunnel := reactor.tunnelsByLocalID[100]
	captureBirthAdmit(t, reactor, tunnel, 70)
	originalRow := tunnel.sessions[7]
	require.NotNil(t, originalRow)
	subsystem := NewSubsystem(&Parameters{})
	subsystem.reactors = []*l2tpReactor{reactor}
	selector := SessionSelector{SID: 7, TID: 100}
	original, err := subsystem.CaptureSession(selector)
	require.NoError(t, err)
	require.NotNil(t, original)
	originalBirth := original.Birth()
	require.True(t, originalBirth.IsValid())
	before, err := original.Snapshot()
	require.NoError(t, err)
	require.EqualValues(t, 7, before.LocalSID)
	require.EqualValues(t, 100, before.TunnelLocalTID)

	tunnel.removeSession(7, l2tpevents.TerminateCauseAdminReset)
	// RFC 2661 Section 5.8: the synthetic peer acknowledges the first ICRP
	// through the real ZLB receive path before requesting its next call.
	ack := tunnel.engine.OnReceive(MessageHeader{IsControl: true, Nr: tunnel.engine.nextSendSeq}, nil, reactor.params.Clock())
	require.Equal(t, ClassZLB, ack.Class)
	require.Equal(t, 1, ack.Acked)
	captureBirthAdmit(t, reactor, tunnel, 71)
	replacementRow := tunnel.sessions[7]
	require.NotNil(t, replacementRow)
	require.NotSame(t, originalRow, replacementRow)
	selector.Birth = originalBirth
	stale, err := subsystem.CaptureSession(selector)
	require.ErrorIs(t, err, ErrSessionNotFound)
	require.Nil(t, stale)
	_, err = original.Snapshot()
	require.ErrorIs(t, err, ErrSessionNotFound)
	require.Error(t, original.Teardown(t.Context(), l2tpevents.TerminateCauseAdminReset, "old browser page"))
	require.Same(t, replacementRow, tunnel.sessions[7], "the old capture must leave its successor installed")

	fresh, err := subsystem.CaptureSession(SessionSelector{SID: 7, TID: 100})
	require.NoError(t, err)
	require.NotNil(t, fresh)
	require.True(t, fresh.Birth().IsValid())
	require.NotEqual(t, originalBirth, fresh.Birth())
	after, err := fresh.Snapshot()
	require.NoError(t, err)
	require.EqualValues(t, 7, after.LocalSID)
	require.EqualValues(t, 100, after.TunnelLocalTID)
}

// TestL2TPSessionCaptureRefusesEqualSIDAcrossRealTunnels exercises two actual
// ICRQ allocations with the same wire SID. The unqualified selector refuses,
// while a tunnel-qualified selector retains the exact useful original row.
func TestL2TPSessionCaptureRefusesEqualSIDAcrossRealTunnels(t *testing.T) {
	captureBirthSIDDraw(t)
	reactor := newReactorForSnapshot(t)
	insertEstablishedTunnel(t, reactor, 100, 200, netip.MustParseAddrPort("192.0.2.1:1701"))
	insertEstablishedTunnel(t, reactor, 101, 201, netip.MustParseAddrPort("192.0.2.2:1701"))
	captureBirthAdmit(t, reactor, reactor.tunnelsByLocalID[100], 70)
	captureBirthAdmit(t, reactor, reactor.tunnelsByLocalID[101], 71)
	subsystem := NewSubsystem(&Parameters{})
	subsystem.reactors = []*l2tpReactor{reactor}
	missing, err := subsystem.CaptureSession(SessionSelector{SID: 7})
	require.ErrorIs(t, err, ErrSessionAmbiguous)
	require.Nil(t, missing)
	for _, tid := range []uint16{100, 101} {
		captured, err := subsystem.CaptureSession(SessionSelector{SID: 7, TID: tid})
		require.NoError(t, err)
		require.NotNil(t, captured)
		require.True(t, captured.Birth().IsValid())
		snapshot, err := captured.Snapshot()
		require.NoError(t, err)
		require.Equal(t, tid, snapshot.TunnelLocalTID)
	}
}

// captureBirthSIDReader changes only the real two-byte SID entropy draw.
// All logical-birth entropy continues through the captured original reader.
type captureBirthSIDReader struct{ original io.Reader }

func (reader captureBirthSIDReader) Read(buffer []byte) (int, error) {
	if len(buffer) == 2 {
		buffer[0], buffer[1] = 0, 7
		return len(buffer), nil
	}
	return io.ReadFull(reader.original, buffer)
}

func captureBirthSIDDraw(t *testing.T) {
	t.Helper()
	original := rand.Reader
	rand.Reader = captureBirthSIDReader{original: original}
	t.Cleanup(func() { rand.Reader = original })
}

func captureBirthAdmit(t *testing.T, reactor *l2tpReactor, tunnel *L2TPTunnel, remoteSID uint16) {
	t.Helper()
	var packet [512]byte
	// RFC 2661 Section 6.6: the ICRQ starts real incoming-call allocation.
	length := writeICRQBody(packet[:], remoteSID, uint32(remoteSID), 0, "", "")
	// RFC 2661 Section 6.6.
	require.NotEmpty(t, tunnel.handleICRQ(packet[:length], reactor.params.Clock(), reactor.logger))
	require.NotNil(t, tunnel.sessions[7], "the allocator must publish the actual forced wire SID")
}

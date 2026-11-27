// Design: docs/architecture/diagnostics/production-diagnostics.md -- BGP cold-path bad-packet capture
package reactor

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/badpacket"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func badPacketTestSession(addr string) *Session {
	settings := NewPeerSettings(netip.MustParseAddr(addr), 65001, 65002, 0x01020301)
	settings.ReceiveHoldTime = 90 * time.Second
	return NewSession(settings)
}

func newestBGPRecord(t *testing.T, peer string) badpacket.PacketRecord {
	t.Helper()
	recs := badpacket.Snapshot(badpacket.MaxRecords, "bgp")
	for _, rec := range recs {
		if rec.Peer == peer {
			return rec
		}
	}
	t.Fatalf("no badpacket record found for bgp peer %s (total snapshot=%d)", peer, len(recs))
	return badpacket.PacketRecord{}
}

// malformedAtomicAggregateUpdate carries ATOMIC_AGGREGATE (code 6) with length 1
// (RFC 7606 Section 7.6 specifies attribute-discard for non-zero length).
func malformedAtomicAggregateUpdate() []byte {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN: IGP
		0x40, 0x02, 0x04, 0x02, 0x01, 0xfd, 0xea, // AS_PATH: 65002 (2-byte, matches unnegotiated session)
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP
		0x40, 0x06, 0x01, 0x00, // ATOMIC_AGGREGATE with invalid length 1 -> attribute-discard
	}
	return makeUpdateBody(nil, attrs, []byte{0x18, 0xc0, 0x00, 0x02}) // 192.0.2.0/24
}

func validAnnouncementUpdate() []byte {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN: IGP
		0x40, 0x02, 0x04, 0x02, 0x01, 0xfd, 0xea, // AS_PATH: 65002 (2-byte, matches unnegotiated session)
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP
	}
	return makeUpdateBody(nil, attrs, []byte{0x18, 0xc0, 0x00, 0x02}) // 192.0.2.0/24
}

func TestSessionBadPacketRFC7606Actions(t *testing.T) {
	t.Run("treat-as-withdraw", func(t *testing.T) {
		peer := "192.0.2.101"
		s := badPacketTestSession(peer)
		body := malformedOriginUpdate()
		wantHex := hex.EncodeToString(body)
		before := badpacket.Count()

		_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
		require.NoError(t, err)
		require.Equal(t, message.RFC7606ActionTreatAsWithdraw, action)
		require.Equal(t, before+1, badpacket.Count())

		rec := newestBGPRecord(t, peer)
		assert.Equal(t, "bgp", rec.Protocol)
		assert.Equal(t, peer, rec.Peer)
		assert.Equal(t, "UPDATE", rec.MessageType)
		assert.Equal(t, "treat-as-withdraw", rec.Action)
		assert.NotEmpty(t, rec.Reason)
		assert.Equal(t, len(body), rec.Length)
		assert.Equal(t, wantHex, rec.Hex)
	})

	t.Run("attribute-discard-preserves-received-bytes", func(t *testing.T) {
		peer := "192.0.2.102"
		s := badPacketTestSession(peer)
		body := malformedAtomicAggregateUpdate()
		// Snapshot original hex before ApplyAttrDiscard can tombstone in-place.
		wantHex := hex.EncodeToString(body)
		before := badpacket.Count()

		_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
		require.NoError(t, err)
		require.Equal(t, message.RFC7606ActionAttributeDiscard, action)
		require.Equal(t, before+1, badpacket.Count())

		rec := newestBGPRecord(t, peer)
		assert.Equal(t, "bgp", rec.Protocol)
		assert.Equal(t, peer, rec.Peer)
		assert.Equal(t, "UPDATE", rec.MessageType)
		assert.Equal(t, "attribute-discard", rec.Action)
		assert.NotEmpty(t, rec.Reason)
		assert.Equal(t, len(body), rec.Length)
		assert.Equal(t, wantHex, rec.Hex)
	})

	t.Run("session-reset", func(t *testing.T) {
		peer := "192.0.2.103"
		s := badPacketTestSession(peer)
		// Withdrawn Routes Length larger than the message: RFC 7606 Section 3(b).
		body := []byte{0x00, 0x20, 0x00, 0x00}
		wantHex := hex.EncodeToString(body)
		before := badpacket.Count()

		_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
		require.Error(t, err)
		require.Equal(t, message.RFC7606ActionSessionReset, action)
		require.Equal(t, before+1, badpacket.Count())

		rec := newestBGPRecord(t, peer)
		assert.Equal(t, "bgp", rec.Protocol)
		assert.Equal(t, peer, rec.Peer)
		assert.Equal(t, "UPDATE", rec.MessageType)
		assert.Equal(t, "session-reset", rec.Action)
		assert.NotEmpty(t, rec.Reason)
		assert.Equal(t, len(body), rec.Length)
		assert.Equal(t, wantHex, rec.Hex)
	})

	t.Run("valid-update-does-not-record", func(t *testing.T) {
		peer := "192.0.2.104"
		s := badPacketTestSession(peer)
		body := validAnnouncementUpdate()
		before := badpacket.Count()

		_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
		require.NoError(t, err)
		require.Equal(t, message.RFC7606ActionNone, action)
		require.Equal(t, before, badpacket.Count())
	})
}

func TestSessionBadPacketHeaderErrors(t *testing.T) {
	corruptMarkerHeader := make([]byte, message.HeaderLen)
	for i := range 16 {
		corruptMarkerHeader[i] = 0xff
	}
	corruptMarkerHeader[0] = 0x00 // Corrupt marker byte
	binary.BigEndian.PutUint16(corruptMarkerHeader[16:18], message.HeaderLen)
	corruptMarkerHeader[18] = byte(msgtype.TypeKEEPALIVE)

	oversizeHeader := make([]byte, message.HeaderLen)
	for i := range 16 {
		oversizeHeader[i] = 0xff
	}
	binary.BigEndian.PutUint16(oversizeHeader[16:18], message.MaxMsgLen+1)
	oversizeHeader[18] = byte(msgtype.TypeUPDATE)

	modes := []struct {
		name     string
		coalesce bool
	}{
		{name: "standard", coalesce: false},
		{name: "coalesced", coalesce: true},
	}

	cases := []struct {
		name   string
		header []byte
		peer   string
	}{
		{name: "corrupt-marker", header: corruptMarkerHeader, peer: "192.0.2.111"},
		{name: "bad-length", header: oversizeHeader, peer: "192.0.2.112"},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					s := badPacketTestSession(tc.peer)
					s.coalesceEnabled = mode.coalesce
					reader := bufio.NewReader(bytes.NewReader(tc.header))
					before := badpacket.Count()

					var err error
					if mode.coalesce {
						err = s.readAndProcessCoalesced(nil, reader)
					} else {
						err = s.readAndProcessMessage(nil, reader)
					}
					require.Error(t, err)
					require.Equal(t, before+1, badpacket.Count())

					rec := newestBGPRecord(t, tc.peer)
					assert.Equal(t, "bgp", rec.Protocol)
					assert.Equal(t, tc.peer, rec.Peer)
					assert.Equal(t, "HEADER", rec.MessageType)
					assert.Equal(t, "header-error", rec.Action)
					assert.NotEmpty(t, rec.Reason)
					assert.Equal(t, message.HeaderLen, rec.Length)
					assert.Equal(t, hex.EncodeToString(tc.header), rec.Hex)
				})
			}
		})
	}
}

func TestSessionBadPacketPanicCapture(t *testing.T) {
	t.Run("owned-buffer-standard-read", func(t *testing.T) {
		assertPanicCapturesFrame(t, false, "192.0.2.121", eorUpdate(), eorUpdate())
	})

	t.Run("owned-buffer-coalesced-direct", func(t *testing.T) {
		assertPanicCapturesFrame(t, true, "192.0.2.122", eorUpdate(), eorUpdate())
	})

	t.Run("owned-buffer-coalesced-flush", func(t *testing.T) {
		body := validAnnouncementUpdate()
		wireMsg := buildUpdateMsg(body)
		// flushCoalesce holds the synthetic UPDATE body (without the 19-byte BGP header).
		assertPanicCapturesFrame(t, true, "192.0.2.123", wireMsg, body)
	})

	t.Run("transferred-buffer-not-read", func(t *testing.T) {
		s := badPacketTestSession("192.0.2.124")
		buf := s.getReadBuffer()
		require.NotNil(t, buf.Buf)
		defer s.returnReadBuffer(buf)

		frame := eorUpdate()
		copy(buf.Buf, frame)
		frameLen := len(frame)
		kept := false
		wantCause := errors.New("transferred callback panic")

		var gotPanic any
		func() {
			defer func() { gotPanic = recover() }()
			defer s.finishReadBuffer(buf, &frameLen, &kept)
			panic(transferredReadBufferPanic{cause: wantCause, buffer: buf})
		}()

		require.Equal(t, wantCause, gotPanic)
		require.True(t, kept, "transferredReadBufferPanic must mark buffer as kept")
		require.Nil(t, s.panicFrame.Load(), "transferred buffer must never be copied into panicFrame")
		require.True(t, s.panicFrameSuppressed.Load(), "transferredReadBufferPanic must suppress outer panic frame capture")
		described := s.describeRead()
		assert.Empty(t, described.RawFrame)
		assert.False(t, s.panicFrameSuppressed.Load(), "describeRead must clear panicFrameSuppressed")
	})

	t.Run("coalesced-flush-transferred-panic-does-not-misattribute-outer-frame", func(t *testing.T) {
		s := badPacketTestSession("192.0.2.127")
		outerBuf := s.getReadBuffer()
		require.NotNil(t, outerBuf.Buf)
		innerBuf := s.getReadBuffer()
		require.NotNil(t, innerBuf.Buf)
		defer s.returnReadBuffer(innerBuf)

		outerFrame := eorUpdate()
		copy(outerBuf.Buf, outerFrame)
		outerLen := len(outerFrame)
		outerKept := false

		innerFrame := validAnnouncementUpdate()
		copy(innerBuf.Buf, innerFrame)
		innerLen := len(innerFrame)
		innerKept := false

		wantCause := errors.New("transferred coalesced flush panic")

		var gotPanic any
		func() {
			defer func() { gotPanic = recover() }()
			// Outer readAndProcessCoalesced holds Packet 2 (owned, !outerKept).
			defer s.finishReadBuffer(outerBuf, &outerLen, &outerKept)
			// Inner flushCoalesce holds Packet 1 (already transferred on panic).
			func() {
				defer s.finishReadBuffer(innerBuf, &innerLen, &innerKept)
				panic(transferredReadBufferPanic{cause: wantCause, buffer: innerBuf})
			}()
		}()

		require.Equal(t, wantCause, gotPanic)
		require.True(t, innerKept, "inner transferred buffer must remain kept")
		require.False(t, outerKept, "outer owned buffer must still be returned to pool")
		require.Nil(t, s.panicFrame.Load(), "outer defer must not misattribute unrelated packet after inner transferredReadBufferPanic")
		described := s.describeRead()
		assert.Empty(t, described.RawFrame)
		assert.False(t, s.panicFrameSuppressed.Load(), "describeRead must clear panicFrameSuppressed")
	})

	t.Run("describeCancel-isolates-read-fields", func(t *testing.T) {
		s := badPacketTestSession("192.0.2.125")
		s.receivedType = msgtype.TypeUPDATE
		sample := []byte{0x01, 0x02, 0x03}
		s.panicFrame.Store(&sample)

		cancelCtx := s.describeCancel()
		assert.Equal(t, "192.0.2.125", cancelCtx.Unit)
		assert.Empty(t, cancelCtx.MessageType)
		assert.Empty(t, cancelCtx.RawFrame)
		// panicFrame must remain untouched for describeRead.
		require.NotNil(t, s.panicFrame.Load())
		readCtx := s.describeRead()
		assert.Equal(t, sample, readCtx.RawFrame)
		assert.Nil(t, s.panicFrame.Load())
	})

	t.Run("finishReadBuffer-happy-path-zero-allocs", func(t *testing.T) {
		s := badPacketTestSession("192.0.2.126")
		allocs := testing.AllocsPerRun(100, func() {
			buf := s.getReadBuffer()
			kept := false
			frameLen := message.HeaderLen
			func() {
				defer s.finishReadBuffer(buf, &frameLen, &kept)
			}()
		})
		assert.Equal(t, 0.0, allocs, "finishReadBuffer happy path must allocate zero bytes")
	})
}

func assertPanicCapturesFrame(t *testing.T, coalesce bool, peerAddr string, sendWire, wantCaptured []byte) {
	t.Helper()
	restore := crashlog.SetCrashDirForTest(t.TempDir())
	t.Cleanup(restore)

	settings := NewPeerSettings(netip.MustParseAddr(peerAddr), 65001, 65002, 0x01020301)
	session := NewSession(settings)
	establishUpdateReadFixture(t, session)
	session.coalesceEnabled = coalesce
	client, _, drainErr := pipeSession(t, session)
	defer func() { _ = client.Close() }()

	_, inUseBefore := bufMuxStd.Stats()
	beforeCount := badpacket.Count()

	session.onMessageReceived = func(_ *Session, _ netip.Addr, kind msgtype.MessageType, _ []byte,
		_ *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		if kind != msgtype.TypeUPDATE {
			return false
		}
		if direction != rpc.DirectionReceived {
			return false
		}
		panic("injected badpacket panic")
	}

	resultCh := make(chan runOutcome, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultCh <- runOutcome{escaped: r}
			}
		}()
		resultCh <- runOutcome{err: session.Run(t.Context())}
	}()
	go func() { _, _ = client.Write(sendWire) }()

	var outcome runOutcome
	select {
	case outcome = <-resultCh:
	case <-time.After(runExitDeadline):
		t.Fatal("Run did not return within deadline")
	}

	require.Nil(t, outcome.escaped)
	var contained *taskguard.Panic
	require.True(t, errors.As(outcome.err, &contained))
	require.Equal(t, wantCaptured, contained.RawFrame)
	requireConnClosed(t, drainErr)

	_, inUseAfter := bufMuxStd.Stats()
	require.Equal(t, inUseBefore, inUseAfter, "panicking read buffer must be returned to pool")

	require.Equal(t, beforeCount+1, badpacket.Count())
	rec := newestBGPRecord(t, peerAddr)
	assert.Equal(t, "bgp", rec.Protocol)
	assert.Equal(t, peerAddr, rec.Peer)
	assert.Equal(t, "UPDATE", rec.MessageType)
	assert.Equal(t, "task-panic", rec.Action)
	assert.Equal(t, "injected badpacket panic", rec.Reason)
	assert.Equal(t, len(wantCaptured), rec.Length)
	assert.Equal(t, hex.EncodeToString(wantCaptured), rec.Hex)
}

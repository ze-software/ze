// Design: docs/architecture/mrt.md — original packets, not semantic surrogates.
package reactor

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/netip"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/mrt"
	mrtplugin "github.com/ze-software/ze/internal/plugins/mrt"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// recordSessionWire guarantees Stop on failure. The caller MUST also invoke
// the returned idempotent stop before reading the file, to flush queued records.
func recordSessionWire(t *testing.T, session *Session) (string, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "wire.mrt")
	recorder := mrtplugin.New(mrtplugin.Config{AllPath: path}, nil)
	recorder.Start(nil)
	// Start MUST be paired with Stop even when an assertion terminates the test.
	stop := sync.OnceFunc(recorder.Stop)
	t.Cleanup(stop)
	r := New(&Config{})
	r.addMessageObserver(recorder)
	session.onWireMessage = func(wire []byte, ctx bgpctx.ContextID, sent bool, _ *sessionTransport) {
		r.notifyWireMessage(&plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.1"), LocalAddress: netip.MustParseAddr("192.0.2.2"), PeerAS: 65002, LocalAS: 65001, MessageContextID: ctx}, wire, sent)
	}
	return path, stop
}

// TestRFC6396WireCaptureIgnoresSemanticSynthesis observes a real two-family TAW
// UPDATE and a reset-causing complete UPDATE through Session.ReadAndProcess.
// RFC requirement: RFC6396-4.4.2-2 positive -- the actual MRT producer records each complete original received UPDATE exactly once even when validation synthesizes two withdrawals or resets the session; synthetic callbacks never become extra wire records and rejection never suppresses the original packet.
func TestRFC6396WireCaptureIgnoresSemanticSynthesis(t *testing.T) {
	for _, reset := range []bool{false, true} {
		s, client, cleanup := setupEstablishedSessionTwoFamilies(t)
		path, stop := recordSessionWire(t, s)
		semantic := 0
		s.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte, _ *wireu.WireUpdate, _ bgpctx.ContextID, d rpc.MessageDirection, _ BufHandle, _ map[string]any, _ string, _ uint64) bool {
			if d == rpc.DirectionReceived {
				semantic++
			}
			return false
		}
		body := buildTwoFamilyTreatAsWithdrawUpdate()
		if reset {
			body = []byte{0, 9, 0, 0}
		} // Complete frame, impossible withdrawn length.
		original := buildUpdateMsg(body)
		go sendUpdateAndDrain(client, original)
		err := s.ReadAndProcess()
		if reset {
			require.Error(t, err)
			require.Zero(t, semantic)
		} else {
			require.NoError(t, err)
			require.Equal(t, 2, semantic)
		}
		stop()
		cleanup()
		count := 0
		err = mrt.ReadFile(path, &mrt.Handler{OnMessage: func(h mrt.Header, _ uint32, rec *mrt.MessageRecord) error {
			if h.Subtype == mrt.BGP4MPMessageAS4Local {
				return nil
			}
			count++
			require.Equal(t, original, rec.BGPMessage.Bytes)
			return nil
		}})
		require.NoError(t, err)
		require.Equal(t, 1, count)
	}
}

// TestRFC6396WireMessagesNeverCoalesce drives the real coalescing receive rail.
// RFC requirement: RFC6396-4.4.2-2 positive -- two consecutive original UPDATEs with identical attributes become two byte-exact MRT records even though semantic delivery coalesces them into one UPDATE; record count and unequal message boundaries are preserved.
func TestRFC6396WireMessagesNeverCoalesce(t *testing.T) {
	s, bodies := newCoalesceSession(t)
	path, stop := recordSessionWire(t, s)
	first := buildUpdateMsg(buildUpdateBody(sampleAttrs(), []byte{24, 10, 1, 1}))
	second := buildUpdateMsg(buildUpdateBody(sampleAttrs(), []byte{24, 10, 1, 2, 24, 10, 1, 3}))
	server, client := net.Pipe()
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Errorf("close server: %v", err)
		}
	})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})
	go writeAllAndClose(client, first, second)
	reader := bufio.NewReaderSize(server, 65536)
	for range 3 {
		if err := s.readAndProcessCoalesced(server, reader); err != nil {
			break
		}
	}
	stop()
	require.Len(t, *bodies, 1)
	var messages [][]byte
	require.NoError(t, mrt.ReadFile(path, &mrt.Handler{OnMessage: func(_ mrt.Header, _ uint32, rec *mrt.MessageRecord) error {
		messages = append(messages, bytes.Clone(rec.BGPMessage.Bytes))
		return nil
	}}))
	require.Equal(t, [][]byte{first, second}, messages)
}

// TestMRTActualDirectionalOPENs checks both real transport directions, including
// the locally sent OPEN which precedes any negotiated context.
func TestMRTActualDirectionalOPENs(t *testing.T) {
	s := NewSession(NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301))
	defer s.timers.StopAll()
	startSession(t, s)
	path, stop := recordSessionWire(t, s)
	server, client := net.Pipe()
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Errorf("close server: %v", err)
		}
	})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})
	require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))
	require.NoError(t, server.SetDeadline(time.Now().Add(5*time.Second)))
	sent := make(chan []byte, 1)
	go func() {
		header := make([]byte, 19)
		if _, err := io.ReadFull(client, header); err != nil {
			sent <- nil
			return
		}
		length := int(header[16])<<8 | int(header[17])
		wire := make([]byte, length)
		copy(wire, header)
		if _, err := io.ReadFull(client, wire[19:]); err != nil {
			sent <- nil
			return
		}
		sent <- wire
	}()
	require.NoError(t, s.Accept(server))
	local := <-sent
	require.NotEmpty(t, local)
	peerOpen := &message.Open{Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x02020302}
	remote := message.PackTo(peerOpen, nil)
	exchanged := make(chan error, 1)
	finished := make(chan struct{})
	t.Cleanup(func() {
		// MUST close before joining if a session assertion ended the test.
		if err := client.Close(); err != nil {
			t.Errorf("close exchange client: %v", err)
		}
		<-finished
	})
	go func() {
		defer close(finished)
		n, err := client.Write(remote)
		if err != nil {
			exchanged <- err
			return
		}
		if n != len(remote) {
			exchanged <- io.ErrShortWrite
			return
		}
		var ka [19]byte
		_, err = io.ReadFull(client, ka[:])
		exchanged <- err
	}()
	require.NoError(t, s.ReadAndProcess())
	require.NoError(t, <-exchanged)
	stop()
	var opens [][]byte
	var subtypes []uint16
	require.NoError(t, mrt.ReadFile(path, &mrt.Handler{OnMessage: func(h mrt.Header, _ uint32, r *mrt.MessageRecord) error {
		if r.BGPMessage.Bytes[18] == 1 {
			opens = append(opens, r.BGPMessage.Bytes)
			subtypes = append(subtypes, h.Subtype)
		}
		return nil
	}}))
	require.Equal(t, [][]byte{local, remote}, opens)
	require.Equal(t, []uint16{mrt.BGP4MPMessageAS4Local, mrt.BGP4MPMessageAS4}, subtypes)
}

type partialWireWriter struct {
	limit   int
	failure error
}

func (w partialWireWriter) Write(b []byte) (int, error) { return min(w.limit, len(b)), w.failure }

// TestMRTOutboundAcceptedFrames covers short writes, a complete frame before an
// n+error result, and connection-local partial storage without staging callbacks.
func TestMRTOutboundAcceptedFrames(t *testing.T) {
	first := message.PackTo(message.NewKeepalive(), nil)
	second := buildUpdateMsg([]byte{0, 0, 0, 0})
	for _, cut := range []int{1, 17, 19, 20, 22, len(first) + len(second)} {
		s := NewSession(NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 1))
		var observed [][]byte
		s.onWireMessage = func(wire []byte, _ bgpctx.ContextID, sent bool, _ *sessionTransport) {
			require.True(t, sent)
			observed = append(observed, bytes.Clone(wire))
		}
		failure := errors.New("transport write failed")
		w := &observedBGPWriter{session: s, writer: partialWireWriter{limit: cut, failure: failure}}
		all := append(bytes.Clone(first), second...)
		n, err := w.Write(all)
		require.ErrorIs(t, err, failure)
		require.Equal(t, cut, n)
		if cut < 19 {
			require.Empty(t, observed)
		} else {
			require.Equal(t, first, observed[0])
		}
		w.writer = io.Discard
		_, err = w.Write(all[n:])
		require.NoError(t, err)
		require.Equal(t, [][]byte{first, second}, observed)
		other := &observedBGPWriter{session: s, writer: io.Discard}
		require.Empty(t, other.partial)
	}
}

// TestMRTWriterRetainsEpochContext exercises the teardown flush lock order and
// proves a retained writer cannot borrow its replacement's context or endpoints.
func TestMRTWriterRetainsEpochContext(t *testing.T) {
	s := NewSession(NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 1))
	var contexts []bgpctx.ContextID
	var endpoints []*sessionTransport
	s.onWireMessage = func(_ []byte, id bgpctx.ContextID, _ bool, transport *sessionTransport) {
		contexts = append(contexts, id)
		endpoints = append(endpoints, transport)
	}
	oldTransport := &sessionTransport{local: netip.MustParseAddr("192.0.2.2")}
	newTransport := &sessionTransport{local: netip.MustParseAddr("192.0.2.3")}
	old := &observedBGPWriter{writer: io.Discard, session: s, transport: oldTransport}
	s.wireWriter = old
	s.setSendCtxID(11)
	replacement := &observedBGPWriter{writer: io.Discard, session: s, transport: newTransport}
	s.wireWriter = replacement
	s.setSendCtxID(22)
	packet := message.PackTo(message.NewKeepalive(), nil)
	func() { s.mu.Lock(); defer s.mu.Unlock(); _, err := old.Write(packet); require.NoError(t, err) }()
	_, err := replacement.Write(packet)
	require.NoError(t, err)
	require.Equal(t, []bgpctx.ContextID{11, 22}, contexts)
	require.Equal(t, []*sessionTransport{oldTransport, newTransport}, endpoints)
}

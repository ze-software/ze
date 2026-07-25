package pppoe

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/callsink"
)

type fakeRelaySink struct {
	accepted bool
	err      error
	last     callsink.Request
	calls    int
}

func (f *fakeRelaySink) Relay(req callsink.Request) (bool, error) {
	f.calls++
	f.last = req
	return f.accepted, f.err
}

// TestRelayToL2TP_ConsultsSink -- AC-3 (R-1 boundary via callsink).
//
// VALIDATES: the PPPoE server consults the registered call-sink at PADS
// completion; with no sink it terminates locally; an accepting sink relays
// (and receives the service, interface, MAC, session ID, and channel fd); an
// erroring sink falls back to local termination.
func TestRelayToL2TP_ConsultsSink(t *testing.T) {
	callsink.Unregister()
	t.Cleanup(callsink.Unregister)

	s := &InterfaceServer{ifName: "eth0", logger: slog.Default()}
	pkt := &Packet{
		SrcMAC: [EthALen]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
		Tags:   []Tag{{Type: TagServiceName, Value: []byte("wholesale")}},
	}

	// No sink registered -> not relayed (terminate locally).
	require.False(t, s.relayToL2TP(pkt, 7, 99))

	// Sink accepts -> relayed; the request carries the subscriber context.
	fs := &fakeRelaySink{accepted: true}
	callsink.Register(fs)
	require.True(t, s.relayToL2TP(pkt, 7, 99))
	require.Equal(t, 1, fs.calls)
	require.Equal(t, "wholesale", fs.last.Service)
	require.Equal(t, "eth0", fs.last.Interface)
	require.EqualValues(t, 7, fs.last.SessionID)
	require.Equal(t, 99, fs.last.ChannelFD)
	require.Equal(t, "aa:bb:cc:dd:ee:ff", fs.last.SubscriberMAC)

	// Sink returns an error -> not relayed (fall back to local termination).
	callsink.Register(&fakeRelaySink{accepted: false, err: errors.New("dial failed")})
	require.False(t, s.relayToL2TP(pkt, 7, 99))
}

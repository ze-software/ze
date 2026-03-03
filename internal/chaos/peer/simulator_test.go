package peer

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
)

// fakeZePeer accepts a connection and performs Ze's side of the BGP handshake:
// read OPEN, send OPEN, read KEEPALIVE, send KEEPALIVE, then read all messages
// until the connection closes. Returns the count of UPDATE messages received.
func fakeZePeer(t *testing.T, ln net.Listener, done chan<- int) {
	t.Helper()

	conn, err := ln.Accept()
	if err != nil {
		done <- -1
		return
	}
	defer func() { _ = conn.Close() }()

	// Read client's OPEN.
	if _, readErr := readBGPMessage(conn); readErr != nil {
		t.Logf("fakeZe: reading OPEN: %v", readErr)
		done <- -1
		return
	}

	// Send our OPEN.
	open := BuildOpen(SessionConfig{
		ASN:      65000,
		RouterID: netip.MustParseAddr("10.0.0.1"),
		HoldTime: 90,
	})
	data := SerializeMessage(open)
	if _, writeErr := conn.Write(data); writeErr != nil {
		done <- -1
		return
	}

	// Read client's KEEPALIVE.
	if _, readErr := readBGPMessage(conn); readErr != nil {
		done <- -1
		return
	}

	// Send our KEEPALIVE.
	ka := message.NewKeepalive()
	kaData := SerializeMessage(ka)
	if _, writeErr := conn.Write(kaData); writeErr != nil {
		done <- -1
		return
	}

	// Read all subsequent messages (UPDATEs, EOR, KEEPALIVEs, NOTIFICATION).
	updateCount := 0
	for {
		msg, readErr := readBGPMessage(conn)
		if readErr != nil {
			break
		}
		if len(msg) >= 19 && msg[18] == 2 { // UPDATE
			updateCount++
		}
	}

	done <- updateCount
}

// readBGPMessage reads a single BGP message from conn.
func readBGPMessage(conn net.Conn) ([]byte, error) {
	header := make([]byte, message.HeaderLen)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}

	msgLen := int(binary.BigEndian.Uint16(header[16:18]))
	if msgLen < message.HeaderLen {
		return nil, io.ErrUnexpectedEOF
	}

	buf := make([]byte, msgLen)
	copy(buf, header)

	if msgLen > message.HeaderLen {
		if _, err := io.ReadFull(conn, buf[message.HeaderLen:]); err != nil {
			return nil, err
		}
	}

	return buf, nil
}

// TestSimulatorEstablishes verifies that the simulator connects, completes
// the BGP handshake, and emits an EventEstablished event.
//
// VALIDATES: Simulator performs OPEN/KEEPALIVE handshake and reports establishment.
// PREVENTS: Simulator hanging or failing to report session state.
func TestSimulatorEstablishes(t *testing.T) {
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	zeResult := make(chan int, 1)
	go fakeZePeer(t, ln, zeResult)

	events := make(chan Event, 100)
	cfg := SimulatorConfig{
		Profile: SimProfile{
			Index:      0,
			ASN:        65001,
			RouterID:   netip.MustParseAddr("10.255.0.1"),
			IsIBGP:     false,
			HoldTime:   90,
			RouteCount: 0, // No routes — just test establishment.
		},
		Seed:    42,
		Addr:    ln.Addr().String(),
		Events:  events,
		Verbose: false,
		Quiet:   true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		RunSimulator(ctx, cfg)
		close(events)
	}()

	// Collect events until established or timeout.
	established := false
	for ev := range events {
		if ev.Type == EventEstablished {
			established = true
			assert.Equal(t, 0, ev.PeerIndex)
			cancel() // Done — trigger shutdown.
			break
		}
		if ev.Type == EventError {
			t.Fatalf("simulator error: %v", ev.Err)
		}
	}

	require.True(t, established, "should have received EventEstablished")
}

// TestSimulatorSendsRoutes verifies that the simulator sends the expected
// number of UPDATE messages and emits EventRouteSent for each.
//
// VALIDATES: Simulator sends N routes and reports each via events.
// PREVENTS: Route sending silently failing or event channel not populated.
func TestSimulatorSendsRoutes(t *testing.T) {
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	zeResult := make(chan int, 1)
	go fakeZePeer(t, ln, zeResult)

	events := make(chan Event, 200)
	cfg := SimulatorConfig{
		Profile: SimProfile{
			Index:      1,
			ASN:        65002,
			RouterID:   netip.MustParseAddr("10.255.0.2"),
			IsIBGP:     false,
			HoldTime:   90,
			RouteCount: 5,
		},
		Seed:   42,
		Addr:   ln.Addr().String(),
		Events: events,
		Quiet:  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		RunSimulator(ctx, cfg)
		close(events)
	}()

	routesSent := 0
	eorSent := false
	for ev := range events {
		switch ev.Type {
		case EventRouteSent:
			routesSent++
			assert.Equal(t, 1, ev.PeerIndex)
			assert.True(t, ev.Prefix.IsValid(), "route event should have valid prefix")
		case EventEORSent:
			eorSent = true
			assert.Equal(t, 5, ev.Count)
			cancel()
		case EventError:
			t.Fatalf("simulator error: %v", ev.Err)
		case EventEstablished, EventRouteReceived, EventRouteWithdrawn, EventDisconnected,
			EventChaosExecuted, EventReconnecting, EventWithdrawalSent, EventRouteAction,
			EventDroppedEvents:
			// Expected but not checked here.
		}
	}

	assert.Equal(t, 5, routesSent, "should have sent 5 route events")
	assert.True(t, eorSent, "should have sent EOR event")

	// Verify Ze received the UPDATEs (EOR is also an UPDATE, so 5 routes + 1 EOR = 6).
	updateCount := <-zeResult
	assert.Equal(t, 6, updateCount, "Ze should have received 5 UPDATEs + 1 EOR")
}

// TestSimulatorShutdownClean verifies that canceling the context triggers
// a clean shutdown with NOTIFICATION cease and EventDisconnected.
//
// VALIDATES: Context cancellation sends NOTIFICATION and reports disconnect.
// PREVENTS: Simulator hanging or leaking goroutines on shutdown.
func TestSimulatorShutdownClean(t *testing.T) {
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	zeResult := make(chan int, 1)
	go fakeZePeer(t, ln, zeResult)

	events := make(chan Event, 100)
	cfg := SimulatorConfig{
		Profile: SimProfile{
			Index:      0,
			ASN:        65001,
			RouterID:   netip.MustParseAddr("10.255.0.1"),
			IsIBGP:     false,
			HoldTime:   90,
			RouteCount: 0,
		},
		Seed:   42,
		Addr:   ln.Addr().String(),
		Events: events,
		Quiet:  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		RunSimulator(ctx, cfg)
		close(events)
	}()

	// Collect all events — cancel after establishment.
	established := false
	disconnected := false
	for ev := range events {
		switch ev.Type {
		case EventEstablished:
			established = true
			cancel() // Trigger shutdown.
		case EventDisconnected:
			disconnected = true
		case EventRouteSent, EventRouteReceived, EventRouteWithdrawn, EventEORSent, EventError,
			EventChaosExecuted, EventReconnecting, EventWithdrawalSent, EventRouteAction,
			EventDroppedEvents:
			// Not checked in this test.
		}
	}

	assert.True(t, established, "should have received EventEstablished")
	assert.True(t, disconnected, "should have received EventDisconnected")
}

// TestSimulatorConnectionRefused verifies that the simulator reports an error
// when it cannot connect to Ze.
//
// VALIDATES: Connection failure produces EventError, not a hang.
// PREVENTS: Simulator blocking forever on unreachable address.
func TestSimulatorConnectionRefused(t *testing.T) {
	events := make(chan Event, 10)
	cfg := SimulatorConfig{
		Profile: SimProfile{
			Index:    0,
			ASN:      65001,
			RouterID: netip.MustParseAddr("10.255.0.1"),
			HoldTime: 90,
		},
		Seed:   42,
		Addr:   "127.0.0.1:1", // Port 1 — should be refused.
		Events: events,
		Quiet:  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		RunSimulator(ctx, cfg)
		close(events)
	}()

	gotError := false
	for ev := range events {
		if ev.Type == EventError {
			gotError = true
			assert.NotNil(t, ev.Err)
			break
		}
	}

	assert.True(t, gotError, "should have received EventError for connection refused")
}

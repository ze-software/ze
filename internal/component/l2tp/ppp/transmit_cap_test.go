// Design: docs/guide/l2tp.md -- admitted session MRU caps the installed transmit MTU.

package ppp

import (
	"bytes"
	"net"
	"testing"
	"time"
)

// TestOmittedMRUTransmitCap enters through SessionsIn and real LCP packets.
// The default peer receive MRU remains 1500 in the event, while the interface
// obeys the admitted cap. PPPoE and L2TP use the same installation path.
func TestOmittedMRUTransmitCap(t *testing.T) {
	for _, pppoe := range []bool{false, true} {
		name := "l2tp"
		if pppoe {
			name = "pppoe"
		}
		t.Run(name, func(t *testing.T) {
			d, peer, backend := startTransmitCapDriver(t, pppoe, 1492)
			negotiateTransmitCap(t, peer, 0, 1492)
			assertTransmitCapUp(t, d, backend, 1500, 1492, 1)
		})
	}
}

func TestTransmitCapPeerMRUBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name       string
		cap        uint16
		peer       uint16
		negotiated uint16
		mtu        int
	}{
		{"smaller peer", 1492, 1400, 1400, 1400},
		{"equal peer", 1492, 1492, 1492, 1492},
		{"larger peer converges", 1492, 1500, 1492, 1492},
		{"default cap remains 1500", 1500, 0, 1500, 1500},
		{"smaller admitted cap", 1280, 0, 1500, 1280},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, peer, backend := startTransmitCapDriver(t, false, tc.cap)
			negotiateTransmitCap(t, peer, tc.peer, tc.cap)
			assertTransmitCapUp(t, d, backend, tc.negotiated, tc.mtu, 1)
		})
	}
}

// Restart keeps the admitted cap even when both requests omit MRU. The event
// stream still publishes the peer's real default rather than a config copy.
func TestTransmitCapRestartIsolation(t *testing.T) {
	d, peer, backend := startTransmitCapDriver(t, false, 1492)
	negotiateTransmitCap(t, peer, 0, 1492)
	assertTransmitCapUp(t, d, backend, 1500, 1492, 1)

	writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest, 0x41, nil)
	select {
	case ev := <-d.EventsOut():
		down, ok := ev.(EventLCPDown)
		if !ok {
			t.Fatalf("restart event = %T, want EventLCPDown", ev)
		}
		down.Acknowledge()
	case <-time.After(5 * time.Second):
		t.Fatal("restart did not publish LCP down")
	}
	request := readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest)
	ack := readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureAck)
	if ack.Identifier != 0x41 || len(ack.Data) != 0 {
		t.Fatalf("restart Ack = %+v, want the peer's empty request", ack)
	}
	writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureAck, request.Identifier, request.Data)
	assertTransmitCapUp(t, d, backend, 1500, 1492, 2)
}

func startTransmitCapDriver(t *testing.T, pppoe bool, cap uint16) (*Driver, net.Conn, *fakeBackend) {
	t.Helper()
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 23001)
	backend := &fakeBackend{}
	ops, _, _ := newFakeOps()
	d := makeTestDriver(backend, ops)
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeConn(pair.peerEnd)
		d.Stop()
	})
	d.SessionsIn() <- StartSession{
		TunnelID:      23,
		SessionID:     1,
		ChanFD:        23001,
		UnitFD:        23002,
		UnitNum:       7,
		LNSMode:       !pppoe,
		PPPoE:         pppoe,
		MaxMRU:        cap,
		DisableIPCP:   true,
		DisableIPv6CP: true,
	}
	return d, pair.peerEnd, backend
}

func negotiateTransmitCap(t *testing.T, peer net.Conn, requestedMRU, cap uint16) {
	t.Helper()
	request := readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest)
	writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureAck, request.Identifier, request.Data)
	var opts []byte
	if requestedMRU != 0 {
		opts = optStream(mruOpt(requestedMRU))
	}
	id := uint8(0x31)
	writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest, id, opts)
	if requestedMRU > cap {
		nak := readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureNak)
		if nak.Identifier != id || !bytes.Equal(nak.Data, optStream(mruOpt(cap))) {
			t.Fatalf("MRU Nak = %+v, want peer request %d with MRU %d", nak, id, cap)
		}
		opts = nak.Data
		id++
		writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest, id, opts)
	}
	ack := readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureAck)
	if ack.Identifier != id || !bytes.Equal(ack.Data, opts) {
		t.Fatalf("MRU Ack = %+v, want request %d with options %x", ack, id, opts)
	}
}

func assertTransmitCapUp(t *testing.T, d *Driver, backend *fakeBackend, negotiated uint16, mtu, installs int) {
	t.Helper()
	events := drainTwoEvents(t, d.EventsOut(), 5*time.Second)
	up, ok := events[0].(EventLCPUp)
	if !ok || up.NegotiatedMRU != negotiated {
		t.Fatalf("LCP event = %+v, want negotiated peer MRU %d", events[0], negotiated)
	}
	if _, ok := events[1].(EventSessionUp); !ok {
		t.Fatalf("post-NCP event = %T, want EventSessionUp", events[1])
	}
	calls := backend.MTUCalls()
	if len(calls) != installs {
		t.Fatalf("MTU calls = %+v, want %d installs", calls, installs)
	}
	for _, call := range calls {
		if call.name != "ppp7" || call.mtu != mtu {
			t.Fatalf("installed MTU = %+v, want ppp7=%d", call, mtu)
		}
	}
	if calls := backend.UpCalls(); len(calls) != installs {
		t.Fatalf("admin-up calls = %v, want %d", calls, installs)
	}
}

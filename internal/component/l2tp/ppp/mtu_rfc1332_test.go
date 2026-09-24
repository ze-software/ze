// Design: docs/architecture/l2tp/bng-5-pppoe.md -- RFC 1332 conformance coverage
//
// Proves the boundary Ze owns for RFC 1332 Section 2.1: the Linux IP stack
// fragments an oversized datagram on the pppN device MTU, and Ze installs
// that MTU from the peer's Information-field MRU. The Protocol field and
// link framing are outside that MRU.

package ppp

import (
	"testing"
	"time"
)

// TestIPMTUInstalledFromNegotiatedMRU starts a proxied session whose peer
// negotiated a 1400-octet MRU and reads the MTU Ze installs on ppp7.
//
// RFC requirement: RFC1332-2.1-3 positive -- afterLCPOpen installs MTU 1400 on ppp7 for peer MRU 1400, preserving the complete Information field for the IP datagram.
// RFC requirement: RFC1332-2.1-3 negative -- afterLCPOpen never installs the default 1500 on ppp7 when the peer negotiated MRU 1400.
func TestIPMTUInstalledFromNegotiatedMRU(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 1001)
	defer closeConn(pair.peerEnd)

	backend := &fakeBackend{}
	ops, opsCalls, opsMu := newFakeOps()
	d := makeTestDriver(backend, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	const peerMRU = 1400
	stream := buildOptionStream([]LCPOption{mruOpt(peerMRU), magicOpt(0xCAFEBABE)})
	d.SessionsIn() <- StartSession{
		TunnelID:            1,
		SessionID:           42,
		ChanFD:              1001,
		UnitFD:              999,
		UnitNum:             7,
		LNSMode:             true,
		MaxMRU:              1500,
		DisableIPCP:         true,
		DisableIPv6CP:       true,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}
	got := drainTwoEvents(t, d.EventsOut(), time.Second)
	if _, ok := got[1].(EventSessionUp); !ok {
		t.Fatalf("event 1 = %T, want EventSessionUp", got[1])
	}

	const wantMTU = peerMRU
	mtuCalls := backend.MTUCalls()
	if len(mtuCalls) != 1 {
		t.Fatalf("MTU calls = %+v, want exactly one on ppp7", mtuCalls)
	}
	if mtuCalls[0].name != "ppp7" || mtuCalls[0].mtu != wantMTU {
		t.Fatalf("MTU installed = %+v, want ppp7=%d for a negotiated MRU of %d", mtuCalls[0], wantMTU, peerMRU)
	}
	for _, c := range mtuCalls {
		if c.mtu > wantMTU {
			t.Fatalf("MTU %d installed on %s exceeds the %d octets the peer can receive", c.mtu, c.name, wantMTU)
		}
	}
	opsMu.Lock()
	defer opsMu.Unlock()
	if len(*opsCalls) != 1 || (*opsCalls)[0].mru != MaxFrameLen {
		t.Fatalf("kernel receive MRU calls = %+v, want one MRU of 1500", *opsCalls)
	}
}

// Design: docs/guide/l2tp.md -- a malformed proxied Magic-Number renegotiates LCP.

package ppp

import (
	"errors"
	"testing"
	"time"
)

// TestProxyLCPMalformedMagicRefused holds the proxied Magic-Number to the same
// strict decode as the MRU, in both directions. Read as absent, a wrong-width
// peer value leaves peerMagic at zero and Ze drops the peer's real Echo packets.
func TestProxyLCPMalformedMagicRefused(t *testing.T) {
	good := optStream(mruOpt(1500), magicOpt(0x11223344))
	for _, tc := range []struct {
		name string
		opts []LCPOption
	}{
		{"empty", []LCPOption{{Type: LCPOptMagic}}},
		{"two octets", []LCPOption{{Type: LCPOptMagic, Data: []byte{1, 2}}}},
		{"three octets", []LCPOption{{Type: LCPOptMagic, Data: []byte{1, 2, 3}}}},
		{"five octets", []LCPOption{{Type: LCPOptMagic, Data: []byte{1, 2, 3, 4, 5}}}},
		{"zero", []LCPOption{magicOpt(0)}},
		{"listed twice", []LCPOption{magicOpt(0x01020304), magicOpt(0x05060708)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := optStream(tc.opts...)
			if _, err := EvaluateProxyLCP(good, good, bad); !errors.Is(err, errProxyLCPInvalid) {
				t.Fatalf("Last-Received error = %v, want invalid", err)
			}
			if _, err := EvaluateProxyLCP(good, bad, good); !errors.Is(err, errProxyLCPInvalid) {
				t.Fatalf("Last-Sent error = %v, want invalid", err)
			}
		})
	}
}

// TestProxyLCPWellFormedMagicAdmitted is the negative of the refusal: a blanket
// refusal would pass it, so the exact values and an absent option are held too.
func TestProxyLCPWellFormedMagicAdmitted(t *testing.T) {
	sent := optStream(mruOpt(1500), magicOpt(0x01020304))
	recv := optStream(mruOpt(1500), magicOpt(0x11223344))
	result, err := EvaluateProxyLCP(recv, sent, recv)
	if err != nil {
		t.Fatal(err)
	}
	if result.PeerMagic != 0x11223344 || result.LocalMagic != 0x01020304 {
		t.Fatalf("decoded peer %#x, local %#x", result.PeerMagic, result.LocalMagic)
	}
	absent := optStream(mruOpt(1500))
	result, err = EvaluateProxyLCP(absent, absent, absent)
	if err != nil {
		t.Fatal(err)
	}
	if result.PeerMagic != 0 || result.LocalMagic != 0 {
		t.Fatalf("absent options decoded peer %#x, local %#x", result.PeerMagic, result.LocalMagic)
	}
}

// TestProxyLCPMalformedMagicRenegotiates reaches the refusal through the real
// Driver: normal LCP starts on the wire, and no proxied lifetime is published.
func TestProxyLCPMalformedMagicRenegotiates(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 29401)
	defer closeConn(pair.peerEnd)
	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	defer d.Stop()
	sent := optStream(mruOpt(1500), magicOpt(0x01020304))
	recv := optStream(mruOpt(1500), LCPOption{Type: LCPOptMagic, Data: []byte{1, 2}})
	d.SessionsIn() <- StartSession{
		TunnelID: 29, SessionID: 5, ChanFD: 29401, UnitFD: 29402,
		MaxMRU: 1500, DisableIPCP: true, DisableIPv6CP: true,
		AuthTimeout: 5 * time.Second, IPTimeout: 5 * time.Second,
		ProxyLCPInitialRecv: recv, ProxyLCPLastRecv: recv, ProxyLCPLastSent: sent,
	}
	if request := readPolicyLCP(t, pair.peerEnd); request.Code != LCPConfigureRequest {
		t.Fatalf("code = %d, want Configure-Request", request.Code)
	}
	select {
	case ev := <-d.EventsOut():
		t.Fatalf("malformed proxy published %+v before negotiation", ev)
	default:
	}
}

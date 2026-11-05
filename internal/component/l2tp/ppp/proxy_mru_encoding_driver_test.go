// Design: docs/guide/l2tp.md -- invalid proxied MRUs require fresh LCP negotiation.
// Related: proxy_mru_floor_driver_test.go -- observed fallback and usable floor helpers.

package ppp

import (
	"bytes"
	"testing"
)

// TestProxyMRUOptionEncodingDriver passes actual MRU options through SessionsIn.
// A zero value and a malformed option must not acquire the absent option's
// default. Valid absence and converged floor/jumbo options remain usable.
// MUTATION: forget the MRU option's presence or accept an MRU Length other
// than four; the invalid cases then publish Opened before fresh negotiation.
func TestProxyMRUOptionEncodingDriver(t *testing.T) {
	zero := mruOpt(0)
	floor := mruOpt(68)
	ordinary := mruOpt(1492)
	jumbo := mruOpt(9000)
	empty := LCPOption{Type: LCPOptMRU}
	short := LCPOption{Type: LCPOptMRU, Data: []byte{0x05}}
	long := LCPOption{Type: LCPOptMRU, Data: []byte{0x05, 0xdc, 0xaa}}
	for _, tc := range []struct {
		name          string
		cap           uint16
		local         *LCPOption
		peer          *LCPOption
		initial       uint16
		renegotiate   bool
		negotiatedMRU uint16
		mtu           int
	}{
		{"peer explicit zero", 1492, &ordinary, &zero, 1500, true, 68, 68},
		{"peer empty MRU", 1492, &ordinary, &empty, 1500, true, 68, 68},
		{"peer short MRU", 1492, &ordinary, &short, 1500, true, 68, 68},
		{"peer long MRU", 1492, &ordinary, &long, 1500, true, 68, 68},
		{"local explicit zero", 1500, &zero, &ordinary, 1500, true, 68, 68},
		{"local empty MRU", 1500, &empty, &ordinary, 1500, true, 68, 68},
		{"local short MRU", 1500, &short, &ordinary, 1500, true, 68, 68},
		{"local long MRU", 1500, &long, &ordinary, 1500, true, 68, 68},
		{"peer omitted MRU", 1492, &ordinary, nil, 1500, false, 1500, 1492},
		{"local omitted MRU", 1500, nil, &ordinary, 1500, false, 1492, 1492},
		{"converged floor after initial zero", 1492, &ordinary, &floor, 0, false, 68, 68},
		{"jumbo peer remains admitted", 1492, &ordinary, &jumbo, 1500, false, 9000, 1492},
	} {
		t.Run(tc.name, func(t *testing.T) {
			local := []LCPOption{magicOpt(0x01020304), authProtoOpt(authProtoPAP)}
			if tc.local != nil {
				local = append(local, *tc.local)
			}
			peer := []LCPOption{magicOpt(proxyTransmitPeerMagic)}
			if tc.peer != nil {
				peer = append(peer, *tc.peer)
			}
			td := startProxyMRUOptionEncodingDriver(t, tc.cap,
				optStream(mruOpt(tc.initial), magicOpt(proxyTransmitPeerMagic)),
				optStream(local...), optStream(peer...))
			if tc.renegotiate {
				request := awaitProxyMRUFloorRenegotiation(t, td)
				opts, err := ParseLCPOptions(request.Data)
				if err != nil {
					t.Fatal(err)
				}
				if mru, present := lookupMRUOption(opts); !present || mru != tc.cap {
					t.Fatalf("fresh local MRU=%d present=%v, want %d", mru, present, tc.cap)
				}
				auth, present := lookupOption(opts, LCPOptAuthProto)
				if !present || !bytes.Equal(auth, []byte{0xc0, 0x23}) {
					t.Fatalf("fresh authentication=%x, want permitted PAP", auth)
				}
				if calls := td.backend.MTUCalls(); len(calls) != 0 {
					t.Fatalf("invalid proxy installed an interface: %+v", calls)
				}
				corrected := optStream(mruOpt(68), magicOpt(proxyTransmitPeerMagic))
				writeLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureRequest, 0x61, corrected)
				ack := readLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureAck)
				if ack.Identifier != 0x61 || !bytes.Equal(ack.Data, corrected) {
					t.Fatalf("corrected peer Ack=%+v, want its exact request", ack)
				}
				writeLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureAck, request.Identifier, request.Data)
			}
			assertProxyTransmitNegotiation(t, td.driver, tc.negotiatedMRU)
			completeProxyTransmitNetwork(t, td)
			assertProxyTransmitInterface(t, td, tc.mtu, 1)
			assertProxyTransmitEcho(t, td, proxyTransmitPeerMagic)
		})
	}
}

// TestProxyDuplicateMRUOptionDriver puts a duplicate in each converged stream
// and requires fresh negotiation before useful service. Identical valid
// values isolate duplicate rejection from the zero, floor and width guards.
// RFC 1661 Section 6: "(None of the Configuration
// Options in this specification can be listed more than once.)"
// MUTATION: use only the first MRU option; every case then shortcuts to Opened.
func TestProxyDuplicateMRUOptionDriver(t *testing.T) {
	for _, tc := range []struct {
		name      string
		peer      bool
		duplicate LCPOption
	}{
		{"peer duplicate identical", true, mruOpt(1492)},
		{"peer duplicate zero", true, mruOpt(0)},
		{"peer duplicate below floor", true, mruOpt(67)},
		{"peer duplicate malformed", true, LCPOption{Type: LCPOptMRU, Data: []byte{0x05}}},
		{"local duplicate identical", false, mruOpt(1492)},
		{"local duplicate zero", false, mruOpt(0)},
		{"local duplicate below floor", false, mruOpt(67)},
		{"local duplicate malformed", false, LCPOption{Type: LCPOptMRU, Data: []byte{0x05}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			local := []LCPOption{mruOpt(1492), magicOpt(0x01020304), authProtoOpt(authProtoPAP)}
			peer := []LCPOption{mruOpt(1492), magicOpt(proxyTransmitPeerMagic)}
			if tc.peer {
				peer = append(peer, tc.duplicate)
			} else {
				local = append(local, tc.duplicate)
			}
			td := startProxyMRUOptionEncodingDriver(t, 1492,
				optStream(mruOpt(1500), magicOpt(proxyTransmitPeerMagic)),
				optStream(local...), optStream(peer...))
			request := awaitProxyMRUFloorRenegotiation(t, td)
			opts, err := ParseLCPOptions(request.Data)
			if err != nil {
				t.Fatal(err)
			}
			if mru, present := lookupMRUOption(opts); !present || mru != 1492 {
				t.Fatalf("fresh local MRU=%d present=%v, want1492", mru, present)
			}
			auth, present := lookupOption(opts, LCPOptAuthProto)
			if !present || !bytes.Equal(auth, []byte{0xc0, 0x23}) {
				t.Fatalf("fresh authentication=%x, want permitted PAP", auth)
			}
			if calls := td.backend.MTUCalls(); len(calls) != 0 {
				t.Fatalf("duplicate MRU installed an interface before negotiation: %+v", calls)
			}
			corrected := optStream(mruOpt(68), magicOpt(proxyTransmitPeerMagic))
			writeLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureRequest, 0x61, corrected)
			ack := readLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureAck)
			if ack.Identifier != 0x61 || !bytes.Equal(ack.Data, corrected) {
				t.Fatalf("corrected peer Ack=%+v, want its exact request", ack)
			}
			writeLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureAck, request.Identifier, request.Data)
			assertProxyTransmitNegotiation(t, td.driver, 68)
			completeProxyTransmitNetwork(t, td)
			assertProxyTransmitInterface(t, td, 68, 1)
			assertProxyTransmitEcho(t, td, proxyTransmitPeerMagic)
		})
	}
}

func startProxyMRUOptionEncodingDriver(t *testing.T, cap uint16, initial, local, peer []byte) *ncpTestDriver {
	t.Helper()
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 24001)
	backend := &fakeBackend{}
	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(backend, ops)
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeConn(pair.peerEnd)
		d.Stop()
	})
	d.SessionsIn() <- StartSession{
		TunnelID:            24,
		SessionID:           1,
		ChanFD:              24001,
		UnitFD:              24002,
		UnitNum:             9,
		LNSMode:             true,
		MaxMRU:              cap,
		AuthMethod:          AuthMethodPAP,
		AuthFallbackOrder:   []AuthMethod{AuthMethodPAP},
		AuthRequired:        true,
		DisableIPv6CP:       true,
		ProxyLCPInitialRecv: initial,
		ProxyLCPLastSent:    local,
		ProxyLCPLastRecv:    peer,
	}
	return &ncpTestDriver{t: t, driver: d, backend: backend, peer: pair.peerEnd, peerFD: 24001}
}

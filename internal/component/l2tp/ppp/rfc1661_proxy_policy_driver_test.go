package ppp

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// TestProxyLCPPolicy observes real Driver output for both admitted proxy
// lifetimes and renegotiations. Local and peer receive MRUs are independent.
func TestProxyLCPPolicy(t *testing.T) {
	for _, tc := range []struct {
		name     string
		maxMRU   uint16
		localMRU uint16
		peerMRU  uint16
		auth     []byte
		order    []AuthMethod
		required bool
		proxy    bool
		method   AuthMethod
	}{
		{"PAP disallowed", 1500, 1500, 1400, []byte{0xc0, 0x23}, []AuthMethod{AuthMethodCHAPMD5}, true, false, AuthMethodCHAPMD5},
		{"PAP explicitly allowed", 1500, 1500, 1400, []byte{0xc0, 0x23}, []AuthMethod{AuthMethodCHAPMD5, AuthMethodPAP}, true, true, AuthMethodPAP},
		{"CHAP allowed", 1492, 1492, 1400, []byte{0xc2, 0x23, 0x05}, []AuthMethod{AuthMethodCHAPMD5}, true, true, AuthMethodCHAPMD5},
		{"MSCHAP allowed", 1500, 1500, 1400, []byte{0xc2, 0x23, 0x81}, []AuthMethod{AuthMethodMSCHAPv2}, true, true, AuthMethodMSCHAPv2},
		{"absent LastSent MRU means1500", 1492, 0, 1400, []byte{0xc2, 0x23, 0x05}, []AuthMethod{AuthMethodCHAPMD5}, true, false, AuthMethodCHAPMD5},
		{"explicit LastSent above cap", 1492, 1500, 1400, []byte{0xc2, 0x23, 0x05}, []AuthMethod{AuthMethodCHAPMD5}, true, false, AuthMethodCHAPMD5},
		{"peer MRU is not local cap", 1492, 1492, 1500, []byte{0xc2, 0x23, 0x05}, []AuthMethod{AuthMethodCHAPMD5}, true, true, AuthMethodCHAPMD5},
		{"unknown protocol optional", 1500, 1500, 1400, []byte{0xc2, 0x27}, []AuthMethod{AuthMethodCHAPMD5}, false, false, AuthMethodCHAPMD5},
		{"unknown CHAP algorithm optional", 1500, 1500, 1400, []byte{0xc2, 0x23, 0x42}, []AuthMethod{AuthMethodCHAPMD5}, false, false, AuthMethodCHAPMD5},
		{"malformed auth optional", 1500, 1500, 1400, []byte{0xc0}, []AuthMethod{AuthMethodCHAPMD5}, false, false, AuthMethodCHAPMD5},
		{"missing required auth", 1500, 1500, 1400, nil, []AuthMethod{AuthMethodCHAPMD5}, true, false, AuthMethodCHAPMD5},
		{"explicit optional noauth", 1492, 1492, 1400, nil, nil, false, true, AuthMethodNone},
		{"empty list never admits proxy PAP", 1500, 1500, 1400, []byte{0xc0, 0x23}, nil, true, false, AuthMethodCHAPMD5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := newPipeRegistry()
			installPipeRegistry(t, reg)
			pair := newPipePair(reg, 19001)
			defer closeConn(pair.peerEnd)
			ops, _, _ := newFakeOps()
			d := newTestDriverNoResponder(&fakeBackend{}, ops)
			if err := d.Start(); err != nil {
				t.Fatal(err)
			}
			defer d.Stop()
			sent := []LCPOption{magicOpt(0x01020304)}
			if tc.localMRU != 0 {
				sent = append(sent, mruOpt(tc.localMRU))
			}
			if tc.auth != nil {
				sent = append(sent, LCPOption{Type: LCPOptAuthProto, Data: tc.auth})
			}
			peer := optStream(mruOpt(tc.peerMRU), magicOpt(0x11223344))
			d.SessionsIn() <- StartSession{TunnelID: 19, SessionID: 1, ChanFD: 19001, UnitFD: 19002, MaxMRU: tc.maxMRU, AuthMethod: AuthMethodCHAPMD5, AuthFallbackOrder: tc.order, AuthRequired: tc.required, DisableIPCP: true, DisableIPv6CP: true, ProxyLCPInitialRecv: peer, ProxyLCPLastRecv: peer, ProxyLCPLastSent: optStream(sent...)}
			if tc.proxy {
				select {
				case ev := <-d.EventsOut():
					up, ok := ev.(EventLCPUp)
					if !ok {
						t.Fatalf("event %T", ev)
					}
					if up.NegotiatedMRU != tc.peerMRU || up.NegotiatedAuthMethod != tc.method {
						t.Fatalf("proxy actual negotiation %+v", up)
					}
				case <-time.After(time.Second):
					t.Fatal("no proxy LCPUp")
				}
				return
			}
			proto, payload := readPeerFrame(t, pair.peerEnd)
			if proto != ProtoLCP {
				t.Fatalf("protocol %x want LCP", proto)
			}
			pkt, err := ParseLCPPacket(payload)
			if err != nil {
				t.Fatal(err)
			}
			if pkt.Code != LCPConfigureRequest {
				t.Fatalf("code %d want ConfigureRequest", pkt.Code)
			}
			opts, err := ParseLCPOptions(pkt.Data)
			if err != nil {
				t.Fatal(err)
			}
			mru, _ := lookupMRUOption(opts)
			auth, ok := lookupOption(opts, LCPOptAuthProto)
			if mru != tc.maxMRU || !ok || len(auth) < 2 {
				t.Fatalf("renegotiation policy mru=%d auth=%x", mru, auth)
			}
			if method := authMethodFromAuthProto(binary.BigEndian.Uint16(auth), auth[2:]); method != AuthMethodCHAPMD5 {
				t.Fatalf("normal auth offer %v", method)
			}
			select {
			case ev := <-d.EventsOut():
				t.Fatalf("rejected proxy published event %+v", ev)
			default:
			}
		})
	}
}

// writePolicyPeerPacket writes one complete PPP/LCP frame with a bounded wait.
func writePolicyPeerPacket(t *testing.T, peer net.Conn, code, id uint8, data []byte) {
	t.Helper()
	var frame [MaxFrameBufLen]byte
	n := WriteFrame(frame[:], 0, ProtoLCP, nil)
	n += WriteLCPPacket(frame[:], n, code, id, data)
	if err := peer.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := peer.Write(frame[:n]); err != nil {
		t.Fatal(err)
	}
}

// TestReceiveFullFrameBelowConfiguredMRU sends a 1500-octet Information field
// over the real Driver reader at requested MRU1492, and observes its reply.
// RFC requirement: RFC1661-6.1-1 positive -- run receives and answers the complete 1500-octet LCP Information field even though its local requested MRU is1492; the kernel receive MRU remains1500.
// RFC requirement: RFC1661-6.1-1 negative -- a smaller requested MRU does not lower the kernel receive MRU below1500.
func TestReceiveFullFrameBelowConfiguredMRU(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 19101)
	defer closeConn(pair.peerEnd)
	ops, calls, mu := newFakeOps()
	d := makeTestDriver(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	defer d.Stop()
	peer := optStream(mruOpt(1500), magicOpt(0x11223344))
	sent := optStream(mruOpt(1492), magicOpt(0x01020304))
	d.SessionsIn() <- StartSession{TunnelID: 19, SessionID: 2, ChanFD: 19101, UnitFD: 19102, MaxMRU: 1492, DisableIPCP: true, DisableIPv6CP: true, ProxyLCPInitialRecv: peer, ProxyLCPLastRecv: peer, ProxyLCPLastSent: sent}
	evs := drainTwoEvents(t, d.EventsOut(), time.Second)
	if _, ok := evs[1].(EventSessionUp); !ok {
		t.Fatalf("session event %T", evs[1])
	}
	data := bytes.Repeat([]byte{0x5a}, 1500-lcpHeaderLen)
	binary.BigEndian.PutUint32(data, 0x11223344)
	writePolicyPeerPacket(t, pair.peerEnd, LCPEchoRequest, 77, data)
	if err := pair.peerEnd.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var frame [MaxFrameBufLen]byte
	n, err := pair.peerEnd.Read(frame[:])
	if err != nil {
		t.Fatal(err)
	}
	proto, payload, _, err := ParseFrame(frame[:n])
	if err != nil {
		t.Fatal(err)
	}
	reply, err := ParseLCPPacket(payload)
	if err != nil {
		t.Fatal(err)
	}
	if proto != ProtoLCP || reply.Code != LCPEchoReply || reply.Identifier != 77 || len(payload) != 1500 {
		t.Fatalf("full receive reply proto=%x packet=%+v length=%d", proto, reply, len(payload))
	}
	if !bytes.Equal(reply.Data[4:], data[4:]) {
		t.Fatal("full Information field truncated or changed")
	}
	oversized := make([]byte, MaxFrameBufLen+1)
	if _, _, _, err := ParseFrame(oversized); err == nil {
		t.Fatal("accepted 1501-octet Information field")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(*calls) != 1 || (*calls)[0].mru != 1500 {
		t.Fatalf("kernel receive MRU %+v", *calls)
	}
}

// TestAuthAllowPAPFallbackAndOptionalNoAuth reaches successful authentication
// through normal LCP after a PAP Nak, once opted in and once with optional auth.
func TestAuthAllowPAPFallbackAndOptionalNoAuth(t *testing.T) {
	for _, optional := range []bool{false, true} {
		name := "PAP fallback"
		if optional {
			name = "optional noauth"
		}
		t.Run(name, func(t *testing.T) {
			reg := newPipeRegistry()
			installPipeRegistry(t, reg)
			pair := newPipePair(reg, 19201)
			defer closeConn(pair.peerEnd)
			ops, _, _ := newFakeOps()
			d := newTestDriverNoResponder(&fakeBackend{}, ops)
			if err := d.Start(); err != nil {
				t.Fatal(err)
			}
			defer d.Stop()
			order := []AuthMethod{AuthMethodCHAPMD5, AuthMethodPAP}
			wantMethod := AuthMethodPAP
			if optional {
				order = []AuthMethod{AuthMethodCHAPMD5}
				wantMethod = AuthMethodNone
			}
			d.SessionsIn() <- StartSession{
				TunnelID: 19, SessionID: 3, ChanFD: 19201, UnitFD: 19202,
				AuthMethod: AuthMethodCHAPMD5, AuthFallbackOrder: order,
				AuthRequired: !optional, DisableIPCP: true, DisableIPv6CP: true,
			}
			first := readPolicyLCP(t, pair.peerEnd)
			writePolicyPeerPacket(t, pair.peerEnd, LCPConfigureNak, first.Identifier,
				optStream(authProtoOpt(authProtoPAP)))
			second := readPolicyLCP(t, pair.peerEnd)
			if second.Code != LCPConfigureRequest {
				t.Fatalf("second code %d, want ConfigureRequest", second.Code)
			}
			writePolicyPeerPacket(t, pair.peerEnd, LCPConfigureRequest, 31,
				optStream(mruOpt(1492), magicOpt(0x11223344)))
			if ack := readPolicyLCP(t, pair.peerEnd); ack.Code != LCPConfigureAck {
				t.Fatalf("peer request response %+v", ack)
			}
			writePolicyPeerPacket(t, pair.peerEnd, LCPConfigureAck, second.Identifier, second.Data)
			select {
			case ev := <-d.EventsOut():
				up, ok := ev.(EventLCPUp)
				if !ok {
					t.Fatalf("event %T, want LCPUp", ev)
				}
				if up.NegotiatedAuthMethod != wantMethod || up.NegotiatedMRU != 1492 {
					t.Fatalf("negotiated event %+v", up)
				}
			case <-time.After(time.Second):
				t.Fatal("no LCPUp")
			}
			if !optional {
				if _, err := pair.peerEnd.Write(buildPAPRequestFrame(42, "alice", "secret")); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case ev := <-d.AuthEventsOut():
				req, ok := ev.(EventAuthRequest)
				if !ok {
					t.Fatalf("auth event %T", ev)
				}
				if req.Method != wantMethod {
					t.Fatalf("authentication method %v, want %v", req.Method, wantMethod)
				}
				if !optional && (req.Username != "alice" || string(req.Response) != "secret") {
					t.Fatalf("PAP credentials %+v", req)
				}
			case <-time.After(time.Second):
				t.Fatal("no authentication request")
			}
			if err := d.AuthResponse(19, 3, true, "", nil); err != nil {
				t.Fatal(err)
			}
			if !optional {
				proto, payload := readPeerFrame(t, pair.peerEnd)
				if proto != ProtoPAP || payload[0] != 2 {
					t.Fatalf("PAP success reply proto=%x data=%x", proto, payload)
				}
			}
			select {
			case ev := <-d.EventsOut():
				if _, ok := ev.(EventSessionUp); !ok {
					t.Fatalf("event %T, want SessionUp", ev)
				}
			case <-time.After(time.Second):
				t.Fatal("accepted authentication did not bring session up")
			}
		})
	}
}

// readPolicyLCP decodes one peer-visible LCP packet.
func readPolicyLCP(t *testing.T, peer net.Conn) LCPPacket {
	t.Helper()
	proto, payload := readPeerFrame(t, peer)
	if proto != ProtoLCP {
		t.Fatalf("protocol %x, want LCP", proto)
	}
	pkt, err := ParseLCPPacket(payload)
	if err != nil {
		t.Fatal(err)
	}
	return pkt
}

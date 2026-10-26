// Design: docs/architecture/ike/ipsec-14-responder.md -- the Configuration payload in IKE_AUTH
// Related: cp.go -- the consumer these tests drive through handleAuthRequest
// VALIDATES: a pool-bound respond peer's CFG_REQUEST is answered with a unique lease, a
// CFG_REPLY before SAr2 and selectors naming the lease; refusals keep the IKE SA.
// PREVENTS: two peers holding one inner address (RFC 3948 Section 5.1), a CFG_REPLY to a
// peer that asked none, and a lease that outlives or dies before its IKE SA.

package engine

import (
	"bytes"
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/eap"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// cpPool is the pool every test here binds: a /29 (six hosts), IPv4 and IPv6, with one
// DNS server of each family.
func cpPool() ipsec.VirtualIPPool {
	return ipsec.VirtualIPPool{
		Name: "rw", Range: "10.10.0.0/29", Range6: "fd00:10::/120",
		DNS: []string{"192.0.2.53", "2001:db8::53"},
	}
}

// cpVIP returns engine virtual-IP state serving pool.
func cpVIP(t *testing.T, pool ipsec.VirtualIPPool) *virtualIPState {
	t.Helper()
	vip := newVirtualIPState()
	vip.reload(&ipsec.RemoteAccessConfig{Pools: map[string]ipsec.VirtualIPPool{pool.Name: pool}}, slogutil.DiscardLogger())
	if _, ok := vip.pool(pool.Name); !ok {
		t.Fatalf("pool %q was not built", pool.Name)
	}
	return vip
}

// cpResponder builds an initiator and a responder that have finished IKE_SA_INIT, the
// responder peer named name and bound to the pool when bind is set, sharing vip.
func cpResponder(t *testing.T, vip *virtualIPState, name string, bind bool) (ini, resp *SA, ps *PeerSession) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "cp-psk")
	if bind {
		respPeer.VirtualIP = &ipsec.VirtualIPBinding{Pool: "rw"}
	}

	table := NewSATable()
	ini, err := newInitiatorSA(name, iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent

	resp, err = newResponderSA(name, respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)
	ps = &PeerSession{peerName: name, peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup, virtualIP: vip}
	return ini, resp, ps
}

// cpAuth sends the initiator's IKE_AUTH request with cps appended and returns the
// decrypted response.
func cpAuth(t *testing.T, ini, resp *SA, ps *PeerSession, cps ...*wire.PayloadCP) []wire.PayloadEntry {
	t.Helper()
	hdr := parseMsg(t, ini.LastSentMsg).Header
	inner := mbDecrypt(t, resp, ini.LastSentMsg)
	for _, cp := range cps {
		inner = append(inner, wire.PayloadEntry{Payload: cp})
	}
	request, err := buildEncryptedMessageEx(ini, inner, hdr.MessageID, hdr.ExchangeType, hdr.Flags)
	if err != nil {
		t.Fatalf("buildEncryptedMessageEx: %v", err)
	}
	ps.handleAuthRequest(resp, parseMsg(t, request), request, nil, nil, slogutil.DiscardLogger())
	if resp.State != StateEstablished {
		t.Fatalf("responder state = %v, want established", resp.State)
	}
	return mbDecrypt(t, ini, resp.LastSentMsg)
}

// cfgRequest builds a CFG_REQUEST of attrs.
func cfgRequest(attrs ...wire.ConfigAttr) *wire.PayloadCP {
	return &wire.PayloadCP{CFGType: wire.CFGTypeRequest, Attrs: attrs}
}

// ip4Ask asks for an IPv4 address: any when requested is the zero Addr.
func ip4Ask(requested netip.Addr) wire.ConfigAttr {
	if !requested.IsValid() {
		return wire.ConfigAttr{Type: wire.CPAttrInternalIP4Address}
	}
	a := requested.As4()
	return wire.ConfigAttr{Type: wire.CPAttrInternalIP4Address, Value: a[:]}
}

// replyOf returns the response's CP payload and its index, and the SA payload's index.
func replyOf(entries []wire.PayloadEntry) (cp *wire.PayloadCP, cpAt, saAt int) {
	cpAt, saAt = -1, -1
	for i, e := range entries {
		switch p := e.Payload.(type) {
		case *wire.PayloadCP:
			cp, cpAt = p, i
		case *wire.PayloadSA:
			saAt = i
		}
	}
	return cp, cpAt, saAt
}

// attrsOf returns the values of every attribute of type typ.
func attrsOf(cp *wire.PayloadCP, typ uint16) [][]byte {
	var out [][]byte
	for _, a := range cp.Attrs {
		if a.Type == typ {
			out = append(out, a.Value)
		}
	}
	return out
}

// notifyIn reports whether entries carry a notify of type typ.
func notifyIn(entries []wire.PayloadEntry, typ uint16) bool {
	for _, e := range entries {
		if n, ok := e.Payload.(*wire.PayloadNotify); ok && n.NotifyMsgType == typ {
			return true
		}
	}
	return false
}

// tsiOf returns the first selector of the response's TSi.
func tsiOf(t *testing.T, entries []wire.PayloadEntry) wire.TrafficSelector {
	t.Helper()
	for _, e := range entries {
		if ts, ok := e.Payload.(*wire.PayloadTS); ok && ts.TSPayloadType == wire.PayloadTypeTSi {
			if len(ts.TrafficSelectors) == 0 {
				t.Fatal("TSi carries no selector")
			}
			return ts.TrafficSelectors[0]
		}
	}
	t.Fatal("the response carries no TSi")
	return wire.TrafficSelector{}
}

// TestResponderCFGRequestLeasesFromBoundPool drives a bound peer's IKE_AUTH carrying an
// empty INTERNAL_IP4_ADDRESS through handleAuthRequest.
//
// Goal: AC-1 and the wiring row: the registry is reached from the wire. Method: the reply
// carries an INTERNAL_IP4_ADDRESS inside the pool, the registry holds it, and AC-4: the CP
// payload precedes the SA payload.
//
// RFC requirement: RFC7296-2.19-7 positive -- a pool-bound peer's IKE_AUTH request carrying a CFG_REQUEST for an IPv4 address is answered with a CFG_REPLY whose position in the decrypted response comes before the SA payload's.
// RFC requirement: RFC7296-4-6 positive -- the CFG_REQUEST in the peer's first IKE_AUTH message, asking INTERNAL_IP4_ADDRESS with an empty value, is parsed and its address attribute recognized: the response carries a CFG_REPLY with one four-octet INTERNAL_IP4_ADDRESS inside the pool, and the lease registry holds that address.
func TestResponderCFGRequestLeasesFromBoundPool(t *testing.T) {
	vip := cpVIP(t, cpPool())
	ini, resp, ps := cpResponder(t, vip, "ze", true)
	answer := cpAuth(t, ini, resp, ps, cfgRequest(ip4Ask(netip.Addr{})))

	cp, cpAt, saAt := replyOf(answer)
	if cp == nil {
		t.Fatal("no CFG_REPLY in the IKE_AUTH response of a bound peer that asked for an address")
	}
	if cp.CFGType != wire.CFGTypeReply {
		t.Fatalf("CFG type = %d, want CFG_REPLY", cp.CFGType)
	}
	addrs := attrsOf(cp, wire.CPAttrInternalIP4Address)
	if len(addrs) != 1 || len(addrs[0]) != 4 {
		t.Fatalf("INTERNAL_IP4_ADDRESS values = %v, want one of 4 octets", addrs)
	}
	got := netip.AddrFrom4([4]byte(addrs[0]))
	if !netip.MustParsePrefix("10.10.0.0/29").Contains(got) {
		t.Fatalf("leased %v, want inside 10.10.0.0/29", got)
	}
	if !vip.leases.IsHeldForTest(got) {
		t.Fatalf("the registry does not hold %v", got)
	}
	if saAt < 0 {
		t.Fatal("no SA payload: the Child SA was not created")
	}
	if cpAt > saAt {
		t.Fatalf("CP at %d after SA at %d; RFC 7296 Section 2.19 puts CP before SA", cpAt, saAt)
	}
}

// TestNetmaskOnlyBesideIPv4Address checks AC-2: one netmask when asked beside an IPv4
// address, none on an IPv6-only reply.
//
// RFC requirement: RFC7296-3.15.1-8 positive -- a request for an IPv4 address and the netmask is answered with exactly one INTERNAL_IP4_NETMASK, the mask of the pool's /29.
// RFC requirement: RFC7296-3.15.1-8 negative -- a request for an IPv6 address and the netmask, answered with an IPv6 address only, carries no INTERNAL_IP4_NETMASK.
func TestNetmaskOnlyBesideIPv4Address(t *testing.T) {
	ini, resp, ps := cpResponder(t, cpVIP(t, cpPool()), "ze", true)
	cp, _, _ := replyOf(cpAuth(t, ini, resp, ps, cfgRequest(ip4Ask(netip.Addr{}),
		wire.ConfigAttr{Type: wire.CPAttrInternalIP4Netmask})))
	masks := attrsOf(cp, wire.CPAttrInternalIP4Netmask)
	if len(masks) != 1 {
		t.Fatalf("%d netmasks, want exactly one", len(masks))
	}
	if !bytes.Equal(masks[0], net.CIDRMask(29, 32)) {
		t.Fatalf("netmask %v, want the pool's /29", masks[0])
	}

	ini, resp, ps = cpResponder(t, cpVIP(t, cpPool()), "ze", true)
	cp, _, _ = replyOf(cpAuth(t, ini, resp, ps, cfgRequest(
		wire.ConfigAttr{Type: wire.CPAttrInternalIP6Address},
		wire.ConfigAttr{Type: wire.CPAttrInternalIP4Netmask})))
	if cp == nil {
		t.Fatal("no CFG_REPLY for an IPv6 request")
	}
	if n := len(attrsOf(cp, wire.CPAttrInternalIP4Netmask)); n != 0 {
		t.Fatalf("an IPv6-only reply carries %d netmasks, want none", n)
	}
	v6 := attrsOf(cp, wire.CPAttrInternalIP6Address)
	if len(v6) != 1 || len(v6[0]) != 17 || v6[0][16] != 120 {
		t.Fatalf("INTERNAL_IP6_ADDRESS = %v, want 16 octets and prefix length 120", v6)
	}
}

// TestDNSAttributesSplitByFamily checks AC-3: the DNS server of each assigned family is
// sent unrequested, and no server of an unassigned family.
func TestDNSAttributesSplitByFamily(t *testing.T) {
	ini, resp, ps := cpResponder(t, cpVIP(t, cpPool()), "ze", true)
	cp, _, _ := replyOf(cpAuth(t, ini, resp, ps, cfgRequest(ip4Ask(netip.Addr{}))))
	dns4 := attrsOf(cp, wire.CPAttrInternalIP4DNS)
	if len(dns4) != 1 || netip.AddrFrom4([4]byte(dns4[0])) != netip.MustParseAddr("192.0.2.53") {
		t.Fatalf("INTERNAL_IP4_DNS = %v, want 192.0.2.53", dns4)
	}
	if n := len(attrsOf(cp, wire.CPAttrInternalIP6DNS)); n != 0 {
		t.Fatalf("%d IPv6 DNS servers beside an IPv4-only lease, want none", n)
	}
}

// TestRemoteSelectorIsLease checks AC-5: a peer that proposes and requests 10.1.2.3, which
// lies outside the pool, gets a lease, and both the answered TSi and the recorded remote
// selector name the lease, never 10.1.2.3.
func TestRemoteSelectorIsLease(t *testing.T) {
	vip := cpVIP(t, cpPool())
	ini, resp, ps := cpResponder(t, vip, "ze", true)
	answer := cpAuth(t, ini, resp, ps, cfgRequest(ip4Ask(netip.MustParseAddr("10.1.2.3"))))
	lease := resp.VirtualIPs()
	if len(lease) != 1 {
		t.Fatalf("SA holds %v, want one lease", lease)
	}
	ts := tsiOf(t, answer)
	if !bytes.Equal(ts.StartAddress, lease[0].AsSlice()) || !bytes.Equal(ts.EndAddress, lease[0].AsSlice()) {
		t.Fatalf("answered TSi %v-%v, want the lease %v", ts.StartAddress, ts.EndAddress, lease[0])
	}
	if !resp.NegotiatedTSi.IP.Equal(net.IP(lease[0].AsSlice())) {
		t.Fatalf("recorded remote selector %v, want the lease %v", resp.NegotiatedTSi, lease[0])
	}
	if ones, _ := resp.NegotiatedTSi.Mask.Size(); ones != 32 {
		t.Fatalf("recorded remote selector %v, want a host", resp.NegotiatedTSi)
	}
}

// TestTwoPeersSameRequestedAddressDistinctLeases connects two bound peers that both ask
// for the same in-pool address, 10.10.0.3, sharing one engine registry.
//
// Goal: AC-7 over two live responder sessions. Method: both IKE_AUTH requests run through
// handleAuthRequest; the leases and the recorded remote selectors are compared.
//
// RFC 3948 Section 5.1: "Implementors MUST devise ways of preventing this from occurring."
//
// RFC requirement: RFC3948-5.1-1 positive -- two pool-bound peers that both request the inner address 10.10.0.3 through IKE_AUTH are answered with two distinct leases, the first holding 10.10.0.3, and the two responder SAs record two distinct remote inner selectors, each its own lease.
func TestTwoPeersSameRequestedAddressDistinctLeases(t *testing.T) {
	vip := cpVIP(t, cpPool())
	want := netip.MustParseAddr("10.10.0.3")

	iniA, respA, psA := cpResponder(t, vip, "ari", true)
	cpAuth(t, iniA, respA, psA, cfgRequest(ip4Ask(want)))
	iniB, respB, psB := cpResponder(t, vip, "bob", true)
	cpAuth(t, iniB, respB, psB, cfgRequest(ip4Ask(want)))

	a, b := respA.VirtualIPs(), respB.VirtualIPs()
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("leases ari=%v bob=%v, want one each", a, b)
	}
	if a[0] != want {
		t.Fatalf("the first peer holds %v, want the requested %v", a[0], want)
	}
	if a[0] == b[0] {
		t.Fatalf("both peers hold %v: one inner address leads to two SAs", a[0])
	}
	if respA.NegotiatedTSi.IP.Equal(respB.NegotiatedTSi.IP) {
		t.Fatalf("both Child SAs name remote inner source %v", respA.NegotiatedTSi.IP)
	}
	if !respB.NegotiatedTSi.IP.Equal(net.IP(b[0].AsSlice())) {
		t.Fatalf("second Child SA names %v, want its lease %v", respB.NegotiatedTSi.IP, b[0])
	}
}

// TestNoChildSAWithoutLeaseOnBoundPeer refuses the Child SA of a bound peer that holds no
// lease: one that sent no request (FAILED_CP_REQUIRED, AC-21) and one whose pool is
// exhausted (INTERNAL_ADDRESS_FAILURE, AC-9). Both keep the IKE SA, carry no SA, TS or CP
// payload, and leave the peer holding no address.
//
// RFC 7296 Section 2.19: "IRAS MUST fail the request, and terminate the Child SA creation
// with a FAILED_CP_REQUIRED error."
//
// RFC requirement: RFC3948-5.1-1 negative -- a pool-bound peer that holds no lease, because it sent no CFG_REQUEST or because the pool is exhausted, is answered with no SA and no TS payload, so no Child SA naming its self-chosen inner address exists, and its IKE SA stays established.
// RFC requirement: RFC7296-2.19-6 positive -- a pool-bound peer whose IKE_AUTH carries no CP, or a CP of type CFG_REPLY in place of a CFG_REQUEST, is answered with FAILED_CP_REQUIRED and no SA, TS or CP payload, and is leased no address.
// RFC requirement: RFC7296-2.19-5 positive -- the same pool-bound peer, which sent no CP or a CFG_REPLY-typed CP and so no CFG_REQUEST, is sent no Configuration payload at all.
// RFC requirement: RFC7296-3.15.4-1 positive -- a pool-bound peer whose CFG_REQUEST meets an exhausted pool is answered with INTERNAL_ADDRESS_FAILURE and no SA, TS or CP payload, and the pool's held count is unchanged.
// RFC requirement: RFC7296-4-6 negative -- a CP of type CFG_REPLY carrying INTERNAL_IP4_ADDRESS is not parsed as a CFG_REQUEST: the bound peer draws FAILED_CP_REQUIRED and is leased no address.
func TestNoChildSAWithoutLeaseOnBoundPeer(t *testing.T) {
	cases := []struct {
		name   string
		pool   ipsec.VirtualIPPool
		cps    []*wire.PayloadCP
		notify uint16
	}{
		{"no request", cpPool(), nil, wire.NotifyFailedCPRequired},
		{"reply type", cpPool(), []*wire.PayloadCP{{CFGType: wire.CFGTypeReply, Attrs: []wire.ConfigAttr{ip4Ask(netip.Addr{})}}}, wire.NotifyFailedCPRequired},
		{"exhausted", ipsec.VirtualIPPool{Name: "rw", Range: "10.10.0.0/30"}, []*wire.PayloadCP{cfgRequest(ip4Ask(netip.Addr{}))}, wire.NotifyInternalAddressFailure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vip := cpVIP(t, tc.pool)
			if tc.notify == wire.NotifyInternalAddressFailure {
				p, _ := vip.pool("rw")
				for _, owner := range []string{"x", "y"} {
					if _, err := vip.leases.Acquire(p, owner, eap.FamilyIPv4, netip.Addr{}); err != nil {
						t.Fatalf("fill the pool: %v", err)
					}
				}
			}
			before := vip.leases.Held("rw")
			ini, resp, ps := cpResponder(t, vip, "ze", true)
			answer := cpAuth(t, ini, resp, ps, tc.cps...)
			if !notifyIn(answer, tc.notify) {
				t.Fatalf("no %s in the response", wire.NotifyTypeName(tc.notify))
			}
			for _, e := range answer {
				switch e.Payload.(type) {
				case *wire.PayloadSA, *wire.PayloadTS, *wire.PayloadCP:
					t.Fatalf("refused response carries %T", e.Payload)
				}
			}
			if resp.virtualIP != nil {
				t.Fatalf("refused peer holds %v", resp.VirtualIPs())
			}
			if got := vip.leases.Held("rw"); got != before {
				t.Fatalf("held count %d, want %d: no address handed out", got, before)
			}
		})
	}
}

// TestFamilyFidelity checks AC-15: an IPv6 request against an IPv4-only pool draws
// INTERNAL_ADDRESS_FAILURE; a dual request gets the IPv4 address only.
//
// RFC requirement: RFC7296-4-7 positive -- an IPv4 and IPv6 request against an IPv4-only pool is answered with one INTERNAL_IP4_ADDRESS and no INTERNAL_IP6_ADDRESS.
// RFC requirement: RFC7296-4-7 negative -- an IPv6-only request against an IPv4-only pool is answered with INTERNAL_ADDRESS_FAILURE and no Configuration payload, so no address of the other type.
// RFC requirement: RFC7296-3.15.4-1 negative -- an IPv4 and IPv6 request against an IPv4-only pool, one family of which can be assigned, is answered with a CFG_REPLY and no INTERNAL_ADDRESS_FAILURE.
func TestFamilyFidelity(t *testing.T) {
	v4only := ipsec.VirtualIPPool{Name: "rw", Range: "10.10.0.0/29"}
	ini, resp, ps := cpResponder(t, cpVIP(t, v4only), "ze", true)
	answer := cpAuth(t, ini, resp, ps, cfgRequest(wire.ConfigAttr{Type: wire.CPAttrInternalIP6Address}))
	if !notifyIn(answer, wire.NotifyInternalAddressFailure) {
		t.Fatal("IPv6 request against an IPv4-only pool: no INTERNAL_ADDRESS_FAILURE")
	}
	if cp, _, _ := replyOf(answer); cp != nil {
		t.Fatalf("IPv6 request against an IPv4-only pool answered with %+v, want no CP", cp)
	}

	ini, resp, ps = cpResponder(t, cpVIP(t, v4only), "ze", true)
	dual := cpAuth(t, ini, resp, ps, cfgRequest(ip4Ask(netip.Addr{}),
		wire.ConfigAttr{Type: wire.CPAttrInternalIP6Address}))
	if notifyIn(dual, wire.NotifyInternalAddressFailure) {
		t.Fatal("dual request with the IPv4 half assignable drew INTERNAL_ADDRESS_FAILURE")
	}
	cp, _, _ := replyOf(dual)
	if cp == nil {
		t.Fatal("dual request: no CFG_REPLY")
	}
	if n := len(attrsOf(cp, wire.CPAttrInternalIP6Address)); n != 0 {
		t.Fatalf("dual request against an IPv4-only pool answered %d IPv6 addresses", n)
	}
	if n := len(attrsOf(cp, wire.CPAttrInternalIP4Address)); n != 1 {
		t.Fatalf("dual request answered %d IPv4 addresses, want 1", n)
	}
}

// TestNoCPReplyWithoutRequest checks AC-12 and AC-16: an unbound peer sending any CP
// shape, a well-formed request included, gets no CP and its usual selectors.
//
// RFC 7296 Section 2.19: "The responder MUST NOT send a CFG_REPLY without having first
// received a CP(CFG_REQUEST) from the initiator".
//
// RFC requirement: RFC7296-2.19-5 negative -- a peer bound to no pool that sends a CFG_REQUEST, a CFG_SET, a CFG_ACK, an empty CFG_REQUEST or two CFG_REQUESTs is answered with no Configuration payload, keeps its Child SA, and gets the remote selector it gets without any CP.
// RFC requirement: RFC7296-2.19-6 negative -- the same peer, whose configuration requires no CP, is not refused for sending no usable CFG_REQUEST: every one of those responses carries the SA payload.
func TestNoCPReplyWithoutRequest(t *testing.T) {
	_, plainResp, _ := func() (*SA, *SA, *PeerSession) {
		ini, resp, ps := cpResponder(t, nil, "ze", false)
		cpAuth(t, ini, resp, ps)
		return ini, resp, ps
	}()
	shapes := map[string][]*wire.PayloadCP{
		"request": {cfgRequest(ip4Ask(netip.Addr{}))},
		"set":     {{CFGType: wire.CFGTypeSet, Attrs: []wire.ConfigAttr{ip4Ask(netip.Addr{})}}},
		"ack":     {{CFGType: wire.CFGTypeACK}},
		"empty":   {cfgRequest()},
		"two":     {cfgRequest(ip4Ask(netip.Addr{})), cfgRequest(ip4Ask(netip.Addr{}))},
	}
	for name, cps := range shapes {
		t.Run(name, func(t *testing.T) {
			ini, resp, ps := cpResponder(t, cpVIP(t, cpPool()), "ze", false)
			cp, _, saAt := replyOf(cpAuth(t, ini, resp, ps, cps...))
			if cp != nil {
				t.Fatalf("unbound peer answered with %+v", cp)
			}
			if saAt < 0 {
				t.Fatal("unbound peer lost its Child SA")
			}
			if resp.NegotiatedTSi.String() != plainResp.NegotiatedTSi.String() {
				t.Fatalf("selector %v, want %v as without CP", resp.NegotiatedTSi, plainResp.NegotiatedTSi)
			}
		})
	}
}

// TestUnknownAttributesIgnored checks AC-13, AC-14 and AC-23: unknown types and the
// obsolete INTERNAL_ADDRESS_EXPIRY are ignored, and an address request with the Reserved
// bit set (cleared by the codec) is answered.
//
// RFC requirement: RFC7296-3.15.1-10 positive -- a pool-bound peer's CFG_REQUEST carrying the unassigned type 0x3abc beside INTERNAL_IP4_ADDRESS is answered with one INTERNAL_IP4_ADDRESS, and the reply does not echo type 0x3abc.
// RFC requirement: RFC7296-1.7-3 positive -- the same CFG_REQUEST also carries the obsolete type 5, INTERNAL_ADDRESS_EXPIRY; it is answered with one INTERNAL_IP4_ADDRESS, and the reply does not echo type 5.
func TestUnknownAttributesIgnored(t *testing.T) {
	// internalAddressExpiry is type 5, which RFC 7296 Section 1.7 removed; the wire
	// package carries no constant for it.
	const internalAddressExpiry uint16 = 5
	req := cfgRequest(
		wire.ConfigAttr{Type: 0x3abc, Value: []byte{1, 2}},
		wire.ConfigAttr{Type: internalAddressExpiry},
		ip4Ask(netip.Addr{}),
	)
	buf := make([]byte, req.Len())
	req.WriteTo(buf, 0)
	buf[4] |= 0x80 // Reserved bit of the first attribute
	buf[10] |= 0x80
	var parsed wire.PayloadCP
	if err := parsed.ReadFrom(buf); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	ini, resp, ps := cpResponder(t, cpVIP(t, cpPool()), "ze", true)
	cp, _, _ := replyOf(cpAuth(t, ini, resp, ps, &parsed))
	if cp == nil || len(attrsOf(cp, wire.CPAttrInternalIP4Address)) != 1 {
		t.Fatalf("reply %+v, want one INTERNAL_IP4_ADDRESS", cp)
	}
	for _, a := range cp.Attrs {
		if a.Type == 0x3abc || a.Type == internalAddressExpiry {
			t.Fatalf("reply echoes ignored type %d", a.Type)
		}
	}
}

// TestSupportedAttributesDerived checks AC-24 and AC-29 (IKE_AUTH): SUPPORTED_ATTRIBUTES
// lists exactly the cpHandlers types, and APPLICATION_VERSION is answered empty.
//
// RFC requirement: RFC7296-3.15.1-9 positive -- a zero-length SUPPORTED_ATTRIBUTES in a pool-bound peer's CFG_REQUEST is answered with one SUPPORTED_ATTRIBUTES attribute listing, two octets each and in order, exactly the types cpHandlers handles.
func TestSupportedAttributesDerived(t *testing.T) {
	ini, resp, ps := cpResponder(t, cpVIP(t, cpPool()), "ze", true)
	cp, _, _ := replyOf(cpAuth(t, ini, resp, ps, cfgRequest(ip4Ask(netip.Addr{}),
		wire.ConfigAttr{Type: wire.CPAttrSupportedAttributes},
		wire.ConfigAttr{Type: wire.CPAttrApplicationVersion})))
	sup := attrsOf(cp, wire.CPAttrSupportedAttributes)
	if len(sup) != 1 || len(sup[0]) != 2*len(cpHandlers) {
		t.Fatalf("SUPPORTED_ATTRIBUTES = %v, want %d types", sup, len(cpHandlers))
	}
	for i := range cpHandlers {
		got := uint16(sup[0][2*i])<<8 | uint16(sup[0][2*i+1])
		if got != cpHandlers[i].attr {
			t.Fatalf("type %d = %d, want %d", i, got, cpHandlers[i].attr)
		}
	}
	ver := attrsOf(cp, wire.CPAttrApplicationVersion)
	if len(ver) != 1 || len(ver[0]) != 0 {
		t.Fatalf("APPLICATION_VERSION = %v, want one empty value", ver)
	}
}

// TestSupportedAttributesOnlyForZeroLengthQuery checks the other side of AC-24: a
// SUPPORTED_ATTRIBUTES that carries a value is not the zero-length query RFC 7296 Section
// 3.15.1 defines, and a request that asks none gets none.
//
// Goal: the reply lists supported types only when asked the defined way. Method: two
// bound peers, one asking with a two-octet value, one not asking; neither reply carries
// SUPPORTED_ATTRIBUTES, and both carry the requested address.
//
// RFC requirement: RFC7296-3.15.1-9 negative -- a SUPPORTED_ATTRIBUTES carrying a value, which the MUST forbids in a request, is not read as the query: the reply carries one INTERNAL_IP4_ADDRESS and no SUPPORTED_ATTRIBUTES, and neither does the reply to a request that asks none.
func TestSupportedAttributesOnlyForZeroLengthQuery(t *testing.T) {
	for name, extra := range map[string][]wire.ConfigAttr{
		"valued query": {{Type: wire.CPAttrSupportedAttributes, Value: []byte{0, 1}}},
		"not asked":    nil,
	} {
		t.Run(name, func(t *testing.T) {
			ini, resp, ps := cpResponder(t, cpVIP(t, cpPool()), "ze", true)
			cp, _, _ := replyOf(cpAuth(t, ini, resp, ps, cfgRequest(append([]wire.ConfigAttr{ip4Ask(netip.Addr{})}, extra...)...)))
			if cp == nil || len(attrsOf(cp, wire.CPAttrInternalIP4Address)) != 1 {
				t.Fatalf("reply %+v, want one INTERNAL_IP4_ADDRESS", cp)
			}
			if sup := attrsOf(cp, wire.CPAttrSupportedAttributes); len(sup) != 0 {
				t.Fatalf("reply carries SUPPORTED_ATTRIBUTES %v, want none", sup)
			}
		})
	}
}

// TestOnlyUnrecognizedAttributesIsNoRequest checks AC-13 and AC-23 from the refusal side:
// a CFG_REQUEST whose only attribute is ignored asks no address, so a pool-bound peer
// that sends it has sent no CFG_REQUEST Ze serves.
//
// Goal: an ignored attribute is never read as an address request. Method: a bound peer
// sends a CFG_REQUEST holding only the unassigned type 0x3abc, then only type 5; each is
// answered with FAILED_CP_REQUIRED, no CP, and no lease.
//
// RFC requirement: RFC7296-3.15.1-10 negative -- a pool-bound peer whose CFG_REQUEST holds only the unassigned type 0x3abc is answered with FAILED_CP_REQUIRED and no Configuration payload, and holds no lease: the ignored attribute is not read as an address request.
// RFC requirement: RFC7296-1.7-3 negative -- the same for a CFG_REQUEST holding only type 5, INTERNAL_ADDRESS_EXPIRY: FAILED_CP_REQUIRED, no Configuration payload, no lease.
func TestOnlyUnrecognizedAttributesIsNoRequest(t *testing.T) {
	// internalAddressExpiry is type 5, which RFC 7296 Section 1.7 removed; the wire
	// package carries no constant for it.
	const internalAddressExpiry uint16 = 5
	for name, typ := range map[string]uint16{
		"unassigned type":         0x3abc,
		"INTERNAL_ADDRESS_EXPIRY": internalAddressExpiry,
	} {
		t.Run(name, func(t *testing.T) {
			vip := cpVIP(t, cpPool())
			ini, resp, ps := cpResponder(t, vip, "ze", true)
			answer := cpAuth(t, ini, resp, ps, cfgRequest(wire.ConfigAttr{Type: typ}))
			if !notifyIn(answer, wire.NotifyFailedCPRequired) {
				t.Fatal("a CFG_REQUEST holding only an ignored attribute drew no FAILED_CP_REQUIRED")
			}
			if cp, _, _ := replyOf(answer); cp != nil {
				t.Fatalf("answered with %+v, want no CP", cp)
			}
			if got := vip.leases.Held("rw"); got != 0 {
				t.Fatalf("the pool holds %d leases, want none", got)
			}
		})
	}
}

// TestLeaseReleasedOnEveryClosePath checks AC-10, AC-11 and AC-25 over the single close
// point: forgetKeys returns the address once, and the peer gets the same address back on
// each reconnect.
func TestLeaseReleasedOnEveryClosePath(t *testing.T) {
	vip := cpVIP(t, cpPool())
	var first netip.Addr
	for cycle := range 3 {
		ini, resp, ps := cpResponder(t, vip, "ze", true)
		cpAuth(t, ini, resp, ps, cfgRequest(ip4Ask(netip.Addr{})))
		got := resp.VirtualIPs()
		if len(got) != 1 {
			t.Fatalf("cycle %d: %v leases", cycle, got)
		}
		if cycle == 0 {
			first = got[0]
		} else if got[0] != first {
			t.Fatalf("cycle %d: got %v, want the previous address %v", cycle, got[0], first)
		}
		resp.forgetKeys()
		resp.forgetKeys()
		if n := vip.leases.Held("rw"); n != 0 {
			t.Fatalf("cycle %d: %d leases held after close, want 0", cycle, n)
		}
	}
}

// TestParallelReinitSharesLease checks AC-26: a second IKE SA of the same peer holds the
// same address, the held count rises by none, and the address stays held after the first
// SA closes.
func TestParallelReinitSharesLease(t *testing.T) {
	vip := cpVIP(t, cpPool())
	ini1, resp1, ps1 := cpResponder(t, vip, "ze", true)
	cpAuth(t, ini1, resp1, ps1, cfgRequest(ip4Ask(netip.Addr{})))
	ini2, resp2, ps2 := cpResponder(t, vip, "ze", true)
	cpAuth(t, ini2, resp2, ps2, cfgRequest(ip4Ask(netip.Addr{})))
	if resp1.VirtualIPs()[0] != resp2.VirtualIPs()[0] {
		t.Fatalf("re-initiation got %v, want %v", resp2.VirtualIPs(), resp1.VirtualIPs())
	}
	if n := vip.leases.Held("rw"); n != 1 {
		t.Fatalf("held %d, want 1", n)
	}
	resp1.forgetKeys()
	if !vip.leases.IsHeldForTest(resp2.VirtualIPs()[0]) {
		t.Fatal("closing the superseded SA freed the live SA's address")
	}
}

// leaseOnto grants sa one IPv4 lease from vip's pool "rw", as assignVirtualIP does, and
// returns the leased address.
func leaseOnto(t *testing.T, vip *virtualIPState, sa *SA) netip.Addr {
	t.Helper()
	pool, ok := vip.pool("rw")
	if !ok {
		t.Fatal("pool rw is not built")
	}
	lease, err := vip.leases.Acquire(pool, sa.PeerName, eap.FamilyIPv4, netip.Addr{})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	sa.virtualIP = &virtualIPGrant{leases: vip.leases, pool: pool, held: []*eap.Lease{lease},
		req: &cpRequest{want4: true}}
	return lease.Addr()
}

// TestIKERekeyKeepsLease checks AC-32 in both directions, through adoptRekeyedIKESA, the
// swap runEstablished performs for every rekeyed IKE SA. Ze initiating: Ze's rekey
// request, the peer's real respondIKERekey answer, and Ze's handling of that answer
// produce the successor. Ze responding: the peer's rekey request staged in
// pendingIKESwap and the peer's Delete of the old IKE SA produce it. In each the
// successor holds the lease, the held count stays 1 after the retired SA closed, and the
// successor's own close releases it.
func TestIKERekeyKeepsLease(t *testing.T) {
	log := slogutil.DiscardLogger()
	check := func(t *testing.T, vip *virtualIPState, ps *PeerSession, retired, successor *SA, lease netip.Addr) {
		t.Helper()
		if successor == nil {
			t.Fatal("the rekey produced no successor IKE SA")
		}
		ps.adoptRekeyedIKESA(retired, successor, NewSATable(), log)
		if got := successor.VirtualIPs(); len(got) != 1 || got[0] != lease {
			t.Fatalf("the successor holds %v, want the lease %v", got, lease)
		}
		if retired.VirtualIPs() != nil {
			t.Fatal("the retired SA still holds the lease after the swap")
		}
		if !vip.leases.IsHeldForTest(lease) {
			t.Fatal("the retired SA's close released the lease its successor holds")
		}
		if n := vip.leases.Held("rw"); n != 1 {
			t.Fatalf("held count %d after the rekey, want 1", n)
		}
		successor.forgetKeys()
		if n := vip.leases.Held("rw"); n != 0 {
			t.Fatalf("held count %d after the successor closed, want 0", n)
		}
	}

	t.Run("Ze initiating", func(t *testing.T) {
		vip := cpVIP(t, cpPool())
		r := newRefusedRig(t)
		lease := leaseOnto(t, vip, r.sa)
		respond := func(req []wire.PayloadEntry, msgID uint32) []byte {
			answer, peerNew, err := respondIKERekey(r.peer, req, msgID, log)
			if err != nil {
				t.Fatalf("the peer refused the rekey: %v", err)
			}
			peerNew.forgetKeys()
			return answer
		}
		r.ps.startIKERekey(r.sa, r.sa.IKEGroup, r.myTr, log)
		_, out := r.relayToPeer(t, respond)
		check(t, vip, r.ps, r.sa, out.newSA, lease)
	})

	t.Run("Ze responding", func(t *testing.T) {
		vip := cpVIP(t, cpPool())
		link := errLink(t)
		lease := leaseOnto(t, vip, link.resp)
		reqBytes, pending, err := initiateIKERekey(link.ini, link.ini.IKEGroup)
		if err != nil {
			t.Fatalf("initiateIKERekey: %v", err)
		}
		defer pending.clear()
		reqMsg := parseMsg(t, reqBytes)
		inner, err := decryptAndParse(link.resp, reqMsg, reqBytes)
		if err != nil {
			t.Fatalf("the responder could not decrypt the rekey request: %v", err)
		}
		link.ps.handleCreateChildSAOwned(link.resp, reqMsg, inner, false, link.myTr, nil, log)
		if link.ps.pendingIKESwap == nil {
			t.Fatal("the peer's IKE SA rekey staged no swap")
		}
		del := lcyRequest(t, link.ini, link.resp.ExpectedMsgID,
			[]wire.PayloadEntry{{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}}})
		out := link.ps.handleOwnedInbound(link.resp, transport.Packet{Data: del}, link.myTr, nil, log)
		check(t, vip, link.ps, link.resp, out.newSA, lease)
	})
}

// TestCreateChildUsesLease checks AC-7b at narrowChildSelectors, the entry CREATE_CHILD_SA
// and both Child rekey paths share: a leased SA's later Child SA proposing 10.1.2.3 is
// narrowed to the lease.
func TestCreateChildUsesLease(t *testing.T) {
	vip := cpVIP(t, cpPool())
	ini, resp, ps := cpResponder(t, vip, "ze", true)
	cpAuth(t, ini, resp, ps, cfgRequest(ip4Ask(netip.Addr{})))
	lease := resp.VirtualIPs()[0]
	self := wire.TrafficSelector{TSType: wire.TSTypeIPv4AddrRange, EndPort: 65535,
		StartAddress: []byte{10, 1, 2, 3}, EndAddress: []byte{10, 1, 2, 3}}
	any4 := wire.TrafficSelector{TSType: wire.TSTypeIPv4AddrRange, EndPort: 65535,
		StartAddress: []byte{0, 0, 0, 0}, EndAddress: []byte{255, 255, 255, 255}}
	tsi := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSi, TrafficSelectors: []wire.TrafficSelector{self}}
	tsr := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSr, TrafficSelectors: []wire.TrafficSelector{any4}}
	if err := narrowChildSelectors(resp, tsi, tsr, nil); err != nil {
		t.Fatalf("narrowChildSelectors: %v", err)
	}
	if !resp.NegotiatedTSi.IP.Equal(net.IP(lease.AsSlice())) {
		t.Fatalf("second Child SA names %v, want the lease %v", resp.NegotiatedTSi, lease)
	}
}

// TestPoolBoundPeerDeclinesTransportMode checks D-1: a bound peer asking for transport
// mode is answered in tunnel mode.
func TestPoolBoundPeerDeclinesTransportMode(t *testing.T) {
	sa := &SA{PeerCfg: ipsec.SiteToSitePeer{VirtualIP: &ipsec.VirtualIPBinding{Pool: "rw"}}}
	sa.PeerRequestedTransport = true
	decideResponderTransportMode(sa)
	if sa.UseTransportMode {
		t.Fatal("a pool-bound peer was granted transport mode")
	}
}

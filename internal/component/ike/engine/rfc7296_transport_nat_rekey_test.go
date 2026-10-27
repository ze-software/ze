// Design: docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md -- RFC 7296 Section 2.23.1 transport mode NAT traversal
// Related: rekey.go -- respondChildRekey and applyChildRekeyResponse, the two rekey roles
// Related: ts_narrow.go -- narrowChildSelectorsIn and recordInitiatorSelectorsIn, gated on the exchange's mode
// Related: ts_nat_substitute.go -- the substitution producers
// VALIDATES: a transport-mode Child SA behind a NAT is rekeyed, in either role, on the
// successor of a real IKE SA rekey with the RFC 7296 Section 2.23.1 address substitution
// and the single-address rule applied, because the rekey judges its selectors in the mode
// of the Child SA it replaces rather than in flags the IKE SA rekey does not carry.
// PREVENTS: the TS_UNACCEPTABLE answer (ze responding) and the refused answer (ze
// initiating) that every NATed transport-mode Child SA met at its first rekey after an
// IKE SA rekey, after which it expired.

package engine

import (
	"errors"
	"net"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// natRekeyRig is an established IKE SA pair that has been rekeyed once, through the real
// rekey producers, with one installed transport-mode Child SA on each end. The peer is
// the NAT client of the figure in ts_nat_substitute_test.go and the IKE SA initiator; ze
// is the server and the IKE SA responder. peer and ze are the SUCCESSOR SAs.
type natRekeyRig struct {
	peer, ze           *SA
	peerChild, zeChild *ChildSA
}

// hostSel is a selector naming one IPv4 host, any protocol and any port.
func hostSel(t *testing.T, addr string) tsSelector {
	t.Helper()
	return tsSelector{Net: mustNet(t, addr+"/32"), Port: ipsec.AnyPort()}
}

// rekeyIKEPair rekeys the IKE SA between ini and resp through initiateIKERekey,
// respondIKERekey and applyIKERekeyResponse, and returns the two successors.
func rekeyIKEPair(t *testing.T, ini, resp *SA) (newIni, newResp *SA) {
	t.Helper()
	log := slogutil.DiscardLogger()
	req, pending, err := initiateIKERekey(ini, ini.IKEGroup)
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	defer pending.clear()
	answer, newResp, err := respondIKERekey(resp, mbDecrypt(t, resp, req), parseMsg(t, req).Header.MessageID, log)
	if err != nil {
		t.Fatalf("respondIKERekey: %v", err)
	}
	newIni, err = applyIKERekeyResponse(ini, pending, mbDecrypt(t, ini, answer), log)
	if err != nil {
		t.Fatalf("applyIKERekeyResponse: %v", err)
	}
	return newIni, newResp
}

// natRekeySetup builds the rig. NAT B always sits in front of ze, and peerBehindNAT says
// whether NAT A sits in front of the peer; each end holds the verdict from its own side.
// The IKE_AUTH exchange negotiated transport mode (UseTransportMode true on both
// originals), and both first Child SAs carry the host pair in their own end's address
// space, in IKE_AUTH's orientation (TSi is the client).
func natRekeySetup(t *testing.T, peerBehindNAT bool) *natRekeyRig {
	t.Helper()
	log := slogutil.DiscardLogger()
	const behindNAT = true
	ini, resp, _ := establishPSK(t)
	ini.PeerCfg.LocalAddress, ini.PeerCfg.RemoteAddress = natClientReal, natServerPublic
	resp.PeerCfg.LocalAddress, resp.PeerCfg.RemoteAddress = natServerReal, natClientPublic
	// A NAT on the path makes every Child SA UDP-encapsulated, which needs the
	// authenticated endpoint (the observed address) and a bound NAT-T socket.
	ini.peerEndpoint = &net.UDPAddr{IP: net.ParseIP(natServerPublic), Port: 4500}
	resp.peerEndpoint = &net.UDPAddr{IP: net.ParseIP(natClientPublic), Port: 4500}
	for _, sa := range []*SA{ini, resp} {
		sa.PeerCfg.Mode = dataplane.ModeTransport
		sa.UseTransportMode = true
		sa.nattSocket = mbTransport(t, "127.0.0.1:0", true)
		sa.NATDetected = peerBehindNAT || behindNAT
	}
	ini.BehindNAT, ini.PeerBehindNAT = peerBehindNAT, behindNAT
	resp.PeerBehindNAT, resp.BehindNAT = peerBehindNAT, behindNAT

	peerPair := []tsPair{{I: hostSel(t, natClientReal), R: hostSel(t, natServerPublic)}}
	zePair := []tsPair{{I: hostSel(t, natClientPublic), R: hostSel(t, natServerReal)}}
	ini.NegotiatedPairs, ini.NegotiatedTSi, ini.NegotiatedTSr = peerPair, peerPair[0].I.Net, peerPair[0].R.Net
	resp.NegotiatedPairs, resp.NegotiatedTSi, resp.NegotiatedTSr = zePair, zePair[0].I.Net, zePair[0].R.Net
	peerChild, err := createFirstChildSA(ini, testESPGroup(), ini.PeerCfg.LocalAddress, ini.PeerCfg.RemoteAddress, 1, nil, log)
	if err != nil {
		t.Fatalf("the peer's first Child SA: %v", err)
	}
	t.Cleanup(peerChild.Clear)
	zeChild, err := createFirstChildSA(resp, testESPGroup(), resp.PeerCfg.LocalAddress, resp.PeerCfg.RemoteAddress, 1, nil, log)
	if err != nil {
		t.Fatalf("ze's first Child SA: %v", err)
	}
	t.Cleanup(zeChild.Clear)
	if peerChild.Mode != modeTransport || zeChild.Mode != modeTransport {
		t.Fatalf("setup: first Child SAs run modes %d and %d, want transport", peerChild.Mode, zeChild.Mode)
	}

	peer, ze := rekeyIKEPair(t, ini, resp)
	// The precondition the defect needs. RFC 7296 Section 2.8: "An IKE SA so created
	// inherits all of the original IKE SA's Child SAs", and the successor carries no
	// record of the IKE_AUTH mode.
	if peer.UseTransportMode || ze.UseTransportMode {
		t.Fatalf("setup: a successor IKE SA carries UseTransportMode (peer %v, ze %v); the rekey must not",
			peer.UseTransportMode, ze.UseTransportMode)
	}
	if ze.PeerBehindNAT != peerBehindNAT || ze.BehindNAT != behindNAT {
		t.Fatal("setup: the IKE SA rekey did not carry ze's NAT verdict (inheritSendPath)")
	}
	return &natRekeyRig{peer: peer, ze: ze, peerChild: peerChild, zeChild: zeChild}
}

// tsAddrs returns the address of each selector of the TSi and TSr payloads of a chain.
func tsAddrs(inner []wire.PayloadEntry) (tsi, tsr []string) {
	for i := range inner {
		p, ok := inner[i].Payload.(*wire.PayloadTS)
		if !ok {
			continue
		}
		for _, s := range p.TrafficSelectors {
			a := net.IP(s.StartAddress).String()
			if p.TSPayloadType == wire.PayloadTypeTSi {
				tsi = append(tsi, a)
			} else {
				tsr = append(tsr, a)
			}
		}
	}
	return tsi, tsr
}

// replaceTS swaps the TSi and TSr payloads of a decrypted chain for tsi and tsr.
func replaceTS(inner []wire.PayloadEntry, tsi, tsr *wire.PayloadTS) []wire.PayloadEntry {
	out := make([]wire.PayloadEntry, 0, len(inner))
	for i := range inner {
		p, ok := inner[i].Payload.(*wire.PayloadTS)
		if !ok {
			out = append(out, inner[i])
			continue
		}
		if p.TSPayloadType == wire.PayloadTypeTSi {
			out = append(out, wire.PayloadEntry{Payload: tsi})
		} else {
			out = append(out, wire.PayloadEntry{Payload: tsr})
		}
	}
	return out
}

// TestTransportChildRekeyAfterIKERekeySubstitutesBehindNAT covers AC-1 and AC-3.
//
// Goal: a NATed transport-mode Child SA survives its rekey after an IKE SA rekey, in both
// roles. Method: the real IKE SA rekey produces both successors; the Child SA rekey then
// runs through initiateChildRekey, respondChildRekey and applyChildRekeyResponse over the
// encrypted wire bytes, both NATs present.
func TestTransportChildRekeyAfterIKERekeySubstitutesBehindNAT(t *testing.T) {
	log := slogutil.DiscardLogger()

	t.Run("peer_rekeys", func(t *testing.T) {
		r := natRekeySetup(t, true)
		raw, pending, err := initiateChildRekey(r.peer, r.peerChild)
		if err != nil {
			t.Fatalf("the peer's initiateChildRekey: %v", err)
		}
		defer pending.clear()
		dp := &mockDP{}
		answer, next, err := respondChildRekey(r.ze, mbDecrypt(t, r.ze, raw), r.zeChild,
			parseMsg(t, raw).Header.MessageID, dp, log)
		if err != nil {
			t.Fatalf("ze refused the peer's transport-mode Child SA rekey: %v", err)
		}
		defer next.Clear()
		inner := mbDecrypt(t, r.peer, answer)
		if !cctCarriesTransportNotify(inner) {
			t.Error("ze's answer carries no USE_TRANSPORT_MODE")
		}
		tsi, tsr := tsAddrs(inner)
		if len(tsi) != 1 || tsi[0] != natClientPublic || len(tsr) != 1 || tsr[0] != natServerReal {
			t.Errorf("ze answered TSi %v TSr %v, want the substituted [%s] [%s]", tsi, tsr, natClientPublic, natServerReal)
		}
		if next.Mode != modeTransport {
			t.Errorf("ze's replacement runs mode %d, want transport (%d)", next.Mode, modeTransport)
		}
		want := pairsText([]tsPair{{I: hostSel(t, natClientPublic), R: hostSel(t, natServerReal)}})
		if got := pairsText(next.Selectors); got != want {
			t.Errorf("ze's replacement selectors = %s, want %s", got, want)
		}
		if got := r.ze.OriginalTSiAddr.String(); got != natClientReal {
			t.Errorf("OriginalTSiAddr = %s, want the proposal's %s", got, natClientReal)
		}
		if got := r.ze.OriginalTSrAddr.String(); got != natServerPublic {
			t.Errorf("OriginalTSrAddr = %s, want the proposal's %s", got, natServerPublic)
		}
		peerNext, err := applyChildRekeyResponse(r.peer, pending, inner, nil, log)
		if err != nil {
			t.Fatalf("the peer refused ze's answer: %v", err)
		}
		defer peerNext.Clear()
	})

	t.Run("ze_rekeys", func(t *testing.T) {
		r := natRekeySetup(t, true)
		raw, pending, err := initiateChildRekey(r.ze, r.zeChild)
		if err != nil {
			t.Fatalf("ze's initiateChildRekey: %v", err)
		}
		defer pending.clear()
		answer, peerNext, err := respondChildRekey(r.peer, mbDecrypt(t, r.peer, raw), r.peerChild,
			parseMsg(t, raw).Header.MessageID, nil, log)
		if err != nil {
			t.Fatalf("the peer refused ze's transport-mode Child SA rekey: %v", err)
		}
		defer peerNext.Clear()
		inner := mbDecrypt(t, r.ze, answer)
		// The peer answers in ITS address space: ze, the exchange initiator, as the peer
		// sees it (IPN2), and the peer's own IP1.
		tsi, tsr := tsAddrs(inner)
		if len(tsi) != 1 || tsi[0] != natServerPublic || len(tsr) != 1 || tsr[0] != natClientReal {
			t.Fatalf("setup: the peer answered TSi %v TSr %v, want [%s] [%s]", tsi, tsr, natServerPublic, natClientReal)
		}
		dp := &mockDP{}
		child, err := applyChildRekeyResponse(r.ze, pending, inner, dp, log)
		if err != nil {
			t.Fatalf("ze refused the peer's answer to its transport-mode rekey: %v", err)
		}
		defer child.Clear()
		if child.Mode != modeTransport {
			t.Errorf("ze's replacement runs mode %d, want transport (%d)", child.Mode, modeTransport)
		}
		want := pairsText([]tsPair{{I: hostSel(t, natServerReal), R: hostSel(t, natClientPublic)}})
		if got := pairsText(child.Selectors); got != want || !child.SelectorsLocalIsTSi {
			t.Errorf("ze's replacement selectors = %s (local is TSi %v), want %s in ze's space, local TSi",
				got, child.SelectorsLocalIsTSi, want)
		}
	})
}

// TestTransportChildRekeyAfterIKERekeyHoldsOneAddress covers AC-2.
//
// Goal: a transport-mode Child SA rekeyed after an IKE SA rekey installs exactly the one
// host pair, in transport mode. Method: ze sits behind NAT B, the peer behind none, and
// ze's operator wrote a remote range (203.0.113.0/24), so policy alone admits a range;
// the peer's real rekey request is answered with its TSi replaced by the peer's own /32,
// then by that /24.
//
// The RFC 7296 Section 2.23.1 claim is the /32 half only. The /24 half is Section 2.9.2
// floor behavior: a rekey carries the retired pair as its floor, narrowSelectors returns
// that floor for any proposal covering it, and a range that excludes the floor is refused
// as a Section 2.9.2 shrink. The single-address gate in narrowChildSelectorsIn therefore
// never decides a transport rekey, so the /24 half cannot discriminate that gate; the
// gate's negative is carried by the IKE_AUTH path (TestTransportModeUsesExactlyOneAddress).
func TestTransportChildRekeyAfterIKERekeyHoldsOneAddress(t *testing.T) {
	log := slogutil.DiscardLogger()
	rekeyWith := func(t *testing.T, tsiCIDR string) (*natRekeyRig, []byte, *ChildSA, error) {
		t.Helper()
		r := natRekeySetup(t, false)
		r.ze.PeerCfg.TrafficSelectors = []ipsec.TrafficSelectorPolicy{{
			Number:       "1",
			LocalPrefix:  mustNet(t, natServerReal+"/32"),
			LocalPort:    ipsec.AnyPort(),
			RemotePrefix: mustNet(t, "203.0.113.0/24"),
			RemotePort:   ipsec.AnyPort(),
		}}
		raw, pending, err := initiateChildRekey(r.peer, r.peerChild)
		if err != nil {
			t.Fatalf("the peer's initiateChildRekey: %v", err)
		}
		defer pending.clear()
		inner := replaceTS(mbDecrypt(t, r.ze, raw),
			tsPayload(t, wire.PayloadTypeTSi, tsiCIDR),
			tsPayload(t, wire.PayloadTypeTSr, natServerPublic+"/32"))
		dp := &mockDP{}
		answer, next, err := respondChildRekey(r.ze, inner, r.zeChild, parseMsg(t, raw).Header.MessageID, dp, log)
		return r, answer, next, err
	}
	// oneAddressEach fails unless every selector of the answer's TSi and TSr spans
	// exactly one address, and returns those addresses.
	oneAddressEach := func(t *testing.T, inner []wire.PayloadEntry) (tsi, tsr []string) {
		t.Helper()
		for i := range inner {
			p, ok := inner[i].Payload.(*wire.PayloadTS)
			if !ok {
				continue
			}
			for _, s := range p.TrafficSelectors {
				if !net.IP(s.StartAddress).Equal(net.IP(s.EndAddress)) {
					t.Errorf("the answer's TS payload %d spans %v-%v; a transport-mode Child SA MUST use exactly one IP address",
						p.TSPayloadType, net.IP(s.StartAddress), net.IP(s.EndAddress))
				}
			}
		}
		return tsAddrs(inner)
	}

	// RFC requirement: RFC7296-2.23.1-1 positive -- "For transport mode, it MUST use
	// exactly one IP address in the TSi and TSr payloads." The peer's rekey of a
	// transport-mode Child SA after an IKE SA rekey, proposing TSi 203.0.113.1/32 and TSr
	// 203.0.113.2/32 to ze behind a NAT, is accepted and installed in transport mode.
	wantPair := pairsText([]tsPair{{I: hostSel(t, natClientPublic), R: hostSel(t, natServerReal)}})
	r, answer, next, err := rekeyWith(t, natClientPublic+"/32")
	if err != nil {
		t.Fatalf("ze refused a one-address transport-mode rekey: %v", err)
	}
	defer next.Clear()
	if next.Mode != modeTransport {
		t.Errorf("the replacement runs mode %d, want transport (%d)", next.Mode, modeTransport)
	}
	oneAddressEach(t, mbDecrypt(t, r.peer, answer))
	if got := pairsText(next.Selectors); got != wantPair {
		t.Errorf("the replacement selectors = %s, want the one host pair %s", got, wantPair)
	}

	// The RFC 7296 Section 2.9.2 floor, not a Section 2.23.1 claim: the same rekey
	// proposing TSi 203.0.113.0/24, which ze's policy covers, is narrowed to the one host
	// pair the retired Child SA held (the rekey floor), so the answer's TSi and TSr each
	// name one address and the replacement installs that pair, never the range.
	r, answer, wide, err := rekeyWith(t, "203.0.113.0/24")
	if err != nil {
		t.Fatalf("ze refused a /24 rekey it can narrow to the retired host pair: %v", err)
	}
	defer wide.Clear()
	tsi, tsr := oneAddressEach(t, mbDecrypt(t, r.peer, answer))
	if !slices.Equal(tsi, []string{natClientPublic}) || !slices.Equal(tsr, []string{natServerReal}) {
		t.Errorf("ze answered TSi %v TSr %v to a /24 proposal, want [%s] [%s]", tsi, tsr, natClientPublic, natServerReal)
	}
	if got := pairsText(wide.Selectors); got != wantPair {
		t.Errorf("a /24 transport-mode rekey installed selectors %s, want the one host pair %s", got, wantPair)
	}
	if wide.Mode != modeTransport {
		t.Errorf("the replacement runs mode %d, want transport (%d)", wide.Mode, modeTransport)
	}
}

// TestTransportChildRekeyAfterIKERekeyRefusesTwoPairAnswer covers AC-4.
//
// Goal: ze's own transport-mode rekey after an IKE SA rekey refuses an answer of two
// pairs, which one transport-mode XFRM state selector cannot hold. Method: the peer's real
// answer to ze's rekey has its TS payloads replaced by two TSi port ranges on the one
// address, both within ze's proposal.
func TestTransportChildRekeyAfterIKERekeyRefusesTwoPairAnswer(t *testing.T) {
	log := slogutil.DiscardLogger()
	r := natRekeySetup(t, true)
	raw, pending, err := initiateChildRekey(r.ze, r.zeChild)
	if err != nil {
		t.Fatalf("ze's initiateChildRekey: %v", err)
	}
	defer pending.clear()
	answer, peerNext, err := respondChildRekey(r.peer, mbDecrypt(t, r.peer, raw), r.peerChild,
		parseMsg(t, raw).Header.MessageID, nil, log)
	if err != nil {
		t.Fatalf("the peer refused ze's rekey: %v", err)
	}
	defer peerNext.Clear()
	twoPairs := replaceTS(mbDecrypt(t, r.ze, answer),
		&wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSi, TrafficSelectors: []wire.TrafficSelector{
			wireSel(t, natServerPublic+"/32", 80, 80, 6),
			wireSel(t, natServerPublic+"/32", 443, 443, 6),
		}},
		&wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSr, TrafficSelectors: []wire.TrafficSelector{
			wireSel(t, natClientReal+"/32", 0, 65535, 6),
		}})
	dp := &mockDP{}
	child, err := applyChildRekeyResponse(r.ze, pending, twoPairs, dp, log)
	if err == nil {
		child.Clear()
		t.Fatal("ze installed a transport-mode replacement from a two-pair answer")
	}
	if !errors.Is(err, errTSTransportPairs) {
		t.Errorf("refusal = %v, want errTSTransportPairs", err)
	}
	if len(dp.sas) != 0 {
		t.Errorf("a refused answer installed %d states, want none", len(dp.sas))
	}
}

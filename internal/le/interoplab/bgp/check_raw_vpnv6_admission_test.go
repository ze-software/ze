// Design: docs/architecture/testing/interop.md -- raw VPNv6 family admission vacuity controls.
// Related: check_raw_vpnv6_admission_oracle.go -- pure wire and FRR JSON predicates under test.
package bgp

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
)

// VALIDATES: raw VPNv6 wire builders produce valid ASN4 UPDATE bodies and
// full-packet frames carrying the exact AFI 2 / SAFI 128 MP_REACH_NLRI, label,
// RD, prefix and IPv4 controls.
// PREVENTS: a malformed fixture body or mismatched length silently masking the
// family-admission boundary.
func TestRawVPNv6AdmissionBuilders(t *testing.T) {
	ze := netip.MustParseAddr("172.30.0.2")
	body := rawVPNv6UpdateBody(rawVPNv6BodyPrefixBytes, rawVPNv6BodyLabel)
	decoded := decodeSpeakerUpdate(body)
	if decoded.decodeErr != nil {
		t.Fatalf("pure VPNv6 body decode error: %v", decoded.decodeErr)
	}
	if len(decoded.attributes) != 3 {
		t.Fatalf("pure VPNv6 body has %d attributes, want 3", len(decoded.attributes))
	}
	if decoded.attributes[2].code != 14 {
		t.Fatalf("third attribute code is %d, want 14 (MP_REACH_NLRI)", decoded.attributes[2].code)
	}
	if len(decoded.attributes[2].value) != 47 {
		t.Fatalf("MP_REACH_NLRI value length is %d, want 47", len(decoded.attributes[2].value))
	}
	packet := rawVPNv6FullPacket(body)
	size, err := validateSpeakerMessageHeader(packet[:bgpHeaderLength])
	if err != nil {
		t.Fatalf("full-packet header error: %v", err)
	}
	if size != len(packet) || packet[18] != bgpUpdate {
		t.Fatalf("full-packet header size=%d len=%d type=%d", size, len(packet), packet[18])
	}
	mixed := rawVPNv6MixedBody(rawVPNv6MixedPrefixBytes, rawVPNv6MixedLabel, 2, ze)
	decodedMixed := decodeSpeakerUpdate(mixed)
	if decodedMixed.decodeErr != nil {
		t.Fatalf("mixed body decode error: %v", decodedMixed.decodeErr)
	}
	if len(decodedMixed.attributes) != 4 {
		t.Fatalf("mixed body has %d attributes, want 4", len(decodedMixed.attributes))
	}
}

// VALIDATES: judgeRawVPNv6Wire accepts exact negotiated and IPv4-only relay
// captures and rejects leaked VPNv6 frames, unstripped mixed UPDATEs, wrong
// OPEN capabilities, notifications, relay rewrites and session resets.
// PREVENTS: FRR's silent ignore of unnegotiated MP_REACH_NLRI from making an
// IPv4-only leak pass vacuously.
func TestRawVPNv6WireOracle(t *testing.T) {
	ze := netip.MustParseAddr("172.31.46.2")
	frr := netip.MustParseAddr("172.31.46.3")
	sink := netip.MustParseAddr("172.31.46.15")

	negFrames := rawVPNv6TestFrames(ze, frr, true, false)
	negText := rawVPNv6TestText(t, negFrames)
	if complete, err := judgeRawVPNv6Wire(negText, ze, frr, true, false); err != nil || !complete {
		t.Fatalf("negotiated Ze stream: complete=%v err=%v", complete, err)
	}
	if complete, err := judgeRawVPNv6Wire(rawVPNv6TestText(t, rawVPNv6TestFrames(ze, frr, true, true)), ze, frr, true, true); err != nil || !complete {
		t.Fatalf("negotiated native stream: complete=%v err=%v", complete, err)
	}

	v4Frames := rawVPNv6TestFrames(ze, sink, false, false)
	v4Text := rawVPNv6TestText(t, v4Frames)
	if complete, err := judgeRawVPNv6Wire(v4Text, ze, sink, false, false); err != nil || !complete {
		t.Fatalf("v4-only Ze stream: complete=%v err=%v", complete, err)
	}
	if complete, err := judgeRawVPNv6Wire(rawVPNv6TestText(t, rawVPNv6TestFrames(ze, sink, false, true)), ze, sink, false, true); err != nil || !complete {
		t.Fatalf("v4-only native stream: complete=%v err=%v", complete, err)
	}

	// Initial sync oracle accepts complete EORs and rejects missing EORs.
	if ready, err := judgeRawVPNv6InitialSync(rawVPNv6TestText(t, negFrames[:4]), ze, frr, true); err != nil || !ready {
		t.Fatalf("negotiated initial sync: ready=%v err=%v", ready, err)
	}
	if ready, err := judgeRawVPNv6InitialSync(rawVPNv6TestText(t, negFrames[:3]), ze, frr, true); err != nil || ready {
		t.Fatalf("negotiated initial sync missing VPNv6 EOR: ready=%v err=%v", ready, err)
	}
	if ready, err := judgeRawVPNv6InitialSync(rawVPNv6TestText(t, v4Frames[:3]), ze, sink, false); err != nil || !ready {
		t.Fatalf("v4-only initial sync: ready=%v err=%v", ready, err)
	}

	// Discrimination: leaked pure VPNv6 UPDATE or unstripped mixed UPDATE on IPv4-only session MUST fail.
	leakedPure := make([][]byte, 0, len(v4Frames)+1)
	leakedPure = append(leakedPure, v4Frames[:3]...)
	leakedPure = append(leakedPure, speakerMessage(bgpUpdate, rawVPNv6UpdateBody(rawVPNv6BodyPrefixBytes, rawVPNv6BodyLabel)))
	leakedPure = append(leakedPure, v4Frames[3:]...)
	if _, err := judgeRawVPNv6Wire(rawVPNv6TestText(t, leakedPure), ze, sink, false, false); err == nil {
		t.Fatal("leaked pure VPNv6 UPDATE on IPv4-only session passed")
	}

	unstrippedMixed := append([][]byte(nil), v4Frames...)
	unstrippedMixed[3] = speakerMessage(bgpUpdate, rawVPNv6MixedBody(rawVPNv6MixedPrefixBytes, rawVPNv6MixedLabel, 2, ze))
	if _, err := judgeRawVPNv6Wire(rawVPNv6TestText(t, unstrippedMixed), ze, sink, false, false); err == nil {
		t.Fatal("unstripped mixed VPNv6+IPv4 UPDATE on IPv4-only session passed")
	}

	v4WithVPNv6EOR := append([][]byte(nil), v4Frames[:3]...)
	v4WithVPNv6EOR = append(v4WithVPNv6EOR, speakerMessage(bgpUpdate, rawVPNv6ExtEORBody))
	v4WithVPNv6EOR = append(v4WithVPNv6EOR, v4Frames[3:]...)
	if _, err := judgeRawVPNv6Wire(rawVPNv6TestText(t, v4WithVPNv6EOR), ze, sink, false, false); err == nil {
		t.Fatal("unexpected VPNv6 EOR on IPv4-only session passed")
	}

	// Wrong OPEN capabilities, session reset, notification, rewrite.
	wrongCap := append([][]byte(nil), v4Frames...)
	wrongCap[0] = rawVPNv6TestOpen(ze, rawVPNv6LocalASN, true)
	if _, err := judgeRawVPNv6Wire(rawVPNv6TestText(t, wrongCap), ze, sink, false, false); err == nil {
		t.Fatal("IPv4-only session with VPNv6 capability in OPEN passed")
	}
	missingCap := append([][]byte(nil), negFrames...)
	missingCap[0] = rawVPNv6TestOpen(ze, rawVPNv6LocalASN, false)
	if _, err := judgeRawVPNv6Wire(rawVPNv6TestText(t, missingCap), ze, frr, true, false); err == nil {
		t.Fatal("negotiated session missing VPNv6 capability in OPEN passed")
	}
	notif := append(append([][]byte(nil), v4Frames[:3]...), speakerMessage(bgpNotification, []byte{6, 0}))
	if _, err := judgeRawVPNv6Wire(rawVPNv6TestText(t, notif), ze, sink, false, false); err == nil {
		t.Fatal("NOTIFICATION frame passed")
	}
	rewritten := strings.Replace(v4Text, `"original":`, `"delivered":"00","original":`, 1)
	if _, err := judgeRawVPNv6Wire(rewritten, ze, sink, false, false); err == nil {
		t.Fatal("relay rewrite passed")
	}
}

// VALIDATES: FRR neighbor capability, VPNv6 detail, VPNv6 RD table and IPv4
// unicast detail predicates accept exact FRR 10.7.1 JSON documents and refuse
// wrong RD, wrong label, wrong next hop, missing capability, or non-empty table.
// PREVENTS: silent acceptance of unrelated or malformed FRR JSON answers.
func TestRawVPNv6FRRPredicates(t *testing.T) {
	negNeighbor := `{"172.30.0.10":{"neighborCapabilities":{"multiprotocolExtensions":{"ipv4Unicast":{"advertisedAndReceived":true},"ipv6Vpn":{"advertisedAndReceived":true}}}}}`
	v4Neighbor := `{"172.30.0.11":{"neighborCapabilities":{"multiprotocolExtensions":{"ipv4Unicast":{"advertisedAndReceived":true}}}}}`
	if err := requireRawVPNv6NeighborCapabilities(negNeighbor, "172.30.0.10", true); err != nil {
		t.Fatalf("negotiated neighbor capabilities: %v", err)
	}
	if err := requireRawVPNv6NeighborCapabilities(v4Neighbor, "172.30.0.11", false); err != nil {
		t.Fatalf("v4-only neighbor capabilities: %v", err)
	}
	if err := requireRawVPNv6NeighborCapabilities(v4Neighbor, "172.30.0.11", true); err == nil {
		t.Fatal("missing ipv6Vpn capability passed as negotiated")
	}
	if err := requireRawVPNv6NeighborCapabilities(negNeighbor, "172.30.0.10", false); err == nil {
		t.Fatal("present ipv6Vpn capability passed as IPv4-only")
	}

	vpnDetail := `{"65001:100":{"prefix":"2001:db8:4659::/48","paths":[{"peer":{"peerId":"172.30.0.10"},"aspath":{"string":"65001"},"remoteLabel":1000,"nexthops":[{"ip":"2001:db8:ffff::1","scope":"global"}]}]}}`
	wantNH := netip.MustParseAddr(rawVPNv6NextHop)
	if err := requireRawVPNv6FRRRoute(vpnDetail, rawVPNv6RD, rawVPNv6BodyPrefix, "172.30.0.10", 1000, wantNH); err != nil {
		t.Fatalf("valid VPNv6 detail: %v", err)
	}
	for _, bad := range []string{
		`{}`,
		strings.Replace(vpnDetail, `"65001:100"`, `"65001:200"`, 1),
		strings.Replace(vpnDetail, `"remoteLabel":1000`, `"remoteLabel":1001`, 1),
		strings.Replace(vpnDetail, `"2001:db8:ffff::1"`, `"2001:db8:ffff::2"`, 1),
		strings.Replace(vpnDetail, `"172.30.0.10"`, `"172.30.0.11"`, 1),
		strings.Replace(vpnDetail, `"65001"`, `"65009"`, 1),
	} {
		if err := requireRawVPNv6FRRRoute(bad, rawVPNv6RD, rawVPNv6BodyPrefix, "172.30.0.10", 1000, wantNH); err == nil {
			t.Fatalf("invalid VPNv6 detail passed: %s", bad)
		}
	}

	vpnTable := `{"totalRoutes":2,"totalPaths":2,"routes":{"routeDistinguishers":{"65001:100":{"2001:db8:4659::/48":[{"peerId":"172.30.0.10","path":"65001"}],"2001:db8:465a::/48":[{"peerId":"172.30.0.10","path":"65001"}]}}}}`
	if err := requireRawVPNv6FRRTable(vpnTable, rawVPNv6RD, "172.30.0.10", []string{rawVPNv6BodyPrefix, rawVPNv6FullPrefix}); err != nil {
		t.Fatalf("valid VPNv6 table: %v", err)
	}
	if err := requireRawVPNv6FRRTable(`{}`, rawVPNv6RD, "172.30.0.11", nil); err != nil {
		t.Fatalf("empty VPNv6 table: %v", err)
	}
	if err := requireRawVPNv6FRRTable(vpnTable, rawVPNv6RD, "172.30.0.11", nil); err == nil {
		t.Fatal("populated VPNv6 table passed as empty")
	}

	ze := netip.MustParseAddr("172.30.0.2")
	ipv4Detail := `{"prefix":"10.46.255.0/24","paths":[{"valid":true,"peer":{"peerId":"172.30.0.11"},"aspath":{"string":"65001"},"nexthops":[{"ip":"172.30.0.2"}]}]}`
	if err := requireRawVPNv6IPv4Route(ipv4Detail, rawVPNv6V4OnlyFence, "172.30.0.11", ze); err != nil {
		t.Fatalf("valid IPv4 detail: %v", err)
	}
	for _, bad := range []string{
		`{}`,
		strings.Replace(ipv4Detail, `"valid":true`, `"valid":false`, 1),
		strings.Replace(ipv4Detail, `"172.30.0.2"`, `"172.30.0.9"`, 1),
		strings.Replace(ipv4Detail, `"172.30.0.11"`, `"172.30.0.10"`, 1),
	} {
		if err := requireRawVPNv6IPv4Route(bad, rawVPNv6V4OnlyFence, "172.30.0.11", ze); err == nil {
			t.Fatalf("invalid IPv4 detail passed: %s", bad)
		}
	}
}

func rawVPNv6TestFrames(ze, native netip.Addr, negotiatedVPNv6, fromNative bool) [][]byte {
	address, asn := ze, rawVPNv6LocalASN
	if fromNative {
		address, asn = native, rawVPNv6RemoteASN
	}
	frames := [][]byte{
		rawVPNv6TestOpen(address, asn, negotiatedVPNv6),
		speakerMessage(bgpKeepalive, nil),
	}
	if fromNative {
		return frames
	}
	frames = append(frames, speakerMessage(bgpUpdate, rawVPNv6IPv4EORBody))
	if negotiatedVPNv6 {
		frames = append(frames, speakerMessage(bgpUpdate, rawVPNv6ExtEORBody))
	}
	for _, body := range rawVPNv6ExpectedUpdates(ze, negotiatedVPNv6) {
		frames = append(frames, speakerMessage(bgpUpdate, body))
	}
	return frames
}

func rawVPNv6TestOpen(address netip.Addr, asn uint16, vpnv6 bool) []byte {
	router := address.As4()
	params := []byte{
		2, 6, 1, 4, 0, 1, 0, 1, // MP ipv4/unicast
		2, 6, 65, 4, 0, 0, byte(asn >> 8), byte(asn), // ASN4
	}
	if vpnv6 {
		params = append(params, 2, 6, 1, 4, 0, 2, 0, 128) // MP ipv6/mpls-vpn
	}
	open := []byte{4, byte(asn >> 8), byte(asn), 0, 90}
	open = append(open, router[:]...)
	open = append(open, byte(len(params)))
	open = append(open, params...)
	return speakerMessage(bgpOpen, open)
}

func rawVPNv6TestText(t *testing.T, frames [][]byte) string {
	t.Helper()
	var text bytes.Buffer
	encoder := json.NewEncoder(&text)
	for _, frame := range frames {
		if err := encoder.Encode(extendedRelayFrame{Original: hex.EncodeToString(frame)}); err != nil {
			t.Fatal(err)
		}
	}
	return text.String()
}

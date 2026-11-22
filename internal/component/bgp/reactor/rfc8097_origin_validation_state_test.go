// Design: docs/architecture/wire/attributes.md -- RFC 8097 reactor receive-path conformance tests
// Related: session_origin_validation.go -- Session.sanitizeOriginValidationState
// Related: session_problem_diagnostic.go -- codeDoctorBGPOriginValidationStateBad
// Related: session_read.go -- processValidatedMessage
// Related: config.go -- PeersFromTree
// RFC 8097 Section 2 -- see rfc/short/rfc8097.md

package reactor

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func rfc8097ErrorLines(output string) []string {
	var matching []string
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.Contains(line, "level=ERROR") {
			continue
		}
		if !strings.Contains(line, "code="+codeDoctorBGPOriginValidationStateBad) {
			continue
		}
		matching = append(matching, line)
	}
	return matching
}

func buildRFC8097TestUpdateFrame(isIBGP bool, extComms []attribute.ExtendedCommunity) []byte {
	var attrs []byte
	// ORIGIN = IGP
	attrs = append(attrs, 0x40, byte(attribute.AttrOrigin), 1, 0)
	// AS_PATH: empty for iBGP, [65001] (4-byte) for eBGP (matches PeerAS=65001).
	if isIBGP {
		attrs = append(attrs, 0x40, byte(attribute.AttrASPath), 0)
	} else {
		attrs = append(attrs, 0x40, byte(attribute.AttrASPath), 6, byte(attribute.ASSequence), 1, 0, 0, 0xfd, 0xe9)
	}
	// NEXT_HOP = 192.0.2.10
	attrs = append(attrs, 0x40, byte(attribute.AttrNextHop), 4, 192, 0, 2, 10)
	if isIBGP {
		// LOCAL_PREF = 100
		attrs = append(attrs, 0x40, byte(attribute.AttrLocalPref), 4, 0, 0, 0, 100)
	}
	if len(extComms) > 0 {
		valLen := len(extComms) * 8
		attrs = append(attrs, 0xc0, byte(attribute.AttrExtCommunity), byte(valLen))
		for _, ec := range extComms {
			attrs = append(attrs, ec[:]...)
		}
	}

	nlri := []byte{24, 198, 51, 100} // 198.51.100.0/24
	body := make([]byte, 4+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs)))
	copy(body[4:], attrs)
	copy(body[4+len(attrs):], nlri)

	frame := make([]byte, message.HeaderLen+len(body))
	for i := range 16 {
		frame[i] = 0xff
	}
	binary.BigEndian.PutUint16(frame[16:18], uint16(len(frame)))
	frame[18] = byte(msgtype.TypeUPDATE)
	copy(frame[message.HeaderLen:], body)
	return frame
}

func rfc8097BGPConfigTree(remoteAS string, acceptLeaf *string) map[string]any {
	sessionCfg := map[string]any{
		"asn": map[string]any{"remote": remoteAS},
	}
	if acceptLeaf != nil {
		sessionCfg["accept-origin-validation-state"] = *acceptLeaf
	}
	return map[string]any{
		"router-id": "1.2.3.1",
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"peer": map[string]any{
			"p1": map[string]any{
				"connection": map[string]any{
					"remote": map[string]any{"ip": "192.0.2.10"},
					"local":  map[string]any{"ip": "auto"},
				},
				"session": sessionCfg,
			},
		},
	}
}

func setupRFC8097SessionFromSettings(t *testing.T, settings *PeerSettings) (*Session, *recordingConn, *[]*wireu.WireUpdate) {
	t.Helper()
	s := NewSession(settings)
	conn := &recordingConn{}
	s.conn = conn
	s.bufWriter = bufio.NewWriter(conn)
	s.nextHopScope.Store(&receiveNextHopScope{
		addresses: []netip.Prefix{netip.MustParsePrefix("192.0.2.1/24")},
		local:     netip.MustParseAddr("192.0.2.1"),
		remote:    netip.MustParseAddr("192.0.2.10"),
		direct:    true,
	})

	require.NoError(t, s.fsm.Event(fsm.EventManualStart))
	require.NoError(t, s.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, s.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, s.fsm.Event(fsm.EventKeepaliveMsg))
	require.Equal(t, fsm.StateEstablished, s.fsm.State())

	localCaps := []capability.Capability{
		&capability.Multiprotocol{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast},
		&capability.ASN4{ASN: settings.LocalAS},
	}
	peerCaps := []capability.Capability{
		&capability.Multiprotocol{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast},
		&capability.ASN4{ASN: settings.PeerAS},
	}
	s.proposal = preparedOpenForTest(s, &message.Open{Version: 4, MyAS: uint16(settings.LocalAS), HoldTime: 90, BGPIdentifier: 1})
	s.peerOpen = &message.Open{Version: 4, MyAS: uint16(settings.PeerAS), HoldTime: 90, BGPIdentifier: 2}
	s.negotiateWith(localCaps, peerCaps)
	require.NotNil(t, s.negotiated)

	recvCtx := bgpctx.NewEncodingContext(s.negotiated.Identity, s.negotiated.Encoding, bgpctx.DirectionRecv)
	ctxID, err := bgpctx.Registry.Register(recvCtx)
	require.NoError(t, err)
	s.setRecvCtxID(ctxID)

	var delivered []*wireu.WireUpdate
	s.onMessageReceived = func(_ *Session, _ netip.Addr, typ msgtype.MessageType, _ []byte, wu *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection, _ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		if typ == msgtype.TypeUPDATE && wu != nil {
			copied := wireu.NewWireUpdate(append([]byte(nil), wu.Payload()...), wu.SourceCtxID())
			delivered = append(delivered, copied)
		}
		return false
	}
	return s, conn, &delivered
}

func setupRFC8097DefaultSession(t *testing.T, isIBGP bool) (*Session, *recordingConn, *[]*wireu.WireUpdate) {
	t.Helper()
	peerAS := uint32(65001)
	if isIBGP {
		peerAS = 65000
	}
	// Do NOT overwrite AcceptOriginValidationState here: leave NewPeerSettings default untouched.
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.10"), 65000, peerAS, 0x01020301)
	return setupRFC8097SessionFromSettings(t, settings)
}

func extractDeliveredExtComms(t *testing.T, wu *wireu.WireUpdate) (attribute.ExtendedCommunities, bool) {
	t.Helper()
	sections, err := wire.ParseUpdateSections(wu.Payload())
	require.NoError(t, err)
	_, _, raw, found := attribute.AttrFind(sections.Attrs(wu.Payload()), attribute.AttrExtCommunity)
	if !found {
		return nil, false
	}
	parsed, err := attribute.ParseExtendedCommunities(raw)
	require.NoError(t, err)
	return parsed, true
}

// TestRFC8097ReservedFieldIgnoredOnReceive proves RFC 8097 Section 2 through
// the real BGP session receive path (Session.ReadAndProcess -> processValidatedMessage
// -> Session.sanitizeOriginValidationState): the 5-octet Reserved field [2..6]
// MUST be ignored upon receipt of the community.
//
// RFC requirement: RFC8097-2-2 positive -- on an Established iBGP session, wire UPDATEs carrying 0x43/0x00 with non-zero bytes in each of the 5 Reserved positions [2..6] (for all valid states 0, 1, 2) are accepted through Session.ReadAndProcess with zero NOTIFICATIONs, zero RFC 8097 error logs, intact NLRI, and delivered canonical 0x43/0x00 carrying the exact same validation state as zero-Reserved input.
// RFC requirement: RFC8097-2-2 negative -- ONLY the Reserved field [2..6] is ignored on receipt: changing the validation state octet [7] (0 vs 1 vs 2) changes the delivered validation state, and changing sub-type octet [1] (0x43/0x01) is NOT treated as an RFC 8097 community and preserves its octets [2..6] verbatim.
func TestRFC8097ReservedFieldIgnoredOnReceive(t *testing.T) {
	logBuf := captureSessionLog(t, defaultSessionLogLevel)
	s, conn, delivered := setupRFC8097DefaultSession(t, true)

	// Positive: non-zero bytes in every Reserved octet [2..6] across all three valid states (0, 1, 2)
	// are ignored on receipt and yield the exact validation state with canonical zero Reserved octets.
	reservedPatterns := [][5]byte{
		{0x01, 0x00, 0x00, 0x00, 0x00},
		{0x00, 0x02, 0x00, 0x00, 0x00},
		{0x00, 0x00, 0x04, 0x00, 0x00},
		{0x00, 0x00, 0x00, 0x08, 0x00},
		{0x00, 0x00, 0x00, 0x00, 0x10},
		{0xaa, 0xbb, 0xcc, 0xdd, 0xee},
	}
	for _, state := range []uint8{0, 1, 2} {
		for _, pat := range reservedPatterns {
			ec := attribute.ExtendedCommunity{0x43, 0x00, pat[0], pat[1], pat[2], pat[3], pat[4], state}
			frame := buildRFC8097TestUpdateFrame(true, []attribute.ExtendedCommunity{ec})
			s.bufReader = bufio.NewReader(bytes.NewReader(frame))
			require.NoError(t, s.ReadAndProcess())
			require.Equal(t, fsm.StateEstablished, s.fsm.State())
			require.Empty(t, conn.written())

			got := (*delivered)[len(*delivered)-1]
			nlri, err := got.NLRI()
			require.NoError(t, err)
			require.Equal(t, []byte{24, 198, 51, 100}, nlri)

			comms, found := extractDeliveredExtComms(t, got)
			require.True(t, found)
			require.Equal(t, attribute.ExtendedCommunities{
				{0x43, 0x00, 0, 0, 0, 0, 0, state},
			}, comms)
			decoded, ok := attribute.DecodeOriginValidationStateExtCommunity(ec)
			require.True(t, ok)
			require.Equal(t, attribute.OriginValidationState(state), decoded)
		}
	}
	require.Empty(t, rfc8097ErrorLines(logBuf.String()), "non-zero Reserved octets on valid states must emit zero errors")

	// Negative 1: validation state octet [7] is NOT ignored — distinct state bytes produce distinct delivered values.
	var statesSeen []uint8
	for _, state := range []uint8{0, 1, 2} {
		ec := attribute.ExtendedCommunity{0x43, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, state}
		s.bufReader = bufio.NewReader(bytes.NewReader(buildRFC8097TestUpdateFrame(true, []attribute.ExtendedCommunity{ec})))
		require.NoError(t, s.ReadAndProcess())
		comms, found := extractDeliveredExtComms(t, (*delivered)[len(*delivered)-1])
		require.True(t, found)
		require.Len(t, comms, 1)
		statesSeen = append(statesSeen, comms[0][7])
	}
	require.Equal(t, []uint8{0, 1, 2}, statesSeen)

	// Negative 2: non-0x43/0x00 community (sub-type 0x01) is NOT treated as RFC 8097;
	// its octets [2..6] are preserved verbatim on receipt.
	otherOpaque := attribute.ExtendedCommunity{0x43, 0x01, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01}
	s.bufReader = bufio.NewReader(bytes.NewReader(buildRFC8097TestUpdateFrame(true, []attribute.ExtendedCommunity{otherOpaque})))
	require.NoError(t, s.ReadAndProcess())
	commsOther, foundOther := extractDeliveredExtComms(t, (*delivered)[len(*delivered)-1])
	require.True(t, foundOther)
	require.Equal(t, attribute.ExtendedCommunities{otherOpaque}, commsOther)
}

// TestRFC8097MultipleInstancesSelectGreatest proves RFC 8097 Section 2 through
// the real BGP session receive path (Session.ReadAndProcess): when more than
// one instance is received, an implementation MUST disregard all instances
// other than the one with the numerically greatest validation state value.
//
// RFC requirement: RFC8097-2-3 positive -- on an Established iBGP session, receiving UPDATEs with multiple valid 0x43/0x00 instances across all orderings ([0,2,1], [2,0,1], [0,1], [1,0], [1,2], [2,1], [2,2]) delivers the sibling RT plus exactly one 0x43/0x00 carrying the numerically greatest valid state (2 > 1 > 0) with zero error logs.
// RFC requirement: RFC8097-2-3 negative -- when erroneous instances (> 2) appear alongside valid instances ([0,2,5], [0,1,5]) or alone ([4,7]), the erroneous instances are discarded and logged rather than winning numerically over valid states, keeping only the greatest valid state (2 or 1) or stripping all 0x43/0x00 while preserving sibling RT and NLRI.
func TestRFC8097MultipleInstancesSelectGreatest(t *testing.T) {
	rt := attribute.ExtendedCommunity{0x00, 0x02, 0xfd, 0xe8, 0, 0, 0, 1}
	ov0 := attribute.ExtendedCommunity{0x43, 0x00, 0, 0, 0, 0, 0, 0}
	ov1 := attribute.ExtendedCommunity{0x43, 0x00, 0x11, 0x22, 0, 0, 0, 1}
	ov2 := attribute.ExtendedCommunity{0x43, 0x00, 0, 0, 0x33, 0, 0, 2}
	ov4 := attribute.ExtendedCommunity{0x43, 0x00, 0, 0, 0, 0, 0, 4}
	ov5 := attribute.ExtendedCommunity{0x43, 0x00, 0, 0, 0, 0, 0, 5}
	ov7 := attribute.ExtendedCommunity{0x43, 0x00, 0, 0, 0, 0, 0, 7}

	logBuf := captureSessionLog(t, defaultSessionLogLevel)
	s, conn, delivered := setupRFC8097DefaultSession(t, true)

	cases := []struct {
		name      string
		in        []attribute.ExtendedCommunity
		wantFound bool
		wantState uint8
		wantErrs  int
	}{
		{"0_then_2_then_1", []attribute.ExtendedCommunity{rt, ov0, ov2, ov1}, true, 2, 0},
		{"2_then_0_then_1", []attribute.ExtendedCommunity{rt, ov2, ov0, ov1}, true, 2, 0},
		{"0_then_1", []attribute.ExtendedCommunity{rt, ov0, ov1}, true, 1, 0},
		{"1_then_0", []attribute.ExtendedCommunity{rt, ov1, ov0}, true, 1, 0},
		{"1_then_2", []attribute.ExtendedCommunity{rt, ov1, ov2}, true, 2, 0},
		{"2_then_1", []attribute.ExtendedCommunity{rt, ov2, ov1}, true, 2, 0},
		{"duplicate_2_and_2", []attribute.ExtendedCommunity{rt, ov2, ov2}, true, 2, 0},
		{"valid_0_2_and_bogus_5_keeps_2", []attribute.ExtendedCommunity{rt, ov0, ov2, ov5}, true, 2, 1},
		{"valid_0_1_and_bogus_5_keeps_1", []attribute.ExtendedCommunity{rt, ov0, ov1, ov5}, true, 1, 1},
		{"multiple_bogus_4_and_7_yield_none", []attribute.ExtendedCommunity{rt, ov4, ov7}, false, 0, 2},
	}

	totalErrs := 0
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frame := buildRFC8097TestUpdateFrame(true, tc.in)
			s.bufReader = bufio.NewReader(bytes.NewReader(frame))
			require.NoError(t, s.ReadAndProcess())
			require.Equal(t, fsm.StateEstablished, s.fsm.State())
			require.Empty(t, conn.written())

			got := (*delivered)[len(*delivered)-1]
			nlri, err := got.NLRI()
			require.NoError(t, err)
			require.Equal(t, []byte{24, 198, 51, 100}, nlri)

			comms, found := extractDeliveredExtComms(t, got)
			require.True(t, found, "sibling RT must survive")
			if tc.wantFound {
				require.Equal(t, attribute.ExtendedCommunities{
					rt,
					{0x43, 0x00, 0, 0, 0, 0, 0, tc.wantState},
				}, comms)
			} else {
				require.Equal(t, attribute.ExtendedCommunities{rt}, comms)
			}
			totalErrs += tc.wantErrs
			require.Len(t, rfc8097ErrorLines(logBuf.String()), totalErrs)
		})
	}
}

// TestRFC8097EBGPDropsOriginValidationByDefault proves RFC 8097 Section 2:
// by default, implementations MUST drop the origin validation state extended
// community if received from an External BGP (EBGP) peer without processing it
// further, while iBGP peers and explicitly configured EBGP peers retain and
// process it.
//
// RFC requirement: RFC8097-2-4 positive -- both NewPeerSettings (untouched) and PeersFromTree (without accept-origin-validation-state) default AcceptOriginValidationState to false; on that default EBGP session, Session.ReadAndProcess drops every 0x43/0x00 community before publication (preserving sibling RT and removing the attribute cleanly when sole), keeps NLRI and Established session, and does not process or log even an invalid >2 value from that EBGP peer.
// RFC requirement: RFC8097-2-4 negative -- an iBGP session parsed via PeersFromTree without the leaf AND an EBGP session parsed via PeersFromTree with session/accept-origin-validation-state=true retain and deliver valid 0x43/0x00 through Session.ReadAndProcess.
func TestRFC8097EBGPDropsOriginValidationByDefault(t *testing.T) {
	rt := attribute.ExtendedCommunity{0x00, 0x02, 0xfd, 0xe9, 0, 0, 0, 7} // target:65001:7
	ovInvalid := attribute.ExtendedCommunity{0x43, 0x00, 0, 0, 0, 0, 0, 2}
	ovBogus := attribute.ExtendedCommunity{0x43, 0x00, 0, 0, 0, 0, 0, 9}

	logBuf := captureSessionLog(t, defaultSessionLogLevel)

	// Positive 1: NewPeerSettings default (untouched — never overwritten by test helper) must be false.
	rawDefault := NewPeerSettings(netip.MustParseAddr("192.0.2.10"), 65000, 65001, 0x01020301)
	require.False(t, rawDefault.AcceptOriginValidationState, "NewPeerSettings must default AcceptOriginValidationState to false")

	// Positive 2: Real config parsing via PeersFromTree without accept-origin-validation-state
	// defaults AcceptOriginValidationState to false and drops 0x43/0x00 from EBGP without logging even state=9.
	ebgpPeers, err := PeersFromTree(rfc8097BGPConfigTree("65001", nil))
	require.NoError(t, err)
	require.Len(t, ebgpPeers, 1)
	require.False(t, ebgpPeers[0].AcceptOriginValidationState, "PeersFromTree must default AcceptOriginValidationState to false")

	sEBGP, connEBGP, delEBGP := setupRFC8097SessionFromSettings(t, ebgpPeers[0])
	frame := buildRFC8097TestUpdateFrame(false, []attribute.ExtendedCommunity{rt, ovInvalid, ovBogus})
	sEBGP.bufReader = bufio.NewReader(bytes.NewReader(frame))
	require.NoError(t, sEBGP.ReadAndProcess())
	require.Equal(t, fsm.StateEstablished, sEBGP.fsm.State())
	require.Empty(t, connEBGP.written())
	require.Len(t, *delEBGP, 1)
	comms, found := extractDeliveredExtComms(t, (*delEBGP)[0])
	require.True(t, found)
	require.Equal(t, attribute.ExtendedCommunities{rt}, comms, "EBGP must drop 0x43/0x00 and preserve sibling RT")
	require.Empty(t, rfc8097ErrorLines(logBuf.String()), "EBGP default drop must not process/log dropped 0x43/0x00")

	// Positive 3: EBGP default when 0x43/0x00 is the ONLY extended community strips the attribute cleanly while keeping NLRI.
	frameSole := buildRFC8097TestUpdateFrame(false, []attribute.ExtendedCommunity{ovInvalid})
	sEBGP.bufReader = bufio.NewReader(bytes.NewReader(frameSole))
	require.NoError(t, sEBGP.ReadAndProcess())
	require.Len(t, *delEBGP, 2)
	_, foundSole := extractDeliveredExtComms(t, (*delEBGP)[1])
	require.False(t, foundSole, "when 0x43/0x00 is the sole extended community, the attribute must be removed")
	nlri, err := (*delEBGP)[1].NLRI()
	require.NoError(t, err)
	require.Equal(t, []byte{24, 198, 51, 100}, nlri, "route NLRI must survive intact")

	// Negative 1: iBGP session parsed via PeersFromTree without the leaf retains valid 0x43/0x00 by default.
	ibgpPeers, err := PeersFromTree(rfc8097BGPConfigTree("65000", nil))
	require.NoError(t, err)
	require.Len(t, ibgpPeers, 1)
	sIBGP, connIBGP, delIBGP := setupRFC8097SessionFromSettings(t, ibgpPeers[0])
	frameIBGP := buildRFC8097TestUpdateFrame(true, []attribute.ExtendedCommunity{rt, ovInvalid})
	sIBGP.bufReader = bufio.NewReader(bytes.NewReader(frameIBGP))
	require.NoError(t, sIBGP.ReadAndProcess())
	require.Empty(t, connIBGP.written())
	require.Len(t, *delIBGP, 1)
	commsIBGP, foundIBGP := extractDeliveredExtComms(t, (*delIBGP)[0])
	require.True(t, foundIBGP)
	require.Equal(t, attribute.ExtendedCommunities{rt, ovInvalid}, commsIBGP)

	// Negative 2: EBGP session parsed via PeersFromTree with accept-origin-validation-state="true" retains valid 0x43/0x00.
	trueStr := "true"
	overridePeers, err := PeersFromTree(rfc8097BGPConfigTree("65001", &trueStr))
	require.NoError(t, err)
	require.Len(t, overridePeers, 1)
	require.True(t, overridePeers[0].AcceptOriginValidationState)
	sEBGPOverride, _, delOverride := setupRFC8097SessionFromSettings(t, overridePeers[0])
	frameOverride := buildRFC8097TestUpdateFrame(false, []attribute.ExtendedCommunity{rt, ovInvalid})
	sEBGPOverride.bufReader = bufio.NewReader(bytes.NewReader(frameOverride))
	require.NoError(t, sEBGPOverride.ReadAndProcess())
	require.Len(t, *delOverride, 1)
	commsOverride, foundOverride := extractDeliveredExtComms(t, (*delOverride)[0])
	require.True(t, foundOverride)
	require.Equal(t, attribute.ExtendedCommunities{rt, ovInvalid}, commsOverride)
}

// TestRFC8097ValueAboveTwoDiscardedAndLogged proves RFC 8097 Section 2:
// an origin validation state value > 2 MUST be discarded (attribute-discard
// strategy per RFC 7606) and logged for further analysis, while keeping the
// route, other extended communities, valid RFC 8097 instances, and the session.
//
// RFC requirement: RFC8097-2-5 positive -- on an iBGP session, receiving 0x43/0x00 with value 3 (exact lower invalid boundary) or 255 discards the erroneous community (leaving the route, sibling RT, and session Established with no NOTIFICATION) and logs an Error at the default session log level carrying registered doctor code doctor-bgp-origin-validation-state-invalid.
// RFC requirement: RFC8097-2-5 negative -- receiving each valid 0x43/0x00 state individually — 0 (valid), 1 (not-found), and 2 (invalid, exact upper valid boundary) — keeps the community intact and emits zero doctor-bgp-origin-validation-state-invalid error logs.
func TestRFC8097ValueAboveTwoDiscardedAndLogged(t *testing.T) {
	rt := attribute.ExtendedCommunity{0x00, 0x02, 0xfd, 0xe8, 0, 0, 0, 42}

	logBuf := captureSessionLog(t, defaultSessionLogLevel)
	s, conn, delivered := setupRFC8097DefaultSession(t, true)

	// Positive 1: value 3 (exact boundary above max 2) alongside sibling RT -> discarded & logged, RT + route preserved, no NOTIFICATION.
	ov3 := attribute.ExtendedCommunity{0x43, 0x00, 0, 0, 0, 0, 0, 3}
	frame3 := buildRFC8097TestUpdateFrame(true, []attribute.ExtendedCommunity{rt, ov3})
	s.bufReader = bufio.NewReader(bytes.NewReader(frame3))
	require.NoError(t, s.ReadAndProcess())
	require.Equal(t, fsm.StateEstablished, s.fsm.State())
	require.Empty(t, conn.written(), "attribute discard must not send NOTIFICATION")
	require.Len(t, *delivered, 1)
	comms3, found3 := extractDeliveredExtComms(t, (*delivered)[0])
	require.True(t, found3)
	require.Equal(t, attribute.ExtendedCommunities{rt}, comms3, "erroneous state=3 must be discarded while sibling RT survives")
	nlri3, err := (*delivered)[0].NLRI()
	require.NoError(t, err)
	require.Equal(t, []byte{24, 198, 51, 100}, nlri3)
	require.Len(t, rfc8097ErrorLines(logBuf.String()), 1)

	// Positive 2: sole community value 255 -> entire EXTENDED_COMMUNITIES attribute removed, route survives, logged again.
	ov255 := attribute.ExtendedCommunity{0x43, 0x00, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 255}
	frame255 := buildRFC8097TestUpdateFrame(true, []attribute.ExtendedCommunity{ov255})
	s.bufReader = bufio.NewReader(bytes.NewReader(frame255))
	require.NoError(t, s.ReadAndProcess())
	require.Equal(t, fsm.StateEstablished, s.fsm.State())
	require.Empty(t, conn.written())
	require.Len(t, *delivered, 2)
	_, found255 := extractDeliveredExtComms(t, (*delivered)[1])
	require.False(t, found255)
	nlri255, err := (*delivered)[1].NLRI()
	require.NoError(t, err)
	require.Equal(t, []byte{24, 198, 51, 100}, nlri255)
	require.Len(t, rfc8097ErrorLines(logBuf.String()), 2)

	// Negative: EVERY valid state 0 (valid), 1 (not-found), AND 2 (invalid — exact upper valid boundary)
	// is retained individually and emits zero additional error logs.
	beforeLogs := len(rfc8097ErrorLines(logBuf.String()))
	for _, validState := range []uint8{0, 1, 2} {
		ovValid := attribute.ExtendedCommunity{0x43, 0x00, 0x12, 0x34, 0x56, 0x78, 0x9a, validState}
		frameValid := buildRFC8097TestUpdateFrame(true, []attribute.ExtendedCommunity{rt, ovValid})
		s.bufReader = bufio.NewReader(bytes.NewReader(frameValid))
		require.NoError(t, s.ReadAndProcess())
		require.Equal(t, fsm.StateEstablished, s.fsm.State())
		require.Empty(t, conn.written())

		got := (*delivered)[len(*delivered)-1]
		commsValid, foundValid := extractDeliveredExtComms(t, got)
		require.True(t, foundValid)
		require.Equal(t, attribute.ExtendedCommunities{
			rt,
			{0x43, 0x00, 0, 0, 0, 0, 0, validState},
		}, commsValid, "valid state %d must be retained", validState)
	}
	require.Len(t, rfc8097ErrorLines(logBuf.String()), beforeLogs, "valid states 0, 1, and 2 must produce zero error logs")
}

package l2tp

import (
	"bytes"
	"log/slog"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/callsink"
)

// fakeRealmHandoff records what the sink reports to the PPPoE side.
type fakeRealmHandoff struct {
	mu      sync.Mutex
	ended   []string
	bridged []error
}

func (h *fakeRealmHandoff) Quiesce() error { return nil }

func (h *fakeRealmHandoff) Bridged(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.bridged = append(h.bridged, err)
}

func (h *fakeRealmHandoff) Ended(reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ended = append(h.ended, reason)
}

// testAVP is one AVP of a received message, with its flags.
type testAVP struct {
	flags AVPFlags
	value []byte
}

// avpsOf returns the AVPs of a control message body in wire order.
func avpsOf(t *testing.T, body []byte) ([]AVPType, map[AVPType]testAVP) {
	t.Helper()
	order := []AVPType{}
	byType := map[AVPType]testAVP{}
	it := NewAVPIterator(body)
	for {
		_, attrType, flags, value, ok := it.Next()
		if !ok {
			break
		}
		order = append(order, attrType)
		byType[attrType] = testAVP{flags: flags, value: append([]byte(nil), value...)}
	}
	require.NoError(t, it.Err())
	return order, byType
}

// TestRelaySinkRealmMatchPlacesCallWithProxyLCP is the wiring test of a realm
// relay (spec-home-cpe-14 AC-1, AC-2).
//
// VALIDATES: a `relay-realm example.net { remote lns-a }` binding and a CHAP
// subscriber cpe@example.net make RelayRealm dial lns-a, place an incoming
// call, and send an ICCN that carries AVPs 26 to 28 (the captured LCP option
// bodies) and 29 (type 2), 30, 31, 32 (high octet 0) and 33, each with M=0.
// Method: a UDP client plays the LNS through SCCRP and ICRP, then decodes the
// ICCN ze sends.
// PREVENTS: a realm call that reaches the LNS without the proxy data, or with
// the M bit set on an AVP the LNS may not know.
func TestRelaySinkRealmMatchPlacesCallWithProxyLCP(t *testing.T) {
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()
	timer := newTunnelTimer(r.tickCh, r.updateCh)
	require.NoError(t, timer.Start())
	defer timer.Stop()
	client := newClient(t, ln)
	defer client.Close()

	s := &Subsystem{
		params: Parameters{
			Remotes:     []Remote{{Name: "lns-a", Address: clientAddrPort(t, client)}},
			RelayRealms: []RealmRelayBinding{{Realm: "example.net", Remote: "lns-a"}},
		},
		reactors: []*l2tpReactor{r},
		logger:   slog.Default(),
	}
	lcp := callsink.ProxyLCP{
		InitialReceived: []byte{0x01, 0x04, 0x05, 0xd4, 0x05, 0x06, 0x11, 0x22, 0x33, 0x44},
		LastSent:        []byte{0x01, 0x04, 0x05, 0xd4, 0x03, 0x05, 0xc2, 0x23, 0x05},
		LastReceived:    []byte{0x01, 0x04, 0x05, 0xd0, 0x05, 0x06, 0x11, 0x22, 0x33, 0x44},
	}
	challenge := bytes.Repeat([]byte{0xa5}, 16)
	response := bytes.Repeat([]byte{0x5a}, 16)
	handoff := &fakeRealmHandoff{}
	call, accepted, err := (&relaySink{s: s}).RelayRealm(callsink.RealmRequest{
		Realm: "EXAMPLE.NET", Username: "cpe@example.net", SubscriberMAC: "aa:bb:cc:dd:ee:ff",
		ChannelFD: 7, LCP: lcp,
		Auth:    callsink.ProxyAuth{Kind: callsink.AuthKindCHAP, Identifier: 9, Challenge: challenge, Response: response},
		Handoff: handoff,
	})
	require.NoError(t, err)
	require.True(t, accepted, "a matched realm is relayed")
	require.NotNil(t, call)

	sccrq := readDatagram(t, client)
	require.Equal(t, MsgSCCRQ, msgTypeOf(t, sccrq), "ze dialed lns-a")
	zeTIDBytes := extractAVP(t, sccrq, AVPAssignedTunnelID)
	zeTID := uint16(zeTIDBytes[0])<<8 | uint16(zeTIDBytes[1])
	client.Send(t, buildSCCRPForSCCRQ(t, sccrq, 909))
	waitForLog(t, logs, "ICRQ sent; session wait-reply (LAC incoming)")

	icrq := readUntilMsgType(t, client, MsgICRQ, 3)
	zeSIDBytes := extractAVP(t, icrq, AVPAssignedSessionID)
	zeSID := uint16(zeSIDBytes[0])<<8 | uint16(zeSIDBytes[1])
	require.Equal(t, "aa:bb:cc:dd:ee:ff", string(extractAVP(t, icrq, AVPCallingNumber)))

	body := icrpBody(707)
	pkt := make([]byte, ControlHeaderLen+len(body))
	WriteControlHeader(pkt, 0, uint16(len(pkt)), zeTID, zeSID, 1, 3)
	copy(pkt[ControlHeaderLen:], body)
	client.Send(t, pkt)

	iccn := readUntilMsgType(t, client, MsgICCN, 4)
	hdr, err := ParseMessageHeader(iccn)
	require.NoError(t, err)
	order, avps := avpsOf(t, iccn[hdr.PayloadOff:hdr.Length])

	for _, want := range []struct {
		attr  AVPType
		value []byte
	}{
		{AVPInitialReceivedLCPConfReq, lcp.InitialReceived},
		{AVPLastSentLCPConfReq, lcp.LastSent},
		{AVPLastReceivedLCPConfReq, lcp.LastReceived},
		{AVPProxyAuthenType, []byte{0x00, 0x02}},
		{AVPProxyAuthenName, []byte("cpe@example.net")},
		{AVPProxyAuthenChallenge, challenge},
		{AVPProxyAuthenID, []byte{0x00, 0x09}},
		{AVPProxyAuthenResponse, response},
	} {
		got, ok := avps[want.attr]
		require.True(t, ok, "ICCN carries AVP %d", want.attr)
		require.Equal(t, want.value, got.value, "AVP %d value", want.attr)
		require.Zero(t, got.flags&FlagMandatory, "AVP %d has M=0", want.attr)
		require.Zero(t, got.flags&FlagHidden, "AVP %d is not hidden for CHAP", want.attr)
	}
	require.NotContains(t, order, AVPRandomVector, "CHAP hides nothing, so no Random Vector")

	r.tunnelsMu.Lock()
	tun := r.tunnelsByLocalID[zeTID]
	require.NotNil(t, tun)
	sess := tun.lookupSession(zeSID)
	require.NotNil(t, sess)
	require.NotNil(t, sess.realm)
	require.Equal(t, "example.net", sess.realm.realm)
	require.Equal(t, "lns-a", sess.realm.remote)
	r.tunnelsMu.Unlock()
}

// TestRelaySinkRealmNoMatchReturnsUnaccepted -- AC-5.
//
// VALIDATES: a realm no binding names, and an empty realm (a username with no
// '@'), are not relayed, so the PPPoE side authenticates locally.
func TestRelaySinkRealmNoMatchReturnsUnaccepted(t *testing.T) {
	s := &Subsystem{
		params: Parameters{
			Remotes:     []Remote{{Name: "lns-a", Address: netip.MustParseAddrPort("192.0.2.1:1701")}},
			RelayRealms: []RealmRelayBinding{{Realm: "example.net", Remote: "lns-a"}},
		},
		logger: slog.Default(),
	}
	for _, realm := range []string{"example.org", "", "x.example.net"} {
		call, accepted, err := (&relaySink{s: s}).RelayRealm(callsink.RealmRequest{
			Realm: realm, Handoff: &fakeRealmHandoff{},
			Auth: callsink.ProxyAuth{Kind: callsink.AuthKindCHAP},
		})
		require.NoError(t, err, realm)
		require.False(t, accepted, realm)
		require.Nil(t, call, realm)
	}
}

// TestRelaySinkRealmDialFailureReportsRefusal -- AC-6.
//
// VALIDATES: a matched realm whose call cannot start returns an error (the
// PPPoE side then refuses the subscriber), and a matched realm whose tunnel
// fails before the call is placed reports Ended to the handoff.
func TestRelaySinkRealmDialFailureReportsRefusal(t *testing.T) {
	s := &Subsystem{
		params: Parameters{
			Remotes:     []Remote{{Name: "lns-a", Address: netip.MustParseAddrPort("192.0.2.1:1701")}},
			RelayRealms: []RealmRelayBinding{{Realm: "example.net", Remote: "lns-a"}},
		},
		logger: slog.Default(),
	}
	_, accepted, err := (&relaySink{s: s}).RelayRealm(callsink.RealmRequest{
		Realm: "example.net", Handoff: &fakeRealmHandoff{},
		Auth: callsink.ProxyAuth{Kind: callsink.AuthKindCHAP},
	})
	require.ErrorIs(t, err, errRelayNoListener)
	require.False(t, accepted)

	handoff := &fakeRealmHandoff{}
	rc := &realmCall{realm: "example.net", remote: "lns-a", handoff: handoff}
	tun := &L2TPTunnel{pendingCalls: []*pendingCall{{params: callParams{realm: rc}}}}
	tun.resolvePendingCall(callOutcome{err: errCallTunnelSetupFailed})
	require.Len(t, handoff.ended, 1, "a tunnel that died before the call reports Ended once")
	require.Contains(t, handoff.ended[0], "example.net")
	rc.end("again")
	require.Len(t, handoff.ended, 1, "Ended is reported once")
}

// TestRealmCallCancelOrdersAgainstBridge -- the Bridged row of the handoff
// table (AC-15).
//
// VALIDATES: Cancel before the bridge never tears a session down and makes
// markBridged refuse, so the kernel worker fails the setup (CDN) instead of
// reporting a bridge; Cancel after markBridged tears the bridged session
// tid/sid down exactly once.
// PREVENTS: a subscriber PADT after the bridge leaving the L2TP session up,
// and a Cancel racing the bridge leaving it up too.
func TestRealmCallCancelOrdersAgainstBridge(t *testing.T) {
	type torn struct{ tid, sid uint16 }
	var got []torn
	newCall := func() *realmCall {
		return &realmCall{realm: "example.net", handoff: &fakeRealmHandoff{},
			teardown: func(tid, sid uint16) { got = append(got, torn{tid, sid}) }}
	}

	got = nil
	rc := newCall()
	rc.Cancel()
	require.Empty(t, got, "Cancel before the bridge tears nothing down")
	require.False(t, rc.markBridged(4, 9), "a canceled call MUST NOT be marked bridged")
	require.True(t, rc.isCancelled())

	got = nil
	rc = newCall()
	require.True(t, rc.markBridged(4, 9))
	rc.Cancel()
	require.Equal(t, []torn{{4, 9}}, got, "Cancel after the bridge tears the bridged session down")
	rc.Cancel()
	require.Len(t, got, 1, "Cancel is idempotent")
}

// TestRealmCallCancelAfterBridgeSendsCDN -- the Bridged row on the wire.
//
// VALIDATES: a realm call placed through the relay sink, once marked bridged
// by the kernel worker, sends the LNS a CDN with Result Code 1 (RFC 2661
// Section 4.4.2, "Call disconnected due to loss of carrier") for its own
// session when the PPPoE side cancels it, and reports Ended to the handoff.
// PREVENTS: a subscriber hang-up that the LNS never hears about.
func TestRealmCallCancelAfterBridgeSendsCDN(t *testing.T) {
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()
	timer := newTunnelTimer(r.tickCh, r.updateCh)
	require.NoError(t, timer.Start())
	defer timer.Stop()
	client := newClient(t, ln)
	defer client.Close()

	s := &Subsystem{
		params: Parameters{
			Remotes:     []Remote{{Name: "lns-a", Address: clientAddrPort(t, client)}},
			RelayRealms: []RealmRelayBinding{{Realm: "example.net", Remote: "lns-a"}},
		},
		reactors: []*l2tpReactor{r},
		logger:   slog.Default(),
	}
	handoff := &fakeRealmHandoff{}
	call, accepted, err := (&relaySink{s: s}).RelayRealm(callsink.RealmRequest{
		Realm: "example.net", Username: "cpe@example.net", SubscriberMAC: "aa:bb:cc:dd:ee:ff",
		ChannelFD: 7,
		Auth:      callsink.ProxyAuth{Kind: callsink.AuthKindCHAP, Identifier: 1, Challenge: []byte{1}, Response: []byte{2}},
		Handoff:   handoff,
	})
	require.NoError(t, err)
	require.True(t, accepted)

	sccrq := readDatagram(t, client)
	zeTIDBytes := extractAVP(t, sccrq, AVPAssignedTunnelID)
	zeTID := uint16(zeTIDBytes[0])<<8 | uint16(zeTIDBytes[1])
	client.Send(t, buildSCCRPForSCCRQ(t, sccrq, 909))
	waitForLog(t, logs, "ICRQ sent; session wait-reply (LAC incoming)")
	icrq := readUntilMsgType(t, client, MsgICRQ, 3)
	zeSIDBytes := extractAVP(t, icrq, AVPAssignedSessionID)
	zeSID := uint16(zeSIDBytes[0])<<8 | uint16(zeSIDBytes[1])

	body := icrpBody(707)
	pkt := make([]byte, ControlHeaderLen+len(body))
	WriteControlHeader(pkt, 0, uint16(len(pkt)), zeTID, zeSID, 1, 3)
	copy(pkt[ControlHeaderLen:], body)
	client.Send(t, pkt)
	readUntilMsgType(t, client, MsgICCN, 4)
	// Acknowledge the ICCN so the CDN is not queued behind it.
	client.Send(t, buildZLBAck(zeTID, 2, 4))

	rc, ok := call.(*realmCall)
	require.True(t, ok)
	require.True(t, rc.markBridged(zeTID, zeSID), "the kernel worker's mark on a live call")
	call.Cancel()

	cdn := readUntilMsgType(t, client, MsgCDN, 4)
	hdr, err := ParseMessageHeader(cdn)
	require.NoError(t, err)
	require.Equal(t, uint16(707), hdr.SessionID, "the CDN is addressed to the LNS's session")
	result := extractAVP(t, cdn, AVPResultCode)
	require.GreaterOrEqual(t, len(result), 2)
	require.Equal(t, uint16(1), uint16(result[0])<<8|uint16(result[1]), "Result Code 1: loss of carrier")
	require.Equal(t, zeSIDBytes, extractAVP(t, cdn, AVPAssignedSessionID))

	handoff.mu.Lock()
	ended := len(handoff.ended)
	handoff.mu.Unlock()
	require.Equal(t, 1, ended, "the teardown reports Ended once")
}

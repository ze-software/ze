package l2tpauthradius

import (
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/radius"
)

type acctCapture struct {
	mu      sync.Mutex
	packets []capturedAcct
	waitCh  chan struct{}
}

type capturedAcct struct {
	statusType       uint8
	username         string
	sessionID        string
	terminateCause   uint32
	callingStationID string
	peerAddr         string
	nasPortID        string
	numericAttrs     map[uint8][]byte
}

func newAcctCapture() *acctCapture {
	return &acctCapture{waitCh: make(chan struct{}, 50)}
}

func (c *acctCapture) add(pkt *radius.Packet) {
	cap := capturedAcct{numericAttrs: make(map[uint8][]byte)}
	for _, attr := range pkt.Attrs {
		switch attr.Type {
		case radius.AttrAcctInputOctets, radius.AttrAcctOutputOctets,
			radius.AttrAcctInputPackets, radius.AttrAcctOutputPackets,
			radius.AttrAcctInputGigawords, radius.AttrAcctOutputGigawords,
			radius.AttrAcctSessionTime:
			cap.numericAttrs[attr.Type] = append([]byte(nil), attr.Value...)
		}
	}
	if v := pkt.FindAttr(radius.AttrAcctStatusType); len(v) == 4 {
		cap.statusType = v[3]
	}
	if v := pkt.FindAttr(radius.AttrUserName); v != nil {
		cap.username = string(v)
	}
	if v := pkt.FindAttr(radius.AttrAcctSessionID); v != nil {
		cap.sessionID = string(v)
	}
	if v := pkt.FindAttr(radius.AttrAcctTerminateCause); len(v) == 4 {
		cap.terminateCause = binary.BigEndian.Uint32(v)
	}
	if v := pkt.FindAttr(radius.AttrCallingStationID); v != nil {
		cap.callingStationID = string(v)
	}
	if v := pkt.FindAttr(radius.AttrFramedIPAddress); v != nil {
		cap.peerAddr = net.IP(v).String()
	}
	cap.nasPortID = string(pkt.FindAttr(radius.AttrNASPortID))
	c.mu.Lock()
	c.packets = append(c.packets, cap)
	c.mu.Unlock()
	c.waitCh <- struct{}{}
}

func (c *acctCapture) waitN(t *testing.T, n int) []capturedAcct {
	t.Helper()
	for range n {
		select {
		case <-c.waitCh:
		case <-time.After(5 * time.Second):
			c.mu.Lock()
			got := len(c.packets)
			c.mu.Unlock()
			t.Fatalf("timed out waiting for %d acct packets, got %d", n, got)
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]capturedAcct, len(c.packets))
	copy(result, c.packets)
	return result
}

func startAcctServer(t *testing.T, sharedKey []byte, capture *acctCapture) (*net.UDPConn, string) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		buf := make([]byte, radius.MaxPacketLen)
		for {
			n, from, readErr := conn.ReadFromUDP(buf)
			if readErr != nil {
				return
			}
			pkt, decErr := radius.Decode(buf[:n])
			if decErr != nil {
				continue
			}
			if pkt.Code == radius.CodeAccountingReq {
				capture.add(pkt)

				resp := make([]byte, radius.HeaderLen)
				resp[0] = radius.CodeAccountingResp
				resp[1] = pkt.Identifier
				binary.BigEndian.PutUint16(resp[2:4], radius.HeaderLen)

				var reqAuth [radius.AuthenticatorLen]byte
				copy(reqAuth[:], buf[4:4+radius.AuthenticatorLen])
				auth := radius.ResponseAuthenticator(radius.CodeAccountingResp, pkt.Identifier, radius.HeaderLen, reqAuth, nil, sharedKey)
				copy(resp[4:4+radius.AuthenticatorLen], auth[:])

				conn.WriteToUDP(resp, from) //nolint:errcheck // test mock
			}
		}
	}()

	return conn, conn.LocalAddr().String()
}

// VALIDATES: session IP assignment emits subscriber identity in Accounting-Start.
func TestRADIUSAcctStart(t *testing.T) {
	sharedKey := []byte("accttest")
	capture := newAcctCapture()
	conn, addr := startAcctServer(t, sharedKey, capture)
	defer conn.Close() //nolint:errcheck // test cleanup

	client, err := radius.NewClient(radius.ClientConfig{
		Servers: []radius.Server{{Address: addr, SharedKey: sharedKey}},
		Timeout: 2 * time.Second,
		Retries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close() //nolint:errcheck // test cleanup

	acct := newRADIUSAcct()
	acct.setClient(client, "test-nas", 300*time.Second, addr, nil, "")

	acct.onSessionIPAssigned(&events.SessionIPAssignedPayload{
		SubscriberID:     "l2tp-1-2",
		TunnelID:         1,
		SessionID:        2,
		Username:         "alice",
		PeerAddr:         "10.0.0.1",
		CallingStationID: "+441234567890",
	})

	packets := capture.waitN(t, 1)
	if packets[0].statusType != radius.AcctStatusStart {
		t.Errorf("status type: got %d, want %d", packets[0].statusType, radius.AcctStatusStart)
	}
	if packets[0].username != "alice" {
		t.Errorf("username: got %q, want %q", packets[0].username, "alice")
	}
	if packets[0].callingStationID != "+441234567890" {
		t.Errorf("Calling-Station-Id: got %q, want %q", packets[0].callingStationID, "+441234567890")
	}

	acct.Stop()
}

func TestRADIUSAcctStop(t *testing.T) {
	sharedKey := []byte("accttest")
	capture := newAcctCapture()
	conn, addr := startAcctServer(t, sharedKey, capture)
	defer conn.Close() //nolint:errcheck // test cleanup

	client, err := radius.NewClient(radius.ClientConfig{
		Servers: []radius.Server{{Address: addr, SharedKey: sharedKey}},
		Timeout: 2 * time.Second,
		Retries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close() //nolint:errcheck // test cleanup

	acct := newRADIUSAcct()
	acct.setClient(client, "test-nas", 300*time.Second, addr, nil, "")

	acct.onSessionIPAssigned(&events.SessionIPAssignedPayload{
		SubscriberID: "l2tp-1-3",
		TunnelID:     1,
		SessionID:    3,
		Username:     "bob",
		PeerAddr:     "10.0.0.2",
	})

	capture.waitN(t, 1) // wait for start

	acct.onSessionDown(&events.SessionDownPayload{
		TunnelID:  1,
		SessionID: 3,
	})

	packets := capture.waitN(t, 1) // wait for stop (1 more after start)
	last := packets[len(packets)-1]
	if last.statusType != radius.AcctStatusStop {
		t.Errorf("status type: got %d, want %d (stop)", last.statusType, radius.AcctStatusStop)
	}
}

func TestRADIUSAcctInterim(t *testing.T) {
	sharedKey := []byte("accttest")
	capture := newAcctCapture()
	conn, addr := startAcctServer(t, sharedKey, capture)
	defer conn.Close() //nolint:errcheck // test cleanup

	client, err := radius.NewClient(radius.ClientConfig{
		Servers: []radius.Server{{Address: addr, SharedKey: sharedKey}},
		Timeout: 2 * time.Second,
		Retries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close() //nolint:errcheck // test cleanup

	acct := newRADIUSAcct()
	acct.setClient(client, "test-nas", 200*time.Millisecond, addr, nil, "") // short interval for testing

	acct.onSessionIPAssigned(&events.SessionIPAssignedPayload{
		SubscriberID: "l2tp-1-4",
		TunnelID:     1,
		SessionID:    4,
		Username:     "charlie",
		PeerAddr:     "10.0.0.3",
	})

	// Wait for start + at least 1 interim.
	packets := capture.waitN(t, 2)

	foundInterim := false
	for _, p := range packets {
		if p.statusType == radius.AcctStatusInterimUpdate {
			foundInterim = true
			break
		}
	}
	if !foundInterim {
		t.Error("expected at least one interim-update packet")
	}

	acct.Stop()
}

// TestSplitGigawords proves splitGigawords derives Gigawords as uint32(bytes>>32)
// directly from the 64-bit byte counter, not from a separately tracked wrap event.
func TestSplitGigawords(t *testing.T) {
	tests := []struct {
		name     string
		bytes    uint64
		wantOct  uint32
		wantGiga uint32
	}{
		{"zero", 0, 0, 0},
		{"small", 1000, 1000, 0},
		{"max_uint32", 0xFFFFFFFF, 0xFFFFFFFF, 0},
		{"one_wrap", 0x100000000, 0, 1},
		{"wrap_plus_remainder", 0x100000001, 1, 1},
		{"large", 0x300000000 + 42, 42, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oct, giga := splitGigawords(tc.bytes)
			if oct != tc.wantOct {
				t.Errorf("octets: got %d, want %d", oct, tc.wantOct)
			}
			if giga != tc.wantGiga {
				t.Errorf("gigawords: got %d, want %d", giga, tc.wantGiga)
			}
		})
	}
}

// With sub-4GB counters (gigawords == 0) a Stop/Interim Accounting-Request
// carries no Acct-Input/Output-Gigawords attribute.
func TestBuildAcctPacketWithCounters(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{
		username:   "alice",
		acctSessID: "1-2-1",
	}
	bindAcctUnit(sess, newAcctUnitCapture(fixedAcctUnit(unitCounters(5000, 3000, 50, 30), nil)))

	pkt := acct.buildAcctPacket(sess, "nas1", nil, radius.AcctStatusInterimUpdate, 60)

	assertAttrUint32(t, pkt, radius.AttrAcctInputOctets, 5000)
	assertAttrUint32(t, pkt, radius.AttrAcctOutputOctets, 3000)
	assertAttrUint32(t, pkt, radius.AttrAcctInputPackets, 50)
	assertAttrUint32(t, pkt, radius.AttrAcctOutputPackets, 30)

	if v := pkt.FindAttr(radius.AttrAcctInputGigawords); v != nil {
		t.Error("gigawords should not be present for <4GB counters")
	}
	if v := pkt.FindAttr(radius.AttrAcctOutputGigawords); v != nil {
		t.Error("gigawords should not be present for <4GB counters")
	}
}

// RFC requirement: RFC2869-x-2 positive -- an Accounting-Request with Acct-Status-Type
// Stop carries the Acct-Input/Output-Gigawords attributes, and when the octet
// counter has wrapped past 2^32 each holds the wrap count.
func TestBuildAcctPacketGigawords(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{
		username:   "bob",
		acctSessID: "1-3-1",
	}
	unit := newAcctUnitCapture(fixedAcctUnit(unitCounters(0, 0, 0, 0), nil))
	unit.measure(measuredSample(
		0x200000000+100, // 2 gigawords + 100 octets
		0x300000000+200, // 3 gigawords + 200 octets
		10, 20))
	bindAcctUnit(sess, unit)
	// A Stop reports the terminal measurement of the network that ended.
	sess.captureEnd(events.TerminateCauseUserRequest)
	sess.captureTerminal()

	pkt := acct.buildAcctPacket(sess, "nas1", nil, radius.AcctStatusStop, 120)

	assertAttrUint32(t, pkt, radius.AttrAcctInputOctets, 100)
	assertAttrUint32(t, pkt, radius.AttrAcctOutputOctets, 200)
	assertAttrUint32(t, pkt, radius.AttrAcctInputGigawords, 2)
	assertAttrUint32(t, pkt, radius.AttrAcctOutputGigawords, 3)
	assertAttrUint32(t, pkt, radius.AttrAcctInputPackets, 10)
	assertAttrUint32(t, pkt, radius.AttrAcctOutputPackets, 20)
}

func TestBuildAcctPacketStartZeroCounters(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{
		username:   "carol",
		acctSessID: "1-4-1",
	}
	unit := newAcctUnitCapture(fixedAcctUnit(unitCounters(999, 0, 0, 0), nil))
	bindAcctUnit(sess, unit)

	pkt := acct.buildAcctPacket(sess, "nas1", nil, radius.AcctStatusStart, 0)

	if unit.liveReads() != 0 {
		t.Error("the original unit should not be read for Start packets")
	}
	select {
	case <-unit.awaiting:
		t.Error("the original unit's terminal counters should not be awaited for Start packets")
	default:
	}
	if v := pkt.FindAttr(radius.AttrAcctInputOctets); v != nil {
		t.Error("Start packet should not include counter attributes")
	}
}

// A session whose original network was never bound has no counters to report.
func TestBuildAcctPacketMissingIface(t *testing.T) {
	useAcctCaptures(t, func(l2tp.SessionSelector) (l2tp.CapturedSession, error) {
		t.Error("no capture may be sought for a session whose network was never bound")
		return nil, errors.New("unexpected capture")
	})
	acct := newRADIUSAcct()
	sess := &acctSession{
		username:   "dave",
		acctSessID: "1-5-1",
	}

	pkt := acct.buildAcctPacket(sess, "nas1", nil, radius.AcctStatusInterimUpdate, 60)

	assertAcctPacketCountersAbsent(t, pkt)
	assertAttrUint32(t, pkt, radius.AttrAcctSessionTime, 60)
	if got := string(pkt.FindAttr(radius.AttrUserName)); got != "dave" {
		t.Errorf("User-Name: got %q, want dave", got)
	}
	if got := string(pkt.FindAttr(radius.AttrAcctSessionID)); got != "1-5-1" {
		t.Errorf("Acct-Session-Id: got %q, want 1-5-1", got)
	}
}

func TestBuildAcctPacketGetStatsError(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{
		username:   "frank",
		acctSessID: "1-6-1",
	}
	bindAcctUnit(sess, newAcctUnitCapture(fixedAcctUnit(unitCounters(0, 0, 0, 0), errors.New("original unit is closed"))))

	pkt := acct.buildAcctPacket(sess, "nas1", nil, radius.AcctStatusInterimUpdate, 60)

	assertAcctPacketCountersAbsent(t, pkt)
	assertAttrUint32(t, pkt, radius.AttrAcctSessionTime, 60)
	if got := string(pkt.FindAttr(radius.AttrUserName)); got != "frank" {
		t.Errorf("User-Name: got %q, want frank", got)
	}
	if got := string(pkt.FindAttr(radius.AttrAcctSessionID)); got != "1-6-1" {
		t.Errorf("Acct-Session-Id: got %q, want 1-6-1", got)
	}
}

func assertAcctPacketCountersAbsent(t *testing.T, pkt *radius.Packet) {
	t.Helper()
	for _, attr := range []uint8{
		radius.AttrAcctInputOctets, radius.AttrAcctOutputOctets,
		radius.AttrAcctInputPackets, radius.AttrAcctOutputPackets,
		radius.AttrAcctInputGigawords, radius.AttrAcctOutputGigawords,
	} {
		if value := pkt.FindAttr(attr); value != nil {
			t.Errorf("unavailable counter attribute %d present: %x", attr, value)
		}
	}
}

// TestAcctSessionPppInterface proves that the session's counters come from the
// network the assignment named, not from its interface name: the accounting
// task binds the capture of that SID, TID and network lifetime.
func TestAcctSessionPppInterface(t *testing.T) {
	network, err := events.NetworkLifetime{}.Next()
	if err != nil {
		t.Fatal(err)
	}
	selectors := make(chan l2tp.SessionSelector, 1)
	useAcctCaptures(t, func(selector l2tp.SessionSelector) (l2tp.CapturedSession, error) {
		selectors <- selector
		return newAcctUnitCapture(fixedAcctUnit(unitCounters(0, 0, 0, 0), nil)), nil
	})
	sharedKey := []byte("accttest")
	capture := newAcctCapture()
	conn, addr := startAcctServer(t, sharedKey, capture)
	defer conn.Close() //nolint:errcheck // test cleanup

	client, err := radius.NewClient(radius.ClientConfig{
		Servers: []radius.Server{{Address: addr, SharedKey: sharedKey}},
		Timeout: 2 * time.Second,
		Retries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close() //nolint:errcheck // test cleanup

	acct := newRADIUSAcct()
	acct.setClient(client, "test-nas", 300*time.Second, addr, nil, "")

	acct.onSessionIPAssigned(&events.SessionIPAssignedPayload{
		SubscriberID:     "l2tp-1-10",
		TunnelID:         1,
		SessionID:        10,
		Username:         "eve",
		PeerAddr:         "10.0.0.5",
		PppInterface:     "ppp42",
		NetworkLifetime:  network,
		BaselineCounters: measuredSample(0, 0, 0, 0),
	})

	capture.waitN(t, 1) // wait for start

	acct.mu.Lock()
	sess, ok := acct.sessions[sessionKey{tunnelID: 1, sessionID: 10}]
	acct.mu.Unlock()

	if !ok {
		t.Fatal("session not found")
	}
	if sess.network != network {
		t.Errorf("network: got %v, want %v", sess.network, network)
	}
	select {
	case selector := <-selectors:
		if selector.SID != 10 || selector.TID != 1 || selector.NetworkLifetime != network {
			t.Errorf("capture selector = %+v, want SID 10, TID 1 and the assigned network", selector)
		}
	default:
		t.Error("the accounting task bound no capture before its Start")
	}
	if sess.capture.Load() == nil {
		t.Error("the bound capture is not retained by the session")
	}

	acct.Stop()
}

func assertAttrUint32(t *testing.T, pkt *radius.Packet, attrType uint8, want uint32) {
	t.Helper()
	v := pkt.FindAttr(attrType)
	if v == nil {
		t.Errorf("attr %d: not found", attrType)
		return
	}
	if len(v) != 4 {
		t.Errorf("attr %d: length %d, want 4", attrType, len(v))
		return
	}
	got := binary.BigEndian.Uint32(v)
	if got != want {
		t.Errorf("attr %d: got %d, want %d", attrType, got, want)
	}
}

// TestAcctPacketCarriesEventTimestamp covers AC-1.
//
// RFC 2869 Section 5.3: "The Value field is four octets encoding an unsigned
// integer with the number of seconds since January 1, 1970 00:00 UTC." The
// same section gives Length 6, which is Type + Length + those four octets.
func TestAcctPacketCarriesEventTimestamp(t *testing.T) {
	saved := acctNow
	acctNow = func() time.Time { return time.Unix(1756900000, 0) }
	defer func() { acctNow = saved }()

	acct := newRADIUSAcct()
	sess := &acctSession{username: "grace", acctSessID: "1-7-1"}
	// A Stop reports the instant service ended, read under the same clock.
	sess.captureEnd(events.TerminateCauseUserRequest)

	for _, statusType := range []uint8{radius.AcctStatusStart, radius.AcctStatusInterimUpdate, radius.AcctStatusStop} {
		pkt := acct.buildAcctPacket(sess, "nas1", nil, statusType, 0)
		v := pkt.FindAttr(radius.AttrEventTimestamp)
		if v == nil {
			t.Fatalf("status-type %d: no Event-Timestamp attribute", statusType)
		}
		if len(v) != 4 {
			t.Fatalf("status-type %d: Event-Timestamp value is %d octets, want 4", statusType, len(v))
		}
		if got := binary.BigEndian.Uint32(v); got != 1756900000 {
			t.Errorf("status-type %d: Event-Timestamp = %d, want 1756900000", statusType, got)
		}
	}
}

// TestAcctPacketCarriesCallingStationId covers AC-2.
//
// RFC 2865 Section 5.31: "This Attribute allows the NAS to send in the
// Access-Request packet the phone number that the call came from, using
// Automatic Number Identification (ANI) or similar technology." RFC 2866
// Section 5.13 puts it at 0-1 in an Accounting-Request, which is where a BNG
// reports the line the subscriber called from.
func TestAcctPacketCarriesCallingStationId(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{
		username:         "grace",
		acctSessID:       "1-7-1",
		callingStationID: "00:11:22:33:44:55",
	}

	for _, statusType := range []uint8{radius.AcctStatusStart, radius.AcctStatusInterimUpdate, radius.AcctStatusStop} {
		pkt := acct.buildAcctPacket(sess, "nas1", nil, statusType, 0)
		v := pkt.FindAttr(radius.AttrCallingStationID)
		if v == nil {
			t.Fatalf("status-type %d: no Calling-Station-Id attribute", statusType)
		}
		if string(v) != "00:11:22:33:44:55" {
			t.Errorf("status-type %d: Calling-Station-Id = %q, want %q", statusType, v, "00:11:22:33:44:55")
		}
	}
}

// TestAcctPacketOmitsEmptyCallingStationId covers AC-3.
//
// RFC 2865 Section 5: "Text of length zero (0) MUST NOT be sent; omit the
// entire attribute instead." A session whose peer sent no Calling Number AVP
// has no value, so the record carries no attribute rather than an empty one.
func TestAcctPacketOmitsEmptyCallingStationId(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{username: "grace", acctSessID: "1-7-1"}

	pkt := acct.buildAcctPacket(sess, "nas1", nil, radius.AcctStatusStart, 0)
	if v := pkt.FindAttr(radius.AttrCallingStationID); v != nil {
		t.Errorf("Calling-Station-Id present with no value: %q", v)
	}
}

// TestTerminateCauseOnStopOnly covers AC-4 and AC-5.
//
// RFC 2866 Section 5.10: "This attribute indicates how the session was
// terminated, and can only be present in Accounting-Request records where the
// Acct-Status-Type is set to Stop." The absence case is what catches an append
// written in the wrong place.
func TestTerminateCauseOnStopOnly(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{username: "grace", acctSessID: "1-7-1", terminateCause: events.TerminateCauseUserRequest}

	stop := acct.buildAcctPacket(sess, "nas1", nil, radius.AcctStatusStop, 60)
	v := stop.FindAttr(radius.AttrAcctTerminateCause)
	if v == nil {
		t.Fatal("Stop record carries no Acct-Terminate-Cause")
	}
	if len(v) != 4 {
		t.Fatalf("Acct-Terminate-Cause value is %d octets, want 4", len(v))
	}
	if got := binary.BigEndian.Uint32(v); got != 1 {
		t.Errorf("Acct-Terminate-Cause = %d, want 1 (User Request)", got)
	}

	for _, statusType := range []uint8{radius.AcctStatusStart, radius.AcctStatusInterimUpdate} {
		pkt := acct.buildAcctPacket(sess, "nas1", nil, statusType, 60)
		if v := pkt.FindAttr(radius.AttrAcctTerminateCause); v != nil {
			t.Errorf("status-type %d carries Acct-Terminate-Cause %v; RFC 2866 Section 5.10 allows it on Stop only", statusType, v)
		}
	}
}

// TestTerminateCausePerTeardownPath covers AC-6.
//
// Every value ze emits is one of the integers RFC 2866 Section 5.10 defines,
// and a teardown ze cannot attribute reports NAS Error rather than a guess.
func TestTerminateCausePerTeardownPath(t *testing.T) {
	cases := []struct {
		name  string
		cause events.TerminateCause
		want  uint32
	}{
		{"lcp-terminate", events.TerminateCauseUserRequest, 1},
		{"echo-timeout", events.TerminateCauseLostCarrier, 2},
		{"idle-timeout", events.TerminateCauseIdleTimeout, 4},
		{"session-timeout", events.TerminateCauseSessionTimeout, 5},
		{"admin-clear-or-coa-disconnect", events.TerminateCauseAdminReset, 6},
		{"nas-shutdown", events.TerminateCauseAdminReboot, 7},
		{"unattributable", events.TerminateCauseNASError, 9},
		{"local-stop", events.TerminateCauseNASRequest, 10},
		{"unspecified-reports-nas-error", events.TerminateCauseUnspecified, 9},
	}
	acct := newRADIUSAcct()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sess := &acctSession{username: "grace", acctSessID: "1-7-1", terminateCause: tc.cause}
			pkt := acct.buildAcctPacket(sess, "nas1", nil, radius.AcctStatusStop, 60)
			v := pkt.FindAttr(radius.AttrAcctTerminateCause)
			if v == nil {
				t.Fatal("Stop record carries no Acct-Terminate-Cause")
			}
			got := binary.BigEndian.Uint32(v)
			if got != tc.want {
				t.Errorf("Acct-Terminate-Cause = %d, want %d", got, tc.want)
			}
			if got < 1 || got > 11 {
				t.Errorf("Acct-Terminate-Cause = %d, outside the RFC 2866 Section 5.10 values ze emits", got)
			}
		})
	}
}

// TestAcctStopCarriesEventCause proves the cause crosses the boundary: what
// the reactor put on (l2tp, session-down) is what the Stop record reports.
func TestAcctStopCarriesEventCause(t *testing.T) {
	sharedKey := []byte("accttest")
	capture := newAcctCapture()
	conn, addr := startAcctServer(t, sharedKey, capture)
	defer conn.Close() //nolint:errcheck // test cleanup

	client, err := radius.NewClient(radius.ClientConfig{
		Servers: []radius.Server{{Address: addr, SharedKey: sharedKey}},
		Timeout: 2 * time.Second,
		Retries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close() //nolint:errcheck // test cleanup

	acct := newRADIUSAcct()
	acct.setClient(client, "test-nas", 300*time.Second, addr, nil, "")
	acct.onSessionIPAssigned(&events.SessionIPAssignedPayload{
		SubscriberID: "l2tp-42-7",
		TunnelID:     42, SessionID: 7, Username: "grace", PeerAddr: "10.0.0.2",
	})
	capture.waitN(t, 1)

	acct.onSessionDown(&events.SessionDownPayload{
		TunnelID: 42, SessionID: 7, Username: "grace",
		Cause: events.TerminateCauseSessionTimeout,
	})
	packets := capture.waitN(t, 1)

	last := packets[len(packets)-1]
	if last.statusType != radius.AcctStatusStop {
		t.Fatalf("last record status-type = %d, want Stop", last.statusType)
	}
	if last.terminateCause != 5 {
		t.Errorf("Acct-Terminate-Cause = %d, want 5 (Session Timeout)", last.terminateCause)
	}
}

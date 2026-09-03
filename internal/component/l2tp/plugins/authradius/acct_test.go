package l2tpauthradius

import (
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/radius"
)

type acctCapture struct {
	mu      sync.Mutex
	packets []capturedAcct
	waitCh  chan struct{}
}

type capturedAcct struct {
	statusType uint8
	username   string
	sessionID  string
}

func newAcctCapture() *acctCapture {
	return &acctCapture{waitCh: make(chan struct{}, 50)}
}

func (c *acctCapture) add(pkt *radius.Packet) {
	cap := capturedAcct{}
	if v := pkt.FindAttr(radius.AttrAcctStatusType); len(v) == 4 {
		cap.statusType = v[3]
	}
	if v := pkt.FindAttr(radius.AttrUserName); v != nil {
		cap.username = string(v)
	}
	if v := pkt.FindAttr(radius.AttrAcctSessionID); v != nil {
		cap.sessionID = string(v)
	}
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
		TunnelID:  1,
		SessionID: 2,
		Username:  "alice",
		PeerAddr:  "10.0.0.1",
	})

	packets := capture.waitN(t, 1)
	if packets[0].statusType != radius.AcctStatusStart {
		t.Errorf("status type: got %d, want %d", packets[0].statusType, radius.AcctStatusStart)
	}
	if packets[0].username != "alice" {
		t.Errorf("username: got %q, want %q", packets[0].username, "alice")
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
		TunnelID:  1,
		SessionID: 3,
		Username:  "bob",
		PeerAddr:  "10.0.0.2",
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
		TunnelID:  1,
		SessionID: 4,
		Username:  "charlie",
		PeerAddr:  "10.0.0.3",
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

// RFC requirement: RFC2869-x-5 positive -- splitGigawords derives Gigawords as
// uint32(bytes>>32) directly from the 64-bit byte counter, not from a separately
// tracked wrap event.
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

// RFC requirement: RFC2869-x-3 negative -- with sub-4GB counters (gigawords == 0) a
// Stop/Interim Accounting-Request carries no Acct-Input/Output-Gigawords attribute.
func TestBuildAcctPacketWithCounters(t *testing.T) {
	saved := acctGetStats
	acctGetStats = func(name string) (*iface.InterfaceStats, error) {
		return &iface.InterfaceStats{
			RxBytes:   5000,
			TxBytes:   3000,
			RxPackets: 50,
			TxPackets: 30,
		}, nil
	}
	defer func() { acctGetStats = saved }()

	acct := newRADIUSAcct()
	sess := &acctSession{
		username:     "alice",
		acctSessID:   "1-2-1",
		pppInterface: "ppp0",
	}

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
// Stop carries the Acct-Input/Output-Gigawords attributes.
// RFC requirement: RFC2869-x-3 positive -- when the octet counter has wrapped past 2^32
// the Gigawords attribute is present and holds the wrap count.
func TestBuildAcctPacketGigawords(t *testing.T) {
	saved := acctGetStats
	acctGetStats = func(name string) (*iface.InterfaceStats, error) {
		return &iface.InterfaceStats{
			RxBytes:   0x200000000 + 100, // 2 gigawords + 100 octets
			TxBytes:   0x300000000 + 200, // 3 gigawords + 200 octets
			RxPackets: 10,
			TxPackets: 20,
		}, nil
	}
	defer func() { acctGetStats = saved }()

	acct := newRADIUSAcct()
	sess := &acctSession{
		username:     "bob",
		acctSessID:   "1-3-1",
		pppInterface: "ppp1",
	}

	pkt := acct.buildAcctPacket(sess, "nas1", nil, radius.AcctStatusStop, 120)

	assertAttrUint32(t, pkt, radius.AttrAcctInputOctets, 100)
	assertAttrUint32(t, pkt, radius.AttrAcctOutputOctets, 200)
	assertAttrUint32(t, pkt, radius.AttrAcctInputGigawords, 2)
	assertAttrUint32(t, pkt, radius.AttrAcctOutputGigawords, 3)
	assertAttrUint32(t, pkt, radius.AttrAcctInputPackets, 10)
	assertAttrUint32(t, pkt, radius.AttrAcctOutputPackets, 20)
}

func TestBuildAcctPacketStartZeroCounters(t *testing.T) {
	called := false
	saved := acctGetStats
	acctGetStats = func(name string) (*iface.InterfaceStats, error) {
		called = true
		return &iface.InterfaceStats{RxBytes: 999}, nil
	}
	defer func() { acctGetStats = saved }()

	acct := newRADIUSAcct()
	sess := &acctSession{
		username:     "carol",
		acctSessID:   "1-4-1",
		pppInterface: "ppp2",
	}

	pkt := acct.buildAcctPacket(sess, "nas1", nil, radius.AcctStatusStart, 0)

	if called {
		t.Error("GetStats should not be called for Start packets")
	}
	if v := pkt.FindAttr(radius.AttrAcctInputOctets); v != nil {
		t.Error("Start packet should not include counter attributes")
	}
}

func TestBuildAcctPacketMissingIface(t *testing.T) {
	saved := acctGetStats
	acctGetStats = func(name string) (*iface.InterfaceStats, error) {
		t.Error("GetStats should not be called when pppInterface is empty")
		return &iface.InterfaceStats{}, nil
	}
	defer func() { acctGetStats = saved }()

	acct := newRADIUSAcct()
	sess := &acctSession{
		username:   "dave",
		acctSessID: "1-5-1",
	}

	pkt := acct.buildAcctPacket(sess, "nas1", nil, radius.AcctStatusInterimUpdate, 60)

	assertAttrUint32(t, pkt, radius.AttrAcctInputOctets, 0)
	assertAttrUint32(t, pkt, radius.AttrAcctOutputOctets, 0)
	assertAttrUint32(t, pkt, radius.AttrAcctInputPackets, 0)
	assertAttrUint32(t, pkt, radius.AttrAcctOutputPackets, 0)
}

func TestBuildAcctPacketGetStatsError(t *testing.T) {
	saved := acctGetStats
	acctGetStats = func(name string) (*iface.InterfaceStats, error) {
		return nil, errors.New("interface not found")
	}
	defer func() { acctGetStats = saved }()

	acct := newRADIUSAcct()
	sess := &acctSession{
		username:     "frank",
		acctSessID:   "1-6-1",
		pppInterface: "ppp99",
	}

	pkt := acct.buildAcctPacket(sess, "nas1", nil, radius.AcctStatusInterimUpdate, 60)

	assertAttrUint32(t, pkt, radius.AttrAcctInputOctets, 0)
	assertAttrUint32(t, pkt, radius.AttrAcctOutputOctets, 0)
	assertAttrUint32(t, pkt, radius.AttrAcctInputPackets, 0)
	assertAttrUint32(t, pkt, radius.AttrAcctOutputPackets, 0)
}

func TestAcctSessionPppInterface(t *testing.T) {
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
		TunnelID:     1,
		SessionID:    10,
		Username:     "eve",
		PeerAddr:     "10.0.0.5",
		PppInterface: "ppp42",
	})

	capture.waitN(t, 1) // wait for start

	acct.mu.Lock()
	sess, ok := acct.sessions[sessionKey{1, 10}]
	acct.mu.Unlock()

	if !ok {
		t.Fatal("session not found")
	}
	if sess.pppInterface != "ppp42" {
		t.Errorf("pppInterface: got %q, want %q", sess.pppInterface, "ppp42")
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

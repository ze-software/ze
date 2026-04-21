package l2tpauthradius

import (
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp/events"
	"codeberg.org/thomas-mangin/ze/internal/component/radius"
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
	acct.setClient(client, "test-nas", 300*time.Second)

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
	acct.setClient(client, "test-nas", 300*time.Second)

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
	acct.setClient(client, "test-nas", 200*time.Millisecond) // short interval for testing

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

// Subscriber address reporting in Accounting-Request packets.
//
// VALIDATES: RFC 2866 Section 4.1 -- "If the Accounting-Request packet includes a
// Framed-IP-Address, that attribute MUST contain the IP address of the user ... the
// Framed-IP-Address (if any) in the Accounting-Request MUST contain the actual IP
// address assigned or negotiated." Ze sources it from the IPCP-negotiated peer
// address carried by (l2tp, session-ip-assigned), the same value the kernel put on
// pppN, so the attribute reports the address the subscriber actually holds.
// PREVENTS: an Accounting-Request that reports no subscriber address at all (billing
// cannot map traffic to an address), and one that reports something other than the
// negotiated address (a non-IPv4 value encoded into a 4-octet field).
//
// The producer is buildAcctPacket (acct.go). Its address input is
// acctSession.peerAddr, set in onSessionIPAssigned from
// l2tpevents.SessionIPAssignedPayload.PeerAddr, which the reactor fills from
// ppp.EventSessionIPAssigned.Peer (the IPCP-negotiated subscriber address).
//
// Format: RFC 2865 Section 5.8 -- Type 8, Length 6, four octets in network order.
// Cardinality: RFC 2866 Section 5.13 -- 0-1 instances in an Accounting-Request.

package l2tpauthradius

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/radius"
)

// RFC requirement: RFC2866-4.1-1 positive -- every Accounting-Request of a session with an
// IPv4 assignment carries exactly one Framed-IP-Address, and it is the negotiated address.
func TestAcctFramedIPAddressPresent(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{tunnelID: 1, sessionID: 2, username: "alice", acctSessID: "1-2-1", peerAddr: "10.100.0.2"}

	for _, status := range []uint8{radius.AcctStatusStart, radius.AcctStatusInterimUpdate, radius.AcctStatusStop} {
		pkt := acct.buildAcctPacket(sess, "lns1", nil, status, 0)
		vals := pkt.FindAllAttr(radius.AttrFramedIPAddress)
		if len(vals) != 1 {
			t.Fatalf("status %d: Framed-IP-Address count: got %d, want 1", status, len(vals))
		}
		if len(vals[0]) != 4 {
			t.Fatalf("status %d: Framed-IP-Address length: got %d, want 4", status, len(vals[0]))
		}
		if got := net.IP(vals[0]).String(); got != "10.100.0.2" {
			t.Fatalf("status %d: Framed-IP-Address = %s, want 10.100.0.2", status, got)
		}
	}
}

// The attribute reports the address of the SESSION, never the NAS. A session
// whose assigned address differs from the RADIUS source address must report its
// own, and the two attributes must not be confused.
// RFC requirement: RFC2866-4.1-1 negative -- the attribute never reports the NAS address;
// a session whose assignment differs from the RADIUS source address reports its own.
func TestAcctFramedIPAddressIsSubscriberNotNAS(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{tunnelID: 1, sessionID: 2, username: "alice", acctSessID: "1-2-1", peerAddr: "10.100.0.7"}
	nasIP := net.ParseIP("192.0.2.1")

	pkt := acct.buildAcctPacket(sess, "lns1", nasIP, radius.AcctStatusStart, 0)

	framed := pkt.FindAttr(radius.AttrFramedIPAddress)
	if got := net.IP(framed).String(); got != "10.100.0.7" {
		t.Fatalf("Framed-IP-Address = %s, want 10.100.0.7", got)
	}
	nas := pkt.FindAttr(radius.AttrNASIPAddress)
	if got := net.IP(nas).String(); got != "192.0.2.1" {
		t.Fatalf("NAS-IP-Address = %s, want 192.0.2.1", got)
	}
}

// A four-octet field cannot carry a v6 address, and an unset address is not an
// address. Both cases omit the attribute instead of encoding a wrong value: the
// IPv6CP branch of the reactor records a link-local, and a session tracked before
// its IPCP completes has no address at all.
// RFC requirement: RFC2866-4.1-1 negative -- an unset, IPv6 or unparseable assignment emits
// no attribute rather than four octets that are not the user's address.
func TestAcctFramedIPAddressOmittedWhenNotIPv4(t *testing.T) {
	cases := []struct {
		name string
		addr string
	}{
		{"unset", ""},
		{"ipv6 link-local", "fe80::1"},
		{"ipv6 global", "2001:db8::1"},
		{"not an address", "pppoe0"},
	}
	acct := newRADIUSAcct()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sess := &acctSession{tunnelID: 1, sessionID: 2, username: "alice", acctSessID: "1-2-1", peerAddr: tc.addr}
			pkt := acct.buildAcctPacket(sess, "lns1", nil, radius.AcctStatusStart, 0)
			if vals := pkt.FindAllAttr(radius.AttrFramedIPAddress); len(vals) != 0 {
				t.Fatalf("Framed-IP-Address emitted for peerAddr %q: %v", tc.addr, vals)
			}
		})
	}
}

// An IPv4-mapped IPv6 form ("::ffff:10.0.0.1") is still an IPv4 address and is
// encoded as its four octets, not as a 16-octet value.
func TestAcctFramedIPAddressIPv4Mapped(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{tunnelID: 1, sessionID: 2, username: "alice", acctSessID: "1-2-1", peerAddr: "::ffff:10.0.0.1"}

	pkt := acct.buildAcctPacket(sess, "lns1", nil, radius.AcctStatusStart, 0)
	val := pkt.FindAttr(radius.AttrFramedIPAddress)
	if len(val) != 4 {
		t.Fatalf("Framed-IP-Address length: got %d, want 4", len(val))
	}
	if got := net.IP(val).String(); got != "10.0.0.1" {
		t.Fatalf("Framed-IP-Address = %s, want 10.0.0.1", got)
	}
}

// The address reaches the packet from the session event, not from anywhere the
// accounting code reaches into. This drives the real entry point,
// onSessionIPAssigned, so a regression in the payload-to-session field mapping
// fails here: asserting on a hand-built acctSession would pass with that
// mapping deleted.
//
// The same test pins the NAS-Port-Id resolution, which happens once, here, so
// that every record of the session repeats one text even across a config reload.
// RFC requirement: RFC2866-4.1-1 positive -- the reported address is the one the session
// event delivered, which is the address IPCP negotiated and the reactor put on pppN.
func TestSessionEventDrivesAddressAndPortID(t *testing.T) {
	sharedKey := []byte("addrtest")
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
	acct.setClient(client, "lns1", 300*time.Second, addr, nil, "{nas-id}:{tunnel-id}.{session-id}")
	defer acct.Stop()

	acct.onSessionIPAssigned(&events.SessionIPAssignedPayload{
		TunnelID:  1027,
		SessionID: 42,
		Username:  "alice",
		PeerAddr:  "198.51.100.23",
	})
	capture.waitN(t, 1)

	acct.mu.Lock()
	sess, ok := acct.sessions[sessionKey{1027, 42}]
	acct.mu.Unlock()
	if !ok {
		t.Fatal("no accounting session was created for the event")
	}
	if sess.peerAddr != "198.51.100.23" {
		t.Fatalf("session peerAddr = %q, want the address the event carried", sess.peerAddr)
	}
	if sess.nasPortID != "lns1:1027.42" {
		t.Fatalf("session nasPortID = %q, want %q", sess.nasPortID, "lns1:1027.42")
	}

	// A reload after the session started must not move its NAS-Port-Id: the
	// billing system joins the records by that text.
	acct.setClient(client, "lns1", 300*time.Second, addr, nil, "changed-{tunnel-id}")

	pkt := acct.buildAcctPacket(sess, "lns1", nil, radius.AcctStatusInterimUpdate, 60)
	if got := net.IP(pkt.FindAttr(radius.AttrFramedIPAddress)).String(); got != "198.51.100.23" {
		t.Fatalf("Framed-IP-Address = %s, want 198.51.100.23", got)
	}
	if got := string(pkt.FindAttr(radius.AttrNASPortID)); got != "lns1:1027.42" {
		t.Fatalf("NAS-Port-Id = %q after a reload, want the text the session started with", got)
	}
}

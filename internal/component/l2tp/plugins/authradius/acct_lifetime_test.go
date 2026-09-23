package l2tpauthradius

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/radius"
)

// RFC requirement: RFC2866-5.5-2 positive -- IPv4 and IPv6 notifications for
// one network lifetime produce one Start and a Stop with the same session ID.
// RFC requirement: RFC2866-4.1-1 positive -- the Stop reports the negotiated
// IPv4 address regardless of which address family completed first.
func TestAccountingAddressFamiliesShareLifetime(t *testing.T) {
	for _, firstIPv6 := range []bool{false, true} {
		name := "IPv4-first"
		if firstIPv6 {
			name = "IPv6-first"
		}
		t.Run(name, func(t *testing.T) {
			sharedKey := []byte("network-lifetime")
			capture := newAcctCapture()
			conn, addr := startAcctServer(t, sharedKey, capture)
			defer conn.Close() //nolint:errcheck // test cleanup
			client, err := radius.NewClient(radius.ClientConfig{
				Servers: []radius.Server{{Address: addr, SharedKey: sharedKey}},
				Timeout: 2 * time.Second, Retries: 1,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close() //nolint:errcheck // test cleanup
			acct := newRADIUSAcct()
			acct.setClient(client, "lns1", 300*time.Second, addr, nil, "")
			defer acct.Stop()

			v4 := events.SessionIPAssignedPayload{
				TunnelID: 2029, SessionID: 48, Username: "alice", PeerAddr: "198.51.100.23",
			}
			v6 := events.SessionIPAssignedPayload{
				TunnelID: 2029, SessionID: 48, Username: "alice", InterfaceID: [8]byte{2, 0, 0, 0, 0, 0, 0, 8},
			}
			first, second := &v4, &v6
			if firstIPv6 {
				first, second = second, first
			}
			acct.onSessionIPAssigned(first)
			start := capture.waitN(t, 1)[0]
			if start.statusType != radius.AcctStatusStart || start.sessionID == "" {
				t.Fatalf("first record = %+v, want Accounting-Start with an identity", start)
			}
			acct.onSessionIPAssigned(second)
			acct.onSessionDown(&events.SessionDownPayload{TunnelID: 2029, SessionID: 48})
			packets := capture.waitN(t, 1)
			if len(packets) != 2 {
				t.Fatalf("one lifetime emitted %d records, want exactly Start and Stop: %+v", len(packets), packets)
			}
			stop := packets[1]
			if stop.statusType != radius.AcctStatusStop || stop.sessionID != start.sessionID || stop.peerAddr != v4.PeerAddr {
				t.Fatalf("Stop = %+v, want identity %q and IPv4 %s", stop, start.sessionID, v4.PeerAddr)
			}

			// The same transport can authenticate a replacement network lifetime.
			v4.Username = "bob"
			v4.PeerAddr = "198.51.100.24"
			acct.onSessionIPAssigned(&v4)
			packets = capture.waitN(t, 1)
			replacement := packets[len(packets)-1]
			if replacement.statusType != radius.AcctStatusStart || replacement.username != "bob" || replacement.peerAddr != v4.PeerAddr || replacement.sessionID == start.sessionID {
				t.Fatalf("replacement Start retained the old lifetime: %+v", replacement)
			}
			acct.onSessionDown(&events.SessionDownPayload{TunnelID: 2029, SessionID: 48})
			packets = capture.waitN(t, 1)
			stop = packets[len(packets)-1]
			if len(packets) != 4 || stop.statusType != radius.AcctStatusStop || stop.sessionID != replacement.sessionID {
				t.Fatalf("replacement lifetime did not emit its own Start/Stop pair: %+v", packets)
			}
		})
	}
}

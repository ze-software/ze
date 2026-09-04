package radius

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // RFC 2866 Section 3 mandates MD5
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"
)

// TestAcctDelayTimeUpdatesOnRetransmit reads the datagrams Exchange puts on the
// wire and asserts the three things RFC 2866 Section 4.1 asks of an accounting
// retransmission: "if Acct-Delay-Time is included in the attributes of an
// Accounting-Request then the Acct-Delay-Time value will be updated when the
// packet is retransmitted, changing the content of the Attributes field and
// requiring a new Identifier and Request Authenticator."
//
// The delay is the assertion the other tests cannot make.
// TestRFC2866AccountingRetransmitTakesANewIdentifier watches the Identifier
// alone, so it would pass against a client that moved the Identifier and left a
// stale delay behind, which is the osvbng shape: a constant that is true once
// and false on every attempt after it. RFC 2866 Section 5.2 defines the value as
// "how many seconds the client has been trying to send this record", so the
// retry timeout here is over one second: a sub-second retry would leave an
// honest client reporting zero twice and prove nothing either way.
//
// The authenticator is recomputed against an independent MD5 of the second
// datagram rather than against AccountingRequestAuth, so a re-encode that
// produced a well-formed but wrong authenticator fails here.
func TestAcctDelayTimeUpdatesOnRetransmit(t *testing.T) {
	secret := []byte("acct-secret")

	var mu sync.Mutex
	var datagrams [][]byte

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, MaxPacketLen)
		for {
			n, from, readErr := conn.ReadFromUDP(buf)
			if readErr != nil {
				return
			}
			seen := make([]byte, n)
			copy(seen, buf[:n])
			mu.Lock()
			datagrams = append(datagrams, seen)
			count := len(datagrams)
			mu.Unlock()
			if count == 1 {
				continue // drop the first datagram, so the client retransmits
			}
			var reqAuth [AuthenticatorLen]byte
			copy(reqAuth[:], seen[4:4+AuthenticatorLen])
			resp := make([]byte, HeaderLen)
			resp[0] = CodeAccountingResp
			resp[1] = seen[1]
			binary.BigEndian.PutUint16(resp[2:4], HeaderLen)
			auth := ResponseAuthenticator(CodeAccountingResp, seen[1], HeaderLen, reqAuth, nil, secret)
			copy(resp[4:4+AuthenticatorLen], auth[:])
			conn.WriteToUDP(resp, from) //nolint:errcheck // test mock best-effort
		}
	}()
	t.Cleanup(func() {
		closeSilent(conn)
		<-done
	})

	client, err := NewClient(ClientConfig{Timeout: 1100 * time.Millisecond, Retries: 3})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSilent(client)

	pkt := &Packet{
		Code:       CodeAccountingReq,
		Identifier: client.NextID(),
		Attrs: []Attr{
			{Type: AttrUserName, Value: AttrString("alice")},
			{Type: AttrAcctStatusType, Value: AttrUint32(AcctStatusStart)},
			{Type: AttrAcctSessionID, Value: AttrString("1-2-3")},
		},
	}
	if _, err = client.Exchange(context.Background(), pkt, secret, conn.LocalAddr().String()); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(datagrams) < 2 {
		t.Fatalf("the client sent %d datagram(s); the test proves nothing without a retransmission", len(datagrams))
	}
	first, second := datagrams[0], datagrams[1]

	firstDelay := wireAcctDelayTime(t, first)
	secondDelay := wireAcctDelayTime(t, second)
	if firstDelay != 0 {
		t.Errorf("the first attempt reported Acct-Delay-Time %d; nothing had been waited for yet", firstDelay)
	}
	if secondDelay < 1 {
		t.Errorf("the retransmission reported Acct-Delay-Time %d after a %s wait; RFC 2866 Section 5.2 counts "+
			"the seconds the client has been trying to send this record", secondDelay, 1100*time.Millisecond)
	}
	if first[1] == second[1] {
		t.Errorf("both attempts used Identifier %d; RFC 2866 Section 4.1 requires a new one when the delay moves", first[1])
	}

	// The authenticator is derived from the attributes, so it MUST move with
	// them, and it MUST be right about the bytes it was computed over.
	firstAuth := first[4 : 4+AuthenticatorLen]
	secondAuth := second[4 : 4+AuthenticatorLen]
	if bytes.Equal(firstAuth, secondAuth) {
		t.Error("both attempts carried the same Request Authenticator, which RFC 2866 Section 3 derives from the attributes")
	}
	if want := rfc2866RequestAuth(second, secret); !bytes.Equal(secondAuth, want) {
		t.Errorf("the retransmission's Request Authenticator is %x, and RFC 2866 Section 3 over its own bytes gives %x", secondAuth, want)
	}
}

// rfc2866RequestAuth is an independent implementation of the RFC 2866 Section 3
// formula: "MD5 hash calculated over a stream of octets consisting of the Code +
// Identifier + Length + 16 zero octets + request attributes + shared secret".
// It is written out here rather than calling AccountingRequestAuth, because a
// test that asks the producer what it produced asserts nothing.
func rfc2866RequestAuth(packet, secret []byte) []byte {
	hash := md5.New() //nolint:gosec // RFC 2866 Section 3 mandates MD5
	hash.Write(packet[:4])
	hash.Write(make([]byte, AuthenticatorLen))
	hash.Write(packet[HeaderLen:])
	hash.Write(secret)
	return hash.Sum(nil)
}

// wireAcctDelayTime reads Acct-Delay-Time off a datagram and fails the test when
// the attribute is absent, which is AC-7: ze stamps it on every attempt,
// including the first.
func wireAcctDelayTime(t *testing.T, packet []byte) uint32 {
	t.Helper()
	value, found := wireAttrValue(t, packet, AttrAcctDelayTime)
	if !found {
		t.Fatalf("the datagram carries no Acct-Delay-Time: %x", packet)
	}
	if len(value) != 4 {
		t.Fatalf("Acct-Delay-Time is %d octets; RFC 2866 Section 5.2 makes it four", len(value))
	}
	return binary.BigEndian.Uint32(value)
}

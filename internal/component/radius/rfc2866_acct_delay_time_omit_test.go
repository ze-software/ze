package radius

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"
)

// TestAccountingRequestOmitsAcctDelayTimeOnRequest proves that a record whose
// author held Acct-Delay-Time back reaches the server without the attribute,
// and that the same record with the field cleared carries it.
//
// RFC 2866 Section 5.13 gives Acct-Delay-Time a count of "0-1" in the Table of
// Attributes, so both records are conformant. The operator chooses between them
// through `l2tp auth radius attributes exclude acct-delay-time`
// (internal/component/l2tp/plugins/authradius).
func TestAccountingRequestOmitsAcctDelayTimeOnRequest(t *testing.T) {
	for _, test := range []struct {
		name    string
		omit    bool
		present bool
	}{
		{name: "the client stamps the attribute by default", omit: false, present: true},
		{name: "the record holds the attribute back", omit: true, present: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			sharedKey := []byte("testing123")
			received := make(chan []byte, 1)

			conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
			if err != nil {
				t.Fatal(err)
			}
			addr := conn.LocalAddr().String()
			done := make(chan struct{})
			go func() {
				defer close(done)
				buf := make([]byte, MaxPacketLen)
				n, from, readErr := conn.ReadFromUDP(buf)
				if readErr != nil {
					return
				}
				request := make([]byte, n)
				copy(request, buf[:n])
				received <- request
				conn.WriteToUDP(buildResponse(CodeAccountingResp, request, sharedKey), from) //nolint:errcheck // test mock
			}()
			defer func() {
				closeSilent(conn)
				<-done
			}()

			client, err := NewClient(ClientConfig{Timeout: 2 * time.Second, Retries: 1})
			if err != nil {
				t.Fatal(err)
			}
			defer closeSilent(client)

			pkt := &Packet{
				Code:       CodeAccountingReq,
				Identifier: client.NextID(),
				Attrs: []Attr{
					{Type: AttrAcctStatusType, Value: AttrUint32(AcctStatusStart)},
					{Type: AttrAcctSessionID, Value: AttrString("1-2-1")},
					{Type: AttrNASIdentifier, Value: AttrString("lns1")},
				},
				OmitAcctDelayTime: test.omit,
			}
			if _, err = client.Exchange(context.Background(), pkt, sharedKey, addr); err != nil {
				t.Fatal(err)
			}

			request := <-received
			if got := wireCarriesAttr(t, request, AttrAcctDelayTime); got != test.present {
				t.Errorf("Acct-Delay-Time on the wire: got %v, want %v", got, test.present)
			}
			if !wireCarriesAttr(t, request, AttrAcctSessionID) {
				t.Error("Acct-Session-Id left the wire, and this field names Acct-Delay-Time alone")
			}
		})
	}
}

// wireCarriesAttr walks the attributes of an encoded RADIUS packet and reports
// whether attrType is among them.
func wireCarriesAttr(t *testing.T, packet []byte, attrType uint8) bool {
	t.Helper()
	_, found := wireAttrValue(t, packet, attrType)
	return found
}

// wireAttrValue walks the attributes of an encoded RADIUS packet and returns the
// value octets of the first attrType it finds. The second result says whether
// the attribute was there at all, which is what an absence assertion needs.
func wireAttrValue(t *testing.T, packet []byte, attrType uint8) ([]byte, bool) {
	t.Helper()
	for off := HeaderLen; off < len(packet); {
		if off+2 > len(packet) {
			t.Fatalf("attribute header runs past the packet at offset %d", off)
		}
		length := int(packet[off+1])
		if length < 2 {
			t.Fatalf("attribute at offset %d declares length %d", off, length)
		}
		if off+length > len(packet) {
			t.Fatalf("attribute at offset %d declares length %d, which runs past the packet", off, length)
		}
		if packet[off] == attrType {
			return packet[off+2 : off+length], true
		}
		off += length
	}
	return nil, false
}

// RFC requirement: RFC2866-3-3 positive -- an Accounting-Request whose author held
// Acct-Delay-Time back is retransmitted byte for byte under its ORIGINAL Identifier.
// RFC 2866 Section 3 carries RFC 2865's rule: "For retransmissions where the contents
// are identical, the Identifier MUST remain unchanged." Section 4.1 makes the other
// half conditional: "if Acct-Delay-Time is included in the attributes of an
// Accounting-Request then the Acct-Delay-Time value will be updated when the packet is
// retransmitted ... requiring a new Identifier and Request Authenticator." An excluded
// Acct-Delay-Time leaves nothing to update, so the contents are identical and the
// Identifier MUST stay.
//
// This case became reachable on the accounting path when Packet.OmitAcctDelayTime
// landed. TestRFC2866AccountingRetransmitTakesANewIdentifier proves the other half,
// where the delay moves.
func TestRFC2866AccountingRetransmitWithoutDelayTimeKeepsIdentifier(t *testing.T) {
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

	client, err := NewClient(ClientConfig{Timeout: 150 * time.Millisecond, Retries: 3})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSilent(client)

	pkt := &Packet{
		Code:       CodeAccountingReq,
		Identifier: client.NextID(),
		Attrs: []Attr{
			{Type: AttrAcctStatusType, Value: AttrUint32(AcctStatusStart)},
			{Type: AttrAcctSessionID, Value: AttrString("1-2-1")},
			{Type: AttrNASIdentifier, Value: AttrString("lns1")},
		},
		OmitAcctDelayTime: true,
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
	if first[1] != second[1] {
		t.Errorf("the retransmission moved the Identifier from %d to %d, and its contents did not change", first[1], second[1])
	}
	if !bytes.Equal(first, second) {
		t.Errorf("the retransmission changed the datagram:\nfirst  %x\nsecond %x", first, second)
	}
	if wireCarriesAttr(t, second, AttrAcctDelayTime) {
		t.Error("the retransmission carries Acct-Delay-Time, which the record held back")
	}
}

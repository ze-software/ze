package radius

import (
	"context"
	"net"
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
	for off := HeaderLen; off < len(packet); {
		if off+2 > len(packet) {
			t.Fatalf("attribute header runs past the packet at offset %d", off)
		}
		length := int(packet[off+1])
		if length < 2 {
			t.Fatalf("attribute at offset %d declares length %d", off, length)
		}
		if packet[off] == attrType {
			return true
		}
		off += length
	}
	return false
}

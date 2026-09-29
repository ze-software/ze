package dhcpserver

import (
	"net"
	"testing"
)

// TestMessageTypeInEveryReply proves every message the server sends carries the
// DHCP message type option naming what the message is.
// VALIDATES: RFC 2131 Section 3 -- the "DHCP message type" option is included in
// every DHCP message; OFFER, ACK and NAK each carry option 53 with its own type.
// PREVENTS: a reply builder that omits option 53, which a client cannot classify.
func TestMessageTypeInEveryReply(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	offer, ack := exchange(t, h, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x31}, 0x3131)
	nak := nakForSelecting(t, h, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x32}, 0x3132)

	for _, tc := range []struct {
		name  string
		reply []byte
		want  byte
	}{{"OFFER", offer, msgOffer}, {"ACK", ack, msgAck}, {"NAK", nak, msgNak}} {
		got := getResponseOption(tc.reply, optMessageType)
		// RFC requirement: RFC2131-3-2 positive -- OFFER, ACK and NAK each carry the DHCP message type option (53), one octet holding that message's own type.
		if len(got) != 1 || got[0] != tc.want {
			t.Errorf("%s message type option = %v, want [%d]", tc.name, got, tc.want)
		}
	}
}

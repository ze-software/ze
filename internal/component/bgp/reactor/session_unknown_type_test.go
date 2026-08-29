// Related: session_handlers.go — handleUnknownType
//
// VALIDATES: an unrecognized BGP message type is refused with the NOTIFICATION
// RFC 4271 Section 6.1 prescribes — Message Header Error, subcode Bad Message
// Type, and the offending Type octet as Data.
// PREVENTS: the subcode-0-with-prose form, which carried a sentence a peer
// cannot parse and omitted the one octet the RFC requires.
package reactor

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// TestUnknownMessageTypeSendsBadMessageType reads the NOTIFICATION off the wire
// rather than asserting on the returned error. The error is for ze's own log;
// the bytes are what the peer is owed, and the Data field is the half of
// Section 6.1 the previous form did not carry at all.
func TestUnknownMessageTypeSendsBadMessageType(t *testing.T) {
	const unknown = msgtype.MessageType(99)

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	session := NewSession(settings)
	_ = session.Start()

	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	t.Cleanup(func() { _ = server.Close() })

	// acceptWithReader wires the session's buffered writer, which is what
	// writeMessageWithin uses. Setting s.conn alone leaves it nil.
	_ = acceptWithReader(t, session, server, client)

	// net.Pipe is unbuffered, so the peer end must be reading before the
	// handler writes.
	type read struct {
		buf []byte
		err error
	}
	got := make(chan read, 1)
	go func() {
		buf := make([]byte, 64)
		n, err := client.Read(buf)
		got <- read{buf: buf[:n], err: err}
	}()

	_ = session.handleUnknownType(unknown)

	var frame read
	select {
	case frame = <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("no NOTIFICATION reached the peer")
	}
	if frame.err != nil {
		t.Fatalf("reading the NOTIFICATION: %v", frame.err)
	}
	if len(frame.buf) <= message.HeaderLen {
		t.Fatalf("the peer received %d octets, too few to be a NOTIFICATION: % x",
			len(frame.buf), frame.buf)
	}

	notification, err := message.UnpackNotification(frame.buf[message.HeaderLen:])
	if err != nil {
		t.Fatalf("the bytes sent are not a NOTIFICATION: %v (% x)", err, frame.buf)
	}

	if notification.ErrorCode != message.NotifyMessageHeader {
		t.Errorf("error code = %d, want %d (Message Header Error)",
			notification.ErrorCode, message.NotifyMessageHeader)
	}
	if notification.ErrorSubcode != message.NotifyHeaderBadType {
		t.Errorf("error subcode = %d, want %d (Bad Message Type). RFC 4271 Section 6.1: "+
			"\"the Error Subcode MUST be set to Bad Message Type\"",
			notification.ErrorSubcode, message.NotifyHeaderBadType)
	}
	if len(notification.Data) != 1 || notification.Data[0] != byte(unknown) {
		t.Errorf("data = % x, want the single offending Type octet %02x. RFC 4271 "+
			"Section 6.1: \"The Data field MUST contain the erroneous Type field\"",
			notification.Data, byte(unknown))
	}
}

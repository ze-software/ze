// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Related: dhcp_v6_exchange_linux.go -- nclient6Link, the production link

//go:build linux

package ifacedhcp

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/nclient6"

	"github.com/ze-software/ze/internal/core/clock"
)

// recordingPacketConn is a net.PacketConn that records every datagram the
// client writes and hands the client the datagrams a test injects.
type recordingPacketConn struct {
	written chan []byte
	inject  chan []byte
	closed  chan struct{}
	once    sync.Once
}

func newRecordingPacketConn() *recordingPacketConn {
	return &recordingPacketConn{
		written: make(chan []byte, 16),
		inject:  make(chan []byte, 16),
		closed:  make(chan struct{}),
	}
}

func (c *recordingPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	select {
	case p := <-c.inject:
		return copy(b, p), &net.UDPAddr{IP: net.ParseIP("fe80::1"), Port: 547}, nil
	case <-c.closed:
		return 0, nil, errors.New("use of closed network connection")
	}
}

func (c *recordingPacketConn) WriteTo(b []byte, _ net.Addr) (int, error) {
	c.written <- append([]byte(nil), b...)
	return len(b), nil
}

func (c *recordingPacketConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}

func (c *recordingPacketConn) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("fe80::2"), Port: 546}
}
func (c *recordingPacketConn) SetDeadline(time.Time) error      { return nil }
func (c *recordingPacketConn) SetReadDeadline(time.Time) error  { return nil }
func (c *recordingPacketConn) SetWriteDeadline(time.Time) error { return nil }

// newRecordingLink builds the production link over a recording connection,
// with the options dialV6Link passes (v6ClientOptions).
func newRecordingLink(t *testing.T) (nclient6Link, *recordingPacketConn) {
	t.Helper()
	conn := newRecordingPacketConn()
	mac := net.HardwareAddr{0x02, 0, 0, 0, 0, 0x01}
	client, err := nclient6.NewWithConn(conn, mac, v6ClientOptions()...)
	if err != nil {
		t.Fatalf("NewWithConn: %v", err)
	}
	link := nclient6Link{client: client}
	t.Cleanup(func() {
		if err := link.close(); err != nil {
			t.Logf("close: %v", err)
		}
	})
	return link, conn
}

func newTestSolicit(t *testing.T) *dhcpv6.Message {
	t.Helper()
	msg, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	msg.MessageType = dhcpv6.MessageTypeSolicit
	return msg
}

func answerTo(msg *dhcpv6.Message, kind dhcpv6.MessageType) []byte {
	answer := &dhcpv6.Message{MessageType: kind, TransactionID: msg.TransactionID}
	return answer.ToBytes()
}

// TestDHCPv6LinkOneDatagramPerExchange validates spec assumptions A-2 and
// A-3 against the vendored client the production link wraps.
//
// Method: the link runs over a recording PacketConn. A-2: one exchange
// writes exactly one datagram, carrying the message's transaction ID, both
// when an answer arrives and when the window closes unanswered. A-3: a
// matcher that refuses an answer keeps the exchange listening, so the
// Advertise collector can see a later message in the same window.
func TestDHCPv6LinkOneDatagramPerExchange(t *testing.T) {
	t.Run("answered after a refused message", func(t *testing.T) {
		link, conn := newRecordingLink(t)
		msg := newTestSolicit(t)
		var seen []dhcpv6.MessageType
		accept := func(m *dhcpv6.Message) bool {
			seen = append(seen, m.MessageType)
			return m.MessageType == dhcpv6.MessageTypeReply
		}

		done := make(chan struct{})
		var reply *dhcpv6.Message
		var err error
		go func() {
			defer close(done)
			reply, err = link.exchange(context.Background(), clock.RealClock{}, 5*time.Second, msg, accept)
		}()

		sent := <-conn.written
		conn.inject <- answerTo(msg, dhcpv6.MessageTypeAdvertise)
		conn.inject <- answerTo(msg, dhcpv6.MessageTypeReply)
		<-done

		if err != nil {
			t.Fatalf("exchange: %v", err)
		}
		if reply.MessageType != dhcpv6.MessageTypeReply {
			t.Fatalf("answer = %v, want REPLY", reply.MessageType)
		}
		if len(seen) != 2 || seen[0] != dhcpv6.MessageTypeAdvertise {
			t.Fatalf("matcher saw %v, want ADVERTISE refused then REPLY", seen)
		}
		assertOneDatagram(t, conn, sent, msg)
	})

	t.Run("unanswered window", func(t *testing.T) {
		link, conn := newRecordingLink(t)
		msg := newTestSolicit(t)
		_, err := link.exchange(context.Background(), clock.RealClock{}, 200*time.Millisecond, msg, func(*dhcpv6.Message) bool { return true })
		if !errors.Is(err, errV6NoAnswer) {
			t.Fatalf("exchange error = %v, want errV6NoAnswer", err)
		}
		assertOneDatagram(t, conn, <-conn.written, msg)
	})
}

func assertOneDatagram(t *testing.T, conn *recordingPacketConn, sent []byte, msg *dhcpv6.Message) {
	t.Helper()
	decoded, err := dhcpv6.MessageFromBytes(sent)
	if err != nil {
		t.Fatalf("decode sent datagram: %v", err)
	}
	if decoded.TransactionID != msg.TransactionID {
		t.Fatalf("sent xid %v, want %v", decoded.TransactionID, msg.TransactionID)
	}
	select {
	case extra := <-conn.written:
		t.Fatalf("a second datagram was written (%d bytes)", len(extra))
	default: // exactly one datagram: the assumption holds
	}
}

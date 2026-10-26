//go:build linux

// RFC 4861 receiver obligations on the Router Solicitations a PPP subscriber
// session reads.
//
// VALIDATES: rsReaderLoop reports a solicitation whatever its Reserved field
// holds and whatever options it carries, in either order, including none, an
// unrecognized one, and one defined only for another message (Sections 4.1,
// 6.1.1 and 9).
// PREVENTS: a subscriber whose solicitation carries an option getting no
// answer until the next unsolicited advertisement.
//
// The reader only calls pc.ReadFrom, so a UDP socket wrapped in
// ipv6.NewPacketConn carries the ICMPv6 body with no raw-socket privilege.
// Like rsReaderLoop itself, it carries no IPv6 header, so this file says
// nothing about the Hop Limit check RFC4861-6.1.1-2 records as a gap.

package ppp

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"golang.org/x/net/ipv6"
)

// rfc4861SolicitationBodies are Router Solicitation bodies (type 133, code 0,
// checksum, Reserved, options), each of which a router must still answer.
var rfc4861SolicitationBodies = []struct {
	name string
	body []byte
}{
	{"no options", []byte{133, 0, 0, 0, 0, 0, 0, 0}},
	{"non-zero Reserved field", []byte{133, 0, 0, 0, 0xde, 0xad, 0xbe, 0xef}},
	{"unrecognized option then Source Link-layer Address", []byte{
		133, 0, 0, 0, 0, 0, 0, 0,
		200, 1, 1, 2, 3, 4, 5, 6,
		1, 1, 0x02, 0, 0, 0, 0, 0x01,
	}},
	{"Source Link-layer Address then unrecognized option", []byte{
		133, 0, 0, 0, 0, 0, 0, 0,
		1, 1, 0x02, 0, 0, 0, 0, 0x01,
		200, 1, 1, 2, 3, 4, 5, 6,
	}},
	{"Prefix Information option, defined for Router Advertisements only", []byte{
		133, 0, 0, 0, 0, 0, 0, 0,
		3, 4, 64, 0xc0, 0, 0, 0x0e, 0x10, 0, 0, 0x07, 0x08, 0, 0, 0, 0,
		0x20, 0x01, 0x0d, 0xb8, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}},
}

// RFC requirement: RFC4861-4.1-1 positive -- a solicitation whose Reserved field holds 0xdeadbeef is still reported by rsReaderLoop.
// RFC requirement: RFC4861-4.1-3 positive -- a solicitation carrying the unrecognized option type 200 is still reported.
// RFC requirement: RFC4861-6.1.1-3 positive -- neither a non-zero Reserved field nor an unrecognized option stops a solicitation being reported.
// RFC requirement: RFC4861-6.1.1-4 positive -- a solicitation carrying a Prefix Information option, which is defined for Router Advertisements only, is still reported.
// RFC requirement: RFC4861-9-1 positive -- a solicitation carrying the unrecognized option type 200 is still reported.
// RFC requirement: RFC4861-9-3 negative -- a solicitation carrying a Source Link-layer Address option beside an unrecognized one is reported, as one without the unrecognized option is.
// RFC requirement: RFC4861-9-4 negative -- a solicitation carrying no option at all is reported, as one carrying options is.
// RFC requirement: RFC4861-9-5 positive -- a solicitation is reported with its Source Link-layer Address option before an unrecognized option and after it.
// RFC requirement: RFC4861-9-8 positive -- a solicitation carrying the unrecognized option type 200 is still reported.
func TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	listener, err := (&net.ListenConfig{}).ListenPacket(ctx, "udp6", "[::1]:0")
	if err != nil {
		cancel()
		t.Skipf("no IPv6 loopback: %v", err)
	}
	defer listener.Close() //nolint:errcheck // test cleanup
	sender, err := (&net.Dialer{}).DialContext(ctx, "udp6", listener.LocalAddr().String())
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	defer sender.Close() //nolint:errcheck // test cleanup

	done := make(chan struct{})
	reports := make(chan struct{}, 1)
	go func() {
		defer close(done)
		rsReaderLoop(ctx, ipv6.NewPacketConn(listener), reports, "ppp0", slog.New(slog.DiscardHandler))
	}()
	defer func() {
		cancel()
		_ = listener.SetReadDeadline(time.Now()) //nolint:errcheck // unblocks the reader for shutdown
		<-done
	}()

	for _, tc := range rfc4861SolicitationBodies {
		if _, err := sender.Write(tc.body); err != nil {
			t.Fatalf("%s: send: %v", tc.name, err)
		}
		select {
		case <-reports:
		case <-time.After(2 * time.Second):
			t.Fatalf("%s: the solicitation was not reported", tc.name)
		}
	}
}

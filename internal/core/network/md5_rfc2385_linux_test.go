//go:build linux

package network

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// RFC 2385 is met through the kernel: ze installs the key with TCP_MD5SIG and
// Linux signs and validates every segment. The tests below therefore drive the
// boundary ze owns -- the key ze installs, on the sockets ze opens, before the
// handshake -- and read the answer the whole stack gives back on loopback.

// rfc2385Key is the key both ends share in the matching-key case.
const rfc2385Key = "rfc2385-loopback-key"

// rfc2385PayloadOctets is larger than one loopback segment, so a transfer that
// arrives whole has crossed many signed segments rather than one.
const rfc2385PayloadOctets = 256 * 1024

// rfc2385DialTimeout bounds a dial the peer's kernel is expected to drop. Two
// seconds covers the first SYN retransmission on Linux and keeps a refused
// case, which answers at once, distinguishable from a dropped one.
const rfc2385DialTimeout = 2 * time.Second

// rfc2385Listener opens a loopback listener carrying one TCP_MD5SIG key for
// 127.0.0.1. A kernel built without CONFIG_TCP_MD5SIG refuses the socket
// option, and the test skips rather than reporting ze's own plumbing broken.
func rfc2385Listener(t *testing.T, key string) net.Listener {
	t.Helper()
	factory := RealListenerFactory{MD5Peers: []MD5Peer{{Addr: net.IPv4(127, 0, 0, 1), Key: key}}}
	ln, err := factory.Listen(context.Background(), "tcp4", "127.0.0.1:0")
	if err == nil {
		return ln
	}
	if errors.Is(err, unix.ENOPROTOOPT) || errors.Is(err, unix.EOPNOTSUPP) {
		t.Skipf("this kernel has no TCP_MD5SIG: %v", err)
	}
	t.Fatalf("listen with TCP_MD5SIG: %v", err)
	return nil
}

// rfc2385DialDropped dials a listener whose kernel is expected to drop the SYN,
// and reports the error the dial ended with.
func rfc2385DialDropped(t *testing.T, dialer *RealDialer, address string) error {
	t.Helper()
	conn, err := dialer.DialContext(context.Background(), "tcp4", address)
	if err == nil {
		closeOrLog(t, conn)
		t.Fatal("the dial succeeded; a segment whose MD5 digest does not verify must be dropped")
	}
	return err
}

// TestRFC2385MatchingKeysCarryASignedSession is the positive case: one key
// installed on both sockets, a session that establishes, and a payload larger
// than one segment that arrives whole.
//
// A payload crossing a keyed connection would also cross an unkeyed one, so
// this test alone cannot tell a signing socket from a silent no-op. The two
// tests below it are what supply that: each drives the same producer and goes
// red when nothing is signed.
//
// VALIDATES: the key ze installs before connect and before bind is the one the
// kernel signs with, and a signed connection carries data end to end.
// PREVENTS: TCP_MD5SIG being set on a socket that never signs, which looks
// identical to a working configuration until a peer refuses the session.
// RFC requirement: RFC2385-2.0-1 positive -- the same operator key installed for
// the peer on the listening socket and on the dialing socket carries a session:
// both ends of the connection know the key.
// RFC requirement: RFC2385-2.0-2 positive -- the receiver validates a segment it
// can verify against its own key and accepts it, so the handshake completes and
// 256 KiB crosses intact.
// RFC requirement: RFC2385-2.0-3 negative -- a segment whose digest DOES verify
// is not dropped: the payload arrives whole rather than being discarded.
// RFC requirement: RFC2385-2.0-6 positive -- every segment of this connection
// carried a digest the peer's kernel recomputed and accepted, over the same
// pseudo-header, header, data and key.
// RFC requirement: RFC2385-3.0-1 positive -- the option ze's key produces is the
// one the peer's TCP accepts, so its Kind, its length and its 16-octet digest
// are the encoding this document fixes.
// RFC requirement: RFC2385-4.3-1 positive -- 256 KiB crosses a signed
// connection, so the MSS offered at setup left room for the option rather than
// producing segments the path could not carry.
// RFC requirement: RFC2385-4.3-2 positive -- the handshake completes with the
// option present, so the header and its options fit the 60 octets TCP allows.
func TestRFC2385MatchingKeysCarryASignedSession(t *testing.T) {
	ln := rfc2385Listener(t, rfc2385Key)
	defer closeOrLog(t, ln)

	received := make(chan int64, 1)
	failed := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			failed <- err
			return
		}
		defer func() { _ = conn.Close() }()
		octets, err := io.Copy(io.Discard, conn)
		if err != nil {
			failed <- err
			return
		}
		received <- octets
	}()

	dialer := &RealDialer{
		PeerAddr: net.IPv4(127, 0, 0, 1),
		MD5Key:   rfc2385Key,
		Timeout:  5 * time.Second,
	}
	conn, err := dialer.DialContext(context.Background(), "tcp4", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial with a matching MD5 key: %v", err)
	}

	if _, err := conn.Write(bytes.Repeat([]byte{'z'}, rfc2385PayloadOctets)); err != nil {
		closeOrLog(t, conn)
		t.Fatalf("write over a signed connection: %v", err)
	}
	closeOrLog(t, conn)

	select {
	case octets := <-received:
		if octets != rfc2385PayloadOctets {
			t.Errorf("received %d octet(s) over the signed connection, want %d",
				octets, rfc2385PayloadOctets)
		}
	case err := <-failed:
		t.Fatalf("accept or read on the signed connection: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("the signed connection carried no payload within 10s")
	}
}

// TestRFC2385MismatchedKeyIsDroppedWithNoResponse is the negative case: two
// different keys, so the digest the listener computes cannot match the one the
// dialer sent.
//
// VALIDATES: a segment whose digest does not verify is dropped and answered
// with nothing, which reaches the dialer as a timeout rather than a refusal.
// PREVENTS: a session established under a key the peer does not hold, which is
// the whole protection this option exists to give.
// RFC requirement: RFC2385-2.0-2 negative -- the receiver refuses a segment
// whose digest does not match the one its own key produces, so no session is
// established under a key the two ends do not share.
// RFC requirement: RFC2385-2.0-3 positive -- the failing comparison drops the
// segment and produces NO response: the dial ends in a timeout, and neither a
// refusal nor a reset comes back.
// RFC requirement: RFC2385-2.0-6 negative -- a digest computed over a different
// key is not the digest this section demands, and the segment carrying it is
// refused.
// RFC requirement: RFC2385-3.0-1 negative -- a well-formed option carrying the
// wrong digest is refused, so acceptance rests on the digest and not on the
// option merely being present.
func TestRFC2385MismatchedKeyIsDroppedWithNoResponse(t *testing.T) {
	ln := rfc2385Listener(t, "rfc2385-listener-key")
	defer closeOrLog(t, ln)

	dialer := &RealDialer{
		PeerAddr: net.IPv4(127, 0, 0, 1),
		MD5Key:   "rfc2385-dialer-key",
		Timeout:  rfc2385DialTimeout,
	}
	err := rfc2385DialDropped(t, dialer, ln.Addr().String())

	if errors.Is(err, unix.ECONNREFUSED) || errors.Is(err, unix.ECONNRESET) {
		t.Fatalf("the dial was answered (%v); a failing comparison must produce no response", err)
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("the dial ended with %v, want a timeout: the segment is dropped in silence", err)
	}
}

// TestRFC2385KeyOnOneEndOnlyCarriesNoSession is the negative case for the
// shared key: the listener holds one and the dialer holds none.
//
// VALIDATES: a key one end does not hold establishes nothing, so the option is
// not a hint the peer may ignore.
// PREVENTS: an unsigned peer reaching a session ze protects with a key, which
// would leave the protection on paper only.
// RFC requirement: RFC2385-2.0-1 negative -- a key known to only one end of the
// connection carries no session: the unsigned dial is dropped in silence.
func TestRFC2385KeyOnOneEndOnlyCarriesNoSession(t *testing.T) {
	ln := rfc2385Listener(t, rfc2385Key)
	defer closeOrLog(t, ln)

	dialer := &RealDialer{Timeout: rfc2385DialTimeout}
	err := rfc2385DialDropped(t, dialer, ln.Addr().String())

	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("the unsigned dial ended with %v, want a timeout: it is dropped in silence", err)
	}
}

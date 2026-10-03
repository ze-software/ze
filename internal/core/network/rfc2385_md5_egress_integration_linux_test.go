//go:build integration && linux

// Design: rfc/short/rfc2385.md -- configured TCP MD5 packet evidence.
// Related: ttl_gtsm_egress_integration_linux_test.go -- isolated veth egress.
package network

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // RFC 2385 fixes the wire algorithm to MD5.
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// TestRFC2385CapturedSegmentsHaveIndependentDigests checks both socket producers,
// the entire handshake, data in both directions, and the orderly FIN exchange.
// RFC 2385 Section 2.0: "Every segment sent on a TCP connection to be protected
// against spoofing will contain the 16-byte MD5 digest produced by applying the
// MD5 algorithm to these items in the following order:".
// RFC requirement: RFC2385-2.0-6 positive -- RealDialer and RealListenerFactory install the application key through setTCPMD5Sig; every captured SYN, SYNACK, ACK, data and FIN has the independently computed digest over pseudo-header, zero-checksum fixed header, data and key in that order.
// RFC requirement: RFC2385-3.0-1 positive -- every captured segment in both directions carries exactly one Kind 19 Length 18 option with a 16-octet digest, including handshake, data, ACK and FIN packets.
func TestRFC2385CapturedSegmentsHaveIndependentDigests(t *testing.T) {
	path := newRFC2385Egress(t, rfc2385Key)
	dialer := RealDialer{LocalAddr: &net.TCPAddr{IP: net.IPv4(192, 0, 2, 1)}, PeerAddr: net.IPv4(192, 0, 2, 2), MD5Key: rfc2385Key, Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(context.Background(), "tcp4", path.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer closeOrLog(t, conn)
	accepted, err := path.listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer closeOrLog(t, accepted)
	for _, endpoint := range []net.Conn{conn, accepted} {
		if err := endpoint.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	payload := bytes.Repeat([]byte("md5-wire-payload!"), 256)
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}
	var received [len("md5-wire-payload!") * 256]byte
	if _, err := io.ReadFull(accepted, received[:]); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(received[:], payload) {
		t.Fatal("signed payload changed")
	}
	if _, err := accepted.Write([]byte("reply")); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(conn, received[:5]); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []net.Conn{conn, accepted} {
		tcp, ok := endpoint.(*net.TCPConn)
		if !ok {
			t.Fatal("production connection is not TCP")
		}
		if err := tcp.CloseWrite(); err != nil {
			t.Fatal(err)
		}
	}
	var seen [2]struct{ syn, ack, data, fin, finAck bool }
	var finEnd [2]uint32
	var dataOctets [2]int
	for range 64 {
		packet := path.receive(t)
		tcp := packet[int(packet[0]&15)*4:]
		// RFC 2385 Sections 2.0 and 3.0: read the option, then recompute it.
		assertRFC2385Digest(t, packet, rfc2385Key)
		direction := 0
		if packet[15] == 2 {
			direction = 1
		}
		seen[direction].syn = seen[direction].syn || tcp[13]&2 != 0
		seen[direction].ack = seen[direction].ack || tcp[13] == 0x10
		data := len(tcp) - int(tcp[12]>>4)*4
		seen[direction].data = seen[direction].data || data != 0
		dataOctets[direction] += data
		if tcp[13]&1 != 0 {
			seen[direction].fin = true
			finEnd[direction] = binary.BigEndian.Uint32(tcp[4:8]) + uint32(data) + 1
		}
		if seen[1-direction].fin && tcp[13]&0x10 != 0 {
			if binary.BigEndian.Uint32(tcp[8:12]) == finEnd[1-direction] {
				seen[direction].finAck = true
			}
		}
		if len(packet) > 1500 {
			t.Fatalf("signed packet exceeds configured MTU: %d", len(packet))
		}
		if tcp[13]&2 != 0 {
			mss := rfc2385Option(t, tcp, 2)
			// RFC 6691 Section 3.2 corrects RFC 2385 Section 4.3: options
			// reduce data length, not the advertised MSS (1500 - 20 - 20).
			if !bytes.Equal(mss, []byte{2, 4, 5, 180}) {
				t.Fatalf("MSS option = %x, want 020405b4 (1460)", mss)
			}
		}
		t.Logf("wire direction=%d flags=%02x ip=%d tcp=%d data=%d options=%x", direction, tcp[13], len(packet), int(tcp[12]>>4)*4, data, tcp[20:int(tcp[12]>>4)*4])
		if seen[0].syn && seen[1].syn && seen[0].ack && seen[1].ack && seen[0].data && seen[1].data && seen[0].fin && seen[1].fin && seen[0].finAck && seen[1].finAck {
			if dataOctets != [2]int{len(payload), 5} {
				t.Fatalf("capture lost data: %v", dataOctets)
			}
			return
		}
	}
	t.Fatalf("incomplete signed lifecycle: %+v", seen)
}

// TestRFC2385UnsignedSYNACKDoesNotDisableSignatures injects a correctly checksummed
// unsigned SYNACK acknowledging the observed SYN, then observes its retransmission.
// RFC 2385 Section 2.0: "the absence of the option in the SYN,ACK segment must not
// cause the sender to disable its sending of signatures."
// RFC requirement: RFC2385-2.0-4 positive -- after a valid unsigned SYNACK reaches RealDialer's keyed socket, the same connection retransmits its SYN with an independently verified signature and never completes unsigned.
// RFC requirement: RFC2385-2.0-5 positive -- the application key installed by RealDialer remains authoritative after the remote endpoint sends an unsigned SYNACK: the retransmitted SYN is still signed and the dial times out instead of downgrading.
func TestRFC2385UnsignedSYNACKDoesNotDisableSignatures(t *testing.T) {
	path := newRFC2385Egress(t, "")
	injector := rfc2385PeerSocket(t, path.capture, 0)
	link, err := netlink.LinkByName("gtsm-send")
	if err != nil {
		t.Fatal(err)
	}
	var destination [8]byte
	copy(destination[:], link.Attrs().HardwareAddr)
	observed := make(chan struct{})
	// The observer owns only the peer-namespace packet socket, not new sockets.
	// The caller MUST join it before newTCPEgress cleanup closes that socket.
	go rfc2385ObserveUnsignedSYNACK(t, path, injector, destination, observed)
	dialer := RealDialer{LocalAddr: &net.TCPAddr{IP: net.IPv4(192, 0, 2, 1)}, PeerAddr: net.IPv4(192, 0, 2, 2), MD5Key: rfc2385Key, Timeout: 2500 * time.Millisecond}
	conn, err := dialer.DialContext(context.Background(), "tcp4", path.listener.Addr().String())
	if conn != nil {
		closeOrLog(t, conn)
	}
	<-observed
	var timeout net.Error
	if !errors.As(err, &timeout) {
		t.Fatalf("unsigned SYNACK dial result = %v, want timeout", err)
	}
	if !timeout.Timeout() {
		t.Fatalf("unsigned SYNACK dial result = %v, want timeout", err)
	}
}

// rfc2385ObserveUnsignedSYNACK MUST close done so the caller joins before cleanup.
func rfc2385ObserveUnsignedSYNACK(t *testing.T, path *tcpEgress, injector int, destination [8]byte, done chan<- struct{}) {
	defer close(done)
	syn := path.receive(t)
	tcp := syn[int(syn[0]&15)*4:]
	if tcp[13] != 2 {
		t.Fatalf("first packet flags = %02x, want SYN", tcp[13])
	}
	// RFC 2385 Section 2.0: the initial SYN proves signing was configured.
	assertRFC2385Digest(t, syn, rfc2385Key)
	response := rfc2385Packet(binary.BigEndian.Uint16(tcp[2:4]), binary.BigEndian.Uint16(tcp[:2]), nil)
	copy(response[12:16], syn[16:20])
	copy(response[16:20], syn[12:16])
	response[33] = 0x12
	binary.BigEndian.PutUint32(response[28:32], binary.BigEndian.Uint32(tcp[4:8])+1)
	rfc2385Checksums(response)
	protocol := binary.NativeEndian.Uint16([]byte{8, 0})
	if err := unix.Sendto(injector, response, 0, &unix.SockaddrLinklayer{Ifindex: path.peerLink, Protocol: protocol, Halen: 6, Addr: destination}); err != nil {
		t.Fatal(err)
	}
	// Seeing our actual outgoing SYNACK on AF_PACKET distinguishes injection
	// from constructing a byte slice that never reached the sender.
	seenSYNACK := false
	for range 16 {
		packet := path.receive(t)
		segment := packet[int(packet[0]&15)*4:]
		if packet[15] == 2 {
			if bytes.Equal(packet, response) {
				seenSYNACK = true
				t.Logf("injected unsigned SYNACK=%x", packet)
			}
			continue
		}
		if !seenSYNACK {
			t.Fatal("sender answered before the injected SYNACK was observed")
		}
		if segment[13] != 2 {
			t.Fatalf("unsigned SYNACK changed sender state: flags=%02x", segment[13])
		}
		if !bytes.Equal(segment[:8], tcp[:8]) {
			t.Fatal("not a retransmission of the same connection's SYN")
		}
		// RFC 2385 Section 2.0: signing survives the remote's unsigned answer.
		assertRFC2385Digest(t, packet, rfc2385Key)
		t.Logf("signed SYN retransmission=%x", packet)
		return
	}
	t.Fatal("no signed SYN retransmission after unsigned SYNACK")
}

// newRFC2385Egress reuses the configured egress and adds a bidirectional capture.
// ETH_P_ALL is needed for the peer's outgoing packets; ETH_P_IP taps ingress only.
func newRFC2385Egress(t *testing.T, key string) *tcpEgress {
	t.Helper()
	path := newTCPEgress(t, key)
	protocol := int(binary.NativeEndian.Uint16([]byte{0, 3}))
	path.capture = rfc2385PeerSocket(t, path.capture, protocol)
	if err := unix.Bind(path.capture, &unix.SockaddrLinklayer{Protocol: uint16(protocol), Ifindex: path.peerLink}); err != nil {
		t.Fatal(err)
	}
	if err := unix.SetsockoptTimeval(path.capture, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 5}); err != nil {
		t.Fatal(err)
	}
	return path
}

// rfc2385PeerSocket creates a separate packet socket in the capture's namespace.
// AF_PACKET does not loop transmitted packets back to their originating socket.
func rfc2385PeerSocket(t *testing.T, capture, protocol int) int {
	t.Helper()
	sender, err := netns.Get()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := netns.Set(sender); err != nil {
			t.Fatal(err)
		}
		if err := sender.Close(); err != nil {
			t.Error(err)
		}
	}()
	peer, err := unix.IoctlRetInt(capture, unix.SIOCGSKNS)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Close(peer); err != nil {
			t.Error(err)
		}
	}()
	if err := netns.Set(netns.NsHandle(peer)); err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM, protocol)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	})
	return fd
}

// TestRFC2385UnconfiguredSocketSendsNoSignature observes the application opt-out,
// not a refusal path: an ordinary TCP session has no MD5 option on either SYN.
// RFC requirement: RFC2385-2.0-5 negative -- with no application key RealDialer and RealListenerFactory send unsigned SYN and SYNACK; the remote host does not turn signing on by itself.
func TestRFC2385UnconfiguredSocketSendsNoSignature(t *testing.T) {
	path := newRFC2385Egress(t, "")
	dialer := RealDialer{LocalAddr: &net.TCPAddr{IP: net.IPv4(192, 0, 2, 1)}, Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(context.Background(), "tcp4", path.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer closeOrLog(t, conn)
	var syns int
	for range 8 {
		packet := path.receive(t)
		tcp := packet[int(packet[0]&15)*4:]
		if rfc2385Option(t, tcp, 19) != nil {
			t.Fatal("application configured no key but socket emitted MD5")
		}
		if tcp[13]&2 != 0 {
			syns++
		}
		if syns == 2 {
			return
		}
	}
	t.Fatal("unsigned handshake was not observed")
}

// TestRFC2385MalformedWireSignaturesAreDropped contrasts each invalid SYN with
// a well-formed independently signed SYN to the same Ze-configured listener.
// RFC requirement: RFC2385-2.0-3 positive -- direct packet capture observes no reply to a SYN with an invalid digest; a separately signed valid control reaches the same configured listener and receives a signed SYNACK.
// RFC requirement: RFC2385-2.0-6 negative -- the listener configured by RealListenerFactory drops a SYN whose otherwise valid Kind 19 option has a corrupted digest, while an independently signed SYN with the exact pseudo-header/header/data/key composition receives a signed SYNACK.
// RFC requirement: RFC2385-3.0-1 negative -- the configured listener sends no response to a SYN missing Kind 19, using a different Kind, or carrying Length 17 instead of 18; a valid Kind 19 Length 18 control SYN receives a signed SYNACK.
func TestRFC2385MalformedWireSignaturesAreDropped(t *testing.T) {
	path := newRFC2385Egress(t, rfc2385Key)
	listener, ok := path.listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("listener address is not TCP")
	}
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, unix.IPPROTO_RAW)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	})
	for index, name := range []string{"digest", "missing", "kind", "length"} {
		t.Run(name, func(t *testing.T) {
			port := uint16(42000 + index*2)
			option := make([]byte, 20)
			option[0], option[1] = 19, 18
			packet := rfc2385Packet(port, uint16(listener.Port), option)
			digest := rfc2385Digest(packet, rfc2385Key)
			copy(packet[42:58], digest[:])
			switch name {
			case "digest":
				packet[42] ^= 0x80
			case "missing":
				packet = rfc2385Packet(port, uint16(listener.Port), nil)
			case "kind":
				packet[40] = 30
			case "length":
				packet[41] = 17
			}
			rfc2385Checksums(packet)
			if err := unix.Sendto(fd, packet, 0, &unix.SockaddrInet4{Addr: [4]byte{192, 0, 2, 2}}); err != nil {
				t.Fatal(err)
			}
			// Queue unrelated traffic after the SYN: readiness for a non-TCP
			// frame must not turn the bounded silence check into a blocking read.
			noise := rfc2385Packet(port, uint16(listener.Port), nil)
			noise[9] = unix.IPPROTO_UDP
			binary.BigEndian.PutUint16(noise[24:26], uint16(len(noise)-20))
			noise[26], noise[27] = 0, 0
			binary.BigEndian.PutUint16(noise[10:12], rfc2385Checksum(noise[:20]))
			if err := unix.Sendto(fd, noise, 0, &unix.SockaddrInet4{Addr: [4]byte{192, 0, 2, 2}}); err != nil {
				t.Fatal(err)
			}
			rfc2385ExpectReply(t, path, port, false)
			control := rfc2385Packet(port+1, uint16(listener.Port), option)
			digest = rfc2385Digest(control, rfc2385Key)
			copy(control[42:58], digest[:])
			rfc2385Checksums(control)
			if err := unix.Sendto(fd, control, 0, &unix.SockaddrInet4{Addr: [4]byte{192, 0, 2, 2}}); err != nil {
				t.Fatal(err)
			}
			rfc2385ExpectReply(t, path, port+1, true)
		})
	}
}

// rfc2385ExpectReply bounds silence and proves a valid control reaches the listener.
func rfc2385ExpectReply(t *testing.T, path *tcpEgress, port uint16, want bool) {
	t.Helper()
	deadline := time.Now().Add(300 * time.Millisecond)
	var buffer [1600]byte
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		poll := []unix.PollFd{{Fd: int32(path.capture), Events: unix.POLLIN}}
		n, err := unix.Poll(poll, int(remaining.Milliseconds())+1)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
		packet, err := path.receiveFrame(t, buffer[:], unix.MSG_DONTWAIT)
		if err == unix.EAGAIN || err == unix.EINTR {
			continue
		}
		if err != nil {
			t.Fatalf("capture configured egress: %v", err)
		}
		if packet == nil || packet[15] != 2 {
			continue
		}
		tcp := packet[int(packet[0]&15)*4:]
		if binary.BigEndian.Uint16(tcp[2:4]) != port {
			continue
		}
		if !want {
			t.Fatalf("malformed MD5 SYN received a response: %x", packet)
		}
		if tcp[13] != 0x12 {
			t.Fatalf("valid signed control received flags=%02x, want SYNACK", tcp[13])
		}
		// RFC 2385 Section 2.0: the positive control uses an independent digest.
		assertRFC2385Digest(t, packet, rfc2385Key)
		t.Logf("valid independently signed SYN received SYNACK=%x", packet)
		return
	}
	if want {
		t.Fatal("valid independently signed control received no SYNACK")
	}
	t.Log("invalid signature produced no response during 300ms observation")
}

// rfc2385Packet builds an IPv4 SYN: IP at 0, fixed TCP at 20, options at 40.
func rfc2385Packet(source, destination uint16, options []byte) []byte {
	packet := make([]byte, 40+len(options))
	packet[0], packet[8], packet[9] = 0x45, 64, 6
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	copy(packet[12:20], []byte{192, 0, 2, 1, 192, 0, 2, 2})
	binary.BigEndian.PutUint16(packet[20:22], source)
	binary.BigEndian.PutUint16(packet[22:24], destination)
	binary.BigEndian.PutUint32(packet[24:28], 0x10203040)
	packet[32], packet[33] = byte((20+len(options))/4)<<4, 2
	binary.BigEndian.PutUint16(packet[34:36], 65535)
	copy(packet[40:], options)
	return packet
}

// rfc2385Checksums writes the IPv4 and TCP Internet checksums after each mutation.
func rfc2385Checksums(packet []byte) {
	packet[10], packet[11], packet[36], packet[37] = 0, 0, 0, 0
	binary.BigEndian.PutUint16(packet[10:12], rfc2385Checksum(packet[:20]))
	pseudo := make([]byte, 12+len(packet)-20)
	copy(pseudo[:8], packet[12:20])
	pseudo[9] = 6
	binary.BigEndian.PutUint16(pseudo[10:12], uint16(len(packet)-20))
	copy(pseudo[12:], packet[20:])
	binary.BigEndian.PutUint16(packet[36:38], rfc2385Checksum(pseudo))
}

// rfc2385Checksum computes the one's complement sum on the independent wire fixture.
func rfc2385Checksum(data []byte) uint16 {
	var sum uint32
	for index := 0; index+1 < len(data); index += 2 {
		sum += uint32(binary.BigEndian.Uint16(data[index : index+2]))
	}
	if len(data)%2 != 0 {
		sum += uint32(data[len(data)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = sum&0xffff + sum>>16
	}
	return ^uint16(sum)
}

// rfc2385Digest implements the independent RFC 2385 Section 2.0 oracle:
// "the TCP header, excluding options, and assuming a checksum of zero" follows
// the pseudo-header; "the TCP segment data (if any)" precedes the shared key.
func rfc2385Digest(packet []byte, key string) [16]byte {
	ipHeader := int(packet[0]&15) * 4
	tcp := packet[ipHeader:]
	var input [12 + 20 + 1600 + 80]byte
	copy(input[:8], packet[12:20])
	input[9] = 6
	binary.BigEndian.PutUint16(input[10:12], uint16(len(tcp)))
	copy(input[12:32], tcp[:20])
	input[28], input[29] = 0, 0
	size := 32 + copy(input[32:], tcp[int(tcp[12]>>4)*4:])
	size += copy(input[size:], key)
	return md5.Sum(input[:size]) //nolint:gosec // Independent RFC 2385 wire oracle.
}

// assertRFC2385Digest checks the whole option, not merely a successful peer transfer.
func assertRFC2385Digest(t *testing.T, packet []byte, key string) {
	t.Helper()
	tcp := packet[int(packet[0]&15)*4:]
	option := rfc2385Option(t, tcp, 19)
	if len(option) != 18 {
		t.Fatalf("MD5 option = %x, want Kind 19 Length 18", option)
	}
	// RFC 2385 Section 2.0: recompute without using the kernel's verifier.
	digest := rfc2385Digest(packet, key)
	if !bytes.Equal(option[2:], digest[:]) {
		t.Fatalf("captured digest=%x independently computed=%x packet=%x", option[2:], digest, packet)
	}
}

// rfc2385Option walks the actual TCP data-offset interval and rejects duplicates.
func rfc2385Option(t *testing.T, tcp []byte, kind byte) []byte {
	t.Helper()
	var found []byte
	options := tcp[20 : int(tcp[12]>>4)*4]
	for offset := 0; offset < len(options); {
		if options[offset] == 0 {
			break
		}
		if options[offset] == 1 {
			offset++
			continue
		}
		if offset+2 > len(options) {
			t.Fatal("TCP option has no length")
		}
		size := int(options[offset+1])
		if size < 2 {
			t.Fatal("TCP option length below two")
		}
		if offset+size > len(options) {
			t.Fatal("TCP option crosses data offset")
		}
		if options[offset] == kind {
			if found != nil {
				t.Fatalf("repeated TCP option kind %d", kind)
			}
			found = options[offset : offset+size]
		}
		offset += size
	}
	return found
}

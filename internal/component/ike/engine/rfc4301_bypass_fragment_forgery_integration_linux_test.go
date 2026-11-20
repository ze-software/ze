//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- protected-host forwarding, operator bypass projection and fragments.
// Related: rfc4301_fragment_forward_integration_linux_test.go -- the three-namespace forwarding lab, sender, capture and the forward-gate check reused here.
// Related: fragment_boundary_integration_linux_test.go -- the production Child SA creator and fragment sender.
// RFC naming: untagged -- the forged-fragment proof of owner decision 4 rides beside the tagged RFC4301-7.4-2 units; it claims no row of its own.
//
// VALIDATES: on Ze's kernel (K1 and K3), that forged IPv4 fragments aimed at an operator
// port-scoped BYPASS on the forward path take the disposition of the datagram the kernel
// reassembles, never the disposition one fragment's ports would earn alone. This is the
// retargeted `bypass-fragment-forgery` proof of owner decision 4 (2026-11-20): the
// node-wide IKE bypass it was planned against no longer exists, and an operator bypass is
// projected onto the forward path only on a kernel advertising capability bit 3, whose
// PRE_ROUTING gathering (gokrazy/kernel/patches/0005-xfrm-fragment-guard.patch,
// xfrm4_fragment_gather) reassembles before ip_forward's FWD check reads the ports.
// PREVENTS: a first fragment whose ports match the bypass carrying a datagram whose
// reassembled content is other-port, other-protocol or protected traffic into the clear.
// No peer daemon takes part: the forgery is cleartext Ze forwards, so the proof needs a raw
// sender and a capture, not an IKE peer.

package engine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/ipv4"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	// forgeryProtoTCP is the protocol of a tail forged to join a UDP datagram.
	forgeryProtoTCP = 6
	// forgeryMarker fills the payload of every forged segment, so a delivered datagram
	// that carries any forged octet is told apart from the legitimate one.
	forgeryMarker = 'F'
)

// forgeryPiece is one fragment of a case: octets start to end of a UDP segment, sent as
// an IPv4 fragment at offset start with the given protocol. start is a multiple of
// eight, as the fragment offset field counts eight-octet units.
type forgeryPiece struct {
	segment    []byte
	start, end int
	more       bool
	protocol   byte
}

// forgeryWire counts the packets of one case's datagram ID seen at the outside peer's
// ingress, of any protocol: a whole datagram, or anything that is still a fragment.
type forgeryWire struct {
	whole, fragments int
}

// TestRFC4301ForgedFragmentsTakeTheReassembledDatagramsDisposition sends forged fragment
// sets from the protected host through Ze to the outside peer, under unmatched DISCARD,
// with one operator BYPASS for UDP to port 6000 and a Child SA protecting UDP to port 5000
// between the same addresses (the RFC 4301 Section 7.4 example: a bypass beside protected
// traffic to the same destination). Each case sends its fragments one at a time, waiting
// for Ze's IPv4 ReasmReqds to count each one, so the kernel queues them in the order the
// case states; a whole bypass datagram follows as the observation barrier.
//
// The positives: a legitimate datagram on the bypass port, split after its 8-octet UDP
// header, crosses exactly once (as its original fragments, which ip_do_fragment re-emits
// from the gathered frag_list, or whole) and the peer's UDP socket receives it with its own
// payload, then the barrier, then nothing; and a later duplicate initial fragment naming
// another port does not change which datagram arrives, because Linux drops a fragment that
// lies inside one already queued (IPFRAG_DUP in inet_frag_queue_insert).
// The negatives: a duplicate initial naming the bypass port after the initial of an
// unnamed-port datagram, or of a protected one, lets nothing out in the clear; an initial
// that rewrites the ports over a longer span, and a non-initial fragment that overlaps the
// queued initial, each make Linux discard the whole queue (IPFRAG_OVERLAP, counted as
// ReasmOverlaps by ip_frag_queue), so nothing crosses; and a tail of another protocol joins
// no UDP queue, so neither half crosses.
//
// RFC 4301 Section 7.4: "All implementations MUST support stateful fragment checking to
// accommodate BYPASS traffic for which a non-trivial port range is specified."
// RFC 4301 Section 7.4: "An attacker could send a non-initial fragment (with a forged
// source address) that, if bypassed, could overlap with IPsec-protected traffic from the
// same source and thus violate the integrity of the IPsec-protected traffic."
// RFC 4301 Section 7.3: "Also, the receiver MUST discard non-initial fragments that do not
// comply with the security policy applied to the overall packet."
// RFC 4301 Appendix D.3: "This concern is easily addressed in IPv4, by checking the
// fragment offset value to ensure that no non-initial fragments have a small enough offset
// to overlap port fields that should be contained in the initial fragment."
//
//ze:guest-root
func TestRFC4301ForgedFragmentsTakeTheReassembledDatagramsDisposition(t *testing.T) {
	lab := newEspPolLab(t)
	sender, capture := fragmentForwardPeers(t, lab)
	espPolSPD(t)
	fragmentChildren(t, fragmentPorts{
		local: ipsec.AnyPort(), remote: ipsec.PortSelector{Form: ipsec.PortSingle, Port: sadSelPort},
	})
	// RFC 4301 Sections 4.4.1 and 7.4: use the configured fallback, not a test-only drop.
	if err := installUnmatched(dataplane.Get(), dataplane.SPActionDiscard, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("select the production unmatched-DISCARD disposition: %v", err)
	}
	entries := map[string]ipsec.SPDPolicy{
		"forgery-bypass": {
			Name: "forgery-bypass", Action: dataplane.SPActionBypass, Order: 1000,
			Direction: ipsec.SPDDirOut, Protocol: sadSelProtoUDP,
			LocalPrefix: mustCIDRNet(t, "10.2.0.0/16"), LocalPort: ipsec.AnyPort(),
			RemotePrefix: mustCIDRNet(t, "10.1.0.0/16"),
			RemotePort:   ipsec.PortSelector{Form: ipsec.PortSingle, Port: fragmentBypassPort},
		},
	}
	// RFC 4301 Sections 4.4.1 and 7.4.
	if err := installSPDPolicies(dataplane.Get(), nil, entries, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("install the operator bypass entry: %v", err)
	}
	t.Cleanup(func() { removeSPDPolicies(dataplane.Get(), entries, slogutil.DiscardLogger()) })
	fragmentWantOperatorPolicies(t, entries)
	fragmentWantForwardBypassGate(t)
	lab.enter(t, lab.peer)
	bypassed := sadSelListen(t, sadSelInnerRemote, fragmentBypassPort)
	lab.enter(t, lab.ze)

	legit := sadSelUDP(fragmentBypassPort)
	protected := sadSelUDP(sadSelPort)
	forgedBypass := forgerySegment(fragmentBypassPort)
	forgedUnnamed := forgerySegment(fragmentUnnamedPort)
	tail := len(legit)
	for index, tc := range []struct {
		name     string
		pieces   []forgeryPiece
		bypass   bool
		geometry int
		overlaps uint64
	}{
		{
			// RFC 4301 Appendix D.3: "Recall that the IPv4 MTU minimum is 576 bytes, and the
			// max IP header length is 60 bytes, so any ports should be present in the initial
			// fragment."
			name: "smallest initial fragment, the UDP header alone, then the rest",
			pieces: []forgeryPiece{
				{legit, 0, 8, true, sadSelProtoUDP}, {legit, 8, tail, false, sadSelProtoUDP},
			},
			bypass: true, geometry: 2,
		},
		{
			name: "duplicate initial naming an unnamed port after the bypass initial",
			pieces: []forgeryPiece{
				{legit, 0, 16, true, sadSelProtoUDP}, {forgedUnnamed, 0, 16, true, sadSelProtoUDP},
				{legit, 16, tail, false, sadSelProtoUDP},
			},
			bypass: true, geometry: 2,
		},
		{
			name: "duplicate initial naming the bypass port after an unnamed-port initial",
			pieces: []forgeryPiece{
				{forgedUnnamed, 0, 16, true, sadSelProtoUDP}, {forgedBypass, 0, 16, true, sadSelProtoUDP},
				{forgedUnnamed, 16, tail, false, sadSelProtoUDP},
			},
		},
		{
			name: "duplicate initial naming the bypass port after a protected initial",
			pieces: []forgeryPiece{
				{protected, 0, 16, true, sadSelProtoUDP}, {forgedBypass, 0, 16, true, sadSelProtoUDP},
				{protected, 16, tail, false, sadSelProtoUDP},
			},
		},
		{
			name: "longer initial rewriting the ports of the bypass initial",
			pieces: []forgeryPiece{
				{legit, 0, 16, true, sadSelProtoUDP}, {forgedUnnamed, 0, 24, true, sadSelProtoUDP},
				{legit, 16, tail, false, sadSelProtoUDP},
			},
			overlaps: 1,
		},
		{
			name: "non-initial fragment overlapping the bypass initial",
			pieces: []forgeryPiece{
				{legit, 0, 16, true, sadSelProtoUDP}, {forgedBypass, 8, 24, true, sadSelProtoUDP},
				{legit, 16, tail, false, sadSelProtoUDP},
			},
			overlaps: 1,
		},
		{
			name: "tail of another protocol after the bypass initial",
			pieces: []forgeryPiece{
				{legit, 0, 16, true, sadSelProtoUDP}, {legit, 16, tail, false, forgeryProtoTCP},
			},
		},
	} {
		id := uint16(0x6c00 + index*2)
		overlaps := forgeryIPCounter(t, "netstat", "IpExt:", "ReasmOverlaps")
		for _, piece := range tc.pieces {
			forgerySendQueued(t, sender, forgeryFragment(t, piece, id))
		}
		// The barrier is a whole datagram on the bypass port: under unmatched DISCARD it
		// crosses only through the forward entry the projection installed.
		fragmentSend(t, sender, fragmentForwardPacket(t, fragmentBypassPort, id+1))
		seen := forgeryObserve(t, capture, id, id+1)
		if got := forgeryIPCounter(t, "netstat", "IpExt:", "ReasmOverlaps") - overlaps; got != tc.overlaps {
			t.Errorf("%s: ReasmOverlaps moved by %d, want %d", tc.name, got, tc.overlaps)
		}
		forgeryWantLegitimate(t, tc.name, bypassed, legit)
		if tc.bypass {
			// K3 gathers with ip_defrag, whose ip_frag_reasm keeps the original fragments
			// on the head's frag_list and records IPCB(skb)->frag_max_size; on egress
			// __ip_finish_output refragments any datagram carrying frag_max_size, and
			// ip_do_fragment's frag_list fast path re-emits the original geometry. The
			// patch (0005-xfrm-fragment-guard.patch) changes none of that, so the stack
			// owes the legitimate datagram once: as its original fragments, or whole on a
			// stack that rebuilt it. A forged duplicate is never re-emitted, because
			// inet_frag_queue_insert dropped it before it joined the queue.
			whole := seen.whole == 1 && seen.fragments == 0
			original := seen.whole == 0 && seen.fragments == tc.geometry
			if !whole && !original {
				t.Errorf("%s: outside peer saw %+v, want the legitimate datagram once: whole, or as its %d original fragments",
					tc.name, seen, tc.geometry)
			}
			// The peer's kernel reassembled what crossed: the one datagram on the bypass
			// port carries the legitimate payload, so a later duplicate initial naming
			// another port did not change which datagram arrived. The barrier follows,
			// and nothing else.
			forgeryWantLegitimate(t, tc.name, bypassed, legit)
			if sadSelDelivered(t, bypassed) {
				t.Errorf("%s: a datagram besides the legitimate one and the barrier reached the bypass port", tc.name)
			}
			continue
		}
		if seen.whole+seen.fragments != 0 {
			t.Errorf("%s: a forged datagram's content reached the outside peer in the clear: %+v", tc.name, seen)
		}
		if sadSelDelivered(t, bypassed) {
			t.Errorf("%s: a datagram besides the barrier reached the bypass port", tc.name)
		}
	}
}

// forgerySegment returns the UDP segment to port whose payload is all forgeryMarker.
func forgerySegment(port uint16) []byte {
	segment := sadSelUDP(port)
	for i := 8; i < len(segment); i++ {
		segment[i] = forgeryMarker
	}
	return segment
}

// forgeryFragment builds the IPv4 fragment of piece, from the protected host to the
// outside peer, with datagram ID id.
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|Version|  IHL  |Type of Service|          Total Length         |  0
//	|         Identification        |Flags|      Fragment Offset    |  4
//	|  Time to Live |    Protocol   |         Header Checksum       |  8
//	|                       Source Address                          | 12
//	|                    Destination Address                        | 16
//	|          octets start to end of the segment ...               | 20
//
// RFC 791 Section 3.1: "The fragment offset is measured in units of 8 octets (64 bits).  The
// first fragment has offset zero." The MF flag is 0x2000 of octets 6 and 7.
func forgeryFragment(t *testing.T, piece forgeryPiece, id uint16) []byte {
	t.Helper()
	if piece.start%8 != 0 {
		t.Fatalf("a fragment offset counts eight-octet units, so start %d must be a multiple of 8", piece.start)
	}
	fragment := sadSelIPv4(fragmentHostAddress, piece.protocol, piece.segment[piece.start:piece.end])
	copy(fragment[16:20], net.ParseIP(sadSelInnerRemote).To4())
	flags := uint16(piece.start / 8)
	if piece.more {
		flags |= 0x2000
	}
	binary.BigEndian.PutUint16(fragment[6:8], flags)
	fragmentPacketID(fragment, id)
	return fragment
}

// forgerySendQueued sends one fragment and waits until Ze's reassembly has taken it,
// read as Ze's IPv4 ReasmReqds moving by one, so the next fragment of the case is queued
// after it. The caller MUST be in Ze's namespace.
func forgerySendQueued(t *testing.T, sender *ipv4.RawConn, fragment []byte) {
	t.Helper()
	queued := forgeryIPCounter(t, "snmp", "Ip:", "ReasmReqds")
	fragmentSend(t, sender, fragment)
	awaitForwardKernelCounter(t, "Ze's forward-path reassembly (Ip ReasmReqds)", queued+1, func(t *testing.T) uint64 {
		return forgeryIPCounter(t, "snmp", "Ip:", "ReasmReqds")
	})
}

// forgeryObserve counts the packets of datagram packetID, of any protocol, from the
// protected host to the outside peer, until the whole UDP barrier barrierID arrives. At
// most 64 received frames are considered, within one bounded receive budget.
func forgeryObserve(t *testing.T, fd int, packetID, barrierID uint16) forgeryWire {
	t.Helper()
	host := net.ParseIP(fragmentHostAddress)
	remote := net.ParseIP(sadSelInnerRemote)
	var seen forgeryWire
	deadline := time.Now().Add(fragmentCaptureWait)
	buf := make([]byte, 65535)
	for range 64 {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("outside observation timed out before the barrier: packets=%+v", seen)
		}
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		ready, err := unix.Poll(poll, max(1, int(remaining/time.Millisecond)))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Fatalf("wait for the outside peer's packet observation: %v", err)
		}
		if ready == 0 {
			t.Fatalf("outside observation saw no barrier: packets=%+v", seen)
		}
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			t.Fatalf("read the outside peer's ingress packet: %v", err)
		}
		h, err := ipv4.ParseHeader(buf[:n])
		if err != nil {
			t.Fatalf("read the received IPv4 header: %v", err)
		}
		if !h.Src.Equal(host) {
			continue
		}
		if !h.Dst.Equal(remote) {
			continue
		}
		fragment := h.FragOff != 0 || h.Flags&ipv4.MoreFragments != 0
		if h.ID == int(barrierID) && h.Protocol == sadSelProtoUDP && !fragment {
			return seen
		}
		if h.ID != int(packetID) {
			continue
		}
		if fragment {
			seen.fragments++
			continue
		}
		seen.whole++
	}
	t.Fatalf("outside observation exceeded its 64-frame bound: packets=%+v", seen)
	return seen
}

// forgeryWantLegitimate requires one datagram on the bypass listener whose payload is the
// legitimate segment's, never a forged octet.
func forgeryWantLegitimate(t *testing.T, name string, conn *net.UDPConn, legit []byte) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set the bypass listener's deadline: %v", err)
	}
	buf := make([]byte, 256)
	n, _, err := conn.ReadFromUDP(buf)
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Errorf("%s: a datagram owed to the bypass port was not delivered", name)
		return
	}
	if err != nil {
		t.Fatalf("read the bypass listener: %v", err)
	}
	if !bytes.Equal(buf[:n], legit[8:]) {
		t.Errorf("%s: the bypass port received payload %q, want the legitimate %q", name, buf[:n], legit[8:])
	}
}

// forgeryIPCounter reads counter name of section ("Ip:" in snmp, "IpExt:" in netstat)
// from /proc/thread-self/net/<file>: the calling thread's own namespace, never the
// process leader's. A section is a line of names followed by a line of values.
func forgeryIPCounter(t *testing.T, file, section, name string) uint64 {
	t.Helper()
	path := "/proc/thread-self/net/" + file
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var names []string
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != section {
			continue
		}
		if names == nil {
			names = fields
			continue
		}
		for i, field := range names {
			if field != name || i >= len(fields) {
				continue
			}
			n, convErr := strconv.ParseUint(fields[i], 10, 64)
			if convErr != nil {
				t.Fatalf("parse %s %s = %q: %v", section, name, fields[i], convErr)
			}
			return n
		}
		break
	}
	t.Fatalf("%s %s absent from %s", section, name, path)
	return 0
}

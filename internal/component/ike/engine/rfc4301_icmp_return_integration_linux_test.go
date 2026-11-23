//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- ICMP error messages carried by a Child SA.
// Related: rfc4301_forward_icmp_integration_linux_test.go -- real Child creator and namespace lab.

package engine

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/net/ipv4"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

type icmpReturnDecoder struct {
	spi  uint32
	aead cipher.AEAD
	salt []byte
}

func newICMPReturnDecoder(t *testing.T, child *ChildSA, check bool) *icmpReturnDecoder {
	t.Helper()
	state, err := netlink.XfrmStateGet(&netlink.XfrmState{
		Dst: net.ParseIP(espPolPeerOuter), Spi: int(child.OutboundSPI), Proto: netlink.XFRM_PROTO_ESP,
	})
	if err != nil {
		t.Fatalf("read the return SA's actual key: %v", err)
	}
	if state.Aead == nil || len(state.Aead.Key) != 36 {
		t.Fatal("the return SA has no AES-256-GCM key")
	}
	block, err := aes.NewCipher(state.Aead.Key[:32])
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	policies, err := dataplane.Get().ListPolicies()
	if err != nil {
		t.Fatalf("read the return policy's actual ICMP mapping flag: %v", err)
	}
	found := false
	for i := range policies {
		policy := &policies[i]
		if policy.Dir == dataplane.SADirOut && policy.Action == dataplane.SPActionProtect &&
			policy.UpperProto == sadSelProtoUDP && sameIPNet(policy.Src, child.TSLocal) && sameIPNet(policy.Dst, child.TSRemote) {
			found = true
			if policy.ICMPErrorReturn != check {
				t.Fatalf("stored return-policy ICMP flag=%v, want %v", policy.ICMPErrorReturn, check)
			}
		}
	}
	if !found {
		t.Fatal("the return SA has no installed UDP PROTECT policy")
	}
	return &icmpReturnDecoder{spi: child.OutboundSPI, aead: aead, salt: bytes.Clone(state.Aead.Key[32:])}
}

func (d *icmpReturnDecoder) open(t *testing.T, packet []byte) []byte {
	t.Helper()
	if len(packet) < 36 || binary.BigEndian.Uint32(packet[:4]) != d.spi {
		t.Fatal("the captured ESP packet does not name the expected return SA")
	}
	nonce := append(bytes.Clone(d.salt), packet[8:16]...)
	plain, err := d.aead.Open(nil, nonce, packet[16:], packet[:8])
	if err != nil {
		t.Fatalf("authenticate the actual return ESP packet: %v", err)
	}
	if len(plain) < 2 || plain[len(plain)-1] != sadSelProtoIPv4 {
		t.Fatal("return ESP does not carry an IPv4 packet")
	}
	end := len(plain) - 2 - int(plain[len(plain)-2])
	if end < 20 {
		t.Fatal("return ESP has invalid padding or a truncated inner IPv4 header")
	}
	return plain[:end]
}

// icmpReturnObserve requires a separately tagged UDP packet through the same SA.
// It decrypts actual peer-ingress ESP, rather than treating an SA counter or SPI
// alone as proof that the ICMP quote was returned. The bounded observation also
// records cleartext errors, which an allow-without-guard defect would leak.
func icmpReturnObserve(t *testing.T, fd int, decoder *icmpReturnDecoder, trigger []byte, tag uint16, local, want bool) {
	t.Helper()
	protectedErrors, clearErrors := 0, 0
	control := false
	deadline := time.Now().Add(icmpForwardWait)
	buf := make([]byte, 65535)
	for range 128 {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		ready, err := unix.Poll(poll, max(1, int(remaining/time.Millisecond)))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Fatalf("wait for real return packets: %v", err)
		}
		if ready == 0 {
			break
		}
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			t.Fatalf("capture a return packet at the peer: %v", err)
		}
		header, err := ipv4.ParseHeader(buf[:n])
		if err != nil {
			t.Fatal(err)
		}
		inner := buf[:n]
		protected := false
		esp := buf[header.Len:n]
		if header.Protocol == sadSelProtoUDP {
			if len(esp) < 8 || binary.BigEndian.Uint16(esp[2:4]) != 4500 {
				continue
			}
			esp = esp[8:]
		}
		if header.Protocol == protoESP || header.Protocol == sadSelProtoUDP {
			if len(esp) < 4 || binary.BigEndian.Uint32(esp[:4]) != decoder.spi {
				continue
			}
			inner = decoder.open(t, esp)
			protected = true
		}
		if len(inner) < 20 {
			t.Fatal("received inner IPv4 packet is truncated")
		}
		innerHeader := int(inner[0]&15) * 4
		if innerHeader < 20 || innerHeader > len(inner) {
			t.Fatal("received inner IPv4 header length is invalid")
		}
		if protected && inner[9] == sadSelProtoUDP && len(inner) >= innerHeader+8 &&
			net.IP(inner[12:16]).Equal(net.ParseIP(sadSelInnerLocal)) &&
			net.IP(inner[16:20]).Equal(net.ParseIP(sadSelInnerRemote)) &&
			binary.BigEndian.Uint16(inner[innerHeader:innerHeader+2]) == sadSelPort &&
			binary.BigEndian.Uint16(inner[innerHeader+2:innerHeader+4]) == sadSelSourcePort &&
			bytes.Equal(inner[innerHeader+8:], icmpReturnControlToken(tag+1)) {
			control = true
		}
		if inner[9] != sadSelProtoICMP || len(inner) < innerHeader+36 {
			continue
		}
		message := inner[innerHeader:]
		matches := binary.BigEndian.Uint16(inner[4:6]) == tag
		if local {
			matches = message[0] == 3 && message[1] == 3 && binary.BigEndian.Uint16(message[12:14]) == tag
		}
		if !matches {
			continue
		}
		if local {
			if !bytes.Equal(message[8:36], trigger[:28]) {
				t.Fatalf("the generated error quoted another packet: got %x, want %x", message[8:36], trigger[:28])
			}
		} else if !bytes.Equal(message, trigger[20:]) {
			t.Fatal("the forwarded return error changed its ICMP message or quote")
		}
		if protected {
			protectedErrors++
		} else {
			clearErrors++
		}
		if want && protectedErrors == 1 && control {
			break
		}
	}
	if !control {
		t.Fatal("the separately tagged UDP control did not leave through the same real SA")
	}
	if clearErrors != 0 || (protectedErrors == 1) != want || protectedErrors > 1 {
		t.Errorf("ICMP return on actual wire: protected=%d clear=%d, want protected=%v and no cleartext", protectedErrors, clearErrors, want)
	}
}

func icmpReturnUDP(source, destination string, tag uint16) []byte {
	packet := sadSelIPv4(source, sadSelProtoUDP, sadSelUDP(sadSelSourcePort))
	copy(packet[16:20], net.ParseIP(destination).To4())
	binary.BigEndian.PutUint16(packet[20:22], sadSelPort)
	fragmentPacketID(packet, tag)
	return packet
}

func icmpReturnControlToken(tag uint16) []byte {
	token := append([]byte("ze-icmp-return-control:"), 0, 0)
	binary.BigEndian.PutUint16(token[len(token)-2:], tag)
	return token
}

func icmpReturnControlSocket(t *testing.T) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(sadSelInnerLocal), Port: sadSelPort})
	if err != nil {
		t.Fatalf("bind the ordinary protected UDP control: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close the protected UDP control: %v", err)
		}
	})
	return conn
}

func icmpReturnSendControl(t *testing.T, conn *net.UDPConn, tag uint16) {
	t.Helper()
	if _, err := conn.WriteToUDP(icmpReturnControlToken(tag), &net.UDPAddr{IP: net.ParseIP(sadSelInnerRemote), Port: sadSelSourcePort}); err != nil {
		t.Fatalf("send the ordinary UDP control through its real UDP policy: %v", err)
	}
}

// A NoPorts increment proves the authenticated trigger passed the receive policy,
// checksum and failed UDP socket lookup before the control socket is allowed to bind.
// Linux udp.c increments it immediately before icmp_send; no elapsed-time guess is used.
func icmpReturnNoPorts(t *testing.T) uint64 {
	t.Helper()
	raw, err := os.ReadFile("/proc/thread-self/net/snmp")
	if err != nil {
		t.Fatalf("read the private UDP no-port counter: %v", err)
	}
	var header []string
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "Udp:" {
			continue
		}
		if header == nil {
			header = fields
			continue
		}
		for index, name := range header {
			if name != "NoPorts" || index >= len(fields) {
				continue
			}
			value, parseErr := strconv.ParseUint(fields[index], 10, 64)
			if parseErr != nil {
				t.Fatalf("parse private UDP NoPorts: %v", parseErr)
			}
			return value
		}
	}
	t.Fatal("the private UDP namespace has no NoPorts counter")
	return 0
}

func icmpReturnPeerCapture(t *testing.T, lab *espPolLab) int {
	t.Helper()
	lab.enter(t, lab.peer)
	fd := fragmentCaptureSocket(t, espPolLink(t, espPolPeerLink).Attrs().Index)
	lab.enter(t, lab.ze)
	return fd
}

// TestRFC4301ForwardedICMPErrorsMapUnderDiscard checks that a forwarded ICMP error
// from a protected host, under the default `unmatched discard` with no operator entry,
// leaves through the SA of the return traffic it quotes, and that one quoting traffic
// no SA carries leaves neither protected nor in clear. Method: Ze's namespace forwards
// between the host and the outside peer. No FWD policy matches the error itself (the
// Child's outbound FWD allow, appendChildOutboundForwardPolicy, is UDP only), so the
// kernel's payload lookup (xfrm_in_fwd_icmp) decides the forward check: the reversed
// quote matches that outbound FWD allow, and a quote no Child carries meets the FWD
// default discard. The OUT catch-all answers BLOCK, and K1's forwarded mapping (bit 6)
// tries the return SA. The peer's capture decrypts the ESP it receives and counts
// errors in clear, beside a UDP control through the same SA.
//
// RFC requirement: RFC4301-6.2-3 positive -- under `unmatched discard` with no operator entry, a forwarded ICMP Destination Unreachable from the protected host, which no SA and no SPD entry carries, that quotes the Child SA's return traffic meets the OUT catch-all (XfrmOutPolBlock rises by one) and is decrypted at the outside peer from that Child SA, and is not seen there in clear.
// RFC requirement: RFC4301-6.2-3 negative -- the same error quoting traffic no SA carries, which has no return SA to be mapped to, is seen at the outside peer neither protected nor in clear.
//
//ze:guest-root
func TestRFC4301ForwardedICMPErrorsMapUnderDiscard(t *testing.T) {
	forwardedICMPErrorUnderDiscard(t, 0x7a10, func(quotedDestination string, tag uint16) []byte {
		return icmpReturnForwarded(3, 1, quotedDestination, tag)
	})
}

// TestRFC4301ForwardedICMPParameterProblemMapsUnderDiscard checks the same mapping for
// an IPv4 Parameter Problem (type 12), the error type Linux's own list leaves out.
// RFC 4301 Section 6 names the class the mapping serves: "There are two categories of
// ICMP traffic: error messages (e.g., type = destination unreachable) and non-error
// messages (e.g., type = echo)." Section 6.2 then requires, for an error message: "If
// no SA exists that would carry the outbound ICMP message in question, and if no SPD
// entry would allow carriage of this outbound ICMP error message, then an IPsec
// implementation MUST map the message to the SA that would carry the return traffic
// associated with the packet that triggered the ICMP error message."
// Stock icmp_err_packet (net/xfrm/xfrm_policy.c) answers only Destination Unreachable
// and Time Exceeded for IPv4, so neither xfrm_in_fwd_icmp nor xfrm_out_fwd_icmp reads
// a Parameter Problem's quote; K1 answers every type icmp_is_err names (3, 4, 5, 11 and
// 12), which is the set icmpErrorTypesV4 names. Method: the lab and observation of
// TestRFC4301ForwardedICMPErrorsMapUnderDiscard, with a Parameter Problem whose pointer
// (octet 4 of the message) is not zero, so the byte comparison at the peer also proves
// that octet crossed unchanged. The one quoting the Child SA's return traffic must leave
// protected; the one quoting traffic no SA carries is not mappable and must leave
// neither protected nor in clear. The harness speaks IPv4 alone, so the ICMPv6
// Parameter Problem (type 4) is not exercised here.
//
//ze:guest-root
func TestRFC4301ForwardedICMPParameterProblemMapsUnderDiscard(t *testing.T) {
	forwardedICMPErrorUnderDiscard(t, 0x7a70, func(quotedDestination string, tag uint16) []byte {
		return icmpReturnParameterProblem(1, quotedDestination, tag)
	})
}

// icmpReturnParameterProblem is icmpReturnForwarded's message as an IPv4 Parameter
// Problem (type 12, code 0) whose pointer names the given octet of the quoted datagram.
//
//	 0       1       2       3       4       5       6       7
//	+-------+-------+---------------+-------+-----------------------+
//	| 12    | 0     | checksum      |pointer| unused (zero)         |
//	+-------+-------+---------------+-------+-----------------------+
//	| octets 8 to 35: the quoted IPv4 header and 8 octets of UDP    |
//	+---------------------------------------------------------------+
func icmpReturnParameterProblem(pointer byte, quotedDestination string, tag uint16) []byte {
	packet := icmpReturnForwarded(12, 0, quotedDestination, tag)
	message := packet[20:]
	message[4] = pointer
	binary.BigEndian.PutUint16(message[2:4], 0)
	binary.BigEndian.PutUint16(message[2:4], sadSelChecksum(message))
	return packet
}

// forwardedICMPErrorUnderDiscard runs the lab of TestRFC4301ForwardedICMPErrorsMapUnderDiscard
// for one error message shape: build answers the protected host's message quoting a
// datagram to quotedDestination, its IPv4 identification set to tag. The first message
// quotes the Child SA's return traffic and must leave protected, after meeting the OUT
// catch-all's BLOCK; the second quotes traffic no SA carries and must not leave at all.
func forwardedICMPErrorUnderDiscard(t *testing.T, tagBase uint16, build func(quotedDestination string, tag uint16) []byte) {
	t.Helper()
	lab := newEspPolLab(t)
	hostSender, capture := fragmentForwardPeers(t, lab)
	espPolSPD(t)
	child := icmpForwardChild(t, 0, true)
	decoder := newICMPReturnDecoder(t, child, true)
	controlSender := icmpReturnControlSocket(t)
	for index, quotedDestination := range []string{fragmentHostAddress, icmpForwardUnmatched} {
		tag := tagBase + uint16(index*2)
		packet := build(quotedDestination, tag)
		blocked := sadSelStat(t, outStepStatPolBlock)
		fragmentSend(t, hostSender, packet)
		icmpReturnSendControl(t, controlSender, tag+1)
		icmpReturnObserve(t, capture, decoder, packet, tag, false, index == 0)
		if index == 0 {
			if got := sadSelStat(t, outStepStatPolBlock) - blocked; got != 1 {
				t.Errorf("XfrmOutPolBlock moved by %d, want 1: the error did not meet the OUT catch-all", got)
			}
		}
	}
}

// icmpReturnForwarded is an ICMP message of the given type and code that the
// protected host 10.2.0.2 sends towards the outside peer 10.1.0.5, carrying in its
// body the first 28 octets of a UDP datagram from 10.1.0.5 to quotedDestination
// (source port 5000, destination port 40000). For an
// error type, reversing that quote selects the Child SA's return traffic when
// quotedDestination is the host, and no SA at all when it is icmpForwardUnmatched.
// For a non-error type the same octets are only its data. The IPv4 identification
// carries tag, which icmpReturnObserve matches.
func icmpReturnForwarded(typ, code byte, quotedDestination string, tag uint16) []byte {
	quote := icmpReturnUDP(sadSelInnerRemote, quotedDestination, tag)
	binary.BigEndian.PutUint16(quote[20:22], sadSelSourcePort)
	binary.BigEndian.PutUint16(quote[22:24], sadSelPort)
	message := make([]byte, 8, 36)
	message[0], message[1] = typ, code
	message = append(message, quote[:28]...)
	binary.BigEndian.PutUint16(message[2:4], sadSelChecksum(message))
	packet := sadSelIPv4(fragmentHostAddress, sadSelProtoICMP, message)
	copy(packet[16:20], net.ParseIP(sadSelInnerRemote).To4())
	fragmentPacketID(packet, tag)
	return packet
}

// TestRFC4301ForwardedICMPErrorUnderOperatorDiscardMapsToReturnSA checks the owner's
// ruling of 2026-11-20 on RFC 4301 Section 6.2: an operator out DISCARD that covers a
// forwarded ICMP error does not stop its mapping into the return SA, and the mapping
// takes only what it can carry. Method: the entry discards every ICMP message from
// 10.2.0.0/24 to 10.1.0.0/24 at order 100; an operator in BYPASS for ICMP from the
// host 10.2.0.2 alone admits each message at the forward check, so every message
// reaches the OUT lookup, and XfrmOutPolBlock moving by one proves that lookup answered
// BLOCK. The bypass names the host, not the discard's 10.2.0.0/24: its OUT transit gate
// would otherwise carry the discard's selector, which the configuration verifier
// refuses (spdSameSelectorRefusal) because the kernel keeps one policy per selector and
// the gate, installed second, replaces the discard. The pair is checked against that
// verifier before it is installed, so the SPD under test is one Ze accepts. A
// Destination Unreachable quoting the Child SA's return traffic must leave protected;
// an Echo Request (not an error type icmp_err_packet maps) and a Destination
// Unreachable quoting traffic no SA carries must leave neither protected nor in clear.
// The peer's capture decrypts the ESP it receives and counts errors in clear, beside a
// UDP control through the same SA. It needs K1's feature bit 6 on Ze's kernel.
//
// RFC requirement: RFC4301-6-1 negative -- a forwarded ICMP Echo Request from the protected host that an operator out DISCARD for ICMP covers meets that entry's OUT policy (XfrmOutPolBlock rises by one) and is seen at the outside peer neither protected nor in clear.
//
//ze:guest-root
func TestRFC4301ForwardedICMPErrorUnderOperatorDiscardMapsToReturnSA(t *testing.T) {
	lab := newEspPolLab(t)
	hostSender, capture := fragmentForwardPeers(t, lab)
	espPolSPD(t)
	child := icmpForwardChild(t, 0, true)
	decoder := newICMPReturnDecoder(t, child, true)
	controlSender := icmpReturnControlSocket(t)
	entries := map[string]ipsec.SPDPolicy{
		"discard-icmp-to-outside": {
			Name: "discard-icmp-to-outside", Action: dataplane.SPActionDiscard, Order: 100,
			Direction: ipsec.SPDDirOut, Protocol: sadSelProtoICMP,
			LocalPrefix: mustCIDRNet(t, "10.2.0.0/24"), LocalPort: ipsec.AnyPort(),
			RemotePrefix: mustCIDRNet(t, "10.1.0.0/24"), RemotePort: ipsec.AnyPort(),
		},
		// The forward check's permission for ICMP from the protected host. Its OUT
		// transit gate (10.2.0.2/32 to 10.1.0.0/24) ranks beneath the discard and holds
		// a selector of its own, so the discard decides the OUT lookup.
		"icmp-from-protected-host": {
			Name: "icmp-from-protected-host", Action: dataplane.SPActionBypass, Order: 200,
			Direction: ipsec.SPDDirIn, Protocol: sadSelProtoICMP,
			LocalPrefix: mustCIDRNet(t, "10.1.0.0/24"), LocalPort: ipsec.AnyPort(),
			RemotePrefix: mustCIDRNet(t, fragmentHostAddress+"/32"), RemotePort: ipsec.AnyPort(),
		},
	}
	if err := verifySPDOrder(&ipsec.IPsecConfig{Policies: entries}); err != nil {
		t.Fatalf("the operator SPD under test is one Ze refuses at commit: %v", err)
	}
	if err := reconcileForTest(dataplane.Get(), nil, entries, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("install the operator SPD entries: %v", err)
	}
	t.Cleanup(func() { removeSPDPolicies(dataplane.Get(), entries, slogutil.DiscardLogger()) })
	cases := []struct {
		name              string
		typ, code         byte
		quotedDestination string
		mapped            bool
	}{
		{"destination unreachable quoting the return traffic", 3, 1, fragmentHostAddress, true},
		{"echo request, not an error type", 8, 0, fragmentHostAddress, false},
		{"destination unreachable quoting traffic no SA carries", 3, 1, icmpForwardUnmatched, false},
	}
	for index, tc := range cases {
		tag := uint16(0x7a50 + index*2)
		packet := icmpReturnForwarded(tc.typ, tc.code, tc.quotedDestination, tag)
		blocked := sadSelStat(t, outStepStatPolBlock)
		fragmentSend(t, hostSender, packet)
		icmpReturnSendControl(t, controlSender, tag+1)
		icmpReturnObserve(t, capture, decoder, packet, tag, false, tc.mapped)
		if got := sadSelStat(t, outStepStatPolBlock) - blocked; got != 1 {
			t.Errorf("%s: XfrmOutPolBlock moved by %d, want 1: the message did not meet the operator discard", tc.name, got)
		}
	}
}

// transitEchoPacket is an ICMP Echo Request from source to target, tagged id in its
// IPv4 identification, which transitArrived matches.
func transitEchoPacket(source, target string, id uint16) []byte {
	packet := sadSelIPv4(source, sadSelProtoICMP, sadSelICMPEcho())
	copy(packet[16:20], net.ParseIP(target).To4())
	fragmentPacketID(packet, id)
	return packet
}

// TestRFC4301OperatorBypassForwardsTransitEchoInClear checks that the disposition of a
// non-error ICMP message a gateway forwards, not addressed to Ze, is the one an
// operator SPD entry gives it. RFC 4301 Section 6: "Disposition of non-error, ICMP
// messages (that are not addressed to the IPsec implementation itself) MUST be
// explicitly accounted for using SPD entries."
//
// Method: the netns lab with the protected host 10.2.0.2 behind Ze and the outside
// peer 10.1.0.5, the catch-all `unmatched discard` (espPolSPD) and no Child SA. In each
// forwarding direction an ICMP Echo Request is first sent with no operator entry and
// must not leave Ze towards its destination, so the catch-all and not some other path
// decides it. A both-direction operator BYPASS for ICMP between 10.2.0.0/24 and
// 10.1.0.0/24, checked by the configuration verifier and installed through the
// reconciler config apply uses, then carries a second Echo Request in each direction,
// which must be seen leaving Ze in clear on the link towards its destination. The
// negative, an operator DISCARD covering the same Echo Request, is
// TestRFC4301ForwardedICMPErrorUnderOperatorDiscardMapsToReturnSA.
//
// RFC requirement: RFC4301-6-1 positive -- under `unmatched discard` with no Child SA, a forwarded ICMP Echo Request between the protected host and the outside peer, in each direction, is not forwarded while no operator entry names it, and is seen leaving Ze in clear towards its destination once an operator both-direction BYPASS for ICMP between 10.2.0.0/24 and 10.1.0.0/24 is installed.
//
//ze:guest-root
func TestRFC4301OperatorBypassForwardsTransitEchoInClear(t *testing.T) {
	lab := newEspPolLab(t)
	hostSender, peerCapture := fragmentForwardPeers(t, lab)
	espPolSPD(t)

	lab.enter(t, lab.peer)
	if err := netlink.RouteAdd(&netlink.Route{
		LinkIndex: espPolLink(t, espPolPeerLink).Attrs().Index,
		Dst:       mustCIDRNet(t, "10.2.0.0/24"),
		Gw:        net.ParseIP(espPolZeOuter),
	}); err != nil {
		t.Fatalf("route the peer's cleartext towards the protected host through Ze: %v", err)
	}
	peerSender := fragmentRawSender(t, sadSelInnerRemote)
	lab.enter(t, lab.ze)
	hostEgress := transitEgressCapture(t, espPolLink(t, fragmentRouterLink).Attrs().Index)

	directions := []struct {
		name           string
		sender         *ipv4.RawConn
		capture        int
		source, target string
		id             uint16
	}{
		{"remote to local", peerSender, hostEgress, sadSelInnerRemote, fragmentHostAddress, 0x7d01},
		{"local to remote", hostSender, peerCapture, fragmentHostAddress, sadSelInnerRemote, 0x7d11},
	}
	for _, tc := range directions {
		fragmentSend(t, tc.sender, transitEchoPacket(tc.source, tc.target, tc.id))
		if transitArrived(t, tc.capture, sadSelProtoICMP, tc.source, tc.target, tc.id) {
			t.Fatalf("%s: an Echo Request no operator entry names (%s to %s) was forwarded under `unmatched discard`, "+
				"so the bypass below could not be what admits it", tc.name, tc.source, tc.target)
		}
	}

	entries := map[string]ipsec.SPDPolicy{
		"transit-echo": {
			Name: "transit-echo", Action: dataplane.SPActionBypass, Order: 1000,
			Direction: ipsec.SPDDirBoth, Protocol: sadSelProtoICMP,
			LocalPrefix: mustCIDRNet(t, "10.2.0.0/24"), LocalPort: ipsec.AnyPort(),
			RemotePrefix: mustCIDRNet(t, "10.1.0.0/24"), RemotePort: ipsec.AnyPort(),
		},
	}
	if err := verifySPDOrder(&ipsec.IPsecConfig{Policies: entries}); err != nil {
		t.Fatalf("the operator SPD under test is one Ze refuses at commit: %v", err)
	}
	log := slogutil.DiscardLogger()
	// RFC 4301 Section 6.
	if err := reconcileForTest(dataplane.Get(), nil, entries, log); err != nil {
		t.Fatalf("install the operator SPD entry: %v", err)
	}
	t.Cleanup(func() { removeSPDPolicies(dataplane.Get(), entries, log) })
	transitWantInstalled(t, entries)

	for _, tc := range directions {
		id := tc.id + 1
		fragmentSend(t, tc.sender, transitEchoPacket(tc.source, tc.target, id))
		if !transitArrived(t, tc.capture, sadSelProtoICMP, tc.source, tc.target, id) {
			t.Errorf("%s: the Echo Request the operator bypass names (%s to %s) was not forwarded in clear "+
				"(XfrmInPolBlock=%d, XfrmInNoPols=%d, XfrmOutPolBlock=%d)", tc.name, tc.source, tc.target,
				sadSelStat(t, inStepStatPolBlock), sadSelStat(t, "XfrmInNoPols"), sadSelStat(t, outStepStatPolBlock))
		}
	}
}

func localICMPReturn(t *testing.T, check bool) {
	t.Helper()
	lab := newEspPolLab(t)
	espPolSPD(t)
	child := icmpForwardChild(t, 0, check)
	decoder := newICMPReturnDecoder(t, child, check)
	sender := newEspPolSender(t, lab, child)
	capture := icmpReturnPeerCapture(t, lab)
	tag := uint16(0x7a30)
	trigger := sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(sadSelPort))
	fragmentPacketID(trigger, tag)
	// No UDP socket binds this port: the actual Ze namespace generates the error.
	before := icmpReturnNoPorts(t)
	sender.sendEncap(t, trigger)
	awaitForwardKernelCounter(t, "the authenticated trigger's closed UDP port", before+1, icmpReturnNoPorts)
	controlSender := icmpReturnControlSocket(t)
	icmpReturnSendControl(t, controlSender, tag+1)
	icmpReturnObserve(t, capture, decoder, trigger, tag, true, check)
}

// TestRFC4301LocalGeneratedICMPUsesReturnSA checks that an ICMP error Ze's own
// namespace generates, for an authenticated UDP trigger to a closed port received on a
// Child SA that carries the mapping flag, leaves through that Child SA's return SA.
// Method: the trigger arrives encapsulated, the namespace's UDP NoPorts counter rises by
// one, and the outside peer's capture decrypts the error beside a UDP control.
//
// RFC requirement: RFC4301-6.2-3 positive -- under `unmatched discard`, the ICMP error Ze generates for an authenticated trigger received on the Child SA, which no SPD entry carries, is decrypted at the outside peer from that Child SA's return SA and is not seen there in clear.
//
//ze:guest-root
func TestRFC4301LocalGeneratedICMPUsesReturnSA(t *testing.T) {
	localICMPReturn(t, true)
}

// TestRFC4301LocalGeneratedICMPWithoutMappingFlagDoesNotUseReturnSA is the control of
// TestRFC4301LocalGeneratedICMPUsesReturnSA: it shows that the positive's error leaves on
// the return SA because the Child SA carries the mapping flag, and not on any Child SA.
// Method: the same authenticated trigger to a closed port arrives on a Child SA whose
// peer configuration skips the ICMP error check (PeerCfg.SkipICMPErrorCheck), so its
// inbound state carries no XFRM_STATE_ICMP. The namespace's UDP NoPorts counter still
// rises by one, and the tagged UDP control still leaves through the same SA, so the SA
// is live; the outside peer's capture must then see the generated error neither decrypted
// from that SA nor in clear within the observation window.
// It carries no RFC tag: it bounds the positive, and proves no clause on its own.
//
//ze:guest-root
func TestRFC4301LocalGeneratedICMPWithoutMappingFlagDoesNotUseReturnSA(t *testing.T) {
	localICMPReturn(t, false)
}

//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- ICMP error messages carried by a Child SA.
// Related: rfc4301_forward_icmp_integration_linux_test.go -- real Child creator and namespace lab.
// RFC naming: untagged -- actual return packets must be observed before conformance is claimed.

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
	for _, policy := range policies {
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

//ze:guest-root
func TestRFC4301ForwardedICMPErrorsMapUnderDiscard(t *testing.T) {
	lab := newEspPolLab(t)
	hostSender, capture := fragmentForwardPeers(t, lab)
	espPolSPD(t)
	child := icmpForwardChild(t, 0, true)
	decoder := newICMPReturnDecoder(t, child, true)
	controlSender := icmpReturnControlSocket(t)
	// This is the operator's receive permission for errors from the protected side.
	// Its missing FWD projection is part of this gateway path, not a test-only bypass.
	entries := map[string]ipsec.SPDPolicy{"icmp-from-protected-host": {
		Name: "icmp-from-protected-host", Action: dataplane.SPActionBypass, Order: 1000,
		Direction: ipsec.SPDDirIn, Protocol: sadSelProtoICMP,
		LocalPrefix: mustCIDRNet(t, "10.1.0.0/24"), LocalPort: ipsec.AnyPort(),
		RemotePrefix: mustCIDRNet(t, "10.2.0.0/24"), RemotePort: ipsec.AnyPort(),
		ICMP: ipsec.ICMPSelector{HasType: true, Type: 3},
	}}
	installSPDPolicies(dataplane.Get(), nil, entries, slogutil.DiscardLogger())
	t.Cleanup(func() { removeSPDPolicies(dataplane.Get(), entries, slogutil.DiscardLogger()) })
	for index, quotedDestination := range []string{fragmentHostAddress, icmpForwardUnmatched} {
		tag := uint16(0x7a10 + index*2)
		quote := icmpReturnUDP(sadSelInnerRemote, quotedDestination, tag)
		// Reverse quote: local source port 5000, remote destination port 40000.
		binary.BigEndian.PutUint16(quote[20:22], sadSelSourcePort)
		binary.BigEndian.PutUint16(quote[22:24], sadSelPort)
		message := make([]byte, 8, 36)
		message[0], message[1] = 3, 1
		message = append(message, quote[:28]...)
		binary.BigEndian.PutUint16(message[2:4], sadSelChecksum(message))
		packet := sadSelIPv4(fragmentHostAddress, sadSelProtoICMP, message)
		copy(packet[16:20], net.ParseIP(sadSelInnerRemote).To4())
		fragmentPacketID(packet, tag)
		fragmentSend(t, hostSender, packet)
		icmpReturnSendControl(t, controlSender, tag+1)
		icmpReturnObserve(t, capture, decoder, packet, tag, false, index == 0)
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

//ze:guest-root
func TestRFC4301LocalGeneratedICMPUsesReturnSA(t *testing.T) {
	localICMPReturn(t, true)
}

//ze:guest-root
func TestRFC4301LocalGeneratedICMPWithoutMappingFlagDoesNotUseReturnSA(t *testing.T) {
	localICMPReturn(t, false)
}

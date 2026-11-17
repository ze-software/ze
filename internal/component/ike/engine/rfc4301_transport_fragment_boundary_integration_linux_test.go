//go:build integration && linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- transport fragment boundary.
// Related: rfc4301_espform_policy_integration_linux_test.go -- isolated veth lab and ESP seal.

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
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

const (
	transportFragmentLocal      = "2001:db8:43::1"
	transportFragmentRemote     = "2001:db8:43::2"
	transportFragmentSPI        = 0x6f430001
	transportFragmentTunnelSPI  = 0x6f430002
	transportFragmentPort       = 5043
	transportFragmentTunnelPort = 5044
)

// TestRFC4301TransportRejectsAuthenticatedIPv6PlaintextFragments observes the
// authenticated transport boundary before IPv6 reassembly. Whole datagrams,
// atomic fragments and fragmented outer ciphertext remain deliverable.
//
// RFC 4301 Section 4.1: "For IPv6, it would be feasible to carry a plaintext
// fragment on a transport mode SA; however, for simplicity, this restriction
// also applies to IPv6 packets."
//
// The fixture deliberately installs a legacy SA through production InstallSA,
// with an exact address selector and Scope nil. The boundary is also owed to
// manually keyed callers and must be testable before full-scope admission.
// No new-scope or IKE negotiation claim is made by this lower-layer proof.
//
//ze:guest-root
func TestRFC4301TransportRejectsAuthenticatedIPv6PlaintextFragments(t *testing.T) {
	lab := newEspPolLab(t)
	sender, sealed := transportFragmentSetup(t, lab)
	listener := transportFragmentListen(t, transportFragmentPort)
	whole := transportFragmentUDP(transportFragmentPort, []byte("whole-control"))
	transportFragmentSend(t, sender, sealed.seal(unix.IPPROTO_UDP, whole))
	if !fragmentContextDelivered(t, listener, whole[8:]) {
		t.Fatal("the authenticated whole IPv6 transport control was not delivered")
	}
	transportFragmentWantPackets(t, transportFragmentSPI, 1)

	for index, options := range []bool{false, true} {
		label := "Fragment"
		if options {
			label = "DestinationOptions then Fragment"
		}
		packet := transportFragmentUDP(transportFragmentPort, []byte("atomic-control"))
		atomic := transportFragmentHeader(packet, 0, uint32(0x4300+index))
		next, atomic := transportFragmentOptions(atomic, options)
		transportFragmentSend(t, sender, sealed.seal(next, atomic))
		if !fragmentContextDelivered(t, listener, packet[8:]) {
			t.Fatalf("%s: the valid atomic transport fragment was not delivered", label)
		}
		for part, offset := range []uint16{1, 16} {
			before := espPolStat(t, "XfrmInStateModeError")
			reassembly := transportFragmentReassembly(t, "Ip6ReasmReqds")
			payload := packet[:16]
			if part == 1 {
				payload = packet[16:]
			}
			fragment := transportFragmentHeader(payload, offset, uint32(0x4400+index*2+part))
			next, fragment := transportFragmentOptions(fragment, options)
			transportFragmentSend(t, sender, sealed.seal(next, fragment))
			if fragmentContextDelivered(t, listener, packet[8:]) {
				t.Errorf("%s offset=%#x: authenticated plaintext fragment was delivered", label, offset)
			}
			if got := espPolStat(t, "XfrmInStateModeError"); got != before+1 {
				t.Errorf("%s offset=%#x: state mode rejection counter = %d, want %d; fragment was not rejected at its authenticated transport boundary", label, offset, got, before+1)
			}
			if got := transportFragmentReassembly(t, "Ip6ReasmReqds"); got != reassembly {
				t.Errorf("%s offset=%#x: IPv6 reassembly requests moved %d -> %d; transport plaintext reached a fragment queue", label, offset, reassembly, got)
			}
			transportFragmentSend(t, sender, sealed.seal(unix.IPPROTO_UDP, whole))
			if !fragmentContextDelivered(t, listener, whole[8:]) {
				t.Fatalf("%s offset=%#x: lost the subsequent authenticated whole control", label, offset)
			}
		}
	}

	// Fragmentation after encryption is permitted. The peer's real IPv6 output
	// fragments this ESP ciphertext; the receiver must reassemble before auth.
	body := make([]byte, espPolLargeBodyOctets)
	for index := range body {
		body[index] = byte(index)
	}
	large := transportFragmentUDP(transportFragmentPort, body)
	capture := transportFragmentCapture(t)
	transportFragmentSend(t, sender, sealed.seal(unix.IPPROTO_UDP, large))
	transportFragmentWantBody(t, listener, body)
	transportFragmentWantCiphertextFragments(t, capture)
}

// TestRFC4301NestedTransportKeepsTunnelFragments permits genuine tunnel plaintext
// fragments when an earlier authenticated transport state remains in secpath.
// The Fragment header belongs to the current tunnel, not that historical state.
//
//ze:guest-root
func TestRFC4301NestedTransportKeepsTunnelFragments(t *testing.T) {
	lab := newEspPolLab(t)
	sender, outer := transportFragmentSetup(t, lab)
	inner := transportFragmentState(t, transportFragmentTunnelSPI, dataplane.ModeTunnel,
		transportFragmentTunnelPort)
	listener := transportFragmentListen(t, transportFragmentTunnelPort)
	packet := transportFragmentUDP(transportFragmentTunnelPort, []byte("nested-tunnel-fragments"))
	control := transportFragmentIPv6(unix.IPPROTO_UDP, packet)
	transportFragmentSend(t, sender, outer.seal(unix.IPPROTO_ESP, inner.seal(unix.IPPROTO_IPV6, control)))
	if !fragmentContextDelivered(t, listener, packet[8:]) {
		t.Fatal("nested transport-to-tunnel whole control was not delivered")
	}
	for index, offset := range []uint16{16, 1} {
		payload := packet[16:]
		if index == 1 {
			payload = packet[:16]
		}
		fragment := transportFragmentHeader(payload, offset, 0x45ab)
		innerPacket := transportFragmentIPv6(unix.IPPROTO_FRAGMENT, fragment)
		transportFragmentSend(t, sender, outer.seal(unix.IPPROTO_ESP,
			inner.seal(unix.IPPROTO_IPV6, innerPacket)))
	}
	if !fragmentContextDelivered(t, listener, packet[8:]) {
		t.Fatal("tunnel fragments were refused because a historical transport SA remained in secpath")
	}
	transportFragmentWantPackets(t, transportFragmentSPI, 3)
	transportFragmentWantPackets(t, transportFragmentTunnelSPI, 3)
}

// transportFragmentSetup MUST run on the lab's locked thread and return it to
// Ze's namespace. The socket retains the private peer's network namespace.
func transportFragmentSetup(t *testing.T, lab *espPolLab) (*net.IPConn, *espPolSender) {
	t.Helper()
	for _, entry := range []struct{ path, value string }{
		{"/proc/sys/net/ipv6/conf/all/disable_ipv6", "0"},
		{"/proc/sys/net/ipv6/conf/default/disable_ipv6", "0"},
	} {
		if err := os.WriteFile(entry.path, []byte(entry.value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mhBoundaryAddress(t, espPolZeLink, transportFragmentLocal+"/64")
	localMAC := espPolLink(t, espPolZeLink).Attrs().HardwareAddr
	lab.enter(t, lab.peer)
	for _, path := range []string{"/proc/sys/net/ipv6/conf/all/disable_ipv6", "/proc/sys/net/ipv6/conf/default/disable_ipv6"} {
		if err := os.WriteFile(path, []byte("0"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mhBoundaryAddress(t, espPolPeerLink, transportFragmentRemote+"/64")
	if err := netlink.NeighAdd(&netlink.Neigh{
		LinkIndex: espPolLink(t, espPolPeerLink).Attrs().Index, Family: unix.AF_INET6,
		State: netlink.NUD_PERMANENT, IP: net.ParseIP(transportFragmentLocal), HardwareAddr: localMAC,
	}); err != nil {
		t.Fatalf("add the private IPv6 ESP peer neighbor: %v", err)
	}
	sender, err := net.ListenIP("ip6:50", &net.IPAddr{IP: net.ParseIP(transportFragmentRemote)})
	lab.enter(t, lab.ze)
	if err != nil {
		t.Fatalf("open the private IPv6 ESP sender: %v", err)
	}
	t.Cleanup(func() {
		if err := sender.Close(); err != nil {
			t.Errorf("close the private IPv6 ESP sender: %v", err)
		}
	})
	raw, err := sender.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var optionErr error
	if err := raw.Control(func(fd uintptr) {
		optionErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_MTU_DISCOVER, unix.IPV6_PMTUDISC_DONT)
	}); err != nil {
		t.Fatal(err)
	}
	if optionErr != nil {
		t.Fatalf("permit IPv6 fragmentation of outer ESP: %v", optionErr)
	}
	if err := dataplane.Load("xfrm"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Errorf("close the transport fragment dataplane: %v", err)
		}
	})
	return sender, transportFragmentState(t, transportFragmentSPI, dataplane.ModeTransport,
		transportFragmentPort)
}

func transportFragmentState(t *testing.T, spi uint32, mode uint8, port uint16) *espPolSender {
	t.Helper()
	key := make([]byte, 36)
	for index := range key {
		key[index] = byte(index + 1)
	}
	source := mustCIDRNet(t, transportFragmentRemote+"/128")
	target := mustCIDRNet(t, transportFragmentLocal+"/128")
	params := dataplane.SAParams{
		SPI: spi, Src: source.IP, Dst: target.IP, Dir: dataplane.SADirIn,
		Proto: dataplane.ProtoESP, Mode: mode, ReqID: spi, ReplayWin: 32,
		EncAlgo: "aes256gcm", EncKey: key, IsAEAD: true,
		Sel: &dataplane.SASelector{Src: source, Dst: target},
	}
	backend := dataplane.Get()
	if err := backend.InstallSA(params); err != nil {
		t.Fatalf("install legacy transport-boundary SA %#x: %v", spi, err)
	}
	t.Cleanup(func() {
		if err := backend.RemoveSA(spi, target.IP, dataplane.ProtoESP); err != nil {
			t.Errorf("remove transport-boundary SA %#x: %v", spi, err)
		}
	})
	policy := dataplane.SPParams{
		Src: source, Dst: target, Dir: dataplane.SADirIn, Proto: dataplane.ProtoESP,
		Mode: mode, ReqID: spi, UpperProto: unix.IPPROTO_UDP,
		DstPort: dataplane.ExactPortMatch(port), Priority: dataplane.PriorityChildSA,
	}
	if mode == dataplane.ModeTunnel {
		policy.TunnelSrc, policy.TunnelDst = source.IP, target.IP
	}
	if err := backend.InstallPolicy(policy); err != nil {
		t.Fatalf("install transport-boundary policy: %v", err)
	}
	t.Cleanup(func() {
		if err := backend.RemovePolicyParams(policy); err != nil {
			t.Errorf("remove transport-boundary policy: %v", err)
		}
	})
	block, err := aes.NewCipher(key[:32])
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	return &espPolSender{spi: spi, aead: aead, salt: key[32:]}
}

func transportFragmentSend(t *testing.T, sender *net.IPConn, packet []byte) {
	t.Helper()
	if _, err := sender.WriteToIP(packet, &net.IPAddr{IP: net.ParseIP(transportFragmentLocal)}); err != nil {
		t.Fatalf("send authenticated private IPv6 ESP: %v", err)
	}
}

func transportFragmentListen(t *testing.T, port int) *net.UDPConn {
	t.Helper()
	listener, err := net.ListenUDP("udp6", &net.UDPAddr{IP: net.ParseIP(transportFragmentLocal), Port: port})
	if err != nil {
		t.Fatalf("open the private IPv6 transport receiver: %v", err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close the private IPv6 transport receiver: %v", err)
		}
	})
	return listener
}

func transportFragmentHeader(payload []byte, offset uint16, identifier uint32) []byte {
	header := make([]byte, 8, 8+len(payload))
	header[0] = unix.IPPROTO_UDP
	binary.BigEndian.PutUint16(header[2:4], offset)
	binary.BigEndian.PutUint32(header[4:8], identifier)
	return append(header, payload...)
}

func transportFragmentOptions(fragment []byte, options bool) (byte, []byte) {
	if options {
		return unix.IPPROTO_DSTOPTS, append([]byte{unix.IPPROTO_FRAGMENT, 0, 1, 4, 0, 0, 0, 0}, fragment...)
	}
	return unix.IPPROTO_FRAGMENT, fragment
}

func transportFragmentIPv6(next byte, payload []byte) []byte {
	header := make([]byte, 40, 40+len(payload))
	header[0], header[6], header[7] = 0x60, next, 64
	binary.BigEndian.PutUint16(header[4:6], uint16(len(payload)))
	copy(header[8:24], net.ParseIP(transportFragmentRemote).To16())
	copy(header[24:40], net.ParseIP(transportFragmentLocal).To16())
	return append(header, payload...)
}

func transportFragmentUDP(port uint16, body []byte) []byte {
	packet := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint16(packet[0:2], sadSelSourcePort)
	binary.BigEndian.PutUint16(packet[2:4], port)
	binary.BigEndian.PutUint16(packet[4:6], uint16(8+len(body)))
	packet = append(packet, body...)
	pseudo := make([]byte, 40, 40+len(packet))
	copy(pseudo[0:16], net.ParseIP(transportFragmentRemote).To16())
	copy(pseudo[16:32], net.ParseIP(transportFragmentLocal).To16())
	binary.BigEndian.PutUint32(pseudo[32:36], uint32(len(packet)))
	pseudo[39] = unix.IPPROTO_UDP
	pseudo = append(pseudo, packet...)
	if len(pseudo)%2 != 0 {
		pseudo = append(pseudo, 0)
	}
	checksum := sadSelChecksum(pseudo)
	if checksum == 0 {
		checksum = 0xffff
	}
	binary.BigEndian.PutUint16(packet[6:8], checksum)
	return packet
}

func transportFragmentReassembly(t *testing.T, name string) uint64 {
	t.Helper()
	contents, err := os.ReadFile("/proc/thread-self/net/snmp6")
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[0] != name {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	t.Fatalf("IPv6 reassembly counter %s is absent", name)
	return 0
}

func transportFragmentWantPackets(t *testing.T, spi uint32, want uint64) {
	t.Helper()
	state, err := netlink.XfrmStateGet(&netlink.XfrmState{
		Dst: net.ParseIP(transportFragmentLocal), Spi: int(spi), Proto: netlink.XFRM_PROTO_ESP,
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.Statistics.Packets != want {
		t.Errorf("authenticated receiving SA %#x accepted %d packets, want %d", spi, state.Statistics.Packets, want)
	}
}

func transportFragmentWantBody(t *testing.T, listener *net.UDPConn, body []byte) {
	t.Helper()
	// The existing exact-body receiver is intentionally bounded at 256 bytes;
	// the outer-fragment positive needs the same check at the larger payload.
	if err := listener.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, len(body)+1)
	n, address, err := listener.ReadFromUDP(buffer)
	if err != nil {
		t.Fatalf("receive outer-fragmented authenticated transport control: %v", err)
	}
	if !bytes.Equal(buffer[:n], body) {
		t.Fatal("outer-fragmented transport control changed its payload")
	}
	if !address.IP.Equal(net.ParseIP(transportFragmentRemote)) {
		t.Fatalf("outer-fragmented control source = %s, want %s", address.IP, transportFragmentRemote)
	}
}

// transportFragmentCapture observes the actual ingress before either local IPv6
// reassembly or the guard's conntrack gather. The latter has no SNMP reassembly
// counter, so a counter alone cannot establish that encrypted fragments arrived.
func transportFragmentCapture(t *testing.T) int {
	t.Helper()
	var etherType [2]byte
	binary.BigEndian.PutUint16(etherType[:], unix.ETH_P_IPV6)
	protocol := binary.NativeEndian.Uint16(etherType[:])
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, int(protocol))
	if err != nil {
		t.Fatalf("open the private IPv6 ingress capture: %v", err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Errorf("close the private IPv6 ingress capture: %v", err)
		}
	})
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{
		Ifindex: espPolLink(t, espPolZeLink).Attrs().Index, Protocol: protocol,
	}); err != nil {
		t.Fatalf("bind the private IPv6 ingress capture: %v", err)
	}
	if err := unix.SetsockoptInt(fd, unix.SOL_PACKET, unix.PACKET_IGNORE_OUTGOING, 1); err != nil {
		t.Fatalf("observe only incoming IPv6 ESP: %v", err)
	}
	return fd
}

func transportFragmentWantCiphertextFragments(t *testing.T, fd int) {
	t.Helper()
	var buffer [4096]byte
	var identifier uint32
	initial, tails := 0, 0
	for range 64 {
		n, _, err := unix.Recvfrom(fd, buffer[:], 0)
		if errors.Is(err, unix.EAGAIN) {
			break
		}
		if err != nil {
			t.Fatalf("read the actual IPv6 ingress capture: %v", err)
		}
		if n < 48 {
			continue
		}
		if buffer[6] != unix.IPPROTO_FRAGMENT {
			continue
		}
		if !net.IP(buffer[8:24]).Equal(net.ParseIP(transportFragmentRemote)) {
			continue
		}
		if !net.IP(buffer[24:40]).Equal(net.ParseIP(transportFragmentLocal)) {
			continue
		}
		if buffer[40] != unix.IPPROTO_ESP {
			continue
		}
		offset := binary.BigEndian.Uint16(buffer[42:44])
		if offset&0xfff8 == 0 {
			if offset&1 == 0 {
				t.Fatal("the supposed outer ciphertext fragment is atomic")
			}
			if n < 52 {
				t.Fatal("the initial outer ciphertext fragment has no ESP SPI")
			}
			if got := binary.BigEndian.Uint32(buffer[48:52]); got != transportFragmentSPI {
				t.Fatalf("outer ciphertext fragment SPI = %#x, want %#x", got, transportFragmentSPI)
			}
			identifier = binary.BigEndian.Uint32(buffer[44:48])
			initial++
			continue
		}
		if initial == 0 {
			t.Fatal("outer ciphertext tail preceded its initial fragment in the ordered ingress capture")
		}
		if got := binary.BigEndian.Uint32(buffer[44:48]); got != identifier {
			t.Fatalf("outer ciphertext tail identifier = %#x, want %#x", got, identifier)
		}
		tails++
	}
	if initial != 1 {
		t.Errorf("accepted outer ciphertext arrived in %d initial fragments, want one", initial)
	}
	if tails == 0 {
		t.Error("accepted outer ciphertext arrived with no noninitial fragment")
	}
}

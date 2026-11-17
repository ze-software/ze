//go:build integration && linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- transport fragment boundary.
// Related: rfc4301_transport_fragment_boundary_integration_linux_test.go -- receive boundary.
// Related: rfc4301_ipv6_fragment_policy_integration_linux_test.go -- raw IPv6 output helpers.
// RFC naming: untagged -- transport plaintext output rejection awaiting native kernel proof.

package engine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

const transportOutputSPI = 0x6f430005

// TestRFC4301TransportOutputRejectsIPv4PlaintextFragments sends complete IP
// headers through local raw output. Each fragment must fail before encryption;
// exact whole UDP deliveries before and after it keep the SA path observable.
// RFC 4301 Section 4.1: "AH and ESP cannot be applied using transport mode to IPv4
// packets that are fragments."
// MUTATION: remove the IPv4 fragment rejection in xfrm_output_fragment_allowed;
// the fragment must then advance the transmit SA instead of its mode-error counter.
//
//ze:guest-root
func TestRFC4301TransportOutputRejectsIPv4PlaintextFragments(t *testing.T) {
	lab := newEspPolLab(t)
	transportOutputExercise(t, lab, mustCIDRNet(t, espPolZeOuter+"/32"),
		mustCIDRNet(t, espPolPeerOuter+"/32"))
}

// TestRFC4301TransportOutputRejectsIPv6PlaintextFragments exercises the same
// transform boundary with direct and Destination Options Fragment headers.
// Atomic fragments and whole UDP datagrams remain permitted.
// RFC 4301 Section 4.1: "For IPv6, it would be feasible to carry a plaintext
// fragment on a transport mode SA; however, for simplicity, this restriction
// also applies to IPv6 packets."
// MUTATION: remove the IPv6 fragment rejection in xfrm_output_fragment_allowed;
// the fragment must then advance the transmit SA instead of its mode-error counter.
//
//ze:guest-root
func TestRFC4301TransportOutputRejectsIPv6PlaintextFragments(t *testing.T) {
	lab := newEspPolLab(t)
	ipv6FragmentPolicyAddresses(t, lab)
	transportOutputExercise(t, lab, mustCIDRNet(t, transportFragmentRemote+"/128"),
		mustCIDRNet(t, transportFragmentLocal+"/128"))
}

func transportOutputExercise(t *testing.T, lab *espPolLab, source, target *net.IPNet) {
	t.Helper()
	if err := dataplane.Load("xfrm"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		lab.enter(t, lab.ze)
		if err := dataplane.CloseBackend(); err != nil {
			t.Errorf("close the transport output backend: %v", err)
		}
	})
	transportOutputInstall(t, lab, lab.ze, dataplane.SADirOut, source, target)
	transportOutputInstall(t, lab, lab.peer, dataplane.SADirIn, source, target)
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: target.IP, Port: transportFragmentPort})
	if err != nil {
		t.Fatalf("open the transport output receiver: %v", err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close the transport output receiver: %v", err)
		}
	})
	lab.enter(t, lab.ze)
	fd := transportOutputSender(t, source.IP)
	var accepted uint64
	control := func(label string, atomic bool, options bool) {
		body := []byte("transport output permitted " + label)
		udp := transportOutputUDP(source.IP, body)
		var packet []byte
		if source.IP.To4() == nil {
			next, payload := byte(unix.IPPROTO_UDP), udp
			if atomic {
				next, payload = transportFragmentOptions(
					transportFragmentHeader(udp, 0, uint32(0x43050000+accepted)), options)
			}
			packet = transportFragmentIPv6(next, payload)
		} else {
			packet = transportOutputIPv4(source.IP, target.IP, udp, 0, 0)
		}
		before := espPolStat(t, "XfrmOutStateModeError")
		transportOutputSend(t, fd, target.IP, packet, false)
		transportOutputBody(t, listener, source.IP, body)
		accepted++
		transportOutputWantPackets(t, target.IP, accepted)
		if got := espPolStat(t, "XfrmOutStateModeError"); got != before {
			t.Fatalf("%s control changed XfrmOutStateModeError %d -> %d", label, before, got)
		}
		lab.enter(t, lab.peer)
		transportOutputWantPackets(t, target.IP, accepted)
		transportOutputGuardIdle(t)
		lab.enter(t, lab.ze)
		transportOutputGuardIdle(t)
	}
	control("initial whole", false, false)
	optionForms := []bool{false}
	if source.IP.To4() == nil {
		optionForms = append(optionForms, true)
	}
	for optionIndex, options := range optionForms {
		if source.IP.To4() == nil {
			control(fmt.Sprintf("atomic options=%t", options), true, options)
		}
		for part := range 2 {
			label := fmt.Sprintf("options=%t part=%d", options, part)
			udp := transportOutputUDP(source.IP, []byte("forbidden plaintext fragment "+label))
			payload, offset := udp[:24], uint16(0x2000)
			if part == 1 {
				payload, offset = udp[24:], 3
			}
			identifier := uint32(0x43051000 + optionIndex*2 + part)
			var packet []byte
			if source.IP.To4() == nil {
				offset = 1
				if part == 1 {
					offset = 24
				}
				next, fragment := transportFragmentOptions(
					transportFragmentHeader(payload, offset, identifier), options)
				packet = transportFragmentIPv6(next, fragment)
			} else {
				packet = transportOutputIPv4(source.IP, target.IP, payload, offset, uint16(identifier))
			}
			transportOutputGuardIdle(t)
			before := espPolStat(t, "XfrmOutStateModeError")
			transportOutputSend(t, fd, target.IP, packet, true)
			if got := espPolStat(t, "XfrmOutStateModeError"); got != before+1 {
				t.Errorf("%s: XfrmOutStateModeError=%d, want %d; plaintext did not fail at the output transform", label, got, before+1)
			}
			transportOutputWantPackets(t, target.IP, accepted)
			control("whole after "+label, false, false)
		}
	}
}

// transportOutputInstall MUST run on the lab's locked thread. It leaves the
// thread in ns; cleanup returns there before removing its own policy and state.
// Legacy nil Scope with ANY protocol/ports is a valid direct InstallSA caller.
// It keeps guard_refs zero, so gathering cannot hide the output transform check.
func transportOutputInstall(t *testing.T, lab *espPolLab, ns netns.NsHandle,
	direction dataplane.SADir, source, target *net.IPNet,
) {
	t.Helper()
	lab.enter(t, ns)
	transportOutputGuardIdle(t)
	key := make([]byte, 36)
	for index := range key {
		key[index] = byte(index + 1)
	}
	backend := dataplane.Get()
	params := dataplane.SAParams{
		SPI: transportOutputSPI, Src: source.IP, Dst: target.IP, Dir: direction,
		Proto: dataplane.ProtoESP, Mode: dataplane.ModeTransport,
		ReqID: transportOutputSPI, ReplayWin: 32,
		EncAlgo: "aes256gcm", EncKey: key, IsAEAD: true,
		Sel: &dataplane.SASelector{Src: source, Dst: target, UpperProto: 0,
			SrcPort: dataplane.AnyPortMatch(), DstPort: dataplane.AnyPortMatch()},
		Scope: nil,
	}
	if err := backend.InstallSA(params); err != nil {
		t.Fatalf("install the legacy transport output state: %v", err)
	}
	t.Cleanup(func() {
		lab.enter(t, ns)
		if err := backend.RemoveSA(transportOutputSPI, target.IP, dataplane.ProtoESP); err != nil {
			t.Errorf("remove the legacy transport output state: %v", err)
		}
	})
	policy := dataplane.SPParams{
		Src: source, Dst: target, Dir: direction, Proto: dataplane.ProtoESP,
		Mode: dataplane.ModeTransport, ReqID: transportOutputSPI, UpperProto: 0,
		SrcPort: dataplane.AnyPortMatch(), DstPort: dataplane.AnyPortMatch(),
		Priority: dataplane.PriorityChildSA,
	}
	if err := backend.InstallPolicy(policy); err != nil {
		t.Fatalf("install the ANY transport output template policy: %v", err)
	}
	t.Cleanup(func() {
		lab.enter(t, ns)
		if err := backend.RemovePolicyParams(policy); err != nil {
			t.Errorf("remove the ANY transport output template policy: %v", err)
		}
	})
	transportOutputGuardIdle(t)
	transportOutputWantPackets(t, target.IP, 0)
}

func transportOutputGuardIdle(t *testing.T) {
	t.Helper()
	caps := rfcScopeCapabilities(t)
	if caps[3] != rfcScopeABIVersion {
		t.Fatalf("transport output capability ABI=%#x, want %#x", caps[3], rfcScopeABIVersion)
	}
	if caps[2] != 0 {
		t.Fatalf("transport output namespace has %d guard references; gathering would mask the transform boundary", caps[2])
	}
}

func transportOutputWantPackets(t *testing.T, target net.IP, want uint64) {
	t.Helper()
	state, err := netlink.XfrmStateGet(&netlink.XfrmState{
		Dst: target, Spi: transportOutputSPI, Proto: netlink.XFRM_PROTO_ESP,
	})
	if err != nil {
		t.Fatalf("read the transport output SA: %v", err)
	}
	if got := state.Statistics.Packets; got != want {
		t.Errorf("transport SA %#x processed %d packets, want %d", transportOutputSPI, got, want)
	}
}

func transportOutputSender(t *testing.T, source net.IP) int {
	t.Helper()
	if source.To4() == nil {
		fd := ipv6FragmentPolicySender(t)
		address := &unix.SockaddrInet6{}
		copy(address.Addr[:], source.To16())
		if err := unix.Bind(fd, address); err != nil {
			t.Fatalf("bind the raw IPv6 transport sender: %v", err)
		}
		return fd
	}
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.IPPROTO_RAW)
	if err != nil {
		t.Fatalf("open the raw IPv4 transport sender: %v", err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Errorf("close the raw IPv4 transport sender: %v", err)
		}
	})
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_HDRINCL, 1); err != nil {
		t.Fatal(err)
	}
	address := &unix.SockaddrInet4{}
	copy(address.Addr[:], source.To4())
	if err := unix.Bind(fd, address); err != nil {
		t.Fatalf("bind the raw IPv4 transport sender: %v", err)
	}
	return fd
}

func transportOutputSend(t *testing.T, fd int, target net.IP, packet []byte, fragment bool) {
	t.Helper()
	var address unix.Sockaddr
	if target.To4() == nil {
		v6 := &unix.SockaddrInet6{}
		copy(v6.Addr[:], target.To16())
		address = v6
	} else {
		v4 := &unix.SockaddrInet4{}
		copy(v4.Addr[:], target.To4())
		address = v4
	}
	err := unix.Sendto(fd, packet, 0, address)
	if err == nil {
		return
	}
	if fragment {
		if errors.Is(err, unix.EPERM) {
			return
		}
	}
	t.Fatalf("send raw transport output: %v", err)
}

func transportOutputUDP(source net.IP, body []byte) []byte {
	if source.To4() == nil {
		return transportFragmentUDP(transportFragmentPort, body)
	}
	// IPv4 permits a zero UDP checksum; IPv6 uses the existing pseudoheader builder.
	packet := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint16(packet[0:2], sadSelSourcePort)
	binary.BigEndian.PutUint16(packet[2:4], transportFragmentPort)
	binary.BigEndian.PutUint16(packet[4:6], uint16(8+len(body)))
	return append(packet, body...)
}

func transportOutputIPv4(source, target net.IP, payload []byte, offset, identifier uint16) []byte {
	header := make([]byte, 20, 20+len(payload))
	header[0], header[8], header[9] = 0x45, 64, unix.IPPROTO_UDP
	binary.BigEndian.PutUint16(header[2:4], uint16(20+len(payload)))
	binary.BigEndian.PutUint16(header[4:6], identifier)
	binary.BigEndian.PutUint16(header[6:8], offset)
	copy(header[12:16], source.To4())
	copy(header[16:20], target.To4())
	binary.BigEndian.PutUint16(header[10:12], sadSelChecksum(header))
	return append(header, payload...)
}

func transportOutputBody(t *testing.T, receiver *net.UDPConn, source net.IP, body []byte) {
	t.Helper()
	if err := receiver.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, len(body)+1)
	n, address, err := receiver.ReadFromUDP(buffer)
	if err != nil {
		t.Fatalf("receive the permitted transport output control: %v", err)
	}
	if !bytes.Equal(buffer[:n], body) {
		t.Fatalf("transport output payload=%q, want %q", buffer[:n], body)
	}
	if !address.IP.Equal(source) {
		t.Fatalf("transport output source=%s, want %s", address.IP, source)
	}
	if address.Port != sadSelSourcePort {
		t.Fatalf("transport output source port=%d, want %d", address.Port, sadSelSourcePort)
	}
}

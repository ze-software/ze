// VALIDATES: the platform-free half of the bfd-first-packet-link-local fixture:
//            the injected frame, the reading of Ze's captured frames, and the
//            selection of each link's session from `show bfd sessions`.
// PREVENTS:  a .ci that goes red or green in the QEMU guest for a fixture
//            defect (a bad checksum, a misread offset) rather than for Ze.

package fixture

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// TestBFDLinkLocalFrameIsWhatTheKernelAccepts checks the frame the link-local
// fixture injects, field by field, because a frame the kernel drops would make
// the .ci fail for a reason that has nothing to do with Ze: a wrong checksum is
// discarded before the socket, and a wrong hop limit is dropped by Ze's GTSM
// gate. The checksum is verified the way a receiver does it, by summing the
// pseudo-header and the segment with the checksum in place and expecting all
// ones.
func TestBFDLinkLocalFrameIsWhatTheKernelAccepts(t *testing.T) {
	link := bfdLinkLocalLinks()[1]
	frame := make([]byte, bfdLinkLocalFrameOctets)
	written, err := bfdLinkLocalFrame(frame, &link)
	if err != nil {
		t.Fatalf("bfdLinkLocalFrame: %v", err)
	}
	if written != bfdLinkLocalFrameOctets {
		t.Fatalf("wrote %d octets, want %d", written, bfdLinkLocalFrameOctets)
	}

	if got := frame[0:6]; string(got) != string(link.nearMAC) {
		t.Errorf("target MAC %x, want Ze's end %x", got, link.nearMAC)
	}
	if got := frame[6:12]; string(got) != string(link.farMAC) {
		t.Errorf("source MAC %x, want the far end %x", got, link.farMAC)
	}
	if got := binary.BigEndian.Uint16(frame[12:14]); got != 0x86dd {
		t.Errorf("ether type %#x, want 0x86dd", got)
	}

	ip := frame[14:]
	if got := ip[0] >> 4; got != 6 {
		t.Errorf("IP version %d, want 6", got)
	}
	if got := binary.BigEndian.Uint16(ip[4:6]); got != 8+packet.MandatoryLen {
		t.Errorf("payload length %d, want %d", got, 8+packet.MandatoryLen)
	}
	if ip[6] != 17 {
		t.Errorf("next header %d, want UDP (17)", ip[6])
	}
	if ip[7] != 255 {
		t.Errorf("hop limit %d, want 255", ip[7])
	}
	source := netip.AddrFrom16([16]byte(ip[8:24]))
	target := netip.AddrFrom16([16]byte(ip[24:40]))
	if source != netip.MustParseAddr("fe80::b") {
		t.Errorf("source %s, want fe80::b", source)
	}
	if target != netip.MustParseAddr("fe80::a") {
		t.Errorf("destination %s, want fe80::a", target)
	}

	udp := ip[40:]
	if got := binary.BigEndian.Uint16(udp[0:2]); got < 49152 {
		t.Errorf("source port %d, below the RFC 5881 Section 4 range", got)
	}
	if got := binary.BigEndian.Uint16(udp[2:4]); got != 3784 {
		t.Errorf("destination port %d, want 3784", got)
	}
	if got := binary.BigEndian.Uint16(udp[4:6]); int(got) != len(udp) {
		t.Errorf("UDP length %d, want %d", got, len(udp))
	}
	if sum := bfdLinkLocalVerifySum(source, target, udp); sum != 0xffff {
		t.Errorf("checksum does not verify: the receiver's sum is %#x, want 0xffff", sum)
	}

	control, _, err := packet.ParseControl(udp[8:])
	if err != nil {
		t.Fatalf("BFD Control does not parse: %v", err)
	}
	if control.MyDiscriminator != link.discriminator {
		t.Errorf("My Discriminator %#x, want this link's %#x", control.MyDiscriminator, link.discriminator)
	}
	if control.YourDiscriminator != 0 {
		t.Errorf("Your Discriminator %#x, want 0: the packet must select its session by address and link", control.YourDiscriminator)
	}
	if control.State != packet.StateDown {
		t.Errorf("state %v, want Down", control.State)
	}
}

// TestBFDLinkLocalFrameRefusesAShortBuffer checks the one input the builder
// refuses, so a buffer sized wrong fails the fixture with a reason rather than
// a slice panic.
func TestBFDLinkLocalFrameRefusesAShortBuffer(t *testing.T) {
	link := bfdLinkLocalLinks()[0]
	if _, err := bfdLinkLocalFrame(make([]byte, bfdLinkLocalFrameOctets-1), &link); err == nil {
		t.Fatal("a buffer one octet short was accepted")
	}
}

// bfdLinkLocalVerifySum is the receiver's check, written independently of the
// sender's helper so a shared mistake cannot pass: the one's-complement sum of
// the pseudo-header and the whole segment, checksum included, folded to 16 bits.
func bfdLinkLocalVerifySum(source, target netip.Addr, segment []byte) uint16 {
	pseudo := make([]byte, 0, 40+len(segment)+1)
	sourceOctets, targetOctets := source.As16(), target.As16()
	pseudo = append(pseudo, sourceOctets[:]...)
	pseudo = append(pseudo, targetOctets[:]...)
	pseudo = binary.BigEndian.AppendUint32(pseudo, uint32(len(segment)))
	pseudo = append(pseudo, 0, 0, 0, 17)
	pseudo = append(pseudo, segment...)
	if len(pseudo)%2 == 1 {
		pseudo = append(pseudo, 0)
	}
	var sum uint32
	for i := 0; i < len(pseudo); i += 2 {
		sum += uint32(pseudo[i])<<8 | uint32(pseudo[i+1])
	}
	for sum > 0xffff {
		sum = sum>>16 + sum&0xffff
	}
	return uint16(sum)
}

// bfdLinkLocalZeFrame builds the frame Ze is expected to send on a link: from
// fe80::a to fe80::b, hop limit 255, carrying yours as Your Discriminator. It
// reuses the injector's builder and then swaps what differs, so each test below
// changes exactly one field from a frame that is otherwise right.
func bfdLinkLocalZeFrame(t *testing.T, yours uint32) []byte {
	t.Helper()
	link := bfdLinkLocalLinks()[0]
	frame := make([]byte, bfdLinkLocalFrameOctets)
	if _, err := bfdLinkLocalFrame(frame, &link); err != nil {
		t.Fatalf("bfdLinkLocalFrame: %v", err)
	}
	ip := frame[14:]
	local, peer := netip.MustParseAddr("fe80::a").As16(), netip.MustParseAddr("fe80::b").As16()
	copy(ip[8:24], local[:])
	copy(ip[24:40], peer[:])
	binary.BigEndian.PutUint32(ip[40+8+8:40+8+12], yours)
	return frame
}

// TestBFDLinkLocalCaptureReadsZesFrames checks the reading of a captured frame
// in both directions: a frame Ze sends correctly yields its Your Discriminator,
// a frame that is not BFD is skipped, and a BFD frame that breaks AC-3 (wrong
// source, wrong destination, wrong hop limit) fails rather than being skipped,
// because a skipped frame would let the capture pass on silence.
func TestBFDLinkLocalCaptureReadsZesFrames(t *testing.T) {
	t.Run("a frame Ze sends correctly yields its Your Discriminator", func(t *testing.T) {
		yours, err := bfdLinkLocalSentYourDiscriminator(bfdLinkLocalZeFrame(t, 0x6c6c0001))
		if err != nil {
			t.Fatalf("refused: %v", err)
		}
		if yours != 0x6c6c0001 {
			t.Errorf("Your Discriminator %#x, want 0x6c6c0001", yours)
		}
	})
	t.Run("a frame that is not IPv6 is skipped", func(t *testing.T) {
		frame := bfdLinkLocalZeFrame(t, 1)
		binary.BigEndian.PutUint16(frame[12:14], 0x0800)
		if _, err := bfdLinkLocalSentYourDiscriminator(frame); !errors.Is(err, errBfdLinkLocalNotControl) {
			t.Errorf("err %v, want errBfdLinkLocalNotControl", err)
		}
	})
	t.Run("an ICMPv6 frame is skipped", func(t *testing.T) {
		frame := bfdLinkLocalZeFrame(t, 1)
		frame[14+6] = 58
		if _, err := bfdLinkLocalSentYourDiscriminator(frame); !errors.Is(err, errBfdLinkLocalNotControl) {
			t.Errorf("err %v, want errBfdLinkLocalNotControl", err)
		}
	})
	t.Run("a UDP frame to another port is skipped", func(t *testing.T) {
		frame := bfdLinkLocalZeFrame(t, 1)
		binary.BigEndian.PutUint16(frame[14+40+2:14+40+4], 4784)
		if _, err := bfdLinkLocalSentYourDiscriminator(frame); !errors.Is(err, errBfdLinkLocalNotControl) {
			t.Errorf("err %v, want errBfdLinkLocalNotControl", err)
		}
	})
	t.Run("a truncated frame is skipped", func(t *testing.T) {
		if _, err := bfdLinkLocalSentYourDiscriminator(make([]byte, 20)); !errors.Is(err, errBfdLinkLocalNotControl) {
			t.Errorf("err %v, want errBfdLinkLocalNotControl", err)
		}
	})
	t.Run("a wrong source fails", func(t *testing.T) {
		frame := bfdLinkLocalZeFrame(t, 1)
		other := netip.MustParseAddr("fe80::c").As16()
		copy(frame[14+8:14+24], other[:])
		bfdLinkLocalExpectRefusal(t, frame)
	})
	t.Run("a wrong destination fails", func(t *testing.T) {
		frame := bfdLinkLocalZeFrame(t, 1)
		other := netip.MustParseAddr("fe80::c").As16()
		copy(frame[14+24:14+40], other[:])
		bfdLinkLocalExpectRefusal(t, frame)
	})
	t.Run("a hop limit below 255 fails", func(t *testing.T) {
		frame := bfdLinkLocalZeFrame(t, 1)
		frame[14+7] = 64
		bfdLinkLocalExpectRefusal(t, frame)
	})
	t.Run("a payload that is not BFD Control fails", func(t *testing.T) {
		frame := bfdLinkLocalZeFrame(t, 1)
		frame[14+40+8] = 0
		bfdLinkLocalExpectRefusal(t, frame)
	})
}

func bfdLinkLocalExpectRefusal(t *testing.T, frame []byte) {
	t.Helper()
	_, err := bfdLinkLocalSentYourDiscriminator(frame)
	if err == nil {
		t.Fatal("accepted")
	}
	if errors.Is(err, errBfdLinkLocalNotControl) {
		t.Fatalf("skipped as not BFD (%v), want a failure", err)
	}
}

// TestBFDLinkLocalSessionsAreToldApartByInterface checks the reading of `show
// bfd sessions`: each link's session is found by interface among two sessions
// to one peer, a missing session is "not yet", and a duplicate or a field that
// is not a number fails.
func TestBFDLinkLocalSessionsAreToldApartByInterface(t *testing.T) {
	sessions := []map[string]any{
		{"peer": "fe80::b", "interface": "zbl0", "remote-discriminator": float64(0x6c6c0001)},
		{"peer": "fe80::b", "interface": "zbl1", "remote-discriminator": float64(0x6c6c0002)},
		{"peer": "fe80::c", "interface": "zbl0", "remote-discriminator": float64(7)},
	}
	for _, want := range []struct {
		iface string
		discr uint32
	}{{"zbl0", 0x6c6c0001}, {"zbl1", 0x6c6c0002}} {
		got, found, err := bfdLinkLocalRemoteDiscriminator(sessions, want.iface)
		if err != nil {
			t.Fatalf("%s: %v", want.iface, err)
		}
		if !found {
			t.Fatalf("%s: not found", want.iface)
		}
		if got != want.discr {
			t.Errorf("%s: remote-discriminator %#x, want %#x", want.iface, got, want.discr)
		}
	}

	if _, found, err := bfdLinkLocalRemoteDiscriminator(sessions, "zbl2"); err != nil || found {
		t.Errorf("zbl2: found=%v err=%v, want not found and no error", found, err)
	}

	duplicated := make([]map[string]any, 0, len(sessions)+1)
	duplicated = append(duplicated, sessions...)
	duplicated = append(duplicated, map[string]any{"peer": "fe80::b", "interface": "zbl0", "remote-discriminator": float64(1)})
	if _, _, err := bfdLinkLocalRemoteDiscriminator(duplicated, "zbl0"); err == nil {
		t.Error("two sessions on zbl0 were accepted")
	}

	malformed := []map[string]any{{"peer": "fe80::b", "interface": "zbl0", "remote-discriminator": "x"}}
	if _, _, err := bfdLinkLocalRemoteDiscriminator(malformed, "zbl0"); err == nil {
		t.Error("a remote-discriminator that is not a number was accepted")
	}
}

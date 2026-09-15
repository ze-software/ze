//go:build linux

// Design: docs/architecture/diagnostics/active-probes.md -- the error queue behind a DF probe
//
// These tests drive parseQueuedError over control-message bytes built the
// way the kernel writes them (ip_recv_error), so they assert the parse of
// the bytes a real read hands over and run with no privilege. The kernel's
// own delivery is proven by errqueue_integration_linux_test.go.

package probe

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

// extendedErr is the sock_extended_err a test wants the kernel to have
// written, plus the offender it names.
type extendedErr struct {
	errno    uint32
	origin   uint8
	icmpType uint8
	icmpCode uint8
	info     uint32
	offender netip.Addr
}

// recvErrOOB builds the IP_RECVERR or IPV6_RECVERR control message for ee,
// byte for byte as ip_recv_error and ipv6_recv_error write it: the 16-byte
// sock_extended_err, then a sockaddr_in or sockaddr_in6 for the offender,
// or an AF_UNSPEC sockaddr_in for a local error.
func recvErrOOB(t *testing.T, family Family, ee extendedErr) []byte {
	t.Helper()
	var payload []byte
	payload = binary.NativeEndian.AppendUint32(payload, ee.errno)
	payload = append(payload, ee.origin, ee.icmpType, ee.icmpCode, 0)
	payload = binary.NativeEndian.AppendUint32(payload, ee.info)
	payload = binary.NativeEndian.AppendUint32(payload, 0)
	switch {
	case ee.offender.Is4():
		sa := make([]byte, unix.SizeofSockaddrInet4)
		binary.NativeEndian.PutUint16(sa[0:2], unix.AF_INET)
		copy(sa[4:8], ee.offender.AsSlice())
		payload = append(payload, sa...)
	case ee.offender.Is6():
		sa := make([]byte, unix.SizeofSockaddrInet6)
		binary.NativeEndian.PutUint16(sa[0:2], unix.AF_INET6)
		copy(sa[8:24], ee.offender.AsSlice())
		payload = append(payload, sa...)
	default:
		sa := make([]byte, unix.SizeofSockaddrInet4)
		binary.NativeEndian.PutUint16(sa[0:2], unix.AF_UNSPEC)
		payload = append(payload, sa...)
	}
	level, kind := recvErrCmsg(family)
	oob := make([]byte, unix.CmsgSpace(len(payload)))
	hdr := (*unix.Cmsghdr)(unsafe.Pointer(&oob[0]))
	hdr.Level = level
	hdr.Type = kind
	hdr.SetLen(unix.CmsgLen(len(payload)))
	copy(oob[unix.CmsgLen(0):], payload)
	return oob
}

// quotedEchoRequest is the front of the datagram the kernel quotes: the ICMP
// echo header Ze built, as it sits past the quoted IP header.
func quotedEchoRequest(family Family, id, seq uint16) []byte {
	echoType := byte(8)
	if family == FamilyIPv6 {
		echoType = 128
	}
	return BuildICMPEcho(echoType, id, seq, []byte("ze-ping"))
}

// sizeRefusal is a router's Fragmentation Needed or Packet Too Big as the
// kernel queues it, reporting mtu.
func sizeRefusal(family Family, mtu uint32) extendedErr {
	if family == FamilyIPv6 {
		return extendedErr{errno: uint32(unix.EMSGSIZE), origin: unix.SO_EE_ORIGIN_ICMP6, icmpType: 2, info: mtu, offender: netip.MustParseAddr("2001:db8::1")}
	}
	return extendedErr{errno: uint32(unix.EMSGSIZE), origin: unix.SO_EE_ORIGIN_ICMP, icmpType: 3, icmpCode: 4, info: mtu, offender: netip.MustParseAddr("192.0.2.1")}
}

func parseRefusal(t *testing.T, family Family, ee extendedErr) QueuedError {
	t.Helper()
	entry, err := parseQueuedError(family, quotedEchoRequest(family, 0x1234, 7), recvErrOOB(t, family, ee))
	if err != nil {
		t.Fatalf("parseQueuedError: %v", err)
	}
	return entry
}

// TestReportedMTUZeroIsNotAValue VALIDATES AC-4 and R-2: a Datagram Too Big
// carrying zero in the Next-Hop MTU field is RFC 1191 Section 3's signal
// from an unmodified router. It is a distinct outcome, and the zero is never
// returned as an MTU, on either family.
func TestReportedMTUZeroIsNotAValue(t *testing.T) {
	for _, family := range []Family{FamilyIPv4, FamilyIPv6} {
		entry := parseRefusal(t, family, sizeRefusal(family, 0))
		if entry.Outcome != ErrQueueMTUUnreported {
			t.Errorf("%v: zero next-hop MTU outcome = %v, want %v", family, entry.Outcome, ErrQueueMTUUnreported)
		}
		if entry.MTU != 0 {
			t.Errorf("%v: zero next-hop MTU returned MTU %d, want none", family, entry.MTU)
		}
		if entry.Errno != unix.EMSGSIZE {
			t.Errorf("%v: errno = %v, want EMSGSIZE so the caller knows it was a size refusal", family, entry.Errno)
		}
	}
}

// TestReportedMTUBelowIPv6MinimumIsDiscarded VALIDATES the RFC 8201 Section
// 4 rule and the 1280/1279 boundary: a Packet Too Big reporting less than the
// IPv6 minimum link MTU is discarded, so no value is reported, and 1280
// itself is the last value reported.
func TestReportedMTUBelowIPv6MinimumIsDiscarded(t *testing.T) {
	below := parseRefusal(t, FamilyIPv6, sizeRefusal(FamilyIPv6, 1279))
	if below.Outcome != ErrQueueMTUUnreported {
		t.Errorf("1279 on IPv6 outcome = %v, want %v (RFC 8201 Section 4 discards it)", below.Outcome, ErrQueueMTUUnreported)
	}
	if below.MTU != 0 {
		t.Errorf("1279 on IPv6 returned MTU %d, want none", below.MTU)
	}
	floor := parseRefusal(t, FamilyIPv6, sizeRefusal(FamilyIPv6, 1280))
	if floor.Outcome != ErrQueueMTUReported {
		t.Errorf("1280 on IPv6 outcome = %v, want %v", floor.Outcome, ErrQueueMTUReported)
	}
	if floor.MTU != 1280 {
		t.Errorf("1280 on IPv6 returned MTU %d, want 1280", floor.MTU)
	}
}

// TestReportedMTUBelowSixtyEightIsNotDiscardedOnIPv4 VALIDATES the RFC 1191
// Section 3 rule and the 68/67 boundary: the rule clamps the estimate and
// discards no message, so 67 is still reported, raised to the RFC 791 floor
// of 68, and 68 is reported as itself. It also proves the IPv6 floor is not
// applied to IPv4: 1279 is a reported value there.
func TestReportedMTUBelowSixtyEightIsNotDiscardedOnIPv4(t *testing.T) {
	cases := []struct {
		info uint32
		want uint32
	}{
		{info: 67, want: 68},
		{info: 68, want: 68},
		{info: 1279, want: 1279},
	}
	for _, c := range cases {
		entry := parseRefusal(t, FamilyIPv4, sizeRefusal(FamilyIPv4, c.info))
		if entry.Outcome != ErrQueueMTUReported {
			t.Errorf("%d on IPv4 outcome = %v, want %v (RFC 1191 clamps, it does not discard)", c.info, entry.Outcome, ErrQueueMTUReported)
		}
		if entry.MTU != c.want {
			t.Errorf("%d on IPv4 returned MTU %d, want %d", c.info, entry.MTU, c.want)
		}
	}
}

// TestQueuedErrorCarriesOffenderAndQuotedEcho proves a refusal from the
// network names the router that answered and the probe it quotes, so a
// caller can match the identifier and sequence against what it sent before
// it believes the value.
func TestQueuedErrorCarriesOffenderAndQuotedEcho(t *testing.T) {
	for _, family := range []Family{FamilyIPv4, FamilyIPv6} {
		ee := sizeRefusal(family, 1400)
		entry := parseRefusal(t, family, ee)
		if entry.Local {
			t.Errorf("%v: a router's refusal was read as local", family)
		}
		if entry.Offender != ee.offender {
			t.Errorf("%v: offender = %v, want %v", family, entry.Offender, ee.offender)
		}
		want := QuotedEcho{Present: true, ID: 0x1234, Seq: 7}
		if entry.Echo != want {
			t.Errorf("%v: quoted echo = %+v, want %+v", family, entry.Echo, want)
		}
	}
}

// TestQueuedLocalRefusalQuotesNothing proves a send the kernel refused
// against its own cache reads as local, carries the cached estimate, names
// no offender and quotes no probe: the caller knows which send failed.
func TestQueuedLocalRefusalQuotesNothing(t *testing.T) {
	ee := extendedErr{errno: uint32(unix.EMSGSIZE), origin: unix.SO_EE_ORIGIN_LOCAL, info: 1400}
	entry, err := parseQueuedError(FamilyIPv4, nil, recvErrOOB(t, FamilyIPv4, ee))
	if err != nil {
		t.Fatalf("parseQueuedError: %v", err)
	}
	if !entry.Local {
		t.Errorf("a local refusal was not read as local")
	}
	if entry.Outcome != ErrQueueMTUReported || entry.MTU != 1400 {
		t.Errorf("local refusal outcome = %v MTU %d, want %v 1400", entry.Outcome, entry.MTU, ErrQueueMTUReported)
	}
	if entry.Offender.IsValid() {
		t.Errorf("local refusal named offender %v, want none", entry.Offender)
	}
	if entry.Echo.Present {
		t.Errorf("local refusal quoted an echo, want none")
	}
}

// TestQueuedErrorThatIsNotASizeRefusalReportsNoMTU proves an ICMP error the
// kernel queues for another reason (host unreachable) never reads as a
// next-hop MTU, whatever ee_info happens to hold.
func TestQueuedErrorThatIsNotASizeRefusalReportsNoMTU(t *testing.T) {
	ee := extendedErr{errno: uint32(unix.EHOSTUNREACH), origin: unix.SO_EE_ORIGIN_ICMP, icmpType: 3, icmpCode: 1, info: 1400, offender: netip.MustParseAddr("192.0.2.1")}
	entry := parseRefusal(t, FamilyIPv4, ee)
	if entry.Outcome != ErrQueueMTUUnreported {
		t.Errorf("host unreachable outcome = %v, want %v", entry.Outcome, ErrQueueMTUUnreported)
	}
	if entry.MTU != 0 {
		t.Errorf("host unreachable returned MTU %d, want none", entry.MTU)
	}
	if entry.Errno != unix.EHOSTUNREACH {
		t.Errorf("errno = %v, want EHOSTUNREACH", entry.Errno)
	}
}

// TestQueuedErrorWithoutRecvErrControlMessageIsRefused proves a read that
// carries no IP_RECVERR control message is an error, never a zero entry.
func TestQueuedErrorWithoutRecvErrControlMessageIsRefused(t *testing.T) {
	_, err := parseQueuedError(FamilyIPv4, quotedEchoRequest(FamilyIPv4, 1, 1), nil)
	if err == nil {
		t.Fatal("parseQueuedError accepted a read with no control message")
	}
	var zero QueuedError
	entry, _ := parseQueuedError(FamilyIPv4, nil, nil)
	if entry != zero {
		t.Errorf("refused read still filled an entry: %+v", entry)
	}
}

// TestDrainErrorQueueRefusesAConnWithoutRawDescriptor proves the drain names
// a conn that cannot reach its error queue rather than reporting it empty.
func TestDrainErrorQueueRefusesAConnWithoutRawDescriptor(t *testing.T) {
	err := drainErrorQueue(nil, FamilyIPv4, func(QueuedError) { t.Error("visited an entry on a nil conn") })
	if !errors.Is(err, errNoErrQueueAccess) {
		t.Errorf("drainErrorQueue(nil) err = %v, want errNoErrQueueAccess", err)
	}
}

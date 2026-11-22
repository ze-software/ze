//go:build integration && linux

// Design: docs/architecture/fib/fib-depth-4-srv6.md -- selected Service SID reaches the wire as IPv6 encapsulation.
// Related: srv6probe_integration_linux_test.go -- throwaway-namespace probe verification for srv6-encap and srv6-forward-ttl.
// Related: mplsmtu_integration_linux_test.go -- shared veth testbed, return routes, packet injection, and checksum helpers.
package fibkernel

import (
	"encoding/binary"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
)

const (
	srv6IPv6HeaderLen          = 40
	srv6SRHMinHeaderLen        = 8
	srv6SRHRoutingType         = 4
	srv6ICMPHeaderLen          = 8
	srv6ICMPTypeDestUnreach    = 3
	srv6ICMPCodeFragNeeded     = 4
	srv6ICMPTypeTimeExceeded   = 11
	srv6ICMPCodeTTLExceeded    = 0
	srv6ICMPv6TypeTimeExceeded = 3
	srv6ICMPv6CodeHopLimit     = 0
	srv6DefaultIPv6HopLimit    = 64
	srv6LinkMTU                = 1500
	srv6SingleSIDHeadroom      = srv6IPv6HeaderLen + srv6SRHMinHeaderLen + 16
	srv6EffectiveIPv4MTU       = srv6LinkMTU - srv6SingleSIDHeadroom
	srv6DFClearTestSize        = 1460
	srv6CaptureBufSize         = 4096
	srv6UDPPort                = 4000

	srv6PrivilegeReason  = "the userns.Enter trial timed out waiting for an SRv6-encapsulated packet on the capture link"
	srv6TestSID          = "2001:db8:1::2"
	srv6TestIPv4Prefix   = "192.0.2.0/24"
	srv6TestIPv4Src      = "192.0.2.10"
	srv6TestIPv4Dst      = "192.0.2.20"
	srv6TestIPv6Prefix   = "2001:db8:20::/64"
	srv6TestIPv6Src      = "2001:db8:10::10"
	srv6TestIPv6Dst      = "2001:db8:20::20"
	srv6MsgOuterHopLimit = "outer IPv6 Hop Limit must be the egress interface hop limit"
)

// srv6ForwardTTLPatched probes whether the running kernel carries Ze's
// forwarded-TTL seg6 patch (CONFIG_IPV6_SEG6_FORWARD_TTL, patch 0014).
// On Ze's runtime kernel (release suffix "-ze"), it requires StatePresent and
// never takes the stock-kernel branch (AC-5). On a stock kernel without
// CONFIG_IPV6_SEG6_LWTUNNEL it skips; otherwise it returns whether the patch
// is present and fails on unknown or denied verdicts since the caller already
// verified init-namespace privilege.
func srv6ForwardTTLPatched(t *testing.T) bool {
	t.Helper()
	release := kernelRelease(t)
	encap := kernelcap.SRv6Encap()
	probe := kernelcap.SRv6ForwardTTL()
	t.Logf("kernel %s: srv6-forward-ttl %v (%v)", release, probe.State, probe.Reason)
	if strings.HasSuffix(release, "-ze") {
		require.Equal(t, kernelcap.StatePresent, probe.State,
			"Ze kernel %s must carry CONFIG_IPV6_SEG6_FORWARD_TTL (%v)", release, probe.Reason)
		return true
	}
	if encap.State == kernelcap.StateAbsent {
		t.Skipf("stock kernel %s lacks CONFIG_IPV6_SEG6_LWTUNNEL (%v)", release, encap.Reason)
	}
	switch probe.State {
	case kernelcap.StatePresent:
		return true
	case kernelcap.StateAbsent:
		return false
	case kernelcap.StateUnknown, kernelcap.StateDenied, kernelcap.StateUnspecified:
		t.Fatalf("kernel %s: srv6-forward-ttl probe answered %v (%v); expected present or absent",
			release, probe.State, probe.Reason)
		return false
	default:
		panic("BUG: unexpected kernelcap.State")
	}
}

// TestFIBServiceSIDEncapsulationOnWire programs an IPv4 Service SID route
// through fib-kernel and captures the encapsulated IPv6+SRH+IPv4 frame on the
// egress link.
//
// VALIDATES: AC-1, AC-5 (RFC 8986 Section 5.1:
// "S05.   Decrement inner IPv6 Hop Limit or IPv4 TTL"). On Ze's patched kernel
// (and any kernel where srv6-forward-ttl is present), the encapsulated IPv4
// packet leaves with inner TTL decremented once (63), outer Hop Limit equal to
// the egress interface hop limit (64), SRH next header IPIP, and inner
// addresses and payload unchanged; on a stock kernel without patch 0014, inner
// TTL stays unchanged (64). Withdrawal removes the route.
// PREVENTS: Service SID encapsulation silently dropping the inner TTL decrement
// on Ze's kernel, selecting MPLS push when transposed label bits are present,
// or leaving withdrawn routes usable.
func TestFIBServiceSIDEncapsulationOnWire(t *testing.T) {
	skipWithoutInitNamespacePrivilege(t, srv6PrivilegeReason)
	patched := srv6ForwardTTLPatched(t)
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		bed, writer := setupSRv6Testbed(t, h)
		sid := netip.MustParseAddr(srv6TestSID)
		sidBytes := sid.As16()
		entry := incomingChange{
			Action:  routeaction.Add,
			Prefix:  netip.MustParsePrefix(srv6TestIPv4Prefix),
			NextHop: mplsNextHop,
			SRv6SID: sid,
			// These NLRI bits may encode a transposed SID function. They must
			// not select an MPLS push in the presence of the Service SID.
			Labels: []uint32{300},
		}
		writer.processEvent(makeSysribPayload([]incomingChange{entry}))
		packet := ipv4UDP(netip.MustParseAddr(srv6TestIPv4Src), netip.MustParseAddr(srv6TestIPv4Dst), 4000, []byte("service-sid"))
		mplsMTUInject(bed, nil, packet, false)
		frame := bed.awaitForwarded("SRv6-encapsulated IPv4 packet", func(frame []byte) bool {
			return isSRv6Frame(frame, sidBytes)
		})
		outer, inner := extractSRv6Inner(t, frame, unix.IPPROTO_IPIP, len(packet))
		if patched {
			require.Equal(t, packet[8]-1, inner[8], "IPv4 forwarding must decrement TTL once")
			require.Equal(t, byte(srv6DefaultIPv6HopLimit), outer[7], srv6MsgOuterHopLimit)
		} else {
			require.Equal(t, packet[8], inner[8], "stock kernel without patch 0014 leaves inner IPv4 TTL unchanged")
		}
		require.True(t, slices.Equal(packet[12:], inner[12:]), "encapsulation changed the inner addresses or payload")

		entry.Action = routeaction.Withdraw
		writer.processEvent(makeSysribPayload([]incomingChange{entry}))
		_, err = h.RouteGet(netip.MustParseAddr(srv6TestIPv4Dst).AsSlice())
		require.Error(t, err, "withdrawal left the service route usable")
	})
}

// TestFIBServiceSIDEncapsulationIPv6InnerOnWire programs an IPv6 Service SID
// route through fib-kernel and captures the encapsulated IPv6+SRH+IPv6 frame
// on the egress link.
//
// VALIDATES: AC-2, AC-5 (RFC 8986 Section 5.1:
// "S05.   Decrement inner IPv6 Hop Limit or IPv4 TTL"). On Ze's patched kernel,
// a forwarded IPv6 packet with Hop Limit 64 leaves encapsulated with SRH next
// header IPv6 (41), inner Hop Limit 63, outer Hop Limit 63, and inner header
// fields and payload unchanged; on a stock kernel without patch 0014, inner
// Hop Limit stays 64. Withdrawal removes the route.
// PREVENTS: IPv6-inner Service SID forwarding bypassing ip6_forward on Ze's
// kernel or corrupting inner IPv6 addresses or payload.
func TestFIBServiceSIDEncapsulationIPv6InnerOnWire(t *testing.T) {
	skipWithoutInitNamespacePrivilege(t, srv6PrivilegeReason)
	patched := srv6ForwardTTLPatched(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		bed, writer := setupSRv6Testbed(t, h)
		sid := netip.MustParseAddr(srv6TestSID)
		sidBytes := sid.As16()
		entry := incomingChange{
			Action:  routeaction.Add,
			Prefix:  netip.MustParsePrefix(srv6TestIPv6Prefix),
			NextHop: sid,
			SRv6SID: sid,
		}
		writer.processEvent(makeIPv6SysribPayload([]incomingChange{entry}))
		packet := ipv6UDP(
			netip.MustParseAddr(srv6TestIPv6Src),
			netip.MustParseAddr(srv6TestIPv6Dst),
			mplsTTL,
			[]byte("service-sid-v6"),
		)
		mplsMTUInject(bed, nil, packet, true)
		frame := bed.awaitForwarded("SRv6-encapsulated IPv6 packet", func(frame []byte) bool {
			return isSRv6Frame(frame, sidBytes)
		})
		outer, inner := extractSRv6Inner(t, frame, unix.IPPROTO_IPV6, len(packet))
		if patched {
			require.Equal(t, packet[7]-1, inner[7], "IPv6 forwarding must decrement inner Hop Limit once")
			require.Equal(t, packet[7]-1, outer[7], "outer IPv6 Hop Limit must follow the decremented inner Hop Limit")
		} else {
			require.Equal(t, packet[7], inner[7], "stock kernel without patch 0014 leaves inner IPv6 Hop Limit unchanged")
		}
		require.True(t, slices.Equal(packet[:7], inner[:7]), "encapsulation changed the inner IPv6 header prefix")
		require.True(t, slices.Equal(packet[8:], inner[8:]), "encapsulation changed the inner IPv6 addresses or payload")

		entry.Action = routeaction.Withdraw
		writer.processEvent(makeIPv6SysribPayload([]incomingChange{entry}))
		_, err = h.RouteGet(netip.MustParseAddr(srv6TestIPv6Dst).AsSlice())
		require.Error(t, err, "withdrawal left the IPv6 service route usable")
	})
}

// TestFIBServiceSIDExpiringTTLAnswersTimeExceeded verifies boundary IPv4 TTL
// handling at an SRv6 headend for TTL 2 (last valid forwarded value) and TTL 1
// (expiring).
//
// VALIDATES: AC-3, AC-5 (RFC 8986 Section 5.1 S05, RFC 1812). On Ze's patched
// kernel, an IPv4 packet with TTL 2 is encapsulated with inner TTL 1, while a
// packet with TTL 1 is not encapsulated and the sender receives ICMP Time
// Exceeded (type 11, code 0) quoting its packet. On a stock kernel without
// patch 0014, the TTL-1 packet is encapsulated with inner TTL 1.
// PREVENTS: forwarding expired IPv4 packets into an SRv6 tunnel or hiding the
// headend hop from traceroute on Ze's kernel.
func TestFIBServiceSIDExpiringTTLAnswersTimeExceeded(t *testing.T) {
	skipWithoutInitNamespacePrivilege(t, srv6PrivilegeReason)
	patched := srv6ForwardTTLPatched(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		bed, writer := setupSRv6Testbed(t, h)
		sid := netip.MustParseAddr(srv6TestSID)
		sidBytes := sid.As16()
		entry := incomingChange{
			Action:  routeaction.Add,
			Prefix:  netip.MustParsePrefix(srv6TestIPv4Prefix),
			NextHop: mplsNextHop,
			SRv6SID: sid,
		}
		writer.processEvent(makeSysribPayload([]incomingChange{entry}))

		src := netip.MustParseAddr(srv6TestIPv4Src)
		dst := netip.MustParseAddr(srv6TestIPv4Dst)
		if patched {
			// Boundary: TTL 2 is the smallest valid forwarded value and must
			// leave encapsulated with inner TTL 1.
			boundary := ipv4UDPWithTTL(src, dst, 4000, 2, []byte("ttl-boundary-2"))
			mplsMTUInject(bed, nil, boundary, false)
			bFrame := bed.awaitForwarded("SRv6-encapsulated TTL-2 IPv4 packet", func(frame []byte) bool {
				return isSRv6Frame(frame, sidBytes)
			})
			bOuter, bInner := extractSRv6Inner(t, bFrame, unix.IPPROTO_IPIP, len(boundary))
			require.Equal(t, byte(1), bInner[8], "TTL-2 packet must be encapsulated with inner TTL 1")
			require.Equal(t, byte(srv6DefaultIPv6HopLimit), bOuter[7], srv6MsgOuterHopLimit)
			require.True(t, slices.Equal(boundary[12:], bInner[12:]), "encapsulation changed TTL-2 inner addresses or payload")

			expiring := ipv4UDPWithTTL(src, dst, 4000, 1, []byte("ttl-expire-1"))
			mplsMTUInject(bed, nil, expiring, false)
			icmpFrame := bed.awaitForwarded("ICMP Time Exceeded", func(frame []byte) bool {
				if isSRv6Frame(frame, sidBytes) {
					t.Fatal("TTL-1 IPv4 packet was encapsulated instead of answered with ICMP Time Exceeded")
				}
				return isICMPv4Frame(frame)
			})
			require.Equal(t, []byte(mplsNextMAC), icmpFrame[:6], "ICMP Time Exceeded return next hop")
			ip := icmpFrame[ethernetHeaderLen:]
			hlen := int(ip[0]&0x0f) * 4
			length := int(binary.BigEndian.Uint16(ip[2:4]))
			require.GreaterOrEqual(t, hlen, ipv4HeaderLen)
			require.LessOrEqual(t, hlen, length)
			require.LessOrEqual(t, length, len(ip))
			require.Equal(t, expiring[12:16], ip[16:20], "ICMP Time Exceeded destination must be original sender")
			require.Equal(t, []byte{10, 0, 0, 1}, ip[12:16], "inbound ICMP source policy")
			require.Zero(t, internetChecksum(ip[:hlen]), "ICMP outer IPv4 checksum")
			icmp := ip[hlen:length]
			require.GreaterOrEqual(t, len(icmp), srv6ICMPHeaderLen+ipv4HeaderLen+udpHeaderLen)
			require.Equal(t, byte(srv6ICMPTypeTimeExceeded), icmp[0], "ICMP type must be Time Exceeded (11)")
			require.Equal(t, byte(srv6ICMPCodeTTLExceeded), icmp[1], "ICMP code must be TTL exceeded in transit (0)")
			require.Zero(t, internetChecksum(icmp), "ICMP checksum")
			quote := icmp[srv6ICMPHeaderLen:]
			require.Equal(t, expiring[:ipv4HeaderLen+udpHeaderLen], quote[:ipv4HeaderLen+udpHeaderLen],
				"ICMP Time Exceeded must quote original IPv4 and UDP headers")
			requireNoSRv6Frame(t, bed, sidBytes, "TTL-1 IPv4 packet must not be encapsulated")
		} else {
			expiring := ipv4UDPWithTTL(src, dst, 4000, 1, []byte("ttl-expire-1"))
			mplsMTUInject(bed, nil, expiring, false)
			frame := bed.awaitForwarded("stock SRv6-encapsulated TTL-1 IPv4 packet", func(frame []byte) bool {
				return isSRv6Frame(frame, sidBytes)
			})
			_, inner := extractSRv6Inner(t, frame, unix.IPPROTO_IPIP, len(expiring))
			require.Equal(t, byte(1), inner[8], "stock kernel without patch 0014 encapsulates TTL-1 packet with inner TTL 1")
			require.True(t, slices.Equal(expiring[12:], inner[12:]), "encapsulation changed inner addresses or payload")
		}
	})
}

// TestFIBServiceSIDExpiringHopLimitAnswersTimeExceeded verifies boundary IPv6
// Hop Limit handling at an SRv6 headend for Hop Limit 2 (last valid forwarded
// value) and Hop Limit 1 (expiring).
//
// VALIDATES: AC-4, AC-5 (RFC 8986 Section 5.1 S05, RFC 8200). On Ze's patched
// kernel, an IPv6 packet with Hop Limit 2 is encapsulated with inner Hop Limit
// 1, while a packet with Hop Limit 1 is not encapsulated and the sender
// receives ICMPv6 Time Exceeded (type 3, code 0) quoting its packet. On a
// stock kernel without patch 0014, Hop Limit 2 is encapsulated with inner Hop
// Limit 2 unchanged, and Hop Limit 1 is copied to the outer Hop Limit so
// ip6_forward drops the outer packet and never signals the real sender.
// PREVENTS: forwarding expired IPv6 packets into an SRv6 tunnel or failing to
// return ICMPv6 Time Exceeded to the real sender on Ze's kernel.
func TestFIBServiceSIDExpiringHopLimitAnswersTimeExceeded(t *testing.T) {
	skipWithoutInitNamespacePrivilege(t, srv6PrivilegeReason)
	patched := srv6ForwardTTLPatched(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		bed, writer := setupSRv6Testbed(t, h)
		sid := netip.MustParseAddr(srv6TestSID)
		sidBytes := sid.As16()
		entry := incomingChange{
			Action:  routeaction.Add,
			Prefix:  netip.MustParsePrefix(srv6TestIPv6Prefix),
			NextHop: sid,
			SRv6SID: sid,
		}
		writer.processEvent(makeIPv6SysribPayload([]incomingChange{entry}))

		src := netip.MustParseAddr(srv6TestIPv6Src)
		srcBytes := src.As16()
		dst := netip.MustParseAddr(srv6TestIPv6Dst)
		if patched {
			// Boundary: Hop Limit 2 is the smallest valid forwarded value and
			// must leave encapsulated with inner Hop Limit 1.
			boundary := ipv6UDP(src, dst, 2, []byte("hlim-boundary-2"))
			mplsMTUInject(bed, nil, boundary, true)
			bFrame := bed.awaitForwarded("SRv6-encapsulated Hop-Limit-2 IPv6 packet", func(frame []byte) bool {
				return isSRv6Frame(frame, sidBytes)
			})
			bOuter, bInner := extractSRv6Inner(t, bFrame, unix.IPPROTO_IPV6, len(boundary))
			require.Equal(t, byte(1), bInner[7], "Hop-Limit-2 packet must be encapsulated with inner Hop Limit 1")
			require.Equal(t, byte(1), bOuter[7], "outer IPv6 Hop Limit must follow the decremented inner Hop Limit")
			require.True(t, slices.Equal(boundary[8:], bInner[8:]), "encapsulation changed Hop-Limit-2 inner addresses or payload")

			expiring := ipv6UDP(src, dst, 1, []byte("hlim-expire-1"))
			mplsMTUInject(bed, nil, expiring, true)
			icmpFrame := bed.awaitForwarded("ICMPv6 Time Exceeded", func(frame []byte) bool {
				if isSRv6Frame(frame, sidBytes) {
					t.Fatal("Hop-Limit-1 IPv6 packet was encapsulated instead of answered with ICMPv6 Time Exceeded")
				}
				return isICMPv6To(frame, srcBytes)
			})
			require.Equal(t, []byte(mplsNextMAC), icmpFrame[:6], "ICMPv6 Time Exceeded return next hop")
			ip6 := icmpFrame[ethernetHeaderLen:]
			payloadLen := int(binary.BigEndian.Uint16(ip6[4:6]))
			require.GreaterOrEqual(t, len(ip6), srv6IPv6HeaderLen+payloadLen)
			icmp := ip6[srv6IPv6HeaderLen : srv6IPv6HeaderLen+payloadLen]
			require.GreaterOrEqual(t, len(icmp), srv6ICMPHeaderLen+srv6IPv6HeaderLen+udpHeaderLen)
			require.Equal(t, byte(srv6ICMPv6TypeTimeExceeded), icmp[0], "ICMPv6 type must be Time Exceeded (3)")
			require.Equal(t, byte(srv6ICMPv6CodeHopLimit), icmp[1], "ICMPv6 code must be hop limit exceeded in transit (0)")
			require.Zero(t, ipv6TransportChecksum(ip6[8:24], ip6[24:40], unix.IPPROTO_ICMPV6, icmp), "ICMPv6 checksum")
			quote := icmp[srv6ICMPHeaderLen:]
			require.Equal(t, expiring[:srv6IPv6HeaderLen+udpHeaderLen], quote[:srv6IPv6HeaderLen+udpHeaderLen],
				"ICMPv6 Time Exceeded must quote original IPv6 and UDP headers")
			requireNoSRv6Frame(t, bed, sidBytes, "Hop-Limit-1 IPv6 packet must not be encapsulated")
		} else {
			// On a stock kernel without patch 0014, __seg6_do_srh_encap copies
			// inner hop_limit onto the outer header before seg6_input_route
			// hands the outer packet to ip6_forward. A Hop-Limit-2 packet proves
			// stock encapsulation (inner stays 2, outer drops to 1); a
			// Hop-Limit-1 packet causes ip6_forward to drop the outer packet and
			// signal the headend's own address instead of the real sender.
			boundary := ipv6UDP(src, dst, 2, []byte("hlim-boundary-2"))
			mplsMTUInject(bed, nil, boundary, true)
			bFrame := bed.awaitForwarded("stock SRv6-encapsulated Hop-Limit-2 IPv6 packet", func(frame []byte) bool {
				return isSRv6Frame(frame, sidBytes)
			})
			bOuter, bInner := extractSRv6Inner(t, bFrame, unix.IPPROTO_IPV6, len(boundary))
			require.Equal(t, byte(2), bInner[7], "stock kernel without patch 0014 leaves inner IPv6 Hop Limit unchanged")
			require.Equal(t, byte(1), bOuter[7], "stock kernel ip6_forward decrements outer Hop Limit from 2 to 1")

			expiring := ipv6UDP(src, dst, 1, []byte("hlim-expire-1"))
			mplsMTUInject(bed, nil, expiring, true)
			buf := make([]byte, srv6CaptureBufSize)
			deadline := time.Now().Add(mplsSilenceWait)
			for time.Now().Before(deadline) {
				frame := bed.readForwarded(buf)
				if frame == nil {
					continue
				}
				if isSRv6Frame(frame, sidBytes) {
					t.Fatal("stock kernel forwarded Hop-Limit-1 outer IPv6 packet instead of expiring on outer ip6_forward")
				}
				if isICMPv6To(frame, srcBytes) {
					t.Fatal("stock kernel returned ICMPv6 Time Exceeded to real sender without patch 0014")
				}
			}
		}
	})
}

// TestFIBServiceSIDOversizedDFAnswersFragmentationNeeded verifies PMTU
// enforcement on the patched SRv6 headend forward path for exact-fit DF-set
// packets (1500 - 64 = 1436 bytes) and oversized DF-set packets (1437 bytes),
// and records empirical kernel behavior for oversized DF-clear IPv4 (A-8).
//
// VALIDATES: AC-6, A-3, A-8. On Ze's patched kernel over a 1500-byte egress
// link (single-SID seg6 headroom 64 bytes = 40 outer IPv6 + 24 SRH), a 1436-byte
// DF-set IPv4 packet is encapsulated with inner TTL decremented, while a
// 1437-byte DF-set IPv4 packet is not encapsulated and the sender receives ICMP
// Fragmentation Needed (type 3, code 4) with next-hop MTU 1436. An oversized
// DF-clear packet (1460 bytes) is observed and logged for Assumption A-8.
// PREVENTS: ip_forward failing to subtract seg6 lwtunnel headroom from the path
// MTU or silently blackholing oversized DF-set traffic behind the headend.
func TestFIBServiceSIDOversizedDFAnswersFragmentationNeeded(t *testing.T) {
	skipWithoutInitNamespacePrivilege(t, srv6PrivilegeReason)
	patched := srv6ForwardTTLPatched(t)
	if !patched {
		t.Skipf("stock kernel without patch 0014; PMTU enforcement requires CONFIG_IPV6_SEG6_FORWARD_TTL")
	}
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		bed, writer := setupSRv6Testbed(t, h)
		link, err := h.LinkByName(mplsZeLink)
		require.NoError(t, err)
		require.NoError(t, h.LinkSetMTU(link, srv6LinkMTU))

		sid := netip.MustParseAddr(srv6TestSID)
		sidBytes := sid.As16()
		entry := incomingChange{
			Action:  routeaction.Add,
			Prefix:  netip.MustParsePrefix(srv6TestIPv4Prefix),
			NextHop: mplsNextHop,
			SRv6SID: sid,
		}
		writer.processEvent(makeSysribPayload([]incomingChange{entry}))

		// Boundary: exact fit (1500 - 64 = 1436 bytes, DF set) must be
		// encapsulated and forwarded with inner TTL decremented.
		exact := mplsMTUDatagram(srv6EffectiveIPv4MTU, false, true)
		mplsMTUInject(bed, nil, exact, false)
		exactFrame := bed.awaitForwarded("exact-fit SRv6-encapsulated IPv4 packet", func(frame []byte) bool {
			return isSRv6Frame(frame, sidBytes)
		})
		exactOuter, exactInner := extractSRv6Inner(t, exactFrame, unix.IPPROTO_IPIP, len(exact))
		require.Equal(t, srv6LinkMTU, len(exactOuter), "exact-fit encapsulated packet must equal the 1500-byte link MTU")
		require.Equal(t, exact[8]-1, exactInner[8], "exact-fit IPv4 forwarding must decrement TTL once")
		require.Equal(t, byte(srv6DefaultIPv6HopLimit), exactOuter[7], srv6MsgOuterHopLimit)
		require.True(t, slices.Equal(exact[12:], exactInner[12:]), "encapsulation changed exact-fit inner addresses or payload")

		// Oversized DF-set (1437 bytes): fits the 1500-byte link before
		// encapsulation, but exceeds 1500 - 64 (seg6 headroom).
		oversize := mplsMTUDatagram(srv6EffectiveIPv4MTU+1, false, true)
		mplsMTUInject(bed, nil, oversize, false)
		icmpFrame := bed.awaitForwarded("ICMP Fragmentation Needed", func(frame []byte) bool {
			if isSRv6Frame(frame, sidBytes) {
				t.Fatal("oversized DF-set packet was encapsulated instead of answered with ICMP Fragmentation Needed")
			}
			return isICMPv4Frame(frame)
		})
		require.Equal(t, []byte(mplsNextMAC), icmpFrame[:6], "ICMP Fragmentation Needed return next hop")
		ip := icmpFrame[ethernetHeaderLen:]
		hlen := int(ip[0]&0x0f) * 4
		length := int(binary.BigEndian.Uint16(ip[2:4]))
		require.GreaterOrEqual(t, hlen, ipv4HeaderLen)
		require.LessOrEqual(t, hlen, length)
		require.LessOrEqual(t, length, len(ip))
		require.Equal(t, oversize[12:16], ip[16:20], "ICMP Fragmentation Needed destination must be original sender")
		require.Equal(t, []byte{10, 0, 0, 1}, ip[12:16], "inbound ICMP source policy")
		require.Zero(t, internetChecksum(ip[:hlen]), "ICMP outer IPv4 checksum")
		icmp := ip[hlen:length]
		require.GreaterOrEqual(t, len(icmp), srv6ICMPHeaderLen+ipv4HeaderLen+udpHeaderLen)
		require.Equal(t, byte(srv6ICMPTypeDestUnreach), icmp[0], "ICMP type must be Destination Unreachable (3)")
		require.Equal(t, byte(srv6ICMPCodeFragNeeded), icmp[1], "ICMP code must be Fragmentation Needed (4)")
		require.Equal(t, uint16(srv6EffectiveIPv4MTU), binary.BigEndian.Uint16(icmp[6:8]),
			"ICMP next-hop MTU must be link MTU minus seg6 headroom")
		require.Zero(t, internetChecksum(icmp), "ICMP checksum")
		quote := icmp[srv6ICMPHeaderLen:]
		require.Equal(t, oversize[:8], quote[:8], "ICMP Fragmentation Needed must quote original length, ID, and DF")
		require.Equal(t, oversize[12:20], quote[12:20], "ICMP Fragmentation Needed must quote original IPv4 endpoints")
		require.Equal(t, oversize[ipv4HeaderLen:ipv4HeaderLen+udpHeaderLen],
			quote[ipv4HeaderLen:ipv4HeaderLen+udpHeaderLen], "ICMP Fragmentation Needed must quote original UDP header")
		requireNoSRv6Frame(t, bed, sidBytes, "oversized DF-set packet must not be encapsulated")

		// Oversized DF-clear (1460 bytes): measure and log kernel behavior for
		// Assumption A-8 (dropped vs encapsulated vs outer-IPv6-fragmented).
		dfClear := mplsMTUDatagram(srv6DFClearTestSize, false, false)
		mplsMTUInject(bed, nil, dfClear, false)
		unfragSRv6, fragOuterV6, icmpErrors, lengths := observeSRv6DFClearOutcome(bed, sidBytes)
		t.Logf("A-8 DF-clear oversized IPv4 (%d bytes on %d-byte link, headroom %d): "+
			"unfragmented-srv6=%d ipv6-outer-fragments=%d icmp-errors=%d frame-lengths=%v",
			len(dfClear), srv6LinkMTU, srv6SingleSIDHeadroom, unfragSRv6, fragOuterV6, icmpErrors, lengths)
	})
}

func setupSRv6Testbed(t *testing.T, h *netlink.Handle) (*mplsTestbed, *fibKernel) {
	t.Helper()
	bed := newMPLSTestbed(t, h)
	link, err := h.LinkByName(mplsZeLink)
	require.NoError(t, err)
	mplsMTUReturnRoutes(t, h, link)
	return bed, newFIBKernel(newTestBackend(h))
}

func makeIPv6SysribPayload(changes []incomingChange) *incomingBatch {
	return &incomingBatch{
		Family:  family.IPv6Unicast,
		Changes: changes,
	}
}

func ipv4UDPWithTTL(src, dst netip.Addr, port uint16, ttl byte, payload []byte) []byte {
	packet := ipv4UDP(src, dst, port, payload)
	packet[8] = ttl
	mplsMTUChecksum(packet)
	return packet
}

func ipv6UDP(src, dst netip.Addr, hopLimit byte, payload []byte) []byte {
	udpLen := udpHeaderLen + len(payload)
	packet := make([]byte, srv6IPv6HeaderLen+udpLen)
	packet[0] = 0x60
	binary.BigEndian.PutUint16(packet[4:6], uint16(udpLen))
	packet[6] = unix.IPPROTO_UDP
	packet[7] = hopLimit
	copy(packet[8:24], src.AsSlice())
	copy(packet[24:40], dst.AsSlice())

	udp := packet[srv6IPv6HeaderLen:]
	binary.BigEndian.PutUint16(udp[0:2], srv6UDPPort)
	binary.BigEndian.PutUint16(udp[2:4], srv6UDPPort)
	binary.BigEndian.PutUint16(udp[4:6], uint16(udpLen))
	copy(udp[udpHeaderLen:], payload)

	checksum := ipv6TransportChecksum(packet[8:24], packet[24:40], unix.IPPROTO_UDP, udp)
	if checksum == 0 {
		checksum = 0xffff
	}
	binary.BigEndian.PutUint16(udp[6:8], checksum)
	return packet
}

func ipv6TransportChecksum(src, dst []byte, nextHeader byte, payload []byte) uint16 {
	pseudo := make([]byte, srv6IPv6HeaderLen+len(payload))
	copy(pseudo[0:16], src)
	copy(pseudo[16:32], dst)
	binary.BigEndian.PutUint32(pseudo[32:36], uint32(len(payload)))
	pseudo[39] = nextHeader
	copy(pseudo[srv6IPv6HeaderLen:], payload)
	return internetChecksum(pseudo)
}

func isSRv6Frame(frame []byte, sid [16]byte) bool {
	if len(frame) < ethernetHeaderLen+srv6IPv6HeaderLen {
		return false
	}
	if binary.BigEndian.Uint16(frame[12:14]) != unix.ETH_P_IPV6 {
		return false
	}
	return slices.Equal(frame[ethernetHeaderLen+24:ethernetHeaderLen+srv6IPv6HeaderLen], sid[:])
}

func isICMPv4Frame(frame []byte) bool {
	if len(frame) < ethernetHeaderLen+ipv4HeaderLen+srv6ICMPHeaderLen {
		return false
	}
	if !isIPv4(frame) {
		return false
	}
	return frame[ethernetHeaderLen+9] == unix.IPPROTO_ICMP
}

func isICMPv6To(frame []byte, dst [16]byte) bool {
	if len(frame) < ethernetHeaderLen+srv6IPv6HeaderLen+srv6ICMPHeaderLen {
		return false
	}
	if binary.BigEndian.Uint16(frame[12:14]) != unix.ETH_P_IPV6 {
		return false
	}
	if frame[ethernetHeaderLen+6] != unix.IPPROTO_ICMPV6 {
		return false
	}
	return slices.Equal(frame[ethernetHeaderLen+24:ethernetHeaderLen+srv6IPv6HeaderLen], dst[:])
}

func extractSRv6Inner(t *testing.T, frame []byte, expectedProto byte, innerLen int) (outer, inner []byte) {
	t.Helper()
	outer = frame[ethernetHeaderLen:]
	require.Equal(t, byte(unix.IPPROTO_ROUTING), outer[6])
	require.GreaterOrEqual(t, len(outer), srv6IPv6HeaderLen+srv6SRHMinHeaderLen)
	srh := outer[srv6IPv6HeaderLen:]
	require.Equal(t, byte(srv6SRHRoutingType), srh[2], "missing Segment Routing Header")
	require.Equal(t, expectedProto, srh[0])
	headerLength := (int(srh[1]) + 1) * 8
	require.GreaterOrEqual(t, len(srh), headerLength+innerLen)
	return outer, srh[headerLength : headerLength+innerLen]
}

func requireNoSRv6Frame(t *testing.T, bed *mplsTestbed, sid [16]byte, why string) {
	t.Helper()
	buf := make([]byte, srv6CaptureBufSize)
	deadline := time.Now().Add(mplsSilenceWait)
	for time.Now().Before(deadline) {
		frame := bed.readForwarded(buf)
		if frame == nil {
			continue
		}
		if isSRv6Frame(frame, sid) {
			t.Fatal(why)
		}
	}
}

func observeSRv6DFClearOutcome(bed *mplsTestbed, sid [16]byte) (unfragSRv6, fragOuterV6, icmpErrors int, lengths []int) {
	buf := make([]byte, srv6CaptureBufSize)
	deadline := time.Now().Add(mplsSilenceWait)
	for time.Now().Before(deadline) {
		frame := bed.readForwarded(buf)
		if frame == nil {
			continue
		}
		if isSRv6Frame(frame, sid) {
			switch frame[ethernetHeaderLen+6] {
			case unix.IPPROTO_ROUTING:
				unfragSRv6++
				lengths = append(lengths, len(frame)-ethernetHeaderLen)
			case unix.IPPROTO_FRAGMENT:
				fragOuterV6++
				lengths = append(lengths, len(frame)-ethernetHeaderLen)
			}
			continue
		}
		if isICMPv4Frame(frame) {
			icmpErrors++
			lengths = append(lengths, len(frame)-ethernetHeaderLen)
		}
	}
	return unfragSRv6, fragOuterV6, icmpErrors, lengths
}

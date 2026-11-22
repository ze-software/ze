//go:build integration && linux

// Design: docs/architecture/fib/fib-depth-4-srv6.md -- selected Service SID reaches the wire as IPv6 encapsulation.
// Related: srv6probe_integration_linux_test.go -- throwaway-namespace probe verification for srv6-encap, srv6-forward-ttl, and srv6-forward-frag.
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
	srv6IPv4FlagDF             = 0x4000
	srv6IPv4FlagMF             = 0x2000
	srv6IPv4FragOffMask        = 0x1fff
	srv6FragUnit               = 8
	srv6SubQuantumPathMTU      = 84
	srv6SubQuantumPacketLen    = 68
	srv6MarkerPacketLen        = 44

	srv6PrivilegeReason      = "the userns.Enter trial timed out waiting for an SRv6-encapsulated packet on the capture link"
	srv6TestSID              = "2001:db8:1::2"
	srv6TestIPv4Prefix       = "192.0.2.0/24"
	srv6TestIPv4Src          = "192.0.2.10"
	srv6TestIPv4Dst          = "192.0.2.20"
	srv6SubQuantumIPv4Prefix = "192.0.2.20/32"
	srv6MarkerIPv4Prefix     = "192.0.2.21/32"
	srv6MarkerIPv4Dst        = "192.0.2.21"
	srv6TestIPv6Prefix       = "2001:db8:20::/64"
	srv6TestIPv6Src          = "2001:db8:10::10"
	srv6TestIPv6Dst          = "2001:db8:20::20"
	srv6MsgOuterHopLimit     = "outer IPv6 Hop Limit must be the egress interface hop limit"
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

// srv6ForwardFragPatched probes whether the running kernel carries Ze's
// forwarded DF-clear IPv4 fragmentation seg6 patch
// (CONFIG_IPV6_SEG6_FORWARD_FRAG, patch 0015). On Ze's runtime kernel (release
// suffix "-ze"), it requires StatePresent and never takes the stock-kernel
// branch. On a stock kernel without CONFIG_IPV6_SEG6_LWTUNNEL it skips;
// otherwise it returns whether the patch is present and fails on unknown or
// denied verdicts since the caller already verified init-namespace privilege.
func srv6ForwardFragPatched(t *testing.T) bool {
	t.Helper()
	release := kernelRelease(t)
	encap := kernelcap.SRv6Encap()
	probe := kernelcap.SRv6ForwardFrag()
	t.Logf("kernel %s: srv6-forward-frag %v (%v)", release, probe.State, probe.Reason)
	if strings.HasSuffix(release, "-ze") {
		require.Equal(t, kernelcap.StatePresent, probe.State,
			"Ze kernel %s must carry CONFIG_IPV6_SEG6_FORWARD_FRAG (%v)", release, probe.Reason)
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
		t.Fatalf("kernel %s: srv6-forward-frag probe answered %v (%v); expected present or absent",
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

// TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4 verifies pre-encapsulation
// inner IPv4 fragmentation of forwarded DF-clear IPv4 packets crossing a
// single-SID SRv6 headend over a 1500-byte egress link (effective MTU 1436).
//
// VALIDATES: AC-1, A-1, A-4 (RFC 791 Section 3.1, RFC 1812 Section 4.2.2.7,
// RFC 8986 Section 5.1 S05). On Ze's patched kernel (CONFIG_IPV6_SEG6_FORWARD_FRAG),
// a 1436-byte DF-clear IPv4 packet leaves as one unfragmented 1500-byte outer
// IPv6+SRH packet, while 1437-byte and 1460-byte DF-clear packets each leave as
// two unfragmented outer IPv6+SRH packets <= 1500 bytes (1500B+85B and
// 1500B+108B) carrying valid inner IPv4 fragments with inner TTL 63, matching
// IPv4 ID, valid header checksums, zero ICMP errors, zero outer IPv6 Fragment
// Headers, and byte-for-byte payload reassembly. On stock or 0014-only kernels
// without patch 0015, the 1460-byte DF-clear packet is dropped in ip6_fragment
// (LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG).
// PREVENTS: silent blackholing of oversized DF-clear IPv4 traffic at the SRv6
// headend, emitting outer IPv6 Fragment Headers that Linux seg6_local drops, or
// corrupting fragment offsets and payload bytes during reassembly.
func TestFIBServiceSIDOversizedDFClearFragmentsInnerIPv4(t *testing.T) {
	skipWithoutInitNamespacePrivilege(t, srv6PrivilegeReason)
	patched := srv6ForwardFragPatched(t)
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

		if patched {
			// Exact fit (1500 - 64 = 1436 bytes, DF clear) must leave as one
			// unfragmented 1500-byte outer IPv6+SRH packet.
			exact := srv6DFClearDatagram(srv6EffectiveIPv4MTU, 0x2101)
			mplsMTUInject(bed, nil, exact, false)
			exactFrame := bed.awaitForwarded("exact-fit DF-clear SRv6-encapsulated IPv4 packet", func(frame []byte) bool {
				return isSRv6Frame(frame, sidBytes)
			})
			exactOuter, exactInner := extractSRv6Inner(t, exactFrame, unix.IPPROTO_IPIP, len(exact))
			require.Equal(t, srv6LinkMTU, len(exactOuter), "exact-fit DF-clear packet must equal the 1500-byte link MTU")
			require.Equal(t, exact[8]-1, exactInner[8], "exact-fit DF-clear IPv4 forwarding must decrement TTL once")
			require.Equal(t, byte(srv6DefaultIPv6HopLimit), exactOuter[7], srv6MsgOuterHopLimit)
			require.Zero(t, binary.BigEndian.Uint16(exactInner[6:8]), "exact-fit DF-clear packet must stay unfragmented")
			require.Zero(t, internetChecksum(exactInner[:ipv4HeaderLen]), "exact-fit DF-clear inner IPv4 checksum")
			require.True(t, slices.Equal(exact[12:], exactInner[12:]), "encapsulation changed exact-fit DF-clear inner addresses or payload")
			requireNoSRv6Frame(t, bed, sidBytes, "exact-fit DF-clear packet must emit only one SRv6 frame")

			// Boundary + 1 (1437 bytes, DF clear): splits into 1500B + 85B
			// unfragmented outer IPv6+SRH packets.
			boundary := srv6DFClearDatagram(srv6EffectiveIPv4MTU+1, 0x2102)
			mplsMTUInject(bed, nil, boundary, false)
			boundaryLengths := requireSRv6IPv4Fragments(t, bed, sidBytes, boundary, nil, nil)
			require.Equal(t, []int{srv6LinkMTU, 85}, boundaryLengths,
				"1437-byte DF-clear IPv4 packet must emit 1500B and 85B outer IPv6+SRH packets")

			// Oversized (1460 bytes, DF clear): splits into 1500B + 108B
			// unfragmented outer IPv6+SRH packets.
			oversized := srv6DFClearDatagram(srv6DFClearTestSize, 0x2103)
			mplsMTUInject(bed, nil, oversized, false)
			oversizedLengths := requireSRv6IPv4Fragments(t, bed, sidBytes, oversized, nil, nil)
			require.Equal(t, []int{srv6LinkMTU, 108}, oversizedLengths,
				"1460-byte DF-clear IPv4 packet must emit 1500B and 108B outer IPv6+SRH packets")
			return
		}

		// Stock or 0014-only kernel without patch 0015: ip6_fragment drops the
		// oversized encapsulated packet (LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG).
		oversized := srv6DFClearDatagram(srv6DFClearTestSize, 0x2103)
		mplsMTUInject(bed, nil, oversized, false)
		unfragSRv6, fragOuterV6, icmpErrors, lengths := observeSRv6DFClearOutcome(bed, sidBytes)
		require.Zero(t, unfragSRv6, "unpatched kernel must drop oversized DF-clear IPv4 (LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG); lengths=%v", lengths)
		require.Zero(t, fragOuterV6, "unpatched kernel must not emit outer IPv6 fragments; lengths=%v", lengths)
		require.Zero(t, icmpErrors, "unpatched kernel must not emit ICMPv4 errors for DF-clear drop; lengths=%v", lengths)
	})
}

// TestFIBServiceSIDOversizedDFClearPreservesIPv4Options verifies RFC 791 IPv4
// option handling when an oversized DF-clear IPv4 packet carrying both a copied
// option (Router Alert) and a non-copied option (Record Route) fragments before
// SRv6 encapsulation.
//
// VALIDATES: AC-2, A-1 (RFC 791 Section 3.1). Fragment 1 carries Router Alert
// plus Record Route populated by ip_forward_options with the egress interface
// address (10.0.0.1, pointer advanced from 4 to 8), while fragment 2 retains
// copied Router Alert and replaces non-copied Record Route in-place with
// IPOPT_NOOP (0x01) via ip_options_fragment. Both leave as unfragmented outer
// IPv6+SRH packets <= 1500 bytes (1496B + 124B) with inner TTL 63, valid IPv4
// header checksums, and byte-for-byte payload reassembly.
// PREVENTS: dropping copied IPv4 options on later fragments or leaking
// non-copied Record Route options past fragment 1.
func TestFIBServiceSIDOversizedDFClearPreservesIPv4Options(t *testing.T) {
	skipWithoutInitNamespacePrivilege(t, srv6PrivilegeReason)
	patched := srv6ForwardFragPatched(t)
	if !patched {
		t.Skipf("kernel without patch 0015; DF-clear IPv4 option fragmentation requires CONFIG_IPV6_SEG6_FORWARD_FRAG")
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

		sentOptions := []byte{
			0x94, 0x04, 0x00, 0x00, // Router Alert (copied, copy bit 0x80 set).
			0x07, 0x07, 0x04, 0x00, 0x00, 0x00, 0x00, // Record Route (non-copied, one empty slot).
			0x01, // NOP padding to 12 bytes (IHL = 8, 32-byte IPv4 header).
		}
		expectedFirstOptions := []byte{
			0x94, 0x04, 0x00, 0x00,
			0x07, 0x07, 0x08, 10, 0, 0, 1, // Record Route updated by ip_forward_options with zempls0 address.
			0x01,
		}
		expectedLaterOptions := []byte{
			0x94, 0x04, 0x00, 0x00, // Copied Router Alert kept in fragment 2.
			0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, // Non-copied Record Route replaced with IPOPT_NOOP.
		}

		packet := srv6DFClearDatagramWithOptions(srv6DFClearTestSize, 0x2201, sentOptions)
		mplsMTUInject(bed, nil, packet, false)
		lengths := requireSRv6IPv4Fragments(t, bed, sidBytes, packet, expectedFirstOptions, expectedLaterOptions)
		require.Equal(t, []int{1496, 124}, lengths,
			"1460-byte DF-clear IPv4 packet with 12B options must emit 1496B and 124B outer IPv6+SRH packets")
	})
}

// TestFIBServiceSIDOversizedDFClearSubQuantumDropsCleanly verifies that an SRv6
// route whose explicit PathMTU (RTAX_MTU) leaves fewer than hlen + 8 bytes
// after the 64-byte seg6 headroom drops DF-clear IPv4 packets cleanly without
// encapsulating or livelocking the forwarding CPU.
//
// VALIDATES: AC-3, A-5. With PathMTU=84 (84 - 64 = 20 < 28 = 20 + 8), a 68-byte
// DF-clear IPv4 packet to 192.0.2.20 emits zero SRv6 frames, and a 44-byte
// marker packet to 192.0.2.21 injected immediately after on the same veth queue
// arrives encapsulated intact with inner TTL 63.
// PREVENTS: infinite zero-byte fragment livelock in ip_do_fragment or emitting
// malformed fragments when route PathMTU leaves less than one 8-byte quantum.
func TestFIBServiceSIDOversizedDFClearSubQuantumDropsCleanly(t *testing.T) {
	skipWithoutInitNamespacePrivilege(t, srv6PrivilegeReason)
	patched := srv6ForwardFragPatched(t)
	if !patched {
		t.Skipf("kernel without patch 0015; sub-quantum DF-clear IPv4 drop requires CONFIG_IPV6_SEG6_FORWARD_FRAG")
	}
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		bed := newMPLSTestbed(t, h)
		link, err := h.LinkByName(mplsZeLink)
		require.NoError(t, err)
		require.NoError(t, h.LinkSetMTU(link, srv6LinkMTU))
		mplsMTUReturnRoutes(t, h, link)

		sid := netip.MustParseAddr(srv6TestSID)
		sidBytes := sid.As16()
		backend := &netlinkBackend{handle: h}
		require.NoError(t, backend.addRichRoute(RichRoute{
			Prefix:  netip.MustParsePrefix(srv6SubQuantumIPv4Prefix),
			NextHop: mplsNextHop,
			SRv6SID: sid,
			PathMTU: srv6SubQuantumPathMTU,
		}))
		require.NoError(t, backend.addRichRoute(RichRoute{
			Prefix:  netip.MustParsePrefix(srv6MarkerIPv4Prefix),
			NextHop: mplsNextHop,
			SRv6SID: sid,
		}))

		subQuantumDst := netip.MustParseAddr(srv6TestIPv4Dst).As4()
		markerDst := netip.MustParseAddr(srv6MarkerIPv4Dst).As4()

		subQuantum := srv6DFClearDatagram(srv6SubQuantumPacketLen, 0x2301)
		mplsMTUInject(bed, nil, subQuantum, false)

		marker := srv6DFClearDatagram(srv6MarkerPacketLen, 0x2302)
		copy(marker[16:20], markerDst[:])
		mplsMTUChecksum(marker)
		mplsMTUInject(bed, nil, marker, false)

		markerFrame := bed.awaitForwarded("SRv6-encapsulated marker IPv4 packet", func(frame []byte) bool {
			if !isSRv6Frame(frame, sidBytes) {
				return false
			}
			if frame[ethernetHeaderLen+6] == unix.IPPROTO_FRAGMENT {
				t.Fatal("sub-quantum SRv6 route emitted an outer IPv6 fragment instead of dropping cleanly")
			}
			dst, ok := srv6InnerIPv4Dst(frame, sidBytes)
			if !ok {
				return false
			}
			if dst == subQuantumDst {
				t.Fatal("sub-quantum SRv6 route emitted a frame for 192.0.2.20 instead of dropping cleanly")
			}
			return dst == markerDst
		})
		outer, inner := extractSRv6Inner(t, markerFrame, unix.IPPROTO_IPIP, len(marker))
		require.Equal(t, byte(srv6DefaultIPv6HopLimit), outer[7], srv6MsgOuterHopLimit)
		require.Equal(t, marker[8]-1, inner[8], "marker packet must decrement inner IPv4 TTL once")
		require.Zero(t, internetChecksum(inner[:ipv4HeaderLen]), "marker inner IPv4 header checksum")
		require.True(t, slices.Equal(marker[12:], inner[12:]), "marker inner addresses or payload changed")
		requireNoSRv6Frame(t, bed, sidBytes, "sub-quantum DF-clear IPv4 packet emitted a trailing SRv6 frame")
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

func srv6DFClearDatagram(size int, id uint16) []byte {
	return srv6DFClearDatagramWithOptions(size, id, nil)
}

func srv6DFClearDatagramWithOptions(size int, id uint16, options []byte) []byte {
	hlen := ipv4HeaderLen + len(options)
	udpLen := size - hlen
	packet := make([]byte, size)
	packet[0] = 0x40 | byte(hlen/4)
	binary.BigEndian.PutUint16(packet[2:4], uint16(size))
	binary.BigEndian.PutUint16(packet[4:6], id)
	packet[8] = mplsTTL
	packet[9] = unix.IPPROTO_UDP
	copy(packet[12:16], netip.MustParseAddr(srv6TestIPv4Src).AsSlice())
	copy(packet[16:20], netip.MustParseAddr(srv6TestIPv4Dst).AsSlice())
	copy(packet[ipv4HeaderLen:hlen], options)
	mplsMTUChecksum(packet)

	udp := packet[hlen:]
	binary.BigEndian.PutUint16(udp[0:2], srv6UDPPort)
	binary.BigEndian.PutUint16(udp[2:4], srv6UDPPort)
	binary.BigEndian.PutUint16(udp[4:6], uint16(udpLen))
	payload := udp[udpHeaderLen:]
	for i := range payload {
		payload[i] = byte((i*31 + 7) & 0xff)
	}
	return packet
}

func srv6InnerIPv4Dst(frame []byte, sid [16]byte) ([4]byte, bool) {
	var zero [4]byte
	if !isSRv6Frame(frame, sid) {
		return zero, false
	}
	outer := frame[ethernetHeaderLen:]
	if outer[6] != unix.IPPROTO_ROUTING {
		return zero, false
	}
	if len(outer) < srv6IPv6HeaderLen+srv6SRHMinHeaderLen {
		return zero, false
	}
	srh := outer[srv6IPv6HeaderLen:]
	if srh[2] != srv6SRHRoutingType {
		return zero, false
	}
	if srh[0] != unix.IPPROTO_IPIP {
		return zero, false
	}
	srhLen := (int(srh[1]) + 1) * 8
	if len(srh) < srhLen+ipv4HeaderLen {
		return zero, false
	}
	inner := srh[srhLen:]
	if inner[0]>>4 != 4 {
		return zero, false
	}
	var dst [4]byte
	copy(dst[:], inner[16:20])
	return dst, true
}

func requireSRv6IPv4Fragments(
	t *testing.T,
	bed *mplsTestbed,
	sid [16]byte,
	original []byte,
	expectedFirstOptions, expectedLaterOptions []byte,
) []int {
	t.Helper()
	origHlen := int(original[0]&0x0f) * 4
	expectedTTL := original[8] - 1
	expectedPayload := original[origHlen:]
	reassembled := make([]byte, len(expectedPayload))
	seen := make([]bool, len(expectedPayload))
	received, fragments, hasFinal := 0, 0, false
	var outerLengths []int

	buf := make([]byte, srv6CaptureBufSize)
	deadline := time.Now().Add(mplsForwardWait)
	for received < len(expectedPayload) && time.Now().Before(deadline) {
		frame := bed.readForwarded(buf)
		if frame == nil {
			continue
		}
		if isICMPv4Frame(frame) {
			t.Fatal("oversized DF-clear IPv4 packet triggered an ICMPv4 error instead of fragmenting")
		}
		if !isSRv6Frame(frame, sid) {
			continue
		}
		require.Equal(t, []byte(mplsNextMAC), frame[:6], "SRv6 fragment return next hop")

		outer := frame[ethernetHeaderLen:]
		require.GreaterOrEqual(t, len(outer), srv6IPv6HeaderLen+srv6SRHMinHeaderLen, "truncated outer IPv6+SRH frame")
		require.NotEqual(t, byte(unix.IPPROTO_FRAGMENT), outer[6], "outer IPv6 must not carry a Fragment Header (NH=44)")
		require.Equal(t, byte(unix.IPPROTO_ROUTING), outer[6], "outer IPv6 next header must be Routing (43)")
		payloadLen := int(binary.BigEndian.Uint16(outer[4:6]))
		outerLen := srv6IPv6HeaderLen + payloadLen
		require.LessOrEqual(t, outerLen, len(outer), "outer IPv6 payload length exceeds captured frame")
		require.LessOrEqual(t, outerLen, srv6LinkMTU, "outer IPv6+SRH packet must fit within the 1500-byte egress link MTU")
		require.Equal(t, byte(srv6DefaultIPv6HopLimit), outer[7], srv6MsgOuterHopLimit)
		outer = outer[:outerLen]
		outerLengths = append(outerLengths, outerLen)

		srh := outer[srv6IPv6HeaderLen:]
		require.GreaterOrEqual(t, len(srh), srv6SRHMinHeaderLen, "truncated SRH")
		require.Equal(t, byte(srv6SRHRoutingType), srh[2], "missing Segment Routing Header")
		require.Equal(t, byte(unix.IPPROTO_IPIP), srh[0], "SRH next header must be IPIP (4)")
		srhLen := (int(srh[1]) + 1) * 8
		require.GreaterOrEqual(t, len(srh), srhLen+ipv4HeaderLen, "truncated inner IPv4 header after SRH")

		inner := srh[srhLen:]
		require.Equal(t, byte(4), inner[0]>>4, "inner packet must be IPv4")
		ihl := int(inner[0]&0x0f) * 4
		require.GreaterOrEqual(t, ihl, ipv4HeaderLen, "inner IPv4 IHL below 20 bytes")
		totalLen := int(binary.BigEndian.Uint16(inner[2:4]))
		require.Equal(t, len(inner), totalLen, "inner IPv4 total length must match SRH payload length")
		require.LessOrEqual(t, ihl, totalLen, "inner IPv4 IHL exceeds total length")
		require.Zero(t, internetChecksum(inner[:ihl]), "inner IPv4 fragment header checksum")
		require.Equal(t, expectedTTL, inner[8], "inner IPv4 fragment TTL must be decremented once")
		require.Equal(t, original[4:6], inner[4:6], "inner IPv4 fragment ID must match original")
		require.Equal(t, original[12:20], inner[12:20], "inner IPv4 fragment src/dst must match original")

		flagsOff := binary.BigEndian.Uint16(inner[6:8])
		require.Zero(t, flagsOff&srv6IPv4FlagDF, "inner IPv4 fragment must have DF clear")
		offset := int(flagsOff&srv6IPv4FragOffMask) * srv6FragUnit
		fragPayloadLen := totalLen - ihl
		if offset == 0 {
			require.True(t, slices.Equal(expectedFirstOptions, inner[ipv4HeaderLen:ihl]),
				"first-fragment IPv4 options mismatch: got %x want %x", inner[ipv4HeaderLen:ihl], expectedFirstOptions)
		} else {
			require.True(t, slices.Equal(expectedLaterOptions, inner[ipv4HeaderLen:ihl]),
				"later-fragment IPv4 options mismatch: got %x want %x", inner[ipv4HeaderLen:ihl], expectedLaterOptions)
		}

		end := offset + fragPayloadLen
		require.LessOrEqual(t, end, len(expectedPayload), "fragment extends past original IPv4 payload")
		if flagsOff&srv6IPv4FlagMF == 0 {
			require.False(t, hasFinal, "received multiple final IPv4 fragments")
			require.Equal(t, len(expectedPayload), end, "final IPv4 fragment must end at original payload length")
			hasFinal = true
		} else {
			require.Positive(t, fragPayloadLen, "non-final IPv4 fragment must carry non-zero payload")
			require.Zero(t, fragPayloadLen%srv6FragUnit, "non-final IPv4 fragment payload length must be 8-byte aligned")
		}

		for i := offset; i < end; i++ {
			require.False(t, seen[i], "overlapping IPv4 fragment byte at offset %d", i)
			seen[i] = true
		}
		copy(reassembled[offset:end], inner[ihl:totalLen])
		received += fragPayloadLen
		fragments++
	}

	require.GreaterOrEqual(t, fragments, 2, "oversized DF-clear IPv4 packet must produce at least two fragments")
	require.True(t, hasFinal, "missing final IPv4 fragment (MF=0)")
	require.Equal(t, len(expectedPayload), received, "incomplete inner IPv4 fragment payload coverage")
	require.True(t, slices.Equal(expectedPayload, reassembled), "reassembled inner IPv4 payload differs from original")

	silenceDeadline := time.Now().Add(mplsSilenceWait)
	for time.Now().Before(silenceDeadline) {
		frame := bed.readForwarded(buf)
		if frame == nil {
			continue
		}
		if isICMPv4Frame(frame) {
			t.Fatal("trailing ICMPv4 error frame arrived after DF-clear IPv4 fragmentation")
		}
		if isSRv6Frame(frame, sid) {
			t.Fatalf("unexpected trailing SRv6 frame (outer NH=%d, len=%d) after DF-clear IPv4 fragmentation",
				frame[ethernetHeaderLen+6], len(frame)-ethernetHeaderLen)
		}
	}
	return outerLengths
}

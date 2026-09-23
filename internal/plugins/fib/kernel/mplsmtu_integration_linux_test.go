//go:build integration && linux

// Design: docs/architecture/mpls/mpls-kernel.md -- native labeled IP path MTU.
// Related: mplsframe_integration_linux_test.go -- packet injection and capture.
package fibkernel

import (
	"bytes"
	"encoding/binary"
	"net"
	"net/netip"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
)

// TestMPLSIntegration_PathMTU exercises RFC 3209 Section 2.6 through the native
// forwarding owner's acknowledged installation. PathMTU is deliberately different
// from the device MTU. Transit retains label 900 while replacing 100 with two
// labels, so the packet itself determines the full outgoing stack overhead.
func TestMPLSIntegration_PathMTU(t *testing.T) {
	loadMPLSModules(t)
	for _, transit := range []bool{false, true} {
		role := "push"
		if transit {
			role = "transit"
		}
		for _, bound := range []struct {
			name string
			path uint32
			link int
		}{
			{name: "downstream-path", path: 1300, link: 1500},
			{name: "local-link", path: 1500, link: 1280},
		} {
			t.Run(role+"/"+bound.name, func(t *testing.T) {
				withNetNS(t, func() {
					h, err := netlink.NewHandle()
					require.NoError(t, err)
					defer h.Close()
					enableNetnsMPLS(t)
					bed := newMPLSTestbed(t, h)
					link, err := h.LinkByName(mplsZeLink)
					require.NoError(t, err)
					require.NoError(t, h.LinkSetMTU(link, bound.link))
					mplsMTUReturnRoutes(t, h, link)
					backend := newTestBackend(h)
					bus := newMPLSApplyBus(t, newFIBKernel(backend))
					entry := mplsfibevents.Entry{
						Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPush,
						FEC:       netip.MustParsePrefix("192.0.2.20/32"),
						OutLabels: []uint32{200, 300}, NextHop: mplsNextHop,
						PathMTU: bound.path,
					}
					var input []uint32
					output := []uint32{200, 300}
					if transit {
						entry.Op, entry.InLabel = mplsfibevents.OpSwap, 100
						input, output = []uint32{100, 900}, []uint32{200, 300, 900}
					}
					require.NoError(t, mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}))
					if !transit {
						entry.FEC = netip.MustParsePrefix("2001:db8:20::20/128")
						entry.NextHop = netip.MustParseAddr("2001:db8:1::2")
						require.NoError(t, mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}))
					}
					bed.enableInput()
					frameLimit := min(int(bound.path), bound.link)
					limit := frameLimit - labelEntryLen*len(output)

					// Exact fit must cross the same veth and native entry later used
					// for the negative assertions, with every label intact.
					for _, v6 := range []bool{false, true} {
						exact := mplsMTUDatagram(limit, v6, true)
						mplsMTUInject(bed, input, exact, v6)
						frame := bed.awaitForwarded("exact-fit labeled IP datagram", isMPLS)
						require.Equal(t, frameLimit, len(frame)-ethernetHeaderLen)
						require.Equal(t, output, labelStack(frame))
						require.Equal(t, []byte(mplsNextMAC), frame[:6])
						if !transit {
							if v6 {
								exact[7]--
							} else {
								exact[8]--
								mplsMTUChecksum(exact)
							}
						}
						require.Equal(t, exact, frame[ethernetHeaderLen+len(output)*labelEntryLen:])
					}

					// DF clear must deliver the complete payload as correctly labeled,
					// non-overlapping IPv4 fragments, including the final short one.
					fragmented := mplsMTUDatagram(limit+1, false, false)
					mplsMTUInject(bed, input, fragmented, false)
					mplsMTUFragments(t, bed, output, fragmented, limit, nil)

					// These packets still fit the input interface. Only the complete
					// outgoing stack or the downstream path makes them oversized.
					for _, v6 := range []bool{false, true} {
						oversize := mplsMTUDatagram(limit+1, v6, true)
						mplsMTUInject(bed, input, oversize, v6)
						mplsMTUError(t, bed, oversize, limit, v6)
					}
				})
			})
		}
	}
}

// TestMPLSIntegration_FragmentProgress keeps legal small path budgets, but
// requires every attempted fragment to contain at least one payload quantum.
// The marker follows the offending frame on the same veth receive queue: its
// arrival proves the forwarding CPU escaped the fragmentation operation.
func TestMPLSIntegration_FragmentProgress(t *testing.T) {
	loadMPLSModules(t)
	for _, tc := range []struct {
		name               string
		path               uint32
		link, labels, hlen int
	}{
		{name: "rounded-zero-payload", path: 68, link: 1500, labels: 11, hlen: 20},
		{name: "physical-link-underflow", path: 1500, link: 68, labels: 16, hlen: 20},
		{name: "IPv4-options", path: 68, link: 1500, labels: 1, hlen: 60},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withNetNS(t, func() {
				h, err := netlink.NewHandle()
				require.NoError(t, err)
				defer h.Close()
				enableNetnsMPLS(t)
				bed := newMPLSTestbed(t, h)
				link, err := h.LinkByName(mplsZeLink)
				require.NoError(t, err)
				require.NoError(t, h.LinkSetMTU(link, tc.link))
				require.NoError(t, os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0o644))
				require.NoError(t, os.WriteFile("/proc/sys/net/ipv4/conf/all/rp_filter", []byte("0"), 0o644))
				require.NoError(t, os.WriteFile("/proc/sys/net/ipv4/conf/"+mplsZeLink+"/rp_filter", []byte("0"), 0o644))
				labels := make([]uint32, tc.labels)
				for i := range labels {
					labels[i] = uint32(200 + i)
				}
				bus := newMPLSApplyBus(t, newFIBKernel(newTestBackend(h)))
				require.NoError(t, mplsfibevents.Apply(bus, []mplsfibevents.Entry{
					{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPush,
						FEC: netip.MustParsePrefix("192.0.2.20/32"), OutLabels: labels,
						NextHop: mplsNextHop, PathMTU: tc.path},
					{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPush,
						FEC: netip.MustParsePrefix("192.0.2.21/32"), OutLabels: []uint32{777},
						NextHop: mplsNextHop},
				}))
				// Keep the input below even the 68-byte physical MTU. Options
				// consume forty bytes; the eight-byte UDP datagram still fits.
				packet := mplsMTUDatagram(68, false, false)
				if tc.hlen > ipv4HeaderLen {
					copy(packet[tc.hlen:], packet[ipv4HeaderLen:ipv4HeaderLen+udpHeaderLen])
					for i := ipv4HeaderLen; i < tc.hlen; i++ {
						packet[i] = 1 // IPv4 NOP option
					}
					packet[0] = 0x40 | byte(tc.hlen/4)
					binary.BigEndian.PutUint16(packet[tc.hlen+4:tc.hlen+6], udpHeaderLen)
					mplsMTUChecksum(packet)
				}
				mplsMTUInject(bed, nil, packet, false)
				marker := mplsMTUDatagram(32, false, false)
				marker[19] = 21
				mplsMTUChecksum(marker)
				mplsMTUInject(bed, nil, marker, false)
				frame := bed.awaitForwarded("fragmentation progress marker", func(frame []byte) bool {
					if !isMPLS(frame) {
						return false
					}
					require.Equal(t, []uint32{777}, labelStack(frame), "unusable MTU emitted a fragment")
					return true
				})
				marker[8]--
				mplsMTUChecksum(marker)
				require.Equal(t, marker, frame[ethernetHeaderLen+labelEntryLen:ethernetHeaderLen+labelEntryLen+len(marker)])
			})
		})
	}
}

// MPLS transit has no IP forwarding pass to complete option slots reserved by
// ip_options_compile. Fragmentation must preserve them; ICMP needs route context.
func TestMPLSIntegration_TransitIPv4Options(t *testing.T) {
	loadMPLSModules(t)
	options := []byte{
		7, 7, 4, 0, 0, 0, 0, 1, // Record Route with one empty slot, then NOP.
		68, 12, 5, 1, 0, 0, 0, 0, 0, 0, 0, 0, // Timestamp with address.
	}
	for _, df := range []bool{false, true} {
		name := "df-clear"
		if df {
			name = "df-set"
		}
		t.Run(name, func(t *testing.T) {
			withNetNS(t, func() {
				h, err := netlink.NewHandle()
				require.NoError(t, err)
				defer h.Close()
				enableNetnsMPLS(t)
				bed := newMPLSTestbed(t, h)
				link, err := h.LinkByName(mplsZeLink)
				require.NoError(t, err)
				mplsMTUReturnRoutes(t, h, link)
				bus := newMPLSApplyBus(t, newFIBKernel(newTestBackend(h)))
				require.NoError(t, mplsfibevents.Apply(bus, []mplsfibevents.Entry{{
					Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap,
					InLabel: 100, OutLabels: []uint32{200, 300},
					NextHop: mplsNextHop, PathMTU: 1300,
				}}))
				bed.enableInput()
				output := []uint32{200, 300, 900}
				limit := 1300 - labelEntryLen*len(output)
				for _, size := range []int{limit, limit + 1} {
					base := mplsMTUDatagram(size-len(options), false, df)
					packet := make([]byte, size)
					hlen := ipv4HeaderLen + len(options)
					copy(packet, base[:ipv4HeaderLen])
					copy(packet[ipv4HeaderLen:], options)
					copy(packet[hlen:], base[ipv4HeaderLen:])
					packet[0] = 0x40 | byte(hlen/4)
					binary.BigEndian.PutUint16(packet[2:4], uint16(size))
					mplsMTUChecksum(packet)
					mplsMTUInject(bed, []uint32{100, 900}, packet, false)
					if size == limit {
						frame := bed.awaitForwarded("exact-fit IPv4 options", isMPLS)
						require.Equal(t, output, labelStack(frame))
						require.Equal(t, packet, frame[ethernetHeaderLen+len(output)*labelEntryLen:])
						continue
					}
					if !df {
						mplsMTUFragments(t, bed, output, packet, limit, bytes.Repeat([]byte{1}, len(options)))
						continue
					}
					quote := mplsMTUError(t, bed, packet, limit, false)
					require.GreaterOrEqual(t, len(quote), hlen+udpHeaderLen)
					got := quote[ipv4HeaderLen:hlen]
					require.Equal(t, byte(8), got[2], "Record Route pointer")
					require.Equal(t, []byte{10, 0, 0, 1}, got[3:7], "Record Route address")
					require.Equal(t, byte(13), got[10], "Timestamp pointer")
					require.Equal(t, []byte{10, 0, 0, 1}, got[12:16], "Timestamp address")
					require.Less(t, binary.BigEndian.Uint32(got[16:20]), uint32(24*time.Hour/time.Millisecond))
				}
			})
		})
	}
}

func mplsMTUReturnRoutes(t *testing.T, h *netlink.Handle, link netlink.Link) {
	t.Helper()
	for _, knob := range []struct{ path, value string }{
		{"/proc/sys/net/ipv4/ip_forward", "1"},
		{"/proc/sys/net/ipv4/conf/all/rp_filter", "0"},
		{"/proc/sys/net/ipv4/conf/" + mplsZeLink + "/rp_filter", "0"},
		{"/proc/sys/net/ipv4/conf/all/send_redirects", "0"},
		{"/proc/sys/net/ipv4/conf/" + mplsZeLink + "/send_redirects", "0"},
		{"/proc/sys/net/ipv4/icmp_errors_use_inbound_ifaddr", "1"},
		{"/proc/sys/net/ipv6/conf/all/disable_ipv6", "0"},
		{"/proc/sys/net/ipv6/conf/" + mplsZeLink + "/disable_ipv6", "0"},
		{"/proc/sys/net/ipv6/conf/all/forwarding", "1"},
	} {
		require.NoError(t, os.WriteFile(knob.path, []byte(knob.value), 0o644), "write %s", knob.path)
	}
	addr, err := netlink.ParseAddr("2001:db8:1::1/64")
	require.NoError(t, err)
	addr.Flags = unix.IFA_F_NODAD
	require.NoError(t, h.AddrAdd(link, addr))
	require.NoError(t, h.NeighSet(&netlink.Neigh{
		LinkIndex: link.Attrs().Index, Family: unix.AF_INET6,
		State: netlink.NUD_PERMANENT, IP: net.ParseIP("2001:db8:1::2"),
		HardwareAddr: mplsNextMAC,
	}))
	source, err := netlink.ParseAddr("10.0.0.9/32")
	require.NoError(t, err)
	require.NoError(t, h.AddrAdd(link, source))
	// IPv4 return reachability exists only for ICMP. A preflight lookup with
	// protocol zero would discard the error before the native sender runs.
	rule := netlink.NewRule()
	rule.Family, rule.Table, rule.Priority = unix.AF_INET, 200, 100
	rule.IPProto = unix.IPPROTO_ICMP
	require.NoError(t, h.RuleAdd(rule))
	for _, route := range []struct{ prefix, gateway string }{
		{"192.0.2.10/32", "10.0.0.2"},
		{"2001:db8:10::10/128", "2001:db8:1::2"},
	} {
		_, prefix, err := net.ParseCIDR(route.prefix)
		require.NoError(t, err)
		native := &netlink.Route{Dst: prefix, Gw: net.ParseIP(route.gateway), LinkIndex: link.Attrs().Index}
		if prefix.IP.To4() != nil {
			native.Table = 200
			native.Flags = unix.RTNH_F_ONLINK
			// The requested inbound address must beat this route preference.
			native.Src = net.ParseIP("10.0.0.9")
		}
		require.NoError(t, h.RouteAdd(native))
	}
}

func mplsMTUDatagram(size int, v6, df bool) []byte {
	if v6 {
		packet := make([]byte, size)
		packet[0], packet[6], packet[7] = 0x60, unix.IPPROTO_UDP, mplsTTL
		binary.BigEndian.PutUint16(packet[4:6], uint16(size-40))
		copy(packet[8:24], netip.MustParseAddr("2001:db8:10::10").AsSlice())
		copy(packet[24:40], netip.MustParseAddr("2001:db8:20::20").AsSlice())
		udp := packet[40:]
		binary.BigEndian.PutUint16(udp[0:2], 4000)
		binary.BigEndian.PutUint16(udp[2:4], 4000)
		binary.BigEndian.PutUint16(udp[4:6], uint16(len(udp)))
		copy(udp[8:], bytes.Repeat([]byte{0x5a}, len(udp)-8))
		pseudo := make([]byte, 40+len(udp))
		copy(pseudo[:32], packet[8:40])
		binary.BigEndian.PutUint32(pseudo[32:36], uint32(len(udp)))
		pseudo[39] = unix.IPPROTO_UDP
		copy(pseudo[40:], udp)
		checksum := internetChecksum(pseudo)
		if checksum == 0 {
			checksum = 0xffff
		}
		binary.BigEndian.PutUint16(udp[6:8], checksum)
		return packet
	}
	packet := ipv4UDP(netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("192.0.2.20"),
		4000, bytes.Repeat([]byte{0x5a}, size-ipv4HeaderLen-udpHeaderLen))
	binary.BigEndian.PutUint16(packet[4:6], 0x1234)
	if df {
		binary.BigEndian.PutUint16(packet[6:8], 0x4000)
	}
	mplsMTUChecksum(packet)
	return packet
}

func mplsMTUChecksum(packet []byte) {
	binary.BigEndian.PutUint16(packet[10:12], 0)
	binary.BigEndian.PutUint16(packet[10:12], internetChecksum(packet[:int(packet[0]&15)*4]))
}

func mplsMTUInject(bed *mplsTestbed, labels []uint32, packet []byte, v6 bool) {
	bed.t.Helper()
	if len(labels) != 0 {
		bed.inject(labels, packet)
		return
	}
	frame := mplsFrame(bed.zeMAC, nil, packet)
	protocol := uint16(unix.ETH_P_IP)
	if v6 {
		protocol = unix.ETH_P_IPV6
	}
	binary.BigEndian.PutUint16(frame[12:14], protocol)
	address := &unix.SockaddrLinklayer{Protocol: htons(unix.ETH_P_ALL), Ifindex: bed.peerIndex, Halen: 6}
	copy(address.Addr[:], bed.zeMAC)
	require.NoError(bed.t, unix.Sendto(bed.injectFD, frame, 0, address))
}

func mplsMTUFragments(t *testing.T, bed *mplsTestbed, labels []uint32, original []byte, limit int, laterOptions []byte) {
	t.Helper()
	originalHeaderLen := int(original[0]&15) * 4
	payload := make([]byte, len(original)-originalHeaderLen)
	seen := make([]bool, len(payload))
	received, fragments, last := 0, 0, false
	deadline := time.Now().Add(mplsForwardWait)
	buffer := make([]byte, 4096)
	for received < len(payload) && time.Now().Before(deadline) {
		frame := bed.readForwarded(buffer)
		if frame == nil || !isMPLS(frame) {
			continue
		}
		require.True(t, slices.Equal(labels, labelStack(frame)), "fragment label stack")
		require.Equal(t, []byte(mplsNextMAC), frame[:6])
		packet := frame[ethernetHeaderLen+labelEntryLen*len(labels):]
		require.GreaterOrEqual(t, len(packet), ipv4HeaderLen)
		hlen := int(packet[0]&15) * 4
		length := int(binary.BigEndian.Uint16(packet[2:4]))
		require.Equal(t, len(packet), length)
		require.GreaterOrEqual(t, hlen, ipv4HeaderLen)
		require.LessOrEqual(t, hlen, length)
		require.LessOrEqual(t, length, limit)
		require.Equal(t, uint16(0), internetChecksum(packet[:hlen]))
		require.Equal(t, original[4:6], packet[4:6], "fragment ID")
		require.Equal(t, original[12:20], packet[12:20], "fragment endpoints")
		flags := binary.BigEndian.Uint16(packet[6:8])
		require.Zero(t, flags&0x4000, "DF on emitted fragment")
		offset := int(flags&0x1fff) * 8
		if offset == 0 {
			require.Equal(t, original[ipv4HeaderLen:originalHeaderLen], packet[ipv4HeaderLen:hlen], "first-fragment options")
		} else {
			require.True(t, bytes.Equal(laterOptions, packet[ipv4HeaderLen:hlen]), "later-fragment options: %x", packet[ipv4HeaderLen:hlen])
		}
		end := offset + length - hlen
		require.LessOrEqual(t, end, len(payload))
		if flags&0x2000 == 0 {
			require.False(t, last, "multiple final fragments")
			require.Equal(t, len(payload), end)
			last = true
		} else {
			require.Zero(t, (length-hlen)%8, "non-final fragment alignment")
		}
		for i := offset; i < end; i++ {
			require.False(t, seen[i], "overlap at byte %d", i)
			seen[i] = true
		}
		copy(payload[offset:end], packet[hlen:length])
		received += end - offset
		fragments++
	}
	require.Greater(t, fragments, 1)
	require.True(t, last, "missing final fragment")
	require.Equal(t, len(payload), received, "fragment coverage")
	require.Equal(t, original[originalHeaderLen:], payload, "reassembled payload")
}

func mplsMTUError(t *testing.T, bed *mplsTestbed, original []byte, mtu int, v6 bool) []byte {
	t.Helper()
	frame := bed.awaitForwarded("ICMP path MTU error", func(frame []byte) bool {
		if isMPLS(frame) {
			t.Fatal("forwarded oversized original instead of returning ICMP")
		}
		if v6 {
			return len(frame) >= ethernetHeaderLen+48 && binary.BigEndian.Uint16(frame[12:14]) == unix.ETH_P_IPV6 && frame[ethernetHeaderLen+6] == unix.IPPROTO_ICMPV6
		}
		return len(frame) >= ethernetHeaderLen+28 && isIPv4(frame) && frame[ethernetHeaderLen+9] == unix.IPPROTO_ICMP
	})
	require.Equal(t, []byte(mplsNextMAC), frame[:6], "ICMP return next hop")
	packet := frame[ethernetHeaderLen:]
	var quote []byte
	if v6 {
		require.Equal(t, original[8:24], packet[24:40], "PTB destination")
		icmp := packet[40:]
		require.Equal(t, byte(2), icmp[0])
		require.Zero(t, icmp[1])
		require.Equal(t, uint32(mtu), binary.BigEndian.Uint32(icmp[4:8]))
		require.GreaterOrEqual(t, len(icmp), 8+40)
		require.Equal(t, original[:7], icmp[8:15], "PTB quotes original IPv6 header")
		quote = icmp[8:]
		require.Equal(t, original[8:40], icmp[16:48], "PTB quotes original endpoints")
		pseudo := make([]byte, 40+len(icmp))
		copy(pseudo[:32], packet[8:40])
		binary.BigEndian.PutUint32(pseudo[32:36], uint32(len(icmp)))
		pseudo[39] = unix.IPPROTO_ICMPV6
		copy(pseudo[40:], icmp)
		require.Zero(t, internetChecksum(pseudo), "ICMPv6 checksum")
	} else {
		hlen := int(packet[0]&15) * 4
		length := int(binary.BigEndian.Uint16(packet[2:4]))
		require.LessOrEqual(t, length, len(packet))
		require.Equal(t, original[12:16], packet[16:20], "ICMP destination")
		require.Equal(t, []byte{10, 0, 0, 1}, packet[12:16], "inbound ICMP source policy")
		require.Zero(t, internetChecksum(packet[:hlen]), "outer IP checksum")
		icmp := packet[hlen:length]
		require.GreaterOrEqual(t, len(icmp), 8+ipv4HeaderLen+udpHeaderLen)
		require.Equal(t, byte(3), icmp[0])
		require.Equal(t, byte(4), icmp[1])
		require.Equal(t, uint16(mtu), binary.BigEndian.Uint16(icmp[6:8]))
		require.Zero(t, internetChecksum(icmp), "ICMP checksum")
		require.Equal(t, original[:8], icmp[8:16], "ICMP quotes original length, ID and DF")
		quote = icmp[8:]
		originalHeaderLen := int(original[0]&15) * 4
		require.GreaterOrEqual(t, len(quote), originalHeaderLen+udpHeaderLen)
		require.Equal(t, original[12:20], quote[12:20], "ICMP quotes endpoints")
		require.Equal(t, original[originalHeaderLen:originalHeaderLen+udpHeaderLen],
			quote[originalHeaderLen:originalHeaderLen+udpHeaderLen], "ICMP quotes UDP header")
	}
	buffer := make([]byte, 4096)
	deadline := time.Now().Add(mplsSilenceWait)
	for time.Now().Before(deadline) {
		frame := bed.readForwarded(buffer)
		if frame != nil && isMPLS(frame) {
			t.Fatal("oversized original was forwarded after its ICMP error")
		}
	}
	return quote
}

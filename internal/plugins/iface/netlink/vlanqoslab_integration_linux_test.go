// Design: docs/architecture/iface/vlan-qos-map.md -- VLAN QoS wire-level lab tests

//go:build integration && linux

package ifacenetlink

import (
	"encoding/binary"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

// ── netns + veth helpers ────────────────────────────────────────────────────

func labNSName(testName string) string {
	name := strings.NewReplacer("/", "_", " ", "_", "(", "", ")", "").Replace(testName)
	if len(name) > 8 {
		name = name[len(name)-8:]
	}
	return "zlab_" + name
}

func withLabNetNS(t *testing.T, fn func()) {
	t.Helper()

	runtime.LockOSThread()
	unlocked := false
	unlock := func() {
		if !unlocked {
			runtime.UnlockOSThread()
			unlocked = true
		}
	}

	origNS, err := netns.Get()
	if err != nil {
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}

	nsName := labNSName(t.Name())
	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close()
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}

	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("restore namespace: %v", restoreErr)
		}
		origNS.Close()
		newNS.Close()
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort netns cleanup
		unlock()
	})

	if err := netlink.LinkAdd(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: "ze0"},
		PeerName:  "ze1",
	}); err != nil {
		t.Fatalf("add veth: %v", err)
	}
	for _, name := range []string{"ze0", "ze1"} {
		link, lErr := netlink.LinkByName(name)
		if lErr != nil {
			t.Fatalf("link %q: %v", name, lErr)
		}
		if uErr := netlink.LinkSetUp(link); uErr != nil {
			t.Fatalf("up %q: %v", name, uErr)
		}
	}

	fn()
}

func createLabVLAN(t *testing.T, parent string, vlanID int, ingressMap, egressMap map[uint32]uint32) {
	t.Helper()
	b := &netlinkBackend{}
	if err := b.CreateVLAN(iface.VLANSpec{
		Parent:        parent,
		VLANID:        vlanID,
		IngressQoSMap: ingressMap,
		EgressQoSMap:  egressMap,
	}); err != nil {
		t.Fatalf("CreateVLAN %s.%d: %v", parent, vlanID, err)
	}
}

func assignIP(t *testing.T, ifName, cidr string) {
	t.Helper()
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		t.Fatalf("link %q: %v", ifName, err)
	}
	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		t.Fatalf("parse addr %q: %v", cidr, err)
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		t.Fatalf("addr add %q to %q: %v", cidr, ifName, err)
	}
}

func addStaticNeighbor(t *testing.T, ifName string, ip net.IP, mac net.HardwareAddr) {
	t.Helper()
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		t.Fatalf("link %q: %v", ifName, err)
	}
	if err := netlink.NeighAdd(&netlink.Neigh{
		LinkIndex:    link.Attrs().Index,
		IP:           ip,
		HardwareAddr: mac,
		State:        netlink.NUD_PERMANENT,
	}); err != nil {
		t.Fatalf("neigh add on %q: %v", ifName, err)
	}
}

func linkMAC(t *testing.T, ifName string) [6]byte {
	t.Helper()
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		t.Fatalf("link %q: %v", ifName, err)
	}
	var mac [6]byte
	copy(mac[:], link.Attrs().HardwareAddr)
	return mac
}

// ── AF_PACKET helpers (ingress frame injection) ─────────────────────────────

func htons(v uint16) uint16 { return (v >> 8) | (v << 8) }

func sendRawFrame(t *testing.T, ifName string, frame []byte) {
	t.Helper()
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		t.Fatalf("link %q: %v", ifName, err)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ALL)))
	if err != nil {
		t.Fatalf("AF_PACKET send socket: %v", err)
	}
	defer unix.Close(fd)
	addr := &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ALL),
		Ifindex:  link.Attrs().Index,
		Halen:    6,
	}
	copy(addr.Addr[:6], frame[:6])
	if err := unix.Sendto(fd, frame, 0, addr); err != nil {
		t.Fatalf("sendto raw on %q: %v", ifName, err)
	}
}

// buildMinimalIPUDP returns a valid IPv4/UDP payload (28 bytes) with a correct
// IP header checksum. UDP checksum is 0 (optional for IPv4).
func buildMinimalIPUDP(srcIP, dstIP [4]byte) []byte {
	ip := make([]byte, 20)
	ip[0] = 0x45 // version 4, IHL 5
	binary.BigEndian.PutUint16(ip[2:4], 28)
	ip[8] = 64 // TTL
	ip[9] = 17 // protocol UDP
	copy(ip[12:16], srcIP[:])
	copy(ip[16:20], dstIP[:])
	var sum uint32
	for i := 0; i < 20; i += 2 {
		sum += uint32(binary.BigEndian.Uint16(ip[i : i+2]))
	}
	for sum > 0xffff {
		sum = (sum >> 16) + (sum & 0xffff)
	}
	binary.BigEndian.PutUint16(ip[10:12], ^uint16(sum))

	udp := make([]byte, 8)
	binary.BigEndian.PutUint16(udp[0:2], 12345)
	binary.BigEndian.PutUint16(udp[2:4], 9999)
	binary.BigEndian.PutUint16(udp[4:6], 8)
	return append(ip, udp...)
}

// ── UDP send helpers ────────────────────────────────────────────────────────

func sendUDPWithPriority(t *testing.T, src, dst [4]byte, dstPort, priority int) {
	t.Helper()
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		t.Fatalf("UDP socket: %v", err)
	}
	defer unix.Close(fd)
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_PRIORITY, priority); err != nil {
		t.Fatalf("SO_PRIORITY=%d: %v", priority, err)
	}
	if err := unix.Bind(fd, &unix.SockaddrInet4{Addr: src}); err != nil {
		t.Fatalf("bind %v: %v", src, err)
	}
	if err := unix.Sendto(fd, []byte("qos-test"), 0, &unix.SockaddrInet4{
		Port: dstPort, Addr: dst,
	}); err != nil {
		t.Fatalf("sendto: %v", err)
	}
}

func sendUDPWithTOS(t *testing.T, src, dst [4]byte, dstPort, tos int) {
	t.Helper()
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		t.Fatalf("UDP socket: %v", err)
	}
	defer unix.Close(fd)
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_TOS, tos); err != nil {
		t.Fatalf("IP_TOS=0x%02x: %v", tos, err)
	}
	if err := unix.Bind(fd, &unix.SockaddrInet4{Addr: src}); err != nil {
		t.Fatalf("bind %v: %v", src, err)
	}
	if err := unix.Sendto(fd, []byte("dscp-test"), 0, &unix.SockaddrInet4{
		Port: dstPort, Addr: dst,
	}); err != nil {
		t.Fatalf("sendto: %v", err)
	}
}

// ── nftables helpers ────────────────────────────────────────────────────────

func nftExec(t *testing.T, script string) {
	t.Helper()
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nft: %v\n%s", err, out)
	}
}

func nftDelete(family, table string) {
	exec.Command("nft", "delete", "table", family, table).Run() //nolint:errcheck // best-effort nft cleanup
}

// nftCounterPackets reads the first "counter packets N" value from a table.
func nftCounterPackets(t *testing.T, family, table string) uint64 {
	t.Helper()
	out, err := exec.Command("nft", "list", "table", family, table).CombinedOutput()
	if err != nil {
		t.Fatalf("nft list table %s %q: %v\n%s", family, table, err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		idx := strings.Index(line, "counter packets ")
		if idx < 0 {
			continue
		}
		s := line[idx+len("counter packets "):]
		fields := strings.Fields(s)
		if len(fields) == 0 {
			continue
		}
		n, pErr := strconv.ParseUint(fields[0], 10, 64)
		if pErr != nil {
			t.Fatalf("parse counter from %q: %v", line, pErr)
		}
		return n
	}
	t.Fatal("no counter found in nft output")
	return 0
}

// nftCounterByLabel reads a counter from the line containing label.
func nftCounterByLabel(t *testing.T, family, table, label string) uint64 {
	t.Helper()
	out, err := exec.Command("nft", "list", "table", family, table).CombinedOutput()
	if err != nil {
		t.Fatalf("nft list table %s %q: %v\n%s", family, table, err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, label) {
			continue
		}
		idx := strings.Index(line, "counter packets ")
		if idx < 0 {
			continue
		}
		s := line[idx+len("counter packets "):]
		fields := strings.Fields(s)
		if len(fields) == 0 {
			continue
		}
		n, pErr := strconv.ParseUint(fields[0], 10, 64)
		if pErr != nil {
			t.Fatalf("parse counter from %q: %v", line, pErr)
		}
		return n
	}
	t.Fatalf("no counter with label %q in nft output:\n%s", label, out)
	return 0
}

// ── tests ───────────────────────────────────────────────────────────────────

// TestVLANQoSEgressPCPOnWire verifies that the kernel's VLAN egress QoS map
// stamps the correct PCP bits in the 802.1Q header on the wire, using nftables
// netdev ingress on the peer veth to match vlan pcp (handles hw-accelerated tags).
//
// VALIDATES: spec-vlan-qos-lab AC-1 (mapped priority -> correct PCP) and
// AC-2 (unmapped priority -> PCP 0). Also validates assumptions A-1 (veth
// preserves 802.1Q tags), A-2 (SO_PRIORITY sets skb->priority), and
// A-5 (VLAN sub-interface on veth behaves like physical NIC).
// PREVENTS: egress QoS map silently not applied, or PCP bits corrupted by
// VLAN offload on the veth path.
func TestVLANQoSEgressPCPOnWire(t *testing.T) {
	if _, err := exec.LookPath("nft"); err != nil {
		t.Skip("nft not available")
	}

	withLabNetNS(t, func() {
		createLabVLAN(t, "ze0", 100, nil, map[uint32]uint32{6: 6})
		assignIP(t, "ze0.100", "10.0.0.1/24")
		addStaticNeighbor(t, "ze0.100",
			net.IPv4(10, 0, 0, 2),
			net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x02})

		src := [4]byte{10, 0, 0, 1}
		dst := [4]byte{10, 0, 0, 2}

		// AC-1: priority 6, egress map 6:6 -> PCP 6.
		// nftables netdev ingress on ze1: the vlan match handles both inline
		// tags and hw-accelerated skb->vlan_tci.
		nftExec(t, `
table netdev zelab_pcp6 {
	chain ingress {
		type filter hook ingress device "ze1" priority 0;
		vlan id 100 vlan pcp 6 counter accept
	}
}
`)
		t.Cleanup(func() { nftDelete("netdev", "zelab_pcp6") })

		sendUDPWithPriority(t, src, dst, 9999, 6)
		pcp6 := nftCounterPackets(t, "netdev", "zelab_pcp6")
		if pcp6 == 0 {
			t.Errorf("AC-1: pcp6 counter = 0, want >0 (priority 6 with egress map 6:6)")
		}

		// AC-2: priority 3 (no map entry) -> PCP 0
		nftDelete("netdev", "zelab_pcp6")
		nftExec(t, `
table netdev zelab_pcp0 {
	chain ingress {
		type filter hook ingress device "ze1" priority 0;
		vlan id 100 vlan pcp 0 counter accept
	}
}
`)
		t.Cleanup(func() { nftDelete("netdev", "zelab_pcp0") })

		sendUDPWithPriority(t, src, dst, 9998, 3)
		pcp0 := nftCounterPackets(t, "netdev", "zelab_pcp0")
		if pcp0 == 0 {
			t.Errorf("AC-2: pcp0 counter = 0, want >0 (unmapped priority 3 defaults to PCP 0)")
		}
	})
}

// TestVLANQoSIngressClassification verifies that the kernel's VLAN ingress
// QoS map translates PCP bits from incoming 802.1Q frames into skb->priority,
// observable through nftables meta priority counters.
//
// VALIDATES: spec-vlan-qos-lab AC-3 (PCP 6 with ingress map 6:6 -> priority 6
// counter fires) and AC-4 (PCP 6 without ingress map -> counter does NOT fire).
// Also validates assumption A-3 (nft counters expose the mapped priority).
// PREVENTS: ingress QoS map silently ignored, or PCP-to-priority mapping
// producing ambient classification independent of the map configuration.
func TestVLANQoSIngressClassification(t *testing.T) {
	if _, err := exec.LookPath("nft"); err != nil {
		t.Skip("nft not available")
	}

	withLabNetNS(t, func() {
		ze0MAC := linkMAC(t, "ze0")
		ze1MAC := linkMAC(t, "ze1")

		// AC-3: VLAN with ingress-qos-map 6:6
		createLabVLAN(t, "ze0", 100, map[uint32]uint32{6: 6}, nil)
		assignIP(t, "ze0.100", "10.0.0.1/24")

		nftExec(t, `
table ip zelab {
	chain prerouting {
		type filter hook prerouting priority -300;
		iifname "ze0.100" meta priority 6 counter accept
	}
}
`)
		t.Cleanup(func() { nftDelete("ip", "zelab") })

		ipPayload := buildMinimalIPUDP([4]byte{10, 0, 0, 2}, [4]byte{10, 0, 0, 1})
		frame := buildTaggedFrame(ze0MAC, ze1MAC, 100, 6, 0x0800, ipPayload)
		sendRawFrame(t, "ze1", frame)

		count := nftCounterPackets(t, "ip", "zelab")
		if count == 0 {
			t.Errorf("AC-3: priority-6 counter = 0, want >0 (PCP 6 with ingress map 6:6)")
		}

		// AC-4: VLAN without ingress map -- PCP 6 must NOT produce priority 6
		createLabVLAN(t, "ze0", 200, nil, nil)
		assignIP(t, "ze0.200", "10.0.1.1/24")

		nftExec(t, `
table ip zelab4 {
	chain prerouting {
		type filter hook prerouting priority -300;
		iifname "ze0.200" meta priority 6 counter accept
	}
}
`)
		t.Cleanup(func() { nftDelete("ip", "zelab4") })

		ipPayload4 := buildMinimalIPUDP([4]byte{10, 0, 1, 2}, [4]byte{10, 0, 1, 1})
		frame4 := buildTaggedFrame(ze0MAC, ze1MAC, 200, 6, 0x0800, ipPayload4)
		sendRawFrame(t, "ze1", frame4)

		count4 := nftCounterPackets(t, "ip", "zelab4")
		if count4 != 0 {
			t.Errorf("AC-4: priority-6 counter = %d, want 0 (PCP 6 without ingress map)", count4)
		}
	})
}

// TestVLANQoSDSCPFullChain verifies the full BNG-style chain: IP traffic with
// DSCP CS6 is classified by nftables to priority 6, which the VLAN egress map
// then stamps as PCP 6 in the 802.1Q header. PCP is verified via nftables netdev
// vlan pcp match on the peer veth.
//
// VALIDATES: spec-vlan-qos-lab AC-5 (DSCP CS6 -> nftables priority 6 -> egress
// map PCP 6). This is the Ze equivalent of the Juniper MX480 dynamic-profile
// CoS scenario that motivated the feature.
// PREVENTS: DSCP classification and VLAN PCP stamping being tested in isolation
// but never proven to work as a pipeline.
func TestVLANQoSDSCPFullChain(t *testing.T) {
	if _, err := exec.LookPath("nft"); err != nil {
		t.Skip("nft not available")
	}

	withLabNetNS(t, func() {
		createLabVLAN(t, "ze0", 100, nil, map[uint32]uint32{6: 6})
		assignIP(t, "ze0.100", "10.0.0.1/24")
		addStaticNeighbor(t, "ze0.100",
			net.IPv4(10, 0, 0, 2),
			net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x02})

		// nftables: DSCP CS6 -> skb->priority 6
		nftExec(t, `
table ip zechain {
	chain output {
		type route hook output priority mangle;
		ip dscp cs6 meta priority set 6
	}
}
`)
		t.Cleanup(func() { nftDelete("ip", "zechain") })

		// nftables netdev ingress on ze1: verify PCP 6 on the wire
		nftExec(t, `
table netdev zelab_dscp {
	chain ingress {
		type filter hook ingress device "ze1" priority 0;
		vlan id 100 vlan pcp 6 counter accept
	}
}
`)
		t.Cleanup(func() { nftDelete("netdev", "zelab_dscp") })

		src := [4]byte{10, 0, 0, 1}
		dst := [4]byte{10, 0, 0, 2}

		// AC-5: DSCP CS6 (TOS 0xC0) -> priority 6 -> PCP 6
		sendUDPWithTOS(t, src, dst, 9999, 0xC0)
		pcp6 := nftCounterPackets(t, "netdev", "zelab_dscp")
		if pcp6 == 0 {
			t.Errorf("AC-5: pcp6 counter = 0, want >0 (DSCP CS6 -> priority 6 -> PCP 6)")
		}
	})
}

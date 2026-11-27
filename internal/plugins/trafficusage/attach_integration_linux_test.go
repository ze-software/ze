//go:build integration && linux

// VALIDATES: real link.AttachTCX on a veth interface, that injected frames
// traverse the attached ingress/egress programs and update the maps, that Stop
// detaches cleanly, and that the poller publishes ze_traffic_usage_* to a live
// /metrics endpoint (ACs 1, 2, 3, 4, 10, 16; End-to-End User Stories 1 and 2).
// PREVENTS: an attach path that loads but never sees traffic, leaked links on
// stop, or metrics that never reach /metrics.

package trafficusage

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/metrics"
)

// etherTypeAllNetwork is ETH_P_ALL in network byte order, as AF_PACKET wants
// it, on a little-endian host.
const etherTypeAllNetwork uint16 = unix.ETH_P_ALL<<8 | unix.ETH_P_ALL>>8

// disableIPv6 stops the kernel from assigning link-local addresses and sending
// unsolicited DAD/RS/MLD IPv6 frames when a test veth pair comes up, so raw
// AF_PACKET tests observe only the frames they inject.
func disableIPv6(t *testing.T, ifname string) {
	t.Helper()
	err := os.WriteFile("/proc/sys/net/ipv6/conf/"+ifname+"/disable_ipv6", []byte("1"), 0)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("disable ipv6 on %s: %v", ifname, err)
	}
}

// createVethPair creates an up/up veth pair and returns the ifindex of `a`.
func createVethPair(t *testing.T, a, b string) int {
	t.Helper()
	la := netlink.NewLinkAttrs()
	la.Name = a
	veth := &netlink.Veth{LinkAttrs: la, PeerName: b}
	if err := netlink.LinkAdd(veth); err != nil {
		t.Skipf("cannot create veth (need CAP_NET_ADMIN): %v", err)
	}
	linkA, err := netlink.LinkByName(a)
	if err != nil {
		t.Fatalf("LinkByName %s: %v", a, err)
	}
	t.Cleanup(func() { _ = netlink.LinkDel(linkA) })
	linkB, err := netlink.LinkByName(b)
	if err != nil {
		t.Fatalf("LinkByName %s: %v", b, err)
	}
	disableIPv6(t, a)
	disableIPv6(t, b)
	if err := netlink.LinkSetUp(linkA); err != nil {
		t.Fatalf("set up %s: %v", a, err)
	}
	if err := netlink.LinkSetUp(linkB); err != nil {
		t.Fatalf("set up %s: %v", b, err)
	}
	return linkA.Attrs().Index
}

// injectFrame transmits a raw Ethernet frame out of ifname via AF_PACKET. A
// frame sent out the peer arrives on the other veth end's ingress; a frame sent
// out an interface traverses that interface's egress.
func injectFrame(t *testing.T, ifname string, frame []byte) {
	t.Helper()
	ifi, err := net.InterfaceByName(ifname)
	if err != nil {
		t.Fatalf("InterfaceByName %s: %v", ifname, err)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(etherTypeAllNetwork))
	if err != nil {
		t.Skipf("cannot open AF_PACKET socket (need CAP_NET_RAW): %v", err)
	}
	defer func() { _ = unix.Close(fd) }()
	addr := &unix.SockaddrLinklayer{Protocol: etherTypeAllNetwork, Ifindex: ifi.Index}
	if err := unix.Sendto(fd, frame, 0, addr); err != nil {
		t.Fatalf("sendto %s: %v", ifname, err)
	}
}

// waitCounts polls att.Counts until check passes or the deadline elapses (the
// veth receive path is asynchronous).
func waitCounts(t *testing.T, att attachment, check func(counts) bool) counts {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var last counts
	for time.Now().Before(deadline) {
		c, err := att.Counts()
		if err != nil {
			t.Fatalf("Counts: %v", err)
		}
		last = c
		if check(c) {
			return c
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for expected counts; last = %+v", last)
	return last
}

func TestAttachTCX_CountsTraffic(t *testing.T) {
	requireBPF(t)
	ifindexA := createVethPair(t, "tuv0a", "tuv0b")

	att, err := tcxAttacher{}.Attach(ifindexA, "tuv0a", 1024, true)
	if err != nil {
		t.Fatalf("AttachTCX: %v", err)
	}

	// Ingress: a frame transmitted out the peer (tuv0b) is received on tuv0a.
	src := [4]byte{10, 1, 1, 1}
	dst := [4]byte{10, 1, 1, 2}
	srcAddr := netip.AddrFrom4(src)
	dstAddr := netip.AddrFrom4(dst)
	injectFrame(t, "tuv0b", ethIPv4(protoTCP, src, dst, tcpHdr(1234, 80)))
	c := waitCounts(t, att, func(c counts) bool {
		return c.ingressIP[srcAddr] > 0 && c.ingressPort[portProto{port: 80, proto: protoTCP}] > 0
	})
	if c.egressIP[dstAddr] != 0 {
		t.Error("ingress frame should not have touched the egress IP map")
	}

	// Egress: a frame transmitted out tuv0a traverses tuv0a's egress program.
	injectFrame(t, "tuv0a", ethIPv4(protoUDP, dst, src, udpHdr(5000, 53)))
	waitCounts(t, att, func(c counts) bool {
		return c.egressIP[srcAddr] > 0 && c.egressPort[portProto{port: 5000, proto: protoUDP}] > 0
	})

	// Stop detaches both links without error and leaves no TCX filter behind.
	if err := att.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestAttachTCX_CountsIPv6Traffic validates real TCX ingress/egress accounting
// for IPv6 TCP, UDP, and ICMPv6 frames on a veth pair, verifying unified
// netip.Addr readback and ip6_ingress / ip6_egress map entry counts (AC-5).
func TestAttachTCX_CountsIPv6Traffic(t *testing.T) {
	requireBPF(t)
	ifindexA := createVethPair(t, "tuv6a", "tuv6b")

	att, err := tcxAttacher{}.Attach(ifindexA, "tuv6a", 1024, true)
	if err != nil {
		t.Fatalf("AttachTCX: %v", err)
	}

	src6 := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 1, 0, 2, 0, 3, 0, 4, 0, 5, 0, 6}
	dst6 := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0xa, 0, 0xb, 0, 0xc, 0, 0xd, 0, 0xe, 0, 0xf}
	srcAddr := netip.AddrFrom16(src6)
	dstAddr := netip.AddrFrom16(dst6)

	tcpPkt := ethIPv6(protoTCP, src6, dst6, tcpHdr(1234, 443))
	icmpPkt := ethIPv6(protoICMPv6, src6, dst6, []byte{128, 0, 0, 0, 0, 1, 0, 1})
	udpPkt := ethIPv6(protoUDP, src6, dst6, udpHdr(5000, 53))

	// Ingress: TCP + ICMPv6 transmitted out peer tuv6b arrive on tuv6a ingress.
	injectFrame(t, "tuv6b", tcpPkt)
	injectFrame(t, "tuv6b", icmpPkt)
	// Egress: UDP transmitted out tuv6a traverses tuv6a egress.
	injectFrame(t, "tuv6a", udpPkt)

	wantInBytes := uint64(len(tcpPkt) + len(icmpPkt))
	c := waitCounts(t, att, func(c counts) bool {
		return c.ingressIP[srcAddr] == wantInBytes &&
			c.egressIP[dstAddr] == uint64(len(udpPkt)) &&
			c.ingressPort[portProto{port: 443, proto: protoTCP}] == uint64(len(tcpPkt)) &&
			c.ingressPort[portProto{port: 0, proto: protoICMPv6}] == uint64(len(icmpPkt)) &&
			c.egressPort[portProto{port: 5000, proto: protoUDP}] == uint64(len(udpPkt))
	})
	if c.mapEntries["ip6_ingress"] != 1 {
		t.Errorf("mapEntries[ip6_ingress] = %d, want 1", c.mapEntries["ip6_ingress"])
	}
	if c.mapEntries["ip6_egress"] != 1 {
		t.Errorf("mapEntries[ip6_egress] = %d, want 1", c.mapEntries["ip6_egress"])
	}
	if c.mapEntries["ip_ingress"] != 0 || c.mapEntries["ip_egress"] != 0 {
		t.Errorf("IPv4 map entries should be 0 for IPv6-only traffic, got in=%d out=%d",
			c.mapEntries["ip_ingress"], c.mapEntries["ip_egress"])
	}

	if err := att.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestMetricsScrape(t *testing.T) {
	requireBPF(t)
	reg := metrics.NewPrometheusRegistry()
	BindMetrics(reg)

	ifindexA := createVethPair(t, "tuv1a", "tuv1b")
	m := newMonitor(tcxAttacher{}, func(name string) (int, bool, bool) {
		if name == "tuv1a" {
			return ifindexA, true, true
		}
		return 0, false, false
	})
	if err := m.Reconcile(&Config{
		Enabled: true, Interval: time.Second,
		Interfaces: []InterfaceConfig{{Name: "tuv1a", TrackIP: true, StaleTimeout: 5 * time.Minute, MaxEntries: 1024}},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	defer m.Stop()

	src4 := [4]byte{172, 16, 0, 1}
	src6 := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	dst6 := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}
	injectFrame(t, "tuv1b", ethIPv4(protoTCP, src4, [4]byte{172, 16, 0, 2}, tcpHdr(1111, 443)))
	injectFrame(t, "tuv1b", ethIPv6(protoTCP, src6, dst6, tcpHdr(2222, 8443)))

	// Wait until both IPv4 and IPv6 frames land in the BPF maps, then publish.
	addr4 := netip.AddrFrom4(src4)
	addr6 := netip.AddrFrom16(src6)
	settleDeadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(settleDeadline) {
		snap := m.Snapshot()
		if len(snap) == 1 && snap[0].ingressIP[addr4] > 0 && snap[0].ingressIP[addr6] > 0 {
			m.mu.Lock()
			m.publishLocked(snap, time.Now())
			m.mu.Unlock()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// replaced the gated exporter Server.Start (and its start-error
	// check) with httptest.NewServer(reg.Handler()) to drop this test's
	// dependency on the compile-out-able ze_telemetry exporter package. Scraping
	// the always-on registry handler proves the same thing (traffic-usage metrics
	// reach a /metrics page); the metrics-present assertion below is unchanged.
	ts := httptest.NewServer(reg.Handler())
	defer ts.Close()

	var body string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL, http.NoBody)
		if reqErr != nil {
			t.Fatalf("new request: %v", reqErr)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			body = string(b)
			if strings.Contains(body, "ze_traffic_usage_") {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	for _, want := range []string{
		"ze_traffic_usage_ingress_port_bytes_total",
		`src_ip="172.16.0.1"`,
		`src_ip="2001:db8::1"`,
		`map="ip6_ingress"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics missing %q; got:\n%s", want, body)
		}
	}
}

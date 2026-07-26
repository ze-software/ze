// Design: plan/learned/1007-cp-survival-3-egress-cs6-sched.md -- CS6 classification integration test

//go:build integration && linux

package trafficnetlink

import (
	"context"
	"fmt"
	"net"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/traffic"
)

func setTOS(fd uintptr, tos int) error {
	return syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TOS, tos)
}

// VALIDATES: spec-cp-survival-3 AC-2 -- CS6-marked packets hit the control
// class counter (classification works after the translateFilter fix).
// PREVENTS: regression to the broken state where U32 had no Sel and matched nothing.
func TestCS6ClassifyNetns(t *testing.T) {
	withTrafficNetNS(t, func() {
		const ifaceName = "ze_cs0"
		link := addTrafficVeth(t, ifaceName, "ze_cs1")

		registerSnapshotStore(t)
		b := newBackendWithOps(netlinkOps{}, nil, "boot-1", nil)

		desired := map[string]traffic.InterfaceQoS{
			ifaceName: {
				Interface: ifaceName,
				Qdisc: traffic.Qdisc{
					Type:         traffic.QdiscHTB,
					DefaultClass: "default",
					Classes: []traffic.TrafficClass{
						{
							Name:     "control",
							Rate:     1_000_000,
							Ceil:     10_000_000,
							Priority: 0,
							Filters: []traffic.TrafficFilter{
								{Type: traffic.FilterDSCP, Value: 48}, // CS6
							},
						},
						{
							Name:     "default",
							Rate:     1_000_000,
							Ceil:     10_000_000,
							Priority: 1,
						},
					},
				},
			},
		}
		if err := b.Apply(context.Background(), desired); err != nil {
			t.Fatalf("Apply: %v", err)
		}

		if got := rootQdiscTypeInKernel(t, ifaceName); got != "htb" {
			t.Fatalf("root qdisc = %q, want htb", got)
		}

		filters, err := netlink.FilterList(link, netlink.HANDLE_ROOT)
		if err != nil {
			t.Fatalf("FilterList: %v", err)
		}

		var u32Count int
		for _, f := range filters {
			if _, ok := f.(*netlink.U32); ok {
				u32Count++
			}
		}
		if u32Count == 0 {
			// Report WHAT came back, not merely that no U32 did. "no u32 filters" is
			// equally consistent with three causes -- nothing was installed, the
			// filters hang off a parent this query does not cover, or the library
			// decoded them as some other type -- and the bare message cannot tell
			// them apart, which is why this failure survived several QEMU runs
			// undiagnosed. Also query the HTB root HANDLE, since that is the parent
			// applyInterface actually attaches to (backend_linux.go:148 passes
			// rootHandle, not HANDLE_ROOT).
			describe := func(fs []netlink.Filter) []string {
				out := make([]string, 0, len(fs))
				for _, f := range fs {
					a := f.Attrs()
					out = append(out, fmt.Sprintf("%T(kind=%s parent=%#x prio=%d proto=%#04x)",
						f, f.Type(), a.Parent, a.Priority, a.Protocol))
				}
				return out
			}
			byHandle, handleErr := netlink.FilterList(link, netlink.MakeHandle(1, 0))
			t.Fatalf("no u32 filters installed (the bug: translateFilter produced U32 with no Sel)\n"+
				"  parent=HANDLE_ROOT returned %d filter(s): %v\n"+
				"  parent=1:0        returned %d filter(s): %v (err=%v)",
				len(filters), describe(filters), len(byHandle), describe(byHandle), handleErr)
		}
		if u32Count < 2 {
			t.Errorf("u32 filter count = %d, want >= 2 (IPv4 + IPv6)", u32Count)
		}

		peerLink, err := netlink.LinkByName("ze_cs1")
		if err != nil {
			t.Fatalf("link ze_cs1: %v", err)
		}

		addr := &netlink.Addr{IPNet: &net.IPNet{
			IP:   net.IPv4(10, 99, 0, 1),
			Mask: net.CIDRMask(24, 32),
		}}
		if err := netlink.AddrAdd(link, addr); err != nil {
			t.Fatalf("addr add ze_cs0: %v", err)
		}
		peerAddr := &netlink.Addr{IPNet: &net.IPNet{
			IP:   net.IPv4(10, 99, 0, 2),
			Mask: net.CIDRMask(24, 32),
		}}
		if err := netlink.AddrAdd(peerLink, peerAddr); err != nil {
			t.Fatalf("addr add ze_cs1: %v", err)
		}

		conn, err := net.DialUDP("udp4", &net.UDPAddr{IP: net.IPv4(10, 99, 0, 1)}, &net.UDPAddr{IP: net.IPv4(10, 99, 0, 2), Port: 9999})
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer conn.Close()

		rawConn, err := conn.SyscallConn()
		if err != nil {
			t.Fatalf("SyscallConn: %v", err)
		}
		var setErr error
		rawConn.Control(func(fd uintptr) {
			setErr = setTOS(fd, 0xC0) // CS6 = DSCP 48, TOS = 0xC0
		})
		if setErr != nil {
			t.Fatalf("setsockopt IP_TOS: %v", setErr)
		}

		payload := []byte("cs6test")
		for i := 0; i < 50; i++ {
			conn.Write(payload) //nolint:errcheck // best-effort: some may fail (no listener)
		}

		classes, err := netlink.ClassList(link, netlink.HANDLE_ROOT)
		if err != nil {
			t.Fatalf("ClassList: %v", err)
		}

		var controlStats *netlink.ClassStatistics
		for _, cls := range classes {
			htb, ok := cls.(*netlink.HtbClass)
			if !ok {
				continue
			}
			if htb.Handle == makeHandle(1, 1) {
				controlStats = htb.Statistics
			}
		}
		if controlStats == nil {
			t.Fatal("control class (1:1) not found in kernel")
		}
		if controlStats.Basic.Packets == 0 {
			t.Error("control class packet count = 0; CS6-marked packets were not classified (AC-2 fail)")
		}
	})
}

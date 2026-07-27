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

		// Query the HTB root HANDLE (1:0), which is the parent applyInterface
		// attaches filters to (backend_linux.go:148 passes rootQdisc's Handle).
		// HANDLE_ROOT does NOT match them: it returned 0 filters while 1:0 returned
		// both, so the long-standing "no u32 filters installed" failure was this
		// query, not the backend. The assertions below are unchanged -- still
		// >= 2 U32 filters, still one per address family.
		filters, err := netlink.FilterList(link, netlink.MakeHandle(1, 0))
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
			// Report WHAT came back rather than only that no U32 did: "no u32 filters"
			// is equally consistent with nothing being installed, with the filters
			// hanging off a parent this query does not cover, and with the library
			// decoding them as another type. Telling those apart is what identified the
			// HANDLE_ROOT query above as the actual fault.
			got := make([]string, 0, len(filters))
			for _, f := range filters {
				a := f.Attrs()
				got = append(got, fmt.Sprintf("%T(kind=%s parent=%#x prio=%d proto=%#04x)",
					f, f.Type(), a.Parent, a.Priority, a.Protocol))
			}
			t.Fatalf("no u32 filters installed under parent 1:0 (the bug: translateFilter produced U32 with no Sel)\n  returned %d filter(s): %v", len(filters), got)
		}
		if u32Count < 2 {
			t.Errorf("u32 filter count = %d, want >= 2 (IPv4 + IPv6)", u32Count)
		}

		addr := &netlink.Addr{IPNet: &net.IPNet{
			IP:   net.IPv4(10, 99, 0, 1),
			Mask: net.CIDRMask(24, 32),
		}}
		if err := netlink.AddrAdd(link, addr); err != nil {
			t.Fatalf("addr add ze_cs0: %v", err)
		}
		// The peer end goes into a namespace of its own. Addressing it here would
		// make 10.99.0.2 a LOCAL address, and Linux routes to a local address over
		// loopback -- the packets would never egress ze_cs0 and its qdisc would
		// count nothing, which is exactly what this test used to assert against
		// (root qdisc packets=0).
		movePeerToNetNS(t, "ze_cs1", &netlink.Addr{IPNet: &net.IPNet{
			IP:   net.IPv4(10, 99, 0, 2),
			Mask: net.CIDRMask(24, 32),
		}})

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

		// Same parent correction as the filter query above: applyInterface adds
		// classes under the HTB root HANDLE (backend_linux.go:129 passes
		// rootHandle), and HANDLE_ROOT does not return them -- which is why this
		// reported "control class (1:1) not found" while the class was there.
		classes, err := netlink.ClassList(link, netlink.MakeHandle(1, 0))
		if err != nil {
			t.Fatalf("ClassList: %v", err)
		}

		var controlStats *netlink.ClassStatistics
		for _, cls := range classes {
			htb, ok := cls.(*netlink.HtbClass)
			if !ok {
				continue
			}
			if htb.Handle == tcHandle(1) {
				controlStats = htb.Statistics
			}
		}
		if controlStats == nil {
			got := make([]string, 0, len(classes))
			for _, cls := range classes {
				got = append(got, fmt.Sprintf("%T(handle=%#x parent=%#x)", cls, cls.Attrs().Handle, cls.Attrs().Parent))
			}
			t.Fatalf("control class (1:1, handle %#x) not found in kernel under parent 1:0\n  returned %d class(es): %v",
				tcHandle(1), len(classes), got)
		}
		if controlStats.Basic.Packets == 0 {
			// Two very different causes produce a zero count, and the bare message
			// cannot tell them apart:
			//   (a) no traffic traversed the qdisc at all -- both veth ends live in
			//       ONE namespace here, so 10.99.0.2 is a LOCAL address and Linux
			//       routes 10.99.0.1 -> 10.99.0.2 over loopback, never egressing
			//       ze_cs0; or
			//   (b) traffic did traverse it and the u32 filter failed to match, which
			//       is the classification defect this test exists to catch.
			// The root qdisc's own counters separate them: zero there means nothing
			// reached the qdisc and the topology is at fault, non-zero means the
			// packets arrived and were classified into the wrong class.
			var rootPkts, defaultPkts uint64
			if qdiscs, qErr := netlink.QdiscList(link); qErr == nil {
				for _, q := range qdiscs {
					if q.Attrs().Parent == netlink.HANDLE_ROOT {
						if st := q.Attrs().Statistics; st != nil {
							rootPkts = uint64(st.Basic.Packets)
						}
					}
				}
			}
			for _, cls := range classes {
				htb, ok := cls.(*netlink.HtbClass)
				if !ok || htb.Statistics == nil {
					continue
				}
				if htb.Handle == tcHandle(2) {
					defaultPkts = uint64(htb.Statistics.Basic.Packets)
				}
			}
			t.Errorf("control class packet count = 0; CS6-marked packets were not classified (AC-2 fail)\n"+
				"  root qdisc packets=%d (0 => no traffic reached the qdisc: both veth ends share this netns, so the destination is a LOCAL address and the flow goes over loopback)\n"+
				"  default class (1:2) packets=%d (non-zero with root non-zero => traffic arrived but the u32 DSCP filter did not match)",
				rootPkts, defaultPkts)
		}
	})
}

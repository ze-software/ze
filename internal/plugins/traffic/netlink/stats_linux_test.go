// Design: docs/architecture/core-design.md -- kernel traffic control qdisc and class statistics readback tests

//go:build linux

package trafficnetlink

import (
	"context"
	"errors"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/traffic"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/test/userns"
)

func testClassStats(bytes uint64, pkts, qlen, backlog, drops, requeues, overlimits uint32) *netlink.ClassStatistics {
	return &netlink.ClassStatistics{
		Basic: &netlink.GnetStatsBasic{
			Bytes:   bytes,
			Packets: pkts,
		},
		Queue: &netlink.GnetStatsQueue{
			Qlen:       qlen,
			Backlog:    backlog,
			Drops:      drops,
			Requeues:   requeues,
			Overlimits: overlimits,
		},
	}
}

func rootQdiscAttrs(linkIdx int, handle uint32, stats *netlink.ClassStatistics) netlink.QdiscAttrs {
	return netlink.QdiscAttrs{
		LinkIndex:  linkIdx,
		Handle:     handle,
		Parent:     netlink.HANDLE_ROOT,
		Statistics: (*netlink.QdiscStatistics)(stats),
	}
}

func htbClassAttrs(linkIdx int, handle uint32, stats *netlink.ClassStatistics) netlink.ClassAttrs {
	return netlink.ClassAttrs{
		LinkIndex:  linkIdx,
		Handle:     handle,
		Parent:     tcHandle(0),
		Statistics: stats,
	}
}

func assertQdiscStatsEqual(t *testing.T, label string, got *traffic.QdiscStats, want traffic.QdiscStats) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: Stats = nil, want %+v", label, want)
	}
	if *got != want {
		t.Fatalf("%s: Stats = %+v, want %+v", label, *got, want)
	}
}

// VALIDATES: spec-traffic-qdisc-stats AC-1, AC-2, and HTB readback boundary
// guards -- ListQdiscs and ListAllQdiscs preserve raw kernel qdisc kinds
// (never falling back to htb), extract all 7 live qdisc/class stats fields,
// map Ze class names with handle fallback for non-Ze classes, enforce HTB
// rate/ceil overflow refusal, use one LinkList + one QdiscList(nil) bulk dump,
// skip ClassList for classless roots, and tolerate vanished ENOENT/ENODEV links.
// PREVENTS: misreporting noqueue/pfifo_fast/mq as htb, silent uint64*8 wrap on
// HTB readback, nil-statistics zero fabrication, and N+1 ClassList overhead.
func TestNetlinkListQdiscsStatsClassesAndKind(t *testing.T) {
	t.Run("root-qdiscs-kind-handle-and-stats", func(t *testing.T) {
		ops := newFakeTCOps()
		ops.links["eth0"] = testLink("eth0", 2)
		ops.links["lo"] = testLink("lo", 1)
		ops.links["eth1"] = testLink("eth1", 3)
		ops.links["eth2"] = testLink("eth2", 4)
		ops.links["eth3"] = testLink("eth3", 5)

		fqStats := testClassStats(123456, 789, 3, 4500, 12, 4, 9)
		pfifoStats := testClassStats(2048, 16, 1, 128, 2, 0, 1)
		mqStats := testClassStats(9000, 90, 0, 0, 0, 1, 0)
		htbStats := testClassStats(500000, 400, 5, 6000, 7, 2, 11)

		ops.qdiscs["eth0"] = []netlink.Qdisc{&netlink.FqCodel{
			QdiscAttrs: rootQdiscAttrs(2, 0, fqStats),
		}}
		ops.qdiscs["lo"] = []netlink.Qdisc{&netlink.GenericQdisc{
			QdiscAttrs: rootQdiscAttrs(1, 0, nil),
			QdiscType:  qdiscTypeNoqueue,
		}}
		ops.qdiscs["eth1"] = []netlink.Qdisc{&netlink.PfifoFast{
			QdiscAttrs: rootQdiscAttrs(3, 0, pfifoStats),
		}}
		ops.qdiscs["eth2"] = []netlink.Qdisc{&netlink.GenericQdisc{
			QdiscAttrs: rootQdiscAttrs(4, netlink.MakeHandle(0xFFFF, 0), mqStats),
			QdiscType:  "mq",
		}}
		ops.qdiscs["eth3"] = []netlink.Qdisc{&netlink.Htb{
			QdiscAttrs: rootQdiscAttrs(5, tcHandle(0), htbStats),
			Defcls:     2,
		}}

		b := newBackendWithOps(ops, nil, "boot-1", nil)

		fq, err := b.ListQdiscs("eth0")
		if err != nil {
			t.Fatalf("ListQdiscs(eth0): %v", err)
		}
		if fq.Qdisc.Type != traffic.QdiscFQCodel || fq.Qdisc.Kind != "fq_codel" || fq.Qdisc.Handle != "0:0" {
			t.Fatalf("eth0 qdisc = %+v, want fq_codel handle 0:0", fq.Qdisc)
		}
		assertQdiscStatsEqual(t, "eth0 fq_codel", fq.Qdisc.Stats, traffic.QdiscStats{
			Bytes: 123456, Packets: 789, Qlen: 3, Backlog: 4500, Drops: 12, Requeues: 4, Overlimits: 9,
		})

		lo, err := b.ListQdiscs("lo")
		if err != nil {
			t.Fatalf("ListQdiscs(lo): %v", err)
		}
		if lo.Qdisc.Type != 0 || lo.Qdisc.Kind != qdiscTypeNoqueue || lo.Qdisc.KindName() != qdiscTypeNoqueue {
			t.Fatalf("lo qdisc = %+v, want Type=0 Kind=noqueue (never htb)", lo.Qdisc)
		}
		if lo.Qdisc.Stats != nil {
			t.Fatalf("lo Stats = %+v, want nil when netlink Statistics is nil", lo.Qdisc.Stats)
		}

		pfifo, err := b.ListQdiscs("eth1")
		if err != nil {
			t.Fatalf("ListQdiscs(eth1): %v", err)
		}
		if pfifo.Qdisc.Type != 0 || pfifo.Qdisc.Kind != "pfifo_fast" || pfifo.Qdisc.KindName() != "pfifo_fast" {
			t.Fatalf("eth1 qdisc = %+v, want Type=0 Kind=pfifo_fast (never htb)", pfifo.Qdisc)
		}
		assertQdiscStatsEqual(t, "eth1 pfifo_fast", pfifo.Qdisc.Stats, traffic.QdiscStats{
			Bytes: 2048, Packets: 16, Qlen: 1, Backlog: 128, Drops: 2, Requeues: 0, Overlimits: 1,
		})

		mq, err := b.ListQdiscs("eth2")
		if err != nil {
			t.Fatalf("ListQdiscs(eth2): %v", err)
		}
		if mq.Qdisc.Type != 0 || mq.Qdisc.Kind != "mq" || mq.Qdisc.Handle != "ffff:0" {
			t.Fatalf("eth2 qdisc = %+v, want Type=0 Kind=mq Handle=ffff:0", mq.Qdisc)
		}

		htb, err := b.ListQdiscs("eth3")
		if err != nil {
			t.Fatalf("ListQdiscs(eth3): %v", err)
		}
		if htb.Qdisc.Type != traffic.QdiscHTB || htb.Qdisc.Kind != qdiscTypeHTB || htb.Qdisc.Handle != "1:0" {
			t.Fatalf("eth3 qdisc = %+v, want htb handle 1:0", htb.Qdisc)
		}
		if htb.Qdisc.DefaultClass != "1:2" {
			t.Fatalf("eth3 DefaultClass = %q, want 1:2", htb.Qdisc.DefaultClass)
		}
		assertQdiscStatsEqual(t, "eth3 htb", htb.Qdisc.Stats, traffic.QdiscStats{
			Bytes: 500000, Packets: 400, Qlen: 5, Backlog: 6000, Drops: 7, Requeues: 2, Overlimits: 11,
		})
	})

	t.Run("htb-apply-and-class-readback", func(t *testing.T) {
		ops := newFakeTCOps()
		ops.links["eth0"] = testLink("eth0", 5)
		ops.qdiscs["eth0"] = []netlink.Qdisc{originalFQ(5)}
		b := testBackend(t, ops)

		desired := map[string]traffic.InterfaceQoS{
			"eth0": {
				Interface: "eth0",
				Qdisc: traffic.Qdisc{
					Type:         traffic.QdiscHTB,
					DefaultClass: "default",
					Classes: []traffic.TrafficClass{
						{Name: "control", Rate: 10_000_000, Ceil: 50_000_000, Priority: 1},
						{Name: "default", Rate: 90_000_000, Ceil: 100_000_000, Priority: 2},
					},
				},
			},
		}
		if err := b.Apply(context.Background(), desired); err != nil {
			t.Fatalf("Apply: %v", err)
		}

		// Attach live kernel stats to root and installed classes, plus an
		// out-of-order non-Ze class at 1:99 to prove handle sorting and
		// handle fallback naming.
		ops.qdiscs["eth0"][0].Attrs().Statistics = (*netlink.QdiscStatistics)(testClassStats(90000, 120, 2, 2400, 1, 0, 3))
		ops.classes["eth0"][0].Attrs().Statistics = testClassStats(30000, 40, 1, 1000, 0, 0, 1)
		ops.classes["eth0"][1].Attrs().Statistics = testClassStats(60000, 80, 1, 1400, 1, 0, 2)
		foreignClass := &netlink.HtbClass{
			ClassAttrs: htbClassAttrs(5, tcHandle(0x99), testClassStats(512, 4, 0, 0, 0, 0, 0)),
			Rate:       125_000, // 1 Mbps in bytes/s
			Ceil:       250_000, // 2 Mbps in bytes/s
			Prio:       7,
		}
		ops.classes["eth0"] = append([]netlink.Class{foreignClass}, ops.classes["eth0"]...)

		got, err := b.ListQdiscs("eth0")
		if err != nil {
			t.Fatalf("ListQdiscs: %v", err)
		}
		if got.Qdisc.Kind != qdiscTypeHTB || got.Qdisc.Handle != "1:0" || got.Qdisc.DefaultClass != "default" {
			t.Fatalf("qdisc = %+v, want htb 1:0 default-class=default", got.Qdisc)
		}
		assertQdiscStatsEqual(t, "root htb", got.Qdisc.Stats, traffic.QdiscStats{
			Bytes: 90000, Packets: 120, Qlen: 2, Backlog: 2400, Drops: 1, Requeues: 0, Overlimits: 3,
		})
		if len(got.Qdisc.Classes) != 3 {
			t.Fatalf("class count = %d, want 3: %+v", len(got.Qdisc.Classes), got.Qdisc.Classes)
		}

		c0, c1, c2 := got.Qdisc.Classes[0], got.Qdisc.Classes[1], got.Qdisc.Classes[2]
		if c0.Name != "control" || c0.Handle != "1:1" || c0.Rate != 10_000_000 || c0.Ceil != 50_000_000 || c0.Priority != 1 {
			t.Fatalf("class[0] = %+v, want control 1:1 10M/50M prio 1", c0)
		}
		assertQdiscStatsEqual(t, "class control", c0.Stats, traffic.QdiscStats{
			Bytes: 30000, Packets: 40, Qlen: 1, Backlog: 1000, Drops: 0, Requeues: 0, Overlimits: 1,
		})
		if c1.Name != "default" || c1.Handle != "1:2" || c1.Rate != 90_000_000 || c1.Ceil != 100_000_000 || c1.Priority != 2 {
			t.Fatalf("class[1] = %+v, want default 1:2 90M/100M prio 2", c1)
		}
		assertQdiscStatsEqual(t, "class default", c1.Stats, traffic.QdiscStats{
			Bytes: 60000, Packets: 80, Qlen: 1, Backlog: 1400, Drops: 1, Requeues: 0, Overlimits: 2,
		})
		if c2.Name != "1:99" || c2.Handle != "1:99" || c2.Rate != 1_000_000 || c2.Ceil != 2_000_000 || c2.Priority != 7 {
			t.Fatalf("class[2] = %+v, want non-Ze handle fallback 1:99 1M/2M prio 7", c2)
		}

		// Read-only reader (no Ze metadata) falls back to formatted handles.
		ro, err := (readOnlyReader{ops: ops}).ListQdiscs("eth0")
		if err != nil {
			t.Fatalf("readOnlyReader.ListQdiscs: %v", err)
		}
		if ro.Qdisc.DefaultClass != "1:2" {
			t.Fatalf("readOnlyReader DefaultClass = %q, want 1:2", ro.Qdisc.DefaultClass)
		}
		if ro.Qdisc.Classes[0].Name != "1:1" || ro.Qdisc.Classes[1].Name != "1:2" {
			t.Fatalf("readOnlyReader class names = [%q, %q], want [1:1, 1:2]", ro.Qdisc.Classes[0].Name, ro.Qdisc.Classes[1].Name)
		}

		// RestoreOriginal clears Ze class metadata.
		if err := b.RestoreOriginal(context.Background(), "eth0"); err != nil {
			t.Fatalf("RestoreOriginal: %v", err)
		}
		if len(b.classNames) != 0 || len(b.defaultClasses) != 0 {
			t.Fatalf("metadata after RestoreOriginal: classNames=%v defaultClasses=%v, want empty", b.classNames, b.defaultClasses)
		}
	})

	t.Run("htb-class-rate-ceil-boundary-and-overflow", func(t *testing.T) {
		ops := newFakeTCOps()
		ops.links["eth0"] = testLink("eth0", 5)
		ops.qdiscs["eth0"] = []netlink.Qdisc{&netlink.Htb{
			QdiscAttrs: rootQdiscAttrs(5, tcHandle(0), nil),
		}}
		b := newBackendWithOps(ops, nil, "boot-1", nil)

		const maxValidBytesPerSec uint64 = math.MaxUint64 / 8
		cls := &netlink.HtbClass{
			ClassAttrs: htbClassAttrs(5, tcHandle(1), nil),
			Rate:       maxValidBytesPerSec,
			Ceil:       maxValidBytesPerSec,
			Prio:       300, // clamped to MaxUint8
		}
		ops.classes["eth0"] = []netlink.Class{cls}

		got, err := b.ListQdiscs("eth0")
		if err != nil {
			t.Fatalf("last valid HTB rate/ceil refused: %v", err)
		}
		if got.Qdisc.Classes[0].Rate != maxValidBytesPerSec*8 || got.Qdisc.Classes[0].Ceil != maxValidBytesPerSec*8 {
			t.Fatalf("boundary rate/ceil = %d/%d, want %d", got.Qdisc.Classes[0].Rate, got.Qdisc.Classes[0].Ceil, maxValidBytesPerSec*8)
		}
		if got.Qdisc.Classes[0].Priority != math.MaxUint8 {
			t.Fatalf("clamped priority = %d, want %d", got.Qdisc.Classes[0].Priority, math.MaxUint8)
		}

		cls.Rate = maxValidBytesPerSec + 1
		if _, err := b.ListQdiscs("eth0"); err == nil {
			t.Fatal("HTB class rate overflow was accepted")
		}

		cls.Rate = maxValidBytesPerSec
		cls.Ceil = maxValidBytesPerSec + 1
		if _, err := b.ListQdiscs("eth0"); err == nil {
			t.Fatal("HTB class ceil overflow was accepted")
		}
	})

	t.Run("list-all-bulk-dump-classless-skip-and-vanished-links", func(t *testing.T) {
		ops := newFakeTCOps()
		ops.links["lo"] = testLink("lo", 1)
		ops.links["eth0"] = testLink("eth0", 2)
		ops.links["eth1"] = testLink("eth1", 3)
		ops.links["veth_enoent"] = testLink("veth_enoent", 4)
		ops.links["veth_enodev"] = testLink("veth_enodev", 5)

		ops.qdiscs["lo"] = []netlink.Qdisc{&netlink.GenericQdisc{
			QdiscAttrs: rootQdiscAttrs(1, 0, nil),
			QdiscType:  qdiscTypeNoqueue,
		}}
		ops.qdiscs["eth0"] = []netlink.Qdisc{&netlink.FqCodel{
			QdiscAttrs: rootQdiscAttrs(2, 0, testClassStats(1000, 10, 0, 0, 1, 0, 0)),
		}}
		ops.qdiscs["eth1"] = []netlink.Qdisc{&netlink.Htb{
			QdiscAttrs: rootQdiscAttrs(3, tcHandle(0), testClassStats(5000, 50, 2, 1500, 0, 0, 2)),
			Defcls:     1,
		}}
		ops.classes["eth1"] = []netlink.Class{&netlink.HtbClass{
			ClassAttrs: htbClassAttrs(3, tcHandle(1), testClassStats(5000, 50, 2, 1500, 0, 0, 2)),
			Rate:       125_000,
			Ceil:       125_000,
		}}
		ops.qdiscs["veth_enoent"] = []netlink.Qdisc{&netlink.Htb{
			QdiscAttrs: rootQdiscAttrs(4, tcHandle(0), nil),
		}}
		ops.qdiscs["veth_enodev"] = []netlink.Qdisc{&netlink.Htb{
			QdiscAttrs: rootQdiscAttrs(5, tcHandle(0), nil),
		}}
		ops.classListErr["veth_enoent"] = unix.ENOENT
		ops.classListErr["veth_enodev"] = unix.ENODEV

		reader := readOnlyReader{ops: ops}
		all, err := reader.ListAllQdiscs()
		if err != nil {
			t.Fatalf("ListAllQdiscs: %v", err)
		}

		// Verify 1 linkList + 1 bulk qdiscList(nil) and 0 classList calls for classless roots.
		var linkLists, bulkQdiscs, perLinkQdiscs int
		for _, call := range ops.calls {
			switch call {
			case "linkList":
				linkLists++
			case "qdiscList:all":
				bulkQdiscs++
			case "classList:lo", "classList:eth0":
				t.Fatalf("ListAllQdiscs issued wasted %q for classless root; calls=%v", call, ops.calls)
			default:
				if len(call) > len("qdiscList:") && call[:len("qdiscList:")] == "qdiscList:" {
					perLinkQdiscs++
				}
			}
		}
		if linkLists != 1 || bulkQdiscs != 1 || perLinkQdiscs != 0 {
			t.Fatalf("dump counts: linkList=%d qdiscList(nil)=%d perLinkQdiscList=%d; calls=%v",
				linkLists, bulkQdiscs, perLinkQdiscs, ops.calls)
		}

		// Verify sorted surviving links and omitted vanished links.
		if len(all) != 3 {
			t.Fatalf("ListAllQdiscs len = %d, want 3 (eth0, eth1, lo): %+v", len(all), all)
		}
		if all[0].Interface != "eth0" || all[1].Interface != "eth1" || all[2].Interface != "lo" {
			t.Fatalf("interfaces = [%s %s %s], want [eth0 eth1 lo]", all[0].Interface, all[1].Interface, all[2].Interface)
		}
		if all[0].Qdisc.Kind != "fq_codel" || all[0].Qdisc.Stats == nil {
			t.Fatalf("eth0 = %+v, want fq_codel with stats", all[0].Qdisc)
		}
		if all[1].Qdisc.Kind != qdiscTypeHTB || len(all[1].Qdisc.Classes) != 1 || all[1].Qdisc.Classes[0].Stats == nil {
			t.Fatalf("eth1 = %+v, want htb with 1 class and stats", all[1].Qdisc)
		}
		if all[2].Qdisc.Kind != qdiscTypeNoqueue || all[2].Qdisc.Stats != nil {
			t.Fatalf("lo = %+v, want noqueue with nil stats", all[2].Qdisc)
		}

		// Non-vanished netlink errors MUST propagate rather than being skipped.
		ops.classListErr["eth1"] = unix.EPERM
		if _, err := reader.ListAllQdiscs(); !errors.Is(err, unix.EPERM) {
			t.Fatalf("ListAllQdiscs with EPERM err = %v, want wrapped EPERM", err)
		}
	})
}

// VALIDATES: spec-traffic-qdisc-stats AC-7 (and Assumption A-1) -- real Linux
// kernel netlink QdiscList and ClassList populate live non-zero Bytes and
// Packets counters on both the HTB root qdisc and matched control class when
// real UDP traffic traverses a veth pair, and ListAllQdiscs returns both
// shaped and default-qdisc interfaces.
// PREVENTS: a netlink decoding mismatch or silent statistics omission that
// passes fakeTCOps unit tests while reporting nil or zero on a real kernel.
func TestNetlinkIntegration_QdiscAndClassLiveStats(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withTrafficNetNS(t, func() {
		const (
			shapedIface   = "ze_st0"
			shapedPeer    = "ze_st1"
			unshapedIface = "ze_st2"
			unshapedPeer  = "ze_st3"
		)
		link := addTrafficVeth(t, shapedIface, shapedPeer)
		_ = addTrafficVeth(t, unshapedIface, unshapedPeer)

		registerSnapshotStore(t)
		b := newBackendWithOps(netlinkOps{}, nil, "boot-1", nil)
		desired := map[string]traffic.InterfaceQoS{
			shapedIface: {
				Interface: shapedIface,
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

		if err := netlink.AddrAdd(link, &netlink.Addr{IPNet: &net.IPNet{
			IP:   net.IPv4(10, 98, 0, 1),
			Mask: net.CIDRMask(24, 32),
		}}); err != nil {
			t.Fatalf("addr add %s: %v", shapedIface, err)
		}
		movePeerToNetNS(t, shapedPeer, &netlink.Addr{IPNet: &net.IPNet{
			IP:   net.IPv4(10, 98, 0, 2),
			Mask: net.CIDRMask(24, 32),
		}})

		conn, err := net.DialUDP("udp4",
			&net.UDPAddr{IP: net.IPv4(10, 98, 0, 1)},
			&net.UDPAddr{IP: net.IPv4(10, 98, 0, 2), Port: 9998},
		)
		if err != nil {
			t.Fatalf("dial udp: %v", err)
		}
		defer conn.Close() //nolint:errcheck // best-effort cleanup

		rawConn, err := conn.SyscallConn()
		if err != nil {
			t.Fatalf("SyscallConn: %v", err)
		}
		var setErr error
		if err := rawConn.Control(func(fd uintptr) {
			setErr = setTOS(fd, 0xC0) // CS6 = DSCP 48
		}); err != nil {
			t.Fatalf("rawConn.Control: %v", err)
		}
		if setErr != nil {
			t.Fatalf("setsockopt IP_TOS: %v", setErr)
		}

		payload := []byte("qdisc-stats-probe-payload")
		for range 20 {
			conn.Write(payload) //nolint:errcheck // best-effort: peer has no UDP listener
		}

		qos, err := b.ListQdiscs(shapedIface)
		if err != nil {
			t.Fatalf("ListQdiscs(%s): %v", shapedIface, err)
		}
		if qos.Qdisc.Kind != qdiscTypeHTB || qos.Qdisc.Type != traffic.QdiscHTB || qos.Qdisc.Handle != "1:0" {
			t.Fatalf("root qdisc = %+v, want htb handle 1:0", qos.Qdisc)
		}
		if qos.Qdisc.DefaultClass != "default" {
			t.Fatalf("DefaultClass = %q, want default", qos.Qdisc.DefaultClass)
		}
		if qos.Qdisc.Stats == nil || qos.Qdisc.Stats.Bytes == 0 || qos.Qdisc.Stats.Packets == 0 {
			t.Fatalf("root qdisc Stats = %+v, want non-nil with Bytes > 0 and Packets > 0", qos.Qdisc.Stats)
		}

		var foundControl, foundDefault bool
		for _, cls := range qos.Qdisc.Classes {
			switch cls.Name {
			case "control":
				foundControl = true
				if cls.Handle != "1:1" || cls.Rate != 1_000_000 || cls.Ceil != 10_000_000 {
					t.Fatalf("control class = %+v, want handle 1:1 rate 1M ceil 10M", cls)
				}
				if cls.Stats == nil || cls.Stats.Bytes == 0 || cls.Stats.Packets == 0 {
					t.Fatalf("control class Stats = %+v, want non-nil with Bytes > 0 and Packets > 0", cls.Stats)
				}
			case "default":
				foundDefault = true
				if cls.Handle != "1:2" || cls.Stats == nil {
					t.Fatalf("default class = %+v, want handle 1:2 with non-nil Stats", cls)
				}
			}
		}
		if !foundControl || !foundDefault {
			t.Fatalf("classes = %+v, want both control and default", qos.Qdisc.Classes)
		}

		// Verify ListAllQdiscs on both active backend and stateless read-only reader.
		all, err := b.ListAllQdiscs()
		if err != nil {
			t.Fatalf("b.ListAllQdiscs: %v", err)
		}
		var sawShaped, sawUnshaped bool
		for _, entry := range all {
			switch entry.Interface {
			case shapedIface:
				sawShaped = true
				if entry.Qdisc.Stats == nil || entry.Qdisc.Stats.Bytes == 0 || entry.Qdisc.Stats.Packets == 0 {
					t.Fatalf("ListAllQdiscs %s Stats = %+v, want non-zero", shapedIface, entry.Qdisc.Stats)
				}
			case unshapedIface:
				sawUnshaped = true
				if entry.Qdisc.Kind == "" {
					t.Fatalf("ListAllQdiscs %s Kind is empty, want real kernel qdisc name", unshapedIface)
				}
				if entry.Qdisc.Kind == qdiscTypeNoqueue && entry.Qdisc.Type != 0 {
					t.Fatalf("ListAllQdiscs %s noqueue Type = %v, want 0 (never htb)", unshapedIface, entry.Qdisc.Type)
				}
			}
		}
		if !sawShaped || !sawUnshaped {
			t.Fatalf("ListAllQdiscs saw shaped=%v unshaped=%v, want both true", sawShaped, sawUnshaped)
		}

		roAll, err := (readOnlyReader{ops: netlinkOps{}}).ListAllQdiscs()
		if err != nil {
			t.Fatalf("readOnlyReader.ListAllQdiscs: %v", err)
		}
		var sawROShaped bool
		for _, entry := range roAll {
			if entry.Interface != shapedIface {
				continue
			}
			sawROShaped = true
			if entry.Qdisc.Stats == nil || entry.Qdisc.Stats.Packets == 0 {
				t.Fatalf("readOnlyReader %s Stats = %+v, want non-zero", shapedIface, entry.Qdisc.Stats)
			}
			if len(entry.Qdisc.Classes) == 0 || entry.Qdisc.Classes[0].Name != "1:1" {
				t.Fatalf("readOnlyReader %s classes = %+v, want handle fallback 1:1", shapedIface, entry.Qdisc.Classes)
			}
		}
		if !sawROShaped {
			t.Fatalf("readOnlyReader.ListAllQdiscs did not return %s", shapedIface)
		}

		// Prove end-to-end Prometheus /metrics push collection against the real
		// kernel HTB qdisc and named classes (control/default) on ze_st0.
		restoreBackends := traffic.ResetBackendsForTest()
		defer restoreBackends()
		if err := traffic.RegisterBackend(backendName, func() (traffic.Backend, error) { return b, nil }); err != nil {
			t.Fatalf("RegisterBackend(%s): %v", backendName, err)
		}
		if err := traffic.LoadBackend(backendName); err != nil {
			t.Fatalf("LoadBackend(%s): %v", backendName, err)
		}

		reg := metrics.NewPrometheusRegistry()
		// StartPushCollectors runs the initial collect synchronously on this
		// netns-locked thread; stop immediately so the 1s worker goroutine
		// cannot race after withTrafficNetNS exits.
		stopPush := metrics.StartPushCollectors(reg, nil)
		stopPush()

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", http.NoBody)
		reg.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /metrics status = %d, want %d", rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		assertMetricSample(t, body, `ze_traffic_qdisc_bytes_total{handle="1:0",interface="ze_st0",qdisc="htb"}`, true)
		assertMetricSample(t, body, `ze_traffic_qdisc_packets_total{handle="1:0",interface="ze_st0",qdisc="htb"}`, true)
		assertMetricSample(t, body, `ze_traffic_class_bytes_total{class="control",handle="1:1",interface="ze_st0",qdisc="htb"}`, true)
		assertMetricSample(t, body, `ze_traffic_class_packets_total{class="control",handle="1:1",interface="ze_st0",qdisc="htb"}`, true)
		assertMetricSample(t, body, `ze_traffic_class_bytes_total{class="default",handle="1:2",interface="ze_st0",qdisc="htb"}`, false)
	})
}

func assertMetricSample(t *testing.T, body, series string, requirePositive bool) {
	t.Helper()
	prefix := series + " "
	for line := range strings.SplitSeq(body, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		valStr := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			t.Fatalf("parse metric %s value %q: %v", series, valStr, err)
		}
		if requirePositive && val <= 0 {
			t.Fatalf("metric %s = %v, want > 0", series, val)
		}
		return
	}
	t.Fatalf("metrics output missing %s\nfull body:\n%s", series, body)
}

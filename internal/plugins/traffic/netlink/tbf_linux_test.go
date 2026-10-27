// Design: docs/guide/traffic-control.md -- classless root rate semantics

//go:build linux

package trafficnetlink

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/traffic"
)

func TestTBFRateNormalization(t *testing.T) {
	for _, rate := range []uint64{8_000, 10_000_000, 20_000_000, uint64(math.MaxUint32) * 8} {
		qdisc, err := translateQdisc(traffic.Qdisc{Type: traffic.QdiscTBF, RateBps: rate}, 7)
		if err != nil {
			t.Fatalf("rate %d: %v", rate, err)
		}
		tbf := qdisc.(*netlink.Tbf)
		if tbf.Rate != rate/8 {
			t.Fatalf("rate %d: kernel rate = %d bytes/s", rate, tbf.Rate)
		}
		if uint64(tbf.Limit) != max(rate/8, 2048) {
			t.Fatalf("rate %d: queue limit = %d bytes", rate, tbf.Limit)
		}
		// Decode time, not the producer's byte-count formula: Buffer is the
		// scheduler-clock duration of a 125ms bucket (or one full packet).
		seconds := float64(tbf.Buffer) / netlink.TickInUsec() / 1_000_000
		wantSeconds := 0.125
		if rate == 8_000 {
			wantSeconds = 2.048 // 2048-byte packet floor at 1000 bytes/s
		}
		if math.Abs(seconds-wantSeconds) > 0.000002 {
			t.Fatalf("rate %d: buffer duration = %g seconds, want %g", rate, seconds, wantSeconds)
		}
	}
}

func TestHTBRateNormalization(t *testing.T) {
	class, err := translateClass(traffic.QdiscHTB, traffic.TrafficClass{Rate: 10_000_000, Ceil: 20_000_000}, 7, tcHandle(0), 1)
	if err != nil {
		t.Fatal(err)
	}
	htb := class.(*netlink.HtbClass)
	if htb.Rate != 1_250_000 || htb.Ceil != 2_500_000 {
		t.Fatalf("HTB rate/ceil = %d/%d bytes/s, want 1250000/2500000", htb.Rate, htb.Ceil)
	}
	if htb.Buffer == 0 || htb.Cbuffer == 0 {
		t.Fatal("HTB has no token burst")
	}
}

func TestTBFApplyReachesBothDirectionsAndReadback(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["ppp0"] = testLink("ppp0", 7)
	ops.qdiscs["ppp0"] = []netlink.Qdisc{originalFQ(7)}
	b := testBackend(t, ops)
	for _, rates := range [][2]uint64{{10_000_000, 2_000_000}, {20_000_000, 5_000_000}} {
		qos := traffic.InterfaceQoS{
			Interface: "ppp0",
			Qdisc:     traffic.Qdisc{Type: traffic.QdiscTBF, RateBps: rates[0]},
			Ingress:   traffic.NewPolicer(rates[1]),
		}
		if err := b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"ppp0": qos}); err != nil {
			t.Fatal(err)
		}
		root := ops.replaced[len(ops.replaced)-1].(*netlink.Tbf)
		if root.Rate != rates[0]/8 {
			t.Fatalf("installed download = %d bytes/s, want %d", root.Rate, rates[0]/8)
		}
		if len(ops.added) == 0 {
			t.Fatal("classless root prevented ingress policing")
		}
		filter := ops.added[len(ops.added)-1].(*netlink.MatchAll)
		police := filter.Actions[0].(*netlink.PoliceAction)
		if uint64(police.Rate) != rates[1]/8 || filter.Parent != netlink.HANDLE_MIN_INGRESS || police.ExceedAction != netlink.TC_POLICE_SHOT {
			t.Fatalf("installed upload filter = %+v, action = %+v", filter, police)
		}
		readback, err := b.ListQdiscs("ppp0")
		if err != nil {
			t.Fatal(err)
		}
		if readback.Qdisc.RateBps != rates[0] || len(readback.Qdisc.Classes) != 0 {
			t.Fatalf("readback = %+v", readback.Qdisc)
		}
	}
	for _, call := range ops.calls {
		if strings.HasPrefix(call, "classAdd:") {
			t.Fatalf("class operation for a classless root: %s", call)
		}
	}
}

func TestTBFInvalidInputLeavesKernelUntouched(t *testing.T) {
	cases := []struct {
		name    string
		qdisc   traffic.Qdisc
		ingress traffic.Policer
	}{
		{name: "missing", qdisc: traffic.Qdisc{Type: traffic.QdiscTBF}},
		{name: "sub-byte", qdisc: traffic.Qdisc{Type: traffic.QdiscTBF, RateBps: 7}},
		{name: "fractional-byte", qdisc: traffic.Qdisc{Type: traffic.QdiscTBF, RateBps: 10_000_001}},
		{name: "queue-overflow", qdisc: traffic.Qdisc{Type: traffic.QdiscTBF, RateBps: (uint64(math.MaxUint32) + 1) * 8}},
		{name: "maximum-integer", qdisc: traffic.Qdisc{Type: traffic.QdiscTBF, RateBps: math.MaxUint64}},
		{name: "fake-class", qdisc: traffic.Qdisc{Type: traffic.QdiscTBF, RateBps: 10_000_000, Classes: []traffic.TrafficClass{{Name: "default", Rate: 10_000_000}}}},
		{name: "default-class", qdisc: traffic.Qdisc{Type: traffic.QdiscTBF, RateBps: 10_000_000, DefaultClass: "default"}},
		{name: "other-qdisc", qdisc: traffic.Qdisc{Type: traffic.QdiscHTB, RateBps: 10_000_000}},
		{name: "upload-fractional-byte", qdisc: traffic.Qdisc{Type: traffic.QdiscTBF, RateBps: 10_000_000}, ingress: traffic.NewPolicer(9)},
		{name: "upload-overflow", qdisc: traffic.Qdisc{Type: traffic.QdiscTBF, RateBps: 10_000_000}, ingress: traffic.NewPolicer(policerRateBpsMax + 8)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ops := newFakeTCOps()
			b := newBackendWithOps(ops, nil, "boot-1", nil)
			desired := map[string]traffic.InterfaceQoS{"ppp0": {Qdisc: c.qdisc, Ingress: c.ingress}}
			if err := traffic.RunVerifier("tc", desired); err == nil {
				t.Fatal("config verifier accepted invalid input")
			}
			if err := b.Apply(context.Background(), desired); err == nil {
				t.Fatal("Apply accepted invalid input")
			}
			if len(ops.calls) != 0 {
				t.Fatalf("invalid configuration reached kernel operations: %v", ops.calls)
			}
		})
	}
}

func TestTBFReadbackUsesKernelUnits(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["ppp0"] = testLink("ppp0", 7)
	ops.qdiscs["ppp0"] = []netlink.Qdisc{&netlink.Tbf{QdiscAttrs: rootAttrs(7), Rate: 1_250_000}}
	b := newBackendWithOps(ops, nil, "boot-1", nil)
	qos, err := b.ListQdiscs("ppp0")
	if err != nil || qos.Qdisc.RateBps != 10_000_000 {
		t.Fatalf("readback = %+v, error = %v", qos, err)
	}
	ops.qdiscs["ppp0"][0].(*netlink.Tbf).Rate = math.MaxUint64/8 + 1
	if _, err := b.ListQdiscs("ppp0"); err == nil {
		t.Fatal("readback overflow accepted")
	}
}

func TestRateAndBurstNarrowingRefused(t *testing.T) {
	for _, class := range []traffic.TrafficClass{
		{Rate: 0},
		{Rate: 7},
		{Rate: 10_000_000, Ceil: 10_000_001},
		{Rate: 10_000_000, Ceil: 8_000_000},
		{Rate: math.MaxUint64 - 7},
	} {
		if _, err := translateClass(traffic.QdiscHTB, class, 7, tcHandle(0), 1); err == nil {
			t.Fatalf("invalid HTB class accepted: %+v", class)
		}
	}
	if _, err := burstTicks(1, math.MaxUint32); err == nil {
		t.Fatal("microsecond overflow accepted")
	}
	if _, err := burstTicks(1_000_000, 0); err == nil {
		t.Fatal("zero-tick burst accepted")
	}
}

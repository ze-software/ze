// VALIDATES: the netdev collector parses /proc/net/dev and emits per-interface
// bandwidth (bytes→kilobits/s) and packet rates, plus a system.net aggregate
// that excludes loopback, across two collects.
// PREVENTS: a broken rate calculation, wrong bits conversion, or loopback
// leaking into the system-wide network aggregate.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func netDev(eth, lo string) string {
	return "Inter-|   Receive                                                |  Transmit\n" +
		" face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed\n" +
		"  eth0: " + eth + "\n" +
		"    lo: " + lo + "\n"
}

func TestNetDevCollect(t *testing.T) {
	// rx: bytes pkts errs drop fifo frame compressed multicast | tx: bytes pkts ...
	first := netDev("1000 10 0 0 0 0 0 0 2000 20 0 0 0 0 0 0", "500 5 0 0 0 0 0 0 500 5 0 0 0 0 0 0")
	// eth0 +8000 rx bytes / +80 rx pkts, +16000 tx bytes / +160 tx pkts; lo unchanged
	second := netDev("9000 90 0 0 0 0 0 0 18000 180 0 0 0 0 0 0", "500 5 0 0 0 0 0 0 500 5 0 0 0 0 0 0")

	fs, dir := procDir(t, map[string]string{"net/dev": first})
	reg := newRecordingRegistry()
	c := newNetDevCollector(fs, time.Second)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	writeProcFile(t, dir, "net/dev", second)
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}

	const bw = "netdata_net_net_kilobits_persec_average"
	// 8000 bytes * 8 / 1000 = 64 kbit/s; 16000 → 128
	if got := reg.gauge(t, bw, "net.eth0", "received", "eth0"); got != 64 {
		t.Errorf("eth0 bw received = %v, want 64", got)
	}
	if got := reg.gauge(t, bw, "net.eth0", "sent", "eth0"); got != 128 {
		t.Errorf("eth0 bw sent = %v, want 128", got)
	}
	const pkts = "netdata_net_packets_packets_persec_average"
	if got := reg.gauge(t, pkts, "net.eth0", "received", "eth0"); got != 80 {
		t.Errorf("eth0 packets received = %v, want 80", got)
	}

	// system.net aggregate excludes lo; only eth0 contributes.
	const sys = "netdata_system_net_kilobits_persec_average"
	if got := reg.gauge(t, sys, "system.net", "received", "network"); got != 64 {
		t.Errorf("system.net received = %v, want 64", got)
	}
	if got := reg.gauge(t, sys, "system.net", "sent", "network"); got != 128 {
		t.Errorf("system.net sent = %v, want 128", got)
	}
}

// VALIDATES: the ipvs collector parses /proc/net/ip_vs_stats (hex) and emits
// connections/packets/bandwidth as per-second rates across two collects.
// PREVENTS: a broken hex parse or delta on the ipvs.net metrics, and a wrong
// bytes-to-kilobits conversion.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func ipvsStats(conns, inPkts, outPkts, inBytes, outBytes string) string {
	return "   Total Incoming Outgoing         Incoming         Outgoing\n" +
		"   Conns  Packets  Packets            Bytes            Bytes\n" +
		" " + conns + " " + inPkts + " " + outPkts + " " + inBytes + " " + outBytes + "\n"
}

func TestIPVSCollect(t *testing.T) {
	// hex: conns 0, in 0, out 0, inBytes 0, outBytes 0
	first := ipvsStats("0", "0", "0", "0", "0")
	// hex: conns 0x0a=10, in 0x14=20, out 0x1e=30, inBytes 0x64=100, outBytes 0xc8=200
	second := ipvsStats("0A", "14", "1E", "64", "C8")

	fs, dir := procDir(t, map[string]string{"net/ip_vs_stats": first})
	reg := newRecordingRegistry()
	c := newIPVSCollector(fs, time.Second)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	writeProcFile(t, dir, "net/ip_vs_stats", second)
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}

	if got := reg.gauge(t, "netdata_ipvs_net_connections_persec_average", "ipvs.net", "connections", "ipvs"); got != 10 {
		t.Errorf("connections = %v, want 10", got)
	}
	const pkts = "netdata_ipvs_net_packets_persec_average"
	if got := reg.gauge(t, pkts, "ipvs.net", "received", "ipvs"); got != 20 {
		t.Errorf("packets received = %v, want 20", got)
	}
	if got := reg.gauge(t, pkts, "ipvs.net", "sent", "ipvs"); got != 30 {
		t.Errorf("packets sent = %v, want 30", got)
	}
	// bandwidth: bytes * 8 / 1000 / 1s. 100 bytes → 0.8 kbit/s, 200 → 1.6.
	const bw = "netdata_ipvs_net_kilobits_persec_average"
	if got := reg.gauge(t, bw, "ipvs.net", "received", "ipvs"); got != 100.0*8/1000 {
		t.Errorf("bw received = %v, want 0.8", got)
	}
	if got := reg.gauge(t, bw, "ipvs.net", "sent", "ipvs"); got != 200.0*8/1000 {
		t.Errorf("bw sent = %v, want 1.6", got)
	}
}

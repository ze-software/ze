// VALIDATES: the snmp6 collector parses /proc/self/net/snmp6 and emits IPv6
// packet rates and system bandwidth (octet deltas → kilobits/s) across two
// collects.
// PREVENTS: a broken snmp6 field parse, delta, or bytes-to-kilobits conversion on
// the ipv6.* / system.ipv6 metrics.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func snmp6File(inRecv, outReq, inOctets string) string {
	return "Ip6InReceives " + inRecv + "\n" +
		"Ip6OutRequests " + outReq + "\n" +
		"Ip6InOctets " + inOctets + "\n"
}

func TestSNMP6Collect(t *testing.T) {
	first := snmp6File("0", "0", "0")
	// deltas: InReceives 100, OutRequests 50, InOctets 8000 (→ 64 kbit/s).
	second := snmp6File("100", "50", "8000")

	fs, dir := procDirSelf(t, map[string]string{"net/snmp6": first})
	reg := newRecordingRegistry()
	c := newSNMP6Collector(fs, time.Second)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	writeProcFile(t, dir, "net/snmp6", second)
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}

	const pkts = "netdata_ipv6_packets_packets_persec_average"
	if got := reg.gauge(t, pkts, "ipv6.packets", "received", "packets"); got != 100 {
		t.Errorf("ipv6 received = %v, want 100", got)
	}
	if got := reg.gauge(t, pkts, "ipv6.packets", "sent", "packets"); got != 50 {
		t.Errorf("ipv6 sent = %v, want 50", got)
	}
	// 8000 bytes * 8 / 1000 = 64 kbit/s.
	if got := reg.gauge(t, "netdata_system_ipv6_kilobits_persec_average", "system.ipv6", "received", "network"); got != 64 {
		t.Errorf("system.ipv6 received = %v, want 64", got)
	}
}

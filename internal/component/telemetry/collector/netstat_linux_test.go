// VALIDATES: the netstat collector parses /proc/self/net/netstat and emits
// IPv4 bandwidth (octet deltas → kilobits/s), multicast packet rates, and TCP
// out-of-order counters across two collects.
// PREVENTS: a broken name/value pairing or delta on the system.ipv4 / ipv4.* /
// ip.tcpofo metrics.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func netstatFile(ipExt, tcpExt string) string {
	return "IpExt: InOctets OutOctets InMcastPkts\n" +
		"IpExt: " + ipExt + "\n" +
		"TcpExt: TCPOFOQueue\n" +
		"TcpExt: " + tcpExt + "\n"
}

func TestNetstatCollect(t *testing.T) {
	first := netstatFile("0 0 0", "0")
	// deltas: InOctets 8000 (→64), OutOctets 4000 (→32), InMcastPkts 15,
	// TCPOFOQueue 25.
	second := netstatFile("8000 4000 15", "25")

	fs, dir := procDirSelf(t, map[string]string{"net/netstat": first})
	reg := newRecordingRegistry()
	c := newNetstatCollector(fs, time.Second)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	writeProcFile(t, dir, "net/netstat", second)
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}

	const sys = "netdata_system_ipv4_kilobits_persec_average"
	if got := reg.gauge(t, sys, "system.ipv4", "received", "network"); got != 64 {
		t.Errorf("system.ipv4 received = %v, want 64", got)
	}
	if got := reg.gauge(t, sys, "system.ipv4", "sent", "network"); got != 32 {
		t.Errorf("system.ipv4 sent = %v, want 32", got)
	}
	if got := reg.gauge(t, "netdata_ipv4_mcastpkts_packets_persec_average", "ipv4.mcastpkts", "received", "multicast"); got != 15 {
		t.Errorf("mcast pkts received = %v, want 15", got)
	}
	if got := reg.gauge(t, "netdata_ip_tcpofo_packets_persec_average", "ip.tcpofo", "TCPOFOQueue", "tcp"); got != 25 {
		t.Errorf("TCPOFOQueue = %v, want 25", got)
	}
}

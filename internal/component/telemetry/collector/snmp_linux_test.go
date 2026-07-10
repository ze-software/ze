// VALIDATES: the snmp collector parses /proc/self/net/snmp and emits IPv4/TCP/
// UDP/ICMP packet rates (deltas) plus the absolute TCP CurrEstab connection
// count across two collects.
// PREVENTS: a broken name/value pairing, delta, or the absolute-vs-rate handling
// of CurrEstab on the ipv4.* metrics.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func snmpFile(ip, tcp, udp, icmp string) string {
	return "Ip: InReceives OutRequests\n" +
		"Ip: " + ip + "\n" +
		"Tcp: CurrEstab InSegs OutSegs\n" +
		"Tcp: " + tcp + "\n" +
		"Udp: InDatagrams OutDatagrams\n" +
		"Udp: " + udp + "\n" +
		"Icmp: InMsgs OutMsgs\n" +
		"Icmp: " + icmp + "\n"
}

func TestSNMPCollect(t *testing.T) {
	first := snmpFile("0 0", "5 0 0", "0 0", "0 0")
	// deltas: Ip InReceives 100 / OutRequests 50; Tcp InSegs 200; Udp In 300;
	// Icmp InMsgs 10; CurrEstab absolute 7.
	second := snmpFile("100 50", "7 200 150", "300 100", "10 5")

	fs, dir := procDirSelf(t, map[string]string{"net/snmp": first})
	reg := newRecordingRegistry()
	c := newSNMPCollector(fs, time.Second)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	writeProcFile(t, dir, "net/snmp", second)
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}

	// CurrEstab is absolute, set on every collect.
	if got := reg.gauge(t, "netdata_ipv4_tcpsock_active_connections_average", "ipv4.tcpsock", "connections", "tcp"); got != 7 {
		t.Errorf("CurrEstab = %v, want 7", got)
	}
	const pkts = "netdata_ipv4_packets_packets_persec_average"
	if got := reg.gauge(t, pkts, "ipv4.packets", "received", "packets"); got != 100 {
		t.Errorf("ip received = %v, want 100", got)
	}
	if got := reg.gauge(t, pkts, "ipv4.packets", "sent", "packets"); got != 50 {
		t.Errorf("ip sent = %v, want 50", got)
	}
	if got := reg.gauge(t, "netdata_ipv4_tcppackets_packets_persec_average", "ipv4.tcppackets", "received", "tcp"); got != 200 {
		t.Errorf("tcp received = %v, want 200", got)
	}
	if got := reg.gauge(t, "netdata_ipv4_udppackets_packets_persec_average", "ipv4.udppackets", "received", "udp"); got != 300 {
		t.Errorf("udp received = %v, want 300", got)
	}
	if got := reg.gauge(t, "netdata_ipv4_icmp_packets_persec_average", "ipv4.icmp", "InMsgs", "icmp"); got != 10 {
		t.Errorf("icmp InMsgs = %v, want 10", got)
	}
}

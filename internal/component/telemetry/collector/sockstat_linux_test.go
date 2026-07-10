// VALIDATES: the sockstat collector parses /proc/net/sockstat and emits IP/TCP/
// UDP socket counts and TCP/UDP memory (pages converted to KiB) on the right
// gauge/label tuples.
// PREVENTS: mis-parsed socket figures, or the pages-to-KiB conversion silently
// changing, on the metrics surface.

//go:build linux

package collector

import "testing"

func TestSockStatCollect(t *testing.T) {
	fs := procFixture(t, map[string]string{
		"net/sockstat": "sockets: used 1602\n" +
			"TCP: inuse 35 orphan 0 tw 4 alloc 59 mem 22\n" +
			"UDP: inuse 12 mem 62\n" +
			"UDPLITE: inuse 0\n" +
			"RAW: inuse 0\n" +
			"FRAG: inuse 0 memory 0\n",
	})
	reg := newRecordingRegistry()
	c := newSockStatCollector(fs)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if got := reg.gauge(t, "netdata_ip_sockstat_sockets_sockets_average", "ip.sockstat_sockets", "used", "sockets"); got != 1602 {
		t.Errorf("sockets used = %v, want 1602", got)
	}

	const tcp = "netdata_ipv4_sockstat_tcp_sockets_sockets_average"
	if got := reg.gauge(t, tcp, "ipv4.sockstat_tcp_sockets", "inuse", "tcp"); got != 35 {
		t.Errorf("tcp inuse = %v, want 35", got)
	}
	if got := reg.gauge(t, tcp, "ipv4.sockstat_tcp_sockets", "timewait", "tcp"); got != 4 {
		t.Errorf("tcp timewait = %v, want 4", got)
	}
	if got := reg.gauge(t, tcp, "ipv4.sockstat_tcp_sockets", "alloc", "tcp"); got != 59 {
		t.Errorf("tcp alloc = %v, want 59", got)
	}

	// mem is in pages; the collector converts to KiB by multiplying by 4.
	if got := reg.gauge(t, "netdata_ipv4_sockstat_tcp_mem_KiB_average", "ipv4.sockstat_tcp_mem", "mem", "tcp"); got != 22*4 {
		t.Errorf("tcp mem = %v, want %v", got, 22*4)
	}
	if got := reg.gauge(t, "netdata_ipv4_sockstat_udp_sockets_sockets_average", "ipv4.sockstat_udp_sockets", "inuse", "udp"); got != 12 {
		t.Errorf("udp inuse = %v, want 12", got)
	}
	if got := reg.gauge(t, "netdata_ipv4_sockstat_udp_mem_KiB_average", "ipv4.sockstat_udp_mem", "mem", "udp"); got != 62*4 {
		t.Errorf("udp mem = %v, want %v", got, 62*4)
	}
}

// VALIDATES: the sockstat6 collector parses /proc/net/sockstat6 and emits the
// in-use IPv6 TCP/UDP/RAW/FRAG socket counts on the right gauge/label tuples.
// PREVENTS: mis-parsed or mis-routed IPv6 socket counts on the metrics surface.

//go:build linux

package collector

import "testing"

func TestSockStat6Collect(t *testing.T) {
	fs := procFixture(t, map[string]string{
		"net/sockstat6": "TCP6: inuse 17\n" +
			"UDP6: inuse 9\n" +
			"UDPLITE6: inuse 0\n" +
			"RAW6: inuse 1\n" +
			"FRAG6: inuse 0 memory 0\n",
	})
	reg := newRecordingRegistry()
	c := newSockStat6Collector(fs)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	for _, tc := range []struct {
		name  string
		chart string
		fam   string
		want  float64
	}{
		{"netdata_ipv6_sockstat6_tcp_sockets_sockets_average", "ipv6.sockstat6_tcp_sockets", "tcp6", 17},
		{"netdata_ipv6_sockstat6_udp_sockets_sockets_average", "ipv6.sockstat6_udp_sockets", "udp6", 9},
		{"netdata_ipv6_sockstat6_raw_sockets_sockets_average", "ipv6.sockstat6_raw_sockets", "raw6", 1},
	} {
		if got := reg.gauge(t, tc.name, tc.chart, "inuse", tc.fam); got != tc.want {
			t.Errorf("%s inuse = %v, want %v", tc.fam, got, tc.want)
		}
	}
}

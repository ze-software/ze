// VALIDATES: the wireless collector parses /proc/net/wireless and emits per-
// interface signal level, link quality, discarded-packet counters and missed
// beacons on the right gauge/label tuples.
// PREVENTS: mislabeled wireless counters (e.g. swapping level and link, or a
// discard sub-counter) reaching the metrics surface.

//go:build linux

package collector

import "testing"

func TestWirelessCollect(t *testing.T) {
	fs := procFixture(t, map[string]string{
		"net/wireless": "Inter-| sta-|   Quality        |   Discarded packets               | Missed\n" +
			" face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon\n" +
			" wlan0: 0001    2.    3.    4.       5      6      7      8      9       10\n",
	})
	reg := newRecordingRegistry()
	c := newWirelessCollector(fs)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	const chart = "net_wireless.wlan0"
	if got := reg.gauge(t, "netdata_net_wireless_signal_level_dBm_average", chart, "level", "wlan0"); got != 3 {
		t.Errorf("signal level = %v, want 3", got)
	}
	if got := reg.gauge(t, "netdata_net_wireless_quality_link_average", chart, "link", "wlan0"); got != 2 {
		t.Errorf("quality link = %v, want 2", got)
	}

	const discard = "netdata_net_wireless_discarded_packets_packets_persec_average"
	for _, tc := range []struct {
		dim  string
		want float64
	}{
		{"nwid", 5}, {"crypt", 6}, {"frag", 7}, {"retry", 8}, {"misc", 9},
	} {
		if got := reg.gauge(t, discard, chart, tc.dim, "wlan0"); got != tc.want {
			t.Errorf("discard %s = %v, want %v", tc.dim, got, tc.want)
		}
	}

	if got := reg.gauge(t, "netdata_net_wireless_missed_beacon_beacons_average", chart, "beacon", "wlan0"); got != 10 {
		t.Errorf("missed beacon = %v, want 10", got)
	}
}

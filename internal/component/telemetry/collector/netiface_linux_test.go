// VALIDATES: the netiface collector walks a /sys/class/net tree and emits speed
// (Mbit→kbit), MTU, carrier, duplex and operstate gauges per interface, skipping
// loopback (root injected via the c.root seam).
// PREVENTS: a wrong unit on speed, or mis-mapped duplex/operstate one-hot gauges.

//go:build linux

package collector

import "testing"

func TestNetIfaceCollect(t *testing.T) {
	dir := t.TempDir()
	for rel, content := range map[string]string{
		"eth0/speed":     "1000\n",
		"eth0/mtu":       "1500\n",
		"eth0/carrier":   "1\n",
		"eth0/duplex":    "full\n",
		"eth0/operstate": "up\n",
		"lo/mtu":         "65536\n", // must be skipped
	} {
		writeProcFile(t, dir, rel, content)
	}

	c := newNetIfaceCollector()
	c.root = dir
	reg := newRecordingRegistry()
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	const chart = "net.eth0"
	// speed reported in kbit/s: 1000 Mbit * 1000.
	if got := reg.gauge(t, "netdata_net_speed_kilobits_persec_average", chart, "speed", "eth0"); got != 1_000_000 {
		t.Errorf("speed = %v, want 1000000", got)
	}
	if got := reg.gauge(t, "netdata_net_mtu_octets_average", chart, "mtu", "eth0"); got != 1500 {
		t.Errorf("mtu = %v, want 1500", got)
	}
	if got := reg.gauge(t, "netdata_net_carrier_state_average", chart, "carrier", "eth0"); got != 1 {
		t.Errorf("carrier = %v, want 1", got)
	}
	if got := reg.gauge(t, "netdata_net_duplex_state_average", chart, "full", "eth0"); got != 1 {
		t.Errorf("duplex full = %v, want 1", got)
	}
	if got := reg.gauge(t, "netdata_net_operstate_state_average", chart, "up", "eth0"); got != 1 {
		t.Errorf("operstate up = %v, want 1", got)
	}

	// lo must be skipped: its chart is never recorded.
	if _, ok := reg.gaugeOK("netdata_net_mtu_octets_average", "net.lo", "mtu", "lo"); ok {
		t.Error("loopback should be skipped")
	}
}

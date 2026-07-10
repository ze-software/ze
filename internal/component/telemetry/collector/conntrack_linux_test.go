// VALIDATES: the conntrack collector emits the absolute active/max connection
// gauges (from the injected nf_conntrack_count / _max sysctl paths) and per-
// second new/inserted rates parsed from /proc/net/stat/nf_conntrack across two
// collects.
// PREVENTS: a wrong column index, hex parse, or delta on the netfilter.conntrack
// metrics, and regressions in the count/max sysctl reads.

//go:build linux

package collector

import (
	"path/filepath"
	"testing"
	"time"
)

func TestConntrackCollect(t *testing.T) {
	const header = "entries searched found new invalid ignore delete delete_list insert insert_failed drop early_drop x y z search_restart\n"
	// 17 hex columns per CPU row; col 3 = New, col 8 = Insert.
	first := header + "00000010 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0\n"
	// New 0x0a=10, Insert 0x05=5.
	second := header + "00000010 0 0 0a 0 0 0 0 05 0 0 0 0 0 0 0 0\n"

	fs, dir := procDir(t, map[string]string{
		"net/stat/nf_conntrack": first,
		"nf_conntrack_count":    "42\n",
		"nf_conntrack_max":      "65536\n",
	})
	c := newConntrackCollector(fs, time.Second)
	c.countPath = filepath.Join(dir, "nf_conntrack_count")
	c.maxPath = filepath.Join(dir, "nf_conntrack_max")
	reg := newRecordingRegistry()
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}

	// Absolute gauges are set on every collect.
	if got := reg.gauge(t, "netdata_netfilter_conntrack_sockets_active_connections_average", "netfilter.conntrack_sockets", "connections", "conntrack"); got != 42 {
		t.Errorf("sockets = %v, want 42", got)
	}
	if got := reg.gauge(t, "netdata_netfilter_conntrack_sockets_max_connections_average", "netfilter.conntrack_sockets", "max", "conntrack"); got != 65536 {
		t.Errorf("max = %v, want 65536", got)
	}

	writeProcFile(t, dir, "net/stat/nf_conntrack", second)
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}
	if got := reg.gauge(t, "netdata_netfilter_conntrack_new_connections_persec_average", "netfilter.conntrack_new", "new", "conntrack"); got != 10 {
		t.Errorf("new = %v, want 10", got)
	}
	if got := reg.gauge(t, "netdata_netfilter_conntrack_changes_changes_persec_average", "netfilter.conntrack_changes", "inserted", "conntrack"); got != 5 {
		t.Errorf("inserted = %v, want 5", got)
	}
}

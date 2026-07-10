// VALIDATES: the conntrack_expect collector parses the per-CPU hex rows of
// /proc/net/stat/nf_conntrack (columns 13/14/15), sums them, and emits new/
// created/deleted expectation rates across two collects (path injected via seam).
// PREVENTS: a wrong column index, hex parse, or delta on the
// netfilter.conntrack_expect metric.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func TestConntrackExpectCollect(t *testing.T) {
	const header = "entries clashres found new invalid ignore delete delete_list insert insert_failed drop early_drop icmp_error expect_new expect_create expect_delete\n"
	// Two CPU rows; cols 13/14/15 (0-indexed) are the expect fields (hex).
	first := header +
		"00000010 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0\n" +
		"00000010 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0\n"
	// per-CPU expect: new 5+5=10, create 10+10=20, delete 15+15=30 (hex 05/0a/0f)
	second := header +
		"00000010 0 0 0 0 0 0 0 0 0 0 0 0 05 0a 0f\n" +
		"00000010 0 0 0 0 0 0 0 0 0 0 0 0 05 0a 0f\n"

	dir, path := tmpFile(t, "nf_conntrack", first)
	c := newConntrackExpectCollector(time.Second)
	c.path = path
	reg := newRecordingRegistry()
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	writeProcFile(t, dir, "nf_conntrack", second)
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}

	const name = "netdata_netfilter_conntrack_expect_expectations_persec_average"
	if got := reg.gauge(t, name, "netfilter.conntrack_expect", "new", "conntrack"); got != 10 {
		t.Errorf("new = %v, want 10", got)
	}
	if got := reg.gauge(t, name, "netfilter.conntrack_expect", "created", "conntrack"); got != 20 {
		t.Errorf("created = %v, want 20", got)
	}
	if got := reg.gauge(t, name, "netfilter.conntrack_expect", "deleted", "conntrack"); got != 30 {
		t.Errorf("deleted = %v, want 30", got)
	}
}

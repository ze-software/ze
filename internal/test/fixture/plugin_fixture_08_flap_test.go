package fixture

import "testing"

// flapMetricsSample08 is one scrape of the two counters this fixture reads,
// shaped as the daemon exposes them during a config commit. The link worker
// labels a block with the interface name carried on the queue entry it was
// about to handle, and the carrier resync the rate tracker pushes every second
// carries none, so its block lands on the empty label.
const flapMetricsSample08 = `# HELP ze_iface_link_worker_blocked_total link worker waits for a config commit
# TYPE ze_iface_link_worker_blocked_total counter
ze_iface_link_worker_blocked_total{name=""} 4
ze_iface_link_worker_blocked_total{name="zeflapd0"} 3
ze_iface_link_worker_blocked_total{name="zeflapv0"} 0
# HELP ze_iface_link_events_coalesced_total link events folded before the worker took them
# TYPE ze_iface_link_events_coalesced_total counter
ze_iface_link_events_coalesced_total{name="zeflapv0"} 97
`

// VALIDATES: totalCounter08 sums every series of a counter whatever its labels,
// which is what makes the flap test able to see that the worker waited.
//
// PREVENTS: the regression this function was written for. The fixture used to
// read ze_iface_link_worker_blocked_total{name="zeflapv0"} and that series
// stays at zero through a real overlap: during a commit the worker blocks on
// the carrier resync, which carries no interface name, and the burst coalesces
// behind it and finds the lock free. Reading the named series reported "0 of 3
// wanted rounds overlapped a commit" four times on a daemon that was holding
// the lock exactly as the test intended.
func TestTotalCounter08SumsEverySeriesOfACounter(t *testing.T) {
	// Three series, two of them non-zero and NEITHER of them the one the
	// fixture used to read. The values differ so that a reader summing only
	// the first series answers 4 and fails here: a sample whose later series
	// were zero would pass against that mutant and prove nothing.
	if got := totalCounter08(flapMetricsSample08, "ze_iface_link_worker_blocked_total"); got != 7 {
		t.Errorf("totalCounter08 over every series = %v, want 7", got)
	}
	// The named reader is what the fixture used to use, kept here as the
	// contrast: over the same scrape it answers zero, which is the blindness.
	if got := namedCounter08(flapMetricsSample08, "ze_iface_link_worker_blocked_total", flapDevice08); got != 0 {
		t.Errorf("namedCounter08 for %s = %v, want 0: the sample pins the blind case", flapDevice08, got)
	}
}

// VALIDATES: totalCounter08 selects by metric name and reads an unlabelled
// series, so a counter that gains or loses a label does not silently read zero.
func TestTotalCounter08SelectsTheMetricAndReadsBareSeries(t *testing.T) {
	if got := totalCounter08(flapMetricsSample08, "ze_iface_link_events_coalesced_total"); got != 97 {
		t.Errorf("totalCounter08 over the coalesced counter = %v, want 97", got)
	}
	if got := totalCounter08(flapMetricsSample08, "ze_iface_carrier_resyncs_total"); got != 0 {
		t.Errorf("totalCounter08 over an absent counter = %v, want 0", got)
	}
	const bare = "ze_iface_link_worker_blocked_total 7\n"
	if got := totalCounter08(bare, "ze_iface_link_worker_blocked_total"); got != 7 {
		t.Errorf("totalCounter08 over a series with no labels = %v, want 7", got)
	}
	// A different counter whose name merely starts the same must not be summed
	// in. This one is caught by the mandatory space after the labels, not by an
	// anchor, so it does NOT test the anchors and the two cases below do.
	const sibling = "ze_iface_link_worker_blocked_total_extra{name=\"x\"} 5\n"
	if got := totalCounter08(sibling, "ze_iface_link_worker_blocked_total"); got != 0 {
		t.Errorf("totalCounter08 matched a longer metric name, got %v, want 0", got)
	}
	// The leading anchor. A metric whose name ENDS with the target is a
	// different series and must not be summed in. Dropping `^` from the pattern
	// makes this read 5, and nothing else in either test notices.
	const prefixed = "ze_vpp_ze_iface_link_worker_blocked_total{name=\"x\"} 5\n"
	if got := totalCounter08(prefixed, "ze_iface_link_worker_blocked_total"); got != 0 {
		t.Errorf("totalCounter08 matched a metric ending in the target name, got %v, want 0", got)
	}
	// The trailing anchor. Prometheus may append a millisecond timestamp after
	// the value; the value must not be read out of a line whose shape the
	// pattern does not fully describe. Dropping `$` makes this read 5.
	const timestamped = "ze_iface_link_worker_blocked_total{name=\"x\"} 5 1700000000000\n"
	if got := totalCounter08(timestamped, "ze_iface_link_worker_blocked_total"); got != 0 {
		t.Errorf("totalCounter08 read a value off a timestamped series, got %v, want 0", got)
	}
}

// flapNetlinkTable08 is /proc/net/netlink as a 6.x kernel prints it, with two
// sockets that dropped notifications and one that did not.
const flapNetlinkTable08 = `sk               Eth Pid        Groups   Rmem     Wmem     Dump  Locks    Drops    Inode
0000000000000000 0   1          00000550 0        0        0     2        0        12345
0000000000000000 0   4242       00000001 0        0        0     2        1054     12346
0000000000000000 0   4243       00000001 0        0        0     2        207      12347
`

// VALIDATES: sumNetlinkDrops08 sums the Drops column over every socket, and
// refuses a table it cannot read rather than answering a number.
//
// PREVENTS: the vacuous zero-drops assertion. The reader used to return -1 for
// a table with no Drops column and to skip a row it could not parse; the
// fixture subtracts a baseline read the same way, so the run saw zero drops
// whatever the kernel had discarded.
func TestSumNetlinkDrops08ReadsTheDropsColumnOrRefuses(t *testing.T) {
	got, err := sumNetlinkDrops08(flapNetlinkTable08)
	if err != nil {
		t.Fatalf("sumNetlinkDrops08 over a well-formed table: %v", err)
	}
	if got != 1261 {
		t.Errorf("sumNetlinkDrops08 = %d, want 1261 (0 + 1054 + 207)", got)
	}
	refused := map[string]string{
		"no Drops column": "sk Eth Pid Groups Rmem Wmem Dump Locks Inode\n0 0 1 550 0 0 0 2 12345\n",
		"short row":       "sk Eth Pid Groups Rmem Wmem Dump Locks Drops Inode\n0 0 1 550 0 0 0 2\n",
		"non-numeric":     "sk Eth Pid Groups Rmem Wmem Dump Locks Drops Inode\n0 0 1 550 0 0 0 2 x 12345\n",
		"empty file":      "",
	}
	for name, table := range refused {
		if value, err := sumNetlinkDrops08(table); err == nil {
			t.Errorf("%s: sumNetlinkDrops08 = %d with no error, want a refusal", name, value)
		}
	}
}

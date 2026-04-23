// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type conntrackCollector struct {
	fs       procfs.FS
	interval time.Duration

	sockets metrics.GaugeVec
	newConn metrics.GaugeVec
	changes metrics.GaugeVec
	errors  metrics.GaugeVec
	search  metrics.GaugeVec

	prev  conntrackTotals
	first bool
}

type conntrackTotals struct {
	new, ignore, invalid           uint64
	insert, delete_, deleteList    uint64
	insertFailed, drop, earlyDrop  uint64
	searched, searchRestart, found uint64
}

func sumConntrack(entries []procfs.ConntrackStatEntry) conntrackTotals {
	var t conntrackTotals
	for _, e := range entries {
		t.new += e.New
		t.ignore += e.Ignore
		t.invalid += e.Invalid
		t.insert += e.Insert
		t.delete_ += e.Delete
		t.deleteList += e.DeleteList
		t.insertFailed += e.InsertFailed
		t.drop += e.Drop
		t.earlyDrop += e.EarlyDrop
		t.searched += e.Searched
		t.searchRestart += e.SearchRestart
		t.found += e.Found
	}
	return t
}

func newConntrackCollector(fs procfs.FS, interval time.Duration) *conntrackCollector {
	return &conntrackCollector{fs: fs, interval: interval, first: true}
}

func (c *conntrackCollector) Name() string { return "conntrack" }

func (c *conntrackCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.sockets = reg.GaugeVec(prefix+"_netfilter_conntrack_sockets_active_connections_average", "Conntrack Connections", labels)
	c.newConn = reg.GaugeVec(prefix+"_netfilter_conntrack_new_connections_persec_average", "Conntrack New", labels)
	c.changes = reg.GaugeVec(prefix+"_netfilter_conntrack_changes_changes_persec_average", "Conntrack Changes", labels)
	c.errors = reg.GaugeVec(prefix+"_netfilter_conntrack_errors_events_persec_average", "Conntrack Errors", labels)
	c.search = reg.GaugeVec(prefix+"_netfilter_conntrack_search_searches_persec_average", "Conntrack Search", labels)
}

func (c *conntrackCollector) Collect() error {
	entries, err := c.fs.ConntrackStat()
	if err != nil {
		return err
	}

	count := readConntrackCount()
	c.sockets.With("netfilter.conntrack_sockets", "connections", "conntrack").Set(float64(count))

	cur := sumConntrack(entries)

	if c.first {
		c.prev = cur
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()
	p := c.prev

	c.newConn.With("netfilter.conntrack_new", "new", "conntrack").Set(float64(safeDelta(cur.new, p.new)) / secs)
	c.newConn.With("netfilter.conntrack_new", "ignore", "conntrack").Set(float64(safeDelta(cur.ignore, p.ignore)) / secs)
	c.newConn.With("netfilter.conntrack_new", "invalid", "conntrack").Set(float64(safeDelta(cur.invalid, p.invalid)) / secs)

	c.changes.With("netfilter.conntrack_changes", "inserted", "conntrack").Set(float64(safeDelta(cur.insert, p.insert)) / secs)
	c.changes.With("netfilter.conntrack_changes", "deleted", "conntrack").Set(float64(safeDelta(cur.delete_, p.delete_)) / secs)
	c.changes.With("netfilter.conntrack_changes", "delete_list", "conntrack").Set(float64(safeDelta(cur.deleteList, p.deleteList)) / secs)

	c.errors.With("netfilter.conntrack_errors", "insert_failed", "conntrack").Set(float64(safeDelta(cur.insertFailed, p.insertFailed)) / secs)
	c.errors.With("netfilter.conntrack_errors", "drop", "conntrack").Set(float64(safeDelta(cur.drop, p.drop)) / secs)
	c.errors.With("netfilter.conntrack_errors", "early_drop", "conntrack").Set(float64(safeDelta(cur.earlyDrop, p.earlyDrop)) / secs)

	c.search.With("netfilter.conntrack_search", "searched", "conntrack").Set(float64(safeDelta(cur.searched, p.searched)) / secs)
	c.search.With("netfilter.conntrack_search", "restarted", "conntrack").Set(float64(safeDelta(cur.searchRestart, p.searchRestart)) / secs)
	c.search.With("netfilter.conntrack_search", "found", "conntrack").Set(float64(safeDelta(cur.found, p.found)) / secs)

	c.prev = cur
	return nil
}

func readConntrackCount() uint64 {
	b, err := os.ReadFile("/proc/sys/net/netfilter/nf_conntrack_count")
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	return v
}

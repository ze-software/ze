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

	prevNew           uint64
	prevInvalid       uint64
	prevInsert        uint64
	prevInsertFailed  uint64
	prevDrop          uint64
	prevSearched      uint64
	prevSearchRestart uint64
	prevDelete        uint64
	first             bool
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

	// Read current conntrack count from /proc/sys/net/netfilter/nf_conntrack_count
	count := readConntrackCount()
	c.sockets.With("netfilter.conntrack_sockets", "connections", "conntrack").Set(float64(count))

	var totalNew, totalInvalid, totalInsert, totalInsertFailed, totalDrop uint64
	var totalSearched, totalSearchRestart, totalDelete uint64
	for _, e := range entries {
		totalNew += e.New
		totalInvalid += e.Invalid
		totalInsert += e.Insert
		totalInsertFailed += e.InsertFailed
		totalDrop += e.Drop
		totalSearched += e.Searched
		totalSearchRestart += e.SearchRestart
		totalDelete += e.Delete
	}

	if c.first {
		c.prevNew = totalNew
		c.prevInvalid = totalInvalid
		c.prevInsert = totalInsert
		c.prevInsertFailed = totalInsertFailed
		c.prevDrop = totalDrop
		c.prevSearched = totalSearched
		c.prevSearchRestart = totalSearchRestart
		c.prevDelete = totalDelete
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()

	c.newConn.With("netfilter.conntrack_new", "new", "conntrack").Set(float64(safeDelta(totalNew, c.prevNew)) / secs)
	c.newConn.With("netfilter.conntrack_new", "ignore", "conntrack").Set(float64(safeDelta(totalInvalid, c.prevInvalid)) / secs)

	c.changes.With("netfilter.conntrack_changes", "inserted", "conntrack").Set(float64(safeDelta(totalInsert, c.prevInsert)) / secs)
	c.changes.With("netfilter.conntrack_changes", "deleted", "conntrack").Set(float64(safeDelta(totalDelete, c.prevDelete)) / secs)

	c.errors.With("netfilter.conntrack_errors", "insert_failed", "conntrack").Set(float64(safeDelta(totalInsertFailed, c.prevInsertFailed)) / secs)
	c.errors.With("netfilter.conntrack_errors", "drop", "conntrack").Set(float64(safeDelta(totalDrop, c.prevDrop)) / secs)

	c.search.With("netfilter.conntrack_search", "searched", "conntrack").Set(float64(safeDelta(totalSearched, c.prevSearched)) / secs)
	c.search.With("netfilter.conntrack_search", "restarted", "conntrack").Set(float64(safeDelta(totalSearchRestart, c.prevSearchRestart)) / secs)

	c.prevNew = totalNew
	c.prevInvalid = totalInvalid
	c.prevInsert = totalInsert
	c.prevInsertFailed = totalInsertFailed
	c.prevDrop = totalDrop
	c.prevSearched = totalSearched
	c.prevSearchRestart = totalSearchRestart
	c.prevDelete = totalDelete

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

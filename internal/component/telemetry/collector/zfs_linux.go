// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type zfsCollector struct {
	interval time.Duration

	arcSize  metrics.GaugeVec
	reads    metrics.GaugeVec
	hits     metrics.GaugeVec
	hitsRate metrics.GaugeVec
	l2Size   metrics.GaugeVec
	l2Hits   metrics.GaugeVec
	memory   metrics.GaugeVec

	prev  map[string]uint64
	first bool
}

func newZFSCollector(interval time.Duration) *zfsCollector {
	return &zfsCollector{interval: interval, first: true}
}

func (c *zfsCollector) Name() string { return "zfs" }

func (c *zfsCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.arcSize = reg.GaugeVec(prefix+"_zfs_arc_size_MiB_average", "ZFS ARC Size", labels)
	c.reads = reg.GaugeVec(prefix+"_zfs_reads_reads_persec_average", "ZFS Reads", labels)
	c.hits = reg.GaugeVec(prefix+"_zfs_hits_percentage_average", "ZFS ARC Hits", labels)
	c.hitsRate = reg.GaugeVec(prefix+"_zfs_hits_rate_events_persec_average", "ZFS ARC Hit Rate", labels)
	c.l2Size = reg.GaugeVec(prefix+"_zfs_l2_size_MiB_average", "ZFS L2ARC Size", labels)
	c.l2Hits = reg.GaugeVec(prefix+"_zfs_l2_hits_rate_events_persec_average", "ZFS L2ARC Hit Rate", labels)
	c.memory = reg.GaugeVec(prefix+"_zfs_memory_ops_MiB_average", "ZFS ARC Memory", labels)
}

const mibDivisor = 1024 * 1024

func (c *zfsCollector) Collect() error {
	cur, err := readArcStats()
	if err != nil {
		return err
	}
	if len(cur) == 0 {
		return nil
	}

	c.arcSize.With("zfs.arc_size", "arcsz", "size").Set(float64(cur["size"]) / mibDivisor)
	c.arcSize.With("zfs.arc_size", "target", "size").Set(float64(cur["c"]) / mibDivisor)
	c.arcSize.With("zfs.arc_size", "min", "size").Set(float64(cur["c_min"]) / mibDivisor)
	c.arcSize.With("zfs.arc_size", "max", "size").Set(float64(cur["c_max"]) / mibDivisor)

	if cur["l2_size"] > 0 || cur["l2_asize"] > 0 {
		c.l2Size.With("zfs.l2_size", "actual", "l2cache").Set(float64(cur["l2_asize"]) / mibDivisor)
		c.l2Size.With("zfs.l2_size", "size", "l2cache").Set(float64(cur["l2_size"]) / mibDivisor)
	}

	c.memory.With("zfs.memory_ops", "anonymous", "memory").Set(float64(cur["anon_size"]) / mibDivisor)
	c.memory.With("zfs.memory_ops", "header", "memory").Set(float64(cur["hdr_size"]) / mibDivisor)
	c.memory.With("zfs.memory_ops", "metadata", "memory").Set(float64(cur["metadata_size"]) / mibDivisor)
	c.memory.With("zfs.memory_ops", "other", "memory").Set(float64(cur["other_size"]) / mibDivisor)

	if c.first {
		c.prev = cur
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()

	dHits := safeDelta(cur["hits"], c.prev["hits"])
	dMisses := safeDelta(cur["misses"], c.prev["misses"])
	total := dHits + dMisses
	if total > 0 {
		c.hits.With("zfs.hits", "hits", "hits").Set(float64(dHits) / float64(total) * 100)
		c.hits.With("zfs.hits", "misses", "hits").Set(float64(dMisses) / float64(total) * 100)
	}

	c.hitsRate.With("zfs.hits_rate", "hits", "hits").Set(float64(dHits) / secs)
	c.hitsRate.With("zfs.hits_rate", "misses", "hits").Set(float64(dMisses) / secs)

	dArcRead := safeDelta(cur["demand_data_hits"], c.prev["demand_data_hits"]) +
		safeDelta(cur["demand_data_misses"], c.prev["demand_data_misses"]) +
		safeDelta(cur["prefetch_data_hits"], c.prev["prefetch_data_hits"]) +
		safeDelta(cur["prefetch_data_misses"], c.prev["prefetch_data_misses"])
	dL2Read := safeDelta(cur["l2_hits"], c.prev["l2_hits"]) + safeDelta(cur["l2_misses"], c.prev["l2_misses"])
	c.reads.With("zfs.reads", "arc", "reads").Set(float64(dArcRead) / secs)
	c.reads.With("zfs.reads", "l2", "reads").Set(float64(dL2Read) / secs)

	c.l2Hits.With("zfs.l2_hits_rate", "hits", "l2cache").Set(float64(safeDelta(cur["l2_hits"], c.prev["l2_hits"])) / secs)
	c.l2Hits.With("zfs.l2_hits_rate", "misses", "l2cache").Set(float64(safeDelta(cur["l2_misses"], c.prev["l2_misses"])) / secs)

	c.prev = cur
	return nil
}

func readArcStats() (map[string]uint64, error) {
	f, err := os.Open("/proc/spl/kstat/zfs/arcstats")
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	m := make(map[string]uint64, 64)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 {
			continue
		}
		v, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			continue
		}
		m[fields[0]] = v
	}
	return m, scanner.Err()
}

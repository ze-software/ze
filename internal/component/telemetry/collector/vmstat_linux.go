// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type vmstatCollector struct {
	interval time.Duration

	pgfaults metrics.GaugeVec
	pgio     metrics.GaugeVec
	swapio   metrics.GaugeVec

	prev  map[string]uint64
	first bool
}

func newVMStatCollector(interval time.Duration) *vmstatCollector {
	return &vmstatCollector{interval: interval, first: true}
}

func (c *vmstatCollector) Name() string { return "vmstat" }

func (c *vmstatCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.pgfaults = reg.GaugeVec(prefix+"_mem_pgfaults_faults_persec_average", "Page Faults", labels)
	c.pgio = reg.GaugeVec(prefix+"_system_pgpgio_KiB_persec_average", "Disk Paging I/O", labels)
	c.swapio = reg.GaugeVec(prefix+"_mem_swapio_KiB_persec_average", "Swap I/O", labels)
}

func (c *vmstatCollector) Collect() error {
	cur, err := readVMStat()
	if err != nil {
		return err
	}

	if c.first {
		c.prev = cur
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()

	// Page faults
	c.pgfaults.With("mem.pgfaults", "minor", "mem").Set(float64(safeDelta(cur["pgfault"], c.prev["pgfault"])) / secs)
	c.pgfaults.With("mem.pgfaults", "major", "mem").Set(float64(safeDelta(cur["pgmajfault"], c.prev["pgmajfault"])) / secs)

	c.pgio.With("system.pgpgio", "in", "disk").Set(float64(safeDelta(cur["pgpgin"], c.prev["pgpgin"])) / secs)
	c.pgio.With("system.pgpgio", "out", "disk").Set(float64(safeDelta(cur["pgpgout"], c.prev["pgpgout"])) / secs)

	c.swapio.With("mem.swapio", "in", "swap").Set(float64(safeDelta(cur["pswpin"], c.prev["pswpin"])) / secs)
	c.swapio.With("mem.swapio", "out", "swap").Set(float64(safeDelta(cur["pswpout"], c.prev["pswpout"])) / secs)

	c.prev = cur
	return nil
}

func readVMStat() (map[string]uint64, error) {
	f, err := os.Open("/proc/vmstat")
	if err != nil {
		return nil, fmt.Errorf("open /proc/vmstat: %w", err)
	}
	defer func() { _ = f.Close() }()

	m := make(map[string]uint64, 64)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		m[fields[0]] = v
	}
	return m, scanner.Err()
}

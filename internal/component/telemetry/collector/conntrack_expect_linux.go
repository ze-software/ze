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

type conntrackExpectCollector struct {
	interval time.Duration

	gauge metrics.GaugeVec

	prevNew    uint64
	prevCreate uint64
	prevDelete uint64
	first      bool
}

func newConntrackExpectCollector(interval time.Duration) *conntrackExpectCollector {
	return &conntrackExpectCollector{interval: interval, first: true}
}

func (c *conntrackExpectCollector) Name() string { return "conntrack_expect" }

func (c *conntrackExpectCollector) Init(reg metrics.Registry, prefix string) {
	c.gauge = reg.GaugeVec(
		prefix+"_netfilter_conntrack_expect_expectations_persec_average",
		"Conntrack Expectations",
		[]string{"chart", "dimension", "family"},
	)
}

func (c *conntrackExpectCollector) Collect() error {
	newV, create, del, err := readConntrackExpect()
	if err != nil {
		return err
	}

	if c.first {
		c.prevNew = newV
		c.prevCreate = create
		c.prevDelete = del
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()
	const chart = "netfilter.conntrack_expect"
	const family = "conntrack"

	c.gauge.With(chart, "new", family).Set(float64(safeDelta(newV, c.prevNew)) / secs)
	c.gauge.With(chart, "created", family).Set(float64(safeDelta(create, c.prevCreate)) / secs)
	c.gauge.With(chart, "deleted", family).Set(float64(safeDelta(del, c.prevDelete)) / secs)

	c.prevNew = newV
	c.prevCreate = create
	c.prevDelete = del
	return nil
}

// readConntrackExpect parses expect_new, expect_create, expect_delete from
// /proc/net/stat/nf_conntrack. The file has a header row followed by one
// hex-valued row per CPU. Columns 13, 14, 15 (0-indexed) are the expect fields.
func readConntrackExpect() (expectNew, expectCreate, expectDelete uint64, err error) {
	f, err := os.Open("/proc/net/stat/nf_conntrack")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("open nf_conntrack stat: %w", err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 16 {
			continue
		}
		n, _ := strconv.ParseUint(fields[13], 16, 64)
		c, _ := strconv.ParseUint(fields[14], 16, 64)
		d, _ := strconv.ParseUint(fields[15], 16, 64)
		expectNew += n
		expectCreate += c
		expectDelete += d
	}
	return expectNew, expectCreate, expectDelete, scanner.Err()
}

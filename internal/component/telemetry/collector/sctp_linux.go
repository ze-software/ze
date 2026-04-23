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

type sctpCollector struct {
	interval time.Duration

	gauge metrics.GaugeVec

	prev  map[string]uint64
	first bool
}

func newSCTPCollector(interval time.Duration) *sctpCollector {
	return &sctpCollector{interval: interval, first: true}
}

func (c *sctpCollector) Name() string { return "sctp" }

func (c *sctpCollector) Init(reg metrics.Registry, prefix string) {
	c.gauge = reg.GaugeVec(
		prefix+"_sctp_snmp_packets_persec_average",
		"SCTP Statistics",
		[]string{"chart", "dimension", "family"},
	)
}

func (c *sctpCollector) Collect() error {
	cur, err := readSCTPSnmp()
	if err != nil {
		return err
	}

	if c.first {
		c.prev = cur
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()
	const chart = "sctp.snmp"
	const family = "sctp"

	c.gauge.With(chart, "SctpCurrEstab", family).Set(float64(cur["SctpCurrEstab"]))

	for _, dim := range []string{
		"SctpActiveEstabs", "SctpPassiveEstabs",
		"SctpAborteds", "SctpShutdowns",
		"SctpInSCTPPacks", "SctpOutSCTPPacks",
		"SctpInCtrlChunks", "SctpOutCtrlChunks",
		"SctpInOrderChunks", "SctpOutOrderChunks",
		"SctpInUnorderChunks", "SctpOutUnorderChunks",
		"SctpT1InitExpireds", "SctpT2ShutdownExpireds",
		"SctpT3RtxExpireds", "SctpFragUsrMsgs", "SctpReasmUsrMsgs",
	} {
		c.gauge.With(chart, dim, family).Set(float64(safeDelta(cur[dim], c.prev[dim])) / secs)
	}

	c.prev = cur
	return nil
}

func readSCTPSnmp() (map[string]uint64, error) {
	f, err := os.Open("/proc/net/sctp/snmp")
	if err != nil {
		return nil, fmt.Errorf("open /proc/net/sctp/snmp: %w", err)
	}
	defer func() { _ = f.Close() }()

	m := make(map[string]uint64, 32)
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

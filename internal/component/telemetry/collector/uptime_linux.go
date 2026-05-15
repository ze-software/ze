// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

var errMalformedProcUptime = errors.New("malformed /proc/uptime")

type uptimeCollector struct {
	gauge metrics.GaugeVec
}

func newUptimeCollector() *uptimeCollector {
	return &uptimeCollector{}
}

func (c *uptimeCollector) Name() string { return "uptime" }

func (c *uptimeCollector) Init(reg metrics.Registry, prefix string) {
	c.gauge = reg.GaugeVec(
		prefix+"_system_uptime_seconds_average",
		"System Uptime",
		[]string{"chart", "dimension", "family"},
	)
}

func (c *uptimeCollector) Collect() error {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return fmt.Errorf("read /proc/uptime: %w", err)
	}
	fields := strings.Fields(string(b))
	if len(fields) < 1 {
		return errMalformedProcUptime
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return fmt.Errorf("parse uptime: %w", err)
	}
	c.gauge.With("system.uptime", "uptime", "uptime").Set(secs)
	return nil
}

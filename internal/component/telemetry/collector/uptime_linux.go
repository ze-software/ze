// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/metrics"
)

var errMalformedProcUptime = errors.New("malformed /proc/uptime")

type uptimeCollector struct {
	gauge metrics.GaugeVec
	path  string // seam: defaults to /proc/uptime, overridable in tests
}

func newUptimeCollector() *uptimeCollector {
	return &uptimeCollector{path: "/proc/uptime"}
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
	b, err := os.ReadFile(c.path) //nolint:gosec // c.path defaults to the constant /proc/uptime; overridden only in tests
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

// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type fileNRCollector struct {
	gauge metrics.GaugeVec
}

func newFileNRCollector() *fileNRCollector {
	return &fileNRCollector{}
}

func (c *fileNRCollector) Name() string { return "filenr" }

func (c *fileNRCollector) Init(reg metrics.Registry, prefix string) {
	c.gauge = reg.GaugeVec(
		prefix+"_system_file_nr_used_files_average",
		"Open Files",
		[]string{"chart", "dimension", "family"},
	)
}

func (c *fileNRCollector) Collect() error {
	b, err := os.ReadFile("/proc/sys/fs/file-nr")
	if err != nil {
		return fmt.Errorf("read /proc/sys/fs/file-nr: %w", err)
	}
	fields := strings.Fields(string(b))
	if len(fields) < 3 {
		return fmt.Errorf("malformed /proc/sys/fs/file-nr")
	}
	allocated, _ := strconv.ParseFloat(fields[0], 64)
	freeNR, _ := strconv.ParseFloat(fields[1], 64)
	used := allocated - freeNR
	c.gauge.With("system.file_nr_used", "used", "files").Set(used)
	return nil
}

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

var errMalformedProcSysFsFileNr = errors.New("malformed /proc/sys/fs/file-nr")

type fileNRCollector struct {
	gauge metrics.GaugeVec
	path  string // seam: defaults to /proc/sys/fs/file-nr, overridable in tests
}

func newFileNRCollector() *fileNRCollector {
	return &fileNRCollector{path: "/proc/sys/fs/file-nr"}
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
	b, err := os.ReadFile(c.path) //nolint:gosec // c.path defaults to the constant /proc/sys/fs/file-nr; overridden only in tests
	if err != nil {
		return fmt.Errorf("read /proc/sys/fs/file-nr: %w", err)
	}
	fields := strings.Fields(string(b))
	if len(fields) < 3 {
		return errMalformedProcSysFsFileNr
	}
	allocated, _ := strconv.ParseFloat(fields[0], 64)
	freeNR, _ := strconv.ParseFloat(fields[1], 64)
	used := allocated - freeNR
	c.gauge.With("system.file_nr_used", "used", "files").Set(used)
	return nil
}

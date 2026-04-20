// Design: plan/spec-host-0-inventory.md — hardware inventory detection

//go:build linux

package host

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// meminfoFields maps /proc/meminfo keys (trailing colon stripped) to
// the MemoryInfo setter. Values are reported in kB by the kernel and
// converted to bytes here.
var meminfoFields = map[string]func(*MemoryInfo, uint64){
	"MemTotal":     func(m *MemoryInfo, v uint64) { m.TotalBytes = v },
	"MemFree":      func(m *MemoryInfo, v uint64) { m.FreeBytes = v },
	"MemAvailable": func(m *MemoryInfo, v uint64) { m.AvailableBytes = v },
	"Buffers":      func(m *MemoryInfo, v uint64) { m.BuffersBytes = v },
	"Cached":       func(m *MemoryInfo, v uint64) { m.CachedBytes = v },
	"SwapTotal":    func(m *MemoryInfo, v uint64) { m.SwapTotalBytes = v },
	"SwapFree":     func(m *MemoryInfo, v uint64) { m.SwapFreeBytes = v },
}

// DetectMemory reads /proc/meminfo and any edac memory-controller
// directories under /sys/devices/system/edac/mc/. Missing edac (no ECC
// DIMMs, or driver not loaded) leaves the ECC counters at zero with
// ECCPresent=false.
func (d *Detector) DetectMemory() (*MemoryInfo, error) {
	info, err := d.readMeminfo()
	if err != nil {
		return nil, err
	}
	d.readECCCounters(info)
	return info, nil
}

// readMeminfo parses /proc/meminfo into MemoryInfo.
func (d *Detector) readMeminfo() (*MemoryInfo, error) {
	path := d.procPath("meminfo")
	f, err := os.Open(path) //nolint:gosec // path is under /proc root
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	info := &MemoryInfo{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		key, kb, ok := parseMeminfoLine(scanner.Text())
		if !ok {
			continue
		}
		if set, known := meminfoFields[key]; known {
			set(info, kb*1024)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan meminfo: %w", err)
	}
	return info, nil
}

// parseMeminfoLine parses one meminfo line of the form
//
//	Key:        <number> kB
//
// Returns key, value in kB, ok.
func parseMeminfoLine(line string) (key string, kb uint64, ok bool) {
	k, rest, found := strings.Cut(line, ":")
	if !found {
		return "", 0, false
	}
	fields := strings.Fields(strings.TrimSpace(rest))
	if len(fields) == 0 {
		return "", 0, false
	}
	v, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return k, v, true
}

// readECCCounters walks /sys/devices/system/edac/mc/mc*/ and sums the
// ce_count (correctable) and ue_count (uncorrectable) files across all
// memory controllers. When the edac subsystem is absent the walk
// yields no entries and counters stay zero.
func (d *Detector) readECCCounters(info *MemoryInfo) {
	base := d.sysfsPath("devices/system/edac/mc")
	entries, err := os.ReadDir(base)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return
		}
		return
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "mc") {
			continue
		}
		info.ECCPresent = true
		mc := filepath.Join(base, e.Name())
		info.ECCCorrectableErrors += uint64(readFileInt(filepath.Join(mc, "ce_count")))   //nolint:gosec // kernel counter, non-negative
		info.ECCUncorrectableErrors += uint64(readFileInt(filepath.Join(mc, "ue_count"))) //nolint:gosec // kernel counter, non-negative
	}
}

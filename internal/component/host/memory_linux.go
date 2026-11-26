// Design: docs/architecture/host/inventory.md -- hardware inventory detection

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

const (
	meminfoKeyTotal     = "MemTotal"
	meminfoKeyAvailable = "MemAvailable"
)

// meminfoFields maps /proc/meminfo keys (trailing colon stripped) to
// the MemoryInfo setter. Values are reported in kB by the kernel and
// converted to bytes here.
var meminfoFields = map[string]func(*MemoryInfo, uint64){
	meminfoKeyTotal:     func(m *MemoryInfo, v uint64) { m.TotalBytes = v },
	"MemFree":           func(m *MemoryInfo, v uint64) { m.FreeBytes = v },
	meminfoKeyAvailable: func(m *MemoryInfo, v uint64) { m.AvailableBytes = v },
	"Buffers":           func(m *MemoryInfo, v uint64) { m.BuffersBytes = v },
	"Cached":            func(m *MemoryInfo, v uint64) { m.CachedBytes = v },
	"SwapTotal":         func(m *MemoryInfo, v uint64) { m.SwapTotalBytes = v },
	"SwapFree":          func(m *MemoryInfo, v uint64) { m.SwapFreeBytes = v },
	"Hugepagesize":      func(m *MemoryInfo, v uint64) { m.HugepageSizeBytes = v },
}

// meminfoCounts maps the /proc/meminfo keys whose value is a PAGE COUNT, not a
// size in kB. They must not go through the kB-to-bytes conversion above:
// "HugePages_Total: 64" means 64 pages, and multiplying it by 1024 would report
// a nonsense 65536. Only "Hugepagesize" and "Hugetlb" carry the kB suffix.
var meminfoCounts = map[string]func(*MemoryInfo, uint64){
	"HugePages_Total": func(m *MemoryInfo, v uint64) { m.HugepagesTotal = v },
	"HugePages_Free":  func(m *MemoryInfo, v uint64) { m.HugepagesFree = v },
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
	info := &MemoryInfo{}
	err := scanMeminfo(d.procPath("meminfo"), func(key string, value uint64) {
		if set, known := meminfoFields[key]; known {
			set(info, value*1024)
			return
		}
		if set, known := meminfoCounts[key]; known {
			set(info, value)
		}
	})
	if err != nil {
		return nil, err
	}
	return info, nil
}

// kernelMemoryFields maps the /proc/meminfo keys the kernel-memory trend rule
// reads to the KernelMemory setter. All three are kB lines, converted to bytes
// as meminfoFields converts them. They are kept apart from meminfoFields so the
// answer of `show host memory` keeps its shape: SUnreclaim is not one of its
// keys.
var kernelMemoryFields = map[string]func(*KernelMemory, uint64){
	meminfoKeyTotal:     func(m *KernelMemory, v uint64) { m.TotalBytes = v },
	meminfoKeyAvailable: func(m *KernelMemory, v uint64) { m.AvailableBytes = v },
	"SUnreclaim":        func(m *KernelMemory, v uint64) { m.UnreclaimableBytes = v },
}

// errMeminfoLineMissing names a /proc/meminfo line DetectKernelMemory needs
// and did not find.
var errMeminfoLineMissing = errors.New("has no line for a counter the kernel-memory rule needs")

// DetectKernelMemory reads the three /proc/meminfo counters the kernel-memory
// trend rule judges. Every error it answers is an *fs.PathError naming the
// file, so the rule reports which path it could not read. A file without one
// of the three lines is such an error, never a zero counter: the kernel has
// carried all three since Linux 3.14.
func (d *Detector) DetectKernelMemory() (KernelMemory, error) {
	path := d.procPath("meminfo")
	var memory KernelMemory
	found := make(map[string]bool, len(kernelMemoryFields))
	err := scanMeminfo(path, func(key string, value uint64) {
		if set, known := kernelMemoryFields[key]; known {
			set(&memory, value*1024)
			found[key] = true
		}
	})
	if err != nil {
		return KernelMemory{}, err
	}
	for key := range kernelMemoryFields {
		if !found[key] {
			return KernelMemory{}, &fs.PathError{Op: "parse", Path: path,
				Err: fmt.Errorf("%w: %s", errMeminfoLineMissing, key)}
		}
	}
	return memory, nil
}

// scanMeminfo is the one /proc/meminfo reader: it opens path and hands visit
// the key and the bare number of every line parseMeminfoLine accepts, leaving
// the unit to the caller's map. Every error it answers is an *fs.PathError
// naming path.
func scanMeminfo(path string, visit func(key string, value uint64)) error {
	f, err := os.Open(path) //nolint:gosec // path is under /proc root
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		key, value, ok := parseMeminfoLine(scanner.Text())
		if !ok {
			continue
		}
		visit(key, value)
	}
	if err := scanner.Err(); err != nil {
		return &fs.PathError{Op: "read", Path: path, Err: err}
	}
	return nil
}

// parseMeminfoLine parses one meminfo line of the form
//
//	Key:        <number>[ kB]
//
// Returns the key and the bare number, without interpreting its unit: most
// lines are kB, the HugePages_* lines are counts, and only the caller's map
// membership says which. The unit suffix, when present, is ignored here.
func parseMeminfoLine(line string) (key string, value uint64, ok bool) {
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

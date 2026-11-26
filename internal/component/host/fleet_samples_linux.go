// Design: docs/architecture/fleet-config.md — host-owned Linux fleet metric sample producer
// Related: fleet_samples_other.go — non-Linux platform counterpart
// Related: fleet_inventory.go — host hardware inventory projection for fleet reporting

//go:build linux

package host

import (
	"bufio"
	"bytes"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/ze-software/ze/pkg/fleet"
)

const (
	procStatPath    = "/proc/stat"
	procMeminfoPath = "/proc/meminfo"
	procLoadavgPath = "/proc/loadavg"
	procUptimePath  = "/proc/uptime"
	rootFSPath      = "/"
	kibInBytes      = 1024.0
)

type cpuJiffies struct {
	total uint64
	idle  uint64
}

type rootFSStat struct {
	blocks uint64
	bfree  uint64
	bsize  int64
}

type meminfoSnapshot struct {
	totalKiB     uint64
	hasTotal     bool
	availableKiB uint64
	hasAvailable bool
	freeKiB      uint64
	hasFree      bool
	buffersKiB   uint64
	cachedKiB    uint64
}

// FleetSampler reads Linux procfs and root filesystem counters directly to
// produce a bounded host SourceSnapshot while maintaining the CPU jiffies
// baseline between calls. Safe for concurrent use.
type FleetSampler struct {
	mu          sync.Mutex
	hasPrevCPU  bool
	prevCPU     cpuJiffies
	readStat    func() ([]byte, error)
	readMeminfo func() ([]byte, error)
	readLoadavg func() ([]byte, error)
	readUptime  func() ([]byte, error)
	statRootFS  func() (rootFSStat, error)
}

var defaultFleetSampler = NewFleetSampler()

// SampleFleetMetrics returns one bounded host metric snapshot from the
// package-level Linux sampler. Safe for concurrent use.
func SampleFleetMetrics() fleet.SourceSnapshot {
	return defaultFleetSampler.Sample()
}

// NewFleetSampler constructs a FleetSampler bound to the live Linux procfs
// files and root filesystem.
func NewFleetSampler() *FleetSampler {
	return &FleetSampler{
		readStat: func() ([]byte, error) {
			return os.ReadFile(procStatPath)
		},
		readMeminfo: func() ([]byte, error) {
			return os.ReadFile(procMeminfoPath)
		},
		readLoadavg: func() ([]byte, error) {
			return os.ReadFile(procLoadavgPath)
		},
		readUptime: func() ([]byte, error) {
			return os.ReadFile(procUptimePath)
		},
		statRootFS: readRootFSStat,
	}
}

// Sample reads all available host figures and returns a SourceSnapshot owned
// by fleet.SampleOwnerHost. Readings that fail to read, lack a valid baseline,
// or fall outside their finite domain are omitted rather than synthesized.
func (s *FleetSampler) Sample() fleet.SourceSnapshot {
	readings := make([]fleet.SourceReading, 0, 7)

	if cpuPct, ok := s.sampleCPUUsed(); ok {
		readings = append(readings, fleet.SourceReading{
			Figure: fleet.FigureCPUUsed,
			Unit:   fleet.SampleUnitPercent,
			Value:  cpuPct,
		})
	}

	if memUsed, hasUsed, memTotal, hasTotal := s.sampleMemory(); hasTotal {
		if hasUsed {
			readings = append(readings, fleet.SourceReading{
				Figure: fleet.FigureMemoryUsed,
				Unit:   fleet.SampleUnitBytes,
				Value:  memUsed,
			})
		}
		readings = append(readings, fleet.SourceReading{
			Figure: fleet.FigureMemoryTotal,
			Unit:   fleet.SampleUnitBytes,
			Value:  memTotal,
		})
	}

	if storageUsed, storageTotal, ok := s.sampleStorage(); ok {
		readings = append(readings,
			fleet.SourceReading{
				Figure: fleet.FigureStorageUsed,
				Unit:   fleet.SampleUnitBytes,
				Value:  storageUsed,
			},
			fleet.SourceReading{
				Figure: fleet.FigureStorageTotal,
				Unit:   fleet.SampleUnitBytes,
				Value:  storageTotal,
			},
		)
	}

	if load1m, ok := s.sampleLoad1m(); ok {
		readings = append(readings, fleet.SourceReading{
			Figure: fleet.FigureLoad1m,
			Unit:   fleet.SampleUnitNumber,
			Value:  load1m,
		})
	}

	if uptimeSec, ok := s.sampleUptime(); ok {
		readings = append(readings, fleet.SourceReading{
			Figure: fleet.FigureUptime,
			Unit:   fleet.SampleUnitSeconds,
			Value:  uptimeSec,
		})
	}

	return fleet.SourceSnapshot{
		Owner:    fleet.SampleOwnerHost,
		Readings: readings,
	}
}

func (s *FleetSampler) sampleCPUUsed() (float64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.readStat == nil {
		s.hasPrevCPU = false
		return 0, false
	}
	data, err := s.readStat()
	if err != nil {
		s.hasPrevCPU = false
		return 0, false
	}
	curr, ok := parseProcStatCPU(data)
	if !ok {
		s.hasPrevCPU = false
		return 0, false
	}
	if !s.hasPrevCPU {
		s.prevCPU = curr
		s.hasPrevCPU = true
		return 0, false
	}

	prev := s.prevCPU
	s.prevCPU = curr
	if curr.total <= prev.total {
		return 0, false
	}
	if curr.idle < prev.idle {
		return 0, false
	}
	deltaTotal := curr.total - prev.total
	deltaIdle := curr.idle - prev.idle
	if deltaIdle > deltaTotal {
		return 0, false
	}
	deltaBusy := deltaTotal - deltaIdle
	pct := (float64(deltaBusy) / float64(deltaTotal)) * 100.0
	if !isFiniteNonNegative(pct) {
		return 0, false
	}
	if pct > 100.0 {
		pct = 100.0
	}
	return pct, true
}

func (s *FleetSampler) sampleMemory() (usedBytes float64, hasUsed bool, totalBytes float64, hasTotal bool) {
	if s.readMeminfo == nil {
		return 0, false, 0, false
	}
	data, err := s.readMeminfo()
	if err != nil {
		return 0, false, 0, false
	}
	mem := parseProcMeminfo(data)
	if !mem.hasTotal {
		return 0, false, 0, false
	}
	totalBytes = float64(mem.totalKiB) * kibInBytes
	if !isFiniteNonNegative(totalBytes) {
		return 0, false, 0, false
	}
	hasTotal = true

	if mem.hasAvailable {
		var usedKiB uint64
		if mem.totalKiB > mem.availableKiB {
			usedKiB = mem.totalKiB - mem.availableKiB
		}
		usedBytes = float64(usedKiB) * kibInBytes
		if isFiniteNonNegative(usedBytes) {
			hasUsed = true
		}
		return usedBytes, hasUsed, totalBytes, hasTotal
	}
	if mem.hasFree {
		reclaimable := saturatingAddUint64(saturatingAddUint64(mem.freeKiB, mem.buffersKiB), mem.cachedKiB)
		var usedKiB uint64
		if mem.totalKiB > reclaimable {
			usedKiB = mem.totalKiB - reclaimable
		}
		usedBytes = float64(usedKiB) * kibInBytes
		if isFiniteNonNegative(usedBytes) {
			hasUsed = true
		}
	}
	return usedBytes, hasUsed, totalBytes, hasTotal
}

func (s *FleetSampler) sampleStorage() (usedBytes, totalBytes float64, ok bool) {
	if s.statRootFS == nil {
		return 0, 0, false
	}
	st, err := s.statRootFS()
	if err != nil {
		return 0, 0, false
	}
	if st.bsize <= 0 {
		return 0, 0, false
	}
	if st.blocks == 0 {
		return 0, 0, false
	}
	blockSize := float64(uint64(st.bsize))
	totalBytes = float64(st.blocks) * blockSize
	var usedBlocks uint64
	if st.blocks > st.bfree {
		usedBlocks = st.blocks - st.bfree
	}
	usedBytes = float64(usedBlocks) * blockSize
	if !isFiniteNonNegative(totalBytes) {
		return 0, 0, false
	}
	if !isFiniteNonNegative(usedBytes) {
		return 0, 0, false
	}
	return usedBytes, totalBytes, true
}

func (s *FleetSampler) sampleLoad1m() (float64, bool) {
	if s.readLoadavg == nil {
		return 0, false
	}
	data, err := s.readLoadavg()
	if err != nil {
		return 0, false
	}
	return parseFirstNonNegativeFloat(data)
}

func (s *FleetSampler) sampleUptime() (float64, bool) {
	if s.readUptime == nil {
		return 0, false
	}
	data, err := s.readUptime()
	if err != nil {
		return 0, false
	}
	return parseFirstNonNegativeFloat(data)
}

func readRootFSStat() (rootFSStat, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(rootFSPath, &st); err != nil {
		return rootFSStat{}, err
	}
	return rootFSStat{
		blocks: st.Blocks,
		bfree:  st.Bfree,
		bsize:  int64(st.Bsize), //nolint:unconvert // Bsize width differs across Linux architectures
	}, nil
}

// parseProcStatCPU parses the aggregate "cpu" line from /proc/stat. Linux
// already includes guest and guest_nice in user and nice, so only the first
// eight jiffy columns (user, nice, system, idle, iowait, irq, softirq, steal)
// are summed into total; idle combines idle and iowait.
func parseProcStatCPU(data []byte) (cpuJiffies, bool) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		if fields[0] != "cpu" {
			continue
		}
		if len(fields) < 5 {
			return cpuJiffies{}, false
		}
		colCount := min(len(fields)-1, 8)
		var vals [8]uint64
		var total uint64
		for i := range colCount {
			v, err := strconv.ParseUint(fields[i+1], 10, 64)
			if err != nil {
				return cpuJiffies{}, false
			}
			vals[i] = v
			if math.MaxUint64-total < v {
				return cpuJiffies{}, false
			}
			total += v
		}
		idle := vals[3]
		if colCount >= 5 {
			if math.MaxUint64-idle < vals[4] {
				return cpuJiffies{}, false
			}
			idle += vals[4]
		}
		return cpuJiffies{total: total, idle: idle}, true
	}
	return cpuJiffies{}, false
}

func parseProcMeminfo(data []byte) meminfoSnapshot {
	var out meminfoSnapshot
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		val, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case meminfoKeyTotal:
			out.totalKiB = val
			out.hasTotal = true
		case meminfoKeyAvailable:
			out.availableKiB = val
			out.hasAvailable = true
		case "MemFree":
			out.freeKiB = val
			out.hasFree = true
		case "Buffers":
			out.buffersKiB = val
		case "Cached":
			out.cachedKiB = val
		}
	}
	return out
}

func parseFirstNonNegativeFloat(data []byte) (float64, bool) {
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, false
	}
	if !isFiniteNonNegative(v) {
		return 0, false
	}
	return v, true
}

func isFiniteNonNegative(v float64) bool {
	if math.IsNaN(v) {
		return false
	}
	if math.IsInf(v, 0) {
		return false
	}
	return v >= 0
}

func saturatingAddUint64(a, b uint64) uint64 {
	if math.MaxUint64-a < b {
		return math.MaxUint64
	}
	return a + b
}

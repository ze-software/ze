// Design: plan/spec-host-0-inventory.md — hardware inventory detection

//go:build linux

package host

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// DetectCPU reads /proc/cpuinfo and /sys/devices/system/cpu/cpu*/ under
// the Detector's Root and returns a populated *CPUInfo. Non-fatal errors
// (permission denied, malformed line) are left to the caller's Inventory
// — DetectCPU only returns a top-level error for setup failures
// (unreadable /proc/cpuinfo).
//
// The structure follows /proc/cpuinfo blocks: each `processor : N`
// starts a new logical-CPU block; fields like `vendor_id`, `cpu family`,
// `model name`, `flags` repeat per block but are identical on
// non-hybrid parts. The first non-empty value wins at the package level.
//
// Hybrid detection uses /sys/devices/system/cpu/cpu*/cpu_capacity. When
// all capacities are equal, the CPU is uniform (N100). When they split
// into two clusters, the higher-capacity cluster is "performance" and
// the lower is "efficient" (Alder Lake and later).
func (d *Detector) DetectCPU() (*CPUInfo, error) {
	path := d.procPath("cpuinfo")
	f, err := os.Open(path) //nolint:gosec // path is under /proc root
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	cpu := &CPUInfo{Vendor: CPUVendorUnknown, ScalingDriver: ScalingDriverUnknown}

	// Grow the scanner's buffer to 1 MiB. /proc/cpuinfo's `flags:` line
	// on feature-rich CPUs already exceeds 4 KiB and new kernels keep
	// appending; bufio's default 64 KiB is enough today but the margin
	// is cheap and avoids a future bufio.ErrTooLong abort on detection.
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	cores, flags, err := parseCPUInfo(scanner, cpu)
	if err != nil {
		return nil, err
	}
	cpu.LogicalCPUs = len(cores)
	cpu.Flags = flags
	cpu.HWPAvailable = hasFlag(flags, "hwp")

	// Enrich per-core fields from sysfs.
	for i := range cores {
		d.fillCoreSysfs(&cores[i])
	}

	cpu.Cores = cores
	cpu.PhysicalCores = countPhysicalCores(cores)
	if cpu.PhysicalCores > 0 {
		cpu.ThreadsPerCore = cpu.LogicalCPUs / cpu.PhysicalCores
	}

	classifyHybrid(cpu)

	// Scaling driver from cpu0 (same across cores in practice).
	if len(cores) > 0 {
		cpu.ScalingDriver = d.readScalingDriver(0)
	}

	// Base and max freq from cpu0. Current-freq average across cores.
	cpu.BaseFreqMHz = khzToMHz(d.readCPUFreqInt(0, "base_frequency"))
	cpu.MaxFreqMHz = khzToMHz(d.readCPUFreqInt(0, "cpuinfo_max_freq"))
	cpu.CurrentFreqAvgMHz = averageCurrentFreqMHz(cores)

	return cpu, nil
}

// parseCPUInfo walks /proc/cpuinfo blocks. One block per `processor : N`.
// Returns the per-core slice and the (deduplicated) flags list.
func parseCPUInfo(scanner *bufio.Scanner, cpu *CPUInfo) ([]CoreInfo, []string, error) {
	var (
		cores   []CoreInfo
		flags   []string
		current *CoreInfo
	)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			if current != nil {
				cores = append(cores, *current)
				current = nil
			}
			continue
		}
		key, val, ok := splitCPUInfoLine(line)
		if !ok {
			// Malformed line: skip but continue. Callers can enable
			// per-line error reporting via a future audit hook.
			continue
		}
		if err := applyCPUInfoField(cpu, &current, &flags, key, val, lineNum); err != nil {
			return nil, nil, err
		}
	}
	if current != nil {
		cores = append(cores, *current)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scan cpuinfo: %w", err)
	}
	if len(cores) == 0 {
		return nil, nil, errors.New("cpuinfo: no processor blocks found")
	}
	return cores, flags, nil
}

// applyCPUInfoField dispatches one parsed key/value to the right place
// on the accumulator. Extracted from parseCPUInfo so the inner loop
// doesn't carry a large switch statement.
func applyCPUInfoField(cpu *CPUInfo, current **CoreInfo, flags *[]string, key, val string, lineNum int) error {
	switch key {
	case "processor":
		if *current != nil {
			// Caller handles the append at block boundaries; finishing
			// the previous block early is only needed when cpuinfo
			// omits the blank line. /proc/cpuinfo always has a blank
			// line between blocks so we don't emit the append here.
			return nil
		}
		n, perr := strconv.Atoi(val)
		if perr != nil {
			return fmt.Errorf("cpuinfo line %d: processor %q: %w", lineNum, val, perr)
		}
		*current = &CoreInfo{CPU: n}
	case "vendor_id":
		if cpu.Vendor == CPUVendorUnknown {
			cpu.Vendor = parseCPUVendor(val)
		}
	case "cpu family":
		if cpu.Family == 0 {
			cpu.Family, _ = strconv.Atoi(val)
		}
	case "model":
		if cpu.Model == 0 {
			cpu.Model, _ = strconv.Atoi(val)
		}
	case "stepping":
		if cpu.Stepping == 0 {
			cpu.Stepping, _ = strconv.Atoi(val)
		}
	case "model name":
		if cpu.ModelName == "" {
			cpu.ModelName = val
		}
	case "microcode":
		if cpu.Microcode == "" {
			cpu.Microcode = val
		}
	case "core id":
		if *current != nil {
			(*current).CoreID, _ = strconv.Atoi(val)
		}
	case "physical id":
		if *current != nil {
			(*current).PhysicalPackage, _ = strconv.Atoi(val)
		}
	case "flags":
		if len(*flags) == 0 {
			*flags = strings.Fields(val)
		}
	}
	return nil
}

// splitCPUInfoLine parses one "key : value" line. Returns false for
// malformed lines so the caller can skip them.
func splitCPUInfoLine(line string) (key, val string, ok bool) {
	k, v, found := strings.Cut(line, ":")
	if !found {
		return "", "", false
	}
	key = strings.TrimSpace(k)
	val = strings.TrimSpace(v)
	if key == "" {
		return "", "", false
	}
	return key, val, true
}

// parseCPUVendor maps the /proc/cpuinfo `vendor_id` string to a typed
// CPUVendor. Unknown strings become CPUVendorOther to distinguish
// "detected but unrecognized" from "not detected yet" (Unknown).
func parseCPUVendor(s string) CPUVendor {
	if s == "" {
		return CPUVendorUnknown
	}
	if s == "GenuineIntel" {
		return CPUVendorIntel
	}
	if s == "AuthenticAMD" {
		return CPUVendorAMD
	}
	return CPUVendorOther
}

// hasFlag returns true if the flags list contains name.
func hasFlag(flags []string, name string) bool {
	return slices.Contains(flags, name)
}

// countPhysicalCores counts distinct (PhysicalPackage, CoreID) pairs.
func countPhysicalCores(cores []CoreInfo) int {
	seen := make(map[[2]int]struct{}, len(cores))
	for _, c := range cores {
		seen[[2]int{c.PhysicalPackage, c.CoreID}] = struct{}{}
	}
	return len(seen)
}

// averageCurrentFreqMHz averages the per-core current-freq readings,
// skipping zeros (unavailable).
func averageCurrentFreqMHz(cores []CoreInfo) int {
	var sum, n int
	for _, c := range cores {
		if c.CurrentFreqMHz > 0 {
			sum += c.CurrentFreqMHz
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

// fillCoreSysfs populates the sysfs-sourced fields on one CoreInfo:
// cpu_capacity, cpufreq/scaling_cur_freq, thermal_throttle counters.
// Missing files are skipped silently per AC-21.
func (d *Detector) fillCoreSysfs(c *CoreInfo) {
	base := d.sysfsPath("devices/system/cpu", fmt.Sprintf("cpu%d", c.CPU))
	c.Capacity = readFileInt(filepath.Join(base, "cpu_capacity"))
	c.CurrentFreqMHz = khzToMHz(readFileInt(filepath.Join(base, "cpufreq", "scaling_cur_freq")))
	//nolint:gosec // throttle counts are small non-negative integers
	c.CoreThrottleCount = uint64(readFileInt(filepath.Join(base, "thermal_throttle", "core_throttle_count")))
	//nolint:gosec // throttle counts are small non-negative integers
	c.PackageThrottleCount = uint64(readFileInt(filepath.Join(base, "thermal_throttle", "package_throttle_count")))
}

// readScalingDriver reads /sys/devices/system/cpu/cpu<N>/cpufreq/scaling_driver
// and maps to ScalingDriver.
func (d *Detector) readScalingDriver(cpu int) ScalingDriver {
	path := d.sysfsPath("devices/system/cpu", fmt.Sprintf("cpu%d", cpu), "cpufreq/scaling_driver")
	b, err := os.ReadFile(path) //nolint:gosec // path is under /sys root
	if err != nil {
		return ScalingDriverUnknown
	}
	name := strings.TrimSpace(string(b))
	if name == "" {
		return ScalingDriverUnknown
	}
	if name == "intel_pstate" {
		return ScalingDriverIntelPState
	}
	if name == "amd-pstate" || name == "amd_pstate" || name == "amd-pstate-epp" || name == "amd-pstate-guided" {
		return ScalingDriverAMDPState
	}
	if name == "acpi-cpufreq" || name == "acpi_cpufreq" {
		return ScalingDriverACPICpufreq
	}
	return ScalingDriverOther
}

// readCPUFreqInt reads a kHz-valued integer file under
// /sys/devices/system/cpu/cpu<N>/cpufreq/. Returns 0 if unreadable.
func (d *Detector) readCPUFreqInt(cpu int, leaf string) int {
	path := d.sysfsPath("devices/system/cpu", fmt.Sprintf("cpu%d", cpu), "cpufreq", leaf)
	return readFileInt(path)
}

// readFileInt reads a newline-terminated integer file and returns its
// value; returns 0 on any error (missing file, bad content).
func readFileInt(path string) int {
	b, err := os.ReadFile(path) //nolint:gosec // caller-scoped sysfs/procfs paths
	if err != nil {
		return 0
	}
	v, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return v
}

// khzToMHz divides by 1000, returning 0 for 0 input.
func khzToMHz(khz int) int {
	if khz <= 0 {
		return 0
	}
	return khz / 1000
}

// classifyHybrid inspects cpu_capacity across cores and fills per-core
// Role plus package-level Hybrid. Cores with unknown capacity stay
// CoreRoleUnknown.
func classifyHybrid(cpu *CPUInfo) {
	capacities := collectCapacities(cpu.Cores)
	if len(capacities) == 0 {
		assignRoles(cpu.Cores, CoreRoleUniform)
		cpu.Hybrid = false
		return
	}
	sort.Ints(capacities)
	minCap := capacities[0]
	maxCap := capacities[len(capacities)-1]
	if minCap == maxCap {
		assignRoles(cpu.Cores, CoreRoleUniform)
		cpu.Hybrid = false
		return
	}
	cpu.Hybrid = true
	// Two-cluster split: anything at maxCap is performance, anything
	// strictly below is efficient. Real hardware has exactly two
	// capacity levels on Alder Lake and Meteor Lake.
	for i := range cpu.Cores {
		cpu.Cores[i].Role = roleForCapacity(cpu.Cores[i].Capacity, maxCap)
	}
}

// collectCapacities returns the non-zero capacity values across cores.
func collectCapacities(cores []CoreInfo) []int {
	out := make([]int, 0, len(cores))
	for _, c := range cores {
		if c.Capacity > 0 {
			out = append(out, c.Capacity)
		}
	}
	return out
}

// assignRoles sets every core to the same role.
func assignRoles(cores []CoreInfo, role CoreRole) {
	for i := range cores {
		cores[i].Role = role
	}
}

// roleForCapacity picks the role for one core given the package-wide
// maxCap. Zero capacity means the cpu_capacity file was missing.
func roleForCapacity(cap, maxCap int) CoreRole {
	if cap == 0 {
		return CoreRoleUnknown
	}
	if cap >= maxCap {
		return CoreRolePerformance
	}
	return CoreRoleEfficient
}

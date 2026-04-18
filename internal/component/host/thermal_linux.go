// Design: plan/spec-host-0-inventory.md — hardware inventory detection

//go:build linux

package host

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// DetectThermal aggregates hwmon temperature readings and per-CPU
// thermal_throttle counters. Sensors with no temp*_input file are
// skipped (some hwmon nodes only expose fan RPM or voltage).
func (d *Detector) DetectThermal() (*ThermalInfo, error) {
	sensors, err := d.readHwmonSensors()
	if err != nil && !errors.Is(err, ErrUnsupported) {
		return nil, err
	}
	throttle := d.readThrottleCounters()
	return &ThermalInfo{Sensors: sensors, Throttle: throttle}, nil
}

// readHwmonSensors walks /sys/class/hwmon/hwmonN and returns one
// SensorReading per tempM_input file. hwmon nodes may expose multiple
// temp channels (e.g. coretemp exposes one per core); each becomes a
// separate entry.
func (d *Detector) readHwmonSensors() ([]SensorReading, error) {
	base := d.sysfsPath("class/hwmon")
	entries, err := os.ReadDir(base)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", base, err)
	}
	var readings []SensorReading
	for _, e := range entries {
		readings = append(readings, d.readOneHwmon(filepath.Join(base, e.Name()))...)
	}
	sort.Slice(readings, func(i, j int) bool {
		if readings[i].Name != readings[j].Name {
			return readings[i].Name < readings[j].Name
		}
		return readings[i].Device < readings[j].Device
	})
	return readings, nil
}

// readOneHwmon collects all tempN_input readings under one hwmon node.
// Each temp channel may have a sibling tempN_label describing what it
// reads (e.g. "Core 0", "Tctl"); we append the label to the device
// name for disambiguation.
func (d *Detector) readOneHwmon(dir string) []SensorReading {
	name := readFileString(filepath.Join(dir, "name"))
	if name == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []SensorReading
	for _, e := range entries {
		fn := e.Name()
		if !strings.HasPrefix(fn, "temp") || !strings.HasSuffix(fn, "_input") {
			continue
		}
		idx := strings.TrimSuffix(strings.TrimPrefix(fn, "temp"), "_input")
		mc := readFileInt(filepath.Join(dir, fn))
		if mc == 0 {
			// Missing or unreadable: skip.
			continue
		}
		label := readFileString(filepath.Join(dir, "temp"+idx+"_label"))
		alarm := readFileInt(filepath.Join(dir, "temp"+idx+"_alarm")) != 0
		out = append(out, SensorReading{
			Name:   name,
			Device: strings.TrimSpace(label),
			TempMC: int64(mc),
			Alarm:  alarm,
		})
	}
	return out
}

// readThrottleCounters walks /sys/devices/system/cpu/cpu*/thermal_throttle/
// collecting per-CPU core/package counters. The CPU index is extracted
// from the directory name (`cpuN`).
func (d *Detector) readThrottleCounters() []ThrottleEntry {
	base := d.sysfsPath("devices/system/cpu")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var out []ThrottleEntry
	for _, e := range entries {
		cpu, ok := parseCPUIndex(e.Name())
		if !ok {
			continue
		}
		tt := filepath.Join(base, e.Name(), "thermal_throttle")
		if _, err := os.Stat(tt); err != nil {
			continue
		}
		out = append(out, ThrottleEntry{
			CPU:                  cpu,
			CoreThrottleCount:    uint64(readFileInt(filepath.Join(tt, "core_throttle_count"))),    //nolint:gosec // kernel counter
			PackageThrottleCount: uint64(readFileInt(filepath.Join(tt, "package_throttle_count"))), //nolint:gosec // kernel counter
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CPU < out[j].CPU })
	return out
}

// parseCPUIndex returns the N from a `cpuN` directory name, rejecting
// anything else (`cpuidle`, `cpufreq`, etc.).
func parseCPUIndex(name string) (int, bool) {
	if !strings.HasPrefix(name, "cpu") {
		return 0, false
	}
	s := strings.TrimPrefix(name, "cpu")
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

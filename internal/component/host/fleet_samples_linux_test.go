// Design: docs/architecture/fleet-config.md — unit tests for host fleet metric sampling

//go:build linux

package host

import (
	"errors"
	"math"
	"testing"

	"github.com/ze-software/ze/pkg/fleet"
)

func readingMap(t *testing.T, snap *fleet.SourceSnapshot) map[string]fleet.SourceReading {
	t.Helper()
	if err := fleet.ValidateSourceSnapshot(fleet.SampleOwnerHost, snap); err != nil {
		t.Fatalf("ValidateSourceSnapshot failed: %v", err)
	}
	out := make(map[string]fleet.SourceReading, len(snap.Readings))
	for _, r := range snap.Readings {
		out[r.Figure] = r
	}
	return out
}

// VALIDATES: first tick skips cpu-used while emitting memory, root storage,
// load-1m, and uptime with canonical units; second tick computes CPU delta %;
// counter rollback or non-advancing total resets the baseline and omits
// cpu-used until a subsequent advancing tick.
// PREVENTS: fabricated 0% CPU on first tick or negative/NaN CPU on counter reset.
func TestFleetSamplerCPUBaselineAndAllFigures(t *testing.T) {
	statTicks := []string{
		"cpu  100 0 100 800 0 0 0 0 0 0\ncpu0 100 0 100 800 0 0 0 0 0 0\n",
		"cpu  150 0 150 900 0 0 0 0 0 0\n", // deltaTotal=200, deltaBusy=100 -> 50%
		"cpu  150 0 150 900 0 0 0 0 0 0\n", // non-advancing total -> reset/omit
		"cpu  100 0 100 700 0 0 0 0 0 0\n", // rollback -> reset/omit
		"cpu  125 0 100 775 0 0 0 0 0 0\n", // deltaTotal=100, deltaBusy=25 -> 25%
	}
	statIdx := 0

	sampler := &FleetSampler{
		readStat: func() ([]byte, error) {
			s := statTicks[statIdx]
			statIdx++
			return []byte(s), nil
		},
		readMeminfo: func() ([]byte, error) {
			return []byte("MemTotal:       16384 kB\nMemFree:         2048 kB\nMemAvailable:    6144 kB\nBuffers:          512 kB\nCached:          1024 kB\n"), nil
		},
		readLoadavg: func() ([]byte, error) {
			return []byte("1.25 0.90 0.50 1/200 12345\n"), nil
		},
		readUptime: func() ([]byte, error) {
			return []byte("3600.50 7200.00\n"), nil
		},
		statRootFS: func() (rootFSStat, error) {
			return rootFSStat{blocks: 1000, bfree: 250, bsize: 4096}, nil
		},
	}

	// Tick 1: cpu-used omitted; all other figures present with exact units.
	snap1 := sampler.Sample()
	m1 := readingMap(t, &snap1)
	if _, hasCPU := m1[fleet.FigureCPUUsed]; hasCPU {
		t.Fatalf("first tick emitted cpu-used %+v, want omitted", m1[fleet.FigureCPUUsed])
	}
	if got := m1[fleet.FigureMemoryTotal].Value; got != 16384*1024 {
		t.Errorf("memory-total = %v, want %v", got, float64(16384*1024))
	}
	if got := m1[fleet.FigureMemoryUsed].Value; got != (16384-6144)*1024 {
		t.Errorf("memory-used = %v, want %v", got, float64((16384-6144)*1024))
	}
	if got := m1[fleet.FigureStorageTotal].Value; got != 1000*4096 {
		t.Errorf("storage-total = %v, want %v", got, float64(1000*4096))
	}
	if got := m1[fleet.FigureStorageUsed].Value; got != 750*4096 {
		t.Errorf("storage-used = %v, want %v", got, float64(750*4096))
	}
	if got := m1[fleet.FigureLoad1m].Value; got != 1.25 {
		t.Errorf("load-1m = %v, want 1.25", got)
	}
	if got := m1[fleet.FigureUptime].Value; got != 3600.50 {
		t.Errorf("uptime = %v, want 3600.50", got)
	}

	// Tick 2: cpu-used = 50%.
	snap2 := sampler.Sample()
	m2 := readingMap(t, &snap2)
	cpu2, ok := m2[fleet.FigureCPUUsed]
	if !ok {
		t.Fatal("second tick omitted cpu-used")
	}
	if math.Abs(cpu2.Value-50.0) > 1e-9 {
		t.Errorf("second tick cpu-used = %v, want 50", cpu2.Value)
	}

	// Tick 3: non-advancing total omits cpu-used.
	snap3 := sampler.Sample()
	m3 := readingMap(t, &snap3)
	if _, hasCPU := m3[fleet.FigureCPUUsed]; hasCPU {
		t.Fatalf("non-advancing tick emitted cpu-used %+v, want omitted", m3[fleet.FigureCPUUsed])
	}

	// Tick 4: rollback resets baseline and omits cpu-used.
	snap4 := sampler.Sample()
	m4 := readingMap(t, &snap4)
	if _, hasCPU := m4[fleet.FigureCPUUsed]; hasCPU {
		t.Fatalf("rollback tick emitted cpu-used %+v, want omitted", m4[fleet.FigureCPUUsed])
	}

	// Tick 5: advancing tick after rollback reports 25%.
	snap5 := sampler.Sample()
	m5 := readingMap(t, &snap5)
	cpu5, ok := m5[fleet.FigureCPUUsed]
	if !ok {
		t.Fatal("post-rollback advancing tick omitted cpu-used")
	}
	if math.Abs(cpu5.Value-25.0) > 1e-9 {
		t.Errorf("post-rollback cpu-used = %v, want 25", cpu5.Value)
	}
}

// VALIDATES: when MemAvailable is absent from /proc/meminfo, memory-used falls
// back to MemTotal - MemFree - Buffers - Cached clamped >= 0; read failures
// omit their figures rather than synthesizing zeros.
// PREVENTS: synthetic zeros on unreadable procfs/statfs or negative memory-used.
func TestFleetSamplerMemFallbackAndReadErrors(t *testing.T) {
	errBoom := errors.New("read failed")

	t.Run("meminfo-fallback-without-memavailable", func(t *testing.T) {
		sampler := &FleetSampler{
			readStat: func() ([]byte, error) { return nil, errBoom },
			readMeminfo: func() ([]byte, error) {
				return []byte("MemTotal: 10000 kB\nMemFree: 2000 kB\nBuffers: 500 kB\nCached: 1500 kB\n"), nil
			},
			readLoadavg: func() ([]byte, error) { return nil, errBoom },
			readUptime:  func() ([]byte, error) { return nil, errBoom },
			statRootFS:  func() (rootFSStat, error) { return rootFSStat{}, errBoom },
		}
		snap := sampler.Sample()
		m := readingMap(t, &snap)
		if len(m) != 2 {
			t.Fatalf("got %d readings (%+v), want only memory-used and memory-total", len(m), m)
		}
		if got := m[fleet.FigureMemoryTotal].Value; got != 10000*1024 {
			t.Errorf("memory-total = %v, want %v", got, float64(10000*1024))
		}
		if got := m[fleet.FigureMemoryUsed].Value; got != 6000*1024 {
			t.Errorf("memory-used = %v, want %v", got, float64(6000*1024))
		}
	})

	t.Run("meminfo-clamped-at-zero-when-available-exceeds-total", func(t *testing.T) {
		sampler := &FleetSampler{
			readMeminfo: func() ([]byte, error) {
				return []byte("MemTotal: 1000 kB\nMemAvailable: 2000 kB\n"), nil
			},
		}
		snap := sampler.Sample()
		m := readingMap(t, &snap)
		if got := m[fleet.FigureMemoryUsed].Value; got != 0 {
			t.Errorf("memory-used = %v, want 0", got)
		}
	})

	t.Run("all-errors-emit-empty-valid-snapshot", func(t *testing.T) {
		sampler := &FleetSampler{
			readStat:    func() ([]byte, error) { return nil, errBoom },
			readMeminfo: func() ([]byte, error) { return nil, errBoom },
			readLoadavg: func() ([]byte, error) { return nil, errBoom },
			readUptime:  func() ([]byte, error) { return nil, errBoom },
			statRootFS:  func() (rootFSStat, error) { return rootFSStat{}, errBoom },
		}
		snap := sampler.Sample()
		m := readingMap(t, &snap)
		if len(m) != 0 {
			t.Fatalf("got %d readings on total failure, want 0: %+v", len(m), m)
		}
	})
}

// VALIDATES: SampleFleetMetrics runs cleanly against the live Linux host and
// produces a SourceSnapshot accepted by fleet.ValidateSourceSnapshot.
// PREVENTS: broken live procfs/statfs paths on Linux.
func TestSampleFleetMetricsLiveLinuxValid(t *testing.T) {
	snap := NewFleetSampler().Sample()
	if err := fleet.ValidateSourceSnapshot(fleet.SampleOwnerHost, &snap); err != nil {
		t.Fatalf("live SampleFleetMetrics invalid: %v", err)
	}
	if len(snap.Readings) == 0 {
		t.Fatal("live Linux sampler returned zero readings")
	}
}

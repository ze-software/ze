// VALIDATES: the pressure collector parses the PSI files under /proc/pressure
// and emits the some/full avg10/avg60/avg300 percentages for cpu, memory and io.
// PREVENTS: mis-parsed or mis-labeled PSI figures (e.g. swapping some/full or the
// averaging window) on the metrics surface.

//go:build linux

package collector

import "testing"

func TestPressureCollect(t *testing.T) {
	fs := procFixture(t, map[string]string{
		"pressure/cpu":    "some avg10=0.10 avg60=2.00 avg300=3.85 total=15\n",
		"pressure/memory": "some avg10=0.10 avg60=2.00 avg300=3.85 total=15\nfull avg10=0.20 avg60=3.00 avg300=4.95 total=25\n",
		"pressure/io":     "some avg10=0.10 avg60=2.00 avg300=3.85 total=15\nfull avg10=0.20 avg60=3.00 avg300=4.95 total=25\n",
	})
	reg := newRecordingRegistry()
	c := newPressureCollector(fs)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	const cpuSome = "netdata_system_cpu_some_pressure_percentage_average"
	if got := reg.gauge(t, cpuSome, "system.cpu_some_pressure", "some 10", "cpu"); got != 0.10 {
		t.Errorf("cpu some10 = %v, want 0.10", got)
	}
	if got := reg.gauge(t, cpuSome, "system.cpu_some_pressure", "some 300", "cpu"); got != 3.85 {
		t.Errorf("cpu some300 = %v, want 3.85", got)
	}

	if got := reg.gauge(t, "netdata_system_memory_full_pressure_percentage_average", "system.memory_full_pressure", "full 60", "ram"); got != 3.00 {
		t.Errorf("mem full60 = %v, want 3.00", got)
	}
	if got := reg.gauge(t, "netdata_system_io_full_pressure_percentage_average", "system.io_full_pressure", "full 300", "disk"); got != 4.95 {
		t.Errorf("io full300 = %v, want 4.95", got)
	}
}

// VALIDATES: the vmstat collector parses /proc/vmstat key/value pairs and emits
// page-fault, paging-I/O and swap-I/O per-second rates across two collects (path
// injected via the c.path seam).
// PREVENTS: a broken key lookup or delta on the mem.pgfaults / system.pgpgio /
// mem.swapio metrics.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func TestVMStatCollect(t *testing.T) {
	const first = "pgfault 100\npgmajfault 10\npgpgin 50\npgpgout 20\npswpin 1\npswpout 2\n"
	// deltas: pgfault 200, pgmajfault 5, pgpgin 40, pswpin 9
	const second = "pgfault 300\npgmajfault 15\npgpgin 90\npgpgout 60\npswpin 10\npswpout 2\n"

	dir, path := tmpFile(t, "vmstat", first)
	c := newVMStatCollector(time.Second)
	c.path = path
	reg := newRecordingRegistry()
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	writeProcFile(t, dir, "vmstat", second)
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}

	const pgf = "netdata_mem_pgfaults_faults_persec_average"
	if got := reg.gauge(t, pgf, "mem.pgfaults", "minor", "mem"); got != 200 {
		t.Errorf("minor faults = %v, want 200", got)
	}
	if got := reg.gauge(t, pgf, "mem.pgfaults", "major", "mem"); got != 5 {
		t.Errorf("major faults = %v, want 5", got)
	}
	if got := reg.gauge(t, "netdata_system_pgpgio_KiB_persec_average", "system.pgpgio", "in", "disk"); got != 40 {
		t.Errorf("pgpgin = %v, want 40", got)
	}
	if got := reg.gauge(t, "netdata_mem_swapio_KiB_persec_average", "mem.swapio", "in", "swap"); got != 9 {
		t.Errorf("pswpin = %v, want 9", got)
	}
}

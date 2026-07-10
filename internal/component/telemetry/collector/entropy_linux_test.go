// VALIDATES: the entropy collector reads the kernel random pool's available
// entropy and emits it on the system.entropy gauge.
// PREVENTS: a mis-read entropy value (or a silently dropped one) on the metrics
// surface.

//go:build linux

package collector

import "testing"

func TestEntropyCollect(t *testing.T) {
	fs := procFixture(t, map[string]string{
		"sys/kernel/random/entropy_avail": "3072\n",
		"sys/kernel/random/poolsize":      "4096\n",
	})
	reg := newRecordingRegistry()
	c := newEntropyCollector(fs)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	got := reg.gauge(t, "netdata_system_entropy_entropy_average", "system.entropy", "entropy", "entropy")
	if got != 3072 {
		t.Errorf("entropy = %v, want 3072", got)
	}
}

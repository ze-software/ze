package iface

import "testing"

// VALIDATES: ze_iface_config_apply_started_total is registered under that exact name
// and countConfigApply increments it.
//
// PREVENTS: the instrument gap that made a red functional test cost three QEMU
// boots on 2026-09-03. Nothing in this component published whether a config
// apply had happened, so "the reload no longer holds the lock" could be
// believed for hours without anything able to contradict it. A counter that is
// registered under a different name, or that no call site increments, is the
// same gap with a metric-shaped hole in it.
func TestConfigAppliesCounterIsRegisteredAndCounts(t *testing.T) {
	reg := bindCapturingMetrics(t)

	counter, ok := reg.counters["ze_iface_config_apply_started_total"]
	if !ok {
		t.Fatalf("ze_iface_config_apply_started_total is not registered; registry has %v", counterNames(reg))
	}
	if got := counter.get(); got != 0 {
		t.Fatalf("a fresh counter reads %v, want 0", got)
	}

	countConfigApply()
	countConfigApply()

	if got := counter.get(); got != 2 {
		t.Errorf("after two applies the counter reads %v, want 2", got)
	}
}

// VALIDATES: countConfigApply is safe before any registry is bound, which is
// the state a daemon is in until ConfigureMetrics runs.
func TestConfigAppliesCounterSurvivesAnUnboundRegistry(t *testing.T) {
	ifaceMetricsPtr.Store(nil)
	countConfigApply()
}

func counterNames(reg *capturingGaugeRegistry) []string {
	names := make([]string, 0, len(reg.counters))
	for name := range reg.counters {
		names = append(names, name)
	}
	return names
}

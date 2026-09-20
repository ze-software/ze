// Design: docs/architecture/plugin/plugin-system.md -- verify-to-apply seam
// Related: register.go (pendingNTPConfig, applyStagedConfig)

package ntp

import (
	"errors"
	"testing"
)

// TestApplyStagedConfigStopsTheWorkerWhenTheBlockIsRemoved proves that
// deleting the `environment ntp` block reaches the worker.
//
// VALIDATES: the verify stages the parsed default for a removal, and
// applyStagedConfig hands it to startWorker, whose Enabled false branch stops
// the sync worker.
// PREVENTS: a sync worker outliving the config that asked for it, still
// querying the deleted servers and still stepping the system clock.
func TestApplyStagedConfigStopsTheWorkerWhenTheBlockIsRemoved(t *testing.T) {
	var pending pendingNTPConfig

	// What OnConfigVerify stages for the empty body a removal delivers:
	// parseNTPConfig finds no `environment ntp` and returns the defaults.
	removed, err := parseNTPConfig("{}")
	if err != nil {
		t.Fatalf("parseNTPConfig: %v", err)
	}
	if removed.Enabled {
		t.Fatal("setup: a removed ntp block must parse to Enabled false, or the apply cannot stop the worker")
	}
	pending.stage(removed)

	var started []ntpConfig
	if err := applyStagedConfig(&pending, func(cfg ntpConfig) { started = append(started, cfg) }); err != nil {
		t.Fatalf("applyStagedConfig: %v", err)
	}
	if len(started) != 1 {
		t.Fatalf("a removal must reach the worker exactly once, got %d calls", len(started))
	}
	if started[0].Enabled {
		t.Error("a removal must hand the worker a disabled config, which is what stops it")
	}
}

// TestApplyStagedConfigRefusesAnApplyWithoutAVerify proves the apply fails
// closed when no verify staged a candidate.
//
// VALIDATES: the `staged` fact, which the config pointer alone could not
// carry.
// PREVENTS: an apply that reaches the plugin out of protocol leaving the
// running worker in place and reporting success over config it never
// installed, with nothing logged at any level to say so
// (ai/rules/principles.md -- a value that is silently wrong must not be
// reachable).
func TestApplyStagedConfigRefusesAnApplyWithoutAVerify(t *testing.T) {
	var pending pendingNTPConfig

	calls := 0
	err := applyStagedConfig(&pending, func(ntpConfig) { calls++ })
	if !errors.Is(err, errApplyWithoutVerify) {
		t.Errorf("an apply with nothing staged must be refused, got %v", err)
	}
	if calls != 0 {
		t.Errorf("an apply with nothing staged must not touch the worker, called it %d times", calls)
	}
}

// TestApplyStagedConfigUnstagesTheCandidate proves one verify arms exactly one
// apply.
//
// VALIDATES: take unstages, so a second apply behind one verify reaches the
// fail-closed branch instead of restarting the worker.
// PREVENTS: a stale candidate restarting the sync worker on config no commit
// installed.
func TestApplyStagedConfigUnstagesTheCandidate(t *testing.T) {
	var pending pendingNTPConfig
	pending.stage(ntpConfig{Enabled: true, IntervalSec: 3600, Servers: []string{"time.example"}})

	calls := 0
	if err := applyStagedConfig(&pending, func(ntpConfig) { calls++ }); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if err := applyStagedConfig(&pending, func(ntpConfig) { calls++ }); !errors.Is(err, errApplyWithoutVerify) {
		t.Errorf("a second apply behind one verify must be refused, got %v", err)
	}
	if calls != 1 {
		t.Errorf("one verify must arm exactly one start, got %d", calls)
	}
}

// TestPendingNTPConfigClearUnstagesTheCandidate proves a rolled-back
// transaction does not arm the next apply.
//
// VALIDATES: pendingNTPConfig.clear, which the OnConfigRollback handler calls.
// PREVENTS: the candidate from a transaction that failed elsewhere being
// applied by a later apply. The plugin had no rollback handler at all before
// this, so the candidate stayed armed.
func TestPendingNTPConfigClearUnstagesTheCandidate(t *testing.T) {
	var pending pendingNTPConfig
	pending.stage(ntpConfig{Enabled: true})
	pending.clear()

	if cfg, staged := pending.take(); staged || cfg.Enabled {
		t.Errorf("clear must unstage the candidate, got cfg=%+v staged=%v", cfg, staged)
	}
}

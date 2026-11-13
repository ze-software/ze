// Design: docs/architecture/api/process-protocol.md -- configure-stage startup budgets.

package plugin

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStartupConfigureBudgetFreezesTierMaximum(t *testing.T) {
	coord := NewStartupCoordinator(3)
	for id, budget := range []time.Duration{2 * time.Second, 7 * time.Second, 0} {
		if err := coord.DeclareConfigureBudget(id, budget); err != nil {
			t.Fatal(err)
		}
	}
	const base = time.Second
	if got := coord.ConfigureStall(base); got != base {
		t.Fatalf("an incomplete registration tier changed its stall to %s", got)
	}
	coord.StageComplete(0, StageRegistration)
	coord.StageComplete(1, StageRegistration)
	if got := coord.ConfigureStall(base); got != base {
		t.Fatalf("an unfinished registration tier published %s", got)
	}
	coord.StageComplete(2, StageRegistration)
	if got := coord.ConfigureStall(base); got != 7*time.Second {
		t.Fatalf("configure stall = %s, want the tier maximum 7s", got)
	}
	if got := coord.ConfigureStall(10 * time.Second); got != 10*time.Second {
		t.Fatalf("declaration shortened the base stall to %s", got)
	}
	for id := range 3 {
		coord.StageComplete(id, StageConfig)
	}
	if got := coord.ConfigureStall(base); got != base {
		t.Fatalf("configure allowance escaped into Capability: %s", got)
	}
}

func TestStartupConfigureBudgetRefusalsPreserveDeclaration(t *testing.T) {
	coord := NewStartupCoordinator(2)
	if err := coord.DeclareConfigureBudget(0, 3*time.Second); err != nil {
		t.Fatal(err)
	}
	for _, refusal := range []struct {
		name   string
		id     int
		budget time.Duration
	}{
		{name: "duplicate", id: 0, budget: time.Hour},
		{name: "negative member", id: -1, budget: time.Hour},
		{name: "future member", id: 2, budget: time.Hour},
		{name: "negative duration", id: 1, budget: -time.Second},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			if err := coord.DeclareConfigureBudget(refusal.id, refusal.budget); err == nil {
				t.Error("invalid declaration was accepted")
			}
		})
	}
	if err := coord.DeclareConfigureBudget(1, 0); err != nil {
		t.Fatalf("refusal consumed the healthy member's declaration: %v", err)
	}
	coord.StageComplete(0, StageRegistration)
	coord.StageComplete(1, StageRegistration)
	if err := coord.DeclareConfigureBudget(1, time.Hour); err == nil {
		t.Error("a declaration after registration changed the frozen tier")
	}
	if got := coord.ConfigureStall(time.Second); got != 3*time.Second {
		t.Fatalf("a refusal replaced the accepted declaration: %s", got)
	}
}

func TestStartupConfigureBudgetPreservesDisabledStall(t *testing.T) {
	coord := configuredStartupBudget(t, time.Minute)
	coord.mu.Lock()
	coord.lastProgress = time.Now().Add(-time.Hour)
	coord.mu.Unlock()
	for _, disabled := range []time.Duration{0, -time.Second} {
		if got := coord.ConfigureStall(disabled); got != disabled {
			t.Fatalf("disabled stall %s became %s", disabled, got)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if err := coord.WaitForStageProgress(ctx, StageCapability, coord.ConfigureStall(disabled)); !errors.Is(err, context.Canceled) {
			t.Fatalf("disabled stall must wait for its context, got %v", err)
		}
	}
}

func TestStartupConfigureBudgetHonorsCancellation(t *testing.T) {
	coord := configuredStartupBudget(t, time.Minute)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := coord.WaitForStageProgress(ctx, StageCapability, coord.ConfigureStall(time.Second)); !errors.Is(err, context.Canceled) {
		t.Fatalf("declared configure allowance hid cancellation: %v", err)
	}
}

func TestStartupConfigureBudgetHonorsTierFailure(t *testing.T) {
	coord := configuredStartupBudget(t, time.Minute)
	coord.PluginFailed(0, "configure refused")
	if err := coord.WaitForStageProgress(t.Context(), StageCapability, coord.ConfigureStall(time.Second)); err == nil || !strings.Contains(err.Error(), "configure refused") {
		t.Fatalf("declared configure allowance hid the tier failure: %v", err)
	}
}

// TestStartupConfigureBudgetNamesTheTierFailure separates a sibling's failure
// from a refusal of the declaring plugin's own declaration.
//
// GOAL: a healthy plugin whose declaration arrives after another member failed
// the tier can tell that the failure was not its own, while every refusal of its
// own declaration cannot be mistaken for that.
// METHOD: one tier failed by member 0, then a valid declaration from member 1;
// separately, each own-declaration refusal on a tier nobody failed.
// VALIDATES: the first answer wraps ErrStartupTierFailed and carries the tier's
// failure; no own refusal wraps ErrStartupTierFailed.
// PREVENTS: the sibling's failure recorded as the healthy plugin's startup cause,
// which stopped the daemon through a fatal plugin that never failed.
func TestStartupConfigureBudgetNamesTheTierFailure(t *testing.T) {
	failed := NewStartupCoordinator(2)
	failed.PluginFailed(0, "configure refused")
	err := failed.DeclareConfigureBudget(1, time.Second)
	if !errors.Is(err, ErrStartupTierFailed) {
		t.Fatalf("a declaration after a sibling's failure answered %v, want ErrStartupTierFailed", err)
	}
	if got, want := err.Error(), "startup tier already failed: plugin 0 failed: configure refused"; got != want {
		t.Fatalf("tier failure answer = %q, want %q", got, want)
	}

	advanced := NewStartupCoordinator(1)
	if err := advanced.DeclareConfigureBudget(0, 0); err != nil {
		t.Fatal(err)
	}
	advanced.StageComplete(0, StageRegistration)
	duplicate := NewStartupCoordinator(2)
	if err := duplicate.DeclareConfigureBudget(0, 0); err != nil {
		t.Fatal(err)
	}
	for _, refusal := range []struct {
		name   string
		coord  *StartupCoordinator
		id     int
		budget time.Duration
	}{
		{name: "wrong stage", coord: advanced, id: 0, budget: time.Second},
		{name: "unknown member", coord: NewStartupCoordinator(1), id: 1, budget: time.Second},
		{name: "negative duration", coord: NewStartupCoordinator(1), id: 0, budget: -time.Second},
		{name: "duplicate", coord: duplicate, id: 0, budget: time.Second},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			err := refusal.coord.DeclareConfigureBudget(refusal.id, refusal.budget)
			if err == nil {
				t.Fatal("invalid declaration was accepted")
			}
			if errors.Is(err, ErrStartupTierFailed) {
				t.Fatalf("an own-declaration refusal reads as a tier failure: %v", err)
			}
		})
	}
}

func configuredStartupBudget(t *testing.T, budget time.Duration) *StartupCoordinator {
	t.Helper()
	coord := NewStartupCoordinator(1)
	if err := coord.DeclareConfigureBudget(0, budget); err != nil {
		t.Fatal(err)
	}
	coord.StageComplete(0, StageRegistration)
	if got := coord.ConfigureStall(time.Millisecond); got != budget {
		t.Fatalf("configure allowance = %s, want %s", got, budget)
	}
	return coord
}

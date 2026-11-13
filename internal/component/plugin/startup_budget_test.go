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

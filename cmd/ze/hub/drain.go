// Design: docs/architecture/hub-architecture.md -- hub stop orchestration
// Overview: main.go -- hub CLI entry point and stop path

package hub

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// drainCeiling bounds the whole stop drain, counted from the start of the
// stop, so a slow config fence spends from it. Declared once in rpc.
const drainCeiling = rpc.DrainCeilingSeconds * time.Second

// stopDrainDeadline is the earlier of the ceiling counted from the stop start
// and the largest declared budget counted from the drain start.
func stopDrainDeadline(stopStart, drainStart time.Time, budgetMax time.Duration) time.Time {
	ceiling := stopStart.Add(drainCeiling)
	own := drainStart.Add(budgetMax)
	if own.Before(ceiling) {
		return own
	}
	return ceiling
}

// runStopDrain runs the daemon-wide drain phase: after config changes are
// fenced and before any plugin is torn down, every plugin that registered a
// drain handler and a DrainBudget is asked, in parallel, to make its
// protocol unattractive. The hub names no protocol; the plugin server fans
// out over whoever registered. With no drain-registered plugin it returns at
// once, so the stop is as fast as before the drain existed.
func runStopDrain(stopStart time.Time, round *pluginserver.DrainRound, kind rpc.DrainKind) {
	if round.Plugins() == 0 {
		return
	}

	drainStart := time.Now()
	deadline := stopDrainDeadline(stopStart, drainStart, round.BudgetMax())
	if !deadline.After(drainStart) {
		slog.Warn("drain skipped: stop deadline passed during config fence",
			"ceiling", drainCeiling, "elapsed", drainStart.Sub(stopStart).Round(time.Millisecond))
		return
	}

	seconds := int(deadline.Sub(drainStart).Round(time.Second) / time.Second)
	fmt.Println("\nDraining routing protocols for up to", seconds, "s (Ctrl+C again to force)...")

	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	round.Run(ctx, rpc.DrainInput{Kind: kind, Deadline: deadline})
}

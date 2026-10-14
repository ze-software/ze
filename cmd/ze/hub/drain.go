// Design: docs/architecture/hub-architecture.md -- hub stop orchestration
// Overview: main.go -- hub CLI entry point and stop path

package hub

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
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

// stopRestartCause is set by the restart and reboot lifecycle callbacks
// (infra_setup.go, service_ssh.go) before they signal the stop, and read once
// by run after waitLoop returns. Nil means no callback asked for a restart, so
// the stop is DrainKindStop: SIGTERM, `request shutdown`, `quit`, SSH `stop`
// and a plugin failure all reach the hub with nothing set here. That nil is
// the guard that keeps them stop-kind.
var stopRestartCause atomic.Pointer[rpc.DrainRestartCause]

// requestRestartStop records that the coming stop is a restart, and why, so
// the drain RPC carries kind restart and the cause. The caller MUST call it
// before it signals the stop: run reads it once, after waitLoop returns.
func requestRestartStop(cause rpc.DrainRestartCause) {
	stopRestartCause.Store(&cause)
}

// requestRebootStop is the reboot lifecycle callback's body, shared by
// `request reboot` (infra_setup.go) and SSH `reboot` (service_ssh.go). It
// records the two facts run reads after waitLoop returns, the OS reboot
// (rebootRequested) and the drain's restart cause, before stopForRestart stops
// BGP in silence. The caller signals the stop afterwards.
func requestRebootStop(stopForRestart func()) {
	rebootRequested.Store(true)
	requestRestartStop(rpc.DrainRestartReboot)
	stopForRestart()
}

// stopDrainInput answers the stop kind, and on a restart its cause, that the
// drain RPC carries. runStopDrain sets the deadline.
func stopDrainInput() rpc.DrainInput {
	cause := stopRestartCause.Load()
	if cause == nil {
		return rpc.DrainInput{Kind: rpc.DrainKindStop}
	}
	return rpc.DrainInput{Kind: rpc.DrainKindRestart, RestartCause: *cause}
}

// drainRound is the stop drain the plugin server prepared
// (pluginserver.Server.BeginDrain). The hub reads its size and largest budget
// to set the deadline, then runs it once.
type drainRound interface {
	Plugins() int
	BudgetMax() time.Duration
	Run(ctx context.Context, input rpc.DrainInput) pluginserver.DrainResult
}

var _ drainRound = (*pluginserver.DrainRound)(nil)

// stopPhase is the part of the stop a signal arrives in, which decides what
// the signal does (stopSignals.onSignal).
type stopPhase uint8

const (
	stopPhaseUnspecified stopPhase = iota
	// stopPhaseFence is the reload worker, the confirm window and
	// commitReloads.close().
	stopPhaseFence
	// stopPhaseDrain is the drain RPC round.
	stopPhaseDrain
	// stopPhaseTeardown is gNMI, the API servers, the plugins and the engine.
	stopPhaseTeardown
)

// The lines printed on entering a phase. Each states what the next signal does.
const (
	stopLineFence    = "\nStopping: finishing config changes (Ctrl+C again to skip the drain)..."
	stopLineTeardown = "\nShutting down (Ctrl+C again to force)..."
)

// stopSignals owns the daemon's stop signals from waitLoop's return to exit,
// and follows the signal state table of spec-daemon-drain-before-stop: a
// signal before the drain has finished skips it (during the fence) or aborts
// it (during the drain), and the ordered teardown continues; any later signal
// forces exit 1. "Signal" means an OS SIGINT or SIGTERM: the value
// `request shutdown` injects into stopCh never reaches it, so after
// `request shutdown` the operator's first OS signal is the one counted.
//
// The hub MUST create it with newStopSignals when the stop starts, start one
// goroutine on run, then pass it to runStopDrain and call enterTeardown, in
// that order. Safe for concurrent use.
type stopSignals struct {
	mu sync.Mutex
	// phase is where the stop is now.
	phase stopPhase
	// drainOver is true once the drain finished, was skipped or was aborted:
	// from then on a signal forces exit.
	drainOver bool
	// teardownAnnounced keeps stopLineTeardown to one print.
	teardownAnnounced bool
	// cancelDrain aborts the running drain round; nil outside stopPhaseDrain.
	cancelDrain context.CancelFunc
	// forceExit ends the process with exit 1 (exitForced in production).
	forceExit func()
}

// newStopSignals enters the fence phase and prints its line.
func newStopSignals(forceExit func()) *stopSignals {
	fmt.Println(stopLineFence)
	return &stopSignals{phase: stopPhaseFence, forceExit: forceExit}
}

// run reads signals until the process exits. It is the channel's one reader,
// and the loop has no end because the exit ends it.
func (s *stopSignals) run(sigCh <-chan os.Signal) {
	for range sigCh {
		s.onSignal()
	}
}

// onSignal applies one signal to the phase the stop is in.
func (s *stopSignals) onSignal() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.drainOver {
		s.forceExit()
		return
	}
	switch s.phase {
	case stopPhaseFence:
		s.drainOver = true
		slog.Warn("drain skipped by signal")
		s.announceTeardown()
	case stopPhaseDrain:
		s.drainOver = true
		// DrainRound.Run logs "drain aborted by signal" when it sees the
		// cancellation, naming the plugins it stopped waiting for.
		s.cancelDrain()
		s.announceTeardown()
	case stopPhaseTeardown, stopPhaseUnspecified:
		// enterTeardown sets drainOver, so teardown is handled above; an
		// unspecified phase is a stopSignals not built by newStopSignals.
		s.forceExit()
	}
}

// beginDrain moves the stop into the drain phase with the round's cancel
// function, or answers false when a signal during the fence already skipped
// the drain. The caller MUST NOT send any drain RPC on false.
func (s *stopSignals) beginDrain(cancel context.CancelFunc) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.drainOver {
		return false
	}
	s.phase = stopPhaseDrain
	s.cancelDrain = cancel
	return true
}

// enterTeardown ends the drain phase, whatever ended it, and prints the
// teardown line unless a skip or an abort already did. From here a signal
// forces exit.
func (s *stopSignals) enterTeardown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.phase = stopPhaseTeardown
	s.drainOver = true
	s.cancelDrain = nil
	s.announceTeardown()
}

// announceTeardown prints the teardown line once. s.mu MUST be held.
func (s *stopSignals) announceTeardown() {
	if s.teardownAnnounced {
		return
	}
	s.teardownAnnounced = true
	fmt.Println(stopLineTeardown)
}

// runStopDrain runs the daemon-wide drain phase: after config changes are
// fenced and before any plugin is torn down, every plugin that registered a
// drain handler and a DrainBudget is asked, in parallel, to make its
// protocol unattractive. The hub names no protocol; the plugin server fans
// out over whoever registered. With no drain-registered plugin it returns at
// once, so the stop is as fast as before the drain existed. A signal during
// the fence skips it (signals.beginDrain answers false), and a signal during
// the round cancels the round's context.
func runStopDrain(stopStart time.Time, round drainRound, input rpc.DrainInput, signals *stopSignals) {
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

	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	if !signals.beginDrain(cancel) {
		return
	}

	seconds := int(deadline.Sub(drainStart).Round(time.Second) / time.Second)
	fmt.Println("\nDraining routing protocols for up to", seconds, "s (Ctrl+C again to stop now)...")

	input.Deadline = deadline
	round.Run(ctx, input)
}

// Design: docs/architecture/api/process-protocol.md — the stop drain RPC
// Overview: server.go — plugin server lifecycle
// Related: failure_policy.go — superviseProcess, which owns each drain caller

package server

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// drainCallFunc makes one blocking drain call to a plugin. The production
// call is the plugin connection's SendDrain; a test passes its own.
type drainCallFunc func(ctx context.Context, input *rpc.DrainInput) error

// drainRequest is one drain the fan-out hands to a caller, with the reply
// channel the caller posts the call's result to. The channel has capacity 1,
// so the post never blocks, even after the fan-out stopped reading.
type drainRequest struct {
	input rpc.DrainInput
	reply chan error
}

// drainCaller makes the drain calls to one plugin generation, so the fan-out
// overlaps every plugin's drain without starting a goroutine per stop: a
// drain call blocks until the answer or its context ends, and a call made
// under the drain deadline would drop an answer that arrives after it.
//
// Lifecycle: superviseProcess builds it with newDrainCaller when the
// generation's Stage 1 registration declared a DrainBudget, starts run on the
// server's WaitGroup, and adds it to the server's caller set before the
// generation serves its RPCs. When the generation ends, superviseProcess
// removes it from the set, cancels run's context and waits for done, all
// before applyFailurePolicy, so no caller outlives its generation and a
// respawn's caller is never a second caller for one plugin. Safe for
// concurrent use.
type drainCaller struct {
	plugin string
	budget time.Duration
	call   drainCallFunc
	// stop is the stop slot. A stop drain is sent once per daemon stop, so
	// capacity 1 always holds it.
	stop chan drainRequest
	done chan struct{}
}

func newDrainCaller(plugin string, budget time.Duration, call drainCallFunc) *drainCaller {
	return &drainCaller{
		plugin: plugin,
		budget: budget,
		call:   call,
		stop:   make(chan drainRequest, 1),
		done:   make(chan struct{}),
	}
}

// run serves drain requests until ctx ends. Each call runs under ctx, which
// lives as long as the generation, never under the drain deadline. The call
// also returns when the connection closes, which ends the generation.
func (c *drainCaller) run(ctx context.Context) {
	defer close(c.done)
	for {
		select {
		case request := <-c.stop:
			request.reply <- c.call(ctx, &request.input)
		case <-ctx.Done():
			return
		}
	}
}

// startDrainCaller starts the generation's drain caller when its registration
// declared a DrainBudget and returns the function superviseProcess MUST call
// when the generation ends, before applyFailurePolicy. A generation with no
// budget gets no caller and a no-op stop.
func (s *Server) startDrainCaller(proc *process.Process) func() {
	reg := proc.Registration()
	if reg == nil || reg.DrainBudget <= 0 {
		return func() {}
	}
	conn := proc.Conn()
	if conn == nil {
		return func() {}
	}

	caller := newDrainCaller(proc.Name(), reg.DrainBudget, conn.SendDrain)
	ctx, cancel := context.WithCancel(s.ctx)
	s.wg.Go(func() { caller.run(ctx) })
	s.addDrainCaller(proc, caller)

	return func() {
		s.removeDrainCaller(proc)
		cancel()
		<-caller.done
	}
}

func (s *Server) addDrainCaller(proc *process.Process, caller *drainCaller) {
	s.drainMu.Lock()
	defer s.drainMu.Unlock()
	if s.drainCallers == nil {
		s.drainCallers = make(map[*process.Process]*drainCaller)
	}
	s.drainCallers[proc] = caller
}

func (s *Server) removeDrainCaller(proc *process.Process) {
	s.drainMu.Lock()
	defer s.drainMu.Unlock()
	delete(s.drainCallers, proc)
}

// DrainRound is one stop drain over the plugins whose generation was live,
// with a DrainBudget, when BeginDrain took its snapshot. The hub reads its
// size and its largest budget to set the deadline, then runs it once.
type DrainRound struct {
	callers []*drainCaller
}

// BeginDrain snapshots the live drain callers, ordered by plugin name so the
// logs read the same on every stop. A plugin whose process has exited is not
// in the snapshot.
//
// It also ends respawning: from here the stop is terminal, so a plugin that
// exits during the drain stays down until teardown (ProcessManager.Respawn
// answers ErrDaemonStopping), rather than coming back and announcing again
// what its drain made unattractive. The hub MUST call it once, on the stop
// path, before it runs or skips the round.
func (s *Server) BeginDrain() *DrainRound {
	if pm := s.procManager.Load(); pm != nil {
		pm.RefuseRespawns()
	}

	s.drainMu.Lock()
	callers := make([]*drainCaller, 0, len(s.drainCallers))
	for _, caller := range s.drainCallers {
		callers = append(callers, caller)
	}
	s.drainMu.Unlock()

	slices.SortFunc(callers, func(a, b *drainCaller) int { return strings.Compare(a.plugin, b.plugin) })
	return &DrainRound{callers: callers}
}

// Plugins is the number of plugins the round drains.
func (r *DrainRound) Plugins() int { return len(r.callers) }

// BudgetMax is the largest DrainBudget among the round's plugins, zero for
// an empty round.
func (r *DrainRound) BudgetMax() time.Duration {
	var budgetMax time.Duration
	for _, caller := range r.callers {
		budgetMax = max(budgetMax, caller.budget)
	}
	return budgetMax
}

// DrainResult names, by plugin, how each plugin's drain ended.
type DrainResult struct {
	Answered []string // Answered by the deadline with success.
	Failed   []string // Answered with an error, or its caller ended first.
	Late     []string // No answer by the deadline.
	// Aborted is true when ctx was canceled before the deadline (a signal
	// during the drain), which ends the reading at once.
	Aborted bool
}

// Run sends one drain request to every plugin of the round and reads the
// answers until all arrived or ctx ends. ctx carries the drain deadline and
// is canceled by a signal that aborts the drain; input.Deadline MUST be that
// same deadline, which the plugins read. Run starts no goroutine and blocks
// on nothing a plugin can hold past ctx.
func (r *DrainRound) Run(ctx context.Context, input rpc.DrainInput) DrainResult {
	start := time.Now()
	var result DrainResult
	replies := make([]chan error, len(r.callers))

	for i, caller := range r.callers {
		reply := make(chan error, 1)
		select {
		case caller.stop <- drainRequest{input: input, reply: reply}:
			replies[i] = reply
			logger().Info("drain started", "plugin", caller.plugin, "kind", input.Kind.String())
		case <-caller.done:
			result.Failed = append(result.Failed, caller.plugin)
			logger().Warn("drain failed", "plugin", caller.plugin, "error", "plugin connection closed")
		case <-ctx.Done():
			result.Late = append(result.Late, caller.plugin)
		}
	}

	for i, caller := range r.callers {
		if replies[i] == nil {
			continue
		}
		if ctx.Err() != nil {
			result.Late = append(result.Late, caller.plugin)
			continue
		}
		select {
		case err := <-replies[i]:
			if err != nil {
				result.Failed = append(result.Failed, caller.plugin)
				logger().Warn("drain failed", "plugin", caller.plugin, "error", err)
				continue
			}
			result.Answered = append(result.Answered, caller.plugin)
			logger().Info("drain finished", "plugin", caller.plugin, "elapsed", drainElapsed(start))
		case <-caller.done:
			// A request can land in the stop slot of a caller that has
			// already returned; nobody will answer it.
			result.Failed = append(result.Failed, caller.plugin)
			logger().Warn("drain failed", "plugin", caller.plugin, "error", "plugin connection closed")
		case <-ctx.Done():
			result.Late = append(result.Late, caller.plugin)
		}
	}

	if errors.Is(ctx.Err(), context.Canceled) {
		result.Aborted = true
		logger().Warn("drain aborted by signal", "elapsed", drainElapsed(start))
		return result
	}
	if len(result.Late) > 0 {
		logger().Warn("drain deadline passed with no answer", "plugins", strings.Join(result.Late, ","),
			"elapsed", drainElapsed(start))
	}
	return result
}

// drainElapsed is the time since start, as an operator reads it in a drain
// log line. At a millisecond or more it rounds to the millisecond. Below that it
// keeps the full precision, because a drain that answered in 300 µs rounded to
// the millisecond would log "0s", which reads as no time at all.
func drainElapsed(start time.Time) time.Duration {
	elapsed := time.Since(start)
	if elapsed < time.Millisecond {
		return elapsed
	}
	return elapsed.Round(time.Millisecond)
}

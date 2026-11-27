// Design: docs/architecture/diagnostics/production-diagnostics.md -- bounded handler execution and hung-handler tracking

package server

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/flightrec"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/core/watchdog"
)

const (
	defaultHandlerTimeout   = 60 * time.Second
	handlerCancelGrace      = 100 * time.Millisecond
	maxHungHandlerRecords   = 16
	maxActiveHungHandlers   = 8
	maxActiveHungPerCommand = 4
	txLockStallThreshold    = 45 * time.Second
	hungHandlerFix          = "ze show system goroutines mode blocked"
)

// HungHandlerRecord describes a builtin command handler invocation that
// exceeded its execution deadline without returning.
type HungHandlerRecord struct {
	Command    string    `json:"command"`
	OpID       string    `json:"operation-id,omitempty"`
	StartedAt  time.Time `json:"started-at"`
	TimedOutAt time.Time `json:"timed-out-at"`
	ElapsedMS  int64     `json:"elapsed-ms"`
	Surface    string    `json:"surface,omitempty"`
	Actor      string    `json:"actor,omitempty"`
}

type hungHandlerTracker struct {
	mu          sync.Mutex
	records     [maxHungHandlerRecords]HungHandlerRecord
	head        int
	count       int
	activeByCmd map[string]int
	active      atomic.Int64
}

func (t *hungHandlerTracker) addActive(cmdName string) {
	t.active.Add(1)
	t.mu.Lock()
	if t.activeByCmd == nil {
		t.activeByCmd = make(map[string]int)
	}
	t.activeByCmd[cmdName]++
	t.mu.Unlock()
}

func (t *hungHandlerTracker) decActive(cmdName string) {
	t.active.Add(-1)
	t.mu.Lock()
	if t.activeByCmd != nil {
		if n := t.activeByCmd[cmdName] - 1; n <= 0 {
			delete(t.activeByCmd, cmdName)
		} else {
			t.activeByCmd[cmdName] = n
		}
	}
	t.mu.Unlock()
}

func (t *hungHandlerTracker) shouldRefuse(cmdName string) bool {
	t.mu.Lock()
	perCmd := t.activeByCmd[cmdName]
	t.mu.Unlock()
	if perCmd == 0 {
		return false
	}
	if perCmd >= maxActiveHungPerCommand {
		return true
	}
	return t.activeCount() >= maxActiveHungHandlers
}

func (t *hungHandlerTracker) record(rec HungHandlerRecord) {
	t.mu.Lock()
	t.records[t.head] = rec
	t.head = (t.head + 1) % maxHungHandlerRecords
	if t.count < maxHungHandlerRecords {
		t.count++
	}
	t.mu.Unlock()
}

func (t *hungHandlerTracker) snapshot() []HungHandlerRecord {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.count == 0 {
		return []HungHandlerRecord{}
	}
	out := make([]HungHandlerRecord, 0, t.count)
	for i := range t.count {
		idx := (t.head - 1 - i + maxHungHandlerRecords) % maxHungHandlerRecords
		out = append(out, t.records[idx])
	}
	return out
}

func (t *hungHandlerTracker) activeCount() int {
	n := t.active.Load()
	if n < 0 {
		return 0
	}
	return int(n)
}

// HungHandlers returns the newest-first ring of timed-out builtin command
// handlers (up to 16 entries). Always returns a non-nil slice and acquires
// only the leaf hungHandlers.mu lock.
func (s *Server) HungHandlers() []HungHandlerRecord {
	return s.hungHandlers.snapshot()
}

// HungHandlerCount returns the number of timed-out builtin command handlers
// that are still blocked and have not returned yet. Lock-free atomic read.
func (s *Server) HungHandlerCount() int {
	return s.hungHandlers.activeCount()
}

func (s *Server) setTxLockStallThreshold(d time.Duration) {
	s.txLockStallOverride.Store(int64(d))
}

func (s *Server) txLockStallLimit() time.Duration {
	if override := time.Duration(s.txLockStallOverride.Load()); override > 0 {
		return override
	}
	return txLockStallThreshold
}

func (s *Server) txLockWatchdogProbe() (bool, string, time.Duration) {
	held := s.txLockHoldDuration(time.Now())
	limit := s.txLockStallLimit()
	if held > limit {
		var tb textbuf.Buffer
		tb.Str("config transaction lock held for ").Str(held.Round(time.Millisecond).String())
		return true, tb.String(), held
	}
	return false, "", 0
}

func (s *Server) hungHandlerWatchdogProbe() (bool, string, time.Duration) {
	active := s.HungHandlerCount()
	if active > 0 {
		var tb textbuf.Buffer
		tb.Int(int64(active)).Str(" hung command handler(s) active")
		return true, tb.String(), 0
	}
	return false, "", 0
}

func (s *Server) registerWatchdogProbes() []func() {
	return []func(){
		watchdog.RegisterProbe("config-tx-lock", s.txLockWatchdogProbe),
		watchdog.RegisterProbe("command-handlers", s.hungHandlerWatchdogProbe),
	}
}

func (d *Dispatcher) setHandlerTimeout(timeout time.Duration) {
	d.handlerTimeoutMu.Lock()
	d.handlerTimeout = timeout
	d.handlerTimeoutMu.Unlock()
}

func (d *Dispatcher) effectiveHandlerTimeout() time.Duration {
	d.handlerTimeoutMu.RLock()
	timeout := d.handlerTimeout
	d.handlerTimeoutMu.RUnlock()
	if timeout <= 0 {
		return defaultHandlerTimeout
	}
	return timeout
}

// resolveHandlerTimeout determines the execution deadline for one builtin
// command dispatch. defaultHandlerTimeout applies only to operator/external
// dispatches (ctx == nil || ctx.Process == nil); plugin-originated internal
// RPC dispatches (ctx.Process != nil) run under their caller/plugin lifecycle
// context unless the command explicitly registers a HandlerTimeout hook.
func (d *Dispatcher) resolveHandlerTimeout(ctx *CommandContext, cmd *Command, validated command.ValidatedArgs) time.Duration {
	if cmd.HandlerTimeout != nil {
		if budget := cmd.HandlerTimeout(validated); budget > 0 {
			return budget
		}
	}
	if ctx == nil {
		return d.effectiveHandlerTimeout()
	}
	if ctx.Process == nil {
		return d.effectiveHandlerTimeout()
	}
	return 0
}

type handlerCallResult struct {
	resp *plugin.Response
	err  error
}

func (d *Dispatcher) invokeHandlerBounded(ctx *CommandContext, cmd *Command, validated command.ValidatedArgs) (*plugin.Response, error) {
	timeout := d.resolveHandlerTimeout(ctx, cmd, validated)
	if timeout <= 0 {
		return cmd.Handler(ctx, validated)
	}

	var srv *Server
	var opID, surface, actor string
	if ctx != nil {
		srv = ctx.Server
		opID = ctx.OpID()
		surface = ctx.Surface
		actor = ctx.Username
	}

	if srv != nil {
		if srv.hungHandlers.shouldRefuse(cmd.Name) {
			refusal := opererr.New(
				hungLimitProblem(cmd.Name, opID),
				"earlier invocations of this command handler are still blocked past their execution deadline",
				hungHandlerFix,
			)
			return plugin.OperatorErrorResponse(refusal), nil
		}
	}

	parentCtx := context.Background()
	if ctx != nil {
		parentCtx = ctx.Context()
	}
	timedCtx, cancel := context.WithTimeout(parentCtx, timeout)

	var runCtx *CommandContext
	if ctx != nil {
		copied := *ctx
		copied.RequestContext = timedCtx
		runCtx = &copied
	}

	call := &boundedHandlerCall{
		srv:     srv,
		cmdName: cmd.Name,
		done:    make(chan handlerCallResult, 1),
	}
	startedAt := time.Now().UTC()

	go call.run(cmd.Handler, runCtx, validated)

	select {
	case res := <-call.done:
		cancel()
		return res.resp, res.err
	case <-timedCtx.Done():
	}

	if parentCtx.Err() != nil {
		graceTimer := time.NewTimer(handlerCancelGrace)
		defer graceTimer.Stop()
		select {
		case res := <-call.done:
			cancel()
			return res.resp, res.err
		case <-graceTimer.C:
			cancel()
			return nil, parentCtx.Err()
		}
	}

	graceTimer := time.NewTimer(handlerCancelGrace)
	defer graceTimer.Stop()
	select {
	case res := <-call.done:
		cancel()
		return res.resp, res.err
	case <-graceTimer.C:
	}

	if call.markHung() {
		res := <-call.done
		cancel()
		return res.resp, res.err
	}

	timedOutAt := time.Now().UTC()
	elapsedMS := max(timedOutAt.Sub(startedAt).Milliseconds(), 0)

	if srv != nil {
		srv.hungHandlers.record(HungHandlerRecord{
			Command:    cmd.Name,
			OpID:       opID,
			StartedAt:  startedAt,
			TimedOutAt: timedOutAt,
			ElapsedMS:  elapsedMS,
			Surface:    surface,
			Actor:      actor,
		})
	}

	msg := hungHandlerMessage(cmd.Name, opID, elapsedMS)
	report.RaiseErrorContext(
		timedCtx,
		"command",
		CodeDoctorCommandHandlerHung,
		fitSubject(cmd.Name),
		report.FitMessage(msg),
		hungHandlerFix,
	)
	logger().ErrorContext(
		timedCtx,
		"command handler timed out",
		"code", CodeDoctorCommandHandlerHung,
		"command", cmd.Name,
		"operation-id", opID,
		"elapsed-ms", elapsedMS,
		"fix", hungHandlerFix,
	)
	flightrec.CaptureAuto("hung-command", cmd.Name)
	cancel()

	refusal := opererr.New(
		hungTimeoutProblem(cmd.Name, opID),
		"the command handler did not return before its execution deadline",
		hungHandlerFix,
	)
	return plugin.OperatorErrorResponse(refusal), nil
}

func hungTimeoutProblem(cmdName, opID string) string {
	var tb textbuf.Buffer
	tb.Str("command handler timed out: ").Str(cmdName)
	if opID != "" {
		tb.Str(" (operation-id: ").Str(opID).Byte(')')
	}
	return tb.String()
}

func hungLimitProblem(cmdName, opID string) string {
	var tb textbuf.Buffer
	tb.Str("command handler refused: ").Str(cmdName)
	if opID != "" {
		tb.Str(" (operation-id: ").Str(opID).Byte(')')
	}
	return tb.String()
}

type boundedHandlerCall struct {
	srv        *Server
	cmdName    string
	done       chan handlerCallResult
	mu         sync.Mutex
	doneReady  bool
	markedHung bool
}

func (c *boundedHandlerCall) run(handler Handler, runCtx *CommandContext, validated command.ValidatedArgs) {
	resp, err := handler(runCtx, validated)
	c.mu.Lock()
	wasHung := c.markedHung
	c.doneReady = true
	c.mu.Unlock()
	if wasHung {
		if c.srv != nil {
			c.srv.hungHandlers.decActive(c.cmdName)
		}
	}
	c.done <- handlerCallResult{resp: resp, err: err}
}

func (c *boundedHandlerCall) markHung() (alreadyDone bool) {
	c.mu.Lock()
	alreadyDone = c.doneReady
	if !alreadyDone {
		c.markedHung = true
		if c.srv != nil {
			c.srv.hungHandlers.addActive(c.cmdName)
		}
	}
	c.mu.Unlock()
	return alreadyDone
}

func hungHandlerMessage(cmdName, opID string, elapsedMS int64) string {
	var tb textbuf.Buffer
	tb.Str("command handler ").Str(cmdName).Str(" exceeded its execution deadline after ").
		Int(elapsedMS).Str("ms")
	if opID != "" {
		tb.Str(" (operation-id: ").Str(opID).Byte(')')
	}
	return tb.String()
}

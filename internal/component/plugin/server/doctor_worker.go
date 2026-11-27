// Design: docs/architecture/doctor-and-health-checks.md — concurrent per-plugin-generation doctor check workers
// Overview: server.go — plugin server lifecycle
// Related: drain.go — drainCaller, the sibling per-generation worker pattern
// Related: failure_policy.go — superviseProcess, which owns each doctor worker

package server

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// doctorWorkerQueueDepth bounds the buffered request queue of one plugin
// generation's doctor worker (twice defaultMaxSessions = 8 in component/ssh).
const doctorWorkerQueueDepth = 16

// doctorCheckReqState coordinates pickup by the worker against queue-wait
// expiration by RunDoctorChecks via a single atomic CompareAndSwap so an
// expired request is always discarded unrun and a picked-up request is never
// timed out on the queue wait.
const (
	doctorReqQueued   int32 = 0
	doctorReqPickedUp int32 = 1
	doctorReqExpired  int32 = 2
)

// DoctorCheckPluginRequest names one plugin and the ordered list of Stage-1
// doctor checks to run serially against that plugin's live generation.
type DoctorCheckPluginRequest struct {
	Plugin string
	Checks []string
}

// DoctorCheckOutcome is the answer for one check in a plugin batch.
type DoctorCheckOutcome struct {
	Check  string
	Output *rpc.DoctorCheckOutput
	Err    error
}

// DoctorCheckPluginResult groups the ordered outcomes for one plugin request.
type DoctorCheckPluginResult struct {
	Plugin   string
	Outcomes []DoctorCheckOutcome
}

// doctorCheckCallFunc makes one blocking doctor-check call to a plugin.
// Production passes conn.SendDoctorCheck; tests may pass a stub.
type doctorCheckCallFunc func(ctx context.Context, checkName string) (*rpc.DoctorCheckOutput, error)

// doctorCheckBatchRequest is one plugin batch submitted to a doctorWorker.
// reply has capacity 1 so a late answer never blocks worker exit.
type doctorCheckBatchRequest struct {
	checks          []string
	perCheckTimeout time.Duration
	pickedUp        chan struct{}
	state           atomic.Int32
	reply           chan []DoctorCheckOutcome
}

func newDoctorCheckBatchRequest(checks []string, perCheckTimeout time.Duration) *doctorCheckBatchRequest {
	return &doctorCheckBatchRequest{
		checks:          checks,
		perCheckTimeout: perCheckTimeout,
		pickedUp:        make(chan struct{}),
		reply:           make(chan []DoctorCheckOutcome, 1),
	}
}

// doctorWorker executes doctor-check batches for one live plugin generation so
// different plugins run concurrently without spawning a goroutine per request,
// while checks of one plugin run serially to match the SDK event loop.
//
// Lifecycle: superviseProcess starts one worker via startDoctorWorker when the
// generation's Stage-1 registration declares DoctorChecks, runs it on s.wg.Go,
// and registers it in Server.doctorWorkers before serving RPCs. When the
// generation ends, superviseProcess removes it from Server.doctorWorkers,
// cancels its context, and joins <-worker.done before applyFailurePolicy so no
// worker outlives its generation and a respawn never meets an old worker.
// Safe for concurrent use.
type doctorWorker struct {
	plugin string
	call   doctorCheckCallFunc
	queue  chan *doctorCheckBatchRequest
	done   chan struct{}
}

func newDoctorWorker(pluginName string, call doctorCheckCallFunc) *doctorWorker {
	return &doctorWorker{
		plugin: pluginName,
		call:   call,
		queue:  make(chan *doctorCheckBatchRequest, doctorWorkerQueueDepth),
		done:   make(chan struct{}),
	}
}

// run drains doctor-check batches from w.queue until ctx ends. Expired
// requests are discarded unrun. Each surviving batch's checks run serially
// with a fresh per-check timeout started at pickup of each check.
func (w *doctorWorker) run(ctx context.Context) {
	defer close(w.done)
	for {
		select {
		case req := <-w.queue:
			if ctx.Err() != nil {
				return
			}
			if !req.state.CompareAndSwap(doctorReqQueued, doctorReqPickedUp) {
				// Caller's queue wait expired before dequeue; discard unrun.
				continue
			}
			close(req.pickedUp)
			outcomes, ok := w.executeBatch(ctx, req)
			if !ok {
				return
			}
			req.reply <- outcomes
		case <-ctx.Done():
			return
		}
	}
}

// executeBatch runs every check in req serially under a fresh per-check
// timeout derived from the worker's generation context. Returns false when the
// worker's generation context ends mid-batch.
func (w *doctorWorker) executeBatch(ctx context.Context, req *doctorCheckBatchRequest) ([]DoctorCheckOutcome, bool) {
	outcomes := make([]DoctorCheckOutcome, len(req.checks))
	for i, checkName := range req.checks {
		if ctx.Err() != nil {
			return nil, false
		}
		checkCtx, cancel := context.WithTimeout(ctx, req.perCheckTimeout)
		out, err := w.call(checkCtx, checkName)
		cancel()
		if ctx.Err() != nil {
			return nil, false
		}
		outcomes[i] = DoctorCheckOutcome{
			Check:  checkName,
			Output: out,
			Err:    err,
		}
	}
	return outcomes, true
}

// startDoctorWorker starts the generation's doctor worker when its Stage-1
// registration declared at least one doctor check and returns the stop
// function superviseProcess MUST call when the generation ends, before
// applyFailurePolicy. A generation declaring no DoctorChecks gets no worker
// and a no-op stop.
func (s *Server) startDoctorWorker(proc *process.Process) func() {
	reg := proc.Registration()
	if reg == nil {
		return func() {}
	}
	if len(reg.DoctorChecks) == 0 {
		return func() {}
	}
	conn := proc.Conn()
	if conn == nil {
		return func() {}
	}

	worker := newDoctorWorker(proc.Name(), conn.SendDoctorCheck)
	ctx, cancel := context.WithCancel(s.ctx)
	s.wg.Go(func() { worker.run(ctx) })
	s.addDoctorWorker(proc, worker)

	return func() {
		s.removeDoctorWorker(proc)
		cancel()
		<-worker.done
	}
}

func (s *Server) addDoctorWorker(proc *process.Process, worker *doctorWorker) {
	s.doctorWorkersMu.Lock()
	if s.doctorWorkers == nil {
		s.doctorWorkers = make(map[*process.Process]*doctorWorker)
	}
	s.doctorWorkers[proc] = worker
	if s.doctorWorkersNotify != nil {
		close(s.doctorWorkersNotify)
		s.doctorWorkersNotify = nil
	}
	s.doctorWorkersMu.Unlock()
}

func (s *Server) removeDoctorWorker(proc *process.Process) {
	s.doctorWorkersMu.Lock()
	delete(s.doctorWorkers, proc)
	s.doctorWorkersMu.Unlock()
}

func (s *Server) doctorWorkerFor(proc *process.Process) *doctorWorker {
	s.doctorWorkersMu.Lock()
	w := s.doctorWorkers[proc]
	s.doctorWorkersMu.Unlock()
	return w
}

func (s *Server) doctorWorkerOrNotify(proc *process.Process) (*doctorWorker, <-chan struct{}) {
	s.doctorWorkersMu.Lock()
	defer s.doctorWorkersMu.Unlock()
	if w := s.doctorWorkers[proc]; w != nil {
		return w, nil
	}
	if s.doctorWorkersNotify == nil {
		s.doctorWorkersNotify = make(chan struct{})
	}
	return nil, s.doctorWorkersNotify
}

// awaitingDoctorWorkerStart reports whether proc is a live running generation
// with declared doctor checks whose superviseProcess goroutine has not yet
// reached startDoctorWorker.
func (s *Server) awaitingDoctorWorkerStart(proc *process.Process) bool {
	if proc == nil {
		return false
	}
	if proc.PluginEnded() {
		return false
	}
	if proc.Stage() < plugin.StageRunning {
		return false
	}
	if s.ctx != nil && s.ctx.Err() != nil {
		return false
	}
	reg := proc.Registration()
	return reg != nil && len(reg.DoctorChecks) > 0
}

func isDoctorWorkerDone(w *doctorWorker) bool {
	select {
	case <-w.done:
		return true
	default:
		return false
	}
}

func fillBatchError(dst []DoctorCheckOutcome, checks []string, err error) {
	for j, check := range checks {
		dst[j] = DoctorCheckOutcome{Check: check, Err: err}
	}
}

// RunDoctorChecks fans out per-plugin doctor check batches concurrently across
// each plugin's generation-scoped doctorWorker without spawning per-request
// goroutines.
//
// Pass 1a enqueues non-blockingly into every healthy worker first so one full
// or starting plugin never delays others. Pass 1b waits up to the shared
// queueWait deadline for any deferred plugin. Pass 2a awaits worker pickup for
// all submitted plugins under that same queueWait deadline, expiring unpicked
// requests atomically so workers discard them unrun. Pass 2b collects replies
// for all picked-up batches, each bounded by perCheckTimeout per check inside
// its worker.
func (s *Server) RunDoctorChecks(requests []DoctorCheckPluginRequest, perCheckTimeout, queueWait time.Duration) []DoctorCheckPluginResult {
	if len(requests) == 0 {
		return nil
	}

	results := make([]DoctorCheckPluginResult, len(requests))
	for i, req := range requests {
		results[i] = DoctorCheckPluginResult{
			Plugin:   req.Plugin,
			Outcomes: make([]DoctorCheckOutcome, len(req.Checks)),
		}
	}

	pm := s.procManager.Load()
	if pm == nil {
		for i, req := range requests {
			fillBatchError(results[i].Outcomes, req.Checks,
				fmt.Errorf("plugin %q doctor check worker is not available: no process manager", req.Plugin))
		}
		return results
	}

	type pendingPlugin struct {
		index  int
		plugin string
		checks []string
		proc   *process.Process
		worker *doctorWorker
		req    *doctorCheckBatchRequest
	}

	submitted := make([]pendingPlugin, 0, len(requests))
	var deferred []pendingPlugin

	// Pass 1a: non-blocking submit to every ready plugin worker.
	for i, req := range requests {
		if len(req.Checks) == 0 {
			continue
		}
		proc := pm.GetProcess(req.Plugin)
		if proc == nil {
			fillBatchError(results[i].Outcomes, req.Checks,
				fmt.Errorf("plugin %q is not running", req.Plugin))
			continue
		}
		worker := s.doctorWorkerFor(proc)
		if worker == nil {
			if s.awaitingDoctorWorkerStart(proc) {
				deferred = append(deferred, pendingPlugin{
					index:  i,
					plugin: req.Plugin,
					checks: req.Checks,
					proc:   proc,
					req:    newDoctorCheckBatchRequest(req.Checks, perCheckTimeout),
				})
				continue
			}
			fillBatchError(results[i].Outcomes, req.Checks,
				fmt.Errorf("plugin %q doctor check worker is not running", req.Plugin))
			continue
		}
		if isDoctorWorkerDone(worker) {
			fillBatchError(results[i].Outcomes, req.Checks,
				fmt.Errorf("plugin %q doctor check worker stopped", req.Plugin))
			continue
		}
		batchReq := newDoctorCheckBatchRequest(req.Checks, perCheckTimeout)
		item := pendingPlugin{
			index:  i,
			plugin: req.Plugin,
			checks: req.Checks,
			proc:   proc,
			worker: worker,
			req:    batchReq,
		}
		select {
		case worker.queue <- batchReq:
			submitted = append(submitted, item)
		default:
			deferred = append(deferred, item)
		}
	}

	if len(submitted) == 0 && len(deferred) == 0 {
		return results
	}

	queueExpired := false
	var deadlineCh <-chan time.Time
	if queueWait <= 0 {
		queueExpired = true
	} else {
		timer := time.NewTimer(queueWait)
		defer timer.Stop()
		deadlineCh = timer.C
	}

	var serverDone <-chan struct{}
	if s.ctx != nil {
		serverDone = s.ctx.Done()
	}

	// Pass 1b: enqueue any plugins whose worker queue was full or still starting.
	for _, item := range deferred {
		for item.worker == nil {
			w, notify := s.doctorWorkerOrNotify(item.proc)
			if w != nil {
				item.worker = w
				break
			}
			if !s.awaitingDoctorWorkerStart(item.proc) {
				fillBatchError(results[item.index].Outcomes, item.checks,
					fmt.Errorf("plugin %q doctor check worker is not running", item.plugin))
				break
			}
			if queueExpired {
				fillBatchError(results[item.index].Outcomes, item.checks,
					fmt.Errorf("plugin %q doctor check busy: worker not ready within %s", item.plugin, queueWait))
				break
			}
			select {
			case <-notify:
			case <-item.proc.EngineDone():
			case <-serverDone:
			case <-deadlineCh:
				queueExpired = true
				deadlineCh = nil
			}
		}
		if item.worker == nil {
			continue
		}
		if isDoctorWorkerDone(item.worker) {
			fillBatchError(results[item.index].Outcomes, item.checks,
				fmt.Errorf("plugin %q doctor check worker stopped", item.plugin))
			continue
		}
		if queueExpired {
			fillBatchError(results[item.index].Outcomes, item.checks,
				fmt.Errorf("plugin %q doctor check busy: queue full after waiting %s", item.plugin, queueWait))
			continue
		}
		select {
		case item.worker.queue <- item.req:
			submitted = append(submitted, item)
		case <-item.worker.done:
			fillBatchError(results[item.index].Outcomes, item.checks,
				fmt.Errorf("plugin %q doctor check worker stopped", item.plugin))
		case <-deadlineCh:
			queueExpired = true
			deadlineCh = nil
			fillBatchError(results[item.index].Outcomes, item.checks,
				fmt.Errorf("plugin %q doctor check busy: queue full after waiting %s", item.plugin, queueWait))
		}
	}

	// Pass 2a: await worker pickup for every submitted plugin under the shared
	// queueWait deadline before waiting on any slow plugin's reply, so an
	// unpicked plugin is expired promptly at queueWait even while another
	// plugin is already executing.
	picked := submitted[:0]
	for _, item := range submitted {
		if queueExpired {
			if item.req.state.CompareAndSwap(doctorReqQueued, doctorReqExpired) {
				fillBatchError(results[item.index].Outcomes, item.checks,
					fmt.Errorf("plugin %q doctor check busy: not picked up within %s", item.plugin, queueWait))
				continue
			}
			picked = append(picked, item)
			continue
		}

		select {
		case <-item.req.pickedUp:
			picked = append(picked, item)
		case outcomes := <-item.req.reply:
			results[item.index].Outcomes = outcomes
		case <-item.worker.done:
			select {
			case outcomes := <-item.req.reply:
				results[item.index].Outcomes = outcomes
			default:
				fillBatchError(results[item.index].Outcomes, item.checks,
					fmt.Errorf("plugin %q doctor check worker stopped", item.plugin))
			}
		case <-deadlineCh:
			queueExpired = true
			deadlineCh = nil
			if item.req.state.CompareAndSwap(doctorReqQueued, doctorReqExpired) {
				fillBatchError(results[item.index].Outcomes, item.checks,
					fmt.Errorf("plugin %q doctor check busy: not picked up within %s", item.plugin, queueWait))
			} else {
				picked = append(picked, item)
			}
		}
	}

	// Pass 2b: collect replies for all picked-up batches. Each worker bounds
	// every check by perCheckTimeout and closes done if the generation ends.
	for _, item := range picked {
		select {
		case outcomes := <-item.req.reply:
			results[item.index].Outcomes = outcomes
		case <-item.worker.done:
			select {
			case outcomes := <-item.req.reply:
				results[item.index].Outcomes = outcomes
			default:
				fillBatchError(results[item.index].Outcomes, item.checks,
					fmt.Errorf("plugin %q doctor check worker stopped", item.plugin))
			}
		}
	}

	return results
}

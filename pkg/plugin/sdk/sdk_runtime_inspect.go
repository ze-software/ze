// Design: docs/architecture/api/process-protocol.md -- independent runtime inspection worker.
// Related: sdk.go -- Run owns this worker's connection lifetime automatically.
// Related: sdk_read_snapshot.go -- shared independent snapshot worker lifecycle.
package sdk

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/flightrec"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// runtimeInspectQueueDepth bounds waiting diagnostic frames behind the single
// active inspection request. Excess wire frames are dropped without closing
// the production MuxConn; bridge callers fail fast with errRuntimeInspectFull.
const (
	runtimeInspectQueueDepth         = 16
	runtimeInspectGoroutineInitBytes = 256 << 10
)

var (
	sdkCPUProfileMu            sync.Mutex
	errSDKCPUProfileInProgress = errors.New("CPU profile already in progress")
	errRuntimeInspectStopped   = errors.New("runtime-inspect connection stopped; retry on a live plugin connection")
	errRuntimeInspectFull      = errors.New("runtime-inspect queue is full; wait for an earlier inspection before retrying")
)

// runtimeInspectQuery owns a validated inspection input and an optional caller
// context (non-nil on DirectBridge) so early cancellation aborts active CPU
// profiling immediately.
type runtimeInspectQuery struct {
	input   rpc.RuntimeInspectInput
	callCtx context.Context
}

type runtimeInspectAnswer struct {
	output *rpc.RuntimeInspectOutput
	err    error
}

// runtimeInspectJob carries either a raw wire frame or a validated bridge query.
// reply has capacity one so caller cancellation never stalls worker stop.
type runtimeInspectJob struct {
	wire  *rpc.Request
	query *runtimeInspectQuery
	reply chan runtimeInspectAnswer
}

// cappedProfileBuffer captures at most maxBytes of raw pprof data and discards
// any overflow while setting truncated so responses stay within MaxMessageSize.
// NOT safe for concurrent use.
type cappedProfileBuffer struct {
	buf       bytes.Buffer
	maxBytes  int
	truncated bool
}

func (b *cappedProfileBuffer) Write(p []byte) (int, error) {
	remaining := b.maxBytes - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) <= remaining {
		return b.buf.Write(p)
	}
	if _, err := b.buf.Write(p[:remaining]); err != nil {
		return 0, err
	}
	b.truncated = true
	return len(p), nil
}

// runtimeInspectWorker is one long-lived connection worker started automatically
// by Plugin.Run. Independent Mux ingress and the DirectBridge slot enqueue
// directly before ordinary callback queues. Fault isolation invariant: no
// diagnostic error, saturation, deadline expiry, or collector panic may close
// the plugin's production MuxConn or DirectBridge.
type runtimeInspectWorker struct {
	plugin    *Plugin
	collector func(context.Context, rpc.RuntimeInspectInput) (*rpc.RuntimeInspectOutput, error)
	requests  chan runtimeInspectJob
	done      chan struct{}
	failure   atomic.Pointer[error]
	admission sync.Mutex
	stopped   bool
}

func newRuntimeInspectQuery(input rpc.RuntimeInspectInput, callCtx context.Context) (runtimeInspectQuery, error) {
	if err := input.Validate(time.Now()); err != nil {
		return runtimeInspectQuery{}, err
	}
	return runtimeInspectQuery{input: input, callCtx: callCtx}, nil
}

// admitWire enqueues one inbound wire inspection frame without blocking.
// It ALWAYS returns nil so MuxConn.readLoop never closes the production
// connection when diagnostic traffic bursts or races worker shutdown.
func (w *runtimeInspectWorker) admitWire(request *rpc.Request) error {
	if request == nil {
		return nil
	}
	w.admission.Lock()
	defer w.admission.Unlock()
	if w.stopped {
		return nil
	}
	select {
	case w.requests <- runtimeInspectJob{wire: request}:
		return nil
	default:
		// Drop excess diagnostic frame without closing MuxConn; the caller times
		// out cleanly into host-side fallback.
		return nil
	}
}

// inspect serves DirectBridge calls through the same bounded worker queue.
// Saturation returns errRuntimeInspectFull without affecting DirectBridge.
func (w *runtimeInspectWorker) inspect(ctx context.Context, input rpc.RuntimeInspectInput) (*rpc.RuntimeInspectOutput, error) {
	query, err := newRuntimeInspectQuery(input, ctx)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reply := make(chan runtimeInspectAnswer, 1)
	w.admission.Lock()
	if w.stopped {
		w.admission.Unlock()
		return nil, errRuntimeInspectStopped
	}
	select {
	case w.requests <- runtimeInspectJob{query: &query, reply: reply}:
		w.admission.Unlock()
	default:
		w.admission.Unlock()
		return nil, errRuntimeInspectFull
	}
	select {
	case ans := <-reply:
		return ans.output, ans.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-w.done:
		select {
		case ans := <-reply:
			return ans.output, ans.err
		default:
			return nil, errRuntimeInspectStopped
		}
	}
}

func (w *runtimeInspectWorker) run(ctx context.Context, transportDone <-chan struct{}) {
	// Pass a no-op fail callback: diagnostic failures never tear down the
	// plugin's production connection or DirectBridge.
	runReadSnapshotWorker[runtimeInspectJob, runtimeInspectQuery, rpc.RuntimeInspectOutput](
		ctx, transportDone, w.requests, w.done, w, func(error) {},
	)
}

func (w *runtimeInspectWorker) decode(job runtimeInspectJob) (runtimeInspectQuery, error) {
	if job.query != nil {
		return *job.query, nil
	}
	var input rpc.RuntimeInspectInput
	if err := json.Unmarshal(job.wire.Params, &input); err != nil {
		return runtimeInspectQuery{}, fmt.Errorf("runtime-inspect input: %w", err)
	}
	return newRuntimeInspectQuery(input, nil)
}

// serve executes the diagnostic collector within the request deadline and
// converts any panic into an ordinary RPC error, always returning panicked=false
// so the worker and production connection remain alive.
func (w *runtimeInspectWorker) serve(ctx context.Context, query runtimeInspectQuery) (output *rpc.RuntimeInspectOutput, err error, panicked bool) {
	execCtx, cancel := context.WithDeadline(ctx, time.UnixMilli(query.input.DeadlineMS))
	defer cancel()
	if query.callCtx != nil {
		stopCancel := context.AfterFunc(query.callCtx, cancel)
		defer stopCancel()
	}
	if err := execCtx.Err(); err != nil {
		return nil, err, false
	}
	defer func() {
		if r := recover(); r != nil {
			output = nil
			err = fmt.Errorf("runtime-inspect %s panicked: %v", query.input.Kind, r)
			panicked = false
		}
	}()
	output, err = w.collector(execCtx, query.input)
	if err != nil {
		return nil, err, false
	}
	if err := execCtx.Err(); err != nil {
		return nil, err, false
	}
	if err := output.Validate(query.input); err != nil {
		return nil, err, false
	}
	return output, nil, false
}

// answer delivers the diagnostic response on DirectBridge or MuxConn within a
// bounded reply budget and NEVER closes engineMux on reply errors.
func (w *runtimeInspectWorker) answer(ctx context.Context, job runtimeInspectJob, output *rpc.RuntimeInspectOutput, answerErr error) {
	if job.reply != nil {
		job.reply <- runtimeInspectAnswer{output: output, err: answerErr}
		return
	}
	replyCtx, cancel := context.WithTimeout(ctx, rpc.RuntimeInspectDefaultTimeout)
	defer cancel()
	var header struct {
		DeadlineMS int64 `json:"deadline-ms"`
	}
	if err := json.Unmarshal(job.wire.Params, &header); err == nil {
		if header.DeadlineMS > 0 {
			deadline := time.UnixMilli(header.DeadlineMS)
			if deadline.After(time.Now()) {
				var deadlineCancel context.CancelFunc
				replyCtx, deadlineCancel = context.WithDeadline(replyCtx, deadline)
				defer deadlineCancel()
			}
		}
	}
	if answerErr != nil {
		w.plugin.engineMux.SendError(replyCtx, job.wire.ID, answerErr.Error()) //nolint:errcheck // diagnostic reply failure must never close the production MuxConn
		return
	}
	w.plugin.engineMux.SendResult(replyCtx, job.wire.ID, output) //nolint:errcheck // diagnostic reply failure must never close the production MuxConn
}

func (w *runtimeInspectWorker) refuse(job runtimeInspectJob) {
	if job.reply != nil {
		job.reply <- runtimeInspectAnswer{err: errRuntimeInspectStopped}
	}
}

func (w *runtimeInspectWorker) stopAdmission() {
	w.admission.Lock()
	w.stopped = true
	w.admission.Unlock()
}

func (w *runtimeInspectWorker) refuseQueued() {
	for {
		select {
		case job := <-w.requests:
			w.refuse(job)
		default:
			return
		}
	}
}

// startRuntimeInspectWorker installs independent runtime-inspect ingress before
// Stage 5 ready. Every SDK plugin starts it automatically with zero registration
// boilerplate. The caller MUST run the returned stop to remove ingress, seal
// admission, cancel and join the worker goroutine.
func (p *Plugin) startRuntimeInspectWorker(ctx context.Context) (func() error, error) {
	return p.startRuntimeInspectWorkerWithCollector(ctx, collectRuntimeInspect)
}

func (p *Plugin) startRuntimeInspectWorkerWithCollector(
	ctx context.Context,
	collector func(context.Context, rpc.RuntimeInspectInput) (*rpc.RuntimeInspectOutput, error),
) (func() error, error) {
	w := &runtimeInspectWorker{
		plugin:    p,
		collector: collector,
		requests:  make(chan runtimeInspectJob, runtimeInspectQueueDepth),
		done:      make(chan struct{}),
	}
	return p.startReadSnapshotWorker(ctx, rpc.MethodRuntimeInspect, w, w.done, &w.failure,
		func() { p.bridge.SetRuntimeInspect(w.inspect) })
}

func collectRuntimeInspect(ctx context.Context, input rpc.RuntimeInspectInput) (*rpc.RuntimeInspectOutput, error) {
	switch input.Kind {
	case rpc.RuntimeInspectGoroutines:
		stacks, truncated := collectRuntimeGoroutines(rpc.RuntimeInspectMaxGoroutineBytes)
		return &rpc.RuntimeInspectOutput{
			Kind:            rpc.RuntimeInspectGoroutines,
			PID:             os.Getpid(),
			Truncated:       truncated,
			GoroutineStacks: stacks,
		}, nil
	case rpc.RuntimeInspectProfile:
		return collectRuntimeProfile(ctx, input, rpc.RuntimeInspectMaxProfileBytes)
	case rpc.RuntimeInspectLogRecent:
		return collectRuntimeLogRecent(input)
	case rpc.RuntimeInspectRuntimeStats:
		return collectRuntimeStats(), nil
	default:
		return nil, errors.New("runtime-inspect kind is invalid; use goroutines, profile, log-recent, or runtime-stats")
	}
}

func collectRuntimeGoroutines(maxBytes int) (string, bool) {
	if maxBytes <= 0 {
		maxBytes = rpc.RuntimeInspectMaxGoroutineBytes
	}
	size := min(runtimeInspectGoroutineInitBytes, maxBytes)
	for {
		buf := make([]byte, size)
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return string(buf[:n]), false
		}
		if len(buf) >= maxBytes {
			return string(buf[:n]), true
		}
		size = min(size*2, maxBytes)
	}
}

func collectRuntimeProfile(ctx context.Context, input rpc.RuntimeInspectInput, maxBytes int) (*rpc.RuntimeInspectOutput, error) {
	if maxBytes <= 0 {
		maxBytes = rpc.RuntimeInspectMaxProfileBytes
	}
	if input.ProfileType == rpc.RuntimeInspectProfileCPU {
		return collectRuntimeCPUProfile(ctx, input.DurationMS, maxBytes)
	}
	return collectRuntimeSnapshotProfile(input.ProfileType, maxBytes)
}

func collectRuntimeCPUProfile(ctx context.Context, durationMS int64, maxBytes int) (*rpc.RuntimeInspectOutput, error) {
	if !sdkCPUProfileMu.TryLock() {
		return nil, errSDKCPUProfileInProgress
	}
	defer sdkCPUProfileMu.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if durationMS <= 0 {
		durationMS = rpc.RuntimeInspectDefaultCPUDurationMS
	}
	buf := cappedProfileBuffer{maxBytes: maxBytes}
	if err := pprof.StartCPUProfile(&buf); err != nil {
		return nil, err
	}
	timer := time.NewTimer(time.Duration(durationMS) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
	pprof.StopCPUProfile()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &rpc.RuntimeInspectOutput{
		Kind:        rpc.RuntimeInspectProfile,
		PID:         os.Getpid(),
		Truncated:   buf.truncated,
		ProfileType: rpc.RuntimeInspectProfileCPU,
		ProfileData: base64.StdEncoding.EncodeToString(buf.buf.Bytes()),
	}, nil
}

func collectRuntimeSnapshotProfile(profileType string, maxBytes int) (*rpc.RuntimeInspectOutput, error) {
	p := pprof.Lookup(profileType)
	if p == nil {
		return nil, errors.New("unknown profile type: " + profileType)
	}
	buf := cappedProfileBuffer{maxBytes: maxBytes}
	if err := p.WriteTo(&buf, 0); err != nil {
		return nil, err
	}
	out := &rpc.RuntimeInspectOutput{
		Kind:         rpc.RuntimeInspectProfile,
		PID:          os.Getpid(),
		Truncated:    buf.truncated,
		ProfileType:  profileType,
		ProfileCount: p.Count(),
		ProfileData:  base64.StdEncoding.EncodeToString(buf.buf.Bytes()),
	}
	switch profileType {
	case rpc.RuntimeInspectProfileMutex:
		rate := runtime.SetMutexProfileFraction(-1)
		out.SamplingRate = &rate
	case rpc.RuntimeInspectProfileBlock:
		rate := flightrec.BlockProfileRate()
		out.SamplingRate = &rate
	}
	return out, nil
}

func collectRuntimeLogRecent(input rpc.RuntimeInspectInput) (*rpc.RuntimeInspectOutput, error) {
	limit := input.LogCount
	if limit <= 0 {
		limit = rpc.RuntimeInspectMaxLogEntries
	}
	if limit > rpc.RuntimeInspectMaxLogEntries {
		limit = rpc.RuntimeInspectMaxLogEntries
	}
	entries, err := slogutil.GlobalLogRing().Snapshot(limit, input.LogLevel, input.LogComponent)
	if err != nil {
		return nil, err
	}
	outEntries := make([]rpc.RuntimeInspectLogEntry, 0, len(entries))
	for i := range entries {
		attrs, attrErr := entries[i].Attributes()
		if attrErr != nil {
			return nil, attrErr
		}
		outEntries = append(outEntries, rpc.RuntimeInspectLogEntry{
			Timestamp:  entries[i].Timestamp.UTC().Format(time.RFC3339),
			Level:      entries[i].Level,
			Component:  entries[i].Component,
			Message:    entries[i].Message,
			Attributes: attrs,
		})
	}
	return &rpc.RuntimeInspectOutput{
		Kind:       rpc.RuntimeInspectLogRecent,
		PID:        os.Getpid(),
		LogEntries: outEntries,
	}, nil
}

func collectRuntimeStats() *rpc.RuntimeInspectOutput {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return &rpc.RuntimeInspectOutput{
		Kind: rpc.RuntimeInspectRuntimeStats,
		PID:  os.Getpid(),
		RuntimeStats: &rpc.RuntimeInspectStats{
			Alloc:         m.Alloc,
			TotalAlloc:    m.TotalAlloc,
			Sys:           m.Sys,
			HeapAlloc:     m.HeapAlloc,
			HeapSys:       m.HeapSys,
			HeapInuse:     m.HeapInuse,
			HeapObjects:   m.HeapObjects,
			StackInuse:    m.StackInuse,
			NumGC:         m.NumGC,
			GCCPUPct:      m.GCCPUFraction * 100,
			NumGoroutines: runtime.NumGoroutine(),
			NumCPU:        runtime.NumCPU(),
			MaxProcs:      runtime.GOMAXPROCS(0),
			GoVersion:     runtime.Version(),
		},
	}
}

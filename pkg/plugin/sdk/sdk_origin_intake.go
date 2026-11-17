// Design: docs/architecture/api/process-protocol.md -- joined origin mutation worker.
// Related: sdk_current_selection.go -- independent read-only lookup stays separate.
package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type OriginIntakeHandler = rpc.OriginIntakeHandler

// OnOriginIntake registers the deciding plugin's mutating candidate intake.
// Registration precedes Run and grants no peer or route authority.
func (p *Plugin) OnOriginIntake(handler OriginIntakeHandler) {
	p.mu.Lock()
	p.onOriginIntake = handler
	p.mu.Unlock()
}

const originIntakeQueueDepth = 16

var errOriginIntakeStopped = errors.New("origin-intake connection stopped; retry against the live deciding connection")
var errOriginIntakeFull = errors.New("origin-intake queue is full; wait for earlier admitted work")

type originIntakeAnswer struct {
	output *rpc.UpdateRouteOutput
	err    error
}
type originIntakeJob struct {
	wire      *rpc.Request
	input     *rpc.OriginIntakeInput
	admission *rpc.RequestAdmission
	ctx       context.Context
	reply     chan originIntakeAnswer
	refusal   error
}
type originCancellationJob struct {
	wire      *rpc.Request
	admission *rpc.RequestAdmission
	refusal   error
}

// One bounded mutation worker and one bounded cancellation worker belong to the
// connection. The reader enrolls before enqueue, so a later cancel cannot overtake
// request admission. Cancellation joins reply completion without occupying the
// mutation worker. No goroutine is started for an origin event or request.
type originIntakeWorker struct {
	plugin         *Plugin
	handler        OriginIntakeHandler
	receiver       *rpc.RequestReceiver
	requests       chan originIntakeJob
	cancellations  chan originCancellationJob
	ctx            context.Context
	cancel         context.CancelFunc
	done           chan struct{}
	cancelDone     chan struct{}
	supervisorDone chan struct{}
	mu             sync.Mutex
	stopped        bool
	activeCancel   context.CancelFunc
	failure        atomic.Pointer[error]
}

func originIntakeRequestScope(request *rpc.Request) (rpc.RequestScope, error) {
	if request.Method != rpc.MethodOriginIntake {
		return rpc.RequestScope{}, nil
	}
	var header struct {
		PeerOwner     uint64 `json:"peer-owner"`
		OriginReceipt uint64 `json:"origin-receipt"`
		DeadlineMS    int64  `json:"deadline-ms"`
	}
	if err := json.Unmarshal(request.Params, &header); err != nil {
		return rpc.RequestScope{}, err
	}
	if header.PeerOwner == 0 || header.OriginReceipt == 0 {
		return rpc.RequestScope{}, errors.New("origin-intake concrete scope or receipt is missing")
	}
	if header.DeadlineMS <= 0 {
		return rpc.RequestScope{}, errors.New("origin-intake deadline is missing")
	}
	// The bounded mutation worker serializes announce/removal. Cancellation owns
	// a request correlation, not exclusive receipt authority: a consumed reply
	// can precede its admission Finish, and exact cleanup may already be queued.
	// Keep Finish AFTER the response attempt so cancellation cannot false-ACK.
	return rpc.NewRequestCancellationScope(rpc.MethodOriginIntake, time.UnixMilli(header.DeadlineMS)), nil
}

func (w *originIntakeWorker) admitWire(request *rpc.Request) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped || w.ctx.Err() != nil {
		return errOriginIntakeStopped
	}
	admission, err := w.receiver.Admit(request)
	if admission == nil {
		return err
	}
	job := originIntakeJob{wire: request, admission: admission, ctx: admission.Context, refusal: err}
	select {
	case w.requests <- job:
		return nil
	default:
		admission.Finish()
		return errOriginIntakeFull
	}
}

func (w *originIntakeWorker) admitCancellation(request *rpc.Request) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped || w.ctx.Err() != nil {
		return errOriginIntakeStopped
	}
	admission, err := w.receiver.Admit(request)
	if admission == nil {
		return err
	}
	select {
	case w.cancellations <- originCancellationJob{wire: request, admission: admission, refusal: err}:
		return nil
	default:
		admission.Finish()
		return errOriginIntakeFull
	}
}

func (w *originIntakeWorker) offer(ctx context.Context, input rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
	if err := input.Validate(time.Now()); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	input = rpc.CloneOriginIntake(input)
	reply := make(chan originIntakeAnswer, 1)
	w.mu.Lock()
	if w.stopped || w.ctx.Err() != nil {
		w.mu.Unlock()
		return nil, errOriginIntakeStopped
	}
	select {
	case w.requests <- originIntakeJob{input: &input, ctx: ctx, reply: reply}:
		w.mu.Unlock()
	default:
		w.mu.Unlock()
		return nil, errOriginIntakeFull
	}
	// Deliberately join even after ctx cancellation: the handler may still own
	// a selected-writer or candidate mutation. Stop also drains queued answers.
	answer := <-reply
	return answer.output, answer.err
}

func (w *originIntakeWorker) serve(job originIntakeJob) (output *rpc.UpdateRouteOutput, err error, panicked bool) {
	if job.refusal != nil {
		return nil, job.refusal, false
	}
	var input rpc.OriginIntakeInput
	if job.input != nil {
		input = *job.input
	} else if err := json.Unmarshal(job.wire.Params, &input); err != nil {
		return nil, err, false
	}
	if err := input.Validate(time.Now()); err != nil {
		return nil, err, false
	}
	work, cancel := context.WithDeadline(job.ctx, time.UnixMilli(input.DeadlineMS))
	w.mu.Lock()
	if w.stopped {
		cancel()
	}
	w.activeCancel = cancel
	w.mu.Unlock()
	defer func() { cancel(); w.mu.Lock(); w.activeCancel = nil; w.mu.Unlock() }()
	if err := work.Err(); err != nil {
		return nil, err, false
	}
	defer func() {
		if value := recover(); value != nil {
			output = nil
			err = fmt.Errorf("plugin %s origin-intake handler panicked: %v; restart the deciding plugin", w.plugin.name, value)
			panicked = true
		}
	}()
	output, err = w.handler(work, input)
	if work.Err() != nil {
		err = errors.Join(err, work.Err())
	}
	if output == nil {
		if err != nil {
			return nil, err, false
		}
		return nil, errors.New("origin-intake handler returned no admission result"), false
	}
	if err != nil {
		// Preserve the full partial result on both transports. The result owns its
		// failure field; do not turn a transport error into retained admission.
		copy := *output
		copy.Failure = errors.Join(output.Err(), err).Error()
		output = &copy
	}
	if validation := rpc.ValidateOriginIntakeOutput(input, output); validation != nil {
		return nil, errors.Join(err, validation), false
	}
	resultErr := errors.Join(err, output.Err())
	return output, resultErr, false
}

func (w *originIntakeWorker) reply(id uint64, output any, answerErr error) {
	response, cancel := context.WithTimeout(w.ctx, rpc.RequestCancelTimeout)
	defer cancel()
	var err error
	if output != nil {
		err = w.plugin.engineMux.SendResult(response, id, output)
	} else {
		err = w.plugin.engineMux.SendError(response, id, answerErr.Error())
	}
	if err != nil {
		if w.ctx.Err() == nil {
			w.failure.CompareAndSwap(nil, &err)
		}
		w.plugin.engineMux.Close() //nolint:errcheck // failed reply cannot acknowledge mutation completion
	}
}

func (w *originIntakeWorker) answer(job originIntakeJob, output *rpc.UpdateRouteOutput, err error) {
	if job.reply != nil {
		job.reply <- originIntakeAnswer{output, err}
		return
	}
	// A canceled mutation still finishes its response attempt before enrollment
	// is released. Use the connection budget, not the canceled work context.
	if output != nil {
		w.reply(job.wire.ID, output, nil)
	} else {
		w.reply(job.wire.ID, nil, err)
	}
	job.admission.Finish()
}

func (w *originIntakeWorker) run() {
	defer close(w.done)
	defer w.refuseQueued()
	defer w.stopAdmission()
	for {
		select {
		case <-w.ctx.Done():
			return
		case job := <-w.requests:
			output, err, panicked := w.serve(job)
			w.answer(job, output, err)
			if panicked {
				w.failure.Store(&err)
				if w.plugin.bridge != nil {
					w.plugin.bridge.FailCallbacks(err)
				}
				w.plugin.engineMux.Close() //nolint:errcheck // panic revokes the deciding connection
				w.cancel()
				return
			}
		}
	}
}

func (w *originIntakeWorker) runCancellations() {
	defer close(w.cancelDone)
	defer func() {
		for {
			select {
			case job := <-w.cancellations:
				job.admission.Finish()
			default:
				return
			}
		}
	}()
	for {
		select {
		case <-w.ctx.Done():
			return
		case job := <-w.cancellations:
			var input rpc.CancelRequestInput
			err := job.refusal
			if err == nil {
				err = json.Unmarshal(job.wire.Params, &input)
			}
			if err == nil {
				if input.RequestID == 0 || input.RequestID >= job.wire.ID {
					err = errors.New("cancel-request must name an earlier nonzero request")
				}
				if input.Method != rpc.MethodOriginIntake {
					err = errors.New("cancel-request callback has no mutation admission")
				}
			}
			var output *rpc.CancelRequestOutput
			if err == nil {
				output, err = w.receiver.CancelAndJoin(job.admission.Context, input.RequestID, input.Method)
			}
			if output != nil {
				w.reply(job.wire.ID, output, nil)
			} else {
				w.reply(job.wire.ID, nil, err)
			}
			job.admission.Finish()
		}
	}
}

func (w *originIntakeWorker) stopAdmission() {
	w.mu.Lock()
	w.stopped = true
	w.receiver.Stop()
	if w.activeCancel != nil {
		w.activeCancel()
	}
	w.mu.Unlock()
}

func (w *originIntakeWorker) refuseQueued() {
	for {
		select {
		case job := <-w.requests:
			if job.reply != nil {
				job.reply <- originIntakeAnswer{err: errOriginIntakeStopped}
			} else {
				job.admission.Finish()
				w.plugin.engineMux.Close() //nolint:errcheck // no false settlement for an unserved wire request
			}
		default:
			return
		}
	}
}

// supervise is one connection-lifetime step. It cancels an active typed work
// context when the connection ends, then stops both long-lived channel workers.
func (w *originIntakeWorker) supervise(transportDone <-chan struct{}) {
	defer close(w.supervisorDone)
	select {
	case <-w.ctx.Done():
	case <-transportDone:
	}
	w.stopAdmission()
	w.cancel()
}

func (p *Plugin) startOriginIntakeWorker(ctx context.Context) (func() error, error) {
	p.mu.Lock()
	handler := p.onOriginIntake
	p.mu.Unlock()
	if handler == nil {
		return func() error { return nil }, nil
	}
	workerCtx, cancel := context.WithCancel(ctx)
	w := &originIntakeWorker{plugin: p, handler: handler, ctx: workerCtx, cancel: cancel,
		receiver: rpc.NewRequestReceiver(workerCtx, originIntakeRequestScope),
		requests: make(chan originIntakeJob, originIntakeQueueDepth), cancellations: make(chan originCancellationJob, originIntakeQueueDepth),
		done: make(chan struct{}), cancelDone: make(chan struct{}), supervisorDone: make(chan struct{})}
	remove, err := p.engineMux.RegisterIndependentRequestHandler(rpc.MethodOriginIntake, w.admitWire)
	if err != nil {
		cancel()
		w.receiver.CancelAll()
		return nil, err
	}
	removeCancel, err := p.engineMux.RegisterIndependentRequestHandler(rpc.MethodCancelRequest, w.admitCancellation)
	if err != nil {
		remove()
		cancel()
		w.receiver.CancelAll()
		return nil, err
	}
	var transportDone <-chan struct{}
	if p.bridge != nil {
		p.bridge.SetOriginIntake(w.offer)
	} else {
		transportDone = p.engineMux.Done()
	}
	go w.run()
	go w.runCancellations()
	go w.supervise(transportDone)
	var once sync.Once
	return func() error {
		once.Do(func() {
			remove()
			removeCancel()
			w.stopAdmission()
			cancel()
			<-w.supervisorDone
			<-w.done
			<-w.cancelDone
			w.receiver.CancelAll()
		})
		if failure := w.failure.Load(); failure != nil {
			return *failure
		}
		return nil
	}, nil
}

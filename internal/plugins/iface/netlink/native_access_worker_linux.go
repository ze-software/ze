// Design: docs/features/interfaces.md -- retained original native operations
//go:build linux

package ifacenetlink

import (
	"context"
	"errors"
	"os"
	"sync"

	"github.com/ze-software/ze/internal/component/iface"
)

// nativeVLANWorker owns one admitted operation and no waiting queue. The
// backend/resource retains this worker until Close physically joins it. A
// caller deadline abandons only its answer, never this worker or native IO.
// The worker is a single lifecycle goroutine, never a goroutine per request.
// workerMu (embedded in nativeVLANResources) owns worker/current/stopped.
type nativeVLANWorker struct {
	owner    *nativeVLANResources
	requests chan *nativeVLANRequest
	done     chan struct{}
	current  *nativeVLANRequest
	stopped  bool
}

type nativeVLANRequest struct {
	ctx           context.Context
	call          func(context.Context) (any, error)
	discard       func(any) error
	stopOnSuccess bool
	mu            sync.Mutex
	done          chan struct{}
	value         any
	err           error
	abandoned     bool
}

func nativeVLANContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		return nil, nil, errors.New("native VLAN operation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, nativeVLANOperationLimit)
	return bounded, cancel, nil
}

func (resources *nativeVLANResources) run(ctx context.Context, stop bool, call func(context.Context) (any, error), discard func(any) error) (any, error) {
	bounded, cancel, err := nativeVLANContext(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	resources.workerMu.Lock()
	worker := resources.worker
	if worker == nil {
		// Open the original route socket on the caller's namespace-bearing thread,
		// before launching a Go worker that may run on a different OS thread/netns.
		// No native request or resource mutation is sent during this capture.
		if err := resources.initializeOps(); err != nil {
			resources.workerMu.Unlock()
			return nil, err
		}
		worker = &nativeVLANWorker{owner: resources, requests: make(chan *nativeVLANRequest, 1), done: make(chan struct{})}
		resources.worker = worker
		go worker.loop()
	}
	if worker.stopped {
		resources.workerMu.Unlock()
		return nil, os.ErrClosed
	}
	if worker.current != nil {
		resources.workerMu.Unlock()
		return nil, iface.ErrNativeAccessPending
	}
	request := &nativeVLANRequest{ctx: bounded, call: call, discard: discard, stopOnSuccess: stop, done: make(chan struct{})}
	worker.current = request
	worker.requests <- request // Exactly one slot, reserved above; this send cannot wait.
	resources.workerMu.Unlock()
	select {
	case <-request.done:
		value, err := request.take(bounded)
		if err != nil {
			return nil, err
		}
		if stop {
			select {
			case <-worker.done:
				return value, nil
			case <-bounded.Done():
				return nil, errors.Join(iface.ErrNativeAccessPending, bounded.Err())
			}
		}
		return value, nil
	case <-bounded.Done():
		return nil, request.abandon(bounded.Err())
	}
}

func (request *nativeVLANRequest) take(ctx context.Context) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, request.abandon(err)
	}
	request.mu.Lock()
	defer request.mu.Unlock()
	value, err := request.value, request.err
	request.value = nil
	return value, err
}

func (request *nativeVLANRequest) abandon(cause error) error {
	request.mu.Lock()
	request.abandoned = true
	value := request.value
	request.value = nil
	request.mu.Unlock()
	if value != nil {
		if request.discard != nil {
			return errors.Join(iface.ErrNativeAccessPending, cause, request.discard(value))
		}
	}
	return errors.Join(iface.ErrNativeAccessPending, cause)
}

func (request *nativeVLANRequest) finish(value any, err error) {
	request.mu.Lock()
	if request.abandoned {
		if value != nil {
			if request.discard != nil {
				err = errors.Join(err, request.discard(value))
			}
		}
	} else {
		request.value = value
	}
	request.err = err
	close(request.done)
	request.mu.Unlock()
}

func (worker *nativeVLANWorker) loop() {
	defer close(worker.done)
	// One request is admitted at a time. Successful original Close is the sole
	// stop path; a timeout/error retains the same worker and its transport.
	for request := range worker.requests {
		var value any
		var err error
		if err = request.ctx.Err(); err == nil {
			if qosMapMu.TryLock() {
				value, err = request.call(request.ctx)
				qosMapMu.Unlock()
			} else {
				err = iface.ErrNativeAccessPending
			}
		}
		worker.owner.workerMu.Lock()
		worker.current = nil
		stop := false
		if request.stopOnSuccess {
			if err == nil {
				stop = true
				worker.stopped = true
			}
		}
		worker.owner.workerMu.Unlock()
		// Physical native IO ended before admission reopens. A late-result discard
		// closes only a reference; it never waits on native IO or starts a command.
		request.finish(value, err)
		if stop {
			return
		}
	}
}

// join MUST follow a caller fence that prevents further original commands.
// It proves physical return of the admitted syscall, not whether its mutation
// succeeded. An unknown native outcome remains the original caller's receipt.
func (resources *nativeVLANResources) join(ctx context.Context) error {
	bounded, cancel, err := nativeVLANContext(ctx)
	if err != nil {
		return err
	}
	defer cancel()
	resources.workerMu.Lock()
	worker := resources.worker
	if worker == nil {
		resources.workerMu.Unlock()
		return nil
	}
	current := worker.current
	stopped := worker.stopped
	resources.workerMu.Unlock()
	if stopped {
		select {
		case <-worker.done:
			return nil
		case <-bounded.Done():
			return errors.Join(iface.ErrNativeAccessPending, bounded.Err())
		}
	}
	if current != nil {
		select {
		case <-current.done:
		case <-bounded.Done():
			return errors.Join(iface.ErrNativeAccessPending, bounded.Err())
		}
	}
	_, err = resources.run(bounded, false, func(turn context.Context) (any, error) {
		if resources.ops.resumeOriginal == nil {
			return nil, nil
		} // A source-only fake has no native request socket.
		return nil, resources.ops.resumeOriginal(turn)
	}, nil)
	return err
}

func (lease *nativeVLANLease) JoinNativeVLAN(ctx context.Context) error {
	if lease == nil {
		return errors.New("native VLAN map lease is absent")
	}
	if lease.nativeVLANReference == nil {
		return errors.New("native VLAN map lease is uninitialized")
	}
	if lease.resource == nil {
		return errors.New("native VLAN resource is absent")
	}
	return lease.resource.owner.join(ctx)
}

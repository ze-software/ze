// Design: docs/architecture/plugin/rib-storage-design.md -- non-blocking live reflection during peer-UP replay.
// Related: reflect.go -- live ingress dispatches here while any destination awaits session readiness.
package rr

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type rrForwardTask struct {
	source  peerIdentity
	msgID   uint64
	targets []peerTarget
	label   string
}

type rrForwardLane struct {
	queue   []rrForwardTask
	running bool
}

// rrForwardDispatcher runs live ForwardCached calls off Process.deliveryLoop
// whenever a destination is in peer-UP replay (forwardOrderHold active) or the
// source already has queued async forwards, preserving strict FIFO order per
// source peer without blocking event delivery or cold replay's publication cut.
type rrForwardDispatcher struct {
	mu      sync.Mutex
	stopped bool
	lanes   map[peerIdentity]*rrForwardLane
	wg      sync.WaitGroup
	run     func(rrForwardTask)
}

func newRRForwardDispatcher(run func(rrForwardTask)) *rrForwardDispatcher {
	return &rrForwardDispatcher{
		lanes: make(map[peerIdentity]*rrForwardLane),
		run:   run,
	}
}

// enqueueIf queues task onto source's FIFO lane when held is true or source
// already has an active lane. It returns true when ownership of the caller's
// forwards.enter() token transferred to the worker.
func (d *rrForwardDispatcher) enqueueIf(task rrForwardTask, held bool) bool {
	d.mu.Lock()
	if d.stopped {
		d.mu.Unlock()
		return false
	}
	lane := d.lanes[task.source]
	if !held && lane == nil {
		d.mu.Unlock()
		return false
	}
	if lane == nil {
		lane = &rrForwardLane{}
		d.lanes[task.source] = lane
	}
	lane.queue = append(lane.queue, task)
	if !lane.running {
		lane.running = true
		d.wg.Add(1)
		go d.drain(task.source, lane)
	}
	d.mu.Unlock()
	return true
}

func (d *rrForwardDispatcher) drain(source peerIdentity, lane *rrForwardLane) {
	defer d.wg.Done()
	for {
		d.mu.Lock()
		if len(lane.queue) == 0 {
			lane.running = false
			delete(d.lanes, source)
			d.mu.Unlock()
			return
		}
		task := lane.queue[0]
		lane.queue[0] = rrForwardTask{}
		lane.queue = lane.queue[1:]
		if len(lane.queue) == 0 {
			lane.queue = lane.queue[:0]
		}
		d.mu.Unlock()
		d.run(task)
	}
}

// stop prevents new async enqueues and waits for all active source lanes to
// finish. Callers MUST cancel downCtx first so blocked RPCs return promptly.
func (d *rrForwardDispatcher) stop() {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.stopped = true
	d.mu.Unlock()
	d.wg.Wait()
}

func (rr *routeReflector) runForwardTask(task rrForwardTask) {
	defer rr.wakeCurrentReplay()
	defer rr.forwards.leave()
	ctx, cancel := context.WithTimeout(rr.replayParent(), updateRouteTimeout)
	defer cancel()
	err := rr.forwardCaptured(ctx, task.source, task.msgID, task.targets)
	rr.noteForwardFailure(task.label, err, task.source)
}

func (rr *routeReflector) enqueueForwardIfNeeded(source peerIdentity, msgID uint64, targets []peerTarget, held bool, label string) bool {
	d := rr.forwardDispatch
	if d == nil {
		return false
	}
	return d.enqueueIf(rrForwardTask{
		source:  source,
		msgID:   msgID,
		targets: targets,
		label:   label,
	}, held)
}

// isJoinedDepartedSourceRelayError reports whether err is a joined
// RelayStoredRoute failure caused by a source peer disconnecting (without GR
// retention) while a replay page was in flight.
func isJoinedDepartedSourceRelayError(err error) bool {
	if err == nil {
		return false
	}
	var joined *rpc.CachedWriteJoinedError
	if !errors.As(err, &joined) {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "original cold relay source ended without retained membership") ||
		strings.Contains(msg, "received UPDATE delivery stopped before publication") ||
		strings.Contains(msg, "source peer is not an established peer")
}

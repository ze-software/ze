// Design: docs/architecture/api/process-protocol.md -- exact restart ownership.

package server

import (
	"context"
	"errors"
	"sync"
)

var errRestartControllerBusy = errors.New("another restart of this plugin is still in progress")

// restartControllerSet holds only active controllers. Its zero value is empty
// and ready. Different configured names can restart independently.
type restartControllerSet struct {
	mu     sync.Mutex
	active map[string]*restartController
}

// restartController reserves one configured name. Its zero and nil values are
// uninitialized. Only tryAcquire and wait return a usable reservation.
type restartController struct {
	set  *restartControllerSet
	name string
	done chan struct{}
}

// tryAcquire refuses an overlapping controller before it can replace a process
// or record it stopped. A successful caller MUST defer release immediately.
func (set *restartControllerSet) tryAcquire(name string) (*restartController, error) {
	set.mu.Lock()
	defer set.mu.Unlock()
	if set.active[name] != nil {
		return nil, errRestartControllerBusy
	}
	return set.reserveLocked(name), nil
}

// reserveLocked creates the reservation. The caller MUST hold set.mu and have
// established that no controller owns name. Its caller MUST release the result.
func (set *restartControllerSet) reserveLocked(name string) *restartController {
	if set.active == nil {
		set.active = make(map[string]*restartController)
	}
	controller := &restartController{set: set, name: name, done: make(chan struct{})}
	set.active[name] = controller
	return controller
}

// wait retains a failure controller until the preceding controller settles or
// the daemon ends. A successful caller MUST defer release immediately and
// recheck its exact outgoing process before replacing anything.
func (set *restartControllerSet) wait(ctx context.Context, name string) (*restartController, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		set.mu.Lock()
		previous := set.active[name]
		if previous == nil {
			controller := set.reserveLocked(name)
			set.mu.Unlock()
			return controller, nil
		}
		set.mu.Unlock()
		select {
		case <-previous.done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// release ends the reservation. Its successful acquirer MUST call it exactly
// once, after replacement handshake, cleanup, and result recording have ended.
func (controller *restartController) release() {
	controller.set.mu.Lock()
	defer controller.set.mu.Unlock()
	delete(controller.set.active, controller.name)
	close(controller.done)
}

// Design: docs/guide/route-reflection.md -- removing a client's last import policy
// Related: rr.go -- both event paths pass every received UPDATE through the gate
// Related: source_policy.go -- settleSource waits for the gate

package rr

import (
	"context"
	"errors"
	"sync"
)

// forwardGate counts the received UPDATEs being reflected, from the
// withdrawal-map update to the engine's answer to the forward, so a
// source-policy phase can wait for them (settleSource). Event delivery is
// serial, so the count is zero or one and the gate reaches idle between
// events. The zero value is ready. Safe for concurrent use.
type forwardGate struct {
	mu      sync.Mutex
	running int
	// idle is made by the first waiter and closed when running returns to
	// zero, so an UPDATE with no waiter costs no allocation.
	idle chan struct{}
}

// enter counts one UPDATE being reflected. MUST be paired with leave.
func (g *forwardGate) enter() {
	g.mu.Lock()
	g.running++
	g.mu.Unlock()
}

// leave counts one UPDATE done, waking every waiter when none is left. MUST
// be called once after each enter.
func (g *forwardGate) leave() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.running--
	if g.running > 0 {
		return
	}
	if g.idle == nil {
		return
	}
	close(g.idle)
	g.idle = nil
}

// wait returns once no UPDATE is being reflected, and refuses when ctx ends
// first.
func (g *forwardGate) wait(ctx context.Context) error {
	g.mu.Lock()
	if g.running == 0 {
		g.mu.Unlock()
		return nil
	}
	if g.idle == nil {
		g.idle = make(chan struct{})
	}
	idle := g.idle
	g.mu.Unlock()
	select {
	case <-idle:
		return nil
	case <-ctx.Done():
		return errors.New("the forward is still with the engine: " + ctx.Err().Error())
	}
}

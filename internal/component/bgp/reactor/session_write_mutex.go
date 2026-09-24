// Design: docs/architecture/bgp/structural-forwarding.md -- request-scoped route writes
package reactor

import (
	"context"
	"sync"
)

// sessionWriteMutex has zero-value mutex semantics and cancellable acquisition.
// Its token is allocated once per session, never once per message. A canceled
// waiter never acquires ownership later or interrupts the current writer.
type sessionWriteMutex struct {
	once  sync.Once
	token chan struct{}
}

func (m *sessionWriteMutex) init() {
	m.once.Do(func() { m.token = make(chan struct{}, 1) })
}

func (m *sessionWriteMutex) Lock() {
	m.init()
	m.token <- struct{}{}
}

func (m *sessionWriteMutex) LockContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.init()
	select {
	case m.token <- struct{}{}:
		if err := ctx.Err(); err != nil {
			m.Unlock()
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *sessionWriteMutex) TryLock() bool {
	m.init()
	select {
	case m.token <- struct{}{}:
		return true
	default:
		return false
	}
}

func (m *sessionWriteMutex) Unlock() {
	select {
	case <-m.token:
	default:
		panic("unlock of unlocked session write mutex")
	}
}

// Design: docs/architecture/plugin/rib-storage-design.md -- recovery delivery ordering.
package process

import (
	"context"
	"sync"
	"testing"
)

// RecoveryDeliveryGate is test-only instrumentation. The original production
// deliveryLoop owns processing and Result replies; this proxy owns only FIFO
// admission and the original queue's close. Release MUST precede process Stop;
// Wait MUST follow it. Neither the matcher nor a held event owns a RIB/write lock.
// Only GateRecoveryDelivery constructs a usable gate; nil and zero are invalid.
type RecoveryDeliveryGate struct {
	Held     chan struct{}
	Barriers chan struct{}
	release  chan struct{}
	done     chan struct{}
	once     sync.Once
}

// GateRecoveryDelivery drains the real loop before replacing its producer queue.
// The answered drain proves deliveryLoop already captured the original queue.
// No production handler or transport is replaced.
func GateRecoveryDelivery(t *testing.T, ctx context.Context, p *Process, match func(EventDelivery) bool) *RecoveryDeliveryGate {
	t.Helper()
	if err := p.DrainEvents(ctx); err != nil {
		t.Fatal(err)
	}
	gate := &RecoveryDeliveryGate{
		Held: make(chan struct{}), Barriers: make(chan struct{}, 64),
		release: make(chan struct{}), done: make(chan struct{}),
	}
	p.eventMu.Lock()
	original := p.eventQueue
	incoming := newEventQueue(original.MaxCap())
	p.eventQueue = incoming
	p.eventMu.Unlock()
	go gate.forward(t, incoming, original, match)
	return gate
}

// forward runs until Stop closes incoming. Every admitted item, including every
// Result and Barrier, transfers unchanged exactly once to the real deliveryLoop.
// A fixed pending bound makes overload a fixture failure, never a dropped item.
func (gate *RecoveryDeliveryGate) forward(t *testing.T, incoming, original *eventQueue, match func(EventDelivery) bool) {
	defer close(gate.done)
	defer original.Close()

	incomingCh := make(chan EventDelivery)
	go func() {
		defer close(incomingCh)
		var buf []EventDelivery
		for {
			var ok bool
			buf, ok = incoming.drainBatch(buf, eventDeliveryBatchMax, nil)
			if !ok {
				return
			}
			for _, item := range buf {
				incomingCh <- item
			}
		}
	}()

	pending := make([]EventDelivery, 0, 128)
	release := gate.release
	held := false
	matched := false
	for incomingCh != nil || len(pending) != 0 {
		if !held && len(pending) != 0 {
			for len(pending) != 0 {
				if got := original.enqueue(context.Background(), nil, nil, pending[0], nil, nil); got != enqueueAdmitted {
					t.Errorf("recovery fixture failed to forward delivery: %v", got)
				}
				copy(pending, pending[1:])
				pending[len(pending)-1] = EventDelivery{}
				pending = pending[:len(pending)-1]
			}
			continue
		}
		select {
		case delivery, open := <-incomingCh:
			if !open {
				incomingCh = nil
				held = false
				continue
			}
			if !matched && match(delivery) {
				matched = true
				held = release != nil
				close(gate.Held)
			}
			if delivery.Barrier {
				select {
				case gate.Barriers <- struct{}{}:
				default:
					t.Error("recovery fixture exceeded barrier observation bound")
				}
			}
			if len(pending) == cap(pending) {
				t.Error("recovery fixture exceeded bounded delivery queue")
				// Preserve ownership even on failure; unblock and forward rather
				// than losing a Result or returning a pooled event prematurely.
				held = false
				if got := original.enqueue(context.Background(), nil, nil, pending[0], nil, nil); got != enqueueAdmitted {
					t.Errorf("recovery fixture failed to forward overflow delivery: %v", got)
				}
				copy(pending, pending[1:])
				pending[len(pending)-1] = EventDelivery{}
				pending = pending[:len(pending)-1]
			}
			pending = append(pending, delivery)
		case <-release:
			held = false
			release = nil
		}
	}
}

func (gate *RecoveryDeliveryGate) Release() { gate.once.Do(func() { close(gate.release) }) }

func (gate *RecoveryDeliveryGate) Wait(ctx context.Context) error {
	select {
	case <-gate.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

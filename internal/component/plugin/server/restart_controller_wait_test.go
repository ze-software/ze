// Design: docs/architecture/api/process-protocol.md -- automatic restart ownership.

package server

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type restartControllerWaitContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (ctx *restartControllerWaitContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.entered) })
	return ctx.Context.Done()
}

type restartControllerWaitResult struct {
	controller *restartController
	err        error
}

// runRestartControllerWait MUST publish its result and close done before the
// test's cleanup returns. The caller MUST cancel and join it, even on failure.
func runRestartControllerWait(ctx context.Context, set *restartControllerSet, name string, result chan<- restartControllerWaitResult, done chan<- struct{}) {
	defer close(done)
	controller, err := set.wait(ctx, name)
	result <- restartControllerWaitResult{controller: controller, err: err}
}

// A replacement can fail as its predecessor's controller finishes. Its failure
// controller must retain that request until the name is available, then check
// its exact outgoing generation. It must also stop waiting with the daemon.
func TestRestartControllerWaitsForPredecessor(t *testing.T) {
	for _, cancelWait := range []bool{false, true} {
		name := "release"
		if cancelWait {
			name = "cancel"
		}
		t.Run(name, func(t *testing.T) {
			var set restartControllerSet
			first, err := set.tryAcquire("owner")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if first != nil {
					first.release()
				}
			})
			lifetime, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			ctx := &restartControllerWaitContext{Context: lifetime, entered: make(chan struct{})}
			result := make(chan restartControllerWaitResult, 1)
			done := make(chan struct{})
			go runRestartControllerWait(ctx, &set, "owner", result, done)
			t.Cleanup(func() {
				cancel()
				<-done
				select {
				case value := <-result:
					if value.controller != nil {
						value.controller.release()
					}
				default:
				}
			})
			select {
			case <-ctx.entered:
			case value := <-result:
				if value.controller != nil {
					value.controller.release()
				}
				t.Fatalf("failure controller returned before its predecessor settled: %v", value.err)
			case <-lifetime.Done():
				t.Fatal("failure controller never reached its ownership wait")
			}
			other, err := set.tryAcquire("independent")
			if err != nil {
				t.Fatalf("one name blocked an independent restart: %v", err)
			}
			other.release()
			if cancelWait {
				cancel()
			} else {
				first.release()
				first = nil
			}
			value := <-result
			if cancelWait {
				if !errors.Is(value.err, context.Canceled) {
					t.Fatalf("daemon stop did not end the controller wait: %v", value.err)
				}
				return
			}
			if value.err != nil {
				t.Fatal(value.err)
			}
			defer value.controller.release()
			if overlapping, err := set.tryAcquire("owner"); err == nil {
				overlapping.release()
				t.Fatal("resumed failure controller did not reserve its name")
			}
		})
	}
}

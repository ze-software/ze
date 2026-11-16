// Design: docs/architecture/plugin-manager-wiring.md -- initialized plugin startup

package plugin_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/engine"
	plugin "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/pkg/ze"
)

// Engine's real Start must join its plugin manager's initialization before
// exposing any subsystem. A held runner makes the ordering deterministic.
func TestStartupRunnerPrecedesSubsystems(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager := plugin.NewManager()
		entered, release := make(chan struct{}), make(chan struct{})
		if err := manager.SetStartupRunner(func(ctx context.Context) error {
			close(entered)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}); err != nil {
			t.Fatal(err)
		}
		subsystem := &startupOrderSubsystem{started: make(chan struct{})}
		router := engine.NewEngine(&stubBus{}, &stubConfig{}, manager)
		if err := router.RegisterSubsystem(subsystem); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- router.Start(context.Background()) }()
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("plugin initialization runner was not called")
		}
		synctest.Wait()
		select {
		case <-subsystem.started:
			t.Fatal("subsystem started before plugin initialization")
		default:
		}
		close(release)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		select {
		case <-subsystem.started:
		default:
			t.Fatal("subsystem did not start after initialization")
		}
		if err := router.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
}

// A failed initializer is not a started manager. Its resources are stopped and
// a subsequent attempt runs the same declared initializer again.
func TestStartupRunnerFailureCanRetry(t *testing.T) {
	manager := plugin.NewManager()
	failure := errors.New("initialization refused")
	attempts := 0
	if err := manager.SetStartupRunner(func(context.Context) error {
		attempts++
		if attempts == 1 {
			return failure
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := manager.StartAll(context.Background(), &stubBus{}, &stubConfig{}); !errors.Is(err, failure) {
		t.Fatalf("first start = %v", err)
	}
	if err := manager.StartAll(context.Background(), &stubBus{}, &stubConfig{}); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("initializer attempts = %d", attempts)
	}
	if err := manager.StopAll(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// Stop cancels the initializer and joins its actual return. Cancellation alone
// is not a receipt: this runner deliberately retains its resources until released.
func TestStopAllJoinsStartupRunner(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager := plugin.NewManager()
		entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
		if err := manager.SetStartupRunner(func(ctx context.Context) error {
			close(entered)
			<-ctx.Done()
			close(canceled)
			<-release
			return ctx.Err()
		}); err != nil {
			t.Fatal(err)
		}
		started := make(chan error, 1)
		go func() { started <- manager.StartAll(context.Background(), &stubBus{}, &stubConfig{}) }()
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("plugin initialization runner was not called")
		}
		stopped := make(chan error, 1)
		go func() { stopped <- manager.StopAll(context.Background()) }()
		<-canceled
		synctest.Wait()
		select {
		case err := <-stopped:
			t.Fatalf("StopAll returned before initializer joined: %v", err)
		default:
		}
		close(release)
		if err := <-started; !errors.Is(err, context.Canceled) {
			t.Fatalf("start cancellation = %v", err)
		}
		if err := <-stopped; err != nil {
			t.Fatal(err)
		}
	})
}

type startupOrderSubsystem struct{ started chan struct{} }

func (*startupOrderSubsystem) Name() string { return "startup-order-test" }
func (s *startupOrderSubsystem) Start(context.Context, ze.EventBus, ze.ConfigProvider) error {
	close(s.started)
	return nil
}
func (*startupOrderSubsystem) Stop(context.Context) error                      { return nil }
func (*startupOrderSubsystem) Reload(context.Context, ze.ConfigProvider) error { return nil }

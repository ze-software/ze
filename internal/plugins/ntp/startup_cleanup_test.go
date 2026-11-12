// Design: docs/architecture/api/process-protocol.md -- startup refusal releases resources already configured.
// Related: register.go -- the NTP runner owns its worker and DHCP subscription.

package ntp

import (
	"context"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// ntpStartupBus counts the actual DHCP subscription and its removal.
type ntpStartupBus struct {
	subscribed   atomic.Int32
	unsubscribed atomic.Int32
}

func (*ntpStartupBus) Emit(string, string, any) (int, error) { return 0, nil }

func (b *ntpStartupBus) Subscribe(string, string, func(any)) func() {
	b.subscribed.Add(1)
	var once sync.Once
	return func() { once.Do(func() { b.unsubscribed.Add(1) }) }
}

// ntpWorkerGoroutines counts real worker loops, including workers parked in
// sleepOrStop. The runner's stop/join contract requires none remain on return.
// This test is serial: parallel worker tests run after serial tests complete.
func ntpWorkerGoroutines() int {
	var records []runtime.StackRecord
	for {
		count, ok := runtime.GoroutineProfile(records)
		if ok {
			records = records[:count]
			break
		}
		records = make([]runtime.StackRecord, count+16)
	}
	count := 0
	for i := range records {
		frames := runtime.CallersFrames(records[i].Stack())
		for {
			frame, more := frames.Next()
			if frame.Function == "github.com/ze-software/ze/internal/plugins/ntp.(*syncWorker).run" {
				count++
				break
			}
			if !more {
				break
			}
		}
	}
	return count
}

// TestNTPStartupFailureReleasesConfiguredWorker drives a real Stage-2 configure
// followed by a Stage-3 refusal. A real worker publishes before the failure;
// its goroutine and DHCP subscription must both be gone when the runner exits.
// No servers and no state store make every clock-adjustment path unreachable.
func TestNTPStartupFailureReleasesConfiguredWorker(t *testing.T) {
	previousBus, previousState, previousStore := getEventBus(), loadState(), statestore.Store()
	bus := &ntpStartupBus{}
	setEventBus(bus)
	storeState(nil)
	statestore.SetStore(nil)
	t.Cleanup(func() {
		setEventBus(previousBus)
		storeState(previousState)
		statestore.SetStore(previousStore)
	})
	beforeWorkers := ntpWorkerGoroutines()
	pluginSide, engineSide := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineSide, engineSide))
	t.Cleanup(func() { _ = mux.Close(); _ = pluginSide.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan int, 1)
	go func() { result <- runNTPPlugin(pluginSide) }()
	var request *rpc.Request
	select {
	case request = <-mux.Requests():
	case <-ctx.Done():
		t.Fatal("NTP did not declare its registration")
	}
	if request == nil || request.Method != rpc.MethodDeclareRegistration {
		t.Fatalf("unexpected registration request: %v", request)
	}
	if err := mux.SendResult(ctx, request.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}); err != nil {
		t.Fatal(err)
	}
	_, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", struct {
		Sections []sdk.ConfigSection `json:"sections"`
	}{Sections: []sdk.ConfigSection{{Root: configRootEnvironment, Data: `{"environment":{"ntp":{"enabled":"true","interval":"60"}}}`}}})
	if err != nil {
		t.Fatalf("NTP configure failed before the intended later refusal: %v", err)
	}
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for loadState() == nil {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("configured NTP worker did not publish")
		}
	}
	if !loadState().Enabled || ntpWorkerGoroutines() != beforeWorkers+1 || bus.subscribed.Load() != 1 {
		t.Fatalf("configure did not start exactly one real worker and subscription: state=%v workers=%d subscribed=%d",
			loadState(), ntpWorkerGoroutines(), bus.subscribed.Load())
	}
	select {
	case request = <-mux.Requests():
	case <-ctx.Done():
		t.Fatal("NTP did not reach stage 3")
	}
	if request == nil || request.Method != "ze-plugin-engine:declare-capabilities" {
		t.Fatalf("unexpected stage-3 request: %v", request)
	}
	if err := mux.SendError(ctx, request.ID, "test engine refused stage 3 after configure"); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-result:
		if code != 1 {
			t.Errorf("refused startup exit=%d, want 1", code)
		}
	case <-ctx.Done():
		t.Fatal("NTP runner did not complete its failed-startup cleanup")
	}
	if got := bus.unsubscribed.Load(); got != 1 {
		t.Errorf("failed startup removed %d DHCP subscriptions, want 1", got)
	}
	if got := ntpWorkerGoroutines(); got != beforeWorkers {
		t.Errorf("failed startup left %d NTP workers, want baseline %d", got, beforeWorkers)
	}
}

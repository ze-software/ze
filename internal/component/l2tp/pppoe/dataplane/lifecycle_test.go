// Design: docs/architecture/l2tp/bng-5-pppoe.md -- registered provider lifecycle
// Related: lifecycle.go -- selected subscription ownership and admission permits

package dataplane

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/ze"
)

// TestLifecycleRegistrySelectsOnlyRegisteredProvider observes subscription
// factories without opening a native session or changing the global registry.
func TestLifecycleRegistrySelectsOnlyRegisteredProvider(t *testing.T) {
	var registry sessionRegistry
	selected := 0
	other := 0
	if err := registry.register("selected", testUnusedOpen, testUnusedPrepare,
		func(ze.EventBus, LifecycleSignals) (func(), error) {
			selected++
			return func() {}, nil
		}); err != nil {
		t.Fatal(err)
	}
	if err := registry.register("other", testUnusedOpen, testUnusedPrepare,
		func(ze.EventBus, LifecycleSignals) (func(), error) {
			other++
			return func() {}, nil
		}); err != nil {
		t.Fatal(err)
	}
	backend, err := registry.lookup("selected")
	if err != nil {
		t.Fatal(err)
	}
	watch, supported, err := backend.WatchLifecycle(&lifecycleTestBus{})
	if err != nil {
		t.Fatal(err)
	}
	if !supported {
		t.Fatal("selected lifecycle capability absent")
	}
	t.Cleanup(watch.Close)
	if selected != 1 {
		t.Fatalf("selected subscriptions = %d, want 1", selected)
	}
	if other != 0 {
		t.Fatalf("other subscriptions = %d, want 0", other)
	}
}

// TestLifecycleStableBackendHasExplicitAbsence keeps three-argument providers
// independent of lifecycle events and does not require an otherwise unused bus.
func TestLifecycleStableBackendHasExplicitAbsence(t *testing.T) {
	var registry sessionRegistry
	if err := registry.register("stable", testUnusedOpen, testUnusedPrepare); err != nil {
		t.Fatal(err)
	}
	backend, err := registry.lookup("stable")
	if err != nil {
		t.Fatal(err)
	}
	watch, supported, err := backend.WatchLifecycle(nil)
	if err != nil {
		t.Fatal(err)
	}
	if supported {
		t.Fatal("stable provider invented a lifecycle capability")
	}
	if watch != nil {
		t.Fatal("stable provider returned a subscription")
	}
}

// TestLifecycleRegistryRefusesExplicitNil distinguishes omitted support from
// a requested capability whose factory was never supplied.
func TestLifecycleRegistryRefusesExplicitNil(t *testing.T) {
	var registry sessionRegistry
	if err := registry.register("broken", testUnusedOpen, testUnusedPrepare, nil); err == nil {
		t.Fatal("accepted explicitly nil lifecycle factory")
	}
}

// TestLifecycleRegistryRefusesMultipleFactories proves a selection cannot
// attach more than one provider-owned notification source.
func TestLifecycleRegistryRefusesMultipleFactories(t *testing.T) {
	var registry sessionRegistry
	subscribe := func(ze.EventBus, LifecycleSignals) (func(), error) { return func() {}, nil }
	if err := registry.register("broken", testUnusedOpen, testUnusedPrepare, subscribe, subscribe); err == nil { //nolint:gocritic // dupOption: the same factory is deliberately supplied twice to prove the two-factory count is refused.
		t.Fatal("accepted two lifecycle factories")
	}
}

// TestLifecycleInitialObservationNeverAdmits treats initial wakeup as a retry
// hint. Actual access preparation and prior-loss retirement remain the worker's.
func TestLifecycleInitialObservationNeverAdmits(t *testing.T) {
	watch, _ := lifecycleTestWatch(t)
	observation := watch.Snapshot()
	if !observation.Valid() {
		t.Fatal("constructed watch returned invalid observation")
	}
	if !observation.RetryRequested() {
		t.Fatal("initial preparation hint absent")
	}
	if observation.LossSequence() != 0 {
		t.Fatal("initial preparation invented a loss")
	}
	if _, admitted := watch.Admission(); admitted {
		t.Fatal("initial hint admitted discovery")
	}
	select {
	case <-watch.Changed():
	default:
		t.Fatal("initial worker wake absent")
	}
}

// TestLifecycleLossInvalidatesOriginalPermit holds the exact admitted permit
// across a loss and proves a subsequent Ready cannot restore that authority.
func TestLifecycleLossInvalidatesOriginalPermit(t *testing.T) {
	watch, signals := lifecycleTestWatch(t)
	if !watch.Admit(watch.Snapshot()) {
		t.Fatal("actual worker admission refused")
	}
	permit, admitted := watch.Admission()
	if !admitted {
		t.Fatal("admitted watch returned no permit")
	}
	signals.Lost()
	if watch.Current(permit) {
		t.Fatal("lost provider retained original admission permit")
	}
	signals.Ready()
	if _, admitted := watch.Admission(); admitted {
		t.Fatal("Ready hint reopened discovery")
	}
	if watch.Current(permit) {
		t.Fatal("Ready restored original admission permit")
	}
}

// TestLifecycleLossIsStickyAcrossCoalescedReady observes only the latest
// snapshot after Lost/Ready/Lost and duplicate loss, as a delayed worker does.
func TestLifecycleLossIsStickyAcrossCoalescedReady(t *testing.T) {
	watch, signals := lifecycleTestWatch(t)
	signals.Lost()
	signals.Ready()
	signals.Lost()
	signals.Lost()
	observation := watch.Snapshot()
	if observation.LossSequence() != 2 {
		t.Fatalf("loss sequence = %d, want 2", observation.LossSequence())
	}
	if observation.RetryRequested() {
		t.Fatal("latest loss retained an earlier Ready hint")
	}
	if watch.Admit(observation) {
		t.Fatal("unavailable observation admitted discovery")
	}
	signals.Ready()
	if watch.Snapshot().LossSequence() != 2 {
		t.Fatal("Ready erased unprocessed losses")
	}
}

// TestLifecycleDuplicateReadyPreservesAdmittedPermit sends duplicate readiness
// hints while an existing constructor retains the unchanged original permit.
func TestLifecycleDuplicateReadyPreservesAdmittedPermit(t *testing.T) {
	watch, signals := lifecycleTestWatch(t)
	if !watch.Admit(watch.Snapshot()) {
		t.Fatal("worker admission refused")
	}
	permit, admitted := watch.Admission()
	if !admitted {
		t.Fatal("admission absent")
	}
	signals.Ready()
	signals.Ready()
	if !watch.Current(permit) {
		t.Fatal("duplicate Ready invalidated unchanged admission")
	}
}

// TestLifecycleStaleObservationCannotAdmit captures the worker's candidate
// before a loss and proves its completed old preparation cannot reopen it.
func TestLifecycleStaleObservationCannotAdmit(t *testing.T) {
	watch, signals := lifecycleTestWatch(t)
	observation := watch.Snapshot()
	signals.Lost()
	signals.Ready()
	if watch.Admit(observation) {
		t.Fatal("stale preparation admitted after loss")
	}
	if !watch.Admit(watch.Snapshot()) {
		t.Fatal("new observation could not admit after actual recovery")
	}
}

// TestLifecycleForeignObservationCannotAdmit uses two real selections to prove
// one watch cannot authorize a second watch, even with equal loss counters.
func TestLifecycleForeignObservationCannotAdmit(t *testing.T) {
	first, _ := lifecycleTestWatch(t)
	second, _ := lifecycleTestWatch(t)
	if second.Admit(first.Snapshot()) {
		t.Fatal("foreign observation admitted another subscription")
	}
	if second.Admit(LifecycleObservation{}) {
		t.Fatal("zero observation admitted discovery")
	}
	if second.Current(LifecyclePermit{}) {
		t.Fatal("zero permit authorized discovery")
	}
}

// TestLifecycleWakeIsBounded preserves every loss sequence while coalescing
// a thousand undrained event pairs into the existing capacity-one wake.
func TestLifecycleWakeIsBounded(t *testing.T) {
	watch, signals := lifecycleTestWatch(t)
	for range 1000 {
		signals.Lost()
		signals.Ready()
	}
	if watch.Snapshot().LossSequence() != 1000 {
		t.Fatal("bounded wake discarded a loss")
	}
	if len(watch.Changed()) != 1 {
		t.Fatalf("pending wakes = %d, want 1", len(watch.Changed()))
	}
}

// TestLifecycleCloseFencesCopiedCallback models EventBus's actual snapshot
// contract: an unsubscribed callback may still run once after Close returns.
func TestLifecycleCloseFencesCopiedCallback(t *testing.T) {
	var signals LifecycleSignals
	unsubscribed := 0
	watch := lifecycleTestSubscribe(t, func(_ ze.EventBus, got LifecycleSignals) (func(), error) {
		signals = got
		return func() { unsubscribed++ }, nil
	})
	if !watch.Admit(watch.Snapshot()) {
		t.Fatal("worker admission refused")
	}
	permit, admitted := watch.Admission()
	if !admitted {
		t.Fatal("admission absent")
	}
	watch.Close()
	closed := watch.Snapshot()
	signals.Ready()
	signals.Lost()
	watch.Close()
	if unsubscribed != 1 {
		t.Fatalf("unsubscribe calls = %d, want 1", unsubscribed)
	}
	if !closed.Closed() {
		t.Fatal("Close did not publish closed observation")
	}
	if watch.Snapshot() != closed {
		t.Fatal("copied callback changed closed subscription")
	}
	if watch.Current(permit) {
		t.Fatal("Close retained an admitted permit")
	}
	if watch.Admit(closed) {
		t.Fatal("closed observation admitted discovery")
	}
}

// TestLifecycleRefusedSubscriptionClosesOriginalOwner preserves a partial
// provider's cause and invokes its returned unsubscribe before refusing it.
func TestLifecycleRefusedSubscriptionClosesOriginalOwner(t *testing.T) {
	want := errors.New("partial subscription")
	var signals LifecycleSignals
	unsubscribed := 0
	var registry sessionRegistry
	if err := registry.register("partial", testUnusedOpen, testUnusedPrepare,
		func(_ ze.EventBus, got LifecycleSignals) (func(), error) {
			signals = got
			return func() { unsubscribed++ }, want
		}); err != nil {
		t.Fatal(err)
	}
	backend, err := registry.lookup("partial")
	if err != nil {
		t.Fatal(err)
	}
	watch, supported, err := backend.WatchLifecycle(&lifecycleTestBus{})
	if !supported {
		t.Fatal("registered but refused capability presented as absent")
	}
	if watch != nil {
		t.Fatal("refused subscription returned a watch")
	}
	if !errors.Is(err, want) {
		t.Fatalf("original cause lost: %v", err)
	}
	if unsubscribed != 1 {
		t.Fatal("partial subscription owner not released")
	}
	signals.Ready()
	signals.Lost()
}

// TestLifecycleChangingProviderRequiresBus refuses a nil or typed-nil bus
// before invoking its declared subscription factory.
func TestLifecycleChangingProviderRequiresBus(t *testing.T) {
	var registry sessionRegistry
	called := false
	if err := registry.register("selected", testUnusedOpen, testUnusedPrepare,
		func(ze.EventBus, LifecycleSignals) (func(), error) {
			called = true
			return func() {}, nil
		}); err != nil {
		t.Fatal(err)
	}
	backend, err := registry.lookup("selected")
	if err != nil {
		t.Fatal(err)
	}
	for _, bus := range []ze.EventBus{nil, (*lifecycleTestBus)(nil)} {
		watch, supported, err := backend.WatchLifecycle(bus)
		if !supported {
			t.Fatal("missing bus disguised registered capability as absent")
		}
		if watch != nil {
			t.Fatal("nil bus returned a watch")
		}
		if err == nil {
			t.Fatal("nil bus accepted")
		}
	}
	if called {
		t.Fatal("nil bus reached provider factory")
	}
}

// TestLifecycleSubscriptionRequiresUnsubscribe refuses a provider that cannot
// return ownership of its attachment, rather than making Stop a silent no-op.
func TestLifecycleSubscriptionRequiresUnsubscribe(t *testing.T) {
	var registry sessionRegistry
	if err := registry.register("selected", testUnusedOpen, testUnusedPrepare,
		func(ze.EventBus, LifecycleSignals) (func(), error) { return nil, nil }); err != nil { //nolint:nilnil // Deliberately malformed provider fixture: missing unsubscribe ownership must be refused.
		t.Fatal(err)
	}
	backend, err := registry.lookup("selected")
	if err != nil {
		t.Fatal(err)
	}
	watch, supported, err := backend.WatchLifecycle(&lifecycleTestBus{})
	if !supported {
		t.Fatal("broken capability presented as absent")
	}
	if watch != nil {
		t.Fatal("unowned subscription returned watch")
	}
	if err == nil {
		t.Fatal("subscription with no unsubscribe accepted")
	}
}

// TestLifecycleLossSequenceExhaustionRefuses permanently seals the watch
// before an exhausted counter could alias a much older zero-loss generation.
func TestLifecycleLossSequenceExhaustionRefuses(t *testing.T) {
	watch, signals := lifecycleTestWatch(t)
	watch.state.Store(&lifecycleState{phase: lifecycleCandidate, loss: math.MaxUint64})
	signals.Lost()
	observation := watch.Snapshot()
	if !observation.Closed() {
		t.Fatal("exhausted loss identity remained open")
	}
	if observation.LossSequence() != math.MaxUint64 {
		t.Fatal("loss counter wrapped")
	}
	if observation.Failure() == nil {
		t.Fatal("loss exhaustion had no explicit failure")
	}
	signals.Ready()
	if watch.Admit(watch.Snapshot()) {
		t.Fatal("Ready reopened exhausted watch")
	}
}

// TestLifecycleConcurrentCloseLossReadyAndAdmit starts the four actual
// operations at one barrier, holds the original unsubscribe so Close remains
// active, observes its real fence, and joins every fixed test worker. Assertions
// use state and receipts; the timer only bounds a broken lifecycle join.
func TestLifecycleConcurrentCloseLossReadyAndAdmit(t *testing.T) {
	var signals LifecycleSignals
	var unsubscribed atomic.Int64
	unsubscribeEntered := make(chan struct{})
	releaseUnsubscribe := make(chan struct{})
	var releaseOnce sync.Once
	watch := lifecycleTestSubscribe(t, func(_ ze.EventBus, got LifecycleSignals) (func(), error) {
		signals = got
		return func() {
			unsubscribed.Add(1)
			close(unsubscribeEntered)
			<-releaseUnsubscribe
		}, nil
	})
	// This release is registered before any assertion can fail; the original
	// Watch.Close cleanup must never wait on a gate the test left closed.
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(releaseUnsubscribe) })
	})
	initial := watch.Snapshot()
	if !watch.Admit(initial) {
		t.Fatal("original preparation was not admitted")
	}
	permit, admitted := watch.Admission()
	if !admitted {
		t.Fatal("original admitted permit absent")
	}
	begin := make(chan struct{})
	prepared := make(chan struct{}, 4)
	joined := make(chan struct{})
	var workers sync.WaitGroup
	operations := []func(){
		watch.Close,
		signals.Lost,
		signals.Ready,
		func() { watch.Admit(watch.Snapshot()) },
	}
	workers.Add(len(operations))
	// These four fixed lifecycle test helpers all join below; none is an
	// event-driven worker or production scheduling injection.
	for _, operation := range operations {
		go func(run func()) {
			defer workers.Done()
			prepared <- struct{}{}
			<-begin
			run()
		}(operation)
	}
	go func() {
		workers.Wait()
		close(joined)
	}()
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(releaseUnsubscribe) })
		select {
		case <-joined:
		case <-time.After(5 * time.Second):
			t.Error("fixed lifecycle test workers failed to join")
		}
	})
	for range operations {
		<-prepared
	}
	close(begin)
	select {
	case <-unsubscribeEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent Close never reached original unsubscribe")
	}
	closed := watch.Snapshot()
	if !closed.Closed() {
		t.Fatal("concurrent Close did not fence before unsubscribe")
	}
	if watch.Current(permit) {
		t.Fatal("original permit survived concurrent Close")
	}
	if _, admitted := watch.Admission(); admitted {
		t.Fatal("concurrent lifecycle operations retained admission during Close")
	}
	releaseOnce.Do(func() { close(releaseUnsubscribe) })
	select {
	case <-joined:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent lifecycle operations did not join")
	}
	watch.Close()
	signals.Lost()
	signals.Ready()
	if watch.Admit(initial) {
		t.Fatal("old preparation reopened after concurrent Close")
	}
	if watch.Admit(watch.Snapshot()) {
		t.Fatal("closed preparation reopened after concurrent Close")
	}
	if watch.Snapshot() != closed {
		t.Fatal("late lifecycle operation changed concurrent Close result")
	}
	if unsubscribed.Load() != 1 {
		t.Fatalf("original unsubscribe calls = %d, want 1", unsubscribed.Load())
	}
}

// TestLifecycleContentionRefusalPermanentlyFences drives the exact terminal
// producer called after the real bounded CAS loop. It proves that producer's
// outcome, not that this test scheduled 64 failed CompareAndSwap operations.
func TestLifecycleContentionRefusalPermanentlyFences(t *testing.T) {
	watch, signals := lifecycleTestWatch(t)
	initial := watch.Snapshot()
	if !watch.Admit(initial) {
		t.Fatal("original preparation was not admitted")
	}
	permit, admitted := watch.Admission()
	if !admitted {
		t.Fatal("original admitted permit absent")
	}
	<-watch.Changed() // Consume constructor wake before the measured refusal.
	watch.refuseContention()
	refused := watch.Snapshot()
	if !refused.Closed() {
		t.Fatal("contention refusal did not close the watch")
	}
	if !errors.Is(refused.Failure(), errLifecycleContention) {
		t.Fatalf("contention refusal lost its original cause: %v", refused.Failure())
	}
	if refused.LossSequence() != math.MaxUint64 {
		t.Fatal("contention refusal lost terminal loss identity")
	}
	if refused.RetryRequested() {
		t.Fatal("contention refusal retained a preparation hint")
	}
	if watch.Current(permit) {
		t.Fatal("contention refusal retained original admitted permit")
	}
	if _, admitted := watch.Admission(); admitted {
		t.Fatal("contention refusal retained discovery admission")
	}
	select {
	case <-watch.Changed():
	default:
		t.Fatal("contention refusal did not wake the owner")
	}
	signals.Ready()
	signals.Lost()
	if watch.Admit(initial) {
		t.Fatal("old preparation reopened contention refusal")
	}
	if watch.Admit(refused) {
		t.Fatal("refused preparation reopened contention refusal")
	}
	if watch.Snapshot() != refused {
		t.Fatal("late callback changed permanent contention refusal")
	}
}

// lifecycleTestWatch uses the real registry and subscription constructor;
// the provider retains only its original opaque notification capability.
func lifecycleTestWatch(t *testing.T) (*LifecycleWatch, LifecycleSignals) {
	t.Helper()
	var signals LifecycleSignals
	watch := lifecycleTestSubscribe(t, func(_ ze.EventBus, got LifecycleSignals) (func(), error) {
		signals = got
		return func() {}, nil
	})
	return watch, signals
}

func lifecycleTestSubscribe(t *testing.T, subscribe SubscribeLifecycleFunc) *LifecycleWatch {
	t.Helper()
	var registry sessionRegistry
	if err := registry.register("selected", testUnusedOpen, testUnusedPrepare, subscribe); err != nil {
		t.Fatal(err)
	}
	backend, err := registry.lookup("selected")
	if err != nil {
		t.Fatal(err)
	}
	watch, supported, err := backend.WatchLifecycle(&lifecycleTestBus{})
	if err != nil {
		t.Fatal(err)
	}
	if !supported {
		t.Fatal("registered capability absent")
	}
	t.Cleanup(watch.Close)
	return watch
}

// lifecycleTestBus deliberately copies handlers before dispatch, matching the
// production bus contract without claiming native VPP or process authority.
type lifecycleTestBus struct {
	mu       sync.Mutex
	handlers map[[2]string][]func(any)
}

func (b *lifecycleTestBus) Subscribe(namespace, event string, handler func(any)) func() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.handlers == nil {
		b.handlers = make(map[[2]string][]func(any))
	}
	key := [2]string{namespace, event}
	index := len(b.handlers[key])
	b.handlers[key] = append(b.handlers[key], handler)
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.handlers[key][index] = nil
	}
}

func (b *lifecycleTestBus) Emit(namespace, event string, payload any) (int, error) {
	b.mu.Lock()
	handlers := append([]func(any){}, b.handlers[[2]string{namespace, event}]...)
	b.mu.Unlock()
	for _, handler := range handlers {
		if handler != nil {
			handler(payload)
		}
	}
	return 0, nil
}

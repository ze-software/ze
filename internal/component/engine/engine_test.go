package engine_test

import (
	"context"
	"slices"
	"sync"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/engine"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// recorder tracks the order of subsystem lifecycle calls.
type recorder struct {
	mu     sync.Mutex
	events []string
}

func (r *recorder) record(event string) {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
}

func (r *recorder) get() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]string, len(r.events))
	copy(result, r.events)
	return result
}

// stubSubsystem implements ze.Subsystem for testing.
type stubSubsystem struct {
	name     string
	recorder *recorder
}

func newStubSubsystem(name string, rec *recorder) *stubSubsystem {
	return &stubSubsystem{name: name, recorder: rec}
}

func (s *stubSubsystem) Name() string { return s.name }

func (s *stubSubsystem) Start(_ context.Context, _ ze.Bus, _ ze.ConfigProvider) error {
	s.recorder.record("start:" + s.name)
	return nil
}

func (s *stubSubsystem) Stop(_ context.Context) error {
	s.recorder.record("stop:" + s.name)
	return nil
}

func (s *stubSubsystem) Reload(_ context.Context, _ ze.ConfigProvider) error {
	s.recorder.record("reload:" + s.name)
	return nil
}

// VALIDATES: AC-1 — Subsystem registration and accessibility after Start.
// PREVENTS: Lost subsystem registrations.
func TestRegisterSubsystem(t *testing.T) {
	rec := &recorder{}
	eng := engine.NewEngine(stubBus(), stubConfig(), stubPlugins())

	sub := newStubSubsystem("bgp", rec)
	if err := eng.RegisterSubsystem(sub); err != nil {
		t.Fatalf("RegisterSubsystem: %v", err)
	}

	ctx := context.Background()
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = eng.Stop(ctx) }()

	events := rec.get()
	if len(events) == 0 {
		t.Fatal("no events recorded — subsystem not started")
	}
	if events[0] != "start:bgp" {
		t.Errorf("first event = %q, want %q", events[0], "start:bgp")
	}
}

// VALIDATES: AC-2 — Duplicate subsystem name returns error.
// PREVENTS: Silent name collision.
func TestRegisterSubsystemDuplicate(t *testing.T) {
	rec := &recorder{}
	eng := engine.NewEngine(stubBus(), stubConfig(), stubPlugins())

	if err := eng.RegisterSubsystem(newStubSubsystem("bgp", rec)); err != nil {
		t.Fatalf("first Register: %v", err)
	}

	err := eng.RegisterSubsystem(newStubSubsystem("bgp", rec))
	if err == nil {
		t.Fatal("expected error for duplicate subsystem, got nil")
		return
	}
}

// VALIDATES: AC-10 — RegisterSubsystem after Start returns error.
// PREVENTS: Late registration after lifecycle started.
func TestRegisterAfterStart(t *testing.T) {
	rec := &recorder{}
	eng := engine.NewEngine(stubBus(), stubConfig(), stubPlugins())

	ctx := context.Background()
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = eng.Stop(ctx) }()

	err := eng.RegisterSubsystem(newStubSubsystem("bgp", rec))
	if err == nil {
		t.Fatal("expected error for register after start, got nil")
		return
	}
}

// VALIDATES: AC-3 — Start launches subsystems with bus and config.
// PREVENTS: Subsystems not receiving dependencies.
func TestStart(t *testing.T) {
	rec := &recorder{}
	eng := engine.NewEngine(stubBus(), stubConfig(), stubPlugins())

	if err := eng.RegisterSubsystem(newStubSubsystem("bgp", rec)); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := eng.RegisterSubsystem(newStubSubsystem("bmp", rec)); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ctx := context.Background()
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = eng.Stop(ctx) }()

	events := rec.get()
	startCount := 0
	for _, e := range events {
		if len(e) > 6 && e[:6] == "start:" {
			startCount++
		}
	}
	if startCount != 2 {
		t.Errorf("got %d start events, want 2; events: %v", startCount, events)
	}
}

// VALIDATES: AC-3 — Subsystems started in registration order.
// PREVENTS: Wrong startup ordering.
func TestStartOrder(t *testing.T) {
	rec := &recorder{}
	eng := engine.NewEngine(stubBus(), stubConfig(), stubPlugins())

	names := []string{"alpha", "bravo", "charlie"}
	for _, name := range names {
		if err := eng.RegisterSubsystem(newStubSubsystem(name, rec)); err != nil {
			t.Fatalf("Register(%s): %v", name, err)
		}
	}

	ctx := context.Background()
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = eng.Stop(ctx) }()

	events := rec.get()
	expected := []string{"start:alpha", "start:bravo", "start:charlie"}
	if len(events) < len(expected) {
		t.Fatalf("got %d events, want at least %d; events: %v", len(events), len(expected), events)
	}
	for i, exp := range expected {
		if events[i] != exp {
			t.Errorf("events[%d] = %q, want %q", i, events[i], exp)
		}
	}
}

// VALIDATES: AC-4 — Subsystems stopped in reverse registration order.
// PREVENTS: Wrong shutdown ordering.
func TestStopOrder(t *testing.T) {
	rec := &recorder{}
	eng := engine.NewEngine(stubBus(), stubConfig(), stubPlugins())

	names := []string{"alpha", "bravo", "charlie"}
	for _, name := range names {
		if err := eng.RegisterSubsystem(newStubSubsystem(name, rec)); err != nil {
			t.Fatalf("Register(%s): %v", name, err)
		}
	}

	ctx := context.Background()
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := eng.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	events := rec.get()
	// Find stop events.
	var stops []string
	for _, e := range events {
		if len(e) > 5 && e[:5] == "stop:" {
			stops = append(stops, e)
		}
	}

	expected := []string{"stop:charlie", "stop:bravo", "stop:alpha"}
	if len(stops) != len(expected) {
		t.Fatalf("got %d stop events, want %d; stops: %v", len(stops), len(expected), stops)
	}
	for i, exp := range expected {
		if stops[i] != exp {
			t.Errorf("stops[%d] = %q, want %q", i, stops[i], exp)
		}
	}
}

// VALIDATES: AC-5 — Reload calls Reload on all subsystems.
// PREVENTS: Lost reload notifications.
func TestReload(t *testing.T) {
	rec := &recorder{}
	eng := engine.NewEngine(stubBus(), stubConfig(), stubPlugins())

	if err := eng.RegisterSubsystem(newStubSubsystem("bgp", rec)); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ctx := context.Background()
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = eng.Stop(ctx) }()

	if err := eng.Reload(ctx); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	events := rec.get()
	if !slices.Contains(events, "reload:bgp") {
		t.Errorf("Reload not called on bgp subsystem; events: %v", events)
	}
}

// VALIDATES: AC-6 — Bus() returns non-nil after Start.
// PREVENTS: Nil bus reference.
func TestBusAccessor(t *testing.T) {
	eng := engine.NewEngine(stubBus(), stubConfig(), stubPlugins())

	if eng.Bus() == nil {
		t.Fatal("Bus() returned nil")
		return
	}
}

// VALIDATES: AC-7 — Config() returns non-nil.
// PREVENTS: Nil config reference.
func TestConfigAccessor(t *testing.T) {
	eng := engine.NewEngine(stubBus(), stubConfig(), stubPlugins())

	if eng.Config() == nil {
		t.Fatal("Config() returned nil")
		return
	}
}

// VALIDATES: AC-8 — Plugins() returns non-nil.
// PREVENTS: Nil plugin manager reference.
func TestPluginsAccessor(t *testing.T) {
	eng := engine.NewEngine(stubBus(), stubConfig(), stubPlugins())

	if eng.Plugins() == nil {
		t.Fatal("Plugins() returned nil")
		return
	}
}

// VALIDATES: AC-11 — Start with no subsystems succeeds.
// PREVENTS: Error on empty engine.
func TestStartEmpty(t *testing.T) {
	eng := engine.NewEngine(stubBus(), stubConfig(), stubPlugins())

	ctx := context.Background()
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Start with no subsystems: %v", err)
	}
	if err := eng.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

// VALIDATES: AC-12 — Second Stop returns nil (idempotent).
// PREVENTS: Error on double stop.
func TestStopIdempotent(t *testing.T) {
	eng := engine.NewEngine(stubBus(), stubConfig(), stubPlugins())

	ctx := context.Background()
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := eng.Stop(ctx); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	if err := eng.Stop(ctx); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}

// VALIDATES: Full lifecycle: register → start → reload → stop.
// PREVENTS: State machine bugs.
func TestLifecycle(t *testing.T) {
	rec := &recorder{}
	eng := engine.NewEngine(stubBus(), stubConfig(), stubPlugins())

	if err := eng.RegisterSubsystem(newStubSubsystem("bgp", rec)); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ctx := context.Background()

	// Start.
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Reload.
	if err := eng.Reload(ctx); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	// Stop.
	if err := eng.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	events := rec.get()
	expected := []string{"start:bgp", "reload:bgp", "stop:bgp"}
	if len(events) != len(expected) {
		t.Fatalf("got %d events, want %d; events: %v", len(events), len(expected), events)
	}
	for i, exp := range expected {
		if events[i] != exp {
			t.Errorf("events[%d] = %q, want %q", i, events[i], exp)
		}
	}
}

// VALIDATES: Engine interface satisfaction.
// PREVENTS: Compile-time interface drift.
func TestEngineSatisfiesInterface(t *testing.T) {
	var _ ze.Engine = (*engine.Engine)(nil)
}

// --- Stub implementations for Engine dependencies ---

type testBus struct{}

func (b *testBus) CreateTopic(string) (ze.Topic, error)      { return ze.Topic{}, nil }
func (b *testBus) Publish(string, []byte, map[string]string) {}
func (b *testBus) Subscribe(string, map[string]string, ze.Consumer) (ze.Subscription, error) {
	return ze.Subscription{}, nil
}
func (b *testBus) Unsubscribe(ze.Subscription) {}

func stubBus() ze.Bus { return &testBus{} }

type testConfig struct{}

func (c *testConfig) Load(string) error                   { return nil }
func (c *testConfig) Get(string) (map[string]any, error)  { return map[string]any{}, nil }
func (c *testConfig) Validate() []error                   { return nil }
func (c *testConfig) Save(string) error                   { return nil }
func (c *testConfig) Watch(string) <-chan ze.ConfigChange { return nil }
func (c *testConfig) Schema() ze.SchemaTree               { return ze.SchemaTree{} }
func (c *testConfig) RegisterSchema(string, string) error { return nil }

func stubConfig() ze.ConfigProvider { return &testConfig{} }

type testPlugins struct{}

func (p *testPlugins) Register(ze.PluginConfig) error                            { return nil }
func (p *testPlugins) StartAll(context.Context, ze.Bus, ze.ConfigProvider) error { return nil }
func (p *testPlugins) StopAll(context.Context) error                             { return nil }
func (p *testPlugins) Plugin(string) (ze.PluginProcess, bool)                    { return ze.PluginProcess{}, false }
func (p *testPlugins) Plugins() []ze.PluginProcess                               { return nil }
func (p *testPlugins) Capabilities() []ze.Capability                             { return nil }

func stubPlugins() ze.PluginManager { return &testPlugins{} }

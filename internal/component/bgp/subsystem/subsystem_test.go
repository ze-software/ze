package subsystem_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/subsystem"
	"codeberg.org/thomas-mangin/ze/internal/component/bus"
	"codeberg.org/thomas-mangin/ze/internal/component/engine"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// stubReactor implements subsystem.Reactor for testing.
type stubReactor struct {
	started atomic.Bool
	stopped atomic.Bool
	waited  atomic.Bool
}

func (r *stubReactor) StartWithContext(_ context.Context) error {
	r.started.Store(true)
	return nil
}

func (r *stubReactor) Stop() {
	r.stopped.Store(true)
}

func (r *stubReactor) Wait(_ context.Context) error {
	r.waited.Store(true)
	return nil
}

// VALIDATES: AC-1 — BGPSubsystem.Name() returns "bgp".
// PREVENTS: Wrong subsystem identifier.
func TestBGPSubsystemName(t *testing.T) {
	sub := subsystem.NewBGPSubsystem(&stubReactor{})
	if sub.Name() != "bgp" {
		t.Errorf("Name() = %q, want %q", sub.Name(), "bgp")
	}
}

// VALIDATES: AC-1 — BGPSubsystem.Start() calls reactor.StartWithContext().
// VALIDATES: AC-2 — BGPSubsystem.Stop() calls reactor.Stop() + Wait().
// PREVENTS: Adapter fails to delegate lifecycle to reactor.
func TestBGPSubsystemStartStop(t *testing.T) {
	r := &stubReactor{}
	sub := subsystem.NewBGPSubsystem(r)
	b := bus.NewBus()
	defer b.Stop()

	ctx := context.Background()
	if err := sub.Start(ctx, b, stubConfig()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !r.started.Load() {
		t.Fatal("reactor not started")
	}

	if err := sub.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !r.stopped.Load() {
		t.Fatal("reactor not stopped")
	}
	if !r.waited.Load() {
		t.Fatal("reactor.Wait not called")
	}
}

// VALIDATES: AC-3 — BGPSubsystem.Start() creates Bus topics.
// PREVENTS: Missing Bus topics for cross-component consumers.
func TestBGPSubsystemCreatesBusTopics(t *testing.T) {
	r := &stubReactor{}
	sub := subsystem.NewBGPSubsystem(r)
	b := bus.NewBus()
	defer b.Stop()

	ctx := context.Background()
	if err := sub.Start(ctx, b, stubConfig()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = sub.Stop(ctx) }()

	// Try to create the same topics — should fail because they already exist.
	expectedTopics := []string{
		"bgp/update",
		"bgp/state",
		"bgp/negotiated",
		"bgp/eor",
		"bgp/congestion",
	}
	for _, topic := range expectedTopics {
		if _, err := b.CreateTopic(topic); err == nil {
			t.Errorf("topic %q was not created by Start()", topic)
		}
	}
}

// VALIDATES: AC-1 — Engine.Start() reaches BGPSubsystem.Start().
// PREVENTS: Engine not wired to subsystem.
func TestEngineStartsBGPSubsystem(t *testing.T) {
	r := &stubReactor{}
	sub := subsystem.NewBGPSubsystem(r)
	b := bus.NewBus()
	defer b.Stop()

	eng := engine.NewEngine(b, stubConfig(), stubPlugins())
	if err := eng.RegisterSubsystem(sub); err != nil {
		t.Fatalf("RegisterSubsystem: %v", err)
	}

	ctx := context.Background()
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = eng.Stop(ctx) }()

	if !r.started.Load() {
		t.Fatal("Engine.Start did not reach reactor.StartWithContext")
	}
}

// VALIDATES: AC-2 — Engine.Stop() reaches BGPSubsystem.Stop().
// PREVENTS: Engine not stopping subsystem.
func TestEngineStopsBGPSubsystem(t *testing.T) {
	r := &stubReactor{}
	sub := subsystem.NewBGPSubsystem(r)
	b := bus.NewBus()
	defer b.Stop()

	eng := engine.NewEngine(b, stubConfig(), stubPlugins())
	if err := eng.RegisterSubsystem(sub); err != nil {
		t.Fatalf("RegisterSubsystem: %v", err)
	}

	ctx := context.Background()
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := eng.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if !r.stopped.Load() {
		t.Fatal("Engine.Stop did not reach reactor.Stop")
	}
}

// VALIDATES: AC-9 — Bus subscriber receives notification published by reactor.
// PREVENTS: Bus notifications not reaching cross-component consumers.
func TestBusNotificationPublished(t *testing.T) {
	r := &stubReactor{}
	sub := subsystem.NewBGPSubsystem(r)
	b := bus.NewBus()
	defer b.Stop()

	// Subscribe before start.
	received := make(chan ze.Event, 10)
	consumer := &chanConsumer{ch: received}
	if _, err := b.Subscribe("bgp/", nil, consumer); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	ctx := context.Background()
	if err := sub.Start(ctx, b, stubConfig()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = sub.Stop(ctx) }()

	// Publish a notification through the subsystem's Bus reference.
	sub.PublishNotification("bgp/state", map[string]string{"peer": "10.0.0.1", "state": "up"})

	// Wait for delivery.
	select {
	case ev := <-received:
		if ev.Topic != "bgp/state" {
			t.Errorf("topic = %q, want %q", ev.Topic, "bgp/state")
		}
		if ev.Metadata["peer"] != "10.0.0.1" {
			t.Errorf("metadata[peer] = %q, want %q", ev.Metadata["peer"], "10.0.0.1")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for Bus notification")
	}
}

// VALIDATES: AC-9 — Bus notification has correct metadata.
// PREVENTS: Missing or wrong metadata on Bus events.
func TestBusNotificationMetadata(t *testing.T) {
	r := &stubReactor{}
	sub := subsystem.NewBGPSubsystem(r)
	b := bus.NewBus()
	defer b.Stop()

	// Subscribe with metadata filter.
	received := make(chan ze.Event, 10)
	consumer := &chanConsumer{ch: received}
	if _, err := b.Subscribe("bgp/", map[string]string{"direction": "received"}, consumer); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	ctx := context.Background()
	if err := sub.Start(ctx, b, stubConfig()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = sub.Stop(ctx) }()

	// Publish with matching metadata.
	sub.PublishNotification("bgp/update", map[string]string{"peer": "10.0.0.1", "direction": "received"})

	select {
	case ev := <-received:
		if ev.Metadata["direction"] != "received" {
			t.Errorf("metadata[direction] = %q, want %q", ev.Metadata["direction"], "received")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for filtered notification")
	}

	// Publish with non-matching metadata — should NOT be received.
	sub.PublishNotification("bgp/update", map[string]string{"peer": "10.0.0.2", "direction": "sent"})

	select {
	case ev := <-received:
		t.Fatalf("received unexpected event: %+v", ev)
	case <-time.After(100 * time.Millisecond):
		// Expected: no delivery for non-matching metadata.
	}
}

// VALIDATES: compile-time interface satisfaction.
// PREVENTS: BGPSubsystem drifts from ze.Subsystem.
func TestBGPSubsystemSatisfiesInterface(t *testing.T) {
	var _ ze.Subsystem = (*subsystem.BGPSubsystem)(nil)
}

// --- helpers ---

type chanConsumer struct {
	ch chan<- ze.Event
}

func (c *chanConsumer) Deliver(events []ze.Event) error {
	for _, ev := range events {
		c.ch <- ev
	}
	return nil
}

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

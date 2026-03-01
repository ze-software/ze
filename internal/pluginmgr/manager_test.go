package pluginmgr_test

import (
	"context"
	"sort"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/pluginmgr"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// stubBus implements ze.Bus for testing (PluginManager stores but doesn't use it yet).
type stubBus struct{}

func (s *stubBus) CreateTopic(string) (ze.Topic, error)      { return ze.Topic{}, nil }
func (s *stubBus) Publish(string, []byte, map[string]string) {}
func (s *stubBus) Subscribe(string, map[string]string, ze.Consumer) (ze.Subscription, error) {
	return ze.Subscription{}, nil
}
func (s *stubBus) Unsubscribe(ze.Subscription) {}

// stubConfig implements ze.ConfigProvider for testing.
type stubConfig struct{}

func (s *stubConfig) Load(string) error                   { return nil }
func (s *stubConfig) Get(string) (map[string]any, error)  { return map[string]any{}, nil }
func (s *stubConfig) Validate() []error                   { return nil }
func (s *stubConfig) Save(string) error                   { return nil }
func (s *stubConfig) Watch(string) <-chan ze.ConfigChange { return nil }
func (s *stubConfig) Schema() ze.SchemaTree               { return ze.SchemaTree{} }
func (s *stubConfig) RegisterSchema(string, string) error { return nil }

// VALIDATES: AC-1 — Plugin registration and lookup.
// PREVENTS: Lost registrations.
func TestRegister(t *testing.T) {
	mgr := pluginmgr.NewManager()

	err := mgr.Register(ze.PluginConfig{Name: "bgp-rib"})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Not started yet — Plugin() should return not running.
	pp, ok := mgr.Plugin("bgp-rib")
	if !ok {
		t.Fatal("Plugin('bgp-rib') returned false, want true")
	}
	if pp.Name != "bgp-rib" {
		t.Errorf("Name = %q, want %q", pp.Name, "bgp-rib")
	}
	if pp.Running {
		t.Error("Running = true before StartAll, want false")
	}
}

// VALIDATES: AC-2 — Duplicate registration returns error.
// PREVENTS: Silent name collision.
func TestRegisterDuplicate(t *testing.T) {
	mgr := pluginmgr.NewManager()

	if err := mgr.Register(ze.PluginConfig{Name: "bgp-rib"}); err != nil {
		t.Fatalf("first Register: %v", err)
	}

	err := mgr.Register(ze.PluginConfig{Name: "bgp-rib"})
	if err == nil {
		t.Fatal("expected error for duplicate, got nil")
	}
}

// VALIDATES: AC-10 — Register after StartAll returns error.
// PREVENTS: Late registration after lifecycle started.
func TestRegisterAfterStart(t *testing.T) {
	mgr := pluginmgr.NewManager()

	if err := mgr.Register(ze.PluginConfig{Name: "bgp-rib"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ctx := context.Background()
	if err := mgr.StartAll(ctx, &stubBus{}, &stubConfig{}); err != nil {
		t.Fatalf("StartAll: %v", err)
	}
	defer func() { _ = mgr.StopAll(ctx) }()

	err := mgr.Register(ze.PluginConfig{Name: "bgp-rs"})
	if err == nil {
		t.Fatal("expected error for register after start, got nil")
	}
}

// VALIDATES: AC-3 — StartAll marks all plugins running.
// PREVENTS: Plugins stuck in not-running state.
func TestStartAll(t *testing.T) {
	mgr := pluginmgr.NewManager()

	for _, name := range []string{"bgp-rib", "bgp-rs", "bgp-gr"} {
		if err := mgr.Register(ze.PluginConfig{Name: name}); err != nil {
			t.Fatalf("Register(%s): %v", name, err)
		}
	}

	ctx := context.Background()
	if err := mgr.StartAll(ctx, &stubBus{}, &stubConfig{}); err != nil {
		t.Fatalf("StartAll: %v", err)
	}
	defer func() { _ = mgr.StopAll(ctx) }()

	for _, name := range []string{"bgp-rib", "bgp-rs", "bgp-gr"} {
		pp, ok := mgr.Plugin(name)
		if !ok {
			t.Errorf("Plugin(%s) returned false", name)
			continue
		}
		if !pp.Running {
			t.Errorf("Plugin(%s).Running = false, want true", name)
		}
	}
}

// VALIDATES: AC-4 — StopAll marks all plugins stopped.
// PREVENTS: Plugins stuck in running state after stop.
func TestStopAll(t *testing.T) {
	mgr := pluginmgr.NewManager()

	if err := mgr.Register(ze.PluginConfig{Name: "bgp-rib"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ctx := context.Background()
	if err := mgr.StartAll(ctx, &stubBus{}, &stubConfig{}); err != nil {
		t.Fatalf("StartAll: %v", err)
	}

	if err := mgr.StopAll(ctx); err != nil {
		t.Fatalf("StopAll: %v", err)
	}

	pp, ok := mgr.Plugin("bgp-rib")
	if !ok {
		t.Fatal("Plugin('bgp-rib') returned false after stop")
	}
	if pp.Running {
		t.Error("Running = true after StopAll, want false")
	}
}

// VALIDATES: AC-5 and AC-6 — Plugin lookup for existing and non-existing.
// PREVENTS: Wrong lookup results.
func TestPluginLookup(t *testing.T) {
	mgr := pluginmgr.NewManager()

	if err := mgr.Register(ze.PluginConfig{Name: "bgp-rib"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Existing plugin.
	_, ok := mgr.Plugin("bgp-rib")
	if !ok {
		t.Error("Plugin('bgp-rib') = false, want true")
	}

	// Non-existing plugin.
	_, ok = mgr.Plugin("nonexistent")
	if ok {
		t.Error("Plugin('nonexistent') = true, want false")
	}
}

// VALIDATES: AC-7 — Plugins returns all registered plugins.
// PREVENTS: Missing plugins in listing.
func TestPlugins(t *testing.T) {
	mgr := pluginmgr.NewManager()

	names := []string{"bgp-gr", "bgp-rib", "bgp-rs"}
	for _, name := range names {
		if err := mgr.Register(ze.PluginConfig{Name: name}); err != nil {
			t.Fatalf("Register(%s): %v", name, err)
		}
	}

	ctx := context.Background()
	if err := mgr.StartAll(ctx, &stubBus{}, &stubConfig{}); err != nil {
		t.Fatalf("StartAll: %v", err)
	}
	defer func() { _ = mgr.StopAll(ctx) }()

	plugins := mgr.Plugins()
	if len(plugins) != 3 {
		t.Fatalf("got %d plugins, want 3", len(plugins))
	}

	// Sort by name for deterministic comparison.
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].Name < plugins[j].Name })
	for i, name := range names {
		if plugins[i].Name != name {
			t.Errorf("plugins[%d].Name = %q, want %q", i, plugins[i].Name, name)
		}
		if !plugins[i].Running {
			t.Errorf("plugins[%d].Running = false, want true", i)
		}
	}
}

// VALIDATES: AC-8 — Capabilities returns collected capabilities.
// PREVENTS: Lost capabilities.
func TestCapabilities(t *testing.T) {
	mgr := pluginmgr.NewManager()

	mgr.AddCapability(ze.Capability{Plugin: "bgp-gr", Code: 64, Value: []byte{0x01}})
	mgr.AddCapability(ze.Capability{Plugin: "bgp-rib", Code: 65, Value: []byte{0x00, 0x01}})

	caps := mgr.Capabilities()
	if len(caps) != 2 {
		t.Fatalf("got %d capabilities, want 2", len(caps))
	}

	// Sort by code for deterministic comparison.
	sort.Slice(caps, func(i, j int) bool { return caps[i].Code < caps[j].Code })
	if caps[0].Code != 64 || caps[0].Plugin != "bgp-gr" {
		t.Errorf("caps[0] = {Code:%d Plugin:%s}, want {Code:64 Plugin:bgp-gr}", caps[0].Code, caps[0].Plugin)
	}
	if caps[1].Code != 65 || caps[1].Plugin != "bgp-rib" {
		t.Errorf("caps[1] = {Code:%d Plugin:%s}, want {Code:65 Plugin:bgp-rib}", caps[1].Code, caps[1].Plugin)
	}
}

// VALIDATES: Full lifecycle: register → start → query → stop.
// PREVENTS: State machine bugs.
func TestLifecycle(t *testing.T) {
	mgr := pluginmgr.NewManager()

	// Register.
	if err := mgr.Register(ze.PluginConfig{Name: "bgp-rib", Dependencies: []string{"bgp-rs"}}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Before start: registered but not running.
	pp, ok := mgr.Plugin("bgp-rib")
	if !ok || pp.Running {
		t.Fatalf("before start: ok=%v running=%v", ok, pp.Running)
	}

	// Start.
	ctx := context.Background()
	if err := mgr.StartAll(ctx, &stubBus{}, &stubConfig{}); err != nil {
		t.Fatalf("StartAll: %v", err)
	}

	// After start: running.
	pp, ok = mgr.Plugin("bgp-rib")
	if !ok || !pp.Running {
		t.Fatalf("after start: ok=%v running=%v", ok, pp.Running)
	}

	// Stop.
	if err := mgr.StopAll(ctx); err != nil {
		t.Fatalf("StopAll: %v", err)
	}

	// After stop: not running.
	pp, ok = mgr.Plugin("bgp-rib")
	if !ok || pp.Running {
		t.Fatalf("after stop: ok=%v running=%v", ok, pp.Running)
	}
}

// VALIDATES: PluginManager interface satisfaction.
// PREVENTS: Compile-time interface drift.
func TestManagerSatisfiesInterface(t *testing.T) {
	var _ ze.PluginManager = (*pluginmgr.Manager)(nil)
}

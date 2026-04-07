package ze_test

import (
	"context"
	"testing"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// mockEventBus is a minimal EventBus implementation for interface
// satisfaction tests.
type mockEventBus struct{}

func (m *mockEventBus) Emit(namespace, eventType, payload string) (int, error) { return 0, nil }
func (m *mockEventBus) Subscribe(_, _ string, _ func(string)) func() {
	return func() {}
}

// mockConfigProvider is a minimal ConfigProvider implementation.
type mockConfigProvider struct{}

func (m *mockConfigProvider) Load(path string) error                   { return nil }
func (m *mockConfigProvider) Get(_ string) (map[string]any, error)     { return map[string]any{}, nil }
func (m *mockConfigProvider) Validate() []error                        { return nil }
func (m *mockConfigProvider) Save(path string) error                   { return nil }
func (m *mockConfigProvider) Watch(root string) <-chan ze.ConfigChange { return nil }
func (m *mockConfigProvider) Schema() ze.SchemaTree                    { return ze.SchemaTree{} }
func (m *mockConfigProvider) RegisterSchema(name, yang string) error   { return nil }

// mockPluginManager is a minimal PluginManager implementation.
type mockPluginManager struct{}

func (m *mockPluginManager) Register(config ze.PluginConfig) error { return nil }
func (m *mockPluginManager) StartAll(ctx context.Context, eventBus ze.EventBus, config ze.ConfigProvider) error {
	return nil
}
func (m *mockPluginManager) StopAll(ctx context.Context) error { return nil }
func (m *mockPluginManager) Plugin(name string) (ze.PluginProcess, bool) {
	return ze.PluginProcess{}, false
}
func (m *mockPluginManager) Plugins() []ze.PluginProcess   { return nil }
func (m *mockPluginManager) Capabilities() []ze.Capability { return nil }

// mockSubsystem is a minimal Subsystem implementation.
type mockSubsystem struct{}

func (m *mockSubsystem) Name() string { return "mock" }
func (m *mockSubsystem) Start(ctx context.Context, eventBus ze.EventBus, config ze.ConfigProvider) error {
	return nil
}
func (m *mockSubsystem) Stop(ctx context.Context) error                             { return nil }
func (m *mockSubsystem) Reload(ctx context.Context, config ze.ConfigProvider) error { return nil }

// mockEngine is a minimal Engine implementation.
type mockEngine struct{}

func (m *mockEngine) RegisterSubsystem(sub ze.Subsystem) error { return nil }
func (m *mockEngine) Start(ctx context.Context) error          { return nil }
func (m *mockEngine) Stop(ctx context.Context) error           { return nil }
func (m *mockEngine) Reload(ctx context.Context) error         { return nil }
func (m *mockEngine) EventBus() ze.EventBus                    { return &mockEventBus{} }
func (m *mockEngine) Config() ze.ConfigProvider                { return &mockConfigProvider{} }
func (m *mockEngine) Plugins() ze.PluginManager                { return &mockPluginManager{} }

func TestMockEventBusSatisfiesInterface(t *testing.T) {
	var _ ze.EventBus = (*mockEventBus)(nil)
}

func TestMockConfigProviderSatisfiesInterface(t *testing.T) {
	var _ ze.ConfigProvider = (*mockConfigProvider)(nil)
}

func TestMockPluginManagerSatisfiesInterface(t *testing.T) {
	var _ ze.PluginManager = (*mockPluginManager)(nil)
}

func TestMockSubsystemSatisfiesInterface(t *testing.T) {
	var _ ze.Subsystem = (*mockSubsystem)(nil)
}

func TestMockEngineSatisfiesInterface(t *testing.T) {
	var _ ze.Engine = (*mockEngine)(nil)
}

func TestInterfacesCompile(t *testing.T) {
	// Verify all types are usable
	_ = ze.ConfigChange{}
	_ = ze.SchemaTree{}
	_ = ze.PluginConfig{}
	_ = ze.PluginProcess{}
	_ = ze.Capability{}
}

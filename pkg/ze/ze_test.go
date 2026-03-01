package ze_test

import (
	"context"
	"testing"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// mockBus is a minimal Bus implementation for interface satisfaction tests.
type mockBus struct{}

func (m *mockBus) CreateTopic(name string) (ze.Topic, error)                        { return ze.Topic{}, nil }
func (m *mockBus) Publish(topic string, payload []byte, metadata map[string]string) {}
func (m *mockBus) Subscribe(prefix string, filter map[string]string, consumer ze.Consumer) (ze.Subscription, error) {
	return ze.Subscription{}, nil
}
func (m *mockBus) Unsubscribe(sub ze.Subscription) {}

// mockConsumer is a minimal Consumer implementation.
type mockConsumer struct{}

func (m *mockConsumer) Deliver(events []ze.Event) error { return nil }

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
func (m *mockPluginManager) StartAll(ctx context.Context, bus ze.Bus, config ze.ConfigProvider) error {
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
func (m *mockSubsystem) Start(ctx context.Context, bus ze.Bus, config ze.ConfigProvider) error {
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
func (m *mockEngine) Bus() ze.Bus                              { return &mockBus{} }
func (m *mockEngine) Config() ze.ConfigProvider                { return &mockConfigProvider{} }
func (m *mockEngine) Plugins() ze.PluginManager                { return &mockPluginManager{} }

func TestMockBusSatisfiesInterface(t *testing.T) {
	var _ ze.Bus = (*mockBus)(nil)
}

func TestMockConsumerSatisfiesInterface(t *testing.T) {
	var _ ze.Consumer = (*mockConsumer)(nil)
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
	_ = ze.Event{}
	_ = ze.Topic{}
	_ = ze.Subscription{}
	_ = ze.ConfigChange{}
	_ = ze.SchemaTree{}
	_ = ze.PluginConfig{}
	_ = ze.PluginProcess{}
	_ = ze.Capability{}
}

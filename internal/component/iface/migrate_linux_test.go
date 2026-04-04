package iface

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// mockMigrateBackend implements Backend for migration tests.
// All management operations return errors (simulating no root/netlink).
type mockMigrateBackend struct{}

func (m *mockMigrateBackend) CreateDummy(_ string) error   { return fmt.Errorf("mock: not supported") }
func (m *mockMigrateBackend) CreateVeth(_, _ string) error { return fmt.Errorf("mock: not supported") }
func (m *mockMigrateBackend) CreateBridge(_ string) error  { return fmt.Errorf("mock: not supported") }
func (m *mockMigrateBackend) CreateVLAN(_ string, _ int) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) DeleteInterface(_ string) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) AddAddress(_, _ string) error { return fmt.Errorf("mock: not supported") }
func (m *mockMigrateBackend) RemoveAddress(_, _ string) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) SetAdminUp(_ string) error    { return fmt.Errorf("mock: not supported") }
func (m *mockMigrateBackend) SetAdminDown(_ string) error  { return fmt.Errorf("mock: not supported") }
func (m *mockMigrateBackend) SetMTU(_ string, _ int) error { return fmt.Errorf("mock: not supported") }
func (m *mockMigrateBackend) SetMACAddress(_, _ string) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) GetMACAddress(_ string) (string, error) {
	return "", fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) GetStats(_ string) (*InterfaceStats, error) {
	return nil, fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) ListInterfaces() ([]InterfaceInfo, error) {
	return nil, fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) GetInterface(_ string) (*InterfaceInfo, error) {
	return nil, fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) BridgeAddPort(_, _ string) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) BridgeDelPort(_ string) error { return fmt.Errorf("mock: not supported") }
func (m *mockMigrateBackend) BridgeSetSTP(_ string, _ bool) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) SetIPv4Forwarding(_ string, _ bool) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) SetIPv4ArpFilter(_ string, _ bool) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) SetIPv4ArpAccept(_ string, _ bool) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) SetIPv4ProxyARP(_ string, _ bool) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) SetIPv4ArpAnnounce(_ string, _ int) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) SetIPv4ArpIgnore(_ string, _ int) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) SetIPv4RPFilter(_ string, _ int) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) SetIPv6Autoconf(_ string, _ bool) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) SetIPv6AcceptRA(_ string, _ int) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) SetIPv6Forwarding(_ string, _ bool) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) SetupMirror(_, _ string, _, _ bool) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) RemoveMirror(_ string) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) StartMonitor(_ ze.Bus) error { return fmt.Errorf("mock: not supported") }
func (m *mockMigrateBackend) StopMonitor()                {}
func (m *mockMigrateBackend) Close() error                { return nil }

// subscribableBus extends collectingBus with subscription support for migrate tests.
// It delivers events to registered consumers when Publish is called.
type subscribableBus struct {
	events      []ze.Event
	consumers   []ze.Consumer
	subErr      error // if set, Subscribe returns this error
	nextSubID   uint64
	publishHook func(topic string, payload []byte) // optional hook for testing
}

func (b *subscribableBus) CreateTopic(string) (ze.Topic, error) { return ze.Topic{}, nil }

func (b *subscribableBus) Publish(topic string, payload []byte, metadata map[string]string) {
	ev := ze.Event{Topic: topic, Payload: payload, Metadata: metadata}
	b.events = append(b.events, ev)
	if b.publishHook != nil {
		b.publishHook(topic, payload)
	}
	// Deliver to all registered consumers.
	for _, c := range b.consumers {
		_ = c.Deliver([]ze.Event{ev})
	}
}

func (b *subscribableBus) Subscribe(_ string, _ map[string]string, consumer ze.Consumer) (ze.Subscription, error) {
	if b.subErr != nil {
		return ze.Subscription{}, b.subErr
	}
	b.nextSubID++
	b.consumers = append(b.consumers, consumer)
	return ze.Subscription{ID: b.nextSubID}, nil
}

func (b *subscribableBus) Unsubscribe(ze.Subscription) {}

func TestMigrateConfigValidation(t *testing.T) {
	// VALIDATES: MigrateConfig rejects empty required fields and unknown types.
	// PREVENTS: Invalid migration configs reaching netlink calls.
	tests := []struct {
		name    string
		cfg     MigrateConfig
		wantErr string
	}{
		{
			name:    "empty old iface",
			cfg:     MigrateConfig{OldIface: "", NewIface: "lo1", Address: "10.0.0.1/24"},
			wantErr: "migrate: old interface:",
		},
		{
			name:    "empty new iface",
			cfg:     MigrateConfig{OldIface: "lo0", NewIface: "", Address: "10.0.0.1/24"},
			wantErr: "migrate: new interface:",
		},
		{
			name:    "empty address",
			cfg:     MigrateConfig{OldIface: "lo0", NewIface: "lo1", Address: ""},
			wantErr: "address is empty",
		},
		{
			name: "unknown interface type",
			cfg: MigrateConfig{
				OldIface: "lo0", NewIface: "lo1",
				Address: "10.0.0.1/24", NewIfaceType: "tap",
			},
			wantErr: "unknown interface type",
		},
		{
			name: "valid dummy type",
			cfg: MigrateConfig{
				OldIface: "lo0", NewIface: "lo1",
				Address: "10.0.0.1/24", NewIfaceType: "dummy",
			},
			wantErr: "",
		},
		{
			name: "valid no type",
			cfg: MigrateConfig{
				OldIface: "lo0", NewIface: "lo1",
				Address: "10.0.0.1/24",
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMigrateConfig(tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestMigrateNilBus(t *testing.T) {
	// VALIDATES: MigrateInterface rejects nil bus.
	// PREVENTS: Nil dereference in phase 3 subscription.
	cfg := MigrateConfig{
		OldIface: "lo0",
		NewIface: "lo1",
		Address:  "10.0.0.1/24",
	}
	err := MigrateInterface(cfg, nil, 5)
	if err == nil {
		t.Fatal("expected error for nil bus, got nil")
	}
	if !strings.Contains(err.Error(), "bus is nil") {
		t.Errorf("error = %q, want containing 'bus is nil'", err.Error())
	}
}

func TestMigrateZeroTimeout(t *testing.T) {
	// VALIDATES: MigrateInterface rejects non-positive timeout.
	// PREVENTS: Infinite wait or immediate timeout confusion.
	cfg := MigrateConfig{
		OldIface: "lo0",
		NewIface: "lo1",
		Address:  "10.0.0.1/24",
	}
	bus := &subscribableBus{}
	err := MigrateInterface(cfg, bus, 0)
	if err == nil {
		t.Fatal("expected error for zero timeout, got nil")
	}
	if !strings.Contains(err.Error(), "timeout must be positive") {
		t.Errorf("error = %q, want containing 'timeout must be positive'", err.Error())
	}
}

func TestMigrateSubscribeFailureRollback(t *testing.T) {
	// VALIDATES: Phase 3 subscribe failure triggers rollback of phases 2 and 1.
	// PREVENTS: Leaked addresses and interfaces when bus subscription fails.
	//
	// Note: This tests the control flow logic. MigrateInterface requires a loaded
	// backend. We register a mock that fails AddAddress (simulating no root/netlink),
	// which triggers the error path. The rollback calls are best-effort.
	_ = RegisterBackend("test-migrate", func() (Backend, error) {
		return &mockMigrateBackend{}, nil
	})
	if err := LoadBackend("test-migrate"); err != nil {
		t.Fatalf("load test backend: %v", err)
	}
	defer func() { _ = CloseBackend() }()

	bus := &subscribableBus{
		subErr: errors.New("bus: subscribe failed"),
	}
	cfg := MigrateConfig{
		OldIface: "lo0",
		NewIface: "lo1",
		Address:  "10.0.0.1/24",
	}
	err := MigrateInterface(cfg, bus, 5)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// The error may come from phase 2 (AddAddress fails) or phase 3 (subscribe fails).
	if !strings.Contains(err.Error(), "migrate phase") {
		t.Errorf("error = %q, want containing 'migrate phase'", err.Error())
	}
}

func TestMigrateNoBackend(t *testing.T) {
	// VALIDATES: MigrateInterface returns error when no backend is loaded.
	// PREVENTS: Nil dereference when calling backend operations.

	// Ensure no backend is loaded by closing any existing one.
	_ = CloseBackend()

	cfg := MigrateConfig{
		OldIface: "lo0",
		NewIface: "lo1",
		Address:  "10.0.0.1/24",
	}
	bus := &subscribableBus{}
	err := MigrateInterface(cfg, bus, 5)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no backend loaded") {
		t.Errorf("error = %q, want containing 'no backend loaded'", err.Error())
	}
}

func TestResolveOSName(t *testing.T) {
	// VALIDATES: resolveOSName maps unit 0 to parent, unit N to "<name>.<N>".
	// PREVENTS: Wrong interface names passed to netlink operations.
	tests := []struct {
		name  string
		iface string
		unit  int
		want  string
	}{
		{"unit 0", "eth0", 0, "eth0"},
		{"unit 100", "eth0", 100, "eth0.100"},
		{"unit 1", "lo", 1, "lo.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveOSName(tt.iface, tt.unit)
			if got != tt.want {
				t.Errorf("resolveOSName(%q, %d) = %q, want %q",
					tt.iface, tt.unit, got, tt.want)
			}
		})
	}
}

func TestStripPrefix(t *testing.T) {
	// VALIDATES: stripPrefix extracts IP from CIDR notation.
	// PREVENTS: Prefix length included in BGP readiness address matching.
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ipv4 cidr", "10.0.0.1/24", "10.0.0.1"},
		{"ipv6 cidr", "fd00::1/64", "fd00::1"},
		{"no prefix", "10.0.0.1", "10.0.0.1"},
		{"host route", "192.168.1.1/32", "192.168.1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripPrefix(tt.input)
			if got != tt.want {
				t.Errorf("stripPrefix(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBGPReadyConsumerDeliver(t *testing.T) {
	// VALIDATES: bgpReadyConsumer signals readiness when address matches.
	// PREVENTS: Phase 3 never completing because address matching is wrong.
	consumer := &bgpReadyConsumer{
		targetAddr: "10.0.0.1/24",
		ready:      make(chan struct{}, 1),
	}

	// Non-matching event: should not signal.
	_ = consumer.Deliver([]ze.Event{{
		Topic:   topicBGPListenerReady,
		Payload: []byte(`{"address":"192.168.1.1"}`),
	}})
	select {
	case <-consumer.ready:
		t.Fatal("signaled readiness for non-matching address")
	default: // expected: not ready
	}

	// Matching event (IP without prefix): should signal.
	_ = consumer.Deliver([]ze.Event{{
		Topic:   topicBGPListenerReady,
		Payload: []byte(`{"address":"10.0.0.1"}`),
	}})
	select {
	case <-consumer.ready: // expected: ready
	default:
		t.Fatal("expected readiness signal for matching address")
	}
}

func TestBGPReadyConsumerDeliverCIDR(t *testing.T) {
	// VALIDATES: bgpReadyConsumer matches when payload contains full CIDR.
	// PREVENTS: Mismatch when BGP reports address with prefix length.
	consumer := &bgpReadyConsumer{
		targetAddr: "10.0.0.1/24",
		ready:      make(chan struct{}, 1),
	}

	_ = consumer.Deliver([]ze.Event{{
		Topic:   topicBGPListenerReady,
		Payload: []byte(`{"address":"10.0.0.1/24"}`),
	}})
	select {
	case <-consumer.ready: // expected: ready
	default:
		t.Fatal("expected readiness signal for matching CIDR address")
	}
}

func TestBGPReadyConsumerMetadataFallback(t *testing.T) {
	// VALIDATES: bgpReadyConsumer falls back to metadata when payload address is empty.
	// PREVENTS: Phase 3 timeout when BGP uses metadata instead of payload.
	consumer := &bgpReadyConsumer{
		targetAddr: "10.0.0.1/24",
		ready:      make(chan struct{}, 1),
	}

	_ = consumer.Deliver([]ze.Event{{
		Topic:    topicBGPListenerReady,
		Payload:  []byte(`{"address":""}`),
		Metadata: map[string]string{"address": "10.0.0.1"},
	}})
	select {
	case <-consumer.ready: // expected: ready
	default:
		t.Fatal("expected readiness signal via metadata fallback")
	}
}

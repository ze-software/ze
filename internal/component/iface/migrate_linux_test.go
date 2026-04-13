package iface

import (
	"fmt"
	"strings"
	"sync"
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
func (m *mockMigrateBackend) CreateTunnel(_ TunnelSpec) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) CreateWireguardDevice(_ string) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) ConfigureWireguardDevice(_ WireguardSpec) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) GetWireguardDevice(_ string) (WireguardSpec, error) {
	return WireguardSpec{}, fmt.Errorf("mock: not supported")
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
func (m *mockMigrateBackend) SetupMirror(_, _ string, _, _ bool) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) RemoveMirror(_ string) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) ReplaceAddressWithLifetime(_, _ string, _, _ int) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) AddRoute(_, _, _ string, _ int) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) RemoveRoute(_, _, _ string, _ int) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) StartMonitor(_ ze.EventBus) error {
	return fmt.Errorf("mock: not supported")
}
func (m *mockMigrateBackend) StopMonitor() {}
func (m *mockMigrateBackend) Close() error { return nil }

// stubEventBus is a minimal ze.EventBus used by migrate tests.
// Subscribe records handlers; tests drive delivery explicitly via Emit.
type stubEventBus struct {
	mu       sync.Mutex
	handlers map[string][]func(payload string)
	subErr   error
}

func newStubEventBus() *stubEventBus {
	return &stubEventBus{handlers: make(map[string][]func(string))}
}

func (b *stubEventBus) key(namespace, eventType string) string {
	return namespace + ":" + eventType
}

func (b *stubEventBus) Emit(namespace, eventType, payload string) (int, error) {
	b.mu.Lock()
	handlers := append([]func(string){}, b.handlers[b.key(namespace, eventType)]...)
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return len(handlers), nil
}

func (b *stubEventBus) Subscribe(namespace, eventType string, handler func(payload string)) func() {
	if b.subErr != nil {
		return func() {}
	}
	k := b.key(namespace, eventType)
	b.mu.Lock()
	b.handlers[k] = append(b.handlers[k], handler)
	idx := len(b.handlers[k]) - 1
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		hs := b.handlers[k]
		if idx < len(hs) {
			b.handlers[k] = append(hs[:idx], hs[idx+1:]...)
		}
	}
}

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

func TestMigrateNilEventBus(t *testing.T) {
	// VALIDATES: MigrateInterface rejects nil event bus.
	// PREVENTS: Nil dereference in phase 3 subscription.
	cfg := MigrateConfig{
		OldIface: "lo0",
		NewIface: "lo1",
		Address:  "10.0.0.1/24",
	}
	err := MigrateInterface(cfg, nil, 5)
	if err == nil {
		t.Fatal("expected error for nil event bus, got nil")
	}
	if !strings.Contains(err.Error(), "event bus is nil") {
		t.Errorf("error = %q, want containing 'event bus is nil'", err.Error())
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
	err := MigrateInterface(cfg, newStubEventBus(), 0)
	if err == nil {
		t.Fatal("expected error for zero timeout, got nil")
	}
	if !strings.Contains(err.Error(), "timeout must be positive") {
		t.Errorf("error = %q, want containing 'timeout must be positive'", err.Error())
	}
}

func TestMigrateBackendFailureRollback(t *testing.T) {
	// VALIDATES: Phase 2 backend failure triggers rollback cleanly.
	// PREVENTS: Leaked interfaces when the first operation fails.
	//
	// MigrateInterface requires a loaded backend. We register a mock that
	// fails AddAddress (simulating no root/netlink), which triggers the
	// error path. Rollback calls are best-effort.
	_ = RegisterBackend("test-migrate", func() (Backend, error) {
		return &mockMigrateBackend{}, nil
	})
	if err := LoadBackend("test-migrate"); err != nil {
		t.Fatalf("load test backend: %v", err)
	}
	defer func() { _ = CloseBackend() }()

	cfg := MigrateConfig{
		OldIface: "lo0",
		NewIface: "lo1",
		Address:  "10.0.0.1/24",
	}
	err := MigrateInterface(cfg, newStubEventBus(), 5)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
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
	err := MigrateInterface(cfg, newStubEventBus(), 5)
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

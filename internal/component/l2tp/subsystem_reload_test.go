package l2tp

import (
	"context"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// fakeConfigProvider is a minimal ze.ConfigProvider satisfying the
// narrow slice of the interface Reload consumes (Get). Other methods
// are stubs that would never be invoked by Reload; they return zero
// values so the compile-time interface check passes.
type fakeConfigProvider struct {
	trees map[string]map[string]any
}

func (f *fakeConfigProvider) Load(_ string) error { return nil }
func (f *fakeConfigProvider) Get(root string) (map[string]any, error) {
	if f.trees == nil {
		return map[string]any{}, nil
	}
	if t, ok := f.trees[root]; ok {
		return t, nil
	}
	return map[string]any{}, nil
}
func (f *fakeConfigProvider) Validate() []error                     { return nil }
func (f *fakeConfigProvider) Save(_ string) error                   { return nil }
func (f *fakeConfigProvider) Watch(_ string) <-chan ze.ConfigChange { return nil }
func (f *fakeConfigProvider) Schema() ze.SchemaTree                 { return ze.SchemaTree{} }
func (f *fakeConfigProvider) RegisterSchema(_, _ string) error      { return nil }

// newStartedSubsystem returns a Subsystem marked `started` (so the mu
// guard in Reload passes) with fixed initial Parameters and a slog
// logger. No reactors / listeners are wired; the setters on the
// reactor slice are a no-op because the slice is empty.
func newStartedSubsystem(_ *testing.T, p Parameters) *Subsystem {
	return &Subsystem{
		params:  p,
		started: true,
		logger:  slog.Default(),
	}
}

// VALIDATES: AC-1 -- shared-secret change is hot-applied.
func TestReloadAppliesSharedSecret(t *testing.T) {
	s := newStartedSubsystem(t, Parameters{
		Enabled:       true,
		HelloInterval: 60 * time.Second,
		SharedSecret:  "old",
		ListenAddrs:   []netip.AddrPort{netip.MustParseAddrPort("0.0.0.0:1701")},
	})
	cfg := &fakeConfigProvider{trees: map[string]map[string]any{
		"l2tp": {
			"enabled":        "true",
			"hello-interval": "60",
			"shared-secret":  "new",
		},
		"environment": {
			"l2tp": map[string]any{
				"server": map[string]any{
					"default": map[string]any{"ip": "0.0.0.0", "port": "1701"},
				},
			},
		},
	}}
	require.NoError(t, s.Reload(context.Background(), cfg))
	require.Equal(t, "new", s.params.SharedSecret)
}

// VALIDATES: AC-2 -- hello-interval change is hot-applied.
func TestReloadAppliesHelloInterval(t *testing.T) {
	s := newStartedSubsystem(t, Parameters{
		Enabled:       true,
		HelloInterval: 60 * time.Second,
		ListenAddrs:   []netip.AddrPort{netip.MustParseAddrPort("0.0.0.0:1701")},
	})
	cfg := &fakeConfigProvider{trees: map[string]map[string]any{
		"l2tp": {
			"hello-interval": "120",
		},
		"environment": {
			"l2tp": map[string]any{
				"server": map[string]any{
					"default": map[string]any{"ip": "0.0.0.0", "port": "1701"},
				},
			},
		},
	}}
	require.NoError(t, s.Reload(context.Background(), cfg))
	require.Equal(t, 120*time.Second, s.params.HelloInterval)
}

// VALIDATES: AC-3 -- max-tunnels and max-sessions hot-apply.
func TestReloadAppliesLimits(t *testing.T) {
	s := newStartedSubsystem(t, Parameters{
		Enabled:       true,
		HelloInterval: 60 * time.Second,
		MaxTunnels:    100,
		MaxSessions:   200,
		ListenAddrs:   []netip.AddrPort{netip.MustParseAddrPort("0.0.0.0:1701")},
	})
	cfg := &fakeConfigProvider{trees: map[string]map[string]any{
		"l2tp": {
			"max-tunnels":  "500",
			"max-sessions": "1000",
		},
		"environment": {
			"l2tp": map[string]any{
				"server": map[string]any{
					"default": map[string]any{"ip": "0.0.0.0", "port": "1701"},
				},
			},
		},
	}}
	require.NoError(t, s.Reload(context.Background(), cfg))
	require.Equal(t, uint16(500), s.params.MaxTunnels)
	require.Equal(t, uint16(1000), s.params.MaxSessions)
}

// VALIDATES: AC-4 -- listener endpoint change is rejected and logged.
func TestReloadRejectsListenerChange(t *testing.T) {
	s := newStartedSubsystem(t, Parameters{
		Enabled:       true,
		HelloInterval: 60 * time.Second,
		ListenAddrs:   []netip.AddrPort{netip.MustParseAddrPort("0.0.0.0:1701")},
	})
	cfg := &fakeConfigProvider{trees: map[string]map[string]any{
		"l2tp": {},
		"environment": {
			"l2tp": map[string]any{
				"server": map[string]any{
					"default": map[string]any{"ip": "192.0.2.1", "port": "1701"},
				},
			},
		},
	}}
	require.NoError(t, s.Reload(context.Background(), cfg))
	// Listener change MUST NOT be applied -- params retain old value.
	require.Equal(t, []netip.AddrPort{netip.MustParseAddrPort("0.0.0.0:1701")}, s.params.ListenAddrs)
}

// VALIDATES: AC-4 -- enabled flip is rejected and logged.
func TestReloadRejectsEnabledFlip(t *testing.T) {
	s := newStartedSubsystem(t, Parameters{
		Enabled:       true,
		HelloInterval: 60 * time.Second,
		ListenAddrs:   []netip.AddrPort{netip.MustParseAddrPort("0.0.0.0:1701")},
	})
	cfg := &fakeConfigProvider{trees: map[string]map[string]any{
		"l2tp": {"enabled": "false"},
		"environment": {
			"l2tp": map[string]any{
				"server": map[string]any{
					"default": map[string]any{"ip": "0.0.0.0", "port": "1701"},
				},
			},
		},
	}}
	require.NoError(t, s.Reload(context.Background(), cfg))
	// The flip MUST NOT be applied.
	require.True(t, s.params.Enabled, "enabled flip must be rejected")
}

// VALIDATES: AC-5 -- identical Parameters produce a no-op reload.
func TestReloadNoOpOnIdentical(t *testing.T) {
	s := newStartedSubsystem(t, Parameters{
		Enabled:       true,
		HelloInterval: 60 * time.Second,
		SharedSecret:  "same",
		MaxTunnels:    10,
		MaxSessions:   20,
		AuthMethod:    DefaultAuthMethod,
		ListenAddrs:   []netip.AddrPort{netip.MustParseAddrPort("0.0.0.0:1701")},
	})
	prev := s.params
	cfg := &fakeConfigProvider{trees: map[string]map[string]any{
		"l2tp": {
			"hello-interval": "60",
			"shared-secret":  "same",
			"max-tunnels":    "10",
			"max-sessions":   "20",
		},
		"environment": {
			"l2tp": map[string]any{
				"server": map[string]any{
					"default": map[string]any{"ip": "0.0.0.0", "port": "1701"},
				},
			},
		},
	}}
	require.NoError(t, s.Reload(context.Background(), cfg))
	require.Equal(t, prev, s.params)
}

// VALIDATES: AC-26 -- malformed tree returns error and leaves
// Parameters untouched.
func TestReloadMalformedTree(t *testing.T) {
	s := newStartedSubsystem(t, Parameters{
		Enabled:       true,
		HelloInterval: 60 * time.Second,
	})
	prev := s.params
	cfg := &fakeConfigProvider{trees: map[string]map[string]any{
		"l2tp": {
			"hello-interval": "not-a-number",
		},
	}}
	err := s.Reload(context.Background(), cfg)
	require.Error(t, err)
	require.Equal(t, prev, s.params, "params must be unchanged on error")
}

// VALIDATES: Reload before Start returns ErrSubsystemNotStarted.
func TestReloadBeforeStart(t *testing.T) {
	s := &Subsystem{
		params: Parameters{Enabled: true},
		logger: slog.Default(),
	}
	err := s.Reload(context.Background(), &fakeConfigProvider{})
	require.ErrorIs(t, err, ErrSubsystemNotStarted)
}

// VALIDATES: listenAddrsEqual accepts reordered endpoints as equal.
func TestListenAddrsEqualIgnoresOrder(t *testing.T) {
	a := []netip.AddrPort{
		netip.MustParseAddrPort("0.0.0.0:1701"),
		netip.MustParseAddrPort("127.0.0.1:1701"),
	}
	b := []netip.AddrPort{
		netip.MustParseAddrPort("127.0.0.1:1701"),
		netip.MustParseAddrPort("0.0.0.0:1701"),
	}
	require.True(t, listenAddrsEqual(a, b))

	c := []netip.AddrPort{netip.MustParseAddrPort("0.0.0.0:1701")}
	require.False(t, listenAddrsEqual(a, c))
}

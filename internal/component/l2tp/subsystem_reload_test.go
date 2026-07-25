package l2tp

import (
	"context"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/ze"
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

// VALIDATES: spec-l2tp-dead-peer-detection AC-6 -- hello-retries
// (dead-peer detection threshold) is hot-applied on reload.
func TestReloadAppliesHelloRetries(t *testing.T) {
	s := newStartedSubsystem(t, Parameters{
		Enabled:       true,
		HelloInterval: 60 * time.Second,
		HelloRetries:  1,
		ListenAddrs:   []netip.AddrPort{netip.MustParseAddrPort("0.0.0.0:1701")},
	})
	cfg := &fakeConfigProvider{trees: map[string]map[string]any{
		"l2tp": {
			"hello-interval": "60",
			"hello-retries":  "3",
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
	require.Equal(t, uint8(3), s.params.HelloRetries)
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
		HelloRetries:  DefaultHelloRetries,
		SharedSecret:  "same",
		MaxTunnels:    10,
		MaxSessions:   20,
		AuthMethod:    DefaultAuthMethod,
		AuthTimeout:   DefaultAuthTimeoutSecs * time.Second,
		EnableIPCP:    true,
		EnableIPv6CP:  true,
		NCPTimeout:    DefaultNCPTimeoutSecs * time.Second,
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

// VALIDATES: AC-6 -- remote dial targets are hot-applied on reload; the
// provider-map parse path (appendRemotesFromProvider) reads address/port/
// shared-secret/outgoing-calls.
func TestReloadAppliesRemotes(t *testing.T) {
	s := newStartedSubsystem(t, Parameters{
		Enabled:       true,
		HelloInterval: 60 * time.Second,
		ListenAddrs:   []netip.AddrPort{netip.MustParseAddrPort("0.0.0.0:1701")},
	})
	cfg := &fakeConfigProvider{trees: map[string]map[string]any{
		"l2tp": {
			"remote": map[string]any{
				"lns1": map[string]any{
					"address":        "203.0.113.5",
					"port":           "1701",
					"shared-secret":  "s3cr3t",
					"outgoing-calls": "true",
				},
			},
			"relay": map[string]any{
				"wholesale": map[string]any{"remote": "lns1"},
			},
		},
	}}
	require.NoError(t, s.Reload(context.Background(), cfg))
	require.Len(t, s.params.Remotes, 1)
	require.Equal(t, "203.0.113.5:1701", s.params.Remotes[0].Address.String())
	require.True(t, s.params.Remotes[0].OutgoingCalls)
	require.Len(t, s.params.Relays, 1)
	require.Equal(t, "wholesale", s.params.Relays[0].Service)
	require.Equal(t, "lns1", s.params.Relays[0].Remote)
	got, ok := s.params.LookupRemote("lns1")
	require.True(t, ok)
	require.Equal(t, "s3cr3t", got.SharedSecret)
}

// VALIDATES: AC-6 -- a relay binding to an undeclared remote is rejected on
// the reload (provider-map) path too, not just the config.Tree path.
func TestReloadRejectsRelayUnknownRemote(t *testing.T) {
	s := newStartedSubsystem(t, Parameters{
		Enabled:       true,
		HelloInterval: 60 * time.Second,
		ListenAddrs:   []netip.AddrPort{netip.MustParseAddrPort("0.0.0.0:1701")},
	})
	cfg := &fakeConfigProvider{trees: map[string]map[string]any{
		"l2tp": {
			"relay": map[string]any{
				"wholesale": map[string]any{"remote": "ghost"},
			},
		},
	}}
	err := s.Reload(context.Background(), cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown remote")
}

// VALIDATES: remotesEqual / relaysEqual detect membership and order changes.
func TestRemotesAndRelaysEqual(t *testing.T) {
	a := []Remote{{Name: "x", Address: netip.MustParseAddrPort("10.0.0.1:1701")}}
	b := []Remote{{Name: "x", Address: netip.MustParseAddrPort("10.0.0.1:1701")}}
	require.True(t, remotesEqual(a, b))
	c := []Remote{{Name: "x", Address: netip.MustParseAddrPort("10.0.0.2:1701")}}
	require.False(t, remotesEqual(a, c))
	require.False(t, remotesEqual(a, nil))

	ra := []RelayBinding{{Service: "s", Remote: "x"}}
	rb := []RelayBinding{{Service: "s", Remote: "x"}}
	require.True(t, relaysEqual(ra, rb))
	require.False(t, relaysEqual(ra, []RelayBinding{{Service: "s", Remote: "y"}}))
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

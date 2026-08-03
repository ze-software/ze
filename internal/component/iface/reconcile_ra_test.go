package iface

import (
	"errors"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRASender records that it was started and whether it was stopped.
type stubRASender struct {
	spec    RASenderSpec
	stopped bool
}

func (s *stubRASender) Stop() { s.stopped = true }

// raFactoryRecorder installs a stub factory for the duration of a test and
// restores whatever was there before, so one test never leaks a factory into
// the next.
type raFactoryRecorder struct {
	started []*stubRASender
	err     error
}

func newRAFactoryRecorder(t *testing.T) *raFactoryRecorder {
	t.Helper()
	r := &raFactoryRecorder{}
	previous := raSenderFactory
	t.Cleanup(func() { raSenderFactory = previous })
	SetRASenderFactory(func(spec RASenderSpec) (RAStopper, error) {
		if r.err != nil {
			return nil, r.err
		}
		s := &stubRASender{spec: spec}
		r.started = append(r.started, s)
		return s, nil
	})
	return r
}

func raTestConfig(t *testing.T, body string) *ifaceConfig {
	t.Helper()
	return mustParseIfaceJSON(t, raUnitJSON(body))
}

// VALIDATES: spec AC-5 and assumption A-5. reconcileRA starts a sender for a
// unit that enables Router Advertisements, stops it when the config goes away,
// and restarts it when the advertised values change.
// PREVENTS: a config reload that leaves the old advertisement on the wire, and
// a stopped sender that keeps its socket and its multicast group join.
func TestReconcileRA(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	t.Run("starts a sender for an enabled unit", func(t *testing.T) {
		rec := newRAFactoryRecorder(t)
		active := make(map[raUnitKey]raEntry)
		cfg := raTestConfig(t, `{"enabled": "true", "prefix": {"2001:db8:1::/64": {}}}`)

		reconcileRA(cfg, active, log)

		require.Len(t, rec.started, 1)
		require.Len(t, active, 1)
		spec := rec.started[0].spec
		assert.Equal(t, "eth0", spec.Interface)
		assert.Equal(t, "0", spec.Unit)
		require.Len(t, spec.Advertisement.Prefixes, 1)
		assert.Equal(t, netip.MustParsePrefix("2001:db8:1::/64"), spec.Advertisement.Prefixes[0].Prefix)
	})

	t.Run("does not start a sender when the container is disabled", func(t *testing.T) {
		rec := newRAFactoryRecorder(t)
		active := make(map[raUnitKey]raEntry)

		reconcileRA(raTestConfig(t, `{"enabled": "false", "prefix": {"2001:db8:1::/64": {}}}`), active, log)

		assert.Empty(t, rec.started)
		assert.Empty(t, active)
	})

	t.Run("stops the sender when the config is removed", func(t *testing.T) {
		rec := newRAFactoryRecorder(t)
		active := make(map[raUnitKey]raEntry)
		reconcileRA(raTestConfig(t, `{"enabled": "true"}`), active, log)
		require.Len(t, rec.started, 1)

		empty := mustParseIfaceJSON(t, `{"interface": {"ethernet": {"eth0": {"unit": {"0": {}}}}}}`)
		reconcileRA(empty, active, log)

		assert.True(t, rec.started[0].stopped, "sender must be stopped when its config goes away")
		assert.Empty(t, active)
	})

	t.Run("stops the sender when enabled turns false", func(t *testing.T) {
		rec := newRAFactoryRecorder(t)
		active := make(map[raUnitKey]raEntry)
		reconcileRA(raTestConfig(t, `{"enabled": "true"}`), active, log)
		require.Len(t, rec.started, 1)

		reconcileRA(raTestConfig(t, `{"enabled": "false"}`), active, log)

		assert.True(t, rec.started[0].stopped)
		assert.Empty(t, active)
	})

	t.Run("leaves an unchanged sender running", func(t *testing.T) {
		rec := newRAFactoryRecorder(t)
		active := make(map[raUnitKey]raEntry)
		body := `{"enabled": "true", "prefix": {"2001:db8:1::/64": {}}, "rdnss": {"server": ["2001:4860:4860::8888"]}}`
		reconcileRA(raTestConfig(t, body), active, log)
		reconcileRA(raTestConfig(t, body), active, log)

		require.Len(t, rec.started, 1, "an unchanged config must not restart the sender")
		assert.False(t, rec.started[0].stopped)
		assert.Len(t, active, 1)
	})

	changes := []struct {
		name string
		body string
	}{
		{"router lifetime", `{"enabled": "true", "router-lifetime": "0"}`},
		{"managed flag", `{"enabled": "true", "managed": "true"}`},
		{"hop limit", `{"enabled": "true", "hop-limit": "32"}`},
		{"interval", `{"enabled": "true", "maximum-interval": "900"}`},
		{"added prefix", `{"enabled": "true", "prefix": {"2001:db8:1::/64": {}}}`},
		{"resolver list", `{"enabled": "true", "rdnss": {"server": ["2001:4860:4860::8888"]}}`},
	}
	for _, tt := range changes {
		t.Run("restarts the sender when the "+tt.name+" changes", func(t *testing.T) {
			rec := newRAFactoryRecorder(t)
			active := make(map[raUnitKey]raEntry)
			reconcileRA(raTestConfig(t, `{"enabled": "true"}`), active, log)
			require.Len(t, rec.started, 1)

			reconcileRA(raTestConfig(t, tt.body), active, log)

			require.Len(t, rec.started, 2, "a changed advertisement must restart the sender")
			assert.True(t, rec.started[0].stopped, "the old sender must stop before the new one starts")
			assert.Len(t, active, 1)
		})
	}

	t.Run("a prefix whose lifetime changes restarts the sender", func(t *testing.T) {
		rec := newRAFactoryRecorder(t)
		active := make(map[raUnitKey]raEntry)
		reconcileRA(raTestConfig(t, `{"enabled": "true", "prefix": {"2001:db8:1::/64": {"valid-lifetime": "3600", "preferred-lifetime": "1800"}}}`), active, log)
		require.Len(t, rec.started, 1)

		reconcileRA(raTestConfig(t, `{"enabled": "true", "prefix": {"2001:db8:1::/64": {"valid-lifetime": "7200", "preferred-lifetime": "1800"}}}`), active, log)

		require.Len(t, rec.started, 2)
		assert.True(t, rec.started[0].stopped)
	})

	t.Run("a failing factory leaves nothing running", func(t *testing.T) {
		rec := newRAFactoryRecorder(t)
		rec.err = errors.New("no raw socket")
		active := make(map[raUnitKey]raEntry)

		reconcileRA(raTestConfig(t, `{"enabled": "true"}`), active, log)

		assert.Empty(t, active, "a sender that failed to start must not be recorded as running")
	})
}

// VALIDATES: removing the iface-ra plugin leaves the factory unset and
// reconcileRA a no-op, which is the delete-the-folder test for a plugin.
// PREVENTS: the iface component depending on a plugin that is not built in.
func TestReconcileRANoFactoryIsNoOp(t *testing.T) {
	previous := raSenderFactory
	t.Cleanup(func() { raSenderFactory = previous })
	raSenderFactory = nil

	active := make(map[raUnitKey]raEntry)
	reconcileRA(raTestConfig(t, `{"enabled": "true"}`), active, slog.New(slog.DiscardHandler))

	assert.Empty(t, active)
}

// VALIDATES: the sender spec carries the values the encoder needs, including
// the RFC 8106 Section 5.1 default lifetime and the RFC 4861 Section 6.2.1
// intervals.
// PREVENTS: config values reaching the reconcile and then being dropped on the
// way to the sender, which advertises defaults an operator never wrote.
func TestRASenderSpecCarriesConfig(t *testing.T) {
	rec := newRAFactoryRecorder(t)
	active := make(map[raUnitKey]raEntry)

	reconcileRA(raTestConfig(t, `{
		"enabled": "true",
		"maximum-interval": "900",
		"minimum-interval": "300",
		"router-lifetime": "1800",
		"hop-limit": "32",
		"managed": "true",
		"other-config": "true",
		"reachable-time": "30000",
		"retransmit-timer": "1000",
		"prefix": {"2001:db8:1::/64": {"on-link": "true", "autonomous": "true", "valid-lifetime": "7200", "preferred-lifetime": "3600"}},
		"rdnss": {"server": ["2001:4860:4860::8888"]}
	}`), active, slog.New(slog.DiscardHandler))

	require.Len(t, rec.started, 1)
	spec := rec.started[0].spec
	assert.Equal(t, 900*time.Second, spec.MaximumInterval)
	assert.Equal(t, 300*time.Second, spec.MinimumInterval)

	ad := spec.Advertisement
	assert.Equal(t, uint8(32), ad.CurHopLimit)
	assert.True(t, ad.Managed)
	assert.True(t, ad.OtherConfig)
	assert.Equal(t, uint16(1800), ad.RouterLifetime)
	assert.Equal(t, uint32(30000), ad.ReachableTime)
	assert.Equal(t, uint32(1000), ad.RetransTimer)

	require.Len(t, ad.Prefixes, 1)
	assert.True(t, ad.Prefixes[0].OnLink)
	assert.True(t, ad.Prefixes[0].Autonomous)
	assert.Equal(t, uint32(7200), ad.Prefixes[0].ValidLifetime)
	assert.Equal(t, uint32(3600), ad.Prefixes[0].PreferredLifetime)

	require.Len(t, ad.RDNSS, 1)
	// RFC 8106 Section 5.1 recommends 3 x MaxRtrAdvInterval when no lifetime
	// is configured, and the maximum interval here is 900 seconds.
	assert.Equal(t, uint32(2700), ad.RDNSSLifetime)
}

// VALIDATES: every interface kind that can carry a unit reaches reconcileRA.
// PREVENTS: an RA block that config verify accepts on a veth or a bridge and
// that no sender ever picks up, which is silent-wrong rather than an error.
func TestReconcileRACoversEveryInterfaceKind(t *testing.T) {
	kinds := []struct {
		name string
		json string
		want string
	}{
		{"ethernet", `{"interface": {"ethernet": {"eth0": {"unit": {"0": {"ipv6": {"router-advertisement": {"enabled": "true"}}}}}}}}`, "eth0"},
		{"veth", `{"interface": {"veth": {"veth0": {"peer": "veth1", "unit": {"0": {"ipv6": {"router-advertisement": {"enabled": "true"}}}}}}}}`, "veth0"},
		{"bridge", `{"interface": {"bridge": {"br0": {"unit": {"0": {"ipv6": {"router-advertisement": {"enabled": "true"}}}}}}}}`, "br0"},
		{"dummy", `{"interface": {"dummy": {"dum0": {"unit": {"0": {"ipv6": {"router-advertisement": {"enabled": "true"}}}}}}}}`, "dum0"},
	}

	for _, tt := range kinds {
		t.Run(tt.name, func(t *testing.T) {
			rec := newRAFactoryRecorder(t)
			active := make(map[raUnitKey]raEntry)

			reconcileRA(mustParseIfaceJSON(t, tt.json), active, slog.New(slog.DiscardHandler))

			require.Len(t, rec.started, 1, "no sender started for a %s unit", tt.name)
			assert.Equal(t, tt.want, rec.started[0].spec.Interface)
		})
	}
}

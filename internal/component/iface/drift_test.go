package iface

import (
	"errors"
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/drift"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func setupIfaceDriftBackend(t *testing.T, b Backend) {
	t.Helper()
	resetAddressOwners(t)

	backendsMu.Lock()
	prevBackends := maps.Clone(backends)
	prevActive := activeBackend
	prevName := activeBackendName
	prevGen := activeBackendGeneration
	activeBackend = b
	if b == nil {
		activeBackendName = ""
	} else {
		activeBackendName = "drift-test"
	}
	backendsMu.Unlock()

	t.Cleanup(func() {
		backendsMu.Lock()
		backends = prevBackends
		activeBackend = prevActive
		activeBackendName = prevName
		activeBackendGeneration = prevGen
		backendsMu.Unlock()
	})
}

// VALIDATES: checkIfaceDrift returns StatusClean with zero checked when cfg is nil.
// PREVENTS: unconfigured interface plugin reporting drift or error.
func TestIfaceDriftNilConfigClean(t *testing.T) {
	setupIfaceDriftBackend(t, nil)

	out := checkIfaceDrift(nil)
	assert.Equal(t, componentNameInterface, out.Subsystem)
	assert.Equal(t, string(drift.StatusClean), out.Status)
	assert.Equal(t, 0, out.Checked)
	assert.Equal(t, 0, out.Drifted)
	assert.False(t, out.Truncated)
	require.NotNil(t, out.Findings)
	assert.Empty(t, out.Findings)
}

// VALIDATES: checkIfaceDrift returns StatusUnavailable when active backend is nil, without loading a backend.
// PREVENTS: read-only drift check calling LoadBackend or misreporting clean without reading kernel state.
func TestIfaceDriftNilBackendUnavailable(t *testing.T) {
	setupIfaceDriftBackend(t, nil)
	cfg := &ifaceConfig{
		Dummy: []ifaceEntry{{Name: "dum0"}},
	}

	out := checkIfaceDrift(cfg)
	assert.Equal(t, componentNameInterface, out.Subsystem)
	assert.Equal(t, string(drift.StatusUnavailable), out.Status)
	assert.Equal(t, 0, out.Checked)
	assert.Equal(t, 0, out.Drifted)
	require.NotNil(t, out.Findings)
	assert.Empty(t, out.Findings)
	assert.Nil(t, GetBackend(), "drift check must never call LoadBackend")
}

// VALIDATES: checkIfaceDrift returns StatusClean when managed interfaces, MTUs, and normalized addresses match live state.
// PREVENTS: false-positive drift findings on canonical IPv6 or matching configured links.
func TestIfaceDriftCleanMatch(t *testing.T) {
	fb := &fakeBackend{
		ifaces: map[string]fakeIface{
			"dum0": {name: "dum0", linkType: TypeDummy, mtu: 1400, state: LinkStateUp},
			"eth0": {name: "eth0", linkType: zeTypeEthernet, mtu: 9000, state: LinkStateUp},
		},
		addrs: map[string][]string{
			"dum0": {"192.0.2.1/32", "2001:db8::1/64"},
			"eth0": {"198.51.100.1/24"},
		},
	}
	setupIfaceDriftBackend(t, fb)

	cfg := &ifaceConfig{
		Dummy: []ifaceEntry{{
			Name:  "dum0",
			MTU:   1400,
			Units: []unitEntry{{Label: "0", Addresses: []string{"192.0.2.1/32", "2001:0db8::1/64"}}},
		}},
		Ethernet: []ifaceEntry{{
			Name:  "eth0",
			MTU:   9000,
			Units: []unitEntry{{Label: "0", Addresses: []string{"198.51.100.1/24"}}},
		}},
	}

	out := checkIfaceDrift(cfg)
	assert.Equal(t, componentNameInterface, out.Subsystem)
	assert.Equal(t, string(drift.StatusClean), out.Status)
	assert.Equal(t, 2, out.Checked)
	assert.Equal(t, 0, out.Drifted)
	assert.False(t, out.Truncated)
	require.NotNil(t, out.Findings)
	assert.Empty(t, out.Findings)
}

// VALIDATES: checkIfaceDrift reports KindMissing when a managed interface is absent from live interfaces.
// PREVENTS: externally deleted dummy/bridge/vlan links going unnoticed.
func TestIfaceDriftMissingInterface(t *testing.T) {
	fb := &fakeBackend{
		ifaces: map[string]fakeIface{},
	}
	setupIfaceDriftBackend(t, fb)

	cfg := &ifaceConfig{
		Dummy: []ifaceEntry{{
			Name:  "dum0",
			Units: []unitEntry{{Label: "0", Addresses: []string{"192.0.2.1/32"}}},
		}},
	}

	out := checkIfaceDrift(cfg)
	assert.Equal(t, string(drift.StatusDrifted), out.Status)
	assert.Equal(t, 1, out.Checked)
	assert.Equal(t, 1, out.Drifted)
	require.Len(t, out.Findings, 1)
	assert.Equal(t, "dum0", out.Findings[0].Subject)
	assert.Equal(t, string(drift.KindMissing), out.Findings[0].Kind)
	assert.Equal(t, driftStatePresent, out.Findings[0].Expected)
	assert.Equal(t, driftStateAbsent, out.Findings[0].Actual)
}

// VALIDATES: checkIfaceDrift reports KindMissing on "<iface>/<cidr>" when a desired address is missing on a live interface.
// PREVENTS: externally stripped interface addresses going undetected.
func TestIfaceDriftMissingAddress(t *testing.T) {
	fb := &fakeBackend{
		ifaces: map[string]fakeIface{
			"dum0": {name: "dum0", linkType: TypeDummy, mtu: 1500},
		},
		addrs: map[string][]string{
			"dum0": {"192.0.2.1/32"},
		},
	}
	setupIfaceDriftBackend(t, fb)

	cfg := &ifaceConfig{
		Dummy: []ifaceEntry{{
			Name: "dum0",
			Units: []unitEntry{{
				Label:     "0",
				Addresses: []string{"192.0.2.1/32", "192.0.2.2/32"},
			}},
		}},
	}

	out := checkIfaceDrift(cfg)
	assert.Equal(t, string(drift.StatusDrifted), out.Status)
	assert.Equal(t, 1, out.Checked)
	assert.Equal(t, 1, out.Drifted)
	require.Len(t, out.Findings, 1)
	assert.Equal(t, "dum0/192.0.2.2/32", out.Findings[0].Subject)
	assert.Equal(t, string(drift.KindMissing), out.Findings[0].Kind)
	assert.Equal(t, "192.0.2.2/32", out.Findings[0].Expected)
	assert.Equal(t, driftStateAbsent, out.Findings[0].Actual)
}

// VALIDATES: checkIfaceDrift reports KindMismatch on "<iface>/mtu" when configured non-zero MTU differs from live MTU.
// PREVENTS: silent MTU divergence on configured interfaces.
func TestIfaceDriftMTUMismatch(t *testing.T) {
	fb := &fakeBackend{
		ifaces: map[string]fakeIface{
			"dum0": {name: "dum0", linkType: TypeDummy, mtu: 1500},
		},
	}
	setupIfaceDriftBackend(t, fb)

	cfg := &ifaceConfig{
		Dummy: []ifaceEntry{{
			Name: "dum0",
			MTU:  9000,
		}},
	}

	out := checkIfaceDrift(cfg)
	assert.Equal(t, string(drift.StatusDrifted), out.Status)
	assert.Equal(t, 1, out.Checked)
	assert.Equal(t, 1, out.Drifted)
	require.Len(t, out.Findings, 1)
	assert.Equal(t, "dum0/mtu", out.Findings[0].Subject)
	assert.Equal(t, string(drift.KindMismatch), out.Findings[0].Kind)
	assert.Equal(t, "9000", out.Findings[0].Expected)
	assert.Equal(t, "1500", out.Findings[0].Actual)
}

// VALIDATES: checkIfaceDrift returns StatusError when Backend.ListInterfaces fails.
// PREVENTS: netlink dump failures being mistaken for clean interface state.
func TestIfaceDriftBackendListError(t *testing.T) {
	fb := &fakeBackend{
		listErr: errors.New("netlink link dump failed"),
	}
	setupIfaceDriftBackend(t, fb)

	cfg := &ifaceConfig{
		Dummy: []ifaceEntry{{Name: "dum0"}},
	}

	out := checkIfaceDrift(cfg)
	assert.Equal(t, string(drift.StatusError), out.Status)
	assert.Equal(t, "netlink link dump failed", out.Error)
	require.NotNil(t, out.Findings)
	assert.Empty(t, out.Findings)
}

// VALIDATES: checkIfaceDrift caps findings at drift.MaxFindingsPerSubsystem (64) and sets Truncated.
// PREVENTS: unbounded finding payloads when many interfaces or addresses drift.
func TestIfaceDriftTruncation(t *testing.T) {
	const total = drift.MaxFindingsPerSubsystem + 6
	fb := &fakeBackend{ifaces: map[string]fakeIface{}}
	setupIfaceDriftBackend(t, fb)

	dummies := make([]ifaceEntry, 0, total)
	for i := range total {
		var tb textbuf.Buffer
		dummies = append(dummies, ifaceEntry{
			Name: tb.Str("dum").Int(int64(i)).String(),
		})
	}
	cfg := &ifaceConfig{Dummy: dummies}

	out := checkIfaceDrift(cfg)
	assert.Equal(t, string(drift.StatusDrifted), out.Status)
	assert.Equal(t, total, out.Checked)
	assert.Equal(t, total, out.Drifted)
	assert.True(t, out.Truncated)
	assert.Len(t, out.Findings, drift.MaxFindingsPerSubsystem)
}

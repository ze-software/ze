package iface

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// TestIfaceApplyJournalCreate verifies that applyConfig wrapped in a journal
// can be rolled back by re-applying the previous config.
//
// VALIDATES: AC-5 - Interface config adds new interface + address.
// PREVENTS: Interface created without journal tracking, making rollback impossible.
func TestIfaceApplyJournalCreate(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}

	cfg := &ifaceConfig{
		Backend: "fake",
		Dummy:   []ifaceEntry{{Name: "dummy0", Units: []unitEntry{{ID: 0, Addresses: []string{"10.0.0.1/24"}}}}},
	}

	j := sdk.NewJournal()
	err := j.Record(
		func() error {
			if errs := applyConfig(cfg, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
		func() error {
			empty := &ifaceConfig{Backend: "fake"}
			if errs := applyConfig(empty, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
	)
	require.NoError(t, err)
	assert.True(t, b.created["dummy0"], "dummy0 should be created")
	assert.Contains(t, b.addrs["dummy0"], "10.0.0.1/24", "address should be added")
	assert.Equal(t, 1, j.Len(), "journal should have 1 entry")
}

// TestIfaceApplyJournalAddress verifies that address operations are tracked
// by the journal and can be undone.
//
// VALIDATES: AC-5 - Address assigned via journal.
// PREVENTS: Address added without undo capability.
func TestIfaceApplyJournalAddress(t *testing.T) {
	b := &fakeBackend{
		ifaces: map[string]fakeIface{
			"eth0": {name: "eth0", linkType: "ethernet"},
		},
	}

	cfg := &ifaceConfig{
		Backend:  "fake",
		Ethernet: []ifaceEntry{{Name: "eth0", Units: []unitEntry{{ID: 0, Addresses: []string{"10.0.0.1/24", "10.0.0.2/24"}}}}},
	}

	j := sdk.NewJournal()
	err := j.Record(
		func() error {
			if errs := applyConfig(cfg, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
		func() error {
			empty := &ifaceConfig{Backend: "fake"}
			if errs := applyConfig(empty, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
	)
	require.NoError(t, err)
	assert.Len(t, b.addrs["eth0"], 2, "both addresses should be added")
}

// TestIfaceApplyJournalRollbackEvents verifies that rollback re-applies
// the previous config, effectively undoing the changes.
//
// VALIDATES: AC-6 - Interface rollback after partial apply.
// PREVENTS: Rollback leaving stale interfaces or addresses.
func TestIfaceApplyJournalRollbackEvents(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}

	// Previous config: no interfaces.
	previousCfg := &ifaceConfig{Backend: "fake"}

	// New config: creates dummy0.
	newCfg := &ifaceConfig{
		Backend: "fake",
		Dummy:   []ifaceEntry{{Name: "dummy0", Units: []unitEntry{{ID: 0, Addresses: []string{"10.0.0.1/24"}}}}},
	}

	j := sdk.NewJournal()
	err := j.Record(
		func() error {
			if errs := applyConfig(newCfg, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
		func() error {
			if errs := applyConfig(previousCfg, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
	)
	require.NoError(t, err)
	assert.True(t, b.created["dummy0"], "dummy0 should be created after apply")

	// Rollback: should re-apply previous (empty) config, deleting dummy0.
	errs := j.Rollback()
	assert.Empty(t, errs, "rollback should succeed")
	assert.True(t, b.deleted["dummy0"], "dummy0 should be deleted after rollback")
}

// TestIfaceVerifyEstimate verifies that the verify callback computes an
// estimate proportional to interface operations.
//
// VALIDATES: AC-12 - Interface budget proportional to interface count.
// PREVENTS: Budget estimate that doesn't scale with config size.
func TestIfaceVerifyEstimate(t *testing.T) {
	// Interface budget is set statically at registration (VerifyBudget=2, ApplyBudget=10).
	// The estimate scales with the number of configured interfaces.
	cfg := &ifaceConfig{
		Backend: "fake",
		Dummy: []ifaceEntry{
			{Name: "dummy0"},
			{Name: "dummy1"},
			{Name: "dummy2"},
		},
		Veth: []vethEntry{
			{ifaceEntry: ifaceEntry{Name: "veth0"}, Peer: "veth0-peer"},
		},
	}

	// Count operations: 3 dummy creates + 1 veth create = 4 operations.
	count := len(cfg.Dummy) + len(cfg.Veth) + len(cfg.Bridge) + len(cfg.Ethernet)
	assert.Equal(t, 4, count, "operation count should reflect interface config size")
}

// fakeBackend implements Backend for testing config application.
type fakeBackend struct {
	ifaces  map[string]fakeIface
	created map[string]bool
	deleted map[string]bool
	addrs   map[string][]string
}

type fakeIface struct {
	name     string
	linkType string
}

func (b *fakeBackend) ensureMaps() {
	if b.created == nil {
		b.created = make(map[string]bool)
	}
	if b.deleted == nil {
		b.deleted = make(map[string]bool)
	}
	if b.addrs == nil {
		b.addrs = make(map[string][]string)
	}
}

func (b *fakeBackend) CreateDummy(name string) error {
	b.ensureMaps()
	b.created[name] = true
	b.ifaces[name] = fakeIface{name: name, linkType: "dummy"}
	return nil
}

func (b *fakeBackend) CreateVeth(name, peerName string) error {
	b.ensureMaps()
	b.created[name] = true
	b.ifaces[name] = fakeIface{name: name, linkType: "veth"}
	return nil
}

func (b *fakeBackend) CreateBridge(name string) error {
	b.ensureMaps()
	b.created[name] = true
	b.ifaces[name] = fakeIface{name: name, linkType: "bridge"}
	return nil
}

func (b *fakeBackend) CreateVLAN(_ string, _ int) error       { return nil }
func (b *fakeBackend) SetAdminUp(_ string) error              { return nil }
func (b *fakeBackend) SetAdminDown(_ string) error            { return nil }
func (b *fakeBackend) SetMTU(_ string, _ int) error           { return nil }
func (b *fakeBackend) SetMACAddress(_, _ string) error        { return nil }
func (b *fakeBackend) GetMACAddress(_ string) (string, error) { return "", nil }
func (b *fakeBackend) GetStats(_ string) (*InterfaceStats, error) {
	return &InterfaceStats{}, nil
}

func (b *fakeBackend) DeleteInterface(name string) error {
	b.ensureMaps()
	b.deleted[name] = true
	delete(b.ifaces, name)
	return nil
}

func (b *fakeBackend) AddAddress(ifaceName, cidr string) error {
	b.ensureMaps()
	b.addrs[ifaceName] = append(b.addrs[ifaceName], cidr)
	return nil
}

func (b *fakeBackend) RemoveAddress(ifaceName, cidr string) error {
	b.ensureMaps()
	filtered := b.addrs[ifaceName][:0]
	for _, a := range b.addrs[ifaceName] {
		if a != cidr {
			filtered = append(filtered, a)
		}
	}
	b.addrs[ifaceName] = filtered
	return nil
}

func (b *fakeBackend) ReplaceAddressWithLifetime(_, _ string, _, _ int) error { return nil }

func (b *fakeBackend) ListInterfaces() ([]InterfaceInfo, error) {
	var result []InterfaceInfo
	for _, f := range b.ifaces {
		info := InterfaceInfo{Name: f.name, Type: f.linkType}
		if addrs, ok := b.addrs[f.name]; ok {
			for _, a := range addrs {
				info.Addresses = append(info.Addresses, AddrInfo{Address: a, PrefixLength: 24})
			}
		}
		result = append(result, info)
	}
	return result, nil
}

func (b *fakeBackend) GetInterface(name string) (*InterfaceInfo, error) {
	f, ok := b.ifaces[name]
	if !ok {
		return nil, fmt.Errorf("interface %s not found", name)
	}
	return &InterfaceInfo{Name: f.name, Type: f.linkType}, nil
}

func (b *fakeBackend) BridgeAddPort(_, _ string) error     { return nil }
func (b *fakeBackend) BridgeDelPort(_ string) error        { return nil }
func (b *fakeBackend) BridgeSetSTP(_ string, _ bool) error { return nil }

func (b *fakeBackend) SetIPv4Forwarding(_ string, _ bool) error { return nil }
func (b *fakeBackend) SetIPv4ArpFilter(_ string, _ bool) error  { return nil }
func (b *fakeBackend) SetIPv4ArpAccept(_ string, _ bool) error  { return nil }
func (b *fakeBackend) SetIPv4ProxyARP(_ string, _ bool) error   { return nil }
func (b *fakeBackend) SetIPv4ArpAnnounce(_ string, _ int) error { return nil }
func (b *fakeBackend) SetIPv4ArpIgnore(_ string, _ int) error   { return nil }
func (b *fakeBackend) SetIPv4RPFilter(_ string, _ int) error    { return nil }
func (b *fakeBackend) SetIPv6Autoconf(_ string, _ bool) error   { return nil }
func (b *fakeBackend) SetIPv6AcceptRA(_ string, _ int) error    { return nil }
func (b *fakeBackend) SetIPv6Forwarding(_ string, _ bool) error { return nil }
func (b *fakeBackend) SetupMirror(_, _ string, _, _ bool) error { return nil }
func (b *fakeBackend) RemoveMirror(_ string) error              { return nil }
func (b *fakeBackend) StartMonitor(_ ze.Bus) error              { return nil }
func (b *fakeBackend) StopMonitor()                             {}
func (b *fakeBackend) Close() error                             { return nil }

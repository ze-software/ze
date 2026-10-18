package iface

import (
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ReplaceIPv4RPF records each whole set the apply hands the dataplane.
func (b *fakeBackend) ReplaceIPv4RPF(checks []RPFCheck) error {
	b.rpf4Calls = append(b.rpf4Calls, slices.Clone(checks))
	return nil
}

func rpf4Unit(m rpfMode) unitEntry {
	return unitEntry{Label: "0", IPv4: &ipv4Settings{RPFCheck: &m}}
}

// VALIDATES: desiredIPv4RPF turns the configured ipv4 rpf-check of every
// enabled unit into one check on its device and reads no IPv6 leaf: disable,
// a disabled unit, an unbound selector and an IPv6-only unit install none.
// METHOD: one config holding each case, compared with the exact set.
func TestDesiredIPv4RPF(t *testing.T) {
	disabledUnit := rpf4Unit(rpfModeStrict)
	disabledUnit.Disable = true
	cfg := &ifaceConfig{
		Dummy: []ifaceEntry{
			{Name: "dum0", Units: []unitEntry{rpf4Unit(rpfModeStrict)}},
			{Name: "dum1", Units: []unitEntry{rpf4Unit(rpfModeLoose)}},
			{Name: "dum2", Units: []unitEntry{rpf4Unit(rpfModeDisable)}},
			{Name: "dum3", Units: []unitEntry{disabledUnit}},
			{Name: "dum4", Units: []unitEntry{rpf6Unit(rpfModeStrict)}},
		},
		Ethernet: []ifaceEntry{
			{Name: "wan", Units: []unitEntry{func() unitEntry {
				u := rpf4Unit(rpfModeStrict)
				u.VLANID = 100
				return u
			}()}},
			{Name: "absent", Units: []unitEntry{rpf4Unit(rpfModeStrict)}},
		},
		Loopback: &loopbackEntry{Units: []unitEntry{rpf4Unit(rpfModeLoose)}},
	}
	devices := map[string]string{"wan": "xe7", "absent": ""}

	got := desiredIPv4RPF(cfg, devices)

	want := []RPFCheck{
		{Interface: "xe7.100", Mode: RPFStrict},
		{Interface: "dum0", Mode: RPFStrict},
		{Interface: "dum1", Mode: RPFLoose},
		{Interface: "lo", Mode: RPFLoose},
	}
	assert.ElementsMatch(t, want, got)
	assert.Empty(t, desiredIPv4RPF(nil, nil), "a nil config asks for no check")
	assert.Equal(t, []RPFCheck{{Interface: "dum4", Mode: RPFStrict}},
		desiredIPv6RPF(&ifaceConfig{Dummy: cfg.Dummy[3:]}, nil), "the ipv6 set reads no ipv4 leaf")
}

// VALIDATES: the ipv4 rpf-check leaf, parsed from config JSON, reaches the
// set the apply hands the backend, and so reaches a dataplane the rp_filter
// sysctl does not judge.
// PREVENTS: an IPv4 rpf-check that commits on backend vpp and checks only
// the Linux Control Plane tap, while VPP forwards every transit packet
// unchecked.
func TestApplyConfigHandsIPv4RPFToBackend(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{"interface": {"dummy": {"dum0": {"unit": {"0": {"ipv4": {"rpf-check": "strict"}}}}}}}`)
	b := &fakeBackend{ifaces: map[string]fakeIface{}}

	errs := applyConfig(cfg, nil, b)

	require.Empty(t, errs)
	require.NotEmpty(t, b.rpf4Calls)
	assert.Equal(t, []RPFCheck{{Interface: "dum0", Mode: RPFStrict}}, b.rpf4Calls[len(b.rpf4Calls)-1])
}

// VALIDATES: removing the leaf, or the whole interface, hands the backend a
// set without that check, so the dataplane turns it off.
// METHOD: apply a config with two checks, then one without the leaf on dum0,
// then one without dum1 at all, reading the set of each apply.
func TestApplyConfigRemovesIPv4RPF(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}
	first := &ifaceConfig{Backend: "fake", Dummy: []ifaceEntry{
		{Name: "dum0", Units: []unitEntry{rpf4Unit(rpfModeStrict)}},
		{Name: "dum1", Units: []unitEntry{rpf4Unit(rpfModeLoose)}},
	}}
	require.Empty(t, applyConfig(first, nil, b))
	assert.ElementsMatch(t, []RPFCheck{
		{Interface: "dum0", Mode: RPFStrict},
		{Interface: "dum1", Mode: RPFLoose},
	}, b.rpf4Calls[len(b.rpf4Calls)-1])

	leafGone := &ifaceConfig{Backend: "fake", Dummy: []ifaceEntry{
		{Name: "dum0", Units: []unitEntry{{Label: "0"}}},
		{Name: "dum1", Units: []unitEntry{rpf4Unit(rpfModeLoose)}},
	}}
	require.Empty(t, applyConfig(leafGone, first, b))
	assert.Equal(t, []RPFCheck{{Interface: "dum1", Mode: RPFLoose}}, b.rpf4Calls[len(b.rpf4Calls)-1])

	ifaceGone := &ifaceConfig{Backend: "fake", Dummy: []ifaceEntry{
		{Name: "dum0", Units: []unitEntry{{Label: "0"}}},
	}}
	require.Empty(t, applyConfig(ifaceGone, leafGone, b))
	assert.Empty(t, b.rpf4Calls[len(b.rpf4Calls)-1], "no check is left once no unit asks for one")
}

// VALIDATES: an apply that fails after the checks were replaced puts the
// previous config's checks back, like every other journaled step.
func TestApplyConfigRollsBackIPv4RPF(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}
	previous := &ifaceConfig{Backend: "fake", Dummy: []ifaceEntry{
		{Name: "dum0", Units: []unitEntry{rpf4Unit(rpfModeStrict)}},
	}}
	require.Empty(t, applyConfig(previous, nil, b))

	b.addAddressErr = map[string]error{addressErrKey("dum0", "10.0.0.1/24"): errors.New("address failed")}
	next := &ifaceConfig{Backend: "fake", Dummy: []ifaceEntry{
		{Name: "dum0", Units: []unitEntry{{Label: "0", Addresses: []string{"10.0.0.1/24"}, IPv4: &ipv4Settings{RPFCheck: new(rpfModeLoose)}}}},
	}}

	errs := applyConfig(next, previous, b)

	require.NotEmpty(t, errs)
	n := len(b.rpf4Calls)
	require.GreaterOrEqual(t, n, 2)
	assert.Equal(t, []RPFCheck{{Interface: "dum0", Mode: RPFLoose}}, b.rpf4Calls[n-2])
	assert.Equal(t, []RPFCheck{{Interface: "dum0", Mode: RPFStrict}}, b.rpf4Calls[n-1], "rollback restores the previous set")
}

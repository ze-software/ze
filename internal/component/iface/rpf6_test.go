package iface

import (
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ReplaceIPv6RPF records each whole set the apply hands the dataplane.
func (b *fakeBackend) ReplaceIPv6RPF(checks []RPFCheck) error {
	b.rpf6Calls = append(b.rpf6Calls, slices.Clone(checks))
	return nil
}

func rpf6Unit(m rpfMode) unitEntry {
	return unitEntry{Label: "0", IPv6: &ipv6Settings{RPFCheck: &m}}
}

// VALIDATES: desiredIPv6RPF turns the configured ipv6 rpf-check of every
// enabled unit into one check on its device, and nothing else: disable, a
// disabled interface, a disabled unit and an unbound selector install none.
// METHOD: one config holding each case, compared with the exact set.
func TestDesiredIPv6RPF(t *testing.T) {
	disabledUnit := rpf6Unit(rpfModeStrict)
	disabledUnit.Disable = true
	cfg := &ifaceConfig{
		Dummy: []ifaceEntry{
			{Name: "dum0", Units: []unitEntry{rpf6Unit(rpfModeStrict)}},
			{Name: "dum1", Units: []unitEntry{rpf6Unit(rpfModeLoose)}},
			{Name: "dum2", Units: []unitEntry{rpf6Unit(rpfModeDisable)}},
			{Name: "dum3", Disable: true, Units: []unitEntry{rpf6Unit(rpfModeStrict)}},
			{Name: "dum4", Units: []unitEntry{disabledUnit}},
			{Name: "dum5", Units: []unitEntry{{Label: "0", IPv6: &ipv6Settings{}}}},
		},
		Ethernet: []ifaceEntry{
			{Name: "wan", Units: []unitEntry{rpf6Unit(rpfModeStrict), func() unitEntry {
				u := rpf6Unit(rpfModeLoose)
				u.VLANID = 100
				return u
			}()}},
			{Name: "absent", Units: []unitEntry{rpf6Unit(rpfModeStrict)}},
		},
		Loopback: &loopbackEntry{Units: []unitEntry{rpf6Unit(rpfModeLoose)}},
	}
	devices := map[string]string{"wan": "eth7", "absent": ""}

	got := desiredIPv6RPF(cfg, devices)

	want := []RPFCheck{
		{Interface: "eth7", Mode: RPFStrict},
		{Interface: "eth7.100", Mode: RPFLoose},
		{Interface: "dum0", Mode: RPFStrict},
		{Interface: "dum1", Mode: RPFLoose},
		{Interface: "lo", Mode: RPFLoose},
	}
	assert.ElementsMatch(t, want, got)
	assert.Empty(t, desiredIPv6RPF(nil, nil), "a nil config asks for no check")
}

// VALIDATES: the ipv6 rpf-check leaf, parsed from config JSON, reaches the
// set the apply hands the backend. The leaf used to commit and then be
// dropped with a log line on the Linux backend.
// PREVENTS: an IPv6 rpf-check the operator believes is on and that no
// dataplane enforces.
func TestApplyConfigHandsIPv6RPFToBackend(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{"interface": {"dummy": {"dum0": {"unit": {"0": {"ipv6": {"rpf-check": "loose"}}}}}}}`)
	b := &fakeBackend{ifaces: map[string]fakeIface{}}

	errs := applyConfig(cfg, nil, b)

	require.Empty(t, errs)
	require.NotEmpty(t, b.rpf6Calls)
	assert.Equal(t, []RPFCheck{{Interface: "dum0", Mode: RPFLoose}}, b.rpf6Calls[len(b.rpf6Calls)-1])
}

// VALIDATES: removing the leaf, or the whole interface, hands the backend a
// set without that check, so the dataplane drops it. This is the stale-rule
// failure an incremental rule set has after an interface is removed.
// METHOD: apply a config with two checks, then one without the leaf on dum0,
// then one without dum1 at all, reading the set of each apply.
func TestApplyConfigRemovesIPv6RPF(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}
	first := &ifaceConfig{Backend: "fake", Dummy: []ifaceEntry{
		{Name: "dum0", Units: []unitEntry{rpf6Unit(rpfModeStrict)}},
		{Name: "dum1", Units: []unitEntry{rpf6Unit(rpfModeLoose)}},
	}}
	require.Empty(t, applyConfig(first, nil, b))
	assert.ElementsMatch(t, []RPFCheck{
		{Interface: "dum0", Mode: RPFStrict},
		{Interface: "dum1", Mode: RPFLoose},
	}, b.rpf6Calls[len(b.rpf6Calls)-1])

	leafGone := &ifaceConfig{Backend: "fake", Dummy: []ifaceEntry{
		{Name: "dum0", Units: []unitEntry{{Label: "0"}}},
		{Name: "dum1", Units: []unitEntry{rpf6Unit(rpfModeLoose)}},
	}}
	require.Empty(t, applyConfig(leafGone, first, b))
	assert.Equal(t, []RPFCheck{{Interface: "dum1", Mode: RPFLoose}}, b.rpf6Calls[len(b.rpf6Calls)-1])

	ifaceGone := &ifaceConfig{Backend: "fake", Dummy: []ifaceEntry{
		{Name: "dum0", Units: []unitEntry{{Label: "0"}}},
	}}
	require.Empty(t, applyConfig(ifaceGone, leafGone, b))
	assert.Empty(t, b.rpf6Calls[len(b.rpf6Calls)-1], "no check is left once no unit asks for one")
}

// VALIDATES: an apply that fails after the checks were replaced puts the
// previous config's checks back, like every other journaled step.
func TestApplyConfigRollsBackIPv6RPF(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}
	previous := &ifaceConfig{Backend: "fake", Dummy: []ifaceEntry{
		{Name: "dum0", Units: []unitEntry{rpf6Unit(rpfModeStrict)}},
	}}
	require.Empty(t, applyConfig(previous, nil, b))

	b.addAddressErr = map[string]error{addressErrKey("dum0", "2001:db8::1/64"): errors.New("address failed")}
	next := &ifaceConfig{Backend: "fake", Dummy: []ifaceEntry{
		{Name: "dum0", Units: []unitEntry{{Label: "0", Addresses: []string{"2001:db8::1/64"}, IPv6: &ipv6Settings{RPFCheck: new(rpfModeLoose)}}}},
	}}

	errs := applyConfig(next, previous, b)

	require.NotEmpty(t, errs)
	n := len(b.rpf6Calls)
	require.GreaterOrEqual(t, n, 2)
	assert.Equal(t, []RPFCheck{{Interface: "dum0", Mode: RPFLoose}}, b.rpf6Calls[n-2])
	assert.Equal(t, []RPFCheck{{Interface: "dum0", Mode: RPFStrict}}, b.rpf6Calls[n-1], "rollback restores the previous set")
}

// VALIDATES: the vpp backend refuses an ipv6 rpf-check at commit, naming the
// leaf, and the netlink backend accepts it. VPP forwards outside netfilter,
// and VPP's own uRPF drops every unicast packet from a link-local source, so
// the vpp backend refuses the check rather than break neighbor discovery.
// METHOD: the real schema's ze:backend gate over one config per backend.
func TestBackendGateIPv6RPFCheck(t *testing.T) {
	data := func(backend string) string {
		return `{"interface":{"backend":"` + backend + `","ethernet":{"xe0":{"name":"xe0","unit":{"u0":{"name":"u0","ipv6":{"rpf-check":"strict"}}}}}}}`
	}
	err := validateBackendGate(gateSection(data("vpp")), "vpp")
	require.Error(t, err, "vpp must refuse ipv6 rpf-check")
	assert.Contains(t, err.Error(), "rpf-check")
	assert.NoError(t, validateBackendGate(gateSection(data("netlink")), "netlink"))
}

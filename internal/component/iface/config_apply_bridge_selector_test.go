// Design: docs/architecture/iface/logical-name-resolution.md -- the apply path translates
// Related: config_apply.go -- devicesWithMAC, aggregatingDevices, the Phase 2a member loop

package iface

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bridgeMemberConfig is the operator config both defects live in: one ethernet
// entry bound to hardware by MAC, and a bridge that lists that entry as a
// member. Ze creates the bridge, so from the apply after it the kernel holds two
// devices carrying one address.
func bridgeMemberConfig(units ...unitEntry) *ifaceConfig {
	return &ifaceConfig{
		Ethernet: []ifaceEntry{{Name: "wan", MatchMAC: selectedPermMAC, Units: units}},
		Bridge: []bridgeEntry{{
			Name:    "br0",
			Members: []string{"wan"},
		}},
	}
}

// TestSelectorSkipsTheDeviceWearingAMemberAddress verifies that a hardware
// selector names only devices whose match address is their own. Linux gives a
// device an address it did not bring in two ways, and a selector that counts
// either one as a candidate reads as ambiguous and fails the whole apply.
//
// VALIDATES: a mac/match selector answers with one device when an aggregator
// wears a member's address.
// PREVENTS: the measured defect. A bridge reports no permanent address and takes
// its lowest-MAC port's, and a bond master takes a slave's while the slave keeps
// its own IFLA_PERM_ADDRESS. Either way two devices answered one selector,
// validateSelectors errored, and applyConfig rolled the whole interface config
// back -- including the Phase 1 work it had already done.
func TestSelectorSkipsTheDeviceWearingAMemberAddress(t *testing.T) {
	for _, tc := range []struct {
		name  string
		infos []InterfaceInfo
		want  string
	}{
		{
			name: "a bridge wears its port's address",
			infos: []InterfaceInfo{
				{Name: "enp1s0", Index: 2, MAC: selectedPermMAC, PermanentMAC: selectedPermMAC, MasterIndex: 10},
				{Name: "br0", Index: 10, MAC: selectedPermMAC},
			},
			want: "enp1s0",
		},
		{
			name: "a bond master wears its slave's address",
			infos: []InterfaceInfo{
				{Name: "enp1s0", Index: 2, MAC: selectedPermMAC, PermanentMAC: selectedPermMAC, MasterIndex: 11},
				{Name: "bond0", Index: 11, MAC: selectedPermMAC},
			},
			want: "enp1s0",
		},
		{
			name: "a bridge with no port keeps the address the kernel gave it",
			infos: []InterfaceInfo{
				{Name: "br0", Index: 10, MAC: selectedPermMAC},
				{Name: "enp1s0", Index: 2, MAC: decoyPermMAC, PermanentMAC: decoyPermMAC},
			},
			want: "br0",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			matched := devicesWithMAC(tc.infos, selectedPermMAC)

			require.Len(t, matched, 1, "one device owns the address the selector names")
			assert.Equal(t, tc.want, tc.infos[matched[0]].Name)
		})
	}
}

// TestSelectorStillRefusesTwoDevicesOwningOneAddress pins the fail-closed half
// that the fix above must not spend: two devices that each report the selected
// address as their OWN are still ambiguous, and binding to one of them is a
// guess about which physical port the entry's addresses reach.
//
// VALIDATES: an ambiguous selector is refused, not resolved.
// PREVENTS: a fix for the aggregator case that resolves a cloned MAC too.
func TestSelectorStillRefusesTwoDevicesOwningOneAddress(t *testing.T) {
	infos := []InterfaceInfo{
		{Name: "enp1s0", Index: 2, MAC: selectedPermMAC, PermanentMAC: selectedPermMAC},
		{Name: "enp2s0", Index: 5, MAC: selectedPermMAC, PermanentMAC: selectedPermMAC},
	}

	assert.Len(t, devicesWithMAC(infos, selectedPermMAC), 2)
}

// TestBridgeMemberIsEnslavedByItsSelectedDevice verifies that a member naming an
// interface entry is enslaved by the kernel device that entry's selector
// resolves to.
//
// VALIDATES: a bridge member is resolved through the same selector map the rest
// of the apply uses.
// PREVENTS: the measured defect. The bridge phase ran in Phase 1, before the
// selector map existed, and handed BridgeAddPort the logical entry name. A
// member naming an entry bound by mac/match or aliased by os-name enslaved
// whatever kernel device happened to carry that name -- the wrong-physical-port
// landing the selector exists to remove, with a whole bridge behind it.
func TestBridgeMemberIsEnslavedByItsSelectedDevice(t *testing.T) {
	b := selectorBackend()

	errs := applyConfig(bridgeMemberConfig(), nil, b)

	require.Empty(t, errs)
	require.Len(t, b.bridgePorts, 1)
	assert.Equal(t, bridgePortCall{op: bridgePortOpAdd, bridge: "br0", port: "enp1s0"}, b.bridgePorts[0],
		"the member must be enslaved by the device its selector names")
}

// TestBridgeMemberDefersWhenItsHardwareIsAbsent verifies that a member whose
// selector has no answer yet is left out of the bridge, and that leaving it out
// does not fail the commit.
//
// VALIDATES: an unbound member defers, as every other unbound setting does.
// PREVENTS: enslaving the device that merely shares the entry's logical name,
// and the alternative failure of refusing a config the YANG calls valid because
// a NIC has not appeared.
func TestBridgeMemberDefersWhenItsHardwareIsAbsent(t *testing.T) {
	b := selectorBackend()
	delete(b.ifaces, "enp1s0") // the selected NIC has not appeared yet

	errs := applyConfig(bridgeMemberConfig(), nil, b)

	require.Empty(t, errs, "a deferred member must not fail the commit")
	assert.Empty(t, b.bridgePorts, "no device may be enslaved on the logical name")
}

// TestSecondApplySurvivesTheBridgeZeJustBuilt verifies the two defects where
// they meet, which is the only place an operator meets them: ze builds the
// bridge on one apply and every apply after it reads a kernel holding two
// devices with one address.
//
// VALIDATES: an ethernet entry that is both selected by MAC and listed as a
// bridge member stays applied across a reload.
// PREVENTS: the measured defect end to end. From the apply after the one that
// created the bridge, validateSelectors called the selector ambiguous,
// applyConfig recorded a "hardware selector" failure and called rollbackPartial,
// and the ENTIRE interface config was refused and unwound -- Phase 1 included.
func TestSecondApplySurvivesTheBridgeZeJustBuilt(t *testing.T) {
	cfg := bridgeMemberConfig(unitEntry{Label: "0", Addresses: []string{"10.0.0.1/24"}})
	b := selectorBackend()

	require.Empty(t, applyConfig(cfg, nil, b), "the apply that creates the bridge")
	require.Equal(t, selectedPermMAC, b.ifaces["br0"].mac,
		"the bridge must wear its port's address, or this test proves nothing")

	errs := applyConfig(cfg, cfg, b)

	require.Empty(t, errs, "the bridge ze built must not make the next apply ambiguous")
	assert.Equal(t, []string{"10.0.0.1/24"}, b.addrs["enp1s0"])
	assert.Empty(t, b.addrs["wan"], "the coincidentally named device must get nothing")
}

// warningsFrom returns the message of every WARN-or-worse record the package
// logger was given while fn ran. The package logger discards by default, so the
// level a failure is reported at is not observable any other way.
func warningsFrom(t *testing.T, fn func()) []string {
	t.Helper()
	var out bytes.Buffer
	previous := loggerPtr.Load()
	loggerPtr.Store(slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { loggerPtr.Store(previous) })

	fn()

	var warnings []string
	for line := range strings.SplitSeq(out.String(), "\n") {
		if !strings.Contains(line, "level=WARN") && !strings.Contains(line, "level=ERROR") {
			continue
		}
		_, msg, found := strings.Cut(line, "msg=")
		if !found {
			continue
		}
		// A message carrying a space is quoted, and the attributes follow it on
		// the same line. Take the quoted run, so an attribute never reads as
		// part of the message.
		msg = strings.TrimPrefix(msg, `"`)
		if end := strings.Index(msg, `"`); end >= 0 {
			msg = msg[:end]
		}
		warnings = append(warnings, msg)
	}
	return warnings
}

// TestListingFailureIsReportedAtTheLevelOfWhatItCosts verifies that an apply
// which cannot read the interface list states that cause once, and states it at
// the level of what the failure costs.
//
// VALIDATES: the operator learns why every ethernet entry was skipped, and
// learns it only when it is news.
// PREVENTS: the measured defect. The listing failure was logged at DEBUG, and
// the per-entry lines that did survive named a cause that was not the cause:
// they said the device was not present, and a device the backend could not list
// can be. A vpp backend still handshaking is the one case where the deferral is
// the designed path, so it stays at DEBUG and is pinned here as well -- a fix
// that warns on every vpp boot trades one wrong line for a louder one.
func TestListingFailureIsReportedAtTheLevelOfWhatItCosts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		listErr  error
		warnings []string
	}{
		{
			name:     "a backend that cannot answer at all",
			listErr:  errors.New("netlink: rtnetlink receive: permission denied"),
			warnings: []string{"iface config: interface listing unavailable, every ethernet binding deferred"},
		},
		{
			name:    "a vpp backend still handshaking",
			listErr: fmt.Errorf("ifacevpp: VPP connector not ready: %w", ErrBackendNotReady),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := selectorBackend()
			b.listErr = tc.listErr
			cfg := macMatchConfig(unitEntry{Label: "0", Addresses: []string{"10.0.0.1/24"}})

			warnings := warningsFrom(t, func() { applyConfig(cfg, nil, b) })

			assert.Equal(t, tc.warnings, warnings,
				"the listing failure is the only warning owed, and it replaces the per-entry lines that would name the wrong cause")
		})
	}
}

// TestBridgeMemberListSurvivesOneMember verifies that a bridge holds the members
// the config names, whether it names one or several.
//
// VALIDATES: every member of a bridge reaches the apply.
// PREVENTS: the measured defect. The parse asserted `m["member"].([]any)`, and a
// leaf-list carrying ONE value arrives as a bare string. The assertion failed,
// the slice stayed empty, and a bridge configured with a single member enslaved
// nothing at all -- with no error, on the most ordinary bridge an operator
// writes. internal/core/configvalue exists for exactly this, and every sibling
// leaf-list in this file reads through it.
func TestBridgeMemberListSurvivesOneMember(t *testing.T) {
	for _, tc := range []struct {
		name string
		json string
		want []string
	}{
		{
			name: "one member",
			json: `{"interface": {"bridge": {"br0": {"member": "eth0"}}}}`,
			want: []string{"eth0"},
		},
		{
			name: "two members",
			json: `{"interface": {"bridge": {"br0": {"member": ["eth0", "eth1"]}}}}`,
			want: []string{"eth0", "eth1"},
		},
		{
			name: "no member",
			json: `{"interface": {"bridge": {"br0": {}}}}`,
			want: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := mustParseIfaceJSON(t, tc.json)

			require.Len(t, cfg.Bridge, 1)
			assert.Equal(t, tc.want, cfg.Bridge[0].Members)
		})
	}
}

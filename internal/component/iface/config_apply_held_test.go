package iface

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// unprivilegedBackend refuses every link write with EPERM, as the kernel
// refuses an RTM_NEWLINK from a process without CAP_NET_ADMIN, while every
// read still answers. A write the apply owes therefore fails the apply, and a
// write it does not owe must never be issued.
type unprivilegedBackend struct {
	*fakeBackend
}

func refused(op, name string) error {
	return fmt.Errorf("iface: %s %q: %w", op, name, syscall.EPERM)
}

func (b *unprivilegedBackend) CreateBridge(name string) error { return refused("create bridge", name) }
func (b *unprivilegedBackend) BridgeSetSTP(name string, _ bool) error {
	return refused("bridge stp", name)
}
func (b *unprivilegedBackend) SetMTU(name string, _ int) error { return refused("set mtu", name) }
func (b *unprivilegedBackend) SetMACAddress(name, _ string) error {
	return refused("set mac", name)
}
func (b *unprivilegedBackend) SetAdminUp(name string) error   { return refused("set up", name) }
func (b *unprivilegedBackend) SetAdminDown(name string) error { return refused("set down", name) }
func (b *unprivilegedBackend) BridgeAddPort(bridge, _ string) error {
	return refused("bridge add port", bridge)
}
func (b *unprivilegedBackend) BridgeDelPort(port string) error {
	return refused("bridge del port", port)
}
func (b *unprivilegedBackend) UpdateVLANQoSMap(name string, _, _ map[uint32]uint32) error {
	return refused("update vlan qos", name)
}

// hostBridgeIndex is the kernel index of the seeded host bridge.
const hostBridgeIndex = 5

// hostBridgeBackend is a host that already holds a docker0 bridge with STP
// off, MTU 1500, its MAC, administratively up with no carrier, and an
// ethernet port eth1 already enslaved to it and up.
func hostBridgeBackend() *fakeBackend {
	return &fakeBackend{ifaces: map[string]fakeIface{
		"docker0": {name: "docker0", linkType: TypeBridge, index: hostBridgeIndex, mtu: 1500,
			mac: "02:42:ac:11:00:01", state: LinkStateUp, stp: BridgeSTPOff},
		"eth1": {name: "eth1", linkType: zeTypeEthernet, index: 6, mtu: 1500,
			mac: "02:00:00:00:00:06", masterIndex: hostBridgeIndex, state: LinkStateUp},
	}}
}

// hostBridgeConfig asks for what hostBridgeBackend already holds: no stp leaf
// (off), the same MTU, the same MAC spelled in upper case, and eth1 as a
// member.
func hostBridgeConfig() *ifaceConfig {
	return &ifaceConfig{
		Ethernet: []ifaceEntry{{Name: "eth1", MTU: 1500}},
		Bridge: []bridgeEntry{{
			Name: "docker0", MTU: 1500, MACAddress: "02:42:AC:11:00:01",
			Members: []string{"eth1"},
		}},
	}
}

// TestApplyOverMatchingHostBridgeNeedsNoPrivilege proves the owner's case:
// ze started without CAP_NET_ADMIN over a host bridge whose state already
// matches the config applies cleanly. Method: every write the backend is
// asked for fails with EPERM, so any write the apply issues fails it; the
// bridge holds STP off, the MTU, the MAC, admin up, and its port. That an
// admin-up bridge with no carrier reads as up is the netlink backend's part
// (TestLinkToInfoAdminStateIsNotOperState).
//
// VALIDATES: a setting the kernel already holds gets no write: STP, MTU, MAC,
// admin state, bridge membership.
// PREVENTS: the start aborting on IFLA_BR_STP_STATE (or any other write) for a
// change that changes nothing.
func TestApplyOverMatchingHostBridgeNeedsNoPrivilege(t *testing.T) {
	resetAddressOwners(t)
	errs := applyConfig(hostBridgeConfig(), nil, &unprivilegedBackend{hostBridgeBackend()})
	require.Empty(t, errs, "nothing differs, so nothing needs CAP_NET_ADMIN")
}

// TestApplyWritesOnlyWhatDiffers proves each compare in both directions on
// the privileged fake: a setting the device holds gets no write, and one it
// does not hold gets exactly the write. Method: start from the matching host
// bridge, change one held value per case, apply, and read the write logs.
//
// VALIDATES: STP, MTU, MAC, admin up and membership are written when they
// differ and only then.
// PREVENTS: a compare that skips a needed write (the inverse defect).
func TestApplyWritesOnlyWhatDiffers(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*fakeBackend, *ifaceConfig)
		check  func(*testing.T, *fakeBackend)
	}{
		{"all held", func(*fakeBackend, *ifaceConfig) {}, func(t *testing.T, b *fakeBackend) {
			assert.Empty(t, b.stpSet)
			assert.Empty(t, b.mtuSet)
			assert.Empty(t, b.macSet)
			assert.Empty(t, b.adminSet)
			assert.Empty(t, b.bridgePorts)
		}},
		{"stp leaf on over stp off", func(_ *fakeBackend, c *ifaceConfig) { c.Bridge[0].STP = true },
			func(t *testing.T, b *fakeBackend) { assert.Equal(t, map[string]bool{"docker0": true}, b.stpSet) }},
		{"stp on held, leaf absent", func(b *fakeBackend, _ *ifaceConfig) {
			setFake(b, "docker0", func(f *fakeIface) { f.stp = BridgeSTPOn })
		},
			func(t *testing.T, b *fakeBackend) { assert.Equal(t, map[string]bool{"docker0": false}, b.stpSet) }},
		{"stp not reported", func(b *fakeBackend, _ *ifaceConfig) {
			setFake(b, "docker0", func(f *fakeIface) { f.stp = BridgeSTPUnreported })
		}, func(t *testing.T, b *fakeBackend) { assert.Equal(t, map[string]bool{"docker0": false}, b.stpSet) }},
		{"mtu differs", func(_ *fakeBackend, c *ifaceConfig) { c.Ethernet[0].MTU = 9000 },
			func(t *testing.T, b *fakeBackend) { assert.Equal(t, map[string]int{"eth1": 9000}, b.mtuSet) }},
		{"mac differs", func(_ *fakeBackend, c *ifaceConfig) { c.Bridge[0].MACAddress = "02:42:ac:11:00:02" },
			func(t *testing.T, b *fakeBackend) {
				assert.Equal(t, map[string]string{"docker0": "02:42:ac:11:00:02"}, b.macSet)
			}},
		{"admin down", func(b *fakeBackend, _ *ifaceConfig) {
			setFake(b, "eth1", func(f *fakeIface) { f.state = LinkStateDown })
		},
			func(t *testing.T, b *fakeBackend) { assert.Equal(t, map[string]string{"eth1": "up"}, b.adminSet) }},
		{"admin not reported", func(b *fakeBackend, _ *ifaceConfig) { setFake(b, "eth1", func(f *fakeIface) { f.state = "" }) },
			func(t *testing.T, b *fakeBackend) { assert.Equal(t, map[string]string{"eth1": "up"}, b.adminSet) }},
		{"port not in the bridge", func(b *fakeBackend, _ *ifaceConfig) { setFake(b, "eth1", func(f *fakeIface) { f.masterIndex = 0 }) },
			func(t *testing.T, b *fakeBackend) {
				assert.Equal(t, []bridgePortCall{{op: bridgePortOpAdd, bridge: "docker0", port: "eth1"}}, b.bridgePorts)
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetAddressOwners(t)
			b := hostBridgeBackend()
			cfg := hostBridgeConfig()
			tt.mutate(b, cfg)
			require.Empty(t, applyConfig(cfg, nil, b))
			tt.check(t, b)
		})
	}
}

// setFake edits one seeded device in place.
func setFake(b *fakeBackend, name string, edit func(*fakeIface)) {
	f := b.ifaces[name]
	edit(&f)
	b.ifaces[name] = f
}

// TestApplyWritesWhatItCannotRead proves an unread state is written, never
// taken for one that matches. Method: eth1 holds the configured MTU and is up,
// but its link read fails; the apply must still set the MTU and bring it up.
//
// VALIDATES: a failed read leads to the write.
// PREVENTS: a read failure read as "already matches".
func TestApplyWritesWhatItCannotRead(t *testing.T) {
	resetAddressOwners(t)
	b := hostBridgeBackend()
	b.getLinkErr = map[string]error{"eth1": errors.New("link read failed")}
	cfg := &ifaceConfig{Ethernet: []ifaceEntry{{Name: "eth1", MTU: 1500}}}

	require.Empty(t, applyConfig(cfg, nil, b))
	assert.Equal(t, 1500, b.mtuSet["eth1"], "an unread MTU is written")
	assert.Equal(t, "up", b.adminSet["eth1"], "an unread admin state is written")
}

// stateUnreadWarnings answers the active report-bus warnings a failed state
// read raised.
func stateUnreadWarnings() []report.Issue {
	var found []report.Issue
	for _, issue := range report.Warnings() {
		if issue.Source != reportSourceIface {
			continue
		}
		if issue.Code != diagnostic.CodeDoctorIfaceStateUnread {
			continue
		}
		found = append(found, issue)
	}
	return found
}

// TestApplyReportsAStateItCannotRead proves a failed state read reaches show
// doctor under its own code, with the command that reads the state again, and
// that a later read which answers clears it. Method: eth1's link read fails
// through the whole apply; then the read answers and the apply runs again.
//
// VALIDATES: one warning under doctor-iface-state-unread, subject eth1,
// carrying the backend's answer and `ze show interface name eth1 detail`;
// the code is a registered event code; a read that answers clears it.
// PREVENTS: a write made blind that only a log line records, and a warning
// that outlives the fault.
func TestApplyReportsAStateItCannotRead(t *testing.T) {
	resetAddressOwners(t)
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	require.True(t, slices.Contains(diagnostic.EventCodes(), diagnostic.CodeDoctorIfaceStateUnread),
		"the recent-events check declares only a registered event code")

	b := hostBridgeBackend()
	b.getLinkErr = map[string]error{"eth1": errors.New("link read failed")}
	cfg := &ifaceConfig{Ethernet: []ifaceEntry{{Name: "eth1", MTU: 1500}}}
	require.Empty(t, applyConfig(cfg, nil, b))

	warnings := stateUnreadWarnings()
	require.Len(t, warnings, 1, "one warning per device, however many steps read it")
	assert.Equal(t, "eth1", warnings[0].Subject)
	assert.Equal(t, report.SeverityWarning, warnings[0].Severity)
	assert.Equal(t, "ze show interface name eth1 detail", warnings[0].Fix)
	assert.Contains(t, warnings[0].Message, "interface eth1")
	assert.Contains(t, warnings[0].Message, "link read failed", "the backend's answer is the cause")

	b.getLinkErr = nil
	require.Empty(t, applyConfig(cfg, nil, b))
	assert.Empty(t, stateUnreadWarnings(), "a read that answers clears the warning")
}

// TestStateReadAnswersTheWriteReportsRaiseNothing proves the two failed reads
// whose write reports them itself raise no warning. Method: eth1's link read
// answers "backend not ready", then "not found", each wrapped as a backend
// wraps it.
//
// VALIDATES: neither answer raises doctor-iface-state-unread.
// PREVENTS: a warning on every vpp start, or for a vpp loopback read by its
// configured name.
func TestStateReadAnswersTheWriteReportsRaiseNothing(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	for _, readErr := range []error{
		fmt.Errorf("iface: get link %q: %w", "eth1", ErrBackendNotReady),
		fmt.Errorf("iface: get link %q: %w", "eth1", ErrInterfaceNotFound),
	} {
		b := hostBridgeBackend()
		b.getLinkErr = map[string]error{"eth1": readErr}
		assert.Nil(t, linkFacts(b, "eth1"), "a failed read answers no facts")
		assert.Empty(t, stateUnreadWarnings(), "%v raises no warning", readErr)
	}
}

// TestApplyNeededWriteStillRefusedWithoutPrivilege proves a real change keeps
// failing an unprivileged apply with the capability named. Method: the
// unprivileged backend over the matching host bridge, with the stp leaf on.
//
// VALIDATES: the STP write is issued, its EPERM fails the apply, and the
// error is one joinApplyErrors gives the CAP_NET_ADMIN remedy.
// PREVENTS: the compare hiding a change ze could not make.
func TestApplyNeededWriteStillRefusedWithoutPrivilege(t *testing.T) {
	resetAddressOwners(t)
	cfg := hostBridgeConfig()
	cfg.Bridge[0].STP = true

	errs := applyConfig(cfg, nil, &unprivilegedBackend{hostBridgeBackend()})
	require.NotEmpty(t, errs, "turning STP on is a change, and it needs CAP_NET_ADMIN")
	require.True(t, errors.Is(errors.Join(errs...), os.ErrPermission))
	require.ErrorContains(t, joinApplyErrors("interface apply", errs), "CAP_NET_ADMIN")
}

// TestRollbackLeavesSkippedStepsAlone proves a step that was skipped is not
// undone with a write. Method: the matching host bridge, plus an address on
// eth1 that fails so the apply rolls back. Before the compare, the STP undo
// wrote the opposite of the configured state (STP on), and the membership
// undo took eth1 out of docker0.
//
// VALIDATES: a rollback issues no write for a setting the apply did not change.
// PREVENTS: a failed commit turning STP on or breaking up a host bridge.
func TestRollbackLeavesSkippedStepsAlone(t *testing.T) {
	resetAddressOwners(t)
	b := hostBridgeBackend()
	b.addAddressErr = map[string]error{addressErrKey("eth1", "10.0.0.1/24"): errors.New("address refused")}
	cfg := hostBridgeConfig()
	cfg.Ethernet[0].Units = []unitEntry{{Label: "0", Addresses: []string{"10.0.0.1/24"}}}

	require.NotEmpty(t, applyConfig(cfg, nil, b), "the refused address fails the apply")
	assert.Empty(t, b.stpSet, "STP was not changed, so it is not changed back")
	assert.Empty(t, b.mtuSet)
	assert.Empty(t, b.macSet)
	assert.Empty(t, b.adminSet, "nothing was brought up, so nothing is brought down")
	assert.Empty(t, b.bridgePorts, "eth1 was in docker0 before, and stays")
}

// TestApplyVLANQoSComparesTheKernel proves an existing VLAN's QoS maps are
// written only when the VLAN holds other maps, on the boot apply too, where
// no previous config exists. Method: a VLAN that already holds the
// configured maps (an entry at 0 spelled in the config agrees with an absent
// one), then one that holds others.
//
// VALIDATES: matching maps get no UpdateVLANQoSMap; differing maps get one.
// PREVENTS: every boot over an existing VLAN rewriting its maps.
func TestApplyVLANQoSComparesTheKernel(t *testing.T) {
	for _, tc := range []struct {
		name      string
		installed map[uint32]uint32
		wantCalls int
	}{
		{"held", map[uint32]uint32{1: 2}, 0},
		{"differs", map[uint32]uint32{1: 3}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetAddressOwners(t)
			b := &fakeBackend{ifaces: map[string]fakeIface{
				"eth0":     {name: "eth0", linkType: zeTypeEthernet, index: 2, state: LinkStateUp},
				"eth0.100": {name: "eth0.100", linkType: "vlan", index: 3, parentIndex: 2, state: LinkStateUp},
			}, vlans: map[string]VLANSpec{"eth0.100": {Parent: "eth0", VLANID: 100, IngressQoSMap: tc.installed}}}
			cfg := &ifaceConfig{Ethernet: []ifaceEntry{{Name: "eth0", Units: []unitEntry{{
				Label: "v100", VLANID: 100, IngressQoSMap: map[uint32]uint32{1: 2, 4: 0},
			}}}}}

			require.Empty(t, applyConfig(cfg, nil, b))
			assert.Len(t, b.qosUpdates, tc.wantCalls)
		})
	}
}

// TestSameMAC proves the MAC compare reads addresses, not spellings, and
// that an unread ("") or malformed address matches nothing.
func TestSameMAC(t *testing.T) {
	assert.True(t, sameMAC("02:42:ac:11:00:01", "02:42:AC:11:00:01"))
	assert.True(t, sameMAC("02-42-ac-11-00-01", "02:42:ac:11:00:01"))
	assert.False(t, sameMAC("02:42:ac:11:00:01", "02:42:ac:11:00:02"))
	assert.False(t, sameMAC("", "02:42:ac:11:00:01"), "an unread MAC is written")
	assert.False(t, sameMAC("not-a-mac", "not-a-mac"))
}

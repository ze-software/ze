package iface

import (
	"errors"
	"testing"
)

// mirrorCfg builds a config with one dummy interface, "mir0", carrying the
// given mirror destinations on its untagged unit.
func mirrorCfg(ingressDst, egressDst string) *ifaceConfig {
	return &ifaceConfig{
		Backend: "netlink",
		Dummy: []ifaceEntry{{
			Name:  "mir0",
			Units: []unitEntry{{MirrorIngress: ingressDst, MirrorEgress: egressDst}},
		}},
	}
}

// mirrorOps renders the backend's recorded mirror calls as one string per
// call, so a test can assert on the whole sequence.
func mirrorOps(b *fakeBackend) []string {
	ops := make([]string, 0, len(b.mirrorCalls))
	for _, c := range b.mirrorCalls {
		if c.op == mirrorOpRemove {
			ops = append(ops, "remove "+c.iface)
			continue
		}
		dirs := ""
		if c.ingress {
			dirs += "i"
		}
		if c.egress {
			dirs += "e"
		}
		ops = append(ops, "setup "+c.iface+" -> "+c.dst+" ("+dirs+")")
	}
	return ops
}

func requireOps(t *testing.T, b *fakeBackend, want []string) {
	t.Helper()
	got := mirrorOps(b)
	if len(got) != len(want) {
		t.Fatalf("mirror calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mirror calls = %v, want %v", got, want)
		}
	}
}

func TestApplyConfigRemovesMirrorDroppedFromConfig(t *testing.T) {
	// VALIDATES: AC-1 -- a config that no longer asks for a mirror tears the
	// installed one down.
	// PREVENTS: the kernel duplicating traffic on a config that stopped asking
	// for it, which is what applyMirror's empty-destination early return left
	// behind.
	b := &fakeBackend{}
	previous := mirrorCfg("cap0", "")
	if errs := applyConfig(previous, nil, b); len(errs) > 0 {
		t.Fatalf("apply previous config: %v", errs)
	}
	b.mirrorCalls = nil

	current := &ifaceConfig{
		Backend: "netlink",
		Dummy:   []ifaceEntry{{Name: "mir0", Units: []unitEntry{{}}}},
	}
	if errs := applyConfig(current, previous, b); len(errs) > 0 {
		t.Fatalf("apply current config: %v", errs)
	}

	requireOps(t, b, []string{"remove mir0"})
}

func TestApplyConfigRetiresChangedMirrorBeforeSetup(t *testing.T) {
	// VALIDATES: AC-6 -- a changed destination is retired before the new one
	// is installed.
	// PREVENTS: both destinations receiving mirrored traffic, because tc
	// filters are additive and a second SetupMirror never retires the first.
	b := &fakeBackend{}
	previous := mirrorCfg("cap0", "")
	if errs := applyConfig(previous, nil, b); len(errs) > 0 {
		t.Fatalf("apply previous config: %v", errs)
	}
	b.mirrorCalls = nil

	current := mirrorCfg("cap1", "")
	if errs := applyConfig(current, previous, b); len(errs) > 0 {
		t.Fatalf("apply current config: %v", errs)
	}

	requireOps(t, b, []string{"remove mir0", "setup mir0 -> cap1 (i)"})
}

func TestApplyConfigRetiresMirrorDirectionDropped(t *testing.T) {
	// VALIDATES: AC-6 -- dropping one direction of a two-direction mirror
	// retires the mirror before the remaining direction is re-installed.
	// PREVENTS: the egress filter surviving a config that only asks for
	// ingress, because applyMirror installs and never removes.
	b := &fakeBackend{}
	previous := mirrorCfg("cap0", "cap0")
	if errs := applyConfig(previous, nil, b); len(errs) > 0 {
		t.Fatalf("apply previous config: %v", errs)
	}
	b.mirrorCalls = nil

	current := mirrorCfg("cap0", "")
	if errs := applyConfig(current, previous, b); len(errs) > 0 {
		t.Fatalf("apply current config: %v", errs)
	}

	requireOps(t, b, []string{"remove mir0", "setup mir0 -> cap0 (i)"})
}

func TestApplyConfigKeepsUnchangedMirror(t *testing.T) {
	// VALIDATES: AC-7 -- an unchanged mirror is not TORN DOWN on re-apply, so
	// the apply carries one setup and no teardown.
	// PREVENTS: removeStaleMirrors treating an unchanged spec as a changed one,
	// which would retire the mirror and re-install it on every commit.
	//
	// It does NOT prove mirrored traffic is uninterrupted, and it must not be
	// read that way. The setup below re-runs on every commit by design: config
	// apply converges kernel state onto the config, so a filter an operator
	// deleted by hand comes back. addMirrorFilter
	// (internal/plugins/iface/netlink/mirror_linux.go) reaches its EEXIST branch
	// on that re-run and does FilterDel then FilterAdd, because cls_matchall
	// refuses a replace, so packets are unmirrored for that window. Self-healing
	// is worth it; the window is real and belongs in the record.
	b := &fakeBackend{}
	previous := mirrorCfg("cap0", "cap0")
	if errs := applyConfig(previous, nil, b); len(errs) > 0 {
		t.Fatalf("apply previous config: %v", errs)
	}
	b.mirrorCalls = nil

	current := mirrorCfg("cap0", "cap0")
	if errs := applyConfig(current, previous, b); len(errs) > 0 {
		t.Fatalf("apply current config: %v", errs)
	}

	requireOps(t, b, []string{"setup mir0 -> cap0 (ie)"})
}

func TestApplyConfigRemovesMirrorOnVLANUnit(t *testing.T) {
	// VALIDATES: AC-1 on a VLAN unit, whose kernel device is <parent>.<vlan>.
	// PREVENTS: teardown addressing the parent interface and leaving the VLAN
	// device mirroring.
	b := &fakeBackend{}
	previous := &ifaceConfig{
		Backend: "netlink",
		Dummy: []ifaceEntry{{
			Name:  "mir0",
			Units: []unitEntry{{VLANID: 42, MirrorIngress: "cap0"}},
		}},
	}
	if errs := applyConfig(previous, nil, b); len(errs) > 0 {
		t.Fatalf("apply previous config: %v", errs)
	}
	b.mirrorCalls = nil

	current := &ifaceConfig{
		Backend: "netlink",
		Dummy: []ifaceEntry{{
			Name:  "mir0",
			Units: []unitEntry{{VLANID: 42}},
		}},
	}
	if errs := applyConfig(current, previous, b); len(errs) > 0 {
		t.Fatalf("apply current config: %v", errs)
	}

	requireOps(t, b, []string{"remove mir0.42"})
}

func TestApplyConfigRemovesMirrorWhenUnitIsDisabled(t *testing.T) {
	// VALIDATES: AC-1 -- disabling the unit is a way of asking for no mirror.
	// PREVENTS: a disabled unit keeping its mirror, since applyMirror is never
	// reached for a disabled unit.
	b := &fakeBackend{}
	previous := mirrorCfg("cap0", "")
	if errs := applyConfig(previous, nil, b); len(errs) > 0 {
		t.Fatalf("apply previous config: %v", errs)
	}
	b.mirrorCalls = nil

	current := &ifaceConfig{
		Backend: "netlink",
		Dummy: []ifaceEntry{{
			Name:  "mir0",
			Units: []unitEntry{{Disable: true, MirrorIngress: "cap0"}},
		}},
	}
	if errs := applyConfig(current, previous, b); len(errs) > 0 {
		t.Fatalf("apply current config: %v", errs)
	}

	requireOps(t, b, []string{"remove mir0"})
}

func TestApplyConfigSkipsMirrorTeardownForAbsentInterface(t *testing.T) {
	// VALIDATES: a mirror on a device that no longer exists needs no teardown.
	// PREVENTS: a commit failing because the backend cannot find an interface
	// the operator removed along with its mirror.
	b := &fakeBackend{}
	previous := &ifaceConfig{
		Backend: "netlink",
		Ethernet: []ifaceEntry{{
			Name:  "eth9",
			Units: []unitEntry{{MirrorIngress: "cap0"}},
		}},
	}
	current := &ifaceConfig{Backend: "netlink"}

	if errs := applyConfig(current, previous, b); len(errs) > 0 {
		t.Fatalf("apply current config: %v", errs)
	}
	requireOps(t, b, nil)
}

func TestApplyConfigReportsMirrorTeardownFailure(t *testing.T) {
	// VALIDATES: a teardown that fails is reported, never swallowed.
	// PREVENTS: a commit reporting success while the kernel still duplicates
	// traffic the config no longer asks for.
	b := &fakeBackend{}
	previous := mirrorCfg("cap0", "")
	if errs := applyConfig(previous, nil, b); len(errs) > 0 {
		t.Fatalf("apply previous config: %v", errs)
	}
	b.mirrorCalls = nil
	b.removeMirrorErr = map[string]error{"mir0": errors.New("netlink refused")}

	current := &ifaceConfig{
		Backend: "netlink",
		Dummy:   []ifaceEntry{{Name: "mir0", Units: []unitEntry{{}}}},
	}
	errs := applyConfig(current, previous, b)
	if len(errs) == 0 {
		t.Fatal("applyConfig reported success while the mirror teardown failed")
	}
}

func TestApplyConfigRetiresAMirrorNoConfigAsksFor(t *testing.T) {
	// VALIDATES: a mirror the dataplane carries and the configuration does not
	// ask for is torn down, with no previous config to derive it from.
	// PREVENTS: the restart hole. The operator deletes the mirror while ze is
	// down, then the next boot applies with a nil previous config. That leaves
	// removeStaleMirrors nothing to compare, so the tc filter keeps copying
	// every packet to a destination the operator deleted.
	b := &fakeBackend{}
	b.seedMirror("mir0", "cap0", "")

	current := &ifaceConfig{
		Backend: "netlink",
		Dummy:   []ifaceEntry{{Name: "mir0", Units: []unitEntry{{}}}},
	}
	if errs := applyConfig(current, nil, b); len(errs) > 0 {
		t.Fatalf("apply current config: %v", errs)
	}

	requireOps(t, b, []string{"remove mir0"})
}

func TestApplyConfigRetiresAMirrorAnEarlierTeardownSkipped(t *testing.T) {
	// VALIDATES: a live mirror the configuration does not ask for is retired
	// even when the delta between this config and the previous one is EMPTY.
	// PREVENTS: the skipped-teardown hole -- removeStaleMirrors skips a mirror
	// whose interface it cannot read, the apply that skipped it consumes the
	// delta, and every later commit compares two configs that agree.
	b := &fakeBackend{}
	b.seedMirror("mir0", "cap0", "")

	settled := &ifaceConfig{
		Backend: "netlink",
		Dummy:   []ifaceEntry{{Name: "mir0", Units: []unitEntry{{}}}},
	}
	if errs := applyConfig(settled, settled, b); len(errs) > 0 {
		t.Fatalf("apply settled config: %v", errs)
	}

	requireOps(t, b, []string{"remove mir0"})
}

func TestApplyConfigRestoresAMirrorDirectionTheConfigDropped(t *testing.T) {
	// VALIDATES: a live mirror that differs from the configured one is retired
	// and re-installed, rather than overwritten.
	// PREVENTS: a dropped direction surviving -- tc filters are additive per
	// hook, so installing ingress leaves an egress filter from a config nobody
	// asks for any more copying traffic.
	b := &fakeBackend{}
	b.seedMirror("mir0", "cap0", "cap0")

	settled := mirrorCfg("cap0", "")
	if errs := applyConfig(settled, settled, b); len(errs) > 0 {
		t.Fatalf("apply settled config: %v", errs)
	}

	requireOps(t, b, []string{"setup mir0 -> cap0 (i)", "remove mir0", "setup mir0 -> cap0 (i)"})
}

func TestApplyConfigRemovesNoMirrorWhenTheDataplaneCannotBeRead(t *testing.T) {
	// VALIDATES: a backend that cannot report its live mirrors leaves every
	// mirror alone.
	// PREVENTS: reading "I cannot tell" as "there is nothing there". An empty
	// answer from a failed read makes the pass believe the dataplane carries
	// nothing. The mirror the operator DOES ask for is then reinstalled on every
	// apply. A mirror the failed read never reported is never retired. Returning
	// the error and acting on nothing is the only answer that claims neither.
	b := &fakeBackend{}
	b.seedMirror("mir0", "cap0", "")
	b.listMirrorsErr = errors.New("netlink refused the filter dump")

	current := &ifaceConfig{
		Backend: "netlink",
		Dummy:   []ifaceEntry{{Name: "mir0", Units: []unitEntry{{}}}},
	}
	if errs := applyConfig(current, nil, b); len(errs) > 0 {
		t.Fatalf("apply current config: %v", errs)
	}

	requireOps(t, b, nil)
	if _, live := b.mirrors["mir0"]; !live {
		t.Fatal("an unreadable dataplane must leave the installed mirror alone")
	}
}

func TestApplyConfigLeavesAMirrorOnAnInterfaceItDoesNotConfigure(t *testing.T) {
	// VALIDATES: the reconcile acts only on the interfaces the configuration
	// names, and leaves a mirror on every other interface alone.
	// PREVENTS: the pass reading a priority-1 matchall mirred filter as a mark
	// of ownership. It is a SHAPE, and another tool can install the same one.
	// Removing every match would then stop an operator's own capture on an
	// interface ze does not manage, on every apply.
	b := &fakeBackend{}
	b.seedMirror("mir0", "cap0", "")
	b.seedMirror("other0", "tap0", "")

	current := &ifaceConfig{
		Backend: "netlink",
		Dummy:   []ifaceEntry{{Name: "mir0", Units: []unitEntry{{}}}},
	}
	if errs := applyConfig(current, nil, b); len(errs) > 0 {
		t.Fatalf("apply current config: %v", errs)
	}

	requireOps(t, b, []string{"remove mir0"})
	if _, live := b.mirrors["other0"]; !live {
		t.Fatal("the reconcile removed a mirror on an interface the configuration does not name")
	}
}

func TestIndexMirrorSpecsSkipsDisabledEntries(t *testing.T) {
	// VALIDATES: a disabled interface asks for no mirror, so the delta pass
	// retires the one it had.
	// PREVENTS: a disabled interface keeping a mirror the apply loop skips.
	cfg := &ifaceConfig{
		Dummy: []ifaceEntry{
			{Name: "off0", Disable: true, Units: []unitEntry{{MirrorIngress: "cap0"}}},
			{Name: "on0", Units: []unitEntry{{MirrorEgress: "cap1"}}},
			{Name: "bare0", Units: []unitEntry{{}}},
		},
	}
	specs := indexMirrorSpecs(cfg, nil)
	if _, ok := specs["off0"]; ok {
		t.Error("a disabled interface must not be a desired mirror")
	}
	if _, ok := specs["bare0"]; ok {
		t.Error("an interface with no mirror leaves must not be a desired mirror")
	}
	want := mirrorSpec{egress: "cap1"}
	if got := specs["on0"]; got != want {
		t.Errorf("specs[on0] = %+v, want %+v", got, want)
	}
}

func TestIndexMirrorSpecsCoversEveryInterfaceFamily(t *testing.T) {
	// VALIDATES: the delta pass sees a mirror on any interface kind the apply
	// loop can install one on.
	// PREVENTS: a mirror on a bridge, tunnel, veth, wireguard or xfrm device
	// surviving its own deletion because the walk missed that family.
	cfg := &ifaceConfig{
		Ethernet:  []ifaceEntry{{Name: "eth0", Units: []unitEntry{{MirrorIngress: "cap0"}}}},
		Dummy:     []ifaceEntry{{Name: "dum0", Units: []unitEntry{{MirrorIngress: "cap0"}}}},
		Veth:      []vethEntry{{Name: "veth0", Units: []unitEntry{{MirrorIngress: "cap0"}}}},
		Bridge:    []bridgeEntry{{Name: "br0", Units: []unitEntry{{MirrorIngress: "cap0"}}}},
		Tunnel:    []tunnelEntry{{Name: "tun0", Units: []unitEntry{{MirrorIngress: "cap0"}}}},
		Wireguard: []wireguardEntry{{Name: "wg0", Units: []unitEntry{{MirrorIngress: "cap0"}}}},
		XFRM:      []xfrmEntry{{Name: "xfrm0", Units: []unitEntry{{MirrorIngress: "cap0"}}}},
	}
	specs := indexMirrorSpecs(cfg, nil)
	for _, name := range []string{"eth0", "dum0", "veth0", "br0", "tun0", "wg0", "xfrm0"} {
		if _, ok := specs[name]; !ok {
			t.Errorf("mirror on %q not seen by indexMirrorSpecs", name)
		}
	}
}

// TestMirrorDestinationFollowsItsSelector verifies that the capture port a
// mirror points at is resolved through the same selector map the mirror SOURCE
// already used.
//
// VALIDATES: a mirror destination selected by MAC or aliased by os-name
// receives the copy on its kernel device.
// PREVENTS: the measured defect. setupMirrorSpec took the source from the
// resolved device and handed SetupMirror the destination straight out of the
// config, so a mirror toward a selected capture port installed the tc filter
// toward whatever device carried the logical name -- a traffic copy leaving on
// the wrong port, which is worse than no copy at all.
func TestMirrorDestinationFollowsItsSelector(t *testing.T) {
	cfg := &ifaceConfig{
		Backend: "netlink",
		Ethernet: []ifaceEntry{
			{Name: "wan", MatchMAC: selectedPermMAC, Units: []unitEntry{{Label: "0", MirrorIngress: "capture"}}},
			{Name: "capture", OSName: "enp9s0"},
		},
	}
	b := selectorBackend()
	b.ifaces["enp9s0"] = fakeIface{name: "enp9s0", linkType: "device", index: 9}

	errs := applyConfig(cfg, nil, b)

	if len(errs) > 0 {
		t.Fatalf("applyConfig: %v", errs)
	}
	requireOps(t, b, []string{"setup enp1s0 -> enp9s0 (i)"})
}

// TestMirrorDefersWhenItsDestinationIsAbsent verifies that a destination whose
// selector has no answer yet installs nothing, rather than installing toward the
// logical name.
//
// VALIDATES: an unbound mirror destination defers, as an unbound source does.
// PREVENTS: a tc filter mirroring to the device that merely shares the capture
// port's entry name.
func TestMirrorDefersWhenItsDestinationIsAbsent(t *testing.T) {
	cfg := &ifaceConfig{
		Backend: "netlink",
		Ethernet: []ifaceEntry{
			{Name: "wan", MatchMAC: selectedPermMAC, Units: []unitEntry{{Label: "0", MirrorIngress: "capture"}}},
			{Name: "capture", OSName: "enp9s0"}, // enp9s0 has not appeared
		},
	}
	b := selectorBackend()

	errs := applyConfig(cfg, nil, b)

	if len(errs) > 0 {
		t.Fatalf("applyConfig: %v", errs)
	}
	requireOps(t, b, nil)
}

// TestReconcileRetiresTheMirrorOnADisabledInterfacesDevice verifies two things
// about a disabled entry. Its mirror is retired on the device its selector
// names. No mirror is touched on a kernel device that merely shares the entry's
// logical name.
//
// VALIDATES: mirrorScope's own contract -- "A disabled entry or unit is still
// ze's, so it is in scope and its mirror is retired rather than left."
// PREVENTS: the measured defect. bindDevices skipped every disabled entry. The
// entry was therefore absent from the selector map, and deviceFor fell through
// to `return name, true`. For `ethernet uplink { os-name eth3; disable; }` the
// scope then held "uplink" rather than "eth3". That broke the contract in both
// directions at once. The real mirror on eth3 kept copying traffic the config
// no longer asks for. A kernel device literally called `uplink` was claimed,
// and ze configures nothing on it, so the reconcile would remove an operator's
// own capture there.
func TestReconcileRetiresTheMirrorOnADisabledInterfacesDevice(t *testing.T) {
	b := selectorBackend()
	b.ifaces["eth3"] = fakeIface{name: "eth3", linkType: "device", index: 8}
	b.ifaces["uplink"] = fakeIface{name: "uplink", linkType: "device", index: 9}
	b.seedMirror("eth3", "cap0", "")
	b.seedMirror("uplink", "tap0", "")

	cfg := &ifaceConfig{
		Backend:  "netlink",
		Ethernet: []ifaceEntry{{Name: "uplink", OSName: "eth3", Disable: true}},
	}
	if errs := applyConfig(cfg, nil, b); len(errs) > 0 {
		t.Fatalf("applyConfig: %v", errs)
	}

	requireOps(t, b, []string{"remove eth3"})
	if _, live := b.mirrors["uplink"]; !live {
		t.Fatal("the reconcile removed a mirror on the kernel device that merely shares the disabled entry's logical name")
	}
}

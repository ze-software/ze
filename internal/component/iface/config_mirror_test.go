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
	// VALIDATES: AC-7 -- an unchanged mirror is not torn down on re-apply.
	// PREVENTS: every unrelated commit interrupting mirrored traffic.
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
	specs := indexMirrorSpecs(cfg)
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
		Veth:      []vethEntry{{ifaceEntry: ifaceEntry{Name: "veth0", Units: []unitEntry{{MirrorIngress: "cap0"}}}}},
		Bridge:    []bridgeEntry{{ifaceEntry: ifaceEntry{Name: "br0", Units: []unitEntry{{MirrorIngress: "cap0"}}}}},
		Tunnel:    []tunnelEntry{{ifaceEntry: ifaceEntry{Name: "tun0", Units: []unitEntry{{MirrorIngress: "cap0"}}}}},
		Wireguard: []wireguardEntry{{ifaceEntry: ifaceEntry{Name: "wg0", Units: []unitEntry{{MirrorIngress: "cap0"}}}}},
		XFRM:      []xfrmEntry{{ifaceEntry: ifaceEntry{Name: "xfrm0", Units: []unitEntry{{MirrorIngress: "cap0"}}}}},
	}
	specs := indexMirrorSpecs(cfg)
	for _, name := range []string{"eth0", "dum0", "veth0", "br0", "tun0", "wg0", "xfrm0"} {
		if _, ok := specs[name]; !ok {
			t.Errorf("mirror on %q not seen by indexMirrorSpecs", name)
		}
	}
}

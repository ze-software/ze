// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- VRRPv2 groups
// Related: groups.go -- validateGroup, EffectivePriority
// Related: instance.go -- doInstallVIPs, execute
//
// VALIDATES: a group configured with version 2 gets the RFC 3768 owner
// priority, the RFC 3768 Backup priority range, a Backup that holds no virtual
// address, and the RFC 3768 Shutdown sequence, each through the same entry
// point a VRRPv3 group uses.
// PREVENTS: a VRRPv2-only regression hiding behind tests that build VRRPv3
// groups (testSpec and a group with no version leaf are both VRRPv3).
package vrrp

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
)

// v2GroupSpec extracts one IPv4 group configured with version 2 and fails the
// test unless the extraction really produced a VRRPv2 group.
func v2GroupSpec(t *testing.T, group map[string]any, realAddrs ...string) GroupSpec {
	t.Helper()
	group["version"] = "2"
	specs, err := extractGroupSpecs([]configSection{mkSection(t, oneGroup(familyIPv4, "5", group, realAddrs...))})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("extracted %d groups, want 1", len(specs))
	}
	if specs[0].Version != versionV2 {
		t.Fatalf("group version = %d, want %d: the version leaf did not reach the spec", specs[0].Version, versionV2)
	}
	return specs[0]
}

// TestV2OwnerRunsAtPriority255 proves the VRRPv2 address owner runs at 255.
//
// Method: a version 2 group whose virtual address is a real address of the
// unit is configured at priority 120. Its effective priority is 255 under any
// tracked decrement, and the running instance advertises 255. A version 2 group
// whose virtual address is not a unit address keeps its configured 120.
//
// RFC requirement: RFC3768-5.3.4-1 positive -- a VRRPv2 group that owns its virtual address, configured at priority 120, runs with EffectivePriority 255 under any tracked decrement, and its instance advertises Priority 255 on startup (EffectivePriority groups.go, execute instance.go)
// RFC requirement: RFC3768-5.3.4-1 negative -- contrast: a VRRPv2 group that does not own its virtual address keeps its configured priority 120, so 255 marks only the address owner (EffectivePriority groups.go).
func TestV2OwnerRunsAtPriority255(t *testing.T) {
	owner := v2GroupSpec(t, map[string]any{"virtual-address": vips("192.0.2.10"), "priority": float64(120)}, "192.0.2.10/24")
	if !owner.IsOwner {
		t.Fatalf("a virtual address equal to a unit address must mark the owner: %+v", owner)
	}
	if err := validateGroups([]GroupSpec{owner}, backendNetlink); err != nil {
		t.Fatalf("the v2 owner group must validate: %v", err)
	}
	for _, decrement := range []uint16{0, 1, 119, 4064} {
		if got := owner.EffectivePriority(decrement); got != ownerPriority {
			t.Errorf("v2 owner EffectivePriority(%d) = %d, want 255", decrement, got)
		}
	}
	in, f, _ := newTestInstance(t, owner)
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	got := f.snapshot()
	if len(got.adverts) == 0 {
		t.Fatal("the v2 owner sent no advertisement on startup")
	}
	if got.adverts[0].priority != ownerPriority {
		t.Errorf("v2 owner advertised priority %d, want 255", got.adverts[0].priority)
	}

	backup := v2GroupSpec(t, map[string]any{"virtual-address": vips("192.0.2.99"), "priority": float64(120)}, "192.0.2.10/24")
	if backup.IsOwner {
		t.Fatalf("a virtual address that is no unit address must not mark the owner: %+v", backup)
	}
	if got := backup.EffectivePriority(0); got != 120 {
		t.Errorf("v2 non-owner EffectivePriority(0) = %d, want the configured 120", got)
	}
}

// TestV2BoundaryPriority proves a VRRPv2 router backing up a virtual router
// runs with a priority in 1..254.
//
// Method: version 2 groups are validated at priorities 0, 1, 254 and 255, and
// the two outside values must be refused by the range check itself. A version
// 2 non-owner then takes tracked decrements at and past its priority, which
// must floor at 1.
//
// RFC requirement: RFC3768-5.3.4-2 positive -- a VRRPv2 group configured at priority 1 or 254 validates, and a tracked decrement at or past the configured priority floors its run-time priority at 1, so a backing-up v2 router runs inside 1..254 (validateGroup, EffectivePriority groups.go)
// RFC requirement: RFC3768-5.3.4-2 negative -- a VRRPv2 group configured at priority 0 or 255 is refused by the priority range check, so a backing-up v2 router can never be configured outside 1..254 (validateGroup groups.go).
func TestV2BoundaryPriority(t *testing.T) {
	cases := []struct {
		name     string
		priority float64
		wantErr  bool
	}{
		{name: "0", priority: 0, wantErr: true},
		{name: "1", priority: 1, wantErr: false},
		{name: "254", priority: 254, wantErr: false},
		{name: "255", priority: 255, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := v2GroupSpec(t, map[string]any{"virtual-address": vips("10.0.0.1"), "priority": tc.priority})
			err := validateGroups([]GroupSpec{spec}, backendNetlink)
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("v2 priority %v must be accepted: %v", tc.priority, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("v2 priority %v must be rejected", tc.priority)
			}
			if !strings.Contains(err.Error(), "is out of range; configure 1..254") {
				t.Fatalf("v2 priority %v rejected for another reason: %v", tc.priority, err)
			}
		})
	}

	spec := v2GroupSpec(t, map[string]any{"virtual-address": vips("10.0.0.1"), "priority": float64(100)})
	for _, decrement := range []uint16{99, 100, 4064} {
		if got := spec.EffectivePriority(decrement); got != backupPriorityMin {
			t.Errorf("v2 backup EffectivePriority(%d) at priority 100 = %d, want 1", decrement, got)
		}
	}
}

// TestV2BackupHoldsNoVirtualAddress proves a VRRPv2 Backup holds nothing the
// kernel could answer ARP for or accept packets on, while a VRRPv2 Master does.
//
// Method: a version 2 non-owner starts and must be Backup with no virtual
// address installed and no acceptance filter handed to the dataplane. A version
// 2 owner starts and must be Master with the address installed on its
// virtual-MAC device.
//
// RFC requirement: RFC3768-6.4.2-1 positive -- a VRRPv2 Backup installs no virtual address, so the kernel answers no ARP request for it (doInstallVIPs runs only on a Master transition, instance.go)
// RFC requirement: RFC3768-6.4.2-1 negative -- contrast: a VRRPv2 Master DOES install the virtual address on its virtual-MAC device, so the Backup ARP silence is state-specific rather than a blanket refusal (instance.go).
// RFC requirement: RFC3768-6.4.2-3 positive -- a VRRPv2 Backup installs no virtual address, so it accepts no packet addressed to it (doInstallVIPs instance.go)
// RFC requirement: RFC3768-6.4.2-3 negative -- contrast: a VRRPv2 Master DOES install the virtual address, so the Backup refusal is state-specific (instance.go).
func TestV2BackupHoldsNoVirtualAddress(t *testing.T) {
	spec := testSpec()
	spec.Version = versionV2
	in, f, _ := newTestInstance(t, spec)
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	if in.machine.State() != fsm.StateBackup {
		t.Fatalf("v2 non-owner state = %v, want Backup", in.machine.State())
	}
	got := f.snapshot()
	if len(got.installs) != 0 {
		t.Errorf("a v2 Backup must install no virtual address, got %+v", got.installs)
	}
	if len(got.filters) != 0 {
		t.Errorf("a v2 Backup must hand the dataplane no acceptance filter, got %+v", got.filters)
	}

	spec.IsOwner = true
	inM, fM, _ := newTestInstance(t, spec)
	inM.dispatch(fsm.Startup{Config: inM.fsmConfig()})
	if inM.machine.State() != fsm.StateMaster {
		t.Fatalf("v2 owner state = %v, want Master", inM.machine.State())
	}
	gotM := fM.snapshot()
	if len(gotM.installs) != 1 {
		t.Fatalf("a v2 Master must install its virtual address once, got %+v", gotM.installs)
	}
	if gotM.installs[0].dev != inM.dev {
		t.Errorf("v2 Master install device = %q, want the virtual-MAC device %q", gotM.installs[0].dev, inM.dev)
	}
}

// TestV2MasterYieldsToAHigherPriority proves the higher-priority branch of the
// RFC 3768 Master's ADVERTISEMENT handling.
//
// Method: a version 2 Master whose advertisements leave from 192.0.2.2
// receives Priority 250, above its own 200, from the lower address 192.0.2.1,
// so only the priority can decide. It must be Backup with the Adver_Timer
// canceled, send nothing over two intervals, still be Backup 1 ms before
// Master_Down_Interval (3 s plus Skew_Time (256-200)/256 s) and Master 1 ms
// after. A second Master receiving Priority 150 from the greater address
// 192.0.2.8 must stay Master: a lower priority never demotes.
//
// RFC requirement: RFC3768-6.4.3-8 positive -- a VRRPv2 Master receiving an ADVERTISEMENT whose Priority is greater than its own, from a lower sender address, cancels the Adver_Timer, sets the Master_Down_Timer to Master_Down_Interval and transitions to Backup (masterAdvert fsm.go, execute instance.go)
// RFC requirement: RFC3768-6.4.3-8 negative -- contrast: a VRRPv2 Master receiving a lower Priority from a greater sender address stays Master, so the demotion is bound to the priority comparison (masterAdvert fsm.go).
func TestV2MasterYieldsToAHigherPriority(t *testing.T) {
	const masterDownInterval = 3*time.Second + 56*time.Second/256

	spec := testSpec()
	spec.Version = versionV2
	spec.VIPs = []netip.Addr{tieBreakVIP}
	in, f, clk := newTestInstance(t, spec)
	f.source = netip.MustParseAddr("192.0.2.2")
	in.evaluateReadiness()
	promoteToActive(t, in, clk)

	deliverAdvert(in, v2AdvertItem(t, 250, netip.MustParseAddr("192.0.2.1"), tieBreakVIP))
	if got := in.machine.State(); got != fsm.StateBackup {
		t.Fatalf("higher priority from a lower sender: state = %v, want Backup", got)
	}
	if in.advert != nil {
		t.Fatal("the Adver_Timer is still armed after the demotion")
	}
	advertsAtDemotion := len(f.snapshot().adverts)
	clk.Add(2 * time.Duration(spec.AdvertIntervalMs) * time.Millisecond)
	drainEvents(in)
	if got := len(f.snapshot().adverts); got != advertsAtDemotion {
		t.Fatalf("%d advertisement(s) left the demoted router over two intervals, want none", got-advertsAtDemotion)
	}
	clk.Add(masterDownInterval - 2*time.Duration(spec.AdvertIntervalMs)*time.Millisecond - time.Millisecond)
	drainEvents(in)
	if got := in.machine.State(); got != fsm.StateBackup {
		t.Fatalf("1 ms before Master_Down_Interval (%v): state = %v, want Backup", masterDownInterval, got)
	}
	clk.Add(2 * time.Millisecond)
	drainEvents(in)
	if got := in.machine.State(); got != fsm.StateMaster {
		t.Fatalf("1 ms after Master_Down_Interval (%v): state = %v, want Master", masterDownInterval, got)
	}

	holder, fH, clkH := newTestInstance(t, spec)
	fH.source = netip.MustParseAddr("192.0.2.2")
	holder.evaluateReadiness()
	promoteToActive(t, holder, clkH)
	deliverAdvert(holder, v2AdvertItem(t, 150, netip.MustParseAddr("192.0.2.8"), tieBreakVIP))
	if got := holder.machine.State(); got != fsm.StateMaster {
		t.Fatalf("lower priority from a greater sender: state = %v, want Master", got)
	}
	if holder.advert == nil {
		t.Fatal("the Adver_Timer was canceled by a lower-priority advertisement")
	}
}

// TestV2MasterShutdown proves the RFC 3768 Shutdown sequence of a VRRPv2
// Master: the Adver_Timer is canceled, a Priority 0 advertisement is sent, and
// the router is in Initialize.
//
// Method: a version 2 owner starts as Master, then receives Shutdown. The last
// advertisement must carry Priority 0, the state must be Initialize, and after
// the clock passes several advertisement intervals no further advertisement or
// timer event may appear. A version 2 Backup that shuts down is the contrast:
// it sends no advertisement at all.
//
// RFC requirement: RFC3768-6.4.3-5 positive -- a VRRPv2 Master receiving Shutdown sends one advertisement with Priority 0, cancels its advertisement timer so nothing is sent or fired after it, and transitions to Initialize (execute instance.go, fsm master/shutdown)
// RFC requirement: RFC3768-6.4.3-5 negative -- contrast: a VRRPv2 Backup receiving Shutdown sends no advertisement, so the Priority 0 advertisement is bound to the Master state (execute instance.go).
func TestV2MasterShutdown(t *testing.T) {
	spec := testSpec()
	spec.Version = versionV2
	spec.IsOwner = true
	in, f, clk := newTestInstance(t, spec)
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	if in.machine.State() != fsm.StateMaster {
		t.Fatalf("v2 owner state = %v, want Master", in.machine.State())
	}
	in.dispatch(fsm.Shutdown{})

	got := f.snapshot()
	if len(got.adverts) < 2 {
		t.Fatalf("adverts = %+v, want the startup advertisement and the resignation", got.adverts)
	}
	if last := got.adverts[len(got.adverts)-1]; last.priority != 0 {
		t.Fatalf("last v2 advert priority = %d, want 0", last.priority)
	}
	if in.machine.State() != fsm.StateInitialize {
		t.Fatalf("v2 state after Shutdown = %v, want Initialize", in.machine.State())
	}
	if in.advert != nil {
		t.Error("the v2 advertisement timer is still armed after Shutdown")
	}
	clk.Add(10 * time.Second)
	select {
	case ev := <-in.events:
		t.Fatalf("a timer fired after Shutdown: %T", ev)
	case <-time.After(100 * time.Millisecond):
	}
	if after := f.snapshot(); len(after.adverts) != len(got.adverts) {
		t.Fatalf("adverts after Shutdown = %d, want %d", len(after.adverts), len(got.adverts))
	}

	spec.IsOwner = false
	inB, fB, _ := newTestInstance(t, spec)
	inB.dispatch(fsm.Startup{Config: inB.fsmConfig()})
	inB.dispatch(fsm.Shutdown{})
	if n := len(fB.snapshot().adverts); n != 0 {
		t.Fatalf("a v2 Backup sent %d advertisements on Shutdown, want 0", n)
	}
	if inB.machine.State() != fsm.StateInitialize {
		t.Fatalf("v2 Backup state after Shutdown = %v, want Initialize", inB.machine.State())
	}
}

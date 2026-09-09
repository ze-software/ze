// VALIDATES: spec-per-protocol-fib-import AC-1 to AC-4 -- an operator names the
// protocols whose routes are NOT written to the FIB, everything else is written
// as before, and a withheld protocol still wins selection and stays in the RIB.
// PREVENTS: the two failures that make the feature worse than not having it --
// a protocol nobody named losing its routes, and a withheld protocol keeping a
// stale kernel entry after it takes a prefix from a protocol Ze does program.

package sysrib

import (
	"encoding/json"
	"net/netip"
	"slices"
	"testing"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// withholdConfig renders the section body a plugin receives for
// `rib { fib-withhold [ names... ] }`, which is the JSON the parse reads.
func withholdConfig(t *testing.T, names ...string) string {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{
		"rib": map[string]any{"fib-withhold": names},
	})
	if err != nil {
		t.Fatalf("render the config: %v", err)
	}
	return string(encoded)
}

// configuredSysRIB returns a system RIB carrying the permission set the named
// withholds produce, on an event bus the test reads back.
func configuredSysRIB(t *testing.T, bus *testEventBus, withheld ...string) *sysRIB {
	t.Helper()
	setEventBus(bus)
	t.Cleanup(clearEventBus)

	permit, err := parseFIBImportConfig(withholdConfig(t, withheld...))
	if err != nil {
		t.Fatalf("parse the withhold list: %v", err)
	}
	s := newSysRIB()
	s.applyFIBImport(permit)
	return s
}

// publishedChanges returns every change sysrib emitted on (system-rib,
// best-change), in the order it emitted them.
func publishedChanges(t *testing.T, bus *testEventBus) []outgoingChange {
	t.Helper()
	bus.mu.Lock()
	defer bus.mu.Unlock()

	var changes []outgoingChange
	for _, event := range bus.events {
		if event.EventType != sysribevents.EventBestChange {
			continue
		}
		batch, ok := event.Payload.(*outgoingBatch)
		if !ok {
			t.Fatalf("payload is a %T, want *outgoingBatch", event.Payload)
		}
		changes = append(changes, batch.Changes...)
	}
	return changes
}

// addRoute hands sysrib one route over a gateway every test here shares, and
// publishes whatever the arbitration decided, which is the pair of steps every
// live path takes.
func addRoute(s *sysRIB, protocol string, prefix netip.Prefix, priority int) {
	addPath(s, protocol, prefix, netip.MustParseAddr("192.0.2.1"), "", priority)
}

// TestFIBImportDefaultPermitsEveryRegisteredProtocol is AC-1. A config that
// names nothing programs every protocol, and the permission is a named branch
// over the whole registry rather than a map that answers nothing.
func TestFIBImportDefaultPermitsEveryRegisteredProtocol(t *testing.T) {
	// Two protocols so the assertion below reads a populated registry. A
	// registry with nothing in it would satisfy every loop here.
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")

	permit, err := parseFIBImportConfig("{}")
	if err != nil {
		t.Fatalf("parse an empty config: %v", err)
	}

	names := redistevents.ProtocolNames()
	if len(permit) != len(names) {
		t.Errorf("the permission set holds %d protocols, the registry holds %d: it is not complete",
			len(permit), len(names))
	}
	for _, name := range names {
		permitted, known := permit[name]
		if !known {
			t.Errorf("the registered protocol %q is absent from the permission set", name)
			continue
		}
		if !permitted {
			t.Errorf("the registered protocol %q is withheld by a config that names nothing", name)
		}
	}

	s := newSysRIB()
	s.applyFIBImport(permit)
	for _, name := range names {
		if !s.fibPermitted(name) {
			t.Errorf("%q is not permitted to reach the FIB by default", name)
		}
	}
}

// TestWithheldWinnerPublishesNoAdd is AC-2. The withheld protocol's winner
// reaches no FIB plugin, and the protocol beside it is untouched.
func TestWithheldWinnerPublishesNoAdd(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus, "bgp")

	withheld := netip.MustParsePrefix("10.0.0.0/24")
	kept := netip.MustParsePrefix("10.1.0.0/24")
	addRoute(s, "bgp", withheld, 20)
	addRoute(s, "ospf", kept, 110)

	for _, change := range publishedChanges(t, bus) {
		if change.Prefix == withheld {
			t.Errorf("a withheld protocol reached the FIB: %v %s", change.Action, change.Prefix)
		}
	}

	var keptAdds int
	for _, change := range publishedChanges(t, bus) {
		if change.Prefix == kept && change.Action == routeaction.Add {
			keptAdds++
		}
	}
	if keptAdds != 1 {
		t.Errorf("the kept protocol published %d adds for %s, want 1", keptAdds, kept)
	}

	// The withheld route is declined at the FIB write and nowhere else.
	if best := s.best[prefixKey{family: family.IPv4Unicast, prefix: withheld}]; best == nil {
		t.Error("the withheld prefix left the system RIB; only the FIB write is declined")
	} else if best.protocol != "bgp" {
		t.Errorf("the withheld prefix is held by %q, want bgp", best.protocol)
	}
}

// TestWithheldProtocolStillWinsSelection is AC-3. Selection and programming are
// separate stages: the withheld protocol takes the prefix on distance, and the
// entry Ze had programmed for the protocol it beat is withdrawn rather than
// left behind.
func TestWithheldProtocolStillWinsSelection(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus, "bgp")

	contested := netip.MustParsePrefix("10.2.0.0/24")
	addRoute(s, "ospf", contested, 110)
	addRoute(s, "bgp", contested, 20)

	best := s.best[prefixKey{family: family.IPv4Unicast, prefix: contested}]
	if best == nil {
		t.Fatal("the contested prefix is not in the system RIB")
	}
	if best.protocol != "bgp" {
		t.Errorf("the winner is %q, want bgp: withholding a protocol must not change selection", best.protocol)
	}

	changes := publishedChanges(t, bus)
	if len(changes) != 2 {
		t.Fatalf("published %+v, want the OSPF add and then a withdraw", changes)
	}
	if changes[0].Action != routeaction.Add || changes[0].Protocol != "ospf" {
		t.Errorf("first change is %v from %q, want an add from ospf", changes[0].Action, changes[0].Protocol)
	}
	if changes[1].Action != routeaction.Withdraw || changes[1].Prefix != contested {
		t.Errorf("second change is %v %s, want a withdraw of %s: the OSPF entry is stale the moment "+
			"a withheld protocol takes the prefix", changes[1].Action, changes[1].Prefix, contested)
	}
}

// TestFIBImportPermitsAProtocolRegisteredAfterConfigure is AC-4. The permission
// set is complete over the registry AS IT WAS at configure time, so a protocol
// that registers later is unknown to it. Unknown fails OPEN, because reading it
// as a withhold would blackhole every route the new protocol carries.
func TestFIBImportPermitsAProtocolRegisteredAfterConfigure(t *testing.T) {
	bus := newTestEventBus()
	s := configuredSysRIB(t, bus, "bgp")

	const late = "test-late-protocol"
	if _, known := redistevents.ProtocolIDOf(late); known {
		t.Fatalf("%q was already registered, so this test proves nothing about a later registration", late)
	}
	redistevents.RegisterProtocol(late)

	if !s.fibPermitted(late) {
		t.Error("a protocol registered after configure is withheld: an unknown protocol MUST fail open")
	}

	prefix := netip.MustParsePrefix("10.3.0.0/24")
	addRoute(s, late, prefix, 100)

	var adds int
	for _, change := range publishedChanges(t, bus) {
		if change.Prefix == prefix && change.Action == routeaction.Add {
			adds++
		}
	}
	if adds != 1 {
		t.Errorf("a route from a protocol registered after configure published %d adds, want 1", adds)
	}
}

// TestReconfigureWithdrawsAProtocolNewlyWithheld pins the half of AC-2 a route
// update would otherwise hide. An operator who withholds a protocol on a
// running router expects its routes out of the kernel now, not on the next
// update from a peer.
func TestReconfigureWithdrawsAProtocolNewlyWithheld(t *testing.T) {
	redistevents.RegisterProtocol("bgp")

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	prefix := netip.MustParsePrefix("10.4.0.0/24")
	addRoute(s, "bgp", prefix, 20)

	permit, err := parseFIBImportConfig(withholdConfig(t, "bgp"))
	if err != nil {
		t.Fatalf("parse the withhold list: %v", err)
	}
	for fam, changes := range s.applyFIBImport(permit) {
		publishChanges(changes, fam)
	}

	changes := publishedChanges(t, bus)
	if len(changes) != 2 {
		t.Fatalf("published %+v, want the add and then the withdraw the new config owes", changes)
	}
	if changes[1].Action != routeaction.Withdraw || changes[1].Prefix != prefix {
		t.Errorf("the reconfigure published %v %s, want a withdraw of %s",
			changes[1].Action, changes[1].Prefix, prefix)
	}
}

// TestReconfigureReaddsAProtocolNewlyPermitted is the other direction. A
// protocol released from the withhold list gets its routes programmed from the
// table sysrib already holds, with no help from the protocol that produced them.
func TestReconfigureReaddsAProtocolNewlyPermitted(t *testing.T) {
	redistevents.RegisterProtocol("bgp")

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus, "bgp")

	prefix := netip.MustParsePrefix("10.5.0.0/24")
	addRoute(s, "bgp", prefix, 20)
	if changes := publishedChanges(t, bus); len(changes) != 0 {
		t.Fatalf("setup: a withheld protocol published %+v", changes)
	}

	permit, err := parseFIBImportConfig("{}")
	if err != nil {
		t.Fatalf("parse an empty config: %v", err)
	}
	for fam, changes := range s.applyFIBImport(permit) {
		publishChanges(changes, fam)
	}

	changes := publishedChanges(t, bus)
	if len(changes) != 1 {
		t.Fatalf("published %+v, want one add for the prefix the new config permits", changes)
	}
	if changes[0].Action != routeaction.Add || changes[0].Prefix != prefix {
		t.Errorf("the reconfigure published %v %s, want an add of %s",
			changes[0].Action, changes[0].Prefix, prefix)
	}
	if changes[0].Protocol != "bgp" {
		t.Errorf("the add names %q, want bgp", changes[0].Protocol)
	}
}

// wireLocRIB installs a Loc-RIB, and with it the next-hop resolver whose
// tracking table the tests below read. It answers with the RIB, so a test can
// give a next-hop its reachability or take it away.
func wireLocRIB(t *testing.T) *locrib.RIB {
	t.Helper()
	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })
	return loc
}

// addPath hands sysrib one route naming a gateway, a device or both, and
// publishes whatever the arbitration decided.
func addPath(s *sysRIB, protocol string, prefix netip.Prefix, nextHop netip.Addr, iface string, priority int) {
	fam, changes := s.processEvent(makePayload(protocol, family.IPv4Unicast, []incomingChange{{
		Action:    routeaction.Add,
		Prefix:    prefix,
		NextHop:   nextHop,
		Interface: iface,
		Priority:  priority,
	}}))
	if len(changes) > 0 {
		publishChanges(changes, fam)
	}
}

// withdrawPath takes one protocol's route for a prefix away, which hands the
// prefix to the next best candidate sysrib holds.
func withdrawPath(s *sysRIB, protocol string, prefix netip.Prefix) {
	fam, changes := s.processEvent(makePayload(protocol, family.IPv4Unicast, []incomingChange{{
		Action: routeaction.Withdraw,
		Prefix: prefix,
	}}))
	if len(changes) > 0 {
		publishChanges(changes, fam)
	}
}

// reconfigure installs the permission set the named withholds produce and
// publishes what the switch owes, which is the pair of steps a configure
// callback takes.
func reconfigure(t *testing.T, s *sysRIB, withheld ...string) {
	t.Helper()
	permit, err := parseFIBImportConfig(withholdConfig(t, withheld...))
	if err != nil {
		t.Fatalf("parse the withhold list: %v", err)
	}
	publishFIBImport(s, permit)
}

// trackedPrefixes reports the prefixes the resolver re-evaluates when nextHop
// changes. A prefix absent from that set is never re-evaluated, so its kernel
// entry survives its next-hop becoming unreachable.
func trackedPrefixes(t *testing.T, nextHop netip.Addr) []netip.Prefix {
	t.Helper()
	r := getNHResolver()
	if r == nil {
		t.Fatal("no next-hop resolver is wired, so this test proves nothing about tracking")
	}
	return r.Dependents(nextHop)
}

// TestWithheldWinnerWithdrawsADeviceOnlyRoute pins the FIB state test at
// recordWithheldWinner. A route naming a device and no gateway is programmed
// with an INVALID address in resolvedNH, which a validity test cannot tell from
// a prefix Ze never programmed. Reading it that way leaves the device route in
// the kernel after a withheld protocol takes the prefix.
func TestWithheldWinnerWithdrawsADeviceOnlyRoute(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("static")

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus, "bgp")

	pfx := netip.MustParsePrefix("10.6.0.0/24")
	addPath(s, "static", pfx, netip.Addr{}, "eth0", 110)
	addPath(s, "bgp", pfx, netip.MustParseAddr("192.0.2.1"), "", 20)

	changes := publishedChanges(t, bus)
	if len(changes) != 2 {
		t.Fatalf("published %+v, want the static add and then the withdraw a withheld winner owes", changes)
	}
	if changes[1].Action != routeaction.Withdraw || changes[1].Prefix != pfx {
		t.Errorf("published %v %s, want a withdraw of %s: the device route Ze programmed is stale "+
			"the moment a withheld protocol takes the prefix", changes[1].Action, changes[1].Prefix, pfx)
	}
}

// TestWithholdingAProtocolWithdrawsItsDeviceOnlyRoute is the same FIB state
// test on the reconfigure sweep. An operator who withholds a protocol expects
// its routes out of the kernel, and a route reached over a device alone is one
// of them.
func TestWithholdingAProtocolWithdrawsItsDeviceOnlyRoute(t *testing.T) {
	redistevents.RegisterProtocol("static")

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	pfx := netip.MustParsePrefix("10.7.0.0/24")
	addPath(s, "static", pfx, netip.Addr{}, "eth0", 110)
	reconfigure(t, s, "static")

	changes := publishedChanges(t, bus)
	if len(changes) != 2 {
		t.Fatalf("published %+v, want the add and then the withdraw the new config owes", changes)
	}
	if changes[1].Action != routeaction.Withdraw || changes[1].Prefix != pfx {
		t.Errorf("published %v %s, want a withdraw of %s: a device-only route is programmed like any other",
			changes[1].Action, changes[1].Prefix, pfx)
	}
}

// TestPermittedWinnerReclaimsTheNextHopAWithheldWinnerReleased pins the
// resolver tracking a promotion assumes. A withheld winner releases the
// previous winner's next-hop, so a permitted protocol taking the prefix back
// with the SAME next-hop finds no tracking entry, and the kernel keeps
// forwarding to a gateway that has gone unreachable.
func TestPermittedWinnerReclaimsTheNextHopAWithheldWinnerReleased(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("static")
	wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus, "bgp")

	pfx := netip.MustParsePrefix("10.8.0.0/24")
	nextHop := netip.MustParseAddr("192.0.2.1")
	addPath(s, "static", pfx, nextHop, "", 110)
	if !slices.Contains(trackedPrefixes(t, nextHop), pfx) {
		t.Fatalf("setup: the programmed route did not track %v", nextHop)
	}
	addPath(s, "bgp", pfx, nextHop, "", 20)
	withdrawPath(s, "bgp", pfx)

	if !slices.Contains(trackedPrefixes(t, nextHop), pfx) {
		t.Errorf("%s is programmed and %v is not tracked: nothing re-evaluates the prefix when the "+
			"gateway goes unreachable", pfx, nextHop)
	}
}

// TestReconfigureDoesNotResurrectAnUnreachableRoute pins what the sweep is
// about: a PERMISSION that changed. A prefix withdrawn because its next-hop
// stopped resolving is absent from the FIB state for a reason the permission
// set knows nothing about, and every later rib apply would program it again.
func TestReconfigureDoesNotResurrectAnUnreachableRoute(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	covering := netip.MustParsePrefix("192.0.2.0/24")
	loc.Insert(family.IPv4Unicast, covering, locrib.Path{Source: connectedID})
	pfx := netip.MustParsePrefix("10.9.0.0/24")
	nextHop := netip.MustParseAddr("192.0.2.1")
	addPath(s, "bgp", pfx, nextHop, "", 20)

	loc.Remove(family.IPv4Unicast, covering, connectedID, 0)
	s.processCascade([]netip.Addr{nextHop})

	changes := publishedChanges(t, bus)
	if len(changes) != 2 || changes[1].Action != routeaction.Withdraw {
		t.Fatalf("setup: published %+v, want the add and the withdraw an unreachable next-hop owes", changes)
	}

	reconfigure(t, s)

	if extra := publishedChanges(t, bus)[2:]; len(extra) != 0 {
		t.Errorf("a config that changed no permission published %+v: the next-hop is still unreachable "+
			"and nothing permitted the prefix back", extra)
	}
}

// TestWithholdingAgainReleasesTheNextHopThePermitTracked closes the cycle. The
// sweep tracks a next-hop when it permits a protocol, so it owes the release
// when it withholds it again; otherwise the resolver walks a prefix Ze does not
// program and showNHTable prints it.
func TestWithholdingAgainReleasesTheNextHopThePermitTracked(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus, "bgp")

	pfx := netip.MustParsePrefix("10.10.0.0/24")
	nextHop := netip.MustParseAddr("192.0.2.1")
	addPath(s, "bgp", pfx, nextHop, "", 20)
	reconfigure(t, s)
	if !slices.Contains(trackedPrefixes(t, nextHop), pfx) {
		t.Fatalf("setup: permitting the protocol programmed %s without tracking %v", pfx, nextHop)
	}

	reconfigure(t, s, "bgp")

	if slices.Contains(trackedPrefixes(t, nextHop), pfx) {
		t.Errorf("%s still depends on %v after the protocol was withheld again: the entry outlives "+
			"the route it was taken for", pfx, nextHop)
	}
}

// TestWithheldProtocolIsNoMemberOfTheMultipath is the case that defeats the
// feature for the operator who most needs it. Two protocols at one distance and
// one metric make an equal-cost group, and a group member is programmed like
// the winner is: traffic forwards over the withheld protocol's gateway.
func TestWithheldProtocolIsNoMemberOfTheMultipath(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus, "ospf")

	pfx := netip.MustParsePrefix("10.11.0.0/24")
	ospfNextHop := netip.MustParseAddr("192.0.2.2")
	addPath(s, "ospf", pfx, ospfNextHop, "", 20)
	addPath(s, "bgp", pfx, netip.MustParseAddr("192.0.2.1"), "", 20)

	changes := publishedChanges(t, bus)
	if len(changes) != 1 {
		t.Fatalf("published %+v, want the one change the permitted winner owes", changes)
	}
	for _, path := range changes[0].ECMPPaths {
		if path.NextHop == ospfNextHop {
			t.Errorf("the multipath group for %s carries %v, the gateway of a protocol the operator "+
				"withholds: traffic forwards over it", pfx, ospfNextHop)
		}
	}
}

// TestWithholdingAnECMPMemberRegroupsTheMultipath is the same defect on the
// reconfigure sweep. The winner's permission did not change, so a sweep that
// reads the winner alone leaves the withheld member in the kernel group.
func TestWithholdingAnECMPMemberRegroupsTheMultipath(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	pfx := netip.MustParsePrefix("10.12.0.0/24")
	ospfNextHop := netip.MustParseAddr("192.0.2.2")
	addPath(s, "ospf", pfx, ospfNextHop, "", 20)
	addPath(s, "bgp", pfx, netip.MustParseAddr("192.0.2.1"), "", 20)

	reconfigure(t, s, "ospf")

	changes := publishedChanges(t, bus)
	if len(changes) != 3 {
		t.Fatalf("published %+v, want the two adds and the regroup the new config owes", changes)
	}
	if changes[2].Action != routeaction.Update || len(changes[2].ECMPPaths) != 0 {
		t.Errorf("published %v with %+v, want an update whose group is empty: the withheld member "+
			"stays in the kernel group until the winner sends an update of its own",
			changes[2].Action, changes[2].ECMPPaths)
	}
}

// TestWithholdingAMemberRegroupsAPrefixWhoseGatewayDoesNotResolve is the same
// regroup on a box whose Loc-RIB covers no gateway. The prefix is programmed
// all the same, because the live path programs what the resolver cannot
// disprove, and the group it programs is the one the producers named.
//
// So withholding a MEMBER owes the regroup here too. A sweep that read the
// resolver's verdict as reachability LOST withdraws the prefix instead, and
// withholding one protocol then blackholes a prefix another protocol won.
func TestWithholdingAMemberRegroupsAPrefixWhoseGatewayDoesNotResolve(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")
	wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	pfx := netip.MustParsePrefix("10.17.0.0/24")
	ospfNextHop := netip.MustParseAddr("192.0.2.2")
	addPath(s, "ospf", pfx, ospfNextHop, "", 20)
	addPath(s, "bgp", pfx, netip.MustParseAddr("192.0.2.1"), "", 20)

	changes := publishedChanges(t, bus)
	if len(changes) != 2 {
		t.Fatalf("setup: published %+v, want the add and the change the second protocol owes", changes)
	}
	if !slices.ContainsFunc(changes[1].ECMPPaths, func(path sysribevents.ECMPPath) bool {
		return path.NextHop == ospfNextHop
	}) {
		t.Fatalf("setup: the group is %+v, want %v in it: this test proves nothing about a member "+
			"the sweep has to take out", changes[1].ECMPPaths, ospfNextHop)
	}

	reconfigure(t, s, "ospf")

	after := publishedChanges(t, bus)[2:]
	if len(after) != 1 || after[0].Action != routeaction.Update {
		t.Fatalf("withholding a group member published %+v, want one update: the permitted winner "+
			"keeps %s, and a withdraw here takes it out of the FIB", after, pfx)
	}
	if len(after[0].ECMPPaths) != 0 {
		t.Errorf("the update carries %+v, want an empty group: the withheld member stays in the "+
			"kernel group", after[0].ECMPPaths)
	}
}

// TestWithdrawingAWithheldPrefixPublishesNothing is the withdraw side of AC-2.
// Ze programmed nothing for a withheld protocol's prefix, so the producer
// taking its last route away leaves the FIB with nothing to remove. A withdraw
// published here is a delete of an entry that was never made, which the kernel
// writer answers with a FIB sync error for every prefix -- one for every route
// a route collector's peers withdraw.
func TestWithdrawingAWithheldPrefixPublishesNothing(t *testing.T) {
	redistevents.RegisterProtocol("bgp")

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus, "bgp")

	pfx := netip.MustParsePrefix("10.13.0.0/24")
	addRoute(s, "bgp", pfx, 20)
	if changes := publishedChanges(t, bus); len(changes) != 0 {
		t.Fatalf("setup: a withheld protocol published %+v", changes)
	}

	withdrawPath(s, "bgp", pfx)

	if changes := publishedChanges(t, bus); len(changes) != 0 {
		t.Errorf("withdrawing a withheld prefix published %+v: Ze programmed nothing for %s, "+
			"so the FIB writer is asked to delete an entry it never made", changes, pfx)
	}
}

// TestPermittingRestoresThePrefixTheWithholdTook is the newly-permitted half of
// the sweep's rule, and the rule is the LIVE one: permitting a protocol is a
// fresh install decision, not reachability news. A gateway the Loc-RIB does not
// cover is no proof of unreachability, because an OSPF or IS-IS next-hop on a
// link whose connected route no plugin inserted resolves to nothing and is
// on-link all the same. recomputeBest programs that prefix, so the sweep owes
// the same answer.
//
// A sweep that took the cascade's rule instead publishes nothing here, and
// nothing later repairs it: an identical re-announcement is a no-op, and a
// cascade fires only when a covering route changes, which on this box never
// happens. One withhold would take the prefix out of the FIB permanently while
// the RIB reports it as the winner.
func TestPermittingRestoresThePrefixTheWithholdTook(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	pfx := netip.MustParsePrefix("10.14.0.0/24")
	nextHop := netip.MustParseAddr("192.0.2.1")
	addPath(s, "bgp", pfx, nextHop, "", 20)

	reconfigure(t, s, "bgp")
	if changes := publishedChanges(t, bus); len(changes) != 2 || changes[1].Action != routeaction.Withdraw {
		t.Fatalf("setup: published %+v, want the add the live path owes for an unresolved gateway "+
			"and the withdraw the withhold owes", changes)
	}

	reconfigure(t, s)

	back := publishedChanges(t, bus)[2:]
	if len(back) != 1 || back[0].Action != routeaction.Add || back[0].Prefix != pfx {
		t.Fatalf("permitting the protocol published %+v, want an add of %s: the withhold withdrew a "+
			"prefix the live path programs, and nothing else ever puts it back", back, pfx)
	}
	if back[0].NextHop != nextHop {
		t.Errorf("the add names %v, want %v: an unresolved entry carries the gateway its producer named",
			back[0].NextHop, nextHop)
	}
	if !slices.Contains(trackedPrefixes(t, nextHop), pfx) {
		t.Errorf("%s is programmed again and %v is not tracked: nothing re-evaluates the prefix when "+
			"the gateway changes", pfx, nextHop)
	}
}

// TestWithholdingReleasesTheNextHopOfAnUnprogrammedPrefix is the release side
// of the same state. A prefix waiting on an unreachable next-hop is tracked
// with nothing installed, so the withhold owes the release whether or not the
// FIB holds the prefix. Reading the FIB state first leaves the resolver walking
// a prefix a withheld protocol owns.
func TestWithholdingReleasesTheNextHopOfAnUnprogrammedPrefix(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	covering := netip.MustParsePrefix("192.0.2.0/24")
	loc.Insert(family.IPv4Unicast, covering, locrib.Path{Source: connectedID})
	pfx := netip.MustParsePrefix("10.15.0.0/24")
	nextHop := netip.MustParseAddr("192.0.2.1")
	addPath(s, "bgp", pfx, nextHop, "", 20)

	loc.Remove(family.IPv4Unicast, covering, connectedID, 0)
	s.processCascade([]netip.Addr{nextHop})
	if !slices.Contains(trackedPrefixes(t, nextHop), pfx) {
		t.Fatalf("setup: the cascade released %v, so this test proves nothing about the withhold", nextHop)
	}

	reconfigure(t, s, "bgp")

	if slices.Contains(trackedPrefixes(t, nextHop), pfx) {
		t.Errorf("%s still depends on %v after its protocol was withheld: the resolver re-evaluates "+
			"a prefix a withheld protocol owns", pfx, nextHop)
	}
}

// TestUnrelatedApplyKeepsAnUnreachableMemberOutOfTheGroup pins what the regroup
// branch is allowed to act on. A cascade writes the group the FIB holds with
// the unreachable members taken out; an apply that changes no permission must
// not read its own collector against that state, find a difference that is not
// a permission change, and put the member the resolver dropped back into the
// kernel multipath.
func TestUnrelatedApplyKeepsAnUnreachableMemberOutOfTheGroup(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	winnerNH := netip.MustParseAddr("192.0.2.1")
	memberNH := netip.MustParseAddr("192.0.2.2")
	memberCovering := netip.MustParsePrefix("192.0.2.2/32")
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.1/32"), locrib.Path{Source: connectedID})
	loc.Insert(family.IPv4Unicast, memberCovering, locrib.Path{Source: connectedID})

	pfx := netip.MustParsePrefix("10.16.0.0/24")
	addPath(s, "bgp", pfx, winnerNH, "", 20)
	addPath(s, "ospf", pfx, memberNH, "", 20)

	loc.Remove(family.IPv4Unicast, memberCovering, connectedID, 0)
	s.processCascade([]netip.Addr{winnerNH})
	published := publishedChanges(t, bus)
	if len(published) != 3 || len(published[2].ECMPPaths) != 0 {
		t.Fatalf("setup: published %+v, want the cascade to drop the unreachable member", published)
	}

	reconfigure(t, s)

	for _, change := range publishedChanges(t, bus)[3:] {
		for _, path := range change.ECMPPaths {
			if path.NextHop == memberNH {
				t.Errorf("an apply that changed no permission published %v carrying %v: the member "+
					"the resolver dropped is back in the kernel multipath", change.Action, memberNH)
			}
		}
	}
}

// TestWithholdingAnUnreachableMemberPublishesNothing pins the state test in
// fibStateChange, the one branch that keeps the sweep silent. Withholding the
// member's protocol changes the RIB group and changes nothing the FIB was told,
// because the cascade had already taken that member out of the group Ze
// emitted: its gateway stopped resolving.
//
// Both halves of the sweep are needed to reach this branch, and they read the
// group differently on purpose. groupPermissionChanged reads the UNFILTERED
// group, so it answers "changed" and hands the prefix on. fibEntry filters by
// reachability, so it answers with the entry Ze already emitted. Without the
// state test the sweep publishes an Update repeating that entry, which asks the
// kernel writer to replace an entry with itself on every unrelated `rib` apply.
func TestWithholdingAnUnreachableMemberPublishesNothing(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")
	connectedID := redistevents.RegisterProtocol("connected")
	loc := wireLocRIB(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus)

	winnerNH := netip.MustParseAddr("192.0.2.1")
	memberNH := netip.MustParseAddr("192.0.2.2")
	memberCovering := netip.MustParsePrefix("192.0.2.2/32")
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.1/32"), locrib.Path{Source: connectedID})
	loc.Insert(family.IPv4Unicast, memberCovering, locrib.Path{Source: connectedID})

	pfx := netip.MustParsePrefix("10.19.0.0/24")
	addPath(s, "bgp", pfx, winnerNH, "", 20)
	addPath(s, "ospf", pfx, memberNH, "", 20)

	loc.Remove(family.IPv4Unicast, memberCovering, connectedID, 0)
	s.processCascade([]netip.Addr{winnerNH})
	published := publishedChanges(t, bus)
	if len(published) != 3 || len(published[2].ECMPPaths) != 0 {
		t.Fatalf("setup: published %+v, want the cascade to drop the unreachable member", published)
	}

	reconfigure(t, s, "ospf")

	if swept := publishedChanges(t, bus)[3:]; len(swept) != 0 {
		t.Errorf("withholding %v published %+v, want nothing: the FIB already holds %s over %v "+
			"alone, so the permission that moved owes it no change", memberNH, swept, pfx, winnerNH)
	}
}

// ribEntries reads `show rib` as an operator does, through the JSON the command
// answers with: the entry type is declared inside the command, so the encoded
// form is the only shape a test can name.
func ribEntries(t *testing.T, s *sysRIB) []map[string]any {
	t.Helper()
	data, err := s.showRIB()
	if err != nil {
		t.Fatalf("show rib: %v", err)
	}
	return decodeEntries(t, data)
}

// ecmpEntries reads `show ecmp-groups` the same way.
func ecmpEntries(t *testing.T, s *sysRIB) []map[string]any {
	t.Helper()
	data, err := s.showECMPGroups()
	if err != nil {
		t.Fatalf("show ecmp-groups: %v", err)
	}
	return decodeEntries(t, data)
}

func decodeEntries(t *testing.T, data any) []map[string]any {
	t.Helper()
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("render the answer: %v", err)
	}
	var entries []map[string]any
	if err := json.Unmarshal(encoded, &entries); err != nil {
		t.Fatalf("read the answer back: %v", err)
	}
	return entries
}

// TestWithheldPrefixKeepsItsGroupInTheRIBViews is the display half of "a
// withheld route is not a dropped route". Both commands name the RIB, and the
// RIB holds the equal-cost paths that competed for the prefix. Reporting them
// from the last group EMITTED would answer a RIB question with a FIB filter,
// and an operator would read the withheld prefix as one with no second path.
func TestWithheldPrefixKeepsItsGroupInTheRIBViews(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus, "bgp", "ospf")

	pfx := netip.MustParsePrefix("10.17.0.0/24")
	addPath(s, "bgp", pfx, netip.MustParseAddr("192.0.2.1"), "", 20)
	addPath(s, "ospf", pfx, netip.MustParseAddr("192.0.2.2"), "", 20)

	groups := ecmpEntries(t, s)
	if len(groups) != 1 {
		t.Errorf("show ecmp-groups reports %d groups, want the one %s holds: the prefix is "+
			"withheld from the FIB, not dropped from the RIB", len(groups), pfx)
	}

	entries := ribEntries(t, s)
	if len(entries) != 1 {
		t.Fatalf("show rib reports %+v, want the one prefix the RIB holds", entries)
	}
	paths, _ := entries[0]["ecmp-paths"].([]any)
	if len(paths) != 1 {
		t.Errorf("show rib reports %d equal-cost paths for %s, want the one the second protocol "+
			"contributes: the RIB view is taking the FIB's permission filter", len(paths), pfx)
	}
}

// TestPermittingAnOSInstalledProtocolProgramsNothing keeps the sweep out of the
// one prefix that is never Ze's to install. The operator can name such a
// protocol in the leaf-list and take it out again, and neither move gives Ze a
// route to write: the OS already holds the entry, and an Add here is the
// two-writer collision the arbitration exists to prevent.
func TestPermittingAnOSInstalledProtocolProgramsNothing(t *testing.T) {
	protocol := osInstalledProtocol(t)

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus, protocol)

	pfx := netip.MustParsePrefix("10.18.0.0/24")
	addPath(s, protocol, pfx, netip.MustParseAddr("192.0.2.1"), "", 0)

	reconfigure(t, s)

	if changes := publishedChanges(t, bus); len(changes) != 0 {
		t.Errorf("permitting %q published %+v: the operating system owns the entry for %s, so Ze "+
			"has nothing to add whichever way the permission goes", protocol, changes, pfx)
	}
}

// tablePrefixes builds count distinct /24s under one first octet, so each
// protocol in a test carries its own address space and a prefix names its
// producer without a second lookup.
func tablePrefixes(first byte, count int) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, count)
	for i := range count {
		addr := netip.AddrFrom4([4]byte{first, byte(i / 256), byte(i % 256), 0})
		prefixes = append(prefixes, netip.PrefixFrom(addr, 24))
	}
	return prefixes
}

// addRoutes hands sysrib one protocol's whole table in a single batch, which is
// the shape a peer's initial convergence takes, and publishes whatever the
// arbitration decided.
func addRoutes(s *sysRIB, protocol string, prefixes []netip.Prefix, priority int) {
	incoming := make([]incomingChange, 0, len(prefixes))
	for _, prefix := range prefixes {
		incoming = append(incoming, incomingChange{
			Action:   routeaction.Add,
			Prefix:   prefix,
			NextHop:  netip.MustParseAddr("192.0.2.1"),
			Priority: priority,
		})
	}
	fam, changes := s.processEvent(makePayload(protocol, family.IPv4Unicast, incoming))
	if len(changes) > 0 {
		publishChanges(changes, fam)
	}
}

// TestWithholdingBGPProgramsNoneOfAWholeTable is AC-6, the deployment this
// feature exists for: a controller holds every peer's routes in the RIB and
// programs no kernel route at all. Every other test here carries one or two
// prefixes, so a gate that misses one route in a hundred passes all of them.
// This one hands sysrib a TABLE and reads the whole published stream back.
//
// The permitted protocol's prefixes in the same run are the control. A test
// that proved only absence would pass against a sysrib that published nothing,
// which is the failure such a test exists to catch.
func TestWithholdingBGPProgramsNoneOfAWholeTable(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	redistevents.RegisterProtocol("ospf")

	bus := newTestEventBus()
	s := configuredSysRIB(t, bus, "bgp")

	withheld := tablePrefixes(10, 512)
	kept := tablePrefixes(172, 16)
	addRoutes(s, "bgp", withheld, 20)
	addRoutes(s, "ospf", kept, 110)

	withholding := make(map[netip.Prefix]bool, len(withheld))
	for _, prefix := range withheld {
		withholding[prefix] = true
	}

	// Nothing at all is owed for a withheld prefix, and that is wider than "no
	// Add". Ze never installed one, so a Withdraw would ask the kernel writer
	// to delete an entry it never made, which it answers with ESRCH and one
	// fib-sync failure per prefix -- on this deployment, per withdrawn route.
	var reached int
	var first outgoingChange
	var keptAdds int
	for _, change := range publishedChanges(t, bus) {
		if withholding[change.Prefix] {
			if reached == 0 {
				first = change
			}
			reached++
			continue
		}
		if change.Action == routeaction.Add {
			keptAdds++
		}
	}
	if reached != 0 {
		t.Errorf("%d of the %d withheld prefixes reached the FIB stream, the first as %v %s: a "+
			"controller withholding bgp programs no route of its table",
			reached, len(withheld), first.Action, first.Prefix)
	}
	if keptAdds != len(kept) {
		t.Errorf("the permitted protocol published %d adds for its %d prefixes: without them this "+
			"test would pass against a sysrib that publishes nothing at all", keptAdds, len(kept))
	}

	var won int
	for _, prefix := range withheld {
		best := s.best[prefixKey{family: family.IPv4Unicast, prefix: prefix}]
		if best != nil && best.protocol == "bgp" {
			won++
		}
	}
	if won != len(withheld) {
		t.Errorf("the system RIB holds %d of the %d withheld prefixes as won by bgp: withholding "+
			"declines the FIB write and takes nothing out of the RIB", won, len(withheld))
	}

	// The surface an operator and a plugin read answers for the whole table,
	// which is the second half of the deployment: every peer's routes are there
	// to be served while none of them is programmed.
	entries := ribEntries(t, s)
	if len(entries) != len(withheld)+len(kept) {
		t.Errorf("show rib reports %d prefixes, want the %d the two protocols carry: a withheld "+
			"table is still the RIB's answer", len(entries), len(withheld)+len(kept))
	}
}

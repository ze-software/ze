// VALIDATES: a winner whose forwarding entry the OPERATING SYSTEM creates makes
// sysrib WITHDRAW the route Ze had programmed and install nothing; a prefix Ze had
// not programmed produces no FIB event at all; and such a protocol still LOSES a
// prefix on distance like any other, leaving Ze's route in place.
// PREVENTS: a stale RTPROT_ZE route left beside the kernel's own connected route
// for one prefix, which is the two-writer collision by another name; and the
// opposite failure, an OS-installed protocol suppressing a route it did not win.

package sysrib

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// osInstalledProtocol registers a protocol that declares the OS installs its
// routes, and returns its name. Registration is idempotent on the name, so
// repeated calls across tests answer the same id.
func osInstalledProtocol(t *testing.T) string {
	t.Helper()
	const name = "test-os-installed"
	redistevents.RegisterOSInstalled(redistevents.RegisterProtocol(name))
	return name
}

// TestOSInstalledWinnerWithdrawsTheZeRoute is AC-5. Ze had programmed the prefix
// for a BGP path; the OS-installed path then wins on distance, so Ze's own entry
// is now stale beside the one the OS holds and must go.
func TestOSInstalledWinnerWithdrawsTheZeRoute(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	pfx := netip.MustParsePrefix("10.0.0.0/24")
	_, changes := s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{{
		Action:   routeaction.Add,
		Prefix:   pfx,
		NextHop:  netip.MustParseAddr("192.0.2.1"),
		Priority: 20,
	}}))
	if len(changes) != 1 || changes[0].Action != routeaction.Add {
		t.Fatalf("setup: want one Add for the BGP path, got %+v", changes)
	}

	_, changes = s.processEvent(makePayload(osInstalledProtocol(t), family.IPv4Unicast, []incomingChange{{
		Action:   routeaction.Add,
		Prefix:   pfx,
		Priority: 0,
	}}))

	if len(changes) != 1 {
		t.Fatalf("got %d changes, want one withdraw", len(changes))
	}
	if changes[0].Action != routeaction.Withdraw {
		t.Errorf("action = %v, want withdraw: the OS owns this prefix and Ze's entry is stale", changes[0].Action)
	}
	if changes[0].Prefix != pfx {
		t.Errorf("withdrew %s, want %s", changes[0].Prefix, pfx)
	}
}

// TestOSInstalledWinnerEmitsNothingWhenNothingWasProgrammed is AC-6. With no Ze
// route for the prefix there is nothing to withdraw, and a withdraw for a route
// no FIB plugin holds is noise the plugin has to decide what to do with.
func TestOSInstalledWinnerEmitsNothingWhenNothingWasProgrammed(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	pfx := netip.MustParsePrefix("10.1.0.0/24")
	_, changes := s.processEvent(makePayload(osInstalledProtocol(t), family.IPv4Unicast, []incomingChange{{
		Action:   routeaction.Add,
		Prefix:   pfx,
		Priority: 0,
	}}))

	if len(changes) != 0 {
		t.Errorf("got %+v, want no FIB event for a prefix Ze never programmed", changes)
	}
}

// TestOSInstalledWinnerStaysInTheSystemRIB pins what the withdraw does NOT mean.
// The prefix is still held, by the protocol that won it, so `show rib` reports
// the truth and a later withdraw of that path hands the prefix to the next best.
func TestOSInstalledWinnerStaysInTheSystemRIB(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()

	protocol := osInstalledProtocol(t)
	pfx := netip.MustParsePrefix("10.2.0.0/24")
	s.processEvent(makePayload(protocol, family.IPv4Unicast, []incomingChange{{
		Action:   routeaction.Add,
		Prefix:   pfx,
		Priority: 0,
	}}))

	shown, err := s.showRIB()
	if err != nil {
		t.Fatalf("showRIB: %v", err)
	}
	encoded, err := json.Marshal(shown)
	if err != nil {
		t.Fatalf("encode show rib: %v", err)
	}
	var rows []struct {
		Prefix   netip.Prefix `json:"prefix"`
		Protocol string       `json:"protocol"`
	}
	if err := json.Unmarshal(encoded, &rows); err != nil {
		t.Fatalf("decode show rib: %v", err)
	}
	found := false
	for _, row := range rows {
		if row.Prefix == pfx && row.Protocol == protocol {
			found = true
		}
	}
	if !found {
		t.Errorf("the system RIB does not report %s as won by %s: %s", pfx, protocol, encoded)
	}
}

// TestOSInstalledLoserLeavesTheZeRouteProgrammed is AC-3. The declaration is what
// decides, and an OS-installed protocol at a raised distance loses the prefix like
// any other, so Ze's route stays.
func TestOSInstalledLoserLeavesTheZeRouteProgrammed(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)
	s := newSysRIB()
	// The operator raised the OS-installed protocol above BGP's 20.
	s.adminDist = map[string]int{osInstalledProtocol(t): 250, "bgp": 20}

	pfx := netip.MustParsePrefix("10.3.0.0/24")
	nextHop := netip.MustParseAddr("192.0.2.1")
	s.processEvent(makePayload("bgp", family.IPv4Unicast, []incomingChange{{
		Action:   routeaction.Add,
		Prefix:   pfx,
		NextHop:  nextHop,
		Priority: 20,
	}}))

	_, changes := s.processEvent(makePayload(osInstalledProtocol(t), family.IPv4Unicast, []incomingChange{{
		Action:   routeaction.Add,
		Prefix:   pfx,
		Priority: 0,
	}}))

	if len(changes) != 0 {
		t.Errorf("got %+v, want nothing: the OS-installed path lost, so Ze's route stands", changes)
	}
	if best := s.best[prefixKey{family: family.IPv4Unicast, prefix: pfx}]; best == nil || best.protocol != "bgp" {
		t.Errorf("system best is %+v, want the BGP path", best)
	}
	if resolved := s.resolvedNH[prefixKey{family: family.IPv4Unicast, prefix: pfx}]; resolved != nextHop {
		t.Errorf("Ze's programmed next-hop is %v, want %v", resolved, nextHop)
	}
}

// TestUnknownProtocolProgramsNormally is the fail-open direction, stated on
// purpose. A protocol nobody registered answers "not known", and reading that as
// "do not program" would blackhole every route it offers.
func TestUnknownProtocolProgramsNormally(t *testing.T) {
	if protocolInstalledByOS("a-protocol-nobody-registered") {
		t.Error("an unregistered protocol must not be treated as OS-installed")
	}
}

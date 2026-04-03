package watchdog

import (
	"strings"
	"sync"
	"testing"

	bgp "codeberg.org/thomas-mangin/ze/internal/component/bgp"
)

// VALIDATES: announce command sends update text for withdrawn routes
// PREVENTS: Command accepted but no routes injected into engine

func TestCommandAnnounce(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	// Simulate config delivery: one peer, one pool "dnsr", one route
	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	entry := NewPoolEntry("10.0.0.0/24#0",
		"update text origin igp nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
		"update text nlri ipv4/unicast del 10.0.0.0/24")
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dnsr", entry); err != nil {
		t.Fatal(err)
	}
	mgr.peerUp["10.0.0.1"] = true

	status, data, err := mgr.handleCommand("watchdog announce", []string{"dnsr"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("handleCommand: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want done", status)
	}
	if !strings.Contains(data, `"watchdog":"dnsr"`) {
		t.Errorf("data = %q, want watchdog name in response", data)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 1 {
		t.Fatalf("sent %d routes, want 1", len(sent))
	}
	if sent[0].peer != "10.0.0.1" {
		t.Errorf("peer = %q, want 10.0.0.1", sent[0].peer)
	}
	if !strings.Contains(sent[0].cmd, "add 10.0.0.0/24") {
		t.Errorf("cmd = %q, want announce command", sent[0].cmd)
	}
}

// VALIDATES: withdraw command sends withdrawal for announced routes
// PREVENTS: Withdrawal accepted but routes still announced

func TestCommandWithdraw(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	mgr.peerPools["10.0.0.2"] = NewPoolSet()
	entry := NewPoolEntry("10.0.0.0/24#0",
		"update text origin igp nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
		"update text nlri ipv4/unicast del 10.0.0.0/24")
	if err := mgr.peerPools["10.0.0.2"].AddRoute("dnsr", entry); err != nil {
		t.Fatal(err)
	}
	// Mark route as announced first
	mgr.peerPools["10.0.0.2"].AnnouncePool("dnsr", "10.0.0.2")
	mgr.peerUp["10.0.0.2"] = true

	status, _, err := mgr.handleCommand("watchdog withdraw", []string{"dnsr"}, "10.0.0.2")
	if err != nil {
		t.Fatalf("handleCommand: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want done", status)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 1 {
		t.Fatalf("sent %d routes, want 1", len(sent))
	}
	if !strings.Contains(sent[0].cmd, "del 10.0.0.0/24") {
		t.Errorf("cmd = %q, want withdraw command", sent[0].cmd)
	}
}

// VALIDATES: Error returned for nonexistent watchdog group
// PREVENTS: Silent success on typo'd group name

func TestCommandUnknownGroup(t *testing.T) {
	mgr := newWatchdogServer(func(_, _ string) {})
	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	mgr.peerUp["10.0.0.1"] = true

	status, _, err := mgr.handleCommand("watchdog announce", []string{"nonexistent"}, "10.0.0.1")
	if err == nil {
		t.Fatal("expected error for nonexistent group")
		return
	}
	if status != statusError {
		t.Errorf("status = %q, want error", status)
	}
}

// VALIDATES: State-up event triggers resend of announced routes
// PREVENTS: Reconnected peer missing watchdog routes

func TestReconnectResend(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	// Setup: peer has one route, already announced
	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	entry := NewPoolEntry("10.0.0.0/24#0",
		"update text origin igp nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
		"update text nlri ipv4/unicast del 10.0.0.0/24")
	entry.initiallyAnnounced = true
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dnsr", entry); err != nil {
		t.Fatal(err)
	}
	// Mark as announced (simulates previous session)
	mgr.peerPools["10.0.0.1"].AnnouncePool("dnsr", "10.0.0.1")

	// Peer comes up
	mgr.handleStateUp("10.0.0.1")

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 1 {
		t.Fatalf("sent %d routes on reconnect, want 1", len(sent))
	}
	if sent[0].peer != "10.0.0.1" {
		t.Errorf("peer = %q, want 10.0.0.1", sent[0].peer)
	}
	if !strings.Contains(sent[0].cmd, "add 10.0.0.0/24") {
		t.Errorf("cmd = %q, want announce command", sent[0].cmd)
	}
}

// VALIDATES: Announce/withdraw while disconnected updates state for reconnect
// PREVENTS: State lost when peer is down, wrong routes sent on reconnect

func TestDisconnectedStateUpdate(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	// Setup: peer with route, initially withdrawn (withdraw=true)
	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	entry := NewPoolEntry("10.0.0.0/24#0",
		"update text origin igp nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
		"update text nlri ipv4/unicast del 10.0.0.0/24")
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dnsr", entry); err != nil {
		t.Fatal(err)
	}
	// Peer is NOT up — announce while disconnected
	status, _, err := mgr.handleCommand("watchdog announce", []string{"dnsr"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("handleCommand: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want done", status)
	}

	// Nothing sent (peer is down)
	mu.Lock()
	if len(sent) != 0 {
		t.Fatalf("sent %d routes while disconnected, want 0", len(sent))
	}
	mu.Unlock()

	// Now peer comes up — should resend the announced route
	mgr.handleStateUp("10.0.0.1")

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 1 {
		t.Fatalf("sent %d routes on reconnect, want 1", len(sent))
	}
}

// VALIDATES: Initially announced routes (no withdraw flag) sent on first session
// PREVENTS: Routes with no withdraw flag silently dropped

func TestInitiallyAnnouncedRoutes(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	entry := NewPoolEntry("10.0.0.0/24#0",
		"update text origin igp nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
		"update text nlri ipv4/unicast del 10.0.0.0/24")
	entry.initiallyAnnounced = true
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dnsr", entry); err != nil {
		t.Fatal(err)
	}

	// Peer comes up — initially announced routes should be sent
	mgr.handleStateUp("10.0.0.1")

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 1 {
		t.Fatalf("sent %d routes, want 1 (initially announced)", len(sent))
	}
}

// VALIDATES: Initially withdrawn routes (withdraw=true) not sent on first session
// PREVENTS: Withdrawn routes prematurely sent before explicit announce command

func TestInitiallyWithdrawnRoutes(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	mgr.peerPools["10.0.0.3"] = NewPoolSet()
	entry := NewPoolEntry("10.0.0.0/24#0",
		"update text origin igp nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
		"update text nlri ipv4/unicast del 10.0.0.0/24")
	// initiallyAnnounced defaults to false — route starts withdrawn
	if err := mgr.peerPools["10.0.0.3"].AddRoute("dnsr", entry); err != nil {
		t.Fatal(err)
	}

	// Peer comes up — withdrawn routes should NOT be sent
	mgr.handleStateUp("10.0.0.3")

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 0 {
		t.Fatalf("sent %d routes, want 0 (initially withdrawn)", len(sent))
	}
}

// VALIDATES: State-down marks peer as not up, preventing route sends
// PREVENTS: Routes sent to disconnected peer

func TestStateDownPreventsRoutesSending(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	mgr.peerPools["10.0.0.4"] = NewPoolSet()
	entry := NewPoolEntry("10.0.0.0/24#0",
		"update text origin igp nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
		"update text nlri ipv4/unicast del 10.0.0.0/24")
	if err := mgr.peerPools["10.0.0.4"].AddRoute("dnsr", entry); err != nil {
		t.Fatal(err)
	}

	// Peer comes up, then goes down
	mgr.handleStateUp("10.0.0.4")
	mgr.handleStateDown("10.0.0.4")

	// Clear sent routes from the state-up
	mu.Lock()
	sent = nil
	mu.Unlock()

	// Announce while down — state updated but nothing sent
	_, _, err := mgr.handleCommand("watchdog announce", []string{"dnsr"}, "10.0.0.4")
	if err != nil {
		t.Fatalf("handleCommand: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 0 {
		t.Fatalf("sent %d routes after state-down, want 0", len(sent))
	}
}

// VALIDATES: Mixed initial state — only initiallyAnnounced routes sent on first session
// PREVENTS: AnnouncePool over-announcing withdrawn routes in mixed pools

func TestStateUpMixedInitialState(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	mgr.peerPools["10.0.0.5"] = NewPoolSet()

	// Route A: initially announced (default config route)
	routeA := NewPoolEntry("10.0.0.0/24#0",
		"update text origin igp nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
		"update text nlri ipv4/unicast del 10.0.0.0/24")
	routeA.initiallyAnnounced = true
	if err := mgr.peerPools["10.0.0.5"].AddRoute("dnsr", routeA); err != nil {
		t.Fatal(err)
	}

	// Route B: initially withdrawn (withdraw=true in config)
	routeB := NewPoolEntry("10.0.1.0/24#0",
		"update text origin igp nhop 1.2.3.4 nlri ipv4/unicast add 10.0.1.0/24",
		"update text nlri ipv4/unicast del 10.0.1.0/24")
	// initiallyAnnounced defaults to false
	if err := mgr.peerPools["10.0.0.5"].AddRoute("dnsr", routeB); err != nil {
		t.Fatal(err)
	}

	// First session establishment — only route A should be sent
	mgr.handleStateUp("10.0.0.5")

	mu.Lock()
	defer mu.Unlock()

	// Should send exactly 1 route (the initially-announced one)
	if len(sent) != 1 {
		t.Fatalf("sent %d routes, want 1 (only initially-announced)", len(sent))
	}
	if !strings.Contains(sent[0].cmd, "add 10.0.0.0/24") {
		t.Errorf("cmd = %q, want route A (10.0.0.0/24), not route B", sent[0].cmd)
	}
}

// VALIDATES: AC-1 — Rapid flap: up→down→up sends routes only on final up
// PREVENTS: Routes sent to peer during transient down state

func TestRapidFlap(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	entry := NewPoolEntry("10.0.0.0/24#0",
		"update text origin igp nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
		"update text nlri ipv4/unicast del 10.0.0.0/24")
	entry.initiallyAnnounced = true
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dnsr", entry); err != nil {
		t.Fatal(err)
	}

	// Rapid flap: up→down→up→down→up
	mgr.handleStateUp("10.0.0.1")
	mgr.handleStateDown("10.0.0.1")
	mgr.handleStateUp("10.0.0.1")
	mgr.handleStateDown("10.0.0.1")
	mgr.handleStateUp("10.0.0.1")

	mu.Lock()
	defer mu.Unlock()

	// Each state-up should resend — 3 ups = 3 sends
	if len(sent) != 3 {
		t.Fatalf("sent %d routes, want 3 (one per state-up)", len(sent))
	}
	for i, s := range sent {
		if s.peer != "10.0.0.1" {
			t.Errorf("sent[%d].peer = %q, want 10.0.0.1", i, s.peer)
		}
		if !strings.Contains(s.cmd, "add 10.0.0.0/24") {
			t.Errorf("sent[%d].cmd = %q, want announce command", i, s.cmd)
		}
	}

	// Verify no sends happened during down states by checking
	// that announce while down doesn't send
	sent = nil
	mgr.handleStateDown("10.0.0.1")
	if len(sent) != 0 {
		t.Errorf("sent %d routes after final down, want 0", len(sent))
	}
}

// VALIDATES: AC-2 — Wildcard announce with mixed peer states
// PREVENTS: Routes sent to down peers; crash on peers without the pool

func TestWildcardMixedPeerStates(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	// Peer 1: up, has pool "dnsr"
	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dnsr",
		NewPoolEntry("10.0.0.0/24#0", "announce-p1", "withdraw-p1")); err != nil {
		t.Fatal(err)
	}
	mgr.peerUp["10.0.0.1"] = true

	// Peer 2: down, has pool "dnsr"
	mgr.peerPools["10.0.0.2"] = NewPoolSet()
	if err := mgr.peerPools["10.0.0.2"].AddRoute("dnsr",
		NewPoolEntry("10.0.0.0/24#0", "announce-p2", "withdraw-p2")); err != nil {
		t.Fatal(err)
	}
	mgr.peerUp["10.0.0.2"] = false

	// Peer 3: up, has pool "other" (not "dnsr")
	mgr.peerPools["10.0.0.3"] = NewPoolSet()
	if err := mgr.peerPools["10.0.0.3"].AddRoute("other",
		NewPoolEntry("10.0.0.0/24#0", "announce-p3", "withdraw-p3")); err != nil {
		t.Fatal(err)
	}
	mgr.peerUp["10.0.0.3"] = true

	// Wildcard announce for "dnsr"
	status, data, err := mgr.handleCommand("watchdog announce", []string{"dnsr"}, "*")
	if err != nil {
		t.Fatalf("handleCommand: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want done", status)
	}
	// Should report 2 peers (peer1 and peer2 have the pool, peer3 does not)
	if !strings.Contains(data, `"peers":2`) {
		t.Errorf("data = %q, want 2 peers affected", data)
	}

	mu.Lock()
	defer mu.Unlock()

	// Only peer1 should have received the route (peer2 is down)
	if len(sent) != 1 {
		t.Fatalf("sent %d routes, want 1 (only up peer)", len(sent))
	}
	if sent[0].peer != "10.0.0.1" {
		t.Errorf("sent to %q, want 10.0.0.1", sent[0].peer)
	}
}

// VALIDATES: AC-3 — Two pools for same peer, independent state
// PREVENTS: Announce/withdraw on one pool affecting another

func TestMultiPoolIndependence(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dns",
		NewPoolEntry("10.0.0.0/24#0", "announce-dns", "withdraw-dns")); err != nil {
		t.Fatal(err)
	}
	if err := mgr.peerPools["10.0.0.1"].AddRoute("web",
		NewPoolEntry("10.0.1.0/24#0", "announce-web", "withdraw-web")); err != nil {
		t.Fatal(err)
	}
	mgr.peerUp["10.0.0.1"] = true

	// Announce both pools first
	_, _, err := mgr.handleCommand("watchdog announce", []string{"dns"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("announce dns: %v", err)
	}
	_, _, err = mgr.handleCommand("watchdog announce", []string{"web"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("announce web: %v", err)
	}

	mu.Lock()
	sent = nil // Clear initial announces
	mu.Unlock()

	// Now withdraw only "web" — dns should remain announced
	_, _, err = mgr.handleCommand("watchdog withdraw", []string{"web"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("withdraw web: %v", err)
	}

	mu.Lock()
	sentCopy := append([]sentRoute{}, sent...)
	mu.Unlock()

	// Only web withdrawal should have been sent
	if len(sentCopy) != 1 {
		t.Fatalf("sent %d routes, want 1", len(sentCopy))
	}
	if sentCopy[0].cmd != "withdraw-web" {
		t.Errorf("cmd = %q, want withdraw-web", sentCopy[0].cmd)
	}

	// Verify pool states are independent: dns still announced, web withdrawn
	dnsAnnounced := mgr.peerPools["10.0.0.1"].AnnouncedForPeer("dns", "10.0.0.1")
	webAnnounced := mgr.peerPools["10.0.0.1"].AnnouncedForPeer("web", "10.0.0.1")
	if len(dnsAnnounced) != 1 {
		t.Errorf("dns announced = %d, want 1", len(dnsAnnounced))
	}
	if len(webAnnounced) != 0 {
		t.Errorf("web announced = %d, want 0", len(webAnnounced))
	}
}

// VALIDATES: AC-4 — Explicit withdraw of non-initial route survives reconnect
// PREVENTS: Withdrawn routes resent after peer flap

func TestExplicitWithdrawSurvivesReconnect(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	// Route is NOT initiallyAnnounced — requires explicit command
	entry := NewPoolEntry("10.0.0.0/24#0",
		"update text origin igp nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
		"update text nlri ipv4/unicast del 10.0.0.0/24")
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dnsr", entry); err != nil {
		t.Fatal(err)
	}

	// First session: peer comes up, nothing sent (not initiallyAnnounced)
	mgr.handleStateUp("10.0.0.1")

	// Explicit announce
	_, _, err := mgr.handleCommand("watchdog announce", []string{"dnsr"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("announce: %v", err)
	}

	mu.Lock()
	sent = nil
	mu.Unlock()

	// Explicit withdraw
	_, _, err = mgr.handleCommand("watchdog withdraw", []string{"dnsr"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}

	mu.Lock()
	sent = nil // Clear the withdraw send
	mu.Unlock()

	// Peer flaps: down → up
	mgr.handleStateDown("10.0.0.1")
	mgr.handleStateUp("10.0.0.1")

	mu.Lock()
	defer mu.Unlock()

	// Route was explicitly withdrawn and not initiallyAnnounced — must NOT resend
	if len(sent) != 0 {
		t.Fatalf("sent %d routes after reconnect, want 0 (explicitly withdrawn)", len(sent))
	}
}

// VALIDATES: initiallyAnnounced routes are re-announced on every reconnect
// PREVENTS: Config default routes lost after peer flap

func TestInitiallyAnnouncedRestoredOnReconnect(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	entry := NewPoolEntry("10.0.0.0/24#0",
		"update text origin igp nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
		"update text nlri ipv4/unicast del 10.0.0.0/24")
	entry.initiallyAnnounced = true
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dnsr", entry); err != nil {
		t.Fatal(err)
	}

	// First session: auto-announced
	mgr.handleStateUp("10.0.0.1")

	mu.Lock()
	if len(sent) != 1 {
		t.Fatalf("first up: sent %d, want 1", len(sent))
	}
	sent = nil
	mu.Unlock()

	// Explicit withdraw during session
	_, _, err := mgr.handleCommand("watchdog withdraw", []string{"dnsr"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}

	mu.Lock()
	sent = nil
	mu.Unlock()

	// Peer flaps — initiallyAnnounced should be restored
	mgr.handleStateDown("10.0.0.1")
	mgr.handleStateUp("10.0.0.1")

	mu.Lock()
	defer mu.Unlock()

	// initiallyAnnounced routes are restored on every reconnect, even after explicit withdraw
	if len(sent) != 1 {
		t.Fatalf("reconnect: sent %d, want 1 (initiallyAnnounced restored)", len(sent))
	}
	if !strings.Contains(sent[0].cmd, "add 10.0.0.0/24") {
		t.Errorf("cmd = %q, want announce command", sent[0].cmd)
	}
}

// VALIDATES: AC-5 — Full cycle: up→announce→down→up resends
// PREVENTS: Routes lost across peer session lifecycle

func TestReconnectResendAfterEstablished(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	// Route starts withdrawn (not initiallyAnnounced)
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dnsr",
		NewPoolEntry("10.0.0.0/24#0", "announce-cmd", "withdraw-cmd")); err != nil {
		t.Fatal(err)
	}

	// Phase 1: peer comes up, nothing sent (initially withdrawn)
	mgr.handleStateUp("10.0.0.1")

	mu.Lock()
	if len(sent) != 0 {
		t.Fatalf("phase 1: sent %d, want 0", len(sent))
	}
	mu.Unlock()

	// Phase 2: explicit announce
	_, _, err := mgr.handleCommand("watchdog announce", []string{"dnsr"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("announce: %v", err)
	}

	mu.Lock()
	if len(sent) != 1 {
		t.Fatalf("phase 2: sent %d, want 1", len(sent))
	}
	sent = nil
	mu.Unlock()

	// Phase 3: peer flaps down → up
	mgr.handleStateDown("10.0.0.1")
	mgr.handleStateUp("10.0.0.1")

	mu.Lock()
	defer mu.Unlock()

	// Should resend the explicitly-announced route
	if len(sent) != 1 {
		t.Fatalf("phase 3: sent %d, want 1 (resend on reconnect)", len(sent))
	}
	if sent[0].cmd != "announce-cmd" {
		t.Errorf("cmd = %q, want announce-cmd", sent[0].cmd)
	}
}

// VALIDATES: AC-6 — Wildcard on nonexistent pool returns success with 0 peers
// PREVENTS: Error or panic when no peers have the requested pool

func TestWildcardNonexistentPool(t *testing.T) {
	mgr := newWatchdogServer(func(_, _ string) {})

	// Add peers with pools, but none have "missing-pool"
	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dns",
		NewPoolEntry("10.0.0.0/24#0", "a", "w")); err != nil {
		t.Fatal(err)
	}
	mgr.peerUp["10.0.0.1"] = true

	status, data, err := mgr.handleCommand("watchdog announce", []string{"missing-pool"}, "*")
	if err != nil {
		t.Fatalf("handleCommand: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want done", status)
	}
	if !strings.Contains(data, `"peers":0`) {
		t.Errorf("data = %q, want 0 peers affected", data)
	}
}

// newTestEntryWithRoute creates a PoolEntry with a Route for MED override tests.
func newTestEntryWithRoute(announceCmd, withdrawCmd string, med *uint32) *PoolEntry {
	entry := NewPoolEntry("10.0.0.0/24#0", announceCmd, withdrawCmd)
	entry.Route = bgp.Route{
		Origin:  "igp",
		NextHop: "1.2.3.4",
		Family:  "ipv4/unicast",
		Prefix:  "10.0.0.0/24",
		MED:     med,
	}
	return entry
}

// VALIDATES: AC-1 — MED override clones route and formats with new MED
// PREVENTS: MED override modifies stored Route or uses stored AnnounceCmd

func TestMEDOverrideProducesOneOffCommand(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	configMED := uint32(100)
	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	entry := newTestEntryWithRoute(
		"update text origin igp med 100 nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
		"update text nlri ipv4/unicast del 10.0.0.0/24",
		&configMED)
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dns", entry); err != nil {
		t.Fatal(err)
	}
	mgr.peerUp["10.0.0.1"] = true

	// Announce with MED override to 500
	status, _, err := mgr.handleCommand("watchdog announce", []string{"dns", "med", "500"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("handleCommand: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want done", status)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 1 {
		t.Fatalf("sent %d routes, want 1", len(sent))
	}
	// Must contain med 500, not med 100
	if !strings.Contains(sent[0].cmd, "med 500") {
		t.Errorf("cmd = %q, want med 500", sent[0].cmd)
	}
	if strings.Contains(sent[0].cmd, "med 100") {
		t.Errorf("cmd = %q, must not contain config MED 100", sent[0].cmd)
	}

	// Verify stored Route unchanged
	pool := mgr.peerPools["10.0.0.1"].GetPool("dns")
	storedEntry := pool.Routes()[0]
	if *storedEntry.Route.MED != 100 {
		t.Errorf("stored Route.MED = %d, want 100 (unchanged)", *storedEntry.Route.MED)
	}
}

// VALIDATES: AC-2 — Two consecutive MED overrides both produce sendRoute calls
// PREVENTS: Second MED override deduped because route is "already announced"

func TestMEDOverrideBypassesDedup(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	configMED := uint32(100)
	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dns",
		newTestEntryWithRoute(
			"update text origin igp med 100 nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
			"update text nlri ipv4/unicast del 10.0.0.0/24",
			&configMED)); err != nil {
		t.Fatal(err)
	}
	mgr.peerUp["10.0.0.1"] = true

	// First MED override
	_, _, err := mgr.handleCommand("watchdog announce", []string{"dns", "med", "100"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("first announce: %v", err)
	}
	// Second MED override with different value
	_, _, err = mgr.handleCommand("watchdog announce", []string{"dns", "med", "1000"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("second announce: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 2 {
		t.Fatalf("sent %d routes, want 2 (both MED overrides)", len(sent))
	}
	if !strings.Contains(sent[0].cmd, "med 100") {
		t.Errorf("sent[0].cmd = %q, want med 100", sent[0].cmd)
	}
	if !strings.Contains(sent[1].cmd, "med 1000") {
		t.Errorf("sent[1].cmd = %q, want med 1000", sent[1].cmd)
	}
}

// VALIDATES: AC-3 — Non-MED announce deduped when already announced
// PREVENTS: Redundant sends on repeated non-MED announce

func TestNoMEDPreservesDedup(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dns",
		NewPoolEntry("10.0.0.0/24#0", "announce-cmd", "withdraw-cmd")); err != nil {
		t.Fatal(err)
	}
	mgr.peerUp["10.0.0.1"] = true

	// First announce (no MED)
	_, _, err := mgr.handleCommand("watchdog announce", []string{"dns"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("first announce: %v", err)
	}
	// Second announce (no MED, already announced)
	_, _, err = mgr.handleCommand("watchdog announce", []string{"dns"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("second announce: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 1 {
		t.Fatalf("sent %d routes, want 1 (second deduped)", len(sent))
	}
}

// VALIDATES: AC-4 — MED override on withdrawn route sends and sets announced
// PREVENTS: MED override fails when route is in withdrawn state

func TestMEDOverrideFromWithdrawn(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	configMED := uint32(100)
	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dns",
		newTestEntryWithRoute(
			"update text origin igp med 100 nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
			"update text nlri ipv4/unicast del 10.0.0.0/24",
			&configMED)); err != nil {
		t.Fatal(err)
	}
	mgr.peerUp["10.0.0.1"] = true
	// Route starts withdrawn (not announced for this peer)

	status, _, err := mgr.handleCommand("watchdog announce", []string{"dns", "med", "500"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("handleCommand: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want done", status)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 1 {
		t.Fatalf("sent %d routes, want 1", len(sent))
	}
	if !strings.Contains(sent[0].cmd, "med 500") {
		t.Errorf("cmd = %q, want med 500", sent[0].cmd)
	}
}

// VALIDATES: AC-5 — MED override with wildcard peer
// PREVENTS: MED override not dispatched to all peers

func TestMEDOverrideWildcard(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	configMED := uint32(100)
	for _, peer := range []string{"10.0.0.1", "10.0.0.2"} {
		mgr.peerPools[peer] = NewPoolSet()
		if err := mgr.peerPools[peer].AddRoute("dns",
			newTestEntryWithRoute(
				"update text origin igp med 100 nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
				"update text nlri ipv4/unicast del 10.0.0.0/24",
				&configMED)); err != nil {
			t.Fatal(err)
		}
		mgr.peerUp[peer] = true
	}

	_, _, err := mgr.handleCommand("watchdog announce", []string{"dns", "med", "500"}, "*")
	if err != nil {
		t.Fatalf("handleCommand: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 2 {
		t.Fatalf("sent %d routes, want 2 (one per peer)", len(sent))
	}
	for _, s := range sent {
		if !strings.Contains(s.cmd, "med 500") {
			t.Errorf("cmd = %q, want med 500", s.cmd)
		}
	}
}

// VALIDATES: AC-9 — MED override with explicit peer arg
// PREVENTS: Peer arg after med <N> ignored

func TestMEDOverrideWithExplicitPeer(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	configMED := uint32(100)
	for _, peer := range []string{"10.0.0.1", "10.0.0.2"} {
		mgr.peerPools[peer] = NewPoolSet()
		if err := mgr.peerPools[peer].AddRoute("dns",
			newTestEntryWithRoute(
				"update text origin igp med 100 nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
				"update text nlri ipv4/unicast del 10.0.0.0/24",
				&configMED)); err != nil {
			t.Fatal(err)
		}
		mgr.peerUp[peer] = true
	}

	// MED override with explicit peer -- should only go to 10.0.0.1
	status, _, err := mgr.handleCommand("watchdog announce", []string{"dns", "med", "500", "10.0.0.1"}, "*")
	if err != nil {
		t.Fatalf("handleCommand: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want done", status)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 1 {
		t.Fatalf("sent %d routes, want 1 (explicit peer only)", len(sent))
	}
	if sent[0].peer != "10.0.0.1" {
		t.Errorf("peer = %q, want 10.0.0.1", sent[0].peer)
	}
	if !strings.Contains(sent[0].cmd, "med 500") {
		t.Errorf("cmd = %q, want med 500", sent[0].cmd)
	}
}

// VALIDATES: AC-6 — "med" rejected as group name
// PREVENTS: Ambiguity between group name "med" and med keyword

func TestMEDGroupNameRejected(t *testing.T) {
	mgr := newWatchdogServer(func(_, _ string) {})
	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	mgr.peerUp["10.0.0.1"] = true

	status, _, err := mgr.handleCommand("watchdog announce", []string{"med"}, "10.0.0.1")
	if err == nil {
		t.Fatal("expected error for group name 'med'")
	}
	if status != statusError {
		t.Errorf("status = %q, want error", status)
	}
}

// VALIDATES: AC-7, AC-8 — Invalid MED values
// PREVENTS: Non-numeric or missing MED accepted

func TestMEDInvalidValue(t *testing.T) {
	mgr := newWatchdogServer(func(_, _ string) {})
	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dns",
		NewPoolEntry("10.0.0.0/24#0", "a", "w")); err != nil {
		t.Fatal(err)
	}
	mgr.peerUp["10.0.0.1"] = true

	tests := []struct {
		name string
		args []string
	}{
		{"non-numeric", []string{"dns", "med", "abc"}},
		{"missing value", []string{"dns", "med"}},
		{"negative", []string{"dns", "med", "-1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _, err := mgr.handleCommand("watchdog announce", tt.args, "10.0.0.1")
			if err == nil {
				t.Fatal("expected error")
			}
			if status != statusError {
				t.Errorf("status = %q, want error", status)
			}
		})
	}
}

// VALIDATES: AC-10 — Non-MED announce deduped after MED override
// PREVENTS: Non-MED announce re-sends after MED override set announced=true

func TestNoMEDAfterMEDOverride(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	configMED := uint32(100)
	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dns",
		newTestEntryWithRoute(
			"update text origin igp med 100 nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
			"update text nlri ipv4/unicast del 10.0.0.0/24",
			&configMED)); err != nil {
		t.Fatal(err)
	}
	mgr.peerUp["10.0.0.1"] = true

	// MED override
	_, _, _ = mgr.handleCommand("watchdog announce", []string{"dns", "med", "500"}, "10.0.0.1")
	// Non-MED announce (should be deduped because announced=true after MED override)
	_, _, _ = mgr.handleCommand("watchdog announce", []string{"dns"}, "10.0.0.1")

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 1 {
		t.Fatalf("sent %d routes, want 1 (non-MED deduped)", len(sent))
	}
}

// VALIDATES: AC-11 — Withdraw after MED override uses stored WithdrawCmd
// PREVENTS: Withdraw broken after MED override

func TestWithdrawAfterMEDOverride(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	configMED := uint32(100)
	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dns",
		newTestEntryWithRoute(
			"update text origin igp med 100 nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
			"update text nlri ipv4/unicast del 10.0.0.0/24",
			&configMED)); err != nil {
		t.Fatal(err)
	}
	mgr.peerUp["10.0.0.1"] = true

	// MED override announce
	_, _, _ = mgr.handleCommand("watchdog announce", []string{"dns", "med", "500"}, "10.0.0.1")
	// Withdraw
	_, _, err := mgr.handleCommand("watchdog withdraw", []string{"dns"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 2 {
		t.Fatalf("sent %d routes, want 2", len(sent))
	}
	if !strings.Contains(sent[1].cmd, "del 10.0.0.0/24") {
		t.Errorf("withdraw cmd = %q, want del", sent[1].cmd)
	}
}

// VALIDATES: AC-12 — Reconnect uses stored AnnounceCmd, not overridden MED
// PREVENTS: Transient MED override persists across reconnect

func TestReconnectUsesStoredNotOverride(t *testing.T) {
	var sent []sentRoute
	var mu sync.Mutex
	mgr := newWatchdogServer(func(peer, cmd string) {
		mu.Lock()
		sent = append(sent, sentRoute{peer, cmd})
		mu.Unlock()
	})

	configMED := uint32(100)
	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	entry := newTestEntryWithRoute(
		"update text origin igp med 100 nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
		"update text nlri ipv4/unicast del 10.0.0.0/24",
		&configMED)
	entry.initiallyAnnounced = true
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dns", entry); err != nil {
		t.Fatal(err)
	}

	// First up
	mgr.handleStateUp("10.0.0.1")
	// MED override
	_, _, _ = mgr.handleCommand("watchdog announce", []string{"dns", "med", "500"}, "10.0.0.1")

	mu.Lock()
	sent = nil
	mu.Unlock()

	// Reconnect
	mgr.handleStateDown("10.0.0.1")
	mgr.handleStateUp("10.0.0.1")

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 1 {
		t.Fatalf("sent %d routes on reconnect, want 1", len(sent))
	}
	// Must use stored AnnounceCmd (med 100), not override (med 500)
	if !strings.Contains(sent[0].cmd, "med 100") {
		t.Errorf("reconnect cmd = %q, want stored med 100", sent[0].cmd)
	}
	if strings.Contains(sent[0].cmd, "med 500") {
		t.Errorf("reconnect cmd = %q, must not contain override med 500", sent[0].cmd)
	}
}

// VALIDATES: AC-15, AC-16 — MED boundary values
// PREVENTS: Overflow or invalid large MED values accepted

func TestMEDOverrideBoundary(t *testing.T) {
	mgr := newWatchdogServer(func(_, _ string) {})
	mgr.peerPools["10.0.0.1"] = NewPoolSet()
	configMED := uint32(100)
	if err := mgr.peerPools["10.0.0.1"].AddRoute("dns",
		newTestEntryWithRoute("a", "w", &configMED)); err != nil {
		t.Fatal(err)
	}
	mgr.peerUp["10.0.0.1"] = true

	// Max uint32 should work
	status, _, err := mgr.handleCommand("watchdog announce", []string{"dns", "med", "4294967295"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("max uint32: %v", err)
	}
	if status != statusDone {
		t.Errorf("max uint32 status = %q, want done", status)
	}

	// Overflow should fail
	status, _, err = mgr.handleCommand("watchdog announce", []string{"dns", "med", "4294967296"}, "10.0.0.1")
	if err == nil {
		t.Fatal("expected error for overflow")
	}
	if status != statusError {
		t.Errorf("overflow status = %q, want error", status)
	}
}

// sentRoute records a route sent to the engine for test assertions.
type sentRoute struct {
	peer string
	cmd  string
}

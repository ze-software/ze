package bgp_watchdog

import (
	"strings"
	"sync"
	"testing"
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

	status, data, err := mgr.handleCommand("bgp watchdog announce", []string{"dnsr"}, "10.0.0.1")
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

	status, _, err := mgr.handleCommand("bgp watchdog withdraw", []string{"dnsr"}, "10.0.0.2")
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

	status, _, err := mgr.handleCommand("bgp watchdog announce", []string{"nonexistent"}, "10.0.0.1")
	if err == nil {
		t.Fatal("expected error for nonexistent group")
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
	status, _, err := mgr.handleCommand("bgp watchdog announce", []string{"dnsr"}, "10.0.0.1")
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
	_, _, err := mgr.handleCommand("bgp watchdog announce", []string{"dnsr"}, "10.0.0.4")
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

// sentRoute records a route sent to the engine for test assertions.
type sentRoute struct {
	peer string
	cmd  string
}

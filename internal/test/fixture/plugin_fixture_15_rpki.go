package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func plugin15RPKIPassthrough(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin15RPKIReady(ctx, p, false); err != nil {
		return err
	}
	r := plugin15PollCommand(ctx, p, "show bgp adj-rib-in", 40, 250*time.Millisecond, func(r plugin15Result) bool {
		return plugin15Done(r) && strings.Contains(r.text(), "0.0.0.0")
	})
	if !plugin15Done(r) {
		return fmt.Errorf("rib routes received status=%s: %s", r.status, r.text())
	}
	text := r.text()
	if !strings.Contains(text, "0.0.0.0") {
		return fmt.Errorf("default route not in adj-rib-in: %s", text)
	}
	if strings.Contains(text, `"validation-state":4`) || strings.Contains(text, `"validation-state": 4`) {
		return fmt.Errorf("route stuck in pending state without RPKI plugin: %s", text)
	}
	fmt.Fprintln(os.Stderr, "OK: route stored immediately without RPKI (passthrough)")
	return nil
}

func plugin15RPKICount(ctx context.Context, p *sdk.Plugin) float64 {
	r := plugin15Dispatch(ctx, p, "show bgp rpki status")
	m, err := plugin15Map(r)
	if err != nil {
		return -1
	}
	count, ok := m["vrp-count-ipv4"].(float64)
	if !ok {
		return -1
	}
	return count
}

func plugin15UpdatesReceived(ctx context.Context, p *sdk.Plugin) float64 {
	r := plugin15Dispatch(ctx, p, "show bgp peer peer1 detail")
	m, _ := plugin15Map(r)
	peer := plugin15NestedMap(m, "peers", "127.0.0.1")
	return plugin15Number(peer["updates-received"])
}

func plugin15RPKIReady(ctx context.Context, p *sdk.Plugin, requireVRP bool) error {
	if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
		if plugin15UpdatesReceived(ctx, p) < 1 {
			return false
		}
		return !requireVRP || plugin15RPKICount(ctx, p) >= 1
	}) {
		if requireVRP {
			return fmt.Errorf("the peer UPDATE and RTR cache never both became ready")
		}
		return fmt.Errorf("ze never received the peer UPDATE")
	}
	if r := plugin15Dispatch(ctx, p, "request quiesce"); !plugin15Done(r) {
		return fmt.Errorf("quiesce failed: status=%s data=%s", r.status, r.text())
	}
	return nil
}

func plugin15RPKIPerPeer(ctx context.Context, p *sdk.Plugin) error {
	if !Poll(ctx, 150, 200*time.Millisecond, func() bool { return plugin15RPKICount(ctx, p) >= 1 }) {
		return fmt.Errorf("RTR never synced a VRP")
	}
	if !Poll(ctx, 150, 200*time.Millisecond, func() bool { return plugin15UpdatesReceived(ctx, p) >= 1 }) {
		return fmt.Errorf("ze never received the UPDATE")
	}
	if r := plugin15Dispatch(ctx, p, "request quiesce"); !plugin15Done(r) {
		return fmt.Errorf("quiesce failed: %s", r.text())
	}
	if err := plugin14AssertRouteValidation(ctx, p, "10.0.1.0/24", 3, false, "per-peer accept override"); err != nil {
		return err
	}
	status, err := plugin15Map(plugin15Dispatch(ctx, p, "show bgp rpki status"))
	if err != nil {
		return err
	}
	rows, _ := status["peer-actions"].([]any)
	var entry map[string]any
	for _, row := range rows {
		m, _ := row.(map[string]any)
		if m["peer"] == addrLoopback {
			entry = m
			break
		}
	}
	if entry == nil {
		return fmt.Errorf("peer-actions missing 127.0.0.1: %v", rows)
	}
	invalid, _ := entry["invalid"].(map[string]any)
	if invalid["action"] != actionAccept || invalid["source"] != sourcePeer {
		return fmt.Errorf("peer 127.0.0.1 invalid action/source wrong: %v", invalid)
	}
	notFound, _ := entry["not-found"].(map[string]any)
	if notFound["source"] != "global" {
		return fmt.Errorf("peer 127.0.0.1 not-found should inherit global: %v", notFound)
	}
	fmt.Fprintln(os.Stderr, "OK: Invalid route accepted via per-peer override; status reports peer-actions")
	return nil
}

func plugin15RPKILateSync(ctx context.Context, p *sdk.Plugin) error {
	if !Poll(ctx, 24, 500*time.Millisecond, func() bool { return plugin15UpdatesReceived(ctx, p) >= 1 }) {
		return fmt.Errorf("ze never received the UPDATE")
	}
	if err := plugin14AssertRouteValidation(ctx, p, "10.0.1.0/24", 2, false, "before cache synchronization"); err != nil {
		return err
	}
	count := plugin15RPKICount(ctx, p)
	if count < 0 {
		return fmt.Errorf("the pre-sync RPKI status did not report its VRP count")
	}
	if count != 0 {
		return fmt.Errorf("the cache synced before the UPDATE arrived, so this run does not exercise re-validation")
	}
	before, err := plugin15Map(plugin15Dispatch(ctx, p, "show bgp adj-rib-in"))
	if err != nil {
		return err
	}
	original := plugin14FindRoute(before, "10.0.1.0/24")
	if !plugin14RouteValidation(before, "10.0.1.0/24", 2, false) {
		return fmt.Errorf("the pre-sync route changed before releasing the cache: %v", before)
	}
	if err := os.WriteFile("rpki-sync.release", nil, 0o600); err != nil {
		return fmt.Errorf("release the initial RTR synchronization: %w", err)
	}
	if !Poll(ctx, 24, 500*time.Millisecond, func() bool { return plugin15RPKICount(ctx, p) >= 1 }) {
		return fmt.Errorf("the RTR cache never synced a VRP")
	}
	if err := plugin14AssertRouteValidation(ctx, p, "10.0.1.0/24", 3, true, "after cache synchronization"); err != nil {
		return err
	}
	after, err := plugin15Map(plugin15Dispatch(ctx, p, "show bgp adj-rib-in"))
	if err != nil {
		return err
	}
	retained := plugin14FindRoute(after, "10.0.1.0/24")
	if !plugin14RouteValidation(after, "10.0.1.0/24", 3, true) {
		return fmt.Errorf("the post-sync route lost its validation result: %v", after)
	}
	for _, key := range []string{"attr-hex", "nhop-hex", "nlri-hex"} {
		if retained[key] != original[key] {
			return fmt.Errorf("RPKI revalidation changed received %s: before=%v after=%v", key, original[key], retained[key])
		}
	}
	fmt.Fprintln(os.Stderr, "OK re-validation retained 10.0.1.0/24 as ineligible once the cache synced")
	return nil
}

// plugin15RPKIRevalidateRelay proves that a relay plugin (bgp-rr or bgp-rs)
// delivers a path whose RPKI verdict flips AFTER the path is held.
//
// Method: once the destination peer (127.0.0.2) has its first session and its
// End-of-RIB, the observer holds it administratively down (`request peer
// disable`) and only then lets the source peer (127.0.0.1) announce
// 10.0.0.0/24 origin AS 65001. The VRP set is still empty, so the route
// validates NotFound and `not-found reject` holds it ineligible. A teardown
// would not do: ze dials a torn-down peer again with no wait (peer_run.go,
// the ErrTeardown branch), so the second session raced the source UPDATE and
// took its live forward, an ineligible announcement sent as a withdrawal. The
// observer releases the hold (`request peer enable`) only once the route is
// judged ineligible, and the destination's second session then carries only
// its End-of-RIB, because peer-up replay leaves an ineligible path out. The
// observer then releases the RTR cache (10.0.0.0/8 maxlen 24 AS 65001), the
// route becomes Valid, and only the relay plugin's revalidation relay can
// send it. The destination's ze-peer pins the bytes;
// this observer waits for ze to count the UPDATE sent, so it does not shut ze
// down first.
func plugin15RPKIRevalidateRelay(ctx context.Context, p *sdk.Plugin) error {
	const prefix = "10.0.0.0/24"
	if !Poll(ctx, 60, 500*time.Millisecond, func() bool {
		return plugin15PeerCount(ctx, p, "destination", "127.0.0.2", "connections-established") >= 1
	}) {
		return fmt.Errorf("the destination's first session never came up")
	}
	if !plugin15DestinationSettled(ctx, p) {
		return fmt.Errorf("the UPDATEs sent on the destination's first session never stopped")
	}
	if r := plugin15Dispatch(ctx, p, "request peer 127.0.0.2 disable revalidation relay test"); !plugin15Done(r) {
		return fmt.Errorf("hold the destination down: status=%s data=%s", r.status, r.text())
	}
	// The source sends only now, while the destination is held down, so the
	// live forward of the ineligible path reaches no destination session.
	if err := os.WriteFile("source.go", nil, 0o600); err != nil {
		return fmt.Errorf("release the source UPDATE: %w", err)
	}
	if !Poll(ctx, 30, 200*time.Millisecond, func() bool { return plugin15PeerCount(ctx, p, "source", "127.0.0.1", "updates-received") >= 1 }) {
		return fmt.Errorf("ze never received the source UPDATE")
	}
	if err := plugin14AssertRouteValidation(ctx, p, prefix, 2, true, "before cache synchronization"); err != nil {
		return err
	}
	if count := plugin15RPKICount(ctx, p); count != 0 {
		return fmt.Errorf("the VRP count before release is %v, not 0, so this run does not exercise re-validation", count)
	}
	if plugin15PeerCount(ctx, p, "destination", "127.0.0.2", "connections-established") >= 2 {
		return fmt.Errorf("the destination's second session came up before the route was judged ineligible")
	}
	if r := plugin15Dispatch(ctx, p, "request peer 127.0.0.2 enable"); !plugin15Done(r) {
		return fmt.Errorf("release the destination hold: status=%s data=%s", r.status, r.text())
	}
	if !Poll(ctx, 60, 500*time.Millisecond, func() bool {
		return plugin15PeerCount(ctx, p, "destination", "127.0.0.2", "connections-established") >= 2
	}) {
		return fmt.Errorf("the destination's second session never came up")
	}
	if !plugin15DestinationSettled(ctx, p) {
		return fmt.Errorf("the UPDATEs sent on the destination's second session never stopped")
	}
	baseline := plugin15PeerSent(ctx, p)
	if err := os.WriteFile("rpki-sync.release", nil, 0o600); err != nil {
		return fmt.Errorf("release the initial RTR synchronization: %w", err)
	}
	if !Poll(ctx, 24, 500*time.Millisecond, func() bool { return plugin15RPKICount(ctx, p) >= 1 }) {
		return fmt.Errorf("the RTR cache never synced a VRP")
	}
	if err := plugin14AssertRouteValidation(ctx, p, prefix, 1, false, "after cache synchronization"); err != nil {
		return err
	}
	if !Poll(ctx, 30, 500*time.Millisecond, func() bool { return plugin15PeerSent(ctx, p) > baseline }) {
		return fmt.Errorf("the destination never received the revalidated path %s: %v UPDATEs sent before the release, %v after", prefix, baseline, plugin15PeerSent(ctx, p))
	}
	fmt.Fprintln(os.Stderr, "OK the revalidated path reached the destination")
	return nil
}

// plugin15RPKIPendingWithdrawRelay proves that a relay plugin (bgp-rr or
// bgp-rs) sends nothing at all for a path withdrawn while RPKI still holds it,
// neither the announcement nor a withdrawal, even after a cache update would
// have made it Valid (spec-exabgp-6-api-compatibility, OD-2).
//
// Method: once the destination peer (127.0.0.2) has its session and its
// End-of-RIB, the observer lets the source peer (127.0.0.1) send its two
// UPDATEs back to back: 10.0.0.0/24 and 10.0.1.0/24 in one announcement, then
// the withdrawal of 10.0.0.0/24 alone. Both prefixes arrive pending; the VRP
// set is empty, so the verdict is NotFound and `not-found reject` holds them.
// Whether the withdrawal lands before or after that verdict depends on the
// RPKI batch timer, and the answer owed is the same: the destination was never
// advertised 10.0.0.0/24, so it is sent no withdrawal (RFC 4271 Section 3.1),
// and the withdrawal cancels the pending or held path, so no later verdict
// announces it. The observer checks the destination was sent nothing, then
// releases 10.0.0.0/8 maxlen 24 AS 65001. The cache change makes 10.0.1.0/24
// Valid, and the revalidation relay that would carry 10.0.0.0/24 had it
// survived carries 10.0.1.0/24: that announcement is the positive fence, and
// the destination MUST receive exactly it. Its ze-peer pins the bytes.
func plugin15RPKIPendingWithdrawRelay(ctx context.Context, p *sdk.Plugin) error {
	const withdrawn, fence = "10.0.0.0/24", "10.0.1.0/24"
	if !Poll(ctx, 60, 500*time.Millisecond, func() bool {
		return plugin15PeerCount(ctx, p, "destination", "127.0.0.2", "connections-established") >= 1
	}) {
		return fmt.Errorf("the destination's session never came up")
	}
	if !plugin15DestinationSettled(ctx, p) {
		return fmt.Errorf("the UPDATEs sent on the destination's session never stopped")
	}
	baseline := plugin15PeerSent(ctx, p)
	if err := os.WriteFile("source.go", nil, 0o600); err != nil {
		return fmt.Errorf("release the source UPDATEs: %w", err)
	}
	if !Poll(ctx, 30, 200*time.Millisecond, func() bool {
		return plugin15PeerCount(ctx, p, "source", "127.0.0.1", "updates-received") >= 2
	}) {
		return fmt.Errorf("ze never received both source UPDATEs, the announcement and the withdrawal")
	}
	if err := plugin14AssertRouteValidation(ctx, p, fence, 2, true, "before cache synchronization"); err != nil {
		return err
	}
	if err := plugin15AssertRouteAbsent(ctx, p, withdrawn, "after its withdrawal"); err != nil {
		return err
	}
	if count := plugin15RPKICount(ctx, p); count != 0 {
		return fmt.Errorf("the VRP count before release is %v, not 0, so this run does not hold the routes for RPKI", count)
	}
	if !plugin15DestinationSettled(ctx, p) {
		return fmt.Errorf("the UPDATEs sent to the destination after the source UPDATEs never stopped")
	}
	if sent := plugin15PeerSent(ctx, p); sent != baseline {
		return fmt.Errorf("the destination was sent %v UPDATEs for paths RPKI still held, want none", sent-baseline)
	}
	if err := os.WriteFile("rpki-sync.release", nil, 0o600); err != nil {
		return fmt.Errorf("release the initial RTR synchronization: %w", err)
	}
	if !Poll(ctx, 24, 500*time.Millisecond, func() bool { return plugin15RPKICount(ctx, p) >= 1 }) {
		return fmt.Errorf("the RTR cache never synced a VRP")
	}
	if err := plugin14AssertRouteValidation(ctx, p, fence, 1, false, "after cache synchronization"); err != nil {
		return err
	}
	if !Poll(ctx, 30, 500*time.Millisecond, func() bool { return plugin15PeerSent(ctx, p) > baseline }) {
		return fmt.Errorf("the destination never received the revalidated fence path %s", fence)
	}
	if !plugin15DestinationSettled(ctx, p) {
		return fmt.Errorf("the UPDATEs sent to the destination after the cache release never stopped")
	}
	if sent := plugin15PeerSent(ctx, p); sent != baseline+1 {
		return fmt.Errorf("the destination was sent %v UPDATEs after the cache release, want 1, the fence %s alone", sent-baseline, fence)
	}
	if err := plugin15AssertRouteAbsent(ctx, p, withdrawn, "after cache synchronization"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK the destination was sent only the fence path; the withdrawn path reached it as nothing")
	return nil
}

// plugin15AssertRouteAbsent waits for Adj-RIB-In to hold no path for prefix.
func plugin15AssertRouteAbsent(ctx context.Context, p *sdk.Plugin, prefix, label string) error {
	var last map[string]any
	if Poll(ctx, 20, 250*time.Millisecond, func() bool {
		m, err := plugin15Map(plugin15Dispatch(ctx, p, "show bgp adj-rib-in"))
		if err != nil {
			return false
		}
		last = m
		return plugin14FindRoute(m, prefix) == nil
	}) {
		return nil
	}
	return fmt.Errorf("%s is still in adj-rib-in %s: %v", prefix, label, last)
}

// plugin15DestinationSettled reports whether the destination's UPDATE and
// End-of-RIB counters held still for one second, within 20 seconds.
func plugin15DestinationSettled(ctx context.Context, p *sdk.Plugin) bool {
	read := func() [2]float64 {
		return [2]float64{
			plugin15PeerCount(ctx, p, "destination", "127.0.0.2", "updates-sent"),
			plugin15PeerCount(ctx, p, "destination", "127.0.0.2", "eor-sent"),
		}
	}
	last := read()
	return Poll(ctx, 20, time.Second, func() bool {
		now := read()
		settled := now == last
		last = now
		return settled
	})
}

// plugin15PeerSent counts the UPDATEs ze sent the destination, End-of-RIB excluded.
func plugin15PeerSent(ctx context.Context, p *sdk.Plugin) float64 {
	updates := plugin15PeerCount(ctx, p, "destination", "127.0.0.2", "updates-sent")
	eor := plugin15PeerCount(ctx, p, "destination", "127.0.0.2", "eor-sent")
	return updates - eor
}

// plugin15PeerCount reads one counter of `show bgp peer <name> detail`.
func plugin15PeerCount(ctx context.Context, p *sdk.Plugin, name, address, counter string) float64 {
	m, _ := plugin15Map(plugin15Dispatch(ctx, p, "show bgp peer "+name+" detail")) //nolint:errcheck // an unreadable answer reads as a zero count, which every caller polls past or refuses
	return plugin15Number(plugin15NestedMap(m, "peers", address)[counter])
}

func plugin15RPKITimeout(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin15RPKIReady(ctx, p, false); err != nil {
		return err
	}
	r := plugin15PollCommand(ctx, p, "show bgp adj-rib-in", 60, 500*time.Millisecond, func(r plugin15Result) bool {
		text := r.text()
		return plugin15Done(r) && strings.Contains(text, "10.0.1.0/24") && !strings.Contains(text, `"validation-state":4`) && !strings.Contains(text, `"validation-state": 4`)
	})
	if !plugin15Done(r) {
		return fmt.Errorf("rib routes received status=%s: %s", r.status, r.text())
	}
	text := r.text()
	if !strings.Contains(text, "10.0.1.0/24") {
		return fmt.Errorf("route 10.0.1.0/24 not promoted after timeout: %s", text)
	}
	if strings.Contains(text, `"validation-state":4`) || strings.Contains(text, `"validation-state": 4`) {
		return fmt.Errorf("route 10.0.1.0/24 still pending (state 4) after timeout: %s", text)
	}
	fmt.Fprintln(os.Stderr, "OK: route 10.0.1.0/24 promoted after timeout (fail-open)")
	return nil
}

func plugin15RPKIRouteState(ctx context.Context, p *sdk.Plugin, prefix string, state int, success string) error {
	if err := plugin15RPKIReady(ctx, p, true); err != nil {
		return err
	}
	if err := plugin14AssertRouteValidation(ctx, p, prefix, state, false, "origin validation"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, success)
	return nil
}

// plugin15RPKICacheFailover proves cache preference end to end through what an operator reads.
//
// Method: two `le mock rtr` caches hold different sets. The preferred cache 127.0.0.1
// (preference 10) holds 10.0.0.0/8 maxlen 24 AS 65001, so the peer's 10.0.1.0/24 from AS 65001
// is Valid. The standby 127.0.0.2 (preference 20) holds the same prefix for AS 65999, so on its
// data the route is Invalid and `invalid reject` makes it ineligible. Each step waits for the
// state the operator sees (the route's validation state and eligibility, and which cache
// `show bgp rpki status` names as synced) before it moves the next cache: the observer writes
// rpki-preferred.down to take the preferred cache down only after Ze uses it, and
// rpki-preferred.up to bring it back only after Ze failed over to the standby.
func plugin15RPKICacheFailover(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin15RPKIReady(ctx, p, true); err != nil {
		return err
	}
	steps := []struct {
		label      string
		state      int
		ineligible bool
		synced     string
		release    string
	}{
		{"on the preferred cache", 1, false, addrLoopback, "rpki-preferred.down"},
		{"after failover to the standby cache", 3, true, addrLoopbackSecond, "rpki-preferred.up"},
		{"after fail-back to the preferred cache", 1, false, addrLoopback, ""},
	}
	for _, step := range steps {
		if err := plugin15AssertSyncedCache(ctx, p, step.synced, step.label); err != nil {
			return err
		}
		if err := plugin14AssertRouteValidation(ctx, p, "10.0.1.0/24", step.state, step.ineligible, step.label); err != nil {
			return err
		}
		if step.release == "" {
			continue
		}
		if err := os.WriteFile(step.release, nil, 0o600); err != nil {
			return fmt.Errorf("%s: write %s: %w", step.label, step.release, err)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: 10.0.1.0/24 Valid on the preferred cache, Invalid on the standby, Valid again after fail-back")
	return nil
}

// plugin15AssertSyncedCache waits until `show bgp rpki status` lists exactly one synced cache
// server, the one at address, and returns an error naming the last rows it read when it never
// does.
func plugin15AssertSyncedCache(ctx context.Context, p *sdk.Plugin, address, label string) error {
	var rows []any
	if Poll(ctx, 40, 250*time.Millisecond, func() bool {
		status, err := plugin15Map(plugin15Dispatch(ctx, p, "show bgp rpki status"))
		if err != nil {
			return false
		}
		rows, _ = status["cache-servers"].([]any)
		var synced []any
		for _, row := range rows {
			server, _ := row.(map[string]any)
			if server["synced"] == true {
				synced = append(synced, server["address"])
			}
		}
		return len(synced) == 1 && synced[0] == address
	}) {
		return nil
	}
	return fmt.Errorf("%s: show bgp rpki status never named %s as the one synced cache: %v", label, address, rows)
}

func plugin15RPKIAccept(ctx context.Context, p *sdk.Plugin) error {
	return plugin15RPKIRouteState(ctx, p, "10.0.1.0/24", 1, "OK: route 10.0.1.0/24 accepted with validation-state=Valid")
}

func plugin15RPKINotFound(ctx context.Context, p *sdk.Plugin) error {
	return plugin15RPKIRouteState(ctx, p, "192.168.0.0/24", 2, "OK: route 192.168.0.0/24 accepted with validation-state=NotFound")
}

func plugin15RPKIBatch(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin15RPKIReady(ctx, p, true); err != nil {
		return err
	}
	for _, route := range []struct {
		prefix     string
		state      int
		ineligible bool
	}{
		{"10.0.1.0/24", 1, false},
		{"10.0.2.0/24", 1, false},
		{"10.0.3.0/24", 3, true},
		{"192.168.1.0/24", 2, false},
	} {
		if err := plugin14AssertRouteValidation(ctx, p, route.prefix, route.state, route.ineligible, "batch validation"); err != nil {
			return err
		}
	}
	fmt.Fprintln(os.Stderr, "OK: batch validation correct: 2 Valid accepted, 1 Invalid rejected, 1 NotFound accepted")
	return nil
}

func plugin15RPKIReject(ctx context.Context, p *sdk.Plugin) error {
	if !Poll(ctx, 150, 200*time.Millisecond, func() bool { return plugin15RPKICount(ctx, p) >= 1 }) {
		return fmt.Errorf("RTR never synced a VRP")
	}
	if !Poll(ctx, 150, 200*time.Millisecond, func() bool { return plugin15UpdatesReceived(ctx, p) >= 1 }) {
		return fmt.Errorf("ze never received the UPDATE")
	}
	if r := plugin15Dispatch(ctx, p, "request quiesce"); !plugin15Done(r) {
		return fmt.Errorf("quiesce failed: %s", r.text())
	}
	if err := plugin14AssertRouteValidation(ctx, p, "10.0.1.0/24", 3, true, "origin rejection"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: route 10.0.1.0/24 correctly rejected (Invalid)")
	return nil
}

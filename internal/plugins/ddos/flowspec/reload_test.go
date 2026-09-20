// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- responder handover at a config apply
// Related: register.go (replaceResponder), responder.go (adoptAnnouncement, retire)

package flowspec

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/ddosevent"
)

// enforcingConfig is the config every handover case in this file starts from:
// enforce, blackhole-fallback on so one AttackDetected announces, and the probe
// timings the YANG defaults name.
func enforcingConfig() *Config {
	return &Config{
		ResponseLevel: responseEnforce, Action: actionDiscard, BlackholeFallback: true,
		HoldDown: 300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000,
		BackoffCap: 3600, AnnounceRateLimit: 10, MaxMitigationDuration: 3600,
	}
}

// announceOnce drives one critical AttackDetected through r, which the
// blackhole fallback turns into exactly one upstream announcement.
func announceOnce(t *testing.T, r *responder, prefix string) {
	t.Helper()
	r.onDetected(&ddosevent.AttackDetected{
		Interface: "xe0",
		Target:    ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix(prefix), Proto: 17},
		Family:    ddosevent.FamilyUDPFlood,
		Severity:  ddosevent.SeverityCritical,
		Direction: ddosevent.DirectionRemote,
	})
	if active, _, _ := r.status(); !active {
		t.Fatalf("setup: responder did not announce for %s (commands: %v)", prefix, dispatched(r))
	}
}

func dispatched(r *responder) []string {
	f, ok := r.dispatcher.(*fakeDispatcher)
	if !ok {
		return nil
	}
	return f.cmds
}

func countWithdraws(cmds []string) int {
	n := 0
	for _, c := range cmds {
		if strings.Contains(c, " del ") {
			n++
		}
	}
	return n
}

// TestReplaceResponderCarriesTheLiveAnnouncement proves that an ordinary commit
// on a `ddos flowspec` leaf hands the live upstream rule to the responder that
// replaces the one which announced it, and that every removal path still
// reaches it afterwards.
//
// VALIDATES: replaceResponder -> adoptAnnouncement carries active, target,
// match, announcedAt and the running probe, so enforceMaxDuration and probeTick
// still own the rule after the apply.
// PREVENTS: the defect recorded in
// plan/journal/component-rebuilt-during-reload.md (2026-09-08). OnConfigApply
// built a fresh responder through newResponder, which leaves active false and
// announcedAt zero. The FlowSpec NLRI stayed in every peer's RIB while
// enforceMaxDuration, probeTick and onCleared all returned on !active, so the
// announcement was bounded by neither max-mitigation-duration nor a clear, for
// the life of the daemon, with `show ddos flowspec` reporting nothing
// announced.
func TestReplaceResponderCarriesTheLiveAnnouncement(t *testing.T) {
	disp := &fakeDispatcher{}
	first := newResponder(enforcingConfig(), disp)
	announceOnce(t, first, "192.0.2.0/24")

	announcedAt := time.Now().Add(-30 * time.Second)
	first.announcedAt = announcedAt
	firstMatch := first.match

	second := replaceResponder(enforcingConfig(), disp, first, true)

	if second == first {
		t.Fatal("replaceResponder returned the responder it was asked to replace")
	}
	active, target, probing := second.status()
	if !active {
		t.Error("the announcement did not cross: the new responder reports nothing announced")
	}
	if target.DstPrefix.String() != "192.0.2.0/24" {
		t.Errorf("target did not cross: got %s, want 192.0.2.0/24", target.DstPrefix)
	}
	if !probing {
		t.Error("the leak probe did not cross: the new responder reports no probe running")
	}
	if second.match != firstMatch {
		t.Errorf("match did not cross: got %+v, want %+v", second.match, firstMatch)
	}
	if !second.announcedAt.Equal(announcedAt) {
		t.Errorf("announcedAt did not cross: got %v, want %v", second.announcedAt, announcedAt)
	}
	if n := countWithdraws(disp.cmds); n != 0 {
		t.Errorf("a carry must send no withdraw, sent %d (commands: %v)", n, disp.cmds)
	}

	// The outgoing responder must stop claiming the rule, or a late cap tick that
	// loaded the old pointer would withdraw a rule the new one owns.
	if active, _, _ := first.status(); active {
		t.Error("the outgoing responder still claims the announcement")
	}
	first.enforceMaxDuration()
	if n := countWithdraws(disp.cmds); n != 0 {
		t.Errorf("the retired responder withdrew the rule its successor owns (commands: %v)", disp.cmds)
	}

	// The cap must still reach the carried rule, counting from the FIRST
	// announcement rather than from the commit.
	second.cfg.MaxMitigationDuration = 10
	second.enforceMaxDuration()
	if n := countWithdraws(disp.cmds); n != 1 {
		t.Errorf("max-mitigation-duration did not reach the carried rule: %d withdraws (commands: %v)", n, disp.cmds)
	}
	if active, _, _ := second.status(); active {
		t.Error("the responder still reports an announcement after the cap withdrew it")
	}
}

// TestReplaceResponderWithdrawsWhenTheSectionIsRemoved proves that deleting the
// `ddos flowspec` block takes the live upstream rule off the wire.
//
// VALIDATES: replaceResponder withdraws through the OUTGOING responder when
// `configured` is false.
// PREVENTS: a `delete ddos flowspec` leaving the FlowSpec NLRI in every peer's
// RIB. The plugin is stopped as soon as the reload commits, so a carried
// announcement would have no responder left to withdraw it and no cap worker
// left to time it out.
func TestReplaceResponderWithdrawsWhenTheSectionIsRemoved(t *testing.T) {
	disp := &fakeDispatcher{}
	first := newResponder(enforcingConfig(), disp)
	announceOnce(t, first, "198.51.100.0/24")

	// A removal delivers an empty body, which ParseConfig reads as the defaults
	// with configured false. The config is therefore the DEFAULT one here, and
	// the verdict must come from `configured` alone.
	second := replaceResponder(DefaultConfig(), disp, first, false)

	if n := countWithdraws(disp.cmds); n != 1 {
		t.Errorf("a removed section must withdraw the announcement: %d withdraws (commands: %v)", n, disp.cmds)
	}
	if active, _, _ := second.status(); active {
		t.Error("the new responder claims an announcement after the section was removed")
	}
	if active, _, _ := first.status(); active {
		t.Error("the outgoing responder still claims a withdrawn announcement")
	}
}

// TestReplaceResponderWithdrawsWhenResponseLevelLeavesEnforce proves that
// committing `response-level alert` lifts the upstream rule at once.
//
// VALIDATES: replaceResponder withdraws through the outgoing responder when the
// new config is not enforcing.
// PREVENTS: an upstream discard outliving the commit that asked for it to end
// by up to max-mitigation-duration, 3600 seconds by default, because neither
// enforceMaxDuration nor probeTick reads response-level.
func TestReplaceResponderWithdrawsWhenResponseLevelLeavesEnforce(t *testing.T) {
	disp := &fakeDispatcher{}
	first := newResponder(enforcingConfig(), disp)
	announceOnce(t, first, "203.0.113.0/24")

	alerting := enforcingConfig()
	alerting.ResponseLevel = "alert"
	second := replaceResponder(alerting, disp, first, true)

	if n := countWithdraws(disp.cmds); n != 1 {
		t.Errorf("leaving enforce must withdraw the announcement: %d withdraws (commands: %v)", n, disp.cmds)
	}
	if active, _, _ := second.status(); active {
		t.Error("the new responder claims an announcement after response-level left enforce")
	}
}

// TestReplaceResponderRetiresTheResponderItReplaces proves that an event
// already in flight when the apply started cannot put a second rule on the
// wire.
//
// VALIDATES: replaceResponder calls retire on the outgoing responder, and
// announce returns on retired.
// PREVENTS: a handler dispatched before OnConfigApply unsubscribed announcing
// through the replaced responder. Nothing left in the plugin reaches that
// responder, so the second FlowSpec NLRI could never be withdrawn.
func TestReplaceResponderRetiresTheResponderItReplaces(t *testing.T) {
	disp := &fakeDispatcher{}
	first := newResponder(enforcingConfig(), disp)

	second := replaceResponder(enforcingConfig(), disp, first, true)
	if second == nil {
		t.Fatal("replaceResponder returned nil")
	}

	before := len(disp.cmds)
	first.onDetected(&ddosevent.AttackDetected{
		Interface: "xe0",
		Target:    ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("192.0.2.0/24"), Proto: 17},
		Family:    ddosevent.FamilyUDPFlood,
		Severity:  ddosevent.SeverityCritical,
		Direction: ddosevent.DirectionRemote,
	})
	if len(disp.cmds) != before {
		t.Errorf("the retired responder announced: %v", disp.cmds[before:])
	}
	if active, _, _ := first.status(); active {
		t.Error("the retired responder claims an announcement it must not have made")
	}
}

// TestReplaceResponderCarriesTheAnnounceBudget proves that announce-rate-limit
// is not a limit an operator walks around by committing.
//
// VALIDATES: adoptAnnouncement carries the limiter's window across the
// handover.
// PREVENTS: every config apply handing the responder a fresh budget, so a
// commit loop could push unlimited FlowSpec churn onto peers the box does not
// own. The limit exists to protect those sessions, and a peer that drops the
// session over the churn loses every rule it holds.
func TestReplaceResponderCarriesTheAnnounceBudget(t *testing.T) {
	disp := &fakeDispatcher{}
	cfg := enforcingConfig()
	cfg.AnnounceRateLimit = 1
	first := newResponder(cfg, disp)
	announceOnce(t, first, "192.0.2.0/24")

	// The rule goes out, so the handover would otherwise carry it and the next
	// announce would be refused on !active rather than on the budget. Withdraw
	// first: the budget must survive even with nothing announced.
	first.mu.Lock()
	first.withdraw()
	first.mu.Unlock()

	next := enforcingConfig()
	next.AnnounceRateLimit = 1
	second := replaceResponder(next, disp, first, true)

	before := len(disp.cmds)
	announced := second.limiter.allow(time.Now())
	if announced {
		t.Error("the announce budget was reset by the config apply: a second announcement was allowed inside the window")
	}
	if len(disp.cmds) != before {
		t.Errorf("limiter.allow must dispatch nothing: %v", disp.cmds[before:])
	}
}

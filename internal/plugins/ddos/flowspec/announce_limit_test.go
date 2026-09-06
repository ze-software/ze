package flowspec

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/ddosevent"
)

// limitTestResponder builds an enforcing responder whose clock the test drives,
// with announce-rate-limit set to limit. It enters where an operator's config
// enters: newResponder is what register.go calls on every apply.
func limitTestResponder(t *testing.T, limit int, now *time.Time) (*responder, *fakeDispatcher) {
	t.Helper()
	disp := &fakeDispatcher{}
	r := newResponder(&Config{
		ResponseLevel: responseEnforce, Action: actionDiscard,
		HoldDown: 300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600,
		AnnounceRateLimit: limit,
	}, disp)
	r.now = func() time.Time { return *now }
	return r, disp
}

// limitTestVictim is the resolved victim every cycle below announces against.
// One address is enough: the limit is per responder and not per target.
const limitTestVictim = "192.0.2.9/32"

// characterize drives one full mitigation cycle: the responder announces on the
// characterized event, then the caller's withdraw returns it to idle so the next
// event can announce again. That announce/withdraw churn is the only way the
// announce rate rises, because announce refuses to run while active.
func characterize(r *responder) {
	r.onCharacterized(&ddosevent.AttackCharacterized{
		Interface: "xe0",
		Target:    ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix(limitTestVictim), Proto: 17},
		Family:    ddosevent.FamilyUDPFlood,
	})
}

// announceCount counts the FlowSpec announcements the responder sent upstream.
// renderFlowspecCommand writes the mode after the family, so "/flow add " is the
// announce and "/flow del " is the withdraw.
func announceCount(disp *fakeDispatcher) int {
	n := 0
	for _, cmd := range disp.cmds {
		if strings.Contains(cmd, "/flow add ") {
			n++
		}
	}
	return n
}

// VALIDATES: announce-rate-limit bounds the FlowSpec announcements Ze sends its
// peers in any 60-second window. With the limit at 2, a third announce inside
// the minute reaches no dispatcher.
// PREVENTS: the leaf being parsed, range-checked and read by nothing. No rate
// limiter existed anywhere in the plugin, so the responder announced as often as
// the detector asked it to, and the churn landed on BGP sessions Ze does not own.
func TestAnnounceRateLimitBoundsTheWindow(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	r, disp := limitTestResponder(t, 2, &now)

	// Three announce/withdraw cycles inside one minute. The dispatcher sees an
	// "announce" only while the responder still holds budget.
	for i := range 3 {
		now = now.Add(time.Second)
		characterize(r)
		r.mu.Lock()
		if r.active {
			r.withdraw()
		}
		r.mu.Unlock()
		if want := min(i+1, 2); announceCount(disp) != want {
			t.Fatalf("after cycle %d: %d announcements, want %d", i+1, announceCount(disp), want)
		}
	}

	// The last announcement of the window ages out 60 seconds after it was made,
	// and the budget it held returns with it.
	now = now.Add(61 * time.Second)
	characterize(r)
	if announceCount(disp) != 3 {
		t.Errorf("%d announcements after the window rolled, want 3: the budget must return", announceCount(disp))
	}
}

// VALIDATES: a refused announce leaves the responder idle, so it announces the
// moment the window frees rather than believing a rule is live upstream.
// PREVENTS: a refusal that marks the responder active. `show ddos flowspec`
// would report a rule the BGP engine never received, and withdraw would send a
// del for a route no peer holds.
func TestAnnounceRateLimitRefusalLeavesResponderIdle(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	r, disp := limitTestResponder(t, 1, &now)

	characterize(r)
	r.mu.Lock()
	r.withdraw()
	r.mu.Unlock()

	characterize(r)
	if active, _, _ := r.status(); active {
		t.Error("the responder reports an announced rule after a refused announce")
	}
	if announceCount(disp) != 1 {
		t.Errorf("%d announcements, want 1: the second was over the limit", announceCount(disp))
	}
}

// VALIDATES: newResponder takes the documented default when the Config carries
// no announce-rate-limit, rather than reading the zero as a budget of none.
// PREVENTS: a limiter that refuses every announce for a Config that never went
// through Validate. The zero would read as a broken responder, and it would
// disable upstream mitigation entirely (ai/rules/principles.md, "a zero value is
// never an answer").
func TestAnnounceRateLimitUnsetTakesTheDefault(t *testing.T) {
	r := newResponder(&Config{
		ResponseLevel: responseEnforce, Action: actionDiscard,
		HoldDown: 300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600,
	}, &fakeDispatcher{})
	if r.limiter.limit != DefaultConfig().AnnounceRateLimit {
		t.Errorf("limit = %d, want the documented default %d",
			r.limiter.limit, DefaultConfig().AnnounceRateLimit)
	}
}

// VALIDATES: the responder announces nothing when the detector resolved no
// victim, and the refusal consumes no announce-rate-limit budget.
// PREVENTS: a FlowSpec rule rendered from the zero prefix. Prefix.String() writes
// "invalid" for it, so the responder asked the BGP engine to originate
// `destination-ipv4 invalid` and the engine answered `ipv4/flow encode: invalid
// prefix` on every critical detection whose victim was unresolved. Observed in
// the daemon on 2026-09-06 (plan/journal/zero-value-as-valid-answer.md).
func TestAnnounceRefusesAnUnresolvedVictim(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	r, disp := limitTestResponder(t, 1, &now)
	r.cfg.BlackholeFallback = true

	// A critical detection with no victim: what the detector emits when no
	// traffic source can name one (detect/characterize.go characterizeAndEmit).
	r.onDetected(&ddosevent.AttackDetected{
		Interface: "xe0",
		Family:    ddosevent.FamilyGenericFlood,
		Severity:  ddosevent.SeverityCritical,
		Direction: ddosevent.DirectionRemote,
	})
	if len(disp.cmds) != 0 {
		t.Fatalf("the responder dispatched %q for an attack with no resolved victim", disp.cmds)
	}
	if active, _, _ := r.status(); active {
		t.Error("the responder reports an announced rule it never sent")
	}

	// The budget of one is intact, so the next resolved victim still announces.
	characterize(r)
	if announceCount(disp) != 1 {
		t.Errorf("%d announcements for the resolved victim, want 1: a refusal must consume no budget",
			announceCount(disp))
	}
}

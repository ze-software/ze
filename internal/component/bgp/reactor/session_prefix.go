// Design: docs/architecture/core-design.md — prefix limit enforcement (RFC 4486)
// Overview: session.go — Session struct and message processing loop
// Related: session_handlers.go — UPDATE handler calls prefix limit check

package reactor

import (
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// reportSourceBGP is the report bus source name for BGP-originated issues.
const reportSourceBGP = "bgp"

// reportCodePrefixThreshold is the report bus code for "per-family prefix
// count is at or above the configured warning threshold".
const reportCodePrefixThreshold = "prefix-threshold"

// reportCodePrefixStale is the report bus code for "PrefixUpdated date is
// older than stalenessThreshold (180 days)".
const reportCodePrefixStale = "prefix-stale"

// reportCodePrefixHold is the report bus code for "a prefix limit stopped this
// session and the peer is held down until an operator acts". The Subject is the
// peer address: one peer holds for one family at a time, and the family is in
// the message.
const reportCodePrefixHold = "prefix-hold"

// reportCodeNotificationSent is the report bus code for "this ze instance
// sent a BGP NOTIFICATION to a peer". The Subject is the peer address.
const reportCodeNotificationSent = "notification-sent"

// reportCodeNotificationReceived is the report bus code for "this ze instance
// received a BGP NOTIFICATION from a peer". The Subject is the peer address.
const reportCodeNotificationReceived = "notification-received"

// reportCodeSessionDropped is the report bus code for "an Established BGP
// session ended without a NOTIFICATION exchange (hold-timer expiry, TCP
// loss, peer FIN)". The Subject is the peer address.
const reportCodeSessionDropped = "session-dropped"

// reportCodeRouteCountAnomaly is the report bus code for "received prefix
// count dropped >50% in a single UPDATE". This is an error (event, not state)
// because the drop already happened even if the count recovers.
const reportCodeRouteCountAnomaly = "route-count-anomaly"

// minRouteCountAnomalyThreshold is the minimum aggregate prefix count before
// the >50% drop anomaly check activates. Small peer tables (route-server
// clients, management peers) routinely fluctuate by large percentages without
// indicating a real problem.
const minRouteCountAnomalyThreshold int64 = 100

// raiseNotificationError pushes a notification-sent or notification-received
// error event onto the report bus. dir is "sent" or "received".
func raiseNotificationError(dir, peerAddr string, code, subcode uint8) {
	var reportCode string
	if dir == "sent" {
		reportCode = reportCodeNotificationSent
	} else {
		reportCode = reportCodeNotificationReceived
	}
	var b textbuf.Buffer
	report.RaiseError(
		reportSourceBGP,
		reportCode,
		peerAddr,
		b.Reset().Str("BGP NOTIFICATION ").Str(dir).Str(" (code ").Int(int64(code)).Str(" subcode ").Int(int64(subcode)).Str(")").String(),
		map[string]any{"code": code, "subcode": subcode, "direction": dir},
	)
}

// raiseSessionDropped pushes a session-dropped error event onto the report
// bus. Called when the FSM leaves Established without a NOTIFICATION exchange,
// indicating an unexpected teardown (hold-timer expiry, TCP loss, peer FIN).
func raiseSessionDropped(peerAddr, reason string) {
	report.RaiseError(
		reportSourceBGP,
		reportCodeSessionDropped,
		peerAddr,
		"BGP session dropped: "+reason,
		map[string]any{"reason": reason},
	)
}

// prefixThresholdSubject builds the composite report bus Subject for a
// per-(peer, family) prefix-threshold warning. The format is "<addr>/<family>"
// so the bus dedups per family even though the bus key is (Source, Code, Subject).
func prefixThresholdSubject(peerAddr, family string) string {
	return peerAddr + "/" + family
}

// raisePrefixThreshold pushes a prefix-threshold warning onto the report bus.
// The producer is responsible for hot-path dedup (prefixCounts.warned), so
// this is called only on the upward edge.
func raisePrefixThreshold(peerAddr, fam string, count, warning, maximum uint32) {
	var b textbuf.Buffer
	report.RaiseWarning(
		reportSourceBGP,
		reportCodePrefixThreshold,
		prefixThresholdSubject(peerAddr, fam),
		b.Reset().Str(fam).Str(" prefix count ").Uint32(count).Str(" at or above warning threshold ").Uint32(warning).Str(" (max ").Uint32(maximum).Str(")").String(),
		map[string]any{
			"family":  fam,
			"count":   count,
			"warning": warning,
			"maximum": maximum,
		},
	)
}

// clearPrefixThreshold removes a prefix-threshold warning from the report bus.
// Called on the downward edge.
func clearPrefixThreshold(peerAddr, fam string) {
	report.ClearWarning(
		reportSourceBGP,
		reportCodePrefixThreshold,
		prefixThresholdSubject(peerAddr, fam),
	)
}

// raisePrefixHold pushes a prefix-hold warning onto the report bus when a peer
// is held down after a prefix-limit teardown. It stays raised for as long as the
// peer is held, which is until an operator recreates the peer, so `ze show
// warnings` and the login banner can tell a deliberate hold from a broken peer.
func raisePrefixHold(peerAddr, fam string) {
	var b textbuf.Buffer
	report.RaiseWarning(
		reportSourceBGP,
		reportCodePrefixHold,
		peerAddr,
		b.Reset().Str("held down: ").Str(fam).Str(" exceeded its prefix maximum and asks for no reconnect").String(),
		map[string]any{"family": fam, "reconnect": PrefixReconnectNever.String()},
	)
}

// clearPrefixHold removes any prefix-hold warning for a peer. Called from
// Peer.cleanup, so a peer that stops or is recreated leaves no stale warning.
func clearPrefixHold(peerAddr string) {
	report.ClearWarning(reportSourceBGP, reportCodePrefixHold, peerAddr)
}

// RaisePrefixStale pushes a prefix-stale warning for a peer if its
// PrefixUpdated date is older than stalenessThreshold. Otherwise it clears
// any existing prefix-stale warning for the peer. Called at peer add and
// peer config reload.
func RaisePrefixStale(peerAddr, prefixUpdated string, now time.Time) {
	if IsPrefixDataStale(prefixUpdated, now) {
		report.RaiseWarning(
			reportSourceBGP,
			reportCodePrefixStale,
			peerAddr,
			"prefix data updated "+prefixUpdated+" (>180 days old)",
			map[string]any{"updated": prefixUpdated},
		)
		return
	}
	report.ClearWarning(reportSourceBGP, reportCodePrefixStale, peerAddr)
}

// ClearPrefixStale removes any prefix-stale warning for a peer. Called on
// peer remove so cleared peers do not linger on the report bus.
func ClearPrefixStale(peerAddr string) {
	report.ClearWarning(reportSourceBGP, reportCodePrefixStale, peerAddr)
}

// raiseRouteCountAnomaly pushes a route-count-anomaly error event onto the
// report bus when the aggregate received prefix count drops >50% in a single
// UPDATE. This is an error (event) because the drop already happened.
func raiseRouteCountAnomaly(peerAddr string, before, after int64) {
	var b textbuf.Buffer
	report.RaiseError(
		reportSourceBGP,
		reportCodeRouteCountAnomaly,
		peerAddr,
		b.Reset().Str("received prefix count dropped from ").Int(before).Str(" to ").Int(after).Str(" (>50% in one update)").String(),
		map[string]any{"before": before, "after": after},
	)
}

// familyKey encodes an family.Family as a uint32 map key, avoiding fmt.Sprintf allocations
// on the hot path. Layout: AFI in upper 16 bits, SAFI in bits 8-15, lower 8 bits zero.
func familyKey(f family.Family) uint32 {
	return uint32(f.AFI)<<16 | uint32(f.SAFI)<<8
}

// familyKeyString converts a "afi/safi" config string to the uint32 key used by prefixCounts.
// Returns 0, false if the string is not a recognized family.
func familyKeyString(s string) (uint32, bool) {
	f, ok := family.LookupFamily(s)
	if !ok {
		return 0, false
	}
	return familyKey(f), true
}

// prefixCounts tracks the current number of received prefixes per family.
// Incremented on announced NLRIs, decremented on withdrawn NLRIs.
// Reset when the session is destroyed (Peer creates a new Session per connection).
// Keys are uint32 family keys (see familyKey) to avoid string allocations on the hot path.
type prefixCounts struct {
	counts map[uint32]int64
	warned map[uint32]bool // true once warning has been logged for a family (reset on drop below)

	// dropped holds, per family, how many announced prefixes warn-only
	// enforcement refused and never counted. Only a PrefixCountInstalled family
	// gets an entry. The peer keeps a refused prefix in its own Adj-RIB-Out and
	// withdraws it later, and planInstalledPrefixFamily spends the credit on
	// that withdrawal so the count does not fall below the number of routes the
	// session delivered. Lazily allocated: an offered-only peer never has one.
	dropped map[uint32]int64

	// installed holds the family keys whose `count` leaf asked for
	// PrefixCountInstalled. Nil when no family did, which is the check every
	// UPDATE makes before it does anything different. Resolved ONCE, when the
	// session is built: reading the mode by family name per UPDATE would build a
	// string on the wire path (ai/rules/performance.md).
	installed map[uint32]bool
}

// newPrefixCounts builds the per-session prefix tally from the peer settings.
func newPrefixCounts(settings *PeerSettings) *prefixCounts {
	return &prefixCounts{
		counts:    make(map[uint32]int64),
		warned:    make(map[uint32]bool),
		installed: installedPrefixFamilies(settings),
	}
}

// installedPrefixFamilies returns the family keys that asked for
// PrefixCountInstalled, or nil when none did.
func installedPrefixFamilies(settings *PeerSettings) map[uint32]bool {
	var out map[uint32]bool
	// The map is walked for its KEYS and read through the accessor, which is the
	// contract on the field (peersettings.go): an absent or zero value is the
	// offered mode, and no site decides that twice.
	for fam := range settings.PrefixCount {
		if settings.PrefixCountFor(fam) != PrefixCountInstalled {
			continue
		}
		fk, ok := familyKeyString(fam)
		if !ok {
			sessionLogger().Warn("prefix count: unrecognized family in config", "family", fam)
			continue
		}
		if out == nil {
			out = make(map[uint32]bool)
		}
		out[fk] = true
	}
	return out
}

// totalCount returns the sum of all per-family prefix counts.
// Used to snapshot aggregate count before/after an UPDATE for anomaly detection.
func (pc *prefixCounts) totalCount() int64 {
	var total int64
	for _, c := range pc.counts {
		total += c
	}
	return total
}

// add adjusts the count for a family by delta (positive for announce, negative for withdraw).
// Count is clamped to 0 (cannot go negative from withdraw-more-than-announced).
func (pc *prefixCounts) add(fk uint32, delta int64) int64 {
	pc.counts[fk] += delta
	if pc.counts[fk] < 0 {
		pc.counts[fk] = 0
	}
	return pc.counts[fk]
}

// ipv4AddPathReceive returns whether ADD-PATH receive is negotiated for IPv4 unicast.
func ipv4AddPathReceive(neg *capability.Negotiated) bool {
	if neg == nil {
		return false
	}
	mode := neg.AddPathMode(capability.Family{
		AFI:  capability.AFIIPv4,
		SAFI: capability.SAFIUnicast,
	})
	return mode == capability.AddPathReceive || mode == capability.AddPathBoth
}

// prefixDelta is one family section's change to the prefix count, taken from one
// UPDATE. checkPrefixLimits collects every section of a message before it counts
// any of them: a PrefixCountInstalled family has to know whether the message
// survives before the count moves.
type prefixDelta struct {
	fk    uint32
	delta int64
}

// maxPrefixDeltas is how many family sections one UPDATE can carry: IPv4 unicast
// withdrawn and announced from the body, plus one MP_UNREACH and one MP_REACH
// family. The array lives on the stack of checkPrefixLimits, so collecting the
// sections allocates nothing on the wire path.
const maxPrefixDeltas = 4

// collectPrefixDeltas reads the NLRI counts of every family section of one
// UPDATE into out, and returns how many entries it wrote. It changes no state.
//
// Withdrawals come first so that one UPDATE replacing prefixes (withdraw old,
// announce new) never reads as an overflow. Every partial sum of the entries is
// therefore at or below the final one, which is what lets the installed mode
// judge a family on its net change and reach the same verdict the section by
// section apply below would.
func (s *Session) collectPrefixDeltas(wu *wireu.WireUpdate, out *[maxPrefixDeltas]prefixDelta) int {
	n := 0

	// Determine ADD-PATH state for IPv4 unicast body NLRI parsing.
	addPath := ipv4AddPathReceive(s.negotiated)
	ipv4Key := familyKey(family.IPv4Unicast)

	// Count IPv4 unicast body Withdrawn.
	if withdrawn, err := countBodyWithdrawn(wu, addPath); err == nil && withdrawn > 0 {
		out[n] = prefixDelta{fk: ipv4Key, delta: -int64(withdrawn)}
		n++
	}

	// Count MP_UNREACH_NLRI (non-IPv4 families).
	if mpUnreach, err := wu.MPUnreach(); err == nil && mpUnreach != nil {
		fam := family.Family{
			AFI:  family.AFI(mpUnreach.AFI()),
			SAFI: family.SAFI(mpUnreach.SAFI()),
		}
		if wdBytes := mpUnreach.WithdrawnBytes(); len(wdBytes) > 0 {
			if count := countPrefixEntries(wdBytes, s.mpAddPathReceive(fam)); count > 0 {
				out[n] = prefixDelta{fk: familyKey(fam), delta: -int64(count)}
				n++
			}
		}
	}

	// Count IPv4 unicast body NLRI (announced).
	if announced, err := countBodyNLRI(wu, addPath); err == nil && announced > 0 {
		out[n] = prefixDelta{fk: ipv4Key, delta: int64(announced)}
		n++
	}

	// Count MP_REACH_NLRI (non-IPv4 families).
	if mpReach, err := wu.MPReach(); err == nil && mpReach != nil {
		fam := family.Family{
			AFI:  family.AFI(mpReach.AFI()),
			SAFI: family.SAFI(mpReach.SAFI()),
		}
		if nlriBytes := mpReach.NLRIBytes(); len(nlriBytes) > 0 {
			if count := countPrefixEntries(nlriBytes, s.mpAddPathReceive(fam)); count > 0 {
				out[n] = prefixDelta{fk: familyKey(fam), delta: int64(count)}
				n++
			}
		}
	}

	return n
}

// checkPrefixLimits counts NLRIs in the UPDATE and checks against configured limits.
// Returns:
//   - notif non-nil: maximum exceeded, teardown=true. Caller sends NOTIFICATION and closes.
//   - drop true: maximum exceeded, teardown=false. Caller skips plugin delivery (AC-27).
//   - both nil/false: within limits, proceed normally.
//
// An `offered` family (the default) counts every announced prefix, a prefix of
// an UPDATE this call then drops included, and its withdrawals are counted
// whatever the outcome. An `installed` family is settled first, by
// applyInstalledPrefixDeltas, and takes nothing from a dropped UPDATE.
//
// RFC 4486 Section 4: "Maximum Number of Prefixes Reached" -- Cease subcode 1.
func (s *Session) checkPrefixLimits(wu *wireu.WireUpdate) (notif *message.Notification, drop bool) {
	if s.prefixCounts == nil {
		return nil, false
	}

	hasLimits := len(s.settings.PrefixMaximum) > 0

	// Snapshot aggregate prefix count before this UPDATE for anomaly detection.
	totalBefore := s.prefixCounts.totalCount()
	defer func() {
		if notif != nil {
			return // teardown path, anomaly check not useful
		}
		totalAfter := s.prefixCounts.totalCount()
		if totalBefore >= minRouteCountAnomalyThreshold && totalAfter*2 < totalBefore {
			raiseRouteCountAnomaly(s.peerLabel(), totalBefore, totalAfter)
		}
	}()

	var buf [maxPrefixDeltas]prefixDelta
	deltas := buf[:s.collectPrefixDeltas(wu, &buf)]

	// Families that asked to count what ze installed are settled on the whole
	// message, before any of it is counted. A peer whose families all keep the
	// default never enters this branch.
	if len(s.prefixCounts.installed) > 0 {
		if n, d := s.applyInstalledPrefixDeltas(deltas, hasLimits); n != nil || d {
			return n, d
		}
	}

	for _, pd := range deltas {
		if s.prefixCounts.installed[pd.fk] {
			continue // already applied above
		}
		// Withdrawals only decrement and never trigger enforcement.
		// Without limits, applyPrefixDelta just counts for anomaly detection.
		if pd.delta < 0 || !hasLimits {
			s.applyPrefixDelta(pd.fk, pd.delta)
			continue
		}
		if n, d := s.applyPrefixCheck(pd.fk, pd.delta); n != nil || d {
			return n, d
		}
	}

	return nil, false
}

// installedPrefixPlan is one PrefixCountInstalled family's whole change from one
// UPDATE: what the count becomes, and how much of the withdrawal a refused
// announce already paid for.
type installedPrefixPlan struct {
	fk     uint32
	net    int64
	absorb int64
}

// applyInstalledPrefixDeltas settles every PrefixCountInstalled family of one
// UPDATE. It returns the same pair as applyPrefixCheck.
//
// The mode's whole content is here: a message this function refuses never moves
// the count, exactly as it never reaches the RIB (processMessage,
// session_read.go, returns before plugin delivery). So the families are judged
// first, on their net change, and only a message that every one of them accepts
// is counted.
//
// A refusal credits the refused announcement to prefixCounts.dropped. Without
// that credit the peer's later withdrawal of a prefix ze never counted would
// take the count BELOW the number of routes ze holds, and the family would then
// accept one more route past its maximum on every announce, drop, withdraw
// cycle. The credit makes the limit hold: planInstalledPrefixFamily spends it
// on that withdrawal instead.
//
// A message mixing an installed family with an offered one is refused whole:
// nothing is counted for either, and only the installed family is credited. The
// message reached no family's RIB, so counting one of them would report
// prefixes no reader can see.
func (s *Session) applyInstalledPrefixDeltas(deltas []prefixDelta, hasLimits bool) (*message.Notification, bool) {
	var planBuf [maxPrefixDeltas]installedPrefixPlan
	plans := planBuf[:0]

	for i, pd := range deltas {
		if !s.prefixCounts.installed[pd.fk] || containsPrefixFamily(deltas[:i], pd.fk) {
			continue
		}
		plan := s.planInstalledPrefixFamily(deltas, pd.fk)
		if !hasLimits {
			plans = append(plans, plan)
			continue
		}
		maximum, _, hasMax := s.prefixConfigLookup(pd.fk)
		if !hasMax || plan.net+s.prefixCounts.counts[pd.fk] <= int64(maximum) {
			plans = append(plans, plan)
			continue
		}
		// This family goes past its maximum, so the whole message is refused.
		famName := familyString(pd.fk)
		current := s.prefixCounts.counts[pd.fk] + plan.net
		notif, drop := s.reportPrefixExceeded(pd.fk, famName, current, maximum)
		if drop {
			s.creditDroppedPrefixes(deltas)
		}
		return notif, drop
	}

	for _, plan := range plans {
		if plan.absorb > 0 {
			s.prefixCounts.dropped[plan.fk] -= plan.absorb
		}
		switch {
		case plan.net < 0:
			s.applyPrefixDelta(plan.fk, plan.net)
		case plan.net > 0:
			// The maximum was checked above on this same net change, so this
			// call cannot refuse the message a second time. The answer is
			// propagated rather than discarded: a refusal here would mean two
			// readings of one number disagree, and enforcement wins over
			// tidiness whenever they do.
			if n, d := s.applyPrefixCheck(plan.fk, plan.net); n != nil || d {
				return n, d
			}
		}
	}
	return nil, false
}

// planInstalledPrefixFamily folds one family's sections of an UPDATE into the
// single change the installed mode applies, and states how much of the
// withdrawal the refused-announcement credit covers.
//
// The credit is spent on the withdrawal before the announcement is added,
// because the peer withdraws what ze refused earlier and what ze holds in the
// same message shape, and only the count can tell them apart.
func (s *Session) planInstalledPrefixFamily(deltas []prefixDelta, fk uint32) installedPrefixPlan {
	var withdrawn, announced int64
	for _, pd := range deltas {
		if pd.fk != fk {
			continue
		}
		if pd.delta < 0 {
			withdrawn -= pd.delta
			continue
		}
		announced += pd.delta
	}

	absorb := min(s.prefixCounts.dropped[fk], withdrawn)
	return installedPrefixPlan{fk: fk, net: announced - (withdrawn - absorb), absorb: absorb}
}

// creditDroppedPrefixes records every announcement a refused message carried,
// per installed family, so the peer's later withdrawal of those prefixes is
// absorbed instead of counted.
func (s *Session) creditDroppedPrefixes(deltas []prefixDelta) {
	for i, pd := range deltas {
		if pd.delta <= 0 || !s.prefixCounts.installed[pd.fk] || containsPrefixFamily(deltas[:i], pd.fk) {
			continue
		}
		var announced int64
		for _, other := range deltas {
			if other.fk == pd.fk && other.delta > 0 {
				announced += other.delta
			}
		}
		if s.prefixCounts.dropped == nil {
			s.prefixCounts.dropped = make(map[uint32]int64)
		}
		s.prefixCounts.dropped[pd.fk] += announced
	}
}

// containsPrefixFamily reports whether fk already appears in deltas. The caller
// uses it to settle each family once, on the first section that names it.
func containsPrefixFamily(deltas []prefixDelta, fk uint32) bool {
	for _, pd := range deltas {
		if pd.fk == fk {
			return true
		}
	}
	return false
}

// mpAddPathReceive returns whether ADD-PATH receive is negotiated for a given MP family.
func (s *Session) mpAddPathReceive(fam family.Family) bool {
	if s.negotiated == nil {
		return false
	}
	mode := s.negotiated.AddPathMode(fam)
	return mode == capability.AddPathReceive || mode == capability.AddPathBoth
}

// prefixConfigLookup resolves a uint32 family key against PrefixMaximum/PrefixWarning config maps.
// Config maps are keyed by "afi/safi" strings; this helper converts the numeric key to string
// only when a config lookup is needed (cold path).
func (s *Session) prefixConfigLookup(fk uint32) (maximum, warning uint32, hasMax bool) {
	for fam, max := range s.settings.PrefixMaximum {
		k, ok := familyKeyString(fam)
		if !ok {
			sessionLogger().Warn("prefix-maximum: unrecognized family in config", "family", fam)
			continue
		}
		if k == fk {
			maximum = max
			hasMax = true
			warning = s.settings.PrefixWarning[fam]
			return maximum, warning, hasMax
		}
	}
	return 0, 0, false
}

// familyString converts a uint32 family key back to "afi/safi" string for display/metrics.
// Only called on cold paths (logging, metrics, notifications).
func familyString(fk uint32) string {
	afi := family.AFI(fk >> 16)
	safi := family.SAFI((fk >> 8) & 0xFF)
	return family.Family{AFI: afi, SAFI: safi}.String()
}

// applyPrefixDelta adjusts a family's prefix count without checking thresholds.
// Used for withdrawals which only decrement and never trigger enforcement.
func (s *Session) applyPrefixDelta(fk uint32, delta int64) {
	current := s.prefixCounts.add(fk, delta)

	if s.prefixMetrics == nil && len(s.settings.PrefixWarning) == 0 {
		return
	}

	famName := familyString(fk)
	s.setPrefixCountMetric(famName, current)

	_, warning, _ := s.prefixConfigLookup(fk)
	if warning > 0 && current < int64(warning) {
		if s.prefixCounts.warned[fk] {
			s.prefixCounts.warned[fk] = false
			s.setPrefixWarningExceededMetric(famName, 0)
			clearPrefixThreshold(s.peerLabel(), famName)
		}
	}
}

// applyPrefixCheck adjusts a family's prefix count and checks thresholds.
// Returns (notif, false) for teardown, (nil, true) for drop-without-teardown, (nil, false) for OK.
func (s *Session) applyPrefixCheck(fk uint32, delta int64) (*message.Notification, bool) {
	current := s.prefixCounts.add(fk, delta)

	// Update Prometheus gauge (cold path -- string conversion OK).
	famName := familyString(fk)
	s.setPrefixCountMetric(famName, current)

	maximum, warning, hasMax := s.prefixConfigLookup(fk)
	if !hasMax {
		return nil, false
	}

	// Check warning threshold -- log once when crossing upward.
	if warning > 0 && current >= int64(warning) && current < int64(maximum) {
		if !s.prefixCounts.warned[fk] {
			s.prefixCounts.warned[fk] = true
			s.setPrefixWarningExceededMetric(famName, 1)
			raisePrefixThreshold(s.peerLabel(), famName, uint32(current), warning, maximum) //nolint:gosec // current bounded by maximum (uint32) before this branch
			sessionLogger().Warn("prefix count reached warning threshold",
				"peer", s.settings.Address,
				"family", famName,
				"count", current,
				"warning", warning,
				"maximum", maximum,
			)
		}
	}

	// Check maximum.
	if current > int64(maximum) {
		return s.reportPrefixExceeded(fk, famName, current, maximum)
	}

	return nil, false
}

// reportPrefixExceeded logs and meters one family going past its maximum, and
// returns the enforcement answer: a NOTIFICATION for teardown, drop=true for
// warn-only. Both count modes reach it, so the operator reads the same line
// whichever one the family asked for, and `count` is the number the mode
// governs.
func (s *Session) reportPrefixExceeded(fk uint32, famName string, current int64, maximum uint32) (*message.Notification, bool) {
	teardown := s.settings.PrefixTeardownFor(famName)
	s.incrPrefixExceededMetric(famName)
	sessionLogger().Error("prefix count exceeded maximum",
		"peer", s.settings.Address,
		"family", famName,
		"count", current,
		"maximum", maximum,
		"teardown", teardown,
	)

	if teardown {
		// Record which family made the decision. session_read.go reads it
		// to name the family in the teardown error, and peer_run.go reads
		// that family's own idle-timeout to size the reconnect delay.
		s.prefixExceededFamily = fk
		s.incrPrefixTeardownMetric()
		return buildPrefixNotification(fk, uint32(current)), false //nolint:gosec // Clamped by prefix maximum (uint32)
	}
	// AC-27: teardown=false. Return drop=true to skip plugin delivery.
	// NLRIs beyond maximum are not installed in RIB or forwarded.
	return nil, true
}

// ClearReportedWarnings emits report.ClearWarning for every prefix-threshold
// warning this session has raised. Called by Peer.runOnce in its teardown
// defer so warnings do not linger on the bus after the session ends.
//
// Walks prefixCounts.warned (the per-session dedup flag set) and clears the
// matching bus entries by composite subject.
//
// MUST be called only after the session's read goroutine has exited (i.e.,
// after Session.Run has returned). prefixCounts is documented as "only
// accessed from the session read goroutine", so calling this concurrently
// with that goroutine would race. The runOnce defer in peer_run.go is the
// only safe call site today.
func (s *Session) ClearReportedWarnings() {
	if s.prefixCounts == nil {
		return
	}
	peerAddr := s.peerLabel()
	for fk, warned := range s.prefixCounts.warned {
		if !warned {
			continue
		}
		clearPrefixThreshold(peerAddr, familyString(fk))
	}
}

// prefixLimitError names the family whose prefix maximum tore the session down.
//
// It WRAPS ErrPrefixLimitExceeded rather than replacing it, so every existing
// errors.Is(err, ErrPrefixLimitExceeded) on the reconnect path keeps matching.
// peer_run.go recovers the family with errors.As and reads that family's own
// idle-timeout: the YANG leaf is per family, so the delay must not come from a
// family that did not overflow.
type prefixLimitError struct {
	// Family is the "afi/safi" string of the family that exceeded its maximum.
	Family string
}

func (e *prefixLimitError) Error() string {
	var tb textbuf.Buffer
	return tb.Str("prefix limit exceeded for family ").Str(e.Family).String()
}

func (e *prefixLimitError) Unwrap() error { return ErrPrefixLimitExceeded }

// prefixTeardownCause builds the teardown error for the family recorded by the
// last applyPrefixCheck teardown decision. Called on the session read goroutine.
func (s *Session) prefixTeardownCause() error {
	return &prefixLimitError{Family: familyString(s.prefixExceededFamily)}
}

// buildPrefixNotification builds a Cease/MaxPrefixes NOTIFICATION.
// RFC 4486 Section 4: Data = AFI (2 bytes) + SAFI (1 byte) + count (4 bytes).
func buildPrefixNotification(fk, count uint32) *message.Notification {
	afi := uint16(fk >> 16)
	safi := uint8((fk >> 8) & 0xFF)
	notif := &message.Notification{
		ErrorCode:    message.NotifyCease,
		ErrorSubcode: message.NotifyCeaseMaxPrefixes,
	}
	data := make([]byte, 7)
	data[0] = byte(afi >> 8)
	data[1] = byte(afi)
	data[2] = safi
	data[3] = byte(count >> 24)
	data[4] = byte(count >> 16)
	data[5] = byte(count >> 8)
	data[6] = byte(count)
	notif.Data = data
	return notif
}

// countBodyNLRI counts IPv4 unicast NLRI entries in the UPDATE body.
func countBodyNLRI(wu *wireu.WireUpdate, addPath bool) (int, error) {
	iter, err := wu.NLRIIterator(addPath)
	if err != nil || iter == nil {
		return 0, err
	}
	count := 0
	for _, _, ok := iter.Next(); ok; _, _, ok = iter.Next() {
		count++
	}
	return count, nil
}

// countBodyWithdrawn counts IPv4 unicast withdrawn entries in the UPDATE body.
func countBodyWithdrawn(wu *wireu.WireUpdate, addPath bool) (int, error) {
	iter, err := wu.WithdrawnIterator(addPath)
	if err != nil || iter == nil {
		return 0, err
	}
	count := 0
	for _, _, ok := iter.Next(); ok; _, _, ok = iter.Next() {
		count++
	}
	return count, nil
}

// countPrefixEntries counts prefix entries in raw NLRI bytes.
// Works for families using standard prefix-length encoding (unicast, multicast).
// For complex families (VPN, flowspec), the count may be inaccurate but is
// bounded (cannot overcount due to prefix-length advancing).
func countPrefixEntries(data []byte, addPath bool) int {
	count := 0
	offset := 0
	for offset < len(data) {
		if addPath {
			if offset+4 > len(data) {
				break
			}
			offset += 4 // Skip path-ID
		}
		if offset >= len(data) {
			break
		}
		prefixLen := int(data[offset])
		offset++ // Skip prefix-length byte
		// Prefix bytes = ceil(prefixLen / 8)
		prefixBytes := (prefixLen + 7) / 8
		offset += prefixBytes
		if offset > len(data) {
			break // Truncated entry, stop counting
		}
		count++
	}
	return count
}

// --- Prometheus metric helpers ---
// All are no-ops when prefixMetrics is nil (metrics not enabled).

func (s *Session) peerLabel() string {
	return s.settings.Address.String()
}

func (s *Session) setPrefixCountMetric(family string, count int64) {
	if s.prefixMetrics == nil {
		return
	}
	s.prefixMetrics.prefixCount.With(s.peerLabel(), family).Set(float64(count))

	// Update ratio: count / maximum.
	if maximum, ok := s.settings.PrefixMaximum[family]; ok && maximum > 0 {
		s.prefixMetrics.prefixRatio.With(s.peerLabel(), family).Set(float64(count) / float64(maximum))
	}
}

func (s *Session) setPrefixWarningExceededMetric(family string, val float64) {
	if s.prefixMetrics == nil {
		return
	}
	s.prefixMetrics.prefixWarningExceeded.With(s.peerLabel(), family).Set(val)
}

func (s *Session) incrPrefixExceededMetric(family string) {
	if s.prefixMetrics == nil {
		return
	}
	s.prefixMetrics.prefixExceededTotal.With(s.peerLabel(), family).Inc()
}

func (s *Session) incrPrefixTeardownMetric() {
	if s.prefixMetrics == nil {
		return
	}
	s.prefixMetrics.prefixTeardownTotal.With(s.peerLabel()).Inc()
}

// setPrefixConfigMetrics publishes the static prefix configuration as Prometheus gauges.
// Called once when the peer is added to the reactor.
func setPrefixConfigMetrics(m *reactorMetrics, peerAddr string, settings *PeerSettings, now time.Time) {
	if m == nil {
		return
	}
	for fam, maximum := range settings.PrefixMaximum {
		m.prefixMaximum.With(peerAddr, fam).Set(float64(maximum))
	}
	for fam, warning := range settings.PrefixWarning {
		m.prefixWarning.With(peerAddr, fam).Set(float64(warning))
	}

	// Staleness: set metric based on the oldest per-family updated date, so the
	// gauge stays set while any one family is stale.
	setPrefixStaleMetric(m, peerAddr, settings.OldestPrefixUpdated(), now)
}

// stalenessThreshold is the age beyond which prefix data is considered stale.
const stalenessThreshold = 180 * 24 * time.Hour // 6 months

// IsPrefixDataStale reports whether a prefix updated timestamp is older than 6 months.
// Returns false for empty timestamps (manually configured, no staleness tracking).
func IsPrefixDataStale(updated string, now time.Time) bool {
	if updated == "" {
		return false
	}
	t, err := time.Parse(time.DateOnly, updated)
	if err != nil {
		return false
	}
	return now.Sub(t) > stalenessThreshold
}

// setPrefixStaleMetric sets the ze_bgp_prefix_stale gauge for a peer.
func setPrefixStaleMetric(m *reactorMetrics, peerAddr, updated string, now time.Time) {
	if m == nil {
		return
	}
	val := float64(0)
	if IsPrefixDataStale(updated, now) {
		val = 1
	}
	m.prefixStale.With(peerAddr).Set(val)
}

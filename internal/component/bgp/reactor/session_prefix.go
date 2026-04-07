// Design: docs/architecture/core-design.md — prefix limit enforcement (RFC 4486)
// Overview: session.go — Session struct and message processing loop
// Related: session_handlers.go — UPDATE handler calls prefix limit check

package reactor

import (
	"fmt"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/report"
)

// reportSourceBGP is the report bus source name for BGP-originated issues.
const reportSourceBGP = "bgp"

// reportCodePrefixThreshold is the report bus code for "per-family prefix
// count is at or above the configured warning threshold".
const reportCodePrefixThreshold = "prefix-threshold"

// reportCodePrefixStale is the report bus code for "PrefixUpdated date is
// older than stalenessThreshold (180 days)".
const reportCodePrefixStale = "prefix-stale"

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

// raiseNotificationError pushes a notification-sent or notification-received
// error event onto the report bus. dir is "sent" or "received".
func raiseNotificationError(dir, peerAddr string, code, subcode uint8) {
	var reportCode string
	if dir == "sent" {
		reportCode = reportCodeNotificationSent
	} else {
		reportCode = reportCodeNotificationReceived
	}
	report.RaiseError(
		reportSourceBGP,
		reportCode,
		peerAddr,
		fmt.Sprintf("BGP NOTIFICATION %s (code %d subcode %d)", dir, code, subcode),
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
		fmt.Sprintf("BGP session dropped: %s", reason),
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
	report.RaiseWarning(
		reportSourceBGP,
		reportCodePrefixThreshold,
		prefixThresholdSubject(peerAddr, fam),
		fmt.Sprintf("%s prefix count %d at or above warning threshold %d (max %d)", fam, count, warning, maximum),
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
			fmt.Sprintf("prefix data updated %s (>180 days old)", prefixUpdated),
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

// checkPrefixLimits counts NLRIs in the UPDATE and checks against configured limits.
// Returns:
//   - notif non-nil: maximum exceeded, teardown=true. Caller sends NOTIFICATION and closes.
//   - drop true: maximum exceeded, teardown=false. Caller skips plugin delivery (AC-27).
//   - both nil/false: within limits, proceed normally.
//
// Withdrawals are always counted (processed before announces) regardless of outcome.
//
// RFC 4486 Section 4: "Maximum Number of Prefixes Reached" -- Cease subcode 1.
func (s *Session) checkPrefixLimits(wu *wireu.WireUpdate) (notif *message.Notification, drop bool) {
	if s.prefixCounts == nil || len(s.settings.PrefixMaximum) == 0 {
		return nil, false
	}

	// Determine ADD-PATH state for IPv4 unicast body NLRI parsing.
	addPath := ipv4AddPathReceive(s.negotiated)

	// Process withdrawals BEFORE announces so that a single UPDATE replacing
	// prefixes (withdraw old + announce new) doesn't falsely exceed the limit.

	ipv4Key := familyKey(family.IPv4Unicast)

	// Count IPv4 unicast body Withdrawn.
	if withdrawn, err := countBodyWithdrawn(wu, addPath); err == nil && withdrawn > 0 {
		s.applyPrefixDelta(ipv4Key, -int64(withdrawn))
	}

	// Count MP_UNREACH_NLRI (non-IPv4 families).
	if mpUnreach, err := wu.MPUnreach(); err == nil && mpUnreach != nil {
		fam := family.Family{
			AFI:  family.AFI(mpUnreach.AFI()),
			SAFI: family.SAFI(mpUnreach.SAFI()),
		}
		mpAddPath := s.mpAddPathReceive(fam)
		if wdBytes := mpUnreach.WithdrawnBytes(); len(wdBytes) > 0 {
			count := countPrefixEntries(wdBytes, mpAddPath)
			if count > 0 {
				s.applyPrefixDelta(familyKey(fam), -int64(count))
			}
		}
	}

	// Count IPv4 unicast body NLRI (announced).
	if announced, err := countBodyNLRI(wu, addPath); err == nil && announced > 0 {
		if n, d := s.applyPrefixCheck(ipv4Key, int64(announced)); n != nil || d {
			return n, d
		}
	}

	// Count MP_REACH_NLRI (non-IPv4 families).
	if mpReach, err := wu.MPReach(); err == nil && mpReach != nil {
		fam := family.Family{
			AFI:  family.AFI(mpReach.AFI()),
			SAFI: family.SAFI(mpReach.SAFI()),
		}
		mpAddPath := s.mpAddPathReceive(fam)
		if nlriBytes := mpReach.NLRIBytes(); len(nlriBytes) > 0 {
			count := countPrefixEntries(nlriBytes, mpAddPath)
			if count > 0 {
				if n, d := s.applyPrefixCheck(familyKey(fam), int64(count)); n != nil || d {
					return n, d
				}
			}
		}
	}

	return nil, false
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

	// Update Prometheus gauge (cold path -- string conversion OK).
	famName := familyString(fk)
	s.setPrefixCountMetric(famName, current)

	// Reset warning flag and metric when count drops below threshold.
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
		s.incrPrefixExceededMetric(famName)
		sessionLogger().Error("prefix count exceeded maximum",
			"peer", s.settings.Address,
			"family", famName,
			"count", current,
			"maximum", maximum,
			"teardown", s.settings.PrefixTeardown,
		)

		if s.settings.PrefixTeardown {
			s.incrPrefixTeardownMetric()
			return buildPrefixNotification(fk, uint32(current)), false //nolint:gosec // Clamped by prefix maximum (uint32)
		}
		// AC-27: teardown=false. Return drop=true to skip plugin delivery.
		// NLRIs beyond maximum are not installed in RIB or forwarded.
		return nil, true
	}

	return nil, false
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

	// Staleness: set metric based on PrefixUpdated timestamp age.
	setPrefixStaleMetric(m, peerAddr, settings.PrefixUpdated, now)
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

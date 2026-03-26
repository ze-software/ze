// Design: docs/architecture/core-design.md — prefix limit enforcement (RFC 4486)
// Overview: session.go — Session struct and message processing loop
// Related: session_handlers.go — UPDATE handler calls prefix limit check

package reactor

import (
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
)

// familyKey encodes an nlri.Family as a uint32 map key, avoiding fmt.Sprintf allocations
// on the hot path. Layout: AFI in upper 16 bits, SAFI in bits 8-15, lower 8 bits zero.
func familyKey(f nlri.Family) uint32 {
	return uint32(f.AFI)<<16 | uint32(f.SAFI)<<8
}

// familyKeyString converts a "afi/safi" config string to the uint32 key used by prefixCounts.
// Returns 0, false if the string is not a recognized family.
func familyKeyString(s string) (uint32, bool) {
	f, ok := nlri.ParseFamily(s)
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

	ipv4Key := familyKey(nlri.IPv4Unicast)

	// Count IPv4 unicast body Withdrawn.
	if withdrawn, err := countBodyWithdrawn(wu, addPath); err == nil && withdrawn > 0 {
		s.applyPrefixDelta(ipv4Key, -int64(withdrawn))
	}

	// Count MP_UNREACH_NLRI (non-IPv4 families).
	if mpUnreach, err := wu.MPUnreach(); err == nil && mpUnreach != nil {
		family := nlri.Family{
			AFI:  nlri.AFI(mpUnreach.AFI()),
			SAFI: nlri.SAFI(mpUnreach.SAFI()),
		}
		mpAddPath := s.mpAddPathReceive(family)
		if wdBytes := mpUnreach.WithdrawnBytes(); len(wdBytes) > 0 {
			count := countPrefixEntries(wdBytes, mpAddPath)
			if count > 0 {
				s.applyPrefixDelta(familyKey(family), -int64(count))
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
		family := nlri.Family{
			AFI:  nlri.AFI(mpReach.AFI()),
			SAFI: nlri.SAFI(mpReach.SAFI()),
		}
		mpAddPath := s.mpAddPathReceive(family)
		if nlriBytes := mpReach.NLRIBytes(); len(nlriBytes) > 0 {
			count := countPrefixEntries(nlriBytes, mpAddPath)
			if count > 0 {
				if n, d := s.applyPrefixCheck(familyKey(family), int64(count)); n != nil || d {
					return n, d
				}
			}
		}
	}

	return nil, false
}

// mpAddPathReceive returns whether ADD-PATH receive is negotiated for a given MP family.
func (s *Session) mpAddPathReceive(family nlri.Family) bool {
	if s.negotiated == nil {
		return false
	}
	mode := s.negotiated.AddPathMode(family)
	return mode == capability.AddPathReceive || mode == capability.AddPathBoth
}

// prefixConfigLookup resolves a uint32 family key against PrefixMaximum/PrefixWarning config maps.
// Config maps are keyed by "afi/safi" strings; this helper converts the numeric key to string
// only when a config lookup is needed (cold path).
func (s *Session) prefixConfigLookup(fk uint32) (maximum uint32, warning uint32, hasMax bool) {
	for fam, max := range s.settings.PrefixMaximum {
		if k, ok := familyKeyString(fam); ok && k == fk {
			maximum = max
			hasMax = true
			// Look up matching warning.
			warning = s.settings.PrefixWarning[fam]
			return maximum, warning, hasMax
		}
	}
	return 0, 0, false
}

// familyString converts a uint32 family key back to "afi/safi" string for display/metrics.
// Only called on cold paths (logging, metrics, notifications).
func familyString(fk uint32) string {
	afi := nlri.AFI(fk >> 16)
	safi := nlri.SAFI((fk >> 8) & 0xFF)
	return nlri.Family{AFI: afi, SAFI: safi}.String()
}

// applyPrefixDelta adjusts a family's prefix count without checking thresholds.
// Used for withdrawals which only decrement and never trigger enforcement.
func (s *Session) applyPrefixDelta(fk uint32, delta int64) {
	current := s.prefixCounts.add(fk, delta)

	// Update Prometheus gauge (cold path -- string conversion OK).
	family := familyString(fk)
	s.setPrefixCountMetric(family, current)

	// Reset warning flag and metric when count drops below threshold.
	_, warning, _ := s.prefixConfigLookup(fk)
	if warning > 0 && current < int64(warning) {
		if s.prefixCounts.warned[fk] {
			s.prefixCounts.warned[fk] = false
			s.setPrefixWarningExceededMetric(family, 0)
			if s.prefixWarningNotifier != nil {
				s.prefixWarningNotifier(family, false)
			}
		}
	}
}

// applyPrefixCheck adjusts a family's prefix count and checks thresholds.
// Returns (notif, false) for teardown, (nil, true) for drop-without-teardown, (nil, false) for OK.
func (s *Session) applyPrefixCheck(fk uint32, delta int64) (*message.Notification, bool) {
	current := s.prefixCounts.add(fk, delta)

	// Update Prometheus gauge (cold path -- string conversion OK).
	family := familyString(fk)
	s.setPrefixCountMetric(family, current)

	maximum, warning, hasMax := s.prefixConfigLookup(fk)
	if !hasMax {
		return nil, false
	}

	// Check warning threshold -- log once when crossing upward.
	if warning > 0 && current >= int64(warning) && current < int64(maximum) {
		if !s.prefixCounts.warned[fk] {
			s.prefixCounts.warned[fk] = true
			s.setPrefixWarningExceededMetric(family, 1)
			if s.prefixWarningNotifier != nil {
				s.prefixWarningNotifier(family, true)
			}
			sessionLogger().Warn("prefix count reached warning threshold",
				"peer", s.settings.Address,
				"family", family,
				"count", current,
				"warning", warning,
				"maximum", maximum,
			)
		}
	}

	// Check maximum.
	if current > int64(maximum) {
		s.incrPrefixExceededMetric(family)
		sessionLogger().Error("prefix count exceeded maximum",
			"peer", s.settings.Address,
			"family", family,
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

// buildPrefixNotification builds a Cease/MaxPrefixes NOTIFICATION.
// RFC 4486 Section 4: Data = AFI (2 bytes) + SAFI (1 byte) + count (4 bytes).
func buildPrefixNotification(fk uint32, count uint32) *message.Notification {
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
	for family, maximum := range settings.PrefixMaximum {
		m.prefixMaximum.With(peerAddr, family).Set(float64(maximum))
	}
	for family, warning := range settings.PrefixWarning {
		m.prefixWarning.With(peerAddr, family).Set(float64(warning))
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

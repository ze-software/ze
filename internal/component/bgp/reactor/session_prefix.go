// Design: docs/architecture/core-design.md — prefix limit enforcement (RFC 4486)
// Overview: session.go — Session struct and message processing loop
// Related: session_handlers.go — UPDATE handler calls prefix limit check

package reactor

import (
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
)

// prefixCounts tracks the current number of received prefixes per family.
// Incremented on announced NLRIs, decremented on withdrawn NLRIs.
// Reset when the session is destroyed (Peer creates a new Session per connection).
type prefixCounts struct {
	counts map[string]int64
	warned map[string]bool // true once warning has been logged for a family (reset on drop below)
}

// add adjusts the count for a family by delta (positive for announce, negative for withdraw).
// Count is clamped to 0 (cannot go negative from withdraw-more-than-announced).
func (pc *prefixCounts) add(family string, delta int64) int64 {
	pc.counts[family] += delta
	if pc.counts[family] < 0 {
		pc.counts[family] = 0
	}
	return pc.counts[family]
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

	// Count IPv4 unicast body Withdrawn.
	if withdrawn, err := countBodyWithdrawn(wu, addPath); err == nil && withdrawn > 0 {
		s.applyPrefixDelta(nlri.IPv4Unicast.String(), -int64(withdrawn))
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
				s.applyPrefixDelta(family.String(), -int64(count))
			}
		}
	}

	// Count IPv4 unicast body NLRI (announced).
	if announced, err := countBodyNLRI(wu, addPath); err == nil && announced > 0 {
		if n, d := s.applyPrefixCheck(nlri.IPv4Unicast.String(), int64(announced)); n != nil || d {
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
				if n, d := s.applyPrefixCheck(family.String(), int64(count)); n != nil || d {
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

// applyPrefixDelta adjusts a family's prefix count without checking thresholds.
// Used for withdrawals which only decrement and never trigger enforcement.
func (s *Session) applyPrefixDelta(family string, delta int64) {
	current := s.prefixCounts.add(family, delta)

	// Update Prometheus gauge.
	s.setPrefixCountMetric(family, current)

	// Reset warning flag and metric when count drops below threshold.
	warning := s.settings.PrefixWarning[family]
	if warning > 0 && current < int64(warning) {
		if s.prefixCounts.warned[family] {
			s.prefixCounts.warned[family] = false
			s.setPrefixWarningExceededMetric(family, 0)
		}
	}
}

// applyPrefixCheck adjusts a family's prefix count and checks thresholds.
// Returns (notif, false) for teardown, (nil, true) for drop-without-teardown, (nil, false) for OK.
func (s *Session) applyPrefixCheck(family string, delta int64) (*message.Notification, bool) {
	current := s.prefixCounts.add(family, delta)

	// Update Prometheus gauge.
	s.setPrefixCountMetric(family, current)

	maximum, hasMax := s.settings.PrefixMaximum[family]
	if !hasMax {
		return nil, false
	}

	// Check warning threshold -- log once when crossing upward.
	warning := s.settings.PrefixWarning[family]
	if warning > 0 && current >= int64(warning) && current < int64(maximum) {
		if !s.prefixCounts.warned[family] {
			s.prefixCounts.warned[family] = true
			s.setPrefixWarningExceededMetric(family, 1)
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
			return buildPrefixNotification(family, uint32(current)), false //nolint:gosec // Clamped by prefix maximum (uint32)
		}
		// AC-27: teardown=false. Return drop=true to skip plugin delivery.
		// NLRIs beyond maximum are not installed in RIB or forwarded.
		return nil, true
	}

	return nil, false
}

// buildPrefixNotification builds a Cease/MaxPrefixes NOTIFICATION.
// RFC 4486 Section 4: Data = AFI (2 bytes) + SAFI (1 byte) + count (4 bytes).
func buildPrefixNotification(family string, count uint32) *message.Notification {
	f, ok := nlri.ParseFamily(family)
	notif := &message.Notification{
		ErrorCode:    message.NotifyCease,
		ErrorSubcode: message.NotifyCeaseMaxPrefixes,
	}
	if ok {
		data := make([]byte, 7)
		data[0] = byte(f.AFI >> 8)
		data[1] = byte(f.AFI)
		data[2] = byte(f.SAFI)
		data[3] = byte(count >> 24)
		data[4] = byte(count >> 16)
		data[5] = byte(count >> 8)
		data[6] = byte(count)
		notif.Data = data
	}
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

// SetPrefixConfigMetrics publishes the static prefix configuration as Prometheus gauges.
// Called once when the peer is added to the reactor.
func setPrefixConfigMetrics(m *reactorMetrics, peerAddr string, settings *PeerSettings) {
	if m == nil {
		return
	}
	for family, maximum := range settings.PrefixMaximum {
		m.prefixMaximum.With(peerAddr, family).Set(float64(maximum))
	}
	for family, warning := range settings.PrefixWarning {
		m.prefixWarning.With(peerAddr, family).Set(float64(warning))
	}
}

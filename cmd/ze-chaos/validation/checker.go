// Design: docs/architecture/chaos-web-dashboard.md — property-based validation

package validation

// PeerResult holds the validation result for a single peer.
type PeerResult struct {
	// Missing contains routes expected but not received.
	Missing *PrefixSet
	// Extra contains routes received but not expected.
	Extra *PrefixSet
	// ExpectedCount is the total number of routes the peer should have.
	ExpectedCount int
	// ActualCount is the total number of routes the peer actually has.
	ActualCount int
}

// CheckResult holds the overall validation result.
type CheckResult struct {
	// Peers holds per-peer results indexed by peer index.
	Peers []PeerResult

	// TotalMissing is the total count of missing routes across all peers.
	TotalMissing int

	// TotalExtra is the total count of extra routes across all peers.
	TotalExtra int

	// Pass is true when there are no discrepancies.
	Pass bool
}

// Check compares the expected state (from Model) against the actual state
// (from Tracker) and returns discrepancies.
func Check(m *Model, t *Tracker) CheckResult {
	result := CheckResult{
		Peers: make([]PeerResult, m.peerCount),
		Pass:  true,
	}

	for peer := range m.peerCount {
		expected := m.Expected(peer)
		actual := t.ActualRoutes(peer)

		missing := NewPrefixSet()
		extra := NewPrefixSet()

		// Find missing: in expected but not in actual.
		for _, prefix := range expected.All() {
			if !actual.Contains(prefix) {
				missing.Add(prefix)
			}
		}

		// Find extra: in actual but not in expected.
		for _, prefix := range actual.All() {
			if !expected.Contains(prefix) {
				extra.Add(prefix)
			}
		}

		result.Peers[peer] = PeerResult{
			Missing:       missing,
			Extra:         extra,
			ExpectedCount: expected.Len(),
			ActualCount:   actual.Len(),
		}

		result.TotalMissing += missing.Len()
		result.TotalExtra += extra.Len()
	}

	result.Pass = result.TotalMissing == 0 && result.TotalExtra == 0
	return result
}

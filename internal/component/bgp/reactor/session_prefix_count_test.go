// Overview: session_prefix.go — checkPrefixLimits and applyInstalledPrefixDeltas
// Related: peer_settings.go — PrefixCountMode, the per-family `count` leaf
// Related: config_prefix.go — parsePrefixCount reads the leaf

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// announceBody builds an UPDATE body announcing n /24 prefixes from 10.0.0.0/24
// upward, with no path attributes.
func announceBody(t *testing.T, first, n int) []byte {
	t.Helper()
	body := []byte{0, 0, 0, 0}
	for i := range n {
		body = append(body, 24, 10, 0, byte(first+i))
	}
	return body
}

// withdrawBody builds an UPDATE body withdrawing n /24 prefixes from
// 10.0.<first>.0/24 upward.
func withdrawBody(t *testing.T, first, n int) []byte {
	t.Helper()
	wdLen := n * 4
	body := []byte{byte(wdLen >> 8), byte(wdLen)}
	for i := range n {
		body = append(body, 24, 10, 0, byte(first+i))
	}
	return append(body, 0, 0) // empty path attributes
}

// testCountMaximum is the ipv4/unicast prefix maximum every count-mode test
// below uses. Two is the smallest number that still lets a peer sit exactly at
// the limit before it crosses it, which is the state both modes disagree about.
const testCountMaximum = 2

// newInstalledCountSettings returns warn-only settings whose ipv4/unicast family
// counts what the session delivered rather than what the peer offered.
func newInstalledCountSettings() *PeerSettings {
	ps := newOfferedCountSettings()
	ps.PrefixCount = map[string]PrefixCountMode{"ipv4/unicast": PrefixCountInstalled}
	return ps
}

// newOfferedCountSettings returns warn-only settings whose ipv4/unicast family
// keeps the default count mode.
func newOfferedCountSettings() *PeerSettings {
	ps := newTestPeerSettingsWithPrefix(testCountMaximum, 0)
	ps.PrefixTeardown = map[string]bool{"ipv4/unicast": false}
	return ps
}

// TestPrefixCountModeDefaultIsOffered pins the default an unconfigured family
// gets. Every config written before the `count` leaf existed states nothing, so
// the accessor is what decides that those peers keep the behavior they had.
func TestPrefixCountModeDefaultIsOffered(t *testing.T) {
	ps := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0)
	assert.Equal(t, PrefixCountOffered, ps.PrefixCountFor("ipv4/unicast"))
	assert.Equal(t, PrefixCountOffered, ps.PrefixCountFor("ipv6/unicast"))

	// The zero value of the map's value type is the offered mode, so a family
	// present with no explicit value reads the same way.
	ps.PrefixCount = map[string]PrefixCountMode{"ipv4/unicast": PrefixCountOffered}
	assert.Equal(t, PrefixCountOffered, ps.PrefixCountFor("ipv4/unicast"))
}

// TestPrefixCountOfferedKeepsDroppedPrefixes proves today's behavior is intact:
// an over-limit UPDATE that warn-only drops still raises the count, so the count
// sits above the number of routes the session delivered and every later announce
// of that family is dropped too.
//
// This is the behavior the `count` leaf makes selectable rather than fixed. It
// is the default, so this test is the regression guard on the default.
//
// VALIDATES: the `count` leaf's default preserves the behavior every existing
// config has today.
// PREVENTS: the new leaf changing what an operator who states nothing gets.
func TestPrefixCountOfferedKeepsDroppedPrefixes(t *testing.T) {
	s := NewSession(newOfferedCountSettings())

	checkOK(t, s, announceBody(t, 0, 2))
	require.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])

	// One prefix past the maximum. The UPDATE is dropped and the count keeps it.
	_, drop := s.checkPrefixLimits(testWireUpdate(announceBody(t, 2, 1)))
	assert.True(t, drop, "warn-only must drop the over-limit UPDATE")
	assert.Equal(t, int64(3), s.prefixCounts.counts[ipv4UKey],
		"offered counts the prefix of a dropped UPDATE")

	// The count is now above the maximum, so the family is closed: an announce
	// that would fit inside the maximum is dropped as well.
	_, drop = s.checkPrefixLimits(testWireUpdate(announceBody(t, 3, 1)))
	assert.True(t, drop)
	assert.Equal(t, int64(4), s.prefixCounts.counts[ipv4UKey])
}

// TestPrefixCountInstalledIgnoresDroppedPrefixes is the same peer with the same
// traffic and the other mode. The dropped UPDATE leaves the count where it was,
// which is what the operator picks the mode for.
func TestPrefixCountInstalledIgnoresDroppedPrefixes(t *testing.T) {
	s := NewSession(newInstalledCountSettings())

	checkOK(t, s, announceBody(t, 0, 2))
	require.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])

	_, drop := s.checkPrefixLimits(testWireUpdate(announceBody(t, 2, 1)))
	assert.True(t, drop, "the over-limit UPDATE is still dropped")
	assert.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey],
		"installed does not count a prefix the session never delivered")

	// The refused prefix never entered the set, so nothing about it is
	// remembered and nothing about it can be spent later.
	assert.Equal(t, 2, len(s.prefixCounts.sets[ipv4UKey]))
	assert.NotContains(t, s.prefixCounts.sets[ipv4UKey], string([]byte{24, 10, 0, 2}))
}

// TestPrefixCountInstalledIgnoresWithdrawOfRefusedPrefix is the fail-closed
// guard on the installed mode. The peer withdraws the prefix ze refused, which
// ze never installed, so the withdrawal must not decrement the count.
//
// A count that fell here would let the next announce fit, so the family would
// end with maximum+1 routes and grow once more on every announce, drop, withdraw
// cycle. The assertion on the second drop is what catches that: it fails when
// the count fell.
//
// The set is what makes this hold without bookkeeping: 10.0.0.2/24 is not in it,
// so deleting it is a no-op. The mechanism it replaces spent a fungible credit
// on whichever withdrawal arrived first, which is why the same credit could pay
// for a prefix ze DID hold.
func TestPrefixCountInstalledIgnoresWithdrawOfRefusedPrefix(t *testing.T) {
	s := NewSession(newInstalledCountSettings())

	checkOK(t, s, announceBody(t, 0, 2))
	require.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])

	// 10.0.0.2/24 is refused.
	_, drop := s.checkPrefixLimits(testWireUpdate(announceBody(t, 2, 1)))
	require.True(t, drop)
	require.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])

	// The peer withdraws the prefix ze refused. The count does not move.
	checkOK(t, s, withdrawBody(t, 2, 1))
	assert.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey],
		"a withdrawal of a refused prefix must not free a slot ze never used")

	// The family is still full, so the next announce is refused too.
	_, drop = s.checkPrefixLimits(testWireUpdate(announceBody(t, 3, 1)))
	assert.True(t, drop, "the maximum still holds after the announce, drop, withdraw cycle")
	assert.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])
}

// TestPrefixCountInstalledIgnoresUnmatchedWithdraw is the same property without
// a refusal anywhere in the story. A peer that withdraws prefixes it never
// announced must not free slots ze is still using.
//
// Under an event tally each unmatched withdrawal freed one slot, so N of them
// let N routes past the maximum. Nothing in the tally could tell an unmatched
// withdrawal from a real one.
func TestPrefixCountInstalledIgnoresUnmatchedWithdraw(t *testing.T) {
	s := NewSession(newInstalledCountSettings())

	checkOK(t, s, announceBody(t, 0, 2))
	require.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])

	// Two withdrawals for prefixes this peer never sent.
	checkOK(t, s, withdrawBody(t, 50, 2))
	assert.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey],
		"a withdrawal of a prefix ze never held must not free a slot")

	_, drop := s.checkPrefixLimits(testWireUpdate(announceBody(t, 3, 1)))
	assert.True(t, drop, "the family is still full")
}

// TestPrefixCountInstalledIsImmuneToReannounce is the defect that made the mode
// unusable, and the reason it counts a set. A peer that re-announces ONE prefix
// holds ONE route: BGP has no explicit refresh, so an attribute change arrives
// as the same NLRI again and implicitly withdraws the old path (RFC 4271
// Section 3.1).
//
// Under the event tally this walked the count up one per announcement until the
// family sat at its maximum over a RIB of one, then refused everything. The
// prefix could then be withdrawn, leaving the count stranded at the maximum over
// an EMPTY RIB, with no withdrawal left that could ever bring it down. That is a
// permanent lockout of the family the mode exists to protect from one.
func TestPrefixCountInstalledIsImmuneToReannounce(t *testing.T) {
	s := NewSession(newInstalledCountSettings())

	// MED churn: the same prefix, six times, each an implicit withdraw.
	for range 6 {
		checkOK(t, s, announceBody(t, 0, 1))
		require.Equal(t, int64(1), s.prefixCounts.counts[ipv4UKey],
			"a re-announced prefix is one route, whatever the announcement count")
	}

	// The peer withdraws the one prefix it ever announced. The RIB is empty and
	// so is the count.
	checkOK(t, s, withdrawBody(t, 0, 1))
	assert.Equal(t, int64(0), s.prefixCounts.counts[ipv4UKey],
		"an empty adj-rib-in counts zero")

	// The family is open, which the lockout made impossible.
	checkOK(t, s, announceBody(t, 9, 1))
	assert.Equal(t, int64(1), s.prefixCounts.counts[ipv4UKey])
}

// TestPrefixCountInstalledCountsARefusedThenAcceptedPrefixOnce closes the third
// arithmetic hole. A prefix refused while the family was full, then accepted
// after a slot freed, is ONE route and must be counted once.
func TestPrefixCountInstalledCountsARefusedThenAcceptedPrefixOnce(t *testing.T) {
	s := NewSession(newInstalledCountSettings())

	checkOK(t, s, announceBody(t, 0, 2)) // set {10.0.0.0, 10.0.1.0}
	_, drop := s.checkPrefixLimits(testWireUpdate(announceBody(t, 2, 1)))
	require.True(t, drop, "10.0.0.2/24 is refused while the family is full")

	// A slot frees.
	checkOK(t, s, withdrawBody(t, 0, 1))
	require.Equal(t, int64(1), s.prefixCounts.counts[ipv4UKey])

	// The peer re-announces the prefix ze refused. It is now installed, once.
	checkOK(t, s, announceBody(t, 2, 1))
	assert.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])
}

// TestPrefixCountInstalledIsImmuneToRouteRefresh proves the same set property
// over the message pattern that stresses it hardest. A ROUTE-REFRESH makes the
// peer replay its whole table (RFC 2918), so every prefix arrives again.
//
// Nothing about the peer's routes changed, so nothing about the count may.
func TestPrefixCountInstalledIsImmuneToRouteRefresh(t *testing.T) {
	s := NewSession(newInstalledCountSettings())

	checkOK(t, s, announceBody(t, 0, 2))
	require.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])

	// The peer replays the table three times over.
	for range 3 {
		checkOK(t, s, announceBody(t, 0, 2))
	}
	assert.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey],
		"a replayed table is the same routes")
}

// TestPrefixCountInstalledRefusalLeavesTheSetUntouched is the atomicity guard.
// A refused message reaches no RIB (processMessage, session_read.go), so it must
// leave no trace in the set either -- including the withdrawals it carried,
// which the section order applies BEFORE the announcement that gets refused.
//
// Without the rollback the withdrawal would take effect while the announcement
// did not, and the count would report routes the peer had already replaced.
//
// The message is built so that the rollback has to be exact rather than merely
// present. It carries every shape the journal can record:
//
//   - 10.0.0.0/24 is withdrawn and re-announced, over one identity. Only
//     replaying the journal backwards puts it back. Forwards re-inserts it and
//     then deletes it, losing a prefix the peer never stopped advertising.
//   - 10.0.99.0/24 is withdrawn without ever being held. Journaling that
//     no-op would make the rollback INSERT a prefix ze never had.
//   - 10.0.1.0/24 is re-announced while already held. Journaling that no-op
//     would make the rollback DELETE a prefix ze does hold.
func TestPrefixCountInstalledRefusalLeavesTheSetUntouched(t *testing.T) {
	s := NewSession(newInstalledCountSettings())

	checkOK(t, s, announceBody(t, 0, 2)) // set {10.0.0.0/24, 10.0.1.0/24}

	body := []byte{
		0, 8, 24, 10, 0, 0, 24, 10, 0, 99, // withdraw one held prefix and one never held
		0, 0, // no path attributes
		24, 10, 0, 0, // re-announce the one just withdrawn
		24, 10, 0, 1, // re-announce one already held
		24, 10, 0, 5, // and one new prefix: this is what crosses the maximum
	}
	_, drop := s.checkPrefixLimits(testWireUpdate(body))
	require.True(t, drop)

	assert.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])
	set := s.prefixCounts.sets[ipv4UKey]
	assert.Contains(t, set, string([]byte{24, 10, 0, 0}), "the withdrawal was rolled back")
	assert.Contains(t, set, string([]byte{24, 10, 0, 1}), "a re-announced prefix was not lost")
	assert.Len(t, set, 2, "no new prefix stayed and no phantom appeared")
}

// twoFamilyMixedModeSettings gives ipv4/unicast the installed mode with room to
// spare, and ipv6/unicast the default mode with a maximum one message overruns.
// The refusal therefore comes from the family that is NOT the installed one,
// which is the only way to reach the offered loop with an installed family
// already settled.
func twoFamilyMixedModeSettings(v6Maximum uint32, v6Teardown bool) *PeerSettings {
	ps := newTestPeerSettingsWithPrefix(10, 0)
	ps.PrefixMaximum["ipv6/unicast"] = v6Maximum
	ps.PrefixTeardown = map[string]bool{"ipv4/unicast": false, "ipv6/unicast": v6Teardown}
	ps.PrefixCount = map[string]PrefixCountMode{"ipv4/unicast": PrefixCountInstalled}
	return ps
}

// mixedFamilyBody is one UPDATE that reaches two families: an MP_REACH_NLRI
// announcing four IPv6 /32 prefixes (RFC 4760 Section 3) and two IPv4 /24 NLRIs
// in the message body (RFC 4271 Section 4.3).
func mixedFamilyBody() []byte {
	return append(ipv6OverflowBody(), 24, 10, 0, 0, 24, 10, 0, 1)
}

// TestPrefixCountInstalledRefusalByAnotherFamilyLeavesTheSetUntouched is the
// cross-family half of the atomicity guard, and it is the case the single-family
// tests above cannot reach.
//
// One UPDATE carries both families. ipv4/unicast counts what ze installed and
// has room for what the message brings it; ipv6/unicast keeps the default mode
// and overruns its maximum, so the message is refused and processMessage
// (session_read.go) delivers none of it to any plugin. The IPv4 prefixes
// therefore reached no RIB, and the installed set must not hold them.
//
// The defect this pins: checkPrefixLimits settled the installed families first
// and the journal that can undo them died with that call, so a refusal decided
// afterwards had nothing left to roll back. Every single-family test stayed
// green through it, because a single-family message is refused by the same call
// that mutated the set.
func TestPrefixCountInstalledRefusalByAnotherFamilyLeavesTheSetUntouched(t *testing.T) {
	tests := []struct {
		name     string
		teardown bool
	}{
		{"warn-only drop", false},
		{"teardown", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession(twoFamilyMixedModeSettings(3, tt.teardown))

			notif, drop := s.checkPrefixLimits(testWireUpdate(mixedFamilyBody()))

			require.True(t, notif != nil || drop, "four IPv6 prefixes overrun a maximum of 3")
			assert.Equal(t, int64(0), s.prefixCounts.counts[ipv4UKey],
				"a refused message must not move an installed family's count")
			assert.Empty(t, s.prefixCounts.sets[ipv4UKey],
				"a refused message must not move an installed family's set")
		})
	}

	// The same message with room in both families is accepted, so the installed
	// set keeps what it brought. Without this the rollback could fire on every
	// message and the assertions above would still pass.
	t.Run("an accepted message still moves the set", func(t *testing.T) {
		s := NewSession(twoFamilyMixedModeSettings(10, false))

		checkOK(t, s, mixedFamilyBody())

		assert.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])
		assert.Len(t, s.prefixCounts.sets[ipv4UKey], 2)
		assert.Equal(t, int64(4), s.prefixCounts.counts[ipv6UKey],
			"the offered family counts what it was sent")
	})
}

// TestPrefixCountInstalledRefusalByAnotherFamilyRestoresAWithdrawal is the same
// refusal over a message that LOWERS the installed family. The IPv4 section
// withdraws a prefix ze holds; the IPv6 section then overruns its maximum. The
// withdrawal never reached a RIB either, so the set must still hold that prefix
// and the count must still be 2.
//
// A rollback that only undid insertions would pass the test above and fail this
// one, and a count restored by subtraction alone would report 1.
func TestPrefixCountInstalledRefusalByAnotherFamilyRestoresAWithdrawal(t *testing.T) {
	s := NewSession(twoFamilyMixedModeSettings(3, false))

	checkOK(t, s, announceBody(t, 0, 2))
	require.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])

	// The IPv4 withdrawn-routes field takes 10.0.0.0/24 away; the MP_REACH then
	// overruns ipv6/unicast.
	body := append([]byte{0, 4, 24, 10, 0, 0}, ipv6OverflowBody()[2:]...)
	_, drop := s.checkPrefixLimits(testWireUpdate(body))
	require.True(t, drop)

	assert.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey],
		"a refused withdrawal must not free a slot")
	assert.Contains(t, s.prefixCounts.sets[ipv4UKey], string([]byte{24, 10, 0, 0}),
		"the withdrawal was rolled back")
	assert.Len(t, s.prefixCounts.sets[ipv4UKey], 2)
}

// TestPrefixCountInstalledStopsGrowingAtTheMaximum bounds what one refused
// message can make ze allocate. A Go map keeps its buckets after its entries go,
// so a peer that could make ze insert a whole message before throwing it away
// would leave that memory behind on every try.
//
// The bound is not observable through checkPrefixLimits: the rollback empties the
// set either way, and the NOTIFICATION carries the configured upper bound rather
// than any count (RFC 4486 Section 4 Figure 1), so neither observable tells
// maximum+1 from 40. The second half therefore reads the set that
// applyInstalledPrefixSection leaves behind, which is where the bound is decided.
func TestPrefixCountInstalledStopsGrowingAtTheMaximum(t *testing.T) {
	newInstalledSession := func() *Session {
		ps := newTestPeerSettingsWithPrefix(testCountMaximum, 0)
		ps.PrefixTeardown = map[string]bool{"ipv4/unicast": true}
		ps.PrefixCount = map[string]PrefixCountMode{"ipv4/unicast": PrefixCountInstalled}
		return NewSession(ps)
	}

	// One message announcing 40 prefixes into a family whose maximum is 2. The whole
	// message is refused and leaves nothing behind.
	s := newInstalledSession()
	notif, drop := s.checkPrefixLimits(testWireUpdate(announceBody(t, 0, 40)))
	require.NotNil(t, notif)
	require.False(t, drop)
	assert.Equal(t, int64(0), s.prefixCounts.counts[ipv4UKey])
	assert.Empty(t, s.prefixCounts.sets[ipv4UKey], "the refused message left nothing")

	// The same message against the producer of the bound, on a fresh session so no
	// rollback has run. With the bound the set holds maximum+1; without it, all 40.
	s = newInstalledSession()
	var buf [maxPrefixSections]prefixSection
	sections := buf[:s.collectPrefixSections(testWireUpdate(announceBody(t, 0, 40)), &buf)]
	require.Len(t, sections, 1)

	maximum, over := s.applyInstalledPrefixSection(sections[0], true)
	require.True(t, over, "40 prefixes must cross a maximum of 2")
	assert.Equal(t, uint32(testCountMaximum), maximum)
	assert.Len(t, s.prefixCounts.sets[ipv4UKey], testCountMaximum+1,
		"the family stopped taking prefixes at maximum+1")
}

// TestPrefixCountInstalledWithdrawOfDeliveredPrefixFreesASlot is the other half
// of the absorption: once the credit is spent, a withdrawal of a prefix ze DID
// deliver decrements the count and the family accepts an announce again.
//
// This is what the mode is for. Under `offered` the same peer stays closed.
func TestPrefixCountInstalledWithdrawOfDeliveredPrefixFreesASlot(t *testing.T) {
	s := NewSession(newInstalledCountSettings())

	checkOK(t, s, announceBody(t, 0, 2))
	_, drop := s.checkPrefixLimits(testWireUpdate(announceBody(t, 2, 1)))
	require.True(t, drop)
	checkOK(t, s, withdrawBody(t, 2, 1)) // absorbed
	require.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])

	// Withdraw a prefix ze delivered.
	checkOK(t, s, withdrawBody(t, 0, 1))
	require.Equal(t, int64(1), s.prefixCounts.counts[ipv4UKey])

	// The slot is free, so this announce lands.
	checkOK(t, s, announceBody(t, 3, 1))
	assert.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])
}

// TestPrefixCountInstalledReachesZeroWhenPeerWithdrawsEverything proves the mode
// strands no count. A peer that withdraws every prefix it announced, refused
// ones included, leaves the count at zero and the family open again.
//
// The property, and it needs no arithmetic: the count IS the size of the set of
// prefixes ze holds, and a peer that withdraws every prefix empties that set.
// Nothing else can hold the count up, because nothing else contributes to it.
func TestPrefixCountInstalledReachesZeroWhenPeerWithdrawsEverything(t *testing.T) {
	s := NewSession(newInstalledCountSettings())

	checkOK(t, s, announceBody(t, 0, 2))
	_, drop := s.checkPrefixLimits(testWireUpdate(announceBody(t, 2, 2)))
	require.True(t, drop)
	require.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])

	// The peer withdraws all four: the two ze holds and the two ze refused.
	checkOK(t, s, withdrawBody(t, 0, 4))
	assert.Equal(t, int64(0), s.prefixCounts.counts[ipv4UKey])
	// the two prefixCounts.dropped assertions this test carried are
	// gone with the field. The refused-announcement credit was the mechanism
	// this file's four new tests measure as broken, and it is deleted rather
	// than repaired (ai/rules/no-layering.md). The set below is the replacement
	// coverage: it is the thing the count is now derived from, so asserting it
	// is empty is a stronger statement than the credit balance ever was.
	assert.Empty(t, s.prefixCounts.sets[ipv4UKey])

	// The family is open again.
	checkOK(t, s, announceBody(t, 0, 2))
	assert.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])
}

// TestPrefixCountOfferedReachesZeroWhenPeerWithdrawsEverything is the same
// property for the default mode, which the deferral row called self-correcting.
// The count holds every announcement, the refused ones included, and the peer
// withdraws every one of them, so the count comes back down to zero.
func TestPrefixCountOfferedReachesZeroWhenPeerWithdrawsEverything(t *testing.T) {
	s := NewSession(newOfferedCountSettings())

	checkOK(t, s, announceBody(t, 0, 2))
	_, drop := s.checkPrefixLimits(testWireUpdate(announceBody(t, 2, 2)))
	require.True(t, drop)
	require.Equal(t, int64(4), s.prefixCounts.counts[ipv4UKey])

	checkOK(t, s, withdrawBody(t, 0, 4))
	assert.Equal(t, int64(0), s.prefixCounts.counts[ipv4UKey])

	checkOK(t, s, announceBody(t, 0, 2))
	assert.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])
}

// TestPrefixCountInstalledReplacementUpdateIsNotAnOverflow keeps the property
// the section order exists for: one UPDATE that withdraws prefixes and announces
// the same number is judged on its net change, not on the announcement alone.
func TestPrefixCountInstalledReplacementUpdateIsNotAnOverflow(t *testing.T) {
	s := NewSession(newInstalledCountSettings())

	checkOK(t, s, announceBody(t, 0, 2))
	require.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])

	// Withdraw both and announce two others in one message. The count is full
	// before it and full after it, and nothing is refused.
	body := []byte{0, 8, 24, 10, 0, 0, 24, 10, 0, 1, 0, 0, 24, 10, 0, 5, 24, 10, 0, 6}
	checkOK(t, s, body)
	assert.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])
	// the prefixCounts.dropped assertion is gone with the field
	// (see TestPrefixCountInstalledReachesZeroWhenPeerWithdrawsEverything). The
	// set membership below is the replacement, and it says more: it names WHICH
	// two prefixes the family now holds, which a credit balance of zero could
	// not distinguish from the wrong two.
	assert.Equal(t, map[string]struct{}{
		string([]byte{24, 10, 0, 5}): {},
		string([]byte{24, 10, 0, 6}): {},
	}, s.prefixCounts.sets[ipv4UKey])
}

// TestPrefixCountInstalledTeardownStillStopsTheSession proves the mode chooses
// the arithmetic and never the enforcement. A family that asks for the installed
// count and keeps `teardown true` still sends the Cease.
//
// RFC 4486 Section 4 Figure 1: the Data field carries the configured upper bound,
// which is the same number whichever mode the family counts with.
func TestPrefixCountInstalledTeardownStillStopsTheSession(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(2, 0)
	ps.PrefixTeardown = map[string]bool{"ipv4/unicast": true}
	ps.PrefixCount = map[string]PrefixCountMode{"ipv4/unicast": PrefixCountInstalled}
	s := NewSession(ps)

	checkOK(t, s, announceBody(t, 0, 2))
	notif, drop := s.checkPrefixLimits(testWireUpdate(announceBody(t, 2, 1)))
	require.NotNil(t, notif, "teardown true must still send the NOTIFICATION")
	assert.False(t, drop)
	assert.Equal(t, "ipv4/unicast", familyString(s.prefixExceededFamily))
	// Data: AFI 1, SAFI 1, upper bound 2 -- the maximum the family configured.
	assert.Equal(t, []byte{0, 1, 1, 0, 0, 0, 2}, notif.Data)
}

// TestPrefixCountInstalledNeedsNoLimitToCount proves a family with the mode set
// and no maximum still tallies, which is what route-count anomaly detection
// reads. Nothing can be refused, so nothing is credited.
func TestPrefixCountInstalledNeedsNoLimitToCount(t *testing.T) {
	ps := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0)
	ps.PrefixCount = map[string]PrefixCountMode{"ipv4/unicast": PrefixCountInstalled}
	s := NewSession(ps)

	checkOK(t, s, announceBody(t, 0, 3))
	assert.Equal(t, int64(3), s.prefixCounts.counts[ipv4UKey])
	checkOK(t, s, withdrawBody(t, 0, 2))
	assert.Equal(t, int64(1), s.prefixCounts.counts[ipv4UKey])
	// the prefixCounts.dropped assertion is gone with the field
	// (see TestPrefixCountInstalledReachesZeroWhenPeerWithdrawsEverything).
	// Nothing can be refused without a maximum, so the replacement assertion is
	// that the count and the set agree, which is the mode's whole invariant.
	assert.Len(t, s.prefixCounts.sets[ipv4UKey], 1)
}

// TestPrefixCountInstalledFamilyIsResolvedOnce proves the per-session set that
// keeps the mode lookup off the wire path is built from the settings, and that
// an unconfigured family is absent from it.
func TestPrefixCountInstalledFamilyIsResolvedOnce(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(10, 0)
	ps.PrefixCount = map[string]PrefixCountMode{
		"ipv4/unicast": PrefixCountInstalled,
		"ipv6/unicast": PrefixCountOffered,
	}
	s := NewSession(ps)

	assert.True(t, s.prefixCounts.installed[ipv4UKey])
	assert.False(t, s.prefixCounts.installed[ipv6UKey])

	// A peer with no installed family carries no set at all, which is the check
	// every UPDATE makes before it does anything different.
	assert.Nil(t, NewSession(newTestPeerSettingsWithPrefix(10, 0)).prefixCounts.installed)
}

// BenchmarkCheckPrefixLimitsOffered, BenchmarkCheckPrefixLimitsInstalled and
// BenchmarkCheckPrefixLimitsInstalledChurn measure the receive path under each
// mode. checkPrefixLimits runs once per UPDATE, so the section array it collects
// and the journal the installed mode fills have to stay off the heap
// (ai/rules/performance.md). All three are registered in perf.AllocCeilings
// (internal/perf/allocgate.go), so the numbers below are a gate rather than a
// claim.
//
// Measured 2026-09-02 on an M4 Max: 0 allocs/op for the first two and 2 for
// Churn. Two things keep the path off the heap, and each one was a regression
// caught here first. The per-family splitter forEachPrefixEntry dispatches to is
// a WALK rather than a builder (nlrisplit.Splitter), so counting the NLRIs of a
// section materializes no slice. And the installed mode's visitor is bound once
// per session (Session.prefixSetVisit) rather than built per section, because
// the splitter is reached through a func value and Go treats anything handed to
// one as escaping. No ns/op is recorded here: the same code measured 163 and 551
// ns/op an hour apart on this shared machine, and allocs/op is the number that
// does not move with the load.
//
// The first two re-announce one unchanged table, which is the steady state and
// the case that used to walk the count into a lockout. Nothing is inserted
// there, so neither would notice the cost of an insert: Churn is the one that
// pays it. It alternates a four-prefix announce with the matching withdraw, so
// every second call inserts four map keys and the one after it deletes them,
// which is 2 allocs/op by arithmetic rather than by measurement.
func BenchmarkCheckPrefixLimitsOffered(b *testing.B) {
	benchmarkCheckPrefixLimits(b, newTestPeerSettingsWithPrefix(1000000, 0))
}

func BenchmarkCheckPrefixLimitsInstalled(b *testing.B) {
	benchmarkCheckPrefixLimits(b, installedBenchSettings())
}

func BenchmarkCheckPrefixLimitsInstalledChurn(b *testing.B) {
	s := NewSession(installedBenchSettings())
	announce := testWireUpdate([]byte{0, 0, 0, 0, 24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2, 24, 10, 0, 3})
	withdraw := testWireUpdate([]byte{0, 16, 24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2, 24, 10, 0, 3, 0, 0})

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		wu := announce
		if i%2 == 1 {
			wu = withdraw
		}
		s.checkPrefixLimits(wu) //nolint:errcheck // the pair is the subject, not an error
	}
}

// installedBenchSettings returns a peer whose ipv4/unicast family counts what ze
// installed, with a maximum far above anything the benchmarks announce.
func installedBenchSettings() *PeerSettings {
	ps := newTestPeerSettingsWithPrefix(1000000, 0)
	ps.PrefixCount = map[string]PrefixCountMode{"ipv4/unicast": PrefixCountInstalled}
	return ps
}

func benchmarkCheckPrefixLimits(b *testing.B, ps *PeerSettings) {
	b.Helper()
	s := NewSession(ps)
	// One announcement of four prefixes, the shape a full table arrives in.
	body := []byte{0, 0, 0, 0, 24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2, 24, 10, 0, 3}
	wu := testWireUpdate(body)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		s.checkPrefixLimits(wu) //nolint:errcheck // the pair is the subject, not an error
	}
}

// TestParsePrefixCountMode covers every spelling the YANG enum allows and the
// rejection of anything else. The parser refuses rather than approximates: a
// misspelled mode that read as `offered` would silently give the operator the
// behavior they configured against.
func TestParsePrefixCountMode(t *testing.T) {
	tests := []struct {
		input string
		want  PrefixCountMode
		ok    bool
	}{
		{"offered", PrefixCountOffered, true},
		{"installed", PrefixCountInstalled, true},
		{"", PrefixCountOffered, false},
		{"Installed", PrefixCountOffered, false},
		{"received", PrefixCountOffered, false},
	}
	for _, tc := range tests {
		mode, ok := parsePrefixCountMode(tc.input)
		assert.Equal(t, tc.ok, ok, "input %q", tc.input)
		assert.Equal(t, tc.want, mode, "input %q", tc.input)
	}

	assert.Equal(t, "offered", PrefixCountOffered.String())
	assert.Equal(t, "installed", PrefixCountInstalled.String())
}

// TestParsePrefixCountFromFamily proves the leaf reaches PeerSettings, per
// family, and that a value the enum does not carry is a config error rather than
// a silent default.
func TestParsePrefixCountFromFamily(t *testing.T) {
	ps := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0)

	entry := map[string]any{"prefix": map[string]any{"maximum": "100", "count": "installed"}}
	require.NoError(t, parsePrefixLimitFromFamily("ipv4/unicast", entry, ps))
	assert.Equal(t, PrefixCountInstalled, ps.PrefixCountFor("ipv4/unicast"))

	// A second family disagreeing keeps its own answer.
	entry6 := map[string]any{"prefix": map[string]any{"maximum": "100", "count": "offered"}}
	require.NoError(t, parsePrefixLimitFromFamily("ipv6/unicast", entry6, ps))
	assert.Equal(t, PrefixCountOffered, ps.PrefixCountFor("ipv6/unicast"))
	assert.Equal(t, PrefixCountInstalled, ps.PrefixCountFor("ipv4/unicast"))

	// A family that states nothing reads as offered and writes no entry.
	entryNone := map[string]any{"prefix": map[string]any{"maximum": "100"}}
	require.NoError(t, parsePrefixLimitFromFamily("ipv4/multicast", entryNone, ps))
	assert.Equal(t, PrefixCountOffered, ps.PrefixCountFor("ipv4/multicast"))

	bad := map[string]any{"prefix": map[string]any{"maximum": "100", "count": "delivered"}}
	err := parsePrefixLimitFromFamily("ipv4/unicast", bad, ps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "offered, installed")
}

package domain

import (
	"testing"
	"time"

	mdns "github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScheduleClampsBelowFloor proves a TTL shorter than the group's floor is
// scheduled at the floor.
//
// VALIDATES: AC-9 -- a name answering with a very short TTL is not asked for
// sooner than the operator allows.
// PREVENTS: a 1-second TTL driving a query and a set rewrite every second.
func TestScheduleClampsBelowFloor(t *testing.T) {
	cases := []struct {
		name  string
		ttl   uint32
		floor uint32
		want  time.Duration
	}{
		{name: "below the floor", ttl: 5, floor: 300, want: 300 * time.Second},
		{name: "at the floor", ttl: 300, floor: 300, want: 300 * time.Second},
		{name: "above the floor", ttl: 900, floor: 300, want: 900 * time.Second},
		{name: "one second below", ttl: 299, floor: 300, want: 300 * time.Second},
		{name: "one second above", ttl: 301, floor: 300, want: 301 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, refreshInterval(tc.ttl, tc.floor))
		})
	}
}

// TestScheduleTreatsTTLZeroAsDoNotCache proves TTL=0 waits the floor rather
// than firing at once.
//
// A server answering TTL=0 is saying "do not cache this" (RFC 1035 Section
// 3.2.1). extractRecords in internal/component/resolve/dns keeps that 0 rather
// than defaulting it, so it reaches the schedule verbatim.
//
// VALIDATES: AC-9 -- TTL=0 does not spin.
// PREVENTS: a zero interval, which would make the worker resolve the same name
// as fast as the engine can answer, for as long as the server keeps saying 0.
func TestScheduleTreatsTTLZeroAsDoNotCache(t *testing.T) {
	assert.Equal(t, 300*time.Second, refreshInterval(0, 300),
		"TTL=0 means do not cache, not expired now")
	assert.Positive(t, refreshInterval(0, 1), "a zero interval would spin")

	s := newSchedule()
	now := time.Now()
	key := nameKey{group: "g", name: "n.invalid", family: familyV4}
	s.arm(key, 0, 60, now)

	_, due, found := s.nextDue()
	require.True(t, found)
	assert.True(t, due.After(now), "TTL=0 must schedule into the future, never at or before now")
}

// TestScheduleFloorZeroFallsBackToTheDefault proves a floor that never reached
// the parser cannot produce a zero interval. The YANG range starts at 1, so a
// zero here means the value did not arrive, and answering with a zero interval
// would spin.
func TestScheduleFloorZeroFallsBackToTheDefault(t *testing.T) {
	assert.Equal(t, ttlFloorDefault*time.Second, refreshInterval(0, 0))
}

// TestScheduleBoundsTheLongestWait proves a server naming a date years out
// does not stop Ze asking again. A TTL is a uint32 of seconds, so the largest
// answer is over a century.
func TestScheduleBoundsTheLongestWait(t *testing.T) {
	assert.Equal(t, maxRefreshInterval, refreshInterval(^uint32(0), 60))
	assert.Equal(t, maxRefreshInterval, refreshInterval(86401, 60))
}

// TestScheduleFamiliesIndependentTTL proves each family keeps its own due time
// and that rearming one does not move the other.
//
// VALIDATES: AC-10 -- a name with A and AAAA records at different TTLs is
// refreshed per family, and a change in one does not reschedule the other.
// PREVENTS: one shared timer per name, which would refresh both families on
// whichever TTL happened to be read last.
func TestScheduleFamiliesIndependentTTL(t *testing.T) {
	s := newSchedule()
	now := time.Now()
	v4 := nameKey{group: "g", name: "n.invalid", family: familyV4}
	v6 := nameKey{group: "g", name: "n.invalid", family: familyV6}

	s.arm(v4, 60, 60, now)
	s.arm(v6, 3600, 60, now)

	key, due, found := s.nextDue()
	require.True(t, found)
	assert.Equal(t, v4, key, "the shorter TTL is due first")
	assert.Equal(t, now.Add(60*time.Second), due)

	// Rearming IPv4 must leave IPv6 exactly where it was.
	s.arm(v4, 120, 60, now)
	key, due, found = s.nextDue()
	require.True(t, found)
	assert.Equal(t, v4, key)
	assert.Equal(t, now.Add(120*time.Second), due)

	s.arm(v4, 7200, 60, now)
	key, due, found = s.nextDue()
	require.True(t, found)
	assert.Equal(t, v6, key, "IPv6 kept its own due time through two IPv4 rearms")
	assert.Equal(t, now.Add(3600*time.Second), due)
}

// TestScheduleFamiliesAskForDifferentRecordTypes proves the two families query
// different DNS RR types. A schedule that split the units but asked for A twice
// would satisfy every timing assertion above and resolve nothing for IPv6.
func TestScheduleFamiliesAskForDifferentRecordTypes(t *testing.T) {
	assert.Equal(t, mdns.TypeA, familyV4.qtype)
	assert.Equal(t, mdns.TypeAAAA, familyV6.qtype)
	assert.True(t, familyV4.isV4)
	assert.False(t, familyV6.isV4)
	assert.NotEqual(t, familyV4.label, familyV6.label, "the zefs key segment must differ per family")
}

// TestScheduleResetKeepsExistingDueTimes proves a commit that changed
// something else does not restart every TTL in the box, which would send one
// burst of queries upstream on each commit.
func TestScheduleResetKeepsExistingDueTimes(t *testing.T) {
	s := newSchedule()
	now := time.Now()
	groups := []group{{Name: "g", Names: []string{"a.invalid"}, TTLFloor: 60}}

	s.reset(groups, noStoredExpiry, now)
	key := nameKey{group: "g", name: "a.invalid", family: familyV4}
	s.arm(key, 3600, 60, now)

	later := now.Add(time.Minute)
	s.reset(groups, noStoredExpiry, later)

	require.Len(t, s.keys(), 2, "one entry per family")
	found := false
	for _, k := range s.keys() {
		if k != key {
			continue
		}
		found = true
	}
	require.True(t, found)

	_, due, ok := s.nextDue()
	require.True(t, ok)
	// The IPv6 unit was created at `now` and is due then; the IPv4 unit kept
	// its armed time. Neither was moved to `later`.
	assert.Equal(t, now, due, "an already-scheduled unit keeps its due time across a reset")
}

// TestScheduleResetDropsRemovedNames proves a name an operator deleted stops
// being queried. A schedule that only added would keep resolving a name no
// config mentions, forever.
func TestScheduleResetDropsRemovedNames(t *testing.T) {
	s := newSchedule()
	now := time.Now()

	s.reset([]group{{Name: "g", Names: []string{"a.invalid", "b.invalid"}}}, noStoredExpiry, now)
	require.Len(t, s.keys(), 4)

	s.reset([]group{{Name: "g", Names: []string{"a.invalid"}}}, noStoredExpiry, now)
	keys := s.keys()
	require.Len(t, keys, 2)
	for _, k := range keys {
		assert.Equal(t, "a.invalid", k.name, "the removed name must not survive the reset")
	}
}

// TestScheduleNextDueSeparatesEmptyFromZeroTime proves the bool return is what
// tells a caller the schedule is empty. A worker reading only the instant
// would sleep until the zero time, which is in the past, and spin.
func TestScheduleNextDueSeparatesEmptyFromZeroTime(t *testing.T) {
	s := newSchedule()
	_, _, found := s.nextDue()
	assert.False(t, found, "an empty schedule reports not-found rather than the zero time")

	key := nameKey{group: "g", name: "n.invalid", family: familyV4}
	s.arm(key, 60, 60, time.Time{})
	got, _, found := s.nextDue()
	require.True(t, found)
	assert.Equal(t, key, got)

	s.remove(key)
	_, _, found = s.nextDue()
	assert.False(t, found)
}

// TestScheduleNextDueIsDeterministic proves ties break on the key. Every unit
// of a fresh config is due at the same instant and Go randomizes map
// iteration, so without a tie-break the worker would pick a different one on
// each run and a test over it would flake.
func TestScheduleNextDueIsDeterministic(t *testing.T) {
	now := time.Now()
	groups := []group{
		{Name: "b", Names: []string{"z.invalid"}},
		{Name: "a", Names: []string{"y.invalid"}},
	}
	for range 20 {
		s := newSchedule()
		s.reset(groups, noStoredExpiry, now)
		key, _, found := s.nextDue()
		require.True(t, found)
		assert.Equal(t, "a", key.group)
		assert.Equal(t, "ipv4", key.family.label)
	}
}

// noStoredExpiry is the lookup for a schedule test that holds no stored
// answer: every unit it resets is new and unanswered, so it is due at once.
func noStoredExpiry(nameKey) (time.Time, bool) { return time.Time{}, false }

// dueOf reads one unit's due time straight from the schedule, so a test can
// assert where reset armed a unit that nextDue would hide behind an earlier one.
func dueOf(t *testing.T, s *schedule, key nameKey) time.Time {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	due, ok := s.due[key]
	require.True(t, ok, "the unit must be scheduled")
	return due
}

// TestResetUsesStoredExpiry proves a start arms each unit at the expiry its
// stored answer carries, and asks at once only for a unit whose stored answer
// expired or that has none.
//
// VALIDATES: AC-13 -- a name whose stored answer has not expired is not
// resolved at start, and is resolved at its stored expiry; a name past its
// expiry, and a name never answered, are resolved at start.
// PREVENTS: the reset that made every unit due at start, so a reboot asked
// upstream for every name and could enforce different addresses than the
// box enforced before it (Thomas, spec-fetched-data-survives-restart).
func TestResetUsesStoredExpiry(t *testing.T) {
	now := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	unexpired := nameKey{group: "g", name: "a.invalid", family: familyV4}
	expired := nameKey{group: "g", name: "a.invalid", family: familyV6}
	neverAnswered := nameKey{group: "g", name: "b.invalid", family: familyV4}
	stored := map[nameKey]time.Time{
		unexpired: now.Add(50 * time.Minute),
		expired:   now.Add(-time.Minute),
	}
	lookup := func(key nameKey) (time.Time, bool) {
		when, ok := stored[key]
		return when, ok
	}

	s := newSchedule()
	s.reset([]group{{Name: "g", Names: []string{"a.invalid", "b.invalid"}, TTLFloor: 60}}, lookup, now)

	assert.Equal(t, now.Add(50*time.Minute), dueOf(t, s, unexpired),
		"an unexpired stored answer is asked again at its expiry, not at start")
	assert.Equal(t, now, dueOf(t, s, expired), "an expired stored answer is asked at once")
	assert.Equal(t, now, dueOf(t, s, neverAnswered), "a name with no stored answer is asked at once")
}

// TestResetCapsAStoredExpiryAtTheCeiling proves a stored expiry further out
// than the one-day ceiling is armed at the ceiling.
//
// Every expiry this package writes is at most a day out, so a later one is a
// clock that stepped back or a value edited in the store. Honoring it would
// freeze the name past the bound refreshInterval keeps for a live answer.
func TestResetCapsAStoredExpiryAtTheCeiling(t *testing.T) {
	now := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	key := nameKey{group: "g", name: "a.invalid", family: familyV4}
	lookup := func(nameKey) (time.Time, bool) { return now.Add(48 * time.Hour), true }

	s := newSchedule()
	s.reset([]group{{Name: "g", Names: []string{"a.invalid"}, TTLFloor: 60}}, lookup, now)

	assert.Equal(t, now.Add(maxRefreshInterval), dueOf(t, s, key))
}

// TestResetDueNowForAnswerStoredWithoutExpiry proves an answer written before
// the stored expiry existed is loaded, still programs its addresses, and is
// asked again at start.
//
// VALIDATES: AC-13 survivor -- a name stored before this change (no
// expires-at) is resolved at start once; the answer then writes its expiry.
// PREVENTS: an old value read as never expiring, or refused as unreadable and
// its addresses dropped from the set at the upgrade boot.
func TestResetDueNowForAnswerStoredWithoutExpiry(t *testing.T) {
	dir := t.TempDir()
	persistence := domainTestStorage(t, dir)
	key := nameKey{group: "cdn", name: "a.invalid", family: familyV4}
	legacy := `{"addresses":["192.0.2.1"],"resolved-at":"2026-01-01T00:00:00Z","status":"NOERROR"}`
	require.NoError(t, persistence.WriteKey(storedAnswerKey(key), []byte(legacy)))

	cache := newStore(persistence)
	cfg := oneGroupConfig()
	require.NoError(t, cache.open(cfg.groups))

	entry, seen := cache.get(key)
	require.True(t, seen, "an answer stored without an expiry is still loaded")
	assert.Equal(t, []string{"192.0.2.1"}, entry.Addresses)
	_, hasExpiry := cache.expiry(key)
	assert.False(t, hasExpiry, "an answer stored without an expiry reports none")

	now := time.Now()
	s := newSchedule()
	s.reset(cfg.groups, cache.expiry, now)
	assert.Equal(t, now, dueOf(t, s, key), "an answer with no stored expiry is asked at once")
}

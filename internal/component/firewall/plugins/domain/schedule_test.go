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

	s.reset(groups, now)
	key := nameKey{group: "g", name: "a.invalid", family: familyV4}
	s.arm(key, 3600, 60, now)

	later := now.Add(time.Minute)
	s.reset(groups, later)

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

	s.reset([]group{{Name: "g", Names: []string{"a.invalid", "b.invalid"}}}, now)
	require.Len(t, s.keys(), 4)

	s.reset([]group{{Name: "g", Names: []string{"a.invalid"}}}, now)
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
		s.reset(groups, now)
		key, _, found := s.nextDue()
		require.True(t, found)
		assert.Equal(t, "a", key.group)
		assert.Equal(t, "ipv4", key.family.label)
	}
}

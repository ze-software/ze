// Design: docs/architecture/firewall/firewall-domain-group.md -- per-name, per-family TTL schedule
//
// A TTL is a property of ONE answer, so the schedule is per name and per
// family rather than a shared interval. firewall-irr refreshes every reference
// on one `refresh-interval`, which is right for a whois database that has no
// TTL to offer; a DNS server states when its answer stops being true, and
// asking again on somebody else's clock either wastes a query or serves an
// address past its life.
//
// One worker drives the whole schedule. It sleeps until the earliest due entry
// and wakes for that one, so N names and 2 families cost one goroutine and one
// timer rather than 2N of each (ai/rules/goroutine-lifecycle.md).

package domain

import (
	"sort"
	"sync"
	"time"

	mdns "github.com/miekg/dns"
)

// family is one address family, with everything the rest of the package needs
// to act on it: the DNS RR type to ask for, the zefs key segment, and the
// operator-facing label. Keeping the four together is what stops a caller
// pairing TypeA with an IPv6 set.
type family struct {
	qtype uint16
	label string
	isV4  bool
}

var (
	familyV4 = family{qtype: mdns.TypeA, label: "ipv4", isV4: true}
	familyV6 = family{qtype: mdns.TypeAAAA, label: "ipv6", isV4: false}
)

// families is what a refresh iterates. A group asks for both, always: a name
// that holds only A records answers NOERROR with nothing for AAAA, and that
// empty answer is the correct content of the IPv6 set rather than a failure.
var families = [2]family{familyV4, familyV6}

// maxRefreshInterval bounds how long the schedule will wait, whatever a server
// answers with. A TTL is a uint32 of seconds, so a server can name a date 136
// years out; a firewall that then never asks again is enforcing a decision
// nobody can see. One day is the same ceiling the ttl-floor leaf carries.
const maxRefreshInterval = 86400 * time.Second

// nameKey identifies one DNS name inside one group, in one family. It is the
// unit of everything this package does: what the schedule arms, what the cache
// stores, and what the change log records are all keyed by it.
//
// One type rather than two, because a refresh unit and a cache entry are the
// same tuple. Two declarations of it would be a future disagreement with
// nothing to arbitrate it, and the family is part of it because AC-10 requires
// each family to keep its own TTL and its own last-good value.
type nameKey struct {
	group  string
	name   string
	family family
}

// schedule holds when each unit is next due.
// Safe for concurrent use.
//
// Three goroutines reach it: the refresh worker asks what is due next, a
// configure replaces the whole set, and an operator's update command rearms
// the units it resolved. The lock is what makes the third one safe, and the
// map is touched once per refresh rather than once per packet, so its cost is
// not on any path that counts.
type schedule struct {
	mu  sync.Mutex
	due map[nameKey]time.Time
}

func newSchedule() *schedule {
	return &schedule{due: make(map[nameKey]time.Time)}
}

// reset replaces the schedule with one entry per unit of the given groups, all
// due now. It is called on each configure: a name an operator removed must
// stop being queried, and a name they added must be asked for at once.
func (s *schedule) reset(groups []group, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fresh := make(map[nameKey]time.Time)
	for _, g := range groups {
		for _, name := range g.Names {
			for _, fam := range families {
				key := nameKey{group: g.Name, name: name, family: fam}
				// A unit already scheduled keeps its due time, so a commit that
				// changed something else does not restart every TTL in the box
				// and send one burst of queries upstream.
				if when, ok := s.due[key]; ok {
					fresh[key] = when
					continue
				}
				fresh[key] = now
			}
		}
	}
	s.due = fresh
}

// arm sets when this unit is next due, from the TTL the answer carried and the
// group's floor.
//
// The floor is what makes AC-9 hold, and it covers two cases with one rule. A
// TTL below the floor is a server asking to be polled faster than the operator
// allows. A TTL of exactly 0 is a server saying "do not cache this" (RFC 1035
// Section 3.2.1, and extractRecords in internal/component/resolve/dns keeps it
// at 0 rather than defaulting it); read as "expired now" it would drive a
// query every time round the loop. Waiting the floor answers both, and the
// answer is never used past its life because the floor is measured from the
// moment the answer arrived.
func (s *schedule) arm(key nameKey, ttl, floor uint32, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.due[key] = now.Add(refreshInterval(ttl, floor))
}

// refreshInterval is how long to wait before asking again.
func refreshInterval(ttl, floor uint32) time.Duration {
	if floor == 0 {
		floor = ttlFloorDefault
	}
	interval := time.Duration(max(ttl, floor)) * time.Second
	if interval > maxRefreshInterval {
		return maxRefreshInterval
	}
	return interval
}

// nextDue returns the earliest-due unit and when it is due. The bool separates
// "nothing is scheduled" from "something is due at the zero time", which a
// caller sleeping on the returned instant cannot otherwise tell apart.
//
// Ties break on the key so the order is deterministic: every unit of a fresh
// config is due at the same instant, and Go randomizes map iteration.
func (s *schedule) nextDue() (nameKey, time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var (
		best  nameKey
		when  time.Time
		found bool
	)
	for key, due := range s.due {
		if !found || due.Before(when) || (due.Equal(when) && lessKey(key, best)) {
			best, when, found = key, due, true
		}
	}
	return best, when, found
}

// remove drops a unit from the schedule. A group whose cache an operator
// cleared keeps its schedule: the addresses are gone, and asking again is how
// they come back.
func (s *schedule) remove(key nameKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.due, key)
}

// keys returns every scheduled unit, sorted, for a test and for the show
// command to read a stable order.
func (s *schedule) keys() []nameKey {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]nameKey, 0, len(s.due))
	for key := range s.due {
		out = append(out, key)
	}
	sort.Slice(out, func(i, j int) bool { return lessKey(out[i], out[j]) })
	return out
}

func lessKey(a, b nameKey) bool {
	if a.group != b.group {
		return a.group < b.group
	}
	if a.name != b.name {
		return a.name < b.name
	}
	return a.family.label < b.family.label
}

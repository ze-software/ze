// Design: docs/guide/pppoe.md -- numbering a LAN unit from a delegated prefix
// Related: delegated_prefix.go -- the delegated-prefix config this state serves
// Related: delegated_consumer.go -- the worker that fills it from DHCPv6 lease events
// Related: reconcile_ra.go -- advertises each unit's /64 from it
//
// RFC: rfc/short/rfc9915.md

package iface

import (
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/ndp"
)

// lifetimeInfiniteSeconds is the DHCPv6 lifetime meaning infinite (RFC 9915
// Section 7.7), the same value the RA lifetime fields use.
const lifetimeInfiniteSeconds = 0xffffffff

// LifetimeDeadline is the moment a delegated lifetime runs out, or infinite.
// It is a deadline rather than a duration so every consumer computes what
// remains at the moment it uses the value, never at the moment it read the
// lease. The zero value is a deadline already passed.
type LifetimeDeadline struct {
	at       time.Time
	infinite bool
}

// newLifetimeDeadline returns the deadline of a lifetime of seconds counted
// from received, the time of the Reply that carried it.
func newLifetimeDeadline(received time.Time, seconds int) LifetimeDeadline {
	if seconds >= lifetimeInfiniteSeconds {
		return LifetimeDeadline{infinite: true}
	}
	if seconds < 0 {
		seconds = 0
	}
	return LifetimeDeadline{at: received.Add(time.Duration(seconds) * time.Second)}
}

// Remaining returns the whole seconds left at now, rounded down so the answer
// never exceeds what remains, and ndp.LifetimeInfinity for an infinite
// deadline. A finite deadline never answers infinity: it stops one short.
func (d LifetimeDeadline) Remaining(now time.Time) uint32 {
	if d.infinite {
		return ndp.LifetimeInfinity
	}
	left := d.at.Sub(now)
	if left <= 0 {
		return 0
	}
	seconds := left / time.Second // integer division rounds down
	if seconds >= time.Duration(ndp.LifetimeInfinity) {
		return ndp.LifetimeInfinity - 1
	}
	return uint32(seconds) //nolint:gosec // bounded below LifetimeInfinity just above
}

// Equal reports whether two deadlines name the same moment.
func (d LifetimeDeadline) Equal(other LifetimeDeadline) bool {
	return d.infinite == other.infinite && d.at.Equal(other.at)
}

// DelegatedAdvert is one /64 a unit takes from a delegated prefix: the address
// the unit holds in it, and the deadlines it inherits from the delegation.
type DelegatedAdvert struct {
	// Source is the pppoe-client whose lease carries the delegation.
	Source string
	// Address is the unit's address with its /64 length. Its masked form is
	// the prefix advertised for stateless address autoconfiguration.
	Address netip.Prefix
	// Delegated is the whole delegated prefix, advertised in a Route
	// Information option (RFC 7084 Section 4.3 L-3).
	Delegated netip.Prefix
	// Valid and Preferred are the delegation's deadlines.
	Valid     LifetimeDeadline
	Preferred LifetimeDeadline
}

// Equal reports whether two adverts carry the same values.
func (a DelegatedAdvert) Equal(other DelegatedAdvert) bool {
	return a.Source == other.Source &&
		a.Address == other.Address &&
		a.Delegated == other.Delegated &&
		a.Valid.Equal(other.Valid) &&
		a.Preferred.Equal(other.Preferred)
}

// Lifetimes returns the valid and preferred lifetimes that remain at now, the
// preferred one never above the valid one (RFC 4861 Section 4.6.2).
//
// RFC 9915 Section 6.3: "If the client uses a delegated prefix to configure
// addresses on interfaces on itself or other nodes behind it, the preferred
// and valid lifetimes of those addresses MUST be no longer than the remaining
// preferred and valid lifetimes, respectively, for the delegated prefix at any
// time."
func (a DelegatedAdvert) Lifetimes(now time.Time) (valid, preferred uint32) {
	valid = a.Valid.Remaining(now)
	return valid, min(a.Preferred.Remaining(now), valid)
}

// heldDelegation is one delegated prefix a DHCPv6 lease holds.
type heldDelegation struct {
	prefix    netip.Prefix
	valid     LifetimeDeadline
	preferred LifetimeDeadline
}

// delegations holds every delegated prefix the DHCPv6 clients hold, and which
// of them a unit has taken space from. It is filled by the delegation worker
// and read by the RA reconcile and by the DHCPv6 client before a Release. Safe
// for concurrent use.
type delegations struct {
	mu sync.Mutex
	// held maps source -> delegated prefix -> delegation. Bounded by the
	// prefixes the DHCPv6 clients hold, one entry per lease binding.
	held map[string]map[netip.Prefix]heldDelegation
	// inUse maps source -> delegated prefix -> the valid deadline of a
	// delegation a unit took space from. A host's SLAAC address from that
	// space lives no longer than the delegation, so the mark ends there.
	inUse map[string]map[netip.Prefix]LifetimeDeadline
}

// heldDelegations is the process's one table. The DHCPv6 client runs in this
// process: it is an iface backend plugin and already installs its addresses
// through this package.
var heldDelegations = &delegations{
	held:  make(map[string]map[netip.Prefix]heldDelegation),
	inUse: make(map[string]map[netip.Prefix]LifetimeDeadline),
}

// record stores or forgets one delegated prefix of source. A lease time of
// zero is the expiry the DHCPv6 client publishes: the delegation and its
// in-use mark both end, because nothing derived from it outlives it.
func (t *delegations) record(source string, prefix netip.Prefix, received time.Time, validSeconds, preferredSeconds int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if validSeconds == 0 {
		delete(t.held[source], prefix)
		delete(t.inUse[source], prefix)
		return
	}
	if t.held[source] == nil {
		t.held[source] = make(map[netip.Prefix]heldDelegation)
	}
	t.held[source][prefix] = heldDelegation{
		prefix:    prefix,
		valid:     newLifetimeDeadline(received, validSeconds),
		preferred: newLifetimeDeadline(received, preferredSeconds),
	}
}

// chosen returns the delegation a unit numbers from: the lowest live prefix
// of source, so a lease holding two prefixes always yields the same /64 and a
// renew of either never moves the unit.
func (t *delegations) chosen(source string, now time.Time) (heldDelegation, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	var live []heldDelegation
	for _, d := range t.held[source] {
		if d.valid.Remaining(now) > 0 {
			live = append(live, d)
		}
	}
	if len(live) == 0 {
		return heldDelegation{}, false
	}
	slices.SortFunc(live, func(a, b heldDelegation) int {
		if c := a.prefix.Addr().Compare(b.prefix.Addr()); c != 0 {
			return c
		}
		return a.prefix.Bits() - b.prefix.Bits()
	})
	return live[0], true
}

// claim marks a delegation as one a unit took space from.
func (t *delegations) claim(source string, d heldDelegation) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.inUse[source] == nil {
		t.inUse[source] = make(map[netip.Prefix]LifetimeDeadline)
	}
	t.inUse[source][d.prefix] = d.valid
}

// claimed reports whether space from prefix is still outstanding at now.
func (t *delegations) claimed(source string, prefix netip.Prefix, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	valid, ok := t.inUse[source][prefix]
	if !ok {
		return false
	}
	return valid.Remaining(now) > 0
}

// DelegatedPrefixStatus is one delegated prefix a pppoe-client's DHCPv6 lease
// holds, as `show interface pppoe-client` reports it. The lifetimes are the
// whole seconds that remain at the moment of the answer, never the values the
// Reply carried, and 4294967295 is infinite (RFC 9915 Section 7.7).
type DelegatedPrefixStatus struct {
	Prefix            netip.Prefix `json:"prefix"`
	ValidLifetime     uint32       `json:"valid-lifetime"`
	PreferredLifetime uint32       `json:"preferred-lifetime"`
}

// statusOf returns the live delegations of source at now, sorted by prefix.
// It is never nil, so a client that holds none answers an empty list. A
// delegation whose valid lifetime ran out is not listed: the client no longer
// holds it, even before the expiry event removes it from the table.
func (t *delegations) statusOf(source string, now time.Time) []DelegatedPrefixStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]DelegatedPrefixStatus, 0, len(t.held[source]))
	for _, d := range t.held[source] {
		valid := d.valid.Remaining(now)
		if valid == 0 {
			continue
		}
		out = append(out, DelegatedPrefixStatus{
			Prefix:        d.prefix,
			ValidLifetime: valid,
			// RFC 4861 Section 4.6.2: the preferred lifetime never exceeds
			// the valid one, the bound DelegatedAdvert.Lifetimes keeps too.
			PreferredLifetime: min(d.preferred.Remaining(now), valid),
		})
	}
	slices.SortFunc(out, func(a, b DelegatedPrefixStatus) int {
		if c := a.Prefix.Addr().Compare(b.Prefix.Addr()); c != 0 {
			return c
		}
		return a.Prefix.Bits() - b.Prefix.Bits()
	})
	return out
}

// DelegatedPrefixInUse reports whether a unit took space from prefix, which
// the DHCPv6 client of source holds, and that space can still be in use: an
// address on a unit, or a host's address formed from an advertised /64. The
// DHCPv6 client asks before it releases the prefix (RFC 9915 Section 6.3).
// Safe for concurrent use.
func DelegatedPrefixInUse(source string, prefix netip.Prefix) bool {
	return heldDelegations.claimed(source, prefix, time.Now())
}

// delegatedRefusal is a delegated-prefix entry that got no /64: the delegation
// the server chose cannot hold the configured subnet id.
type delegatedRefusal struct {
	source    string
	subnetID  uint16
	delegated netip.Prefix
	err       error
}

// claimDelegatedAdverts returns the /64 of each delegated-prefix entry of u
// whose source holds a live delegation, and records the claim on it, so the
// DHCPv6 client never releases a prefix a unit took space from. An entry whose
// delegation is too small for its subnet id is returned as a refusal instead.
func claimDelegatedAdverts(u *unitEntry, now time.Time) ([]DelegatedAdvert, []delegatedRefusal) {
	if u.IPv6 == nil {
		return nil, nil
	}
	var adverts []DelegatedAdvert
	var refusals []delegatedRefusal
	for _, dp := range u.IPv6.DelegatedPrefixes {
		d, ok := heldDelegations.chosen(dp.Source, now)
		if !ok {
			continue
		}
		addr, err := deriveDelegatedAddress(d.prefix, dp.SubnetID, dp.InterfaceID)
		if err != nil {
			refusals = append(refusals, delegatedRefusal{source: dp.Source, subnetID: dp.SubnetID, delegated: d.prefix, err: err})
			continue
		}
		heldDelegations.claim(dp.Source, d)
		adverts = append(adverts, DelegatedAdvert{
			Source:    dp.Source,
			Address:   addr,
			Delegated: d.prefix,
			Valid:     d.valid,
			Preferred: d.preferred,
		})
	}
	return adverts, refusals
}

// NewDelegatedAdvert returns an advert whose deadlines are the given finite
// moments. The component builds its own adverts from the lease events; this
// serves a caller that already holds absolute deadlines, such as a sender's
// tests.
func NewDelegatedAdvert(source string, address, delegated netip.Prefix, validUntil, preferredUntil time.Time) DelegatedAdvert {
	return DelegatedAdvert{
		Source:    source,
		Address:   address,
		Delegated: delegated,
		Valid:     LifetimeDeadline{at: validUntil},
		Preferred: LifetimeDeadline{at: preferredUntil},
	}
}

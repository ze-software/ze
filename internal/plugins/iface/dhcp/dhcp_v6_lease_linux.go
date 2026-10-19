// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Overview: dhcp_v6_linux.go -- DHCPv6 lease state machine
// Related: dhcp_v6_exchange_linux.go -- the exchanges that read and feed the record
// RFC: rfc/short/rfc9915.md

//go:build linux

package ifacedhcp

import (
	"net/netip"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/iana"
)

// v6Infinity is the lifetime and timer value 0xffffffff.
//
// RFC 9915 Section 7.7: "The value 0xffffffff is taken to mean "infinity"
// when used as a lifetime (as in [RFC4861]) or a value for T1 or T2."
//
// Lifetimes and timers both compare against this constant.
const v6Infinity = time.Duration(0xffffffff) * time.Second

const (
	// v6BindingsMax bounds the addresses, and separately the prefixes, one
	// lease records: a Reply is untrusted input.
	v6BindingsMax = 16

	// v6ClientTimerFloor is the earliest a client-chosen T1 fires after the
	// Reply, so a lease whose lifetimes give a zero T1 is never renewed at
	// once.
	v6ClientTimerFloor = 1 * time.Second
)

// v6Kind says what a binding leases.
type v6Kind uint8

const (
	v6KindUnspecified v6Kind = iota //nolint:unused // Zero value reserved as Unspecified; no binding has it.
	// v6KindAddress: an IA Address inside the IA_NA, installed as a /128.
	v6KindAddress
	// v6KindPrefix: an IA Prefix inside the IA_PD, published, not installed.
	v6KindPrefix
)

// v6Binding is one leased address or delegated prefix, with the lifetimes the
// last Reply gave and the instant that Reply was received.
type v6Binding struct {
	kind      v6Kind
	prefix    netip.Prefix // a /128 for an address
	preferred time.Duration
	valid     time.Duration
	received  time.Time
}

// expires reports whether the binding has a finite valid lifetime, and when
// it runs out.
func (b *v6Binding) expires() (time.Time, bool) {
	if b.valid == v6Infinity {
		return time.Time{}, false
	}
	return b.received.Add(b.valid), true
}

func (b *v6Binding) expiredAt(now time.Time) bool {
	end, finite := b.expires()
	if !finite {
		return false
	}
	return !now.Before(end)
}

// v6Instant is a point in time, or never.
type v6Instant struct {
	at    time.Time
	never bool
}

func v6Never() v6Instant { return v6Instant{never: true} }

// earlier is the earlier of i and j.
func (i v6Instant) earlier(j v6Instant) v6Instant {
	if i.never {
		return j
	}
	if j.never {
		return i
	}
	if j.at.Before(i.at) {
		return j
	}
	return i
}

// v6IATimers is the T1 and T2 of one IA as the last Reply carrying it gave
// them, measured from that Reply's reception.
type v6IATimers struct {
	t1, t2   time.Duration
	received time.Time
	// retryAt delays the renewal of an IA the last Reply omitted, so its past
	// T1 does not fire again at once. The zero time means no delay: it is
	// the earliest instant, so it never wins the comparison it feeds.
	retryAt time.Time
}

// v6Lease is the client's record of its bindings with one server. It is
// owned by the v6 worker goroutine and never shared.
type v6Lease struct {
	server   dhcpv6.DUID
	bindings []v6Binding
	timers   [3]v6IATimers // indexed by v6Kind
	dns      []string
}

// v6Applied is what applying one Reply changed.
type v6Applied struct {
	installed []v6Binding // new or extended, to install or refresh
	removed   []v6Binding // valid lifetime 0, to remove
	noBinding bool        // an IA answered NoBinding
	discarded int         // addresses, prefixes or IAs refused
}

// apply records a Reply received at received.
//
// RFC 9915 Section 18.2.10.1: "Add any new leases in the IA option to the IA
// as recorded by the client." "Update lifetimes for any leases in the IA
// option that the client already has recorded in the IA." "Discard any leases
// from the IA, as recorded by the client, that have a valid lifetime of 0 in
// the IA Address or IA Prefix option." "Leave unchanged any information about
// leases the client has recorded in the IA but that were not included in the
// IA from the server."
//
// apply performs those steps on the lease record.
func (l *v6Lease) apply(m *dhcpv6.Message, received time.Time, iaidNA, iaidPD [4]byte) v6Applied {
	var out v6Applied
	seen := [3]bool{}
	for _, na := range m.Options.IANA() {
		if na.IaId != iaidNA {
			continue
		}
		var leases []v6Binding
		for _, a := range na.Options.Addresses() {
			b, ok := v6AddressBinding(a, received)
			if !ok {
				out.discarded++
				continue
			}
			leases = append(leases, b)
		}
		if l.applyIA(v6KindAddress, na.T1, na.T2, na.Options.Status(), leases, received, &out) {
			seen[v6KindAddress] = true
		}
	}
	for _, pd := range m.Options.IAPD() {
		if pd.IaId != iaidPD {
			continue
		}
		var leases []v6Binding
		for _, p := range pd.Options.Prefixes() {
			b, ok := v6PrefixBinding(p, received)
			if !ok {
				out.discarded++
				continue
			}
			leases = append(leases, b)
		}
		if l.applyIA(v6KindPrefix, pd.T1, pd.T2, pd.Options.Status(), leases, received, &out) {
			seen[v6KindPrefix] = true
		}
	}
	for _, kind := range []v6Kind{v6KindAddress, v6KindPrefix} {
		if !seen[kind] && l.holds(kind) {
			l.timers[kind].retryAt = received.Add(v6RenTimeout)
		}
	}
	// RFC 9915 Section 18.2.10.5: "the client SHOULD stop using the
	// previously received configuration information": a Reply without the
	// DNS Recursive Name Server option leaves the lease with no servers.
	l.dns = l.dns[:0]
	for _, s := range m.Options.DNS() {
		l.dns = append(l.dns, s.String())
	}
	return out
}

// applyIA applies one IA of a Reply. It answers whether the IA was taken.
func (l *v6Lease) applyIA(kind v6Kind, t1, t2 time.Duration, status *dhcpv6.OptStatusCode, leases []v6Binding, received time.Time, out *v6Applied) bool {
	if status != nil {
		//exhaustive:ignore // only the status codes that change IA processing; the set is open (wire)
		switch status.StatusCode {
		case iana.StatusNoBinding:
			out.noBinding = true
			return false
		case iana.StatusNoAddrsAvail, iana.StatusNoPrefixAvail:
			// RFC 9915 Section 18.2.10.1: "The client uses the addresses,
			// delegated prefixes, and other information from any IAs that do
			// not contain a Status Code option with the NoAddrsAvail or
			// NoPrefixAvail status code."
			return false
		}
	}
	// RFC 9915 Section 21.4: "If a client receives an IA_NA with T1 greater
	// than T2 and both T1 and T2 are greater than 0, the client discards the
	// IA_NA option and processes the remainder of the message as though the
	// server had not included the invalid IA_NA option."
	// RFC 9915 Section 21.21 states the same for IA_PD.
	if t1 > 0 && t2 > 0 && t1 > t2 {
		out.discarded++
		return false
	}
	// RFC 9915 Section 18.2.10.1: "Calculate T1 and T2 times (based on T1
	// and T2 values sent in the message and the message reception time), if
	// appropriate for the IA type."
	l.timers[kind] = v6IATimers{t1: t1, t2: t2, received: received}
	for _, b := range leases {
		if b.valid == 0 {
			if old, ok := l.remove(b.kind, b.prefix); ok {
				out.removed = append(out.removed, old)
			}
			continue
		}
		if l.upsert(b) {
			out.installed = append(out.installed, b)
			continue
		}
		out.discarded++
	}
	return true
}

// v6AddressBinding converts an IA Address; false when it MUST be discarded.
//
// RFC 9915 Section 21.6: "The client MUST discard any addresses for which the
// preferred lifetime is greater than the valid lifetime."
//
// The false answer is that discard.
func v6AddressBinding(a *dhcpv6.OptIAAddress, received time.Time) (v6Binding, bool) {
	addr, ok := netip.AddrFromSlice(a.IPv6Addr)
	if !ok {
		return v6Binding{}, false
	}
	if a.PreferredLifetime > a.ValidLifetime {
		return v6Binding{}, false
	}
	// RFC 9915 Section 18.2.10.1: "Addresses obtained from an IA_NA or IA_TA
	// MUST NOT be used to form an implicit prefix with a length other than
	// 128."
	return v6Binding{
		kind:      v6KindAddress,
		prefix:    netip.PrefixFrom(addr.Unmap(), 128),
		preferred: a.PreferredLifetime,
		valid:     a.ValidLifetime,
		received:  received,
	}, true
}

// v6PrefixBinding converts an IA Prefix; false when it MUST be discarded.
//
// RFC 9915 Section 21.22: "The client MUST discard any prefixes for which the
// preferred lifetime is greater than the valid lifetime."
//
// The false answer is that discard.
func v6PrefixBinding(p *dhcpv6.OptIAPrefix, received time.Time) (v6Binding, bool) {
	if p.Prefix == nil {
		return v6Binding{}, false
	}
	addr, ok := netip.AddrFromSlice(p.Prefix.IP)
	if !ok {
		return v6Binding{}, false
	}
	bits, _ := p.Prefix.Mask.Size()
	if p.PreferredLifetime > p.ValidLifetime {
		return v6Binding{}, false
	}
	return v6Binding{
		kind:      v6KindPrefix,
		prefix:    netip.PrefixFrom(addr.Unmap(), bits).Masked(),
		preferred: p.PreferredLifetime,
		valid:     p.ValidLifetime,
		received:  received,
	}, true
}

// upsert records b, replacing the binding of the same prefix. It answers
// false when b is new and the kind's bound is reached.
func (l *v6Lease) upsert(b v6Binding) bool {
	count := 0
	for i := range l.bindings {
		if l.bindings[i].kind != b.kind {
			continue
		}
		if l.bindings[i].prefix == b.prefix {
			l.bindings[i] = b
			return true
		}
		count++
	}
	if count >= v6BindingsMax {
		return false
	}
	l.bindings = append(l.bindings, b)
	return true
}

// remove forgets the binding of prefix and answers it.
func (l *v6Lease) remove(kind v6Kind, prefix netip.Prefix) (v6Binding, bool) {
	for i := range l.bindings {
		if l.bindings[i].kind == kind && l.bindings[i].prefix == prefix {
			old := l.bindings[i]
			l.bindings = append(l.bindings[:i], l.bindings[i+1:]...)
			return old, true
		}
	}
	return v6Binding{}, false
}

// holds reports whether the lease records any binding of kind.
func (l *v6Lease) holds(kind v6Kind) bool {
	for i := range l.bindings {
		if l.bindings[i].kind == kind {
			return true
		}
	}
	return false
}

// live is the bindings of kind whose valid lifetime has not run out at now.
func (l *v6Lease) live(kind v6Kind, now time.Time) []v6Binding {
	var out []v6Binding
	for i := range l.bindings {
		b := l.bindings[i]
		if b.kind == kind && !b.expiredAt(now) {
			out = append(out, b)
		}
	}
	return out
}

// prune forgets the bindings expired at now and answers them.
func (l *v6Lease) prune(now time.Time) []v6Binding {
	var gone []v6Binding
	kept := l.bindings[:0]
	for _, b := range l.bindings {
		if b.expiredAt(now) {
			gone = append(gone, b)
			continue
		}
		kept = append(kept, b)
	}
	l.bindings = kept
	return gone
}

func (l *v6Lease) empty() bool { return len(l.bindings) == 0 }

// nextExpiry is the earliest instant a binding runs out.
func (l *v6Lease) nextExpiry() v6Instant {
	next := v6Never()
	for i := range l.bindings {
		if end, finite := l.bindings[i].expires(); finite {
			next = next.earlier(v6Instant{at: end})
		}
	}
	return next
}

// lastExpiry is the instant every binding has run out, never when one is
// infinite.
//
// RFC 9915 Section 18.2.5: the Rebind exchange's MRD is the "Remaining time
// until valid lifetimes of all leases in all IAs have expired".
func (l *v6Lease) lastExpiry() v6Instant {
	var last time.Time
	for i := range l.bindings {
		end, finite := l.bindings[i].expires()
		if !finite {
			return v6Never()
		}
		if end.After(last) {
			last = end
		}
	}
	return v6Instant{at: last}
}

// renewTimes is the earliest T1 and T2 across the IAs the lease holds.
//
// RFC 9915 Section 18.2.4: "the client MUST select T1 and T2 times from all
// IA options that will guarantee that the client initiates transmissions of
// Renew/Rebind messages not later than at the T1/T2 times associated with any
// of the client's bindings (earliest T1/T2)."
//
// Infinity stays infinity in both answers.
func (l *v6Lease) renewTimes() (t1, t2 v6Instant) {
	t1, t2 = v6Never(), v6Never()
	for _, kind := range []v6Kind{v6KindAddress, v6KindPrefix} {
		if !l.holds(kind) {
			continue
		}
		at1, at2 := l.iaTimes(kind)
		t1, t2 = t1.earlier(at1), t2.earlier(at2)
	}
	return t1, t2
}

// iaTimes is the T1 and T2 instants of one IA.
//
// RFC 9915 Section 7.7: "A client will never attempt to extend the lifetimes
// of any addresses in an IA with T1 set to 0xffffffff. A client will never
// attempt to use a Rebind message to locate a different server to extend the
// lifetimes of any addresses in an IA with T2 set to 0xffffffff."
//
// An infinite T1 or T2 is answered as never.
func (l *v6Lease) iaTimes(kind v6Kind) (t1, t2 v6Instant) {
	tm := l.timers[kind]
	d1, d2 := tm.t1, tm.t2
	base := l.shortestPreferred(kind)
	// RFC 9915 Section 14.2: "When T1 and/or T2 values are set to 0, the
	// client MUST choose a time to avoid message storms. In particular, it
	// MUST NOT transmit immediately."
	// RFC 9915 Section 21.4: "Recommended values for T1 and T2 are 0.5 and
	// 0.8 times the shortest preferred lifetime of the addresses in the IA
	// that the server is willing to extend, respectively."
	if d1 == 0 {
		d1 = v6ClientTimer(base, 0.5)
	}
	if d2 == 0 {
		d2 = max(v6ClientTimer(base, 0.8), d1)
	}
	t1 = v6Never()
	if d1 != v6Infinity {
		at := tm.received.Add(d1)
		if tm.retryAt.After(at) {
			at = tm.retryAt
		}
		t1 = v6Instant{at: at}
	}
	t2 = v6Never()
	if d2 != v6Infinity {
		t2 = v6Instant{at: tm.received.Add(d2)}
	}
	return t1, t2
}

// v6ClientTimer is factor times base, infinity for an infinite base, and at
// least v6ClientTimerFloor.
func v6ClientTimer(base time.Duration, factor float64) time.Duration {
	if base == v6Infinity {
		return v6Infinity
	}
	return max(v6Scale(base, factor), v6ClientTimerFloor)
}

// shortestPreferred is the shortest preferred lifetime among the kind's
// bindings the server is willing to extend (preferred above 0); the shortest
// valid lifetime when the server extends none.
func (l *v6Lease) shortestPreferred(kind v6Kind) time.Duration {
	shortest, fallback := time.Duration(-1), time.Duration(-1)
	for i := range l.bindings {
		b := l.bindings[i]
		if b.kind != kind {
			continue
		}
		if b.preferred > 0 && (shortest < 0 || b.preferred < shortest) {
			shortest = b.preferred
		}
		if fallback < 0 || b.valid < fallback {
			fallback = b.valid
		}
	}
	if shortest >= 0 {
		return shortest
	}
	return max(fallback, 0)
}

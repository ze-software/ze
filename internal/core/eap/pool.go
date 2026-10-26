// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- Virtual IP pool and lease registry
// RFC: rfc/short/rfc7296.md -- Configuration Payload, INTERNAL_IP4_ADDRESS (Section 2.19)
// RFC: rfc/short/rfc3948.md -- locally unique inner addresses (Section 5.1)

package eap

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"sync"
)

var (
	ErrPoolExhausted   = errors.New("pool: all addresses allocated")
	ErrNotAllocated    = errors.New("pool: address not allocated")
	ErrFamilyNotServed = errors.New("pool: the pool serves no address of the requested family")
)

// Family names the address family of a lease. The zero value is no family, so a
// Family nobody set is never mistaken for IPv4.
type Family uint8

const (
	FamilyUnspecified Family = iota
	FamilyIPv4
	FamilyIPv6
)

// String names the family for a log line or an error.
func (f Family) String() string {
	switch f {
	case FamilyUnspecified:
		return "unspecified"
	case FamilyIPv4:
		return "ipv4"
	case FamilyIPv6:
		return "ipv6"
	}
	return "family(" + strconv.Itoa(int(f)) + ")"
}

// Prefix bounds NewPool enforces itself, so a pool built past the config validator still
// cannot name a range with no host, nor one whose scan costs more than the bound states.
const (
	poolIPv4BitsMin = 8
	poolIPv4BitsMax = 30
	poolIPv6BitsMin = 48
	poolIPv6BitsMax = 126
	// poolIPv6HostsMax caps the host identifiers an IPv6 pool hands out (1 to 65535), so
	// the allocation scan has a bound whatever the prefix length.
	poolIPv6HostsMax = 1<<16 - 1
	// registryLastMax bounds the remembered last addresses (one per owner, pool and
	// family). When full, a release is not remembered; uniqueness never depends on it.
	registryLastMax = 1 << 16
)

// Pool describes one configured address range and the attributes served with it. It
// leases nothing itself: every lease is taken through a Registry, which is the only
// uniqueness authority. A Pool is rebuilt when its configuration changes; the Registry and
// its live leases are not, so a rebuild can never hand a live address out again.
//
// Range, DNS and name are immutable after NewPool. The two cursors are written only while
// the Registry mutex is held.
type Pool struct {
	name string

	prefix4 netip.Prefix // invalid when the pool serves no IPv4
	hosts4  uint64       // usable host offsets 1..hosts4 (network and broadcast excluded)

	prefix6   netip.Prefix // invalid when the pool serves no IPv6
	host6Mask uint64       // the host bits that fall inside the low 64 bits of the address
	hosts6    uint64       // host identifiers 1..hosts6

	dns4 []netip.Addr
	dns6 []netip.Addr

	cursor4 uint64 // last offset handed out by the rotating scan; guarded by Registry.mu
	cursor6 uint64 // same, IPv6
}

// NewPool builds the descriptor of the pool named name from an optional IPv4 CIDR, an
// optional IPv6 CIDR and the DNS servers served with its leases. It refuses a range
// outside /8 to /30 (IPv4) or /48 to /126 (IPv6), a pool with no range at all, and a DNS
// entry that is not an address.
func NewPool(name, ipv4CIDR, ipv6CIDR string, dns []string) (*Pool, error) {
	p := &Pool{name: name}

	if ipv4CIDR != "" {
		prefix, err := netip.ParsePrefix(ipv4CIDR)
		if err != nil {
			return nil, fmt.Errorf("pool %q: range %q: %w", name, ipv4CIDR, err)
		}
		if !prefix.Addr().Is4() {
			return nil, fmt.Errorf("pool %q: range %q is not an IPv4 prefix", name, ipv4CIDR)
		}
		bits := prefix.Bits()
		if bits < poolIPv4BitsMin {
			return nil, fmt.Errorf("pool %q: range %q is shorter than /%d", name, ipv4CIDR, poolIPv4BitsMin)
		}
		if bits > poolIPv4BitsMax {
			return nil, fmt.Errorf("pool %q: range %q leaves no host address beside the network and broadcast addresses", name, ipv4CIDR)
		}
		p.prefix4 = prefix.Masked()
		p.hosts4 = (uint64(1) << uint(32-bits)) - 2
	}

	if ipv6CIDR != "" {
		prefix, err := netip.ParsePrefix(ipv6CIDR)
		if err != nil {
			return nil, fmt.Errorf("pool %q: range6 %q: %w", name, ipv6CIDR, err)
		}
		if !prefix.Addr().Is6() {
			return nil, fmt.Errorf("pool %q: range6 %q is not an IPv6 prefix", name, ipv6CIDR)
		}
		if prefix.Addr().Is4In6() {
			return nil, fmt.Errorf("pool %q: range6 %q is an IPv4-mapped prefix, not an IPv6 one", name, ipv6CIDR)
		}
		bits := prefix.Bits()
		if bits < poolIPv6BitsMin {
			return nil, fmt.Errorf("pool %q: range6 %q is shorter than /%d", name, ipv6CIDR, poolIPv6BitsMin)
		}
		if bits > poolIPv6BitsMax {
			return nil, fmt.Errorf("pool %q: range6 %q leaves no host address beside the subnet-router anycast address", name, ipv6CIDR)
		}
		p.prefix6 = prefix.Masked()
		hostBits := 128 - bits
		// A uint64 shift of 64 or more yields 0 in Go, so a host part wider than the low
		// 64 bits takes the whole low half.
		if hostBits >= 64 {
			p.host6Mask = ^uint64(0)
		} else {
			p.host6Mask = (uint64(1) << uint(hostBits)) - 1
		}
		p.hosts6 = min(p.host6Mask, poolIPv6HostsMax)
	}

	if !p.prefix4.IsValid() && !p.prefix6.IsValid() {
		return nil, fmt.Errorf("pool %q: neither range nor range6 is set", name)
	}

	for _, server := range dns {
		addr, err := netip.ParseAddr(server)
		if err != nil {
			return nil, fmt.Errorf("pool %q: dns %q: %w", name, server, err)
		}
		if addr.Is4() {
			p.dns4 = append(p.dns4, addr)
			continue
		}
		p.dns6 = append(p.dns6, addr)
	}

	return p, nil
}

// Name returns the configured pool name.
func (p *Pool) Name() string { return p.name }

// Prefix4 returns the IPv4 range, invalid when the pool serves no IPv4.
func (p *Pool) Prefix4() netip.Prefix { return p.prefix4 }

// Prefix6 returns the IPv6 range, invalid when the pool serves no IPv6.
func (p *Pool) Prefix6() netip.Prefix { return p.prefix6 }

// DNS returns a copy of the DNS servers of one family served with a lease of that family.
func (p *Pool) DNS(family Family) []netip.Addr {
	if family == FamilyIPv4 {
		return slices.Clone(p.dns4)
	}
	if family == FamilyIPv6 {
		return slices.Clone(p.dns6)
	}
	return nil
}

// Serves reports whether the pool has a range of family.
func (p *Pool) Serves(family Family) bool {
	if family == FamilyIPv4 {
		return p.prefix4.IsValid()
	}
	if family == FamilyIPv6 {
		return p.prefix6.IsValid()
	}
	return false
}

// Capacity returns how many addresses the pool can lease across both families.
func (p *Pool) Capacity() uint64 {
	var n uint64
	if p.prefix4.IsValid() {
		n += p.hosts4
	}
	if p.prefix6.IsValid() {
		n += p.hosts6
	}
	return n
}

// Contains reports whether addr is an address this pool may lease: inside a range, and
// neither the IPv4 network or broadcast address nor the IPv6 subnet-router anycast address.
func (p *Pool) Contains(addr netip.Addr) bool {
	if addr.Is4() {
		if !p.prefix4.Contains(addr) {
			return false
		}
		offset := uint64(binary.BigEndian.Uint32(addr.AsSlice()) - binary.BigEndian.Uint32(p.prefix4.Addr().AsSlice()))
		return offset >= 1 && offset <= p.hosts4
	}
	if !p.prefix6.Contains(addr) {
		return false
	}
	return addr != p.prefix6.Addr()
}

// hostAt returns the address at a host offset of one family; offset is 1-based.
func (p *Pool) hostAt(family Family, offset uint64) netip.Addr {
	if family == FamilyIPv4 {
		var a [4]byte
		binary.BigEndian.PutUint32(a[:], binary.BigEndian.Uint32(p.prefix4.Addr().AsSlice())+uint32(offset))
		return netip.AddrFrom4(a)
	}
	a := p.prefix6.Addr().As16()
	low := binary.BigEndian.Uint64(a[8:])
	binary.BigEndian.PutUint64(a[8:], low|(offset&p.host6Mask))
	return netip.AddrFrom16(a)
}

// inPrefixCandidate maps a requested IPv6 address outside the prefix onto the address in
// the prefix that carries the same interface identifier (RFC 7296 Section 3.15.3: the
// initiator MAY send the interface identifier it wants inside the responder's prefix).
func (p *Pool) inPrefixCandidate(requested netip.Addr) netip.Addr {
	a := p.prefix6.Addr().As16()
	r := requested.As16()
	low := binary.BigEndian.Uint64(a[8:])
	binary.BigEndian.PutUint64(a[8:], low|(binary.BigEndian.Uint64(r[8:])&p.host6Mask))
	return netip.AddrFrom16(a)
}

// Lease is one address held for one owner, drawn from one pool. It is returned by
// Registry.Acquire and MUST be handed back to Registry.Release once per Acquire.
type Lease struct {
	addr    netip.Addr
	family  Family
	pool    string
	owner   string
	holders int // guarded by Registry.mu; zero once released
}

// Addr returns the leased address.
func (l *Lease) Addr() netip.Addr { return l.addr }

// Family returns the family of the leased address.
func (l *Lease) Family() Family { return l.family }

// Pool returns the name of the pool the lease was drawn from.
func (l *Lease) Pool() string { return l.pool }

// ownerSlot names the one lease an owner may hold in one pool for one family.
type ownerSlot struct {
	owner  string
	pool   string
	family Family
}

// Registry owns every live virtual-IP lease across every pool. It is created once per
// engine and never replaced, so pool edits cannot forget a live lease. An owner holds at
// most one address per pool and family: a second Acquire for the same slot returns the same
// lease with one more holder (reauthentication, parallel re-initiation).
//
// Safe for concurrent use.
type Registry struct {
	mu      sync.Mutex
	byAddr  map[netip.Addr]*Lease
	byOwner map[ownerSlot]*Lease
	last    map[ownerSlot]netip.Addr // the owner's last released address, if still free
	lastBy  map[netip.Addr]ownerSlot // reverse of last, so a lease to anyone else forgets it
}

// NewRegistry returns an empty lease registry.
func NewRegistry() *Registry {
	return &Registry{
		byAddr:  make(map[netip.Addr]*Lease),
		byOwner: make(map[ownerSlot]*Lease),
		last:    make(map[ownerSlot]netip.Addr),
		lastBy:  make(map[netip.Addr]ownerSlot),
	}
}

// Acquire leases one address of family from pool for owner. requested is the address the
// peer asked for, or the zero Addr; an unspecified address (0.0.0.0, ::) asks for none.
// The order is: the owner's live lease in this slot; the requested address when it is
// leasable and free; the owner's last released address when still free; the next free
// address from the pool's rotating cursor. It returns ErrFamilyNotServed when the pool has
// no range of family and ErrPoolExhausted when no address is free.
//
// Every successful Acquire MUST be paired with exactly one Release of the returned lease.
func (r *Registry) Acquire(pool *Pool, owner string, family Family, requested netip.Addr) (*Lease, error) {
	if !pool.Serves(family) {
		return nil, ErrFamilyNotServed
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	slot := ownerSlot{owner: owner, pool: pool.name, family: family}
	if held, ok := r.byOwner[slot]; ok {
		held.holders++
		return held, nil
	}

	if addr, ok := r.requestedLocked(pool, family, requested); ok {
		return r.grantLocked(slot, addr), nil
	}

	if addr, ok := r.last[slot]; ok {
		if r.byAddr[addr] == nil && pool.Contains(addr) {
			return r.grantLocked(slot, addr), nil
		}
	}

	addr, err := r.scanLocked(pool, family)
	if err != nil {
		return nil, err
	}
	return r.grantLocked(slot, addr), nil
}

// requestedLocked returns the requested address, or for IPv6 its interface identifier
// inside the prefix, when it is leasable and free. MUST be called with r.mu held.
func (r *Registry) requestedLocked(pool *Pool, family Family, requested netip.Addr) (netip.Addr, bool) {
	if !requested.IsValid() {
		return netip.Addr{}, false
	}
	if requested.IsUnspecified() {
		return netip.Addr{}, false
	}
	if family == FamilyIPv4 {
		if !requested.Is4() {
			return netip.Addr{}, false
		}
		if pool.Contains(requested) && r.byAddr[requested] == nil {
			return requested, true
		}
		return netip.Addr{}, false
	}
	if !requested.Is6() {
		return netip.Addr{}, false
	}
	candidate := requested
	if !pool.prefix6.Contains(candidate) {
		candidate = pool.inPrefixCandidate(requested)
	}
	if pool.Contains(candidate) && r.byAddr[candidate] == nil {
		return candidate, true
	}
	return netip.Addr{}, false
}

// scanLocked walks the pool from one past its cursor and returns the first free address,
// moving the cursor onto it. The walk is bounded by the pool's host count, so a released
// address is reached again only after every address after it was tried. MUST be called with
// r.mu held.
func (r *Registry) scanLocked(pool *Pool, family Family) (netip.Addr, error) {
	hosts, cursor := pool.hosts4, &pool.cursor4
	if family == FamilyIPv6 {
		hosts, cursor = pool.hosts6, &pool.cursor6
	}
	for step := range hosts {
		offset := (*cursor+step)%hosts + 1
		addr := pool.hostAt(family, offset)
		if r.byAddr[addr] == nil {
			*cursor = offset
			return addr, nil
		}
	}
	return netip.Addr{}, ErrPoolExhausted
}

// grantLocked records a new lease for slot. MUST be called with r.mu held and addr free.
func (r *Registry) grantLocked(slot ownerSlot, addr netip.Addr) *Lease {
	lease := &Lease{addr: addr, family: slot.family, pool: slot.pool, owner: slot.owner, holders: 1}
	r.byAddr[addr] = lease
	r.byOwner[slot] = lease
	if prior, ok := r.lastBy[addr]; ok {
		delete(r.last, prior)
		delete(r.lastBy, addr)
	}
	if prior, ok := r.last[slot]; ok {
		delete(r.lastBy, prior)
		delete(r.last, slot)
	}
	return lease
}

// Release drops one holder of lease. At the last holder the address is free again and is
// remembered as the owner's last address of that family in that pool. It returns
// ErrNotAllocated for a lease this registry does not hold (already released, or another
// registry's), so a double release is reported rather than freeing another owner's lease.
//
// MUST be called exactly once for each successful Acquire.
func (r *Registry) Release(lease *Lease) error {
	if lease == nil {
		return ErrNotAllocated
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.byAddr[lease.addr] != lease {
		return ErrNotAllocated
	}
	lease.holders--
	if lease.holders > 0 {
		return nil
	}
	slot := ownerSlot{owner: lease.owner, pool: lease.pool, family: lease.family}
	delete(r.byAddr, lease.addr)
	delete(r.byOwner, slot)
	if len(r.last) < registryLastMax {
		r.last[slot] = lease.addr
		r.lastBy[lease.addr] = slot
	}
	return nil
}

// Held returns how many live leases were drawn from the pool named pool.
func (r *Registry) Held(pool string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	n := 0
	for _, lease := range r.byAddr {
		if lease.pool == pool {
			n++
		}
	}
	return n
}

// Available returns how many addresses of pool no live lease holds, across both families.
// A lease held inside the range counts against it whichever pool it was drawn from, so an
// edited pool reports the addresses it can really hand out.
func (r *Registry) Available(pool *Pool) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	var held uint64
	for addr := range r.byAddr {
		if pool.Contains(addr) {
			held++
		}
	}
	return pool.Capacity() - held
}

// IsHeld reports whether addr is leased to anyone.
func (r *Registry) IsHeld(addr netip.Addr) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byAddr[addr] != nil
}

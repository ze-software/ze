// Design: docs/features/dns-resolver.md -- where the router's servers come from
// Related: register.go -- raResolverOrigin, and runEngine, which owns the producer
// Related: ../../core/ndp/rdnss.go -- ParseRDNSS, the option's validity check

package iface

import (
	"encoding/json"
	"log/slog"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/core/ndp"
)

// NDUserOptionPayload is the JSON payload of (interface, nd-user-option): one
// Neighbor Discovery option the kernel handed to userspace from a Router
// Advertisement it accepted (RTM_NEWNDUSEROPT). The netlink monitor emits it.
type NDUserOptionPayload struct {
	// Name is the kernel device the RA arrived on.
	Name string `json:"name"`
	// Router is the RA's source address.
	Router string `json:"router"`
	// Option is the option as received, from its Type octet.
	Option []byte `json:"option"`
}

// raServerListMax bounds the DNS Server List of one unit. RFC 8106 Section
// 5.3.1: "However, in the case of multiple sources, the ability to store a
// total of at least three RDNSS addresses (or DNSSL domain names) from the
// multiple sources is RECOMMENDED." Eight is the most Ze advertises in one
// option itself.
const raServerListMax = 8

// raServer is one DNS Server List entry: an RDNSS address and its
// Expiration-time (RFC 8106 Section 6.1). infinite marks Lifetime
// 0xffffffff, which has no Expiration-time.
type raServer struct {
	addr     netip.Addr
	expires  time.Time
	infinite bool
}

// raServerList is one unit's DNS Server List, in Resolver Repository order.
// It is not safe for concurrent use: raRDNSS guards it.
type raServerList struct {
	servers []raServer
}

// apply processes one valid RDNSS option received at now, as RFC 8106
// Section 6.2 steps (b) to (d) say.
func (l *raServerList) apply(option ndp.RDNSS, now time.Time) {
	entry := raServer{infinite: option.Infinite()}
	if !entry.infinite {
		entry.expires = now.Add(time.Duration(option.Lifetime) * time.Second)
	}
	var added []raServer
	for _, addr := range option.Servers {
		at := slices.IndexFunc(l.servers, func(s raServer) bool { return s.addr == addr })
		// RFC 8106 Section 5.1: "A value of zero means that the RDNSS
		// addresses MUST no longer be used."
		if option.Lifetime == 0 {
			if at >= 0 {
				l.servers = slices.Delete(l.servers, at, at+1)
			}
			continue
		}
		if at >= 0 {
			l.servers[at].expires = entry.expires
			l.servers[at].infinite = entry.infinite
			continue
		}
		if slices.ContainsFunc(added, func(s raServer) bool { return s.addr == addr }) {
			continue
		}
		entry.addr = addr
		added = append(added, entry)
	}
	// RFC 8106 Section 6.2 step (d): a new address is inserted "as the first
	// one in the Resolver Repository", the option's first address first.
	l.servers = append(added, l.servers...)
	for len(l.servers) > raServerListMax {
		l.dropFirstToExpire()
	}
}

// dropFirstToExpire removes the entry with the shortest Expiration-time: RFC
// 8106 Section 6.2 step (d), for a full DNS Server List. An infinite entry
// expires last.
func (l *raServerList) dropFirstToExpire() {
	victim := -1
	for i, s := range l.servers {
		if s.infinite {
			continue
		}
		if victim < 0 || s.expires.Before(l.servers[victim].expires) {
			victim = i
		}
	}
	if victim < 0 {
		victim = len(l.servers) - 1
	}
	l.servers = slices.Delete(l.servers, victim, victim+1)
}

// prune removes every entry expired at now and reports whether any was.
func (l *raServerList) prune(now time.Time) bool {
	before := len(l.servers)
	l.servers = slices.DeleteFunc(l.servers, func(s raServer) bool {
		return !s.infinite && !now.Before(s.expires)
	})
	return len(l.servers) != before
}

// addresses answers the list's addresses, in order.
func (l *raServerList) addresses() []string {
	out := make([]string, 0, len(l.servers))
	for _, s := range l.servers {
		out = append(out, s.addr.String())
	}
	return out
}

// validity answers the lease validity of the whole list: until withdrawn when
// an entry is infinite, else the latest Expiration-time. The producer's own
// timer withdraws each earlier entry at its time, so the registry never ends
// a lease before its last address. now is the clock the Expiration-times
// were read on; the registry takes the span that remains.
func (l *raServerList) validity(now time.Time) system.LeaseValidity {
	var latest time.Time
	for _, s := range l.servers {
		if s.infinite {
			return system.UntilWithdrawn
		}
		if s.expires.After(latest) {
			latest = s.expires
		}
	}
	return system.ValidFor(latest.Sub(now))
}

// nextExpiry answers the earliest Expiration-time in the list, and false when
// no entry has one.
func (l *raServerList) nextExpiry() (time.Time, bool) {
	var earliest time.Time
	found := false
	for _, s := range l.servers {
		if s.infinite {
			continue
		}
		if !found || s.expires.Before(earliest) {
			earliest = s.expires
			found = true
		}
	}
	return earliest, found
}

// raTarget is a unit whose accept-ra leaf lets the kernel process RAs: the
// only units whose RDNSS options become records. A pppoe-client session
// carries its kernel device in device, because the dialer creates the device
// under the client's configured name and the shared resolver does not bind
// it; an interface unit leaves device empty and is resolved through iface.
type raTarget struct {
	iface  string
	unit   string
	vlanID int
	device string
}

// rdnssUnitKey names one unit's list and its record.
type rdnssUnitKey struct {
	iface string
	unit  string
}

// raRDNSS turns the RDNSS options of accepted RAs into records of the ra
// origin, one per unit, each holding that unit's DNS Server List.
//
// Lifecycle: newRARDNSS, then setTargets on every config apply, handle for
// every nd-user-option event, and withdrawPPPoESession when a pppoe-client
// session ends; stop MUST be called when the component stops, and stops the
// expiry timer. A record leaves the registry on a commit that drops its unit,
// on Lifetime 0, at its expiry, or, for a pppoe-client, when its session
// ends. An interface unit whose device goes away keeps its record until one
// of the others: no link-deleted path withdraws a unit's leases. Safe for
// concurrent use.
type raRDNSS struct {
	mu        sync.Mutex
	targets   []raTarget
	lists     map[rdnssUnitKey]*raServerList
	resolvers *system.Resolvers
	// device answers the kernel device a logical interface name is bound to.
	device func(logical string) (string, error)
	timer  *time.Timer
	log    *slog.Logger
}

// newRARDNSS answers a producer writing to resolvers.
func newRARDNSS(resolvers *system.Resolvers, log *slog.Logger) *raRDNSS {
	return &raRDNSS{
		lists:     make(map[rdnssUnitKey]*raServerList),
		resolvers: resolvers,
		device:    resolveDeviceName,
		log:       log,
	}
}

// resolveDeviceName answers the kernel device of a logical interface name
// through the shared resolver.
func resolveDeviceName(logical string) (string, error) {
	b, err := Resolve(logical)
	if err != nil {
		return "", err
	}
	return b.OsName, nil
}

// setTargets replaces the units RDNSS is read for with the enabled units of
// cfg whose accept-ra is set above 0, and the enabled pppoe-clients whose
// ipv6 accept-ra is set above 0, and withdraws the record of every unit no
// longer among them.
func (p *raRDNSS) setTargets(cfg *ifaceConfig) {
	var targets []raTarget
	forEachEnabledUnit(cfg, func(name string, u *unitEntry) {
		if u.IPv6 == nil {
			return
		}
		if u.IPv6.AcceptRA == nil {
			return
		}
		if *u.IPv6.AcceptRA <= 0 {
			return
		}
		targets = append(targets, raTarget{iface: name, unit: u.Label, vlanID: u.VLANID})
	})
	// A pppoe-client's WAN session is the home-CPE uplink: its RAs arrive on
	// the session device, which the dialer names after the client. The record
	// carries the unit label the client's DHCPv6 lease carries, so both
	// IPv6 leases of one session sit together in the received class.
	for i := range cfg.PPPoEClient {
		e := &cfg.PPPoEClient[i]
		if e.Disable {
			continue
		}
		if !e.IPv6.Enabled {
			continue
		}
		if !e.IPv6.acceptsRA() {
			continue
		}
		targets = append(targets, raTarget{iface: e.Name, unit: pppoeClientDHCPUnit, device: e.Name})
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.targets = targets
	for key := range p.lists {
		if slices.ContainsFunc(targets, func(t raTarget) bool { return t.iface == key.iface && t.unit == key.unit }) {
			continue
		}
		delete(p.lists, key)
		p.publishLocked(key, &raServerList{})
	}
	p.armLocked(time.Now())
}

// handle processes one nd-user-option event. It runs on the emitter's
// goroutine; the work is one registry update.
//
// RFC 8106 Section 1.2: "Hosts conforming to this specification MUST extract
// DNS information from RA messages, unless static DNS configuration has been
// specified by the user." Every valid RDNSS option of an accepted RA becomes
// part of an ra record; the router's dns-source selection puts a configured
// name-server first.
func (p *raRDNSS) handle(data string) {
	var payload NDUserOptionPayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		p.log.Warn("interface: nd-user-option event unreadable", "err", err)
		return
	}
	// Other user options (Prefix Information, DNSSL, PREF64) have no
	// consumer yet.
	if len(payload.Option) == 0 {
		return
	}
	if payload.Option[0] != ndp.OptRDNSS {
		return
	}
	target, ok := p.targetFor(payload.Name)
	if !ok {
		return
	}
	// RFC 8106 Section 5.3.1: "Otherwise, the host MUST discard the
	// options."
	option, err := ndp.ParseRDNSS(payload.Option)
	if err != nil {
		length := 0
		if len(payload.Option) > 1 {
			length = int(payload.Option[1])
		}
		p.log.Warn("interface: RDNSS option discarded", "iface", payload.Name, "router", payload.Router, "length", length, "err", err)
		return
	}
	// RFC 8106 Section 5.1: "When a resolver sends a DNS query message to an
	// RDNSS identified by a link-local address, it MUST use the corresponding
	// link." The zone is the device the RA arrived on, so resolv.conf and Ze's
	// own resolver send on that link.
	for i, server := range option.Servers {
		if server.IsLinkLocalUnicast() {
			option.Servers[i] = server.WithZone(payload.Name)
		}
	}
	now := time.Now()
	key := rdnssUnitKey{iface: target.iface, unit: target.unit}
	p.mu.Lock()
	defer p.mu.Unlock()
	list := p.lists[key]
	if list == nil {
		list = &raServerList{}
		p.lists[key] = list
	}
	list.apply(option, now)
	if len(list.servers) == 0 {
		delete(p.lists, key)
	}
	p.publishLocked(key, list)
	p.armLocked(now)
}

// withdrawPPPoESession removes the record of the pppoe-client called name,
// whose session just ended: the session device is gone, so the servers its
// RAs announced are reached over a link that no longer exists. It is the
// PPPoEClient's sessionEnded hook, run beside the IPCP lease's withdrawal.
// The client stays a target, so an RA on the next session's device builds a
// fresh list. A client holding no record still publishes the empty list,
// which the registry answers as a withdrawal of nothing, as leaseDNS(nil)
// does for IPCP.
func (p *raRDNSS) withdrawPPPoESession(name string) {
	key := rdnssUnitKey{iface: name, unit: pppoeClientDHCPUnit}
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.lists, key)
	p.publishLocked(key, &raServerList{})
	p.armLocked(time.Now())
}

// targetFor answers the unit whose kernel device is device.
func (p *raRDNSS) targetFor(device string) (raTarget, bool) {
	p.mu.Lock()
	targets := slices.Clone(p.targets)
	p.mu.Unlock()
	for _, t := range targets {
		if t.device != "" {
			if t.device == device {
				return t, true
			}
			continue
		}
		parent, err := p.device(t.iface)
		// A unit whose interface resolves to no device right now cannot be
		// the device the RA arrived on.
		if err != nil {
			continue
		}
		if unitOSName(parent, &unitEntry{VLANID: t.vlanID}) == device {
			return t, true
		}
	}
	return raTarget{}, false
}

// publishLocked hands list to the registry as key's lease; an empty list
// withdraws it. The caller MUST hold p.mu.
func (p *raRDNSS) publishLocked(key rdnssUnitKey, list *raServerList) {
	source := raResolverOrigin.Source(key.iface, key.unit)
	if err := p.resolvers.SetLease(source, list.addresses(), list.validity(time.Now())); err != nil {
		p.log.Warn("interface: RA resolver lease write failed", "iface", key.iface, "unit", key.unit, "err", err)
	}
}

// armLocked arms the one expiry timer to the earliest Expiration-time of any
// list, or stops it when none has one. The caller MUST hold p.mu.
func (p *raRDNSS) armLocked(now time.Time) {
	var earliest time.Time
	found := false
	for _, list := range p.lists {
		at, ok := list.nextExpiry()
		if !ok {
			continue
		}
		if !found || at.Before(earliest) {
			earliest = at
			found = true
		}
	}
	if !found {
		if p.timer != nil {
			p.timer.Stop()
		}
		return
	}
	wait := max(earliest.Sub(now), 0)
	if p.timer == nil {
		p.timer = time.AfterFunc(wait, p.expire)
		return
	}
	p.timer.Reset(wait)
}

// expire is the timer's callback: it removes every expired entry and
// republishes each list that changed. RFC 8106 Section 6.1: "When the current
// time becomes larger than Expiration-time, this entry is regarded as
// expired, so it should not be used any more."
func (p *raRDNSS) expire() {
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, list := range p.lists {
		if !list.prune(now) {
			continue
		}
		if len(list.servers) == 0 {
			delete(p.lists, key)
		}
		p.publishLocked(key, list)
	}
	p.armLocked(now)
}

// stop stops the expiry timer. It MUST be called when the component stops;
// the records stay in the registry, which the hub owns.
func (p *raRDNSS) stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.timer != nil {
		p.timer.Stop()
	}
}

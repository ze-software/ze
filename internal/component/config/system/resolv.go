// Design: docs/features/dns-resolver.md -- the router's resolvers: resolv.conf and Ze's own lookups
// Related: resolv_linux.go -- the file write on Linux
// Related: resolv_host.go -- the host's own resolv.conf as a received record
// Related: register_dns.go -- the host origin and the system-dns event handles
// Related: system.go -- ExtractSystemConfig, the static name-servers and the path

package system

import (
	"errors"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

// ResolverOrigin is a lease protocol registered with RegisterResolverOrigin.
// Its rank is the order that protocol's sources appear in resolv.conf. The
// zero value is no registered origin: a source built from it names nothing
// and Resolvers.SetLease refuses it. A fallback origin's records stand in
// only while no other origin's record lists a server (dnssource.Record
// Fallback).
type ResolverOrigin struct {
	rank     uint16
	name     string
	fallback bool
}

// resolverOriginRegistry holds every registered origin. Each lease producer
// registers its own from its package, so adding a lease protocol edits only
// that protocol's package: the order is the ranks, each one declared beside
// its producer. Safe for concurrent use.
type resolverOriginRegistry struct {
	mu    sync.Mutex
	names map[string]uint16
	ranks map[uint16]string
}

var resolverOrigins resolverOriginRegistry

// RegisterResolverOrigin registers the lease protocol name with rank and
// answers the origin its sources are built from. A lower rank lists first in
// resolv.conf. It MUST be called from a package-level declaration or init():
// rank zero, an empty name, or a name or rank already registered is a Ze
// defect, and it panics.
func RegisterResolverOrigin(name string, rank uint16) ResolverOrigin {
	return resolverOrigins.register(name, rank, false)
}

// RegisterFallbackResolverOrigin registers name like RegisterResolverOrigin,
// as a fallback: its records are used only while no record of another origin
// lists a server, so they stand in for a lease rather than join one. The
// same calling rules and panics apply.
func RegisterFallbackResolverOrigin(name string, rank uint16) ResolverOrigin {
	return resolverOrigins.register(name, rank, true)
}

func (g *resolverOriginRegistry) register(name string, rank uint16, fallback bool) ResolverOrigin {
	if name == "" {
		panic("BUG: resolver origin registered with no name")
	}
	if rank == 0 {
		panic("BUG: resolver origin " + name + " registered with rank 0, which names no origin")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, taken := g.names[name]; taken {
		panic("BUG: resolver origin " + name + " registered twice")
	}
	if holder, taken := g.ranks[rank]; taken {
		panic("BUG: resolver origin " + name + " registered with the rank of " + holder)
	}
	if g.names == nil {
		g.names = make(map[string]uint16)
		g.ranks = make(map[uint16]string)
	}
	g.names[name] = rank
	g.ranks[rank] = name
	return ResolverOrigin{rank: rank, name: name, fallback: fallback}
}

// Source names the lease of this origin's client on iface and unit.
func (o ResolverOrigin) Source(iface, unit string) ResolverSource {
	return ResolverSource{origin: o, iface: iface, unit: unit}
}

// ResolverSource names one lease that supplied resolver servers: one client,
// keyed by its registered origin, interface and unit. Build it with
// ResolverOrigin.Source; the zero value names no source and
// Resolvers.SetLease refuses it.
type ResolverSource struct {
	origin ResolverOrigin
	iface  string
	unit   string
}

// lease is what one source supplied, when, and until when.
type lease struct {
	servers  []netip.Addr
	received time.Time
	end      leaseEnd
}

// LeaseValidity is how long a lease's servers stay usable, counted from the
// moment the registry records them. Build a bounded one with ValidFor; the
// zero value is UntilWithdrawn, a named state.
//
// It is a span, never an instant: a producer reads its own clock, which need
// not be the registry's, and an instant read on one clock and compared on
// the other ends the lease early or late.
type LeaseValidity struct {
	lifetime time.Duration
	bounded  bool
}

// UntilWithdrawn keeps a lease's servers until its producer sets or
// withdraws the source again: an IPCP lease, the host's resolv.conf, an
// infinite DHCP lease.
var UntilWithdrawn = LeaseValidity{}

// ValidFor keeps a lease's servers for lifetime from the moment the registry
// records them, measured on the registry's clock, after which the registry
// withdraws them as if the producer had sent an empty list. A lifetime of
// zero or less withdraws the source at once.
func ValidFor(lifetime time.Duration) LeaseValidity {
	return LeaseValidity{lifetime: lifetime, bounded: true}
}

// from anchors the validity on now, the registry's time of receipt.
func (v LeaseValidity) from(now time.Time) leaseEnd {
	if !v.bounded {
		return leaseEnd{}
	}
	return leaseEnd{at: now.Add(v.lifetime), bounded: true}
}

// leaseEnd is the registry-clock instant a recorded lease ends; the zero
// value is a lease that stays until withdrawn.
type leaseEnd struct {
	at      time.Time
	bounded bool
}

// expired reports whether the lease ended at or before now.
func (e leaseEnd) expired(now time.Time) bool {
	if !e.bounded {
		return false
	}
	return !now.Before(e.at)
}

// routerDNSSourcePath is the config path of the router consumer's list.
const routerDNSSourcePath = "system dns-source"

// routerDefault is the router consumer's built-in default list: the static
// name-servers when any are configured, else every learned record.
// RFC 3646 Section 6: "Where DNS parameters such as the domain searchlist or
// DNS servers have been manually configured, these parameters SHOULD NOT be
// overridden by DHCP." First-usable over configured then received is exactly
// that order.
var routerDefault = dnssource.StatedDefault(routerDNSSourcePath, dnssource.FamilyAll, dnssource.AudienceZe,
	dnssource.ModeFirstUsable, dnssource.ClassConfigured, dnssource.ClassReceived)

// Resolvers is the router's resolver registry and the only writer of
// resolv.conf. It records the resolver servers each source supplied, selects
// the router's servers from them with the shared rule (dnssource.Select over
// the router's list), rewrites the file from that selection, and hands every
// change to its watchers (Ze's internal resolver) and, once PublishOn wires a
// bus, to the system-dns event. Withdrawing one source removes that source's
// servers and no other's.
//
// Under the router's built-in default the selection is the static
// name-servers alone when any are configured, otherwise every learned
// record's servers ordered by origin rank (DHCPv4, DHCPv6, PPP, then the
// host's own resolv.conf), each group by interface then unit, each in the
// order its source sent them, and a server already listed is not repeated.
//
// A lease with a bounded validity is withdrawn when it ends, by one timer
// armed to the earliest end (no goroutine per lease): an expired lease never
// reaches the file, the internal resolver or the event.
//
// The zero value is ready to use: it writes nothing until Configure names a
// path, it reads no host resolv.conf, and it selects with the router's
// built-in default list until Configure hands it another. Safe for
// concurrent use.
type Resolvers struct {
	mu       sync.Mutex
	path     string                   // empty: writing disabled
	hostPath string                   // the host's resolv.conf; empty: none is read
	static   []netip.Addr             // system { name-server }
	list     dnssource.List           // system dns-source; zero: the built-in default
	leases   map[ResolverSource]lease // non-empty server lists only
	// written is a guard: an empty server set rewrites the file only once ze
	// has written servers to it, so a resolv.conf ze never managed is never
	// replaced by an empty one.
	written  bool
	last     []string           // the servers the file holds, valid when written
	snapshot dnssource.Snapshot // the current published state
	// consumers holds each consumer's last Publication, keyed by its name;
	// the registry names no consumer, it keeps what each one publishes.
	consumers map[string][]dnssource.ConsumerSelection
	// metrics is nil until SetMetrics hands the registry its collectors.
	metrics *resolverMetrics
	clock   clock.Clock // nil: the real clock
	expiry  clock.Timer // armed to the earliest lease end; nil until one is bounded

	// notifyMu serializes delivery to watchers, so each watcher sees the
	// snapshots in sequence order. It is taken before mu, never inside it.
	notifyMu    sync.Mutex
	delivered   uint64
	watchers    map[uint64]func(*dnssource.Snapshot)
	watcherNext uint64
}

// errUnnamedResolverSource refuses the zero ResolverSource: servers recorded
// under it could never be withdrawn by the lease that supplied them.
var errUnnamedResolverSource = errors.New("resolver servers from an unnamed source: build the source with Source on the origin RegisterResolverOrigin returned")

// hostResolvConfPath is where a standard Linux host keeps its resolvers.
const hostResolvConfPath = "/etc/resolv.conf"

// processResolvers reads the host's resolv.conf as a received record, so a
// Ze on a distribution host with no name-server and no lease keeps resolving.
var processResolvers = Resolvers{hostPath: hostResolvConfPath}

// ProcessResolvers answers the daemon's resolver registry: the hub configures
// it and every lease producer (the DHCP and PPPoE clients) sets its lease on
// it.
func ProcessResolvers() *Resolvers { return &processResolvers }

// errNoRouterList refuses a Configure with no router list: the config's
// system dns-source was not read (ExtractSystemConfig logged why), so the
// registry keeps the list it selects with now rather than guess one.
var errNoRouterList = errors.New("the router's dns-source list could not be read from the config, so Ze keeps its current resolvers\n" +
	"run ze config validate to see why system dns-source is refused")

// Configure sets the resolv.conf path (empty disables writing), the static
// name-servers and the router's dns-source list, re-reads the host's
// resolv.conf, then rewrites the file. The hub calls it at startup and on
// every reload. A name-server that is not an IP address is dropped with a
// warning: YANG types the leaf as an address, so only a Ze defect reaches
// that branch. A zero list is refused and changes nothing.
func (r *Resolvers) Configure(path string, static []string, list dnssource.List) error {
	if list.IsZero() {
		return errNoRouterList
	}
	r.mu.Lock()
	r.list = list
	if path != r.path {
		r.written = false
	}
	r.path = path
	r.static = parseServers(static, "system name-server", "")
	r.setLeaseLocked(hostResolverOrigin.Source("", ""), r.readHost(), UntilWithdrawn)
	err := r.refresh()
	r.mu.Unlock()
	r.deliver()
	return err
}

// SetLease records the servers source's lease now carries, valid as validity
// says, and rewrites the file. An empty list, or a validity already ended,
// withdraws the source: its lease ended, or its latest reply carried no
// resolver servers. A server that is not an IP address is dropped with a
// warning naming the source.
func (r *Resolvers) SetLease(source ResolverSource, servers []string, validity LeaseValidity) error {
	if source.origin.rank == 0 {
		return errUnnamedResolverSource
	}
	parsed := parseServers(servers, source.origin.name, source.iface)
	r.mu.Lock()
	r.setLeaseLocked(source, parsed, validity)
	err := r.refresh()
	r.mu.Unlock()
	r.deliver()
	return err
}

// setLeaseLocked records servers for source, or withdraws it when there are
// none or validity already ended. The caller MUST hold r.mu.
func (r *Resolvers) setLeaseLocked(source ResolverSource, servers []netip.Addr, validity LeaseValidity) {
	now := r.now()
	if len(servers) == 0 {
		delete(r.leases, source)
		return
	}
	end := validity.from(now)
	if end.expired(now) {
		delete(r.leases, source)
		return
	}
	if r.leases == nil {
		r.leases = make(map[ResolverSource]lease)
	}
	r.leases[source] = lease{servers: servers, received: now, end: end}
}

// now answers the registry's clock's time.
func (r *Resolvers) now() time.Time {
	if r.clock == nil {
		return time.Now()
	}
	return r.clock.Now()
}

// expire is the expiry timer's callback: it withdraws every lease whose
// validity ended, then publishes the change as SetLease does.
func (r *Resolvers) expire() {
	r.mu.Lock()
	err := r.refresh()
	r.mu.Unlock()
	if err != nil {
		slogutil.Logger("resolv").Warn("resolv.conf write failed after a lease expired", "path", r.path, "err", err)
	}
	r.deliver()
}

// armExpiryLocked arms the one expiry timer to the earliest end of a bounded
// lease, or stops it when no lease ends. The caller MUST hold r.mu.
func (r *Resolvers) armExpiryLocked(now time.Time) {
	var earliest time.Time
	for _, held := range r.leases {
		if !held.end.bounded {
			continue
		}
		if earliest.IsZero() || held.end.at.Before(earliest) {
			earliest = held.end.at
		}
	}
	if earliest.IsZero() {
		if r.expiry != nil {
			r.expiry.Stop()
		}
		return
	}
	wait := earliest.Sub(now)
	if r.expiry == nil {
		clk := r.clock
		if clk == nil {
			clk = clock.RealClock{}
		}
		r.expiry = clk.AfterFunc(wait, r.expire)
		return
	}
	r.expiry.Stop()
	r.expiry.Reset(wait)
}

// Servers answers the servers resolv.conf lists for the recorded sources.
func (r *Resolvers) Servers() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return serverTexts(r.snapshot.Router.Servers)
}

// Snapshot answers the current state: every record and the router's
// selection. The value is shared; the caller MUST NOT modify its slices.
func (r *Resolvers) Snapshot() dnssource.Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshot
}

// Watch registers fn, calls it at once with the current snapshot, then on
// every change, in sequence order, until the returned stop is called. fn
// runs on the goroutine that made the change and MUST NOT call back into r
// (Configure, SetLease, Watch). The caller MUST call stop when fn's receiver
// goes away.
func (r *Resolvers) Watch(fn func(*dnssource.Snapshot)) (stop func()) {
	r.notifyMu.Lock()
	defer r.notifyMu.Unlock()
	if r.watchers == nil {
		r.watchers = make(map[uint64]func(*dnssource.Snapshot))
	}
	id := r.watcherNext
	r.watcherNext++
	r.watchers[id] = fn
	current := r.Snapshot()
	fn(&current)
	return func() {
		r.notifyMu.Lock()
		defer r.notifyMu.Unlock()
		delete(r.watchers, id)
	}
}

// PublishOn emits every snapshot on bus as (system-dns, snapshot), starting
// with the current one, and answers each (system-dns, snapshot-request) by
// emitting the current snapshot again, so a consumer plugin that subscribes
// late, in this process or another, gets the state without waiting for a
// change. The caller MUST call the returned stop before bus goes away.
func (r *Resolvers) PublishOn(bus ze.EventBus) (stop func()) {
	stopWatch := r.Watch(func(snapshot *dnssource.Snapshot) { emitDNSSnapshot(bus, snapshot) })
	stopRequests := dnssource.SnapshotRequest.Subscribe(bus, func() {
		current := r.Snapshot()
		emitDNSSnapshot(bus, &current)
	})
	stopPublications := dnssource.PublicationEvent.Subscribe(bus, func(publication *dnssource.Publication) {
		if err := r.SetPublication(publication); err != nil {
			slogutil.Logger("resolv").Warn("system-dns selection not kept", "err", err)
		}
	})
	return func() {
		stopPublications()
		stopRequests()
		stopWatch()
	}
}

// errUnnamedConsumer refuses a publication with no consumer name: the
// registry could never replace it, so it would stand after the consumer left.
var errUnnamedConsumer = errors.New("a dns-source selection was published with no consumer name, so show system dns cannot attribute it\n" +
	"the publishing plugin MUST set Publication.Consumer to its plugin name")

// SetPublication replaces the selections kept for publication's consumer
// with its set. An empty set keeps the consumer listed with no selection,
// which is what a consumer with no list publishes. The publication is not
// modified, and its slices are kept as they are: a publisher MUST NOT modify
// a Publication after it emits it.
func (r *Resolvers) SetPublication(publication *dnssource.Publication) error {
	if publication.Consumer == "" {
		return errUnnamedConsumer
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.consumers == nil {
		r.consumers = make(map[string][]dnssource.ConsumerSelection)
	}
	previous, held := r.consumers[publication.Consumer]
	r.consumers[publication.Consumer] = publication.Selections
	if held && r.metrics != nil && !dnssource.SameSelections(previous, publication.Selections) {
		r.metrics.selectionChanged(publication.Consumer)
	}
	return nil
}

// Consumers answers every consumer's last publication, sorted by consumer
// name, so show system dns and ze doctor read one stable order. The router
// is not among them: its selection is Snapshot().Router. The slices are
// shared; the caller MUST NOT modify them.
func (r *Resolvers) Consumers() []dnssource.Publication {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]dnssource.Publication, 0, len(r.consumers))
	for consumer, selections := range r.consumers {
		out = append(out, dnssource.Publication{Consumer: consumer, Selections: selections})
	}
	slices.SortFunc(out, func(a, b dnssource.Publication) int { return strings.Compare(a.Consumer, b.Consumer) })
	return out
}

// emitDNSSnapshot publishes one snapshot. Delivery is best effort: a failure
// is logged at WARN, and the next change or request publishes again.
func emitDNSSnapshot(bus ze.EventBus, snapshot *dnssource.Snapshot) {
	if _, err := dnssource.SnapshotEvent.Emit(bus, snapshot); err != nil {
		slogutil.Logger("resolv").Warn("system-dns snapshot not published", "sequence", snapshot.Sequence, "err", err)
	}
}

// deliver hands the current snapshot to every watcher when it is newer than
// the last one delivered. It MUST be called without r.mu held.
func (r *Resolvers) deliver() {
	r.notifyMu.Lock()
	defer r.notifyMu.Unlock()
	current := r.Snapshot()
	if current.Sequence == r.delivered {
		return
	}
	r.delivered = current.Sequence
	for _, fn := range r.watchers {
		fn(&current)
	}
}

// refresh withdraws every lease whose validity ended, re-arms the expiry
// timer, recomputes the snapshot from the recorded sources with the router's
// list, and writes the file when its servers changed. The caller MUST hold
// r.mu.
func (r *Resolvers) refresh() error {
	now := r.now()
	for source, held := range r.leases {
		if held.end.expired(now) {
			delete(r.leases, source)
		}
	}
	r.armExpiryLocked(now)
	list := r.list
	// The zero Resolvers selects with the built-in default until Configure
	// hands it a list it read; Configure itself refuses a zero list.
	// refuse-by-default: guards Configure -- a zero list here is the zero Resolvers, never a refused config
	if list.IsZero() {
		list = routerDefault
	}
	records := r.records()
	selection := dnssource.Select(list, dnssource.Candidates{
		dnssource.ClassConfigured: r.static,
		dnssource.ClassReceived:   list.Received(records, now),
	})
	r.snapshot = dnssource.Snapshot{
		Sequence: r.snapshot.Sequence + 1,
		Records:  records,
		Router:   dnssource.NewConsumerSelection(list, "", selection),
	}
	return r.flush(serverTexts(selection.Servers))
}

// records answers every recorded lease as a fresh record, in received-class
// order. The caller MUST hold r.mu.
func (r *Resolvers) records() []dnssource.Record {
	records := make([]dnssource.Record, 0, len(r.leases))
	for source, held := range r.leases {
		records = append(records, dnssource.Record{
			Origin:    source.origin.name,
			Rank:      source.origin.rank,
			Interface: source.iface,
			Unit:      source.unit,
			Addresses: slices.Clone(held.servers),
			Received:  held.received,
			Expires:   held.end.at,
			Fallback:  source.origin.fallback,
		})
	}
	slices.SortFunc(records, func(a, b dnssource.Record) int { return dnssource.CompareRecords(&a, &b) })
	return records
}

// flush writes servers to the file when they changed. The caller MUST hold
// r.mu.
func (r *Resolvers) flush(servers []string) error {
	if r.path == "" {
		return nil
	}
	if len(servers) == 0 && !r.written {
		return nil
	}
	if r.written && slices.Equal(servers, r.last) {
		return nil
	}
	if err := writeResolvFile(r.path, servers); err != nil {
		return err
	}
	r.written = true
	r.last = servers
	return nil
}

// parseServers converts the server texts a source supplied into addresses,
// in order. A text that is not an IP address (a line break included, which
// would inject a directive into resolv.conf) cannot be a resolver: it is
// dropped with a warning naming the source.
func parseServers(texts []string, origin, iface string) []netip.Addr {
	out := make([]netip.Addr, 0, len(texts))
	for _, text := range texts {
		addr, err := netip.ParseAddr(strings.TrimSpace(text))
		if err != nil {
			slogutil.Logger("resolv").Warn("resolver server dropped: not an IP address",
				"origin", origin, "interface", iface, "server", text, "err", err)
			continue
		}
		out = append(out, addr)
	}
	return out
}

// serverTexts renders addresses in the canonical text resolv.conf and the
// internal resolver take.
func serverTexts(servers []netip.Addr) []string {
	out := make([]string, len(servers))
	for i, addr := range servers {
		out[i] = addr.String()
	}
	return out
}

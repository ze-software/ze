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

	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

// ResolverOrigin is a lease protocol registered with RegisterResolverOrigin.
// Its rank is the order that protocol's sources appear in resolv.conf. The
// zero value is no registered origin: a source built from it names nothing
// and Resolvers.SetLease refuses it.
type ResolverOrigin struct {
	rank uint16
	name string
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
	return resolverOrigins.register(name, rank)
}

func (g *resolverOriginRegistry) register(name string, rank uint16) ResolverOrigin {
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
	return ResolverOrigin{rank: rank, name: name}
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

// lease is what one source supplied, and when.
type lease struct {
	servers  []netip.Addr
	received time.Time
}

// routerDNSSourcePath is the config path of the router consumer's list.
const routerDNSSourcePath = "system dns-source"

// routerDefault is the router consumer's built-in default list: the static
// name-servers when any are configured, else every learned record.
// RFC 3646 Section 6: "Where DNS parameters such as the domain searchlist or
// DNS servers have been manually configured, these parameters SHOULD NOT be
// overridden by DHCP." First-usable over configured then received is exactly
// that order.
var routerDefault = dnssource.StatedDefault(routerDNSSourcePath, dnssource.FamilyAll,
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
// The zero value is ready to use: it writes nothing until Configure names a
// path, and it reads no host resolv.conf. Safe for concurrent use.
type Resolvers struct {
	mu       sync.Mutex
	path     string                   // empty: writing disabled
	hostPath string                   // the host's resolv.conf; empty: none is read
	static   []netip.Addr             // system { name-server }
	leases   map[ResolverSource]lease // non-empty server lists only
	// written is a guard: an empty server set rewrites the file only once ze
	// has written servers to it, so a resolv.conf ze never managed is never
	// replaced by an empty one.
	written  bool
	last     []string           // the servers the file holds, valid when written
	snapshot dnssource.Snapshot // the current published state

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

// Configure sets the resolv.conf path (empty disables writing) and the
// static name-servers, re-reads the host's resolv.conf, then rewrites the
// file. The hub calls it at startup and on every reload. A name-server that
// is not an IP address is dropped with a warning: YANG types the leaf as an
// address, so only a Ze defect reaches that branch.
func (r *Resolvers) Configure(path string, static []string) error {
	r.mu.Lock()
	if path != r.path {
		r.written = false
	}
	r.path = path
	r.static = parseServers(static, "system name-server", "")
	r.setLeaseLocked(hostResolverOrigin.Source("", ""), r.readHost())
	err := r.refresh()
	r.mu.Unlock()
	r.deliver()
	return err
}

// SetLease records the servers source's lease now carries and rewrites the
// file. An empty list withdraws the source: its lease ended, or its latest
// reply carried no resolver servers. A server that is not an IP address is
// dropped with a warning naming the source.
func (r *Resolvers) SetLease(source ResolverSource, servers []string) error {
	if source.origin.rank == 0 {
		return errUnnamedResolverSource
	}
	parsed := parseServers(servers, source.origin.name, source.iface)
	r.mu.Lock()
	r.setLeaseLocked(source, parsed)
	err := r.refresh()
	r.mu.Unlock()
	r.deliver()
	return err
}

// setLeaseLocked records servers for source, or withdraws it when there are
// none. The caller MUST hold r.mu.
func (r *Resolvers) setLeaseLocked(source ResolverSource, servers []netip.Addr) {
	if len(servers) == 0 {
		delete(r.leases, source)
		return
	}
	if r.leases == nil {
		r.leases = make(map[ResolverSource]lease)
	}
	r.leases[source] = lease{servers: servers, received: time.Now()}
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
	stopRequests := dnsSnapshotRequest.Subscribe(bus, func() {
		current := r.Snapshot()
		emitDNSSnapshot(bus, &current)
	})
	return func() {
		stopRequests()
		stopWatch()
	}
}

// emitDNSSnapshot publishes one snapshot. Delivery is best effort: a failure
// is logged at WARN, and the next change or request publishes again.
func emitDNSSnapshot(bus ze.EventBus, snapshot *dnssource.Snapshot) {
	if _, err := dnsSnapshotEvent.Emit(bus, snapshot); err != nil {
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

// refresh recomputes the snapshot from the recorded sources and writes the
// file when its servers changed. The caller MUST hold r.mu.
func (r *Resolvers) refresh() error {
	records := r.records()
	selection := dnssource.Select(routerDefault, dnssource.Candidates{
		dnssource.ClassConfigured: r.static,
		dnssource.ClassReceived:   dnssource.ReceivedAddresses(records, time.Now()),
	})
	r.snapshot = dnssource.Snapshot{
		Sequence: r.snapshot.Sequence + 1,
		Records:  records,
		Router:   dnssource.NewRouterSelection(routerDefault, selection),
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

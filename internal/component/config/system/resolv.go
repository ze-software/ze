// Design: docs/architecture/config/syntax.md -- resolv.conf generation
// Related: resolv_linux.go -- the file write on Linux
// Related: system.go -- ExtractSystemConfig, the static name-servers and the path

package system

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"sync"
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

// Resolvers is the only writer of resolv.conf. It records the resolver
// servers each source supplied and rewrites the file from them on every
// change, so withdrawing one source removes that source's servers and no
// other's.
//
// The file lists the static name-servers alone when any are configured:
// RFC 3646 Section 6: "Where DNS parameters such as the domain searchlist or
// DNS servers have been manually configured, these parameters SHOULD NOT be
// overridden by DHCP." Otherwise it lists every lease's servers, ordered by
// the rank of each source's registered origin (DHCPv4, then DHCPv6, then
// PPP), each group by interface then unit, each
// lease's servers in the order the server sent them, and a server a
// previous source already listed is not repeated.
//
// The zero value is ready to use and writes nothing until Configure names a
// path. Safe for concurrent use.
type Resolvers struct {
	mu     sync.Mutex
	path   string                      // empty: writing disabled
	static []string                    // system { name-server }
	leases map[ResolverSource][]string // non-empty server lists only
	// written is a guard: an empty server set rewrites the file only once ze
	// has written servers to it, so a resolv.conf ze never managed is never
	// replaced by an empty one.
	written bool
	last    []string // the servers the file holds, valid when written
}

// errUnnamedResolverSource refuses the zero ResolverSource: servers recorded
// under it could never be withdrawn by the lease that supplied them.
var errUnnamedResolverSource = errors.New("resolver servers from an unnamed source: build the source with Source on the origin RegisterResolverOrigin returned")

var processResolvers Resolvers

// ProcessResolvers answers the daemon's resolv.conf writer: the hub
// configures it and every lease producer (the DHCP and PPPoE clients) sets
// its lease on it.
func ProcessResolvers() *Resolvers { return &processResolvers }

// Configure sets the resolv.conf path (empty disables writing) and the
// static name-servers, then rewrites the file. The hub calls it at startup
// and on every reload.
func (r *Resolvers) Configure(path string, static []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if path != r.path {
		r.written = false
	}
	r.path = path
	r.static = slices.Clone(static)
	return r.flush()
}

// SetLease records the servers source's lease now carries and rewrites the
// file. An empty list withdraws the source: its lease ended, or its latest
// reply carried no resolver servers.
func (r *Resolvers) SetLease(source ResolverSource, servers []string) error {
	if source.origin.rank == 0 {
		return errUnnamedResolverSource
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(servers) == 0 {
		delete(r.leases, source)
		return r.flush()
	}
	if r.leases == nil {
		r.leases = make(map[ResolverSource][]string)
	}
	r.leases[source] = slices.Clone(servers)
	return r.flush()
}

// Servers answers the servers resolv.conf lists for the recorded sources.
func (r *Resolvers) Servers() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.servers()
}

// servers renders the recorded sources in file order. The caller MUST hold
// r.mu.
func (r *Resolvers) servers() []string {
	if len(r.static) > 0 {
		return appendServers(nil, r.static)
	}
	sources := make([]ResolverSource, 0, len(r.leases))
	for source := range r.leases {
		sources = append(sources, source)
	}
	slices.SortFunc(sources, compareResolverSources)
	var out []string
	for _, source := range sources {
		out = appendServers(out, r.leases[source])
	}
	return out
}

// flush writes the file when its servers changed. The caller MUST hold r.mu.
func (r *Resolvers) flush() error {
	if r.path == "" {
		return nil
	}
	servers := r.servers()
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

// appendServers appends each server out does not already hold. A server
// carrying a line break is skipped: it would inject a directive into the
// file.
func appendServers(out, servers []string) []string {
	for _, s := range servers {
		if strings.ContainsAny(s, "\n\r") {
			continue
		}
		if slices.Contains(out, s) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func compareResolverSources(a, b ResolverSource) int {
	return cmp.Or(
		cmp.Compare(a.origin.rank, b.origin.rank),
		strings.Compare(a.iface, b.iface),
		strings.Compare(a.unit, b.unit),
	)
}

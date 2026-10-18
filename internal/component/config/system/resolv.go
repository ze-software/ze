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

// resolverOrigin is the protocol that leased a set of resolver servers. Its
// numeric order is the order those sources appear in resolv.conf.
type resolverOrigin uint8

const (
	resolverOriginUnspecified resolverOrigin = iota
	resolverOriginDHCPv4
	resolverOriginDHCPv6
)

// ResolverSource names one lease that supplied resolver servers: one DHCP
// client, keyed by its protocol, interface and unit. Build it with
// DHCPv4ResolverSource or DHCPv6ResolverSource; the zero value names no
// source and Resolvers.SetLease refuses it.
type ResolverSource struct {
	origin resolverOrigin
	iface  string
	unit   string
}

// DHCPv4ResolverSource names the DHCPv4 client of iface and unit.
func DHCPv4ResolverSource(iface, unit string) ResolverSource {
	return ResolverSource{origin: resolverOriginDHCPv4, iface: iface, unit: unit}
}

// DHCPv6ResolverSource names the DHCPv6 client of iface and unit.
func DHCPv6ResolverSource(iface, unit string) ResolverSource {
	return ResolverSource{origin: resolverOriginDHCPv6, iface: iface, unit: unit}
}

// Resolvers is the only writer of resolv.conf. It records the resolver
// servers each source supplied and rewrites the file from them on every
// change, so withdrawing one source removes that source's servers and no
// other's.
//
// The file lists the static name-servers alone when any are configured:
// RFC 3646 Section 6: "Where DNS parameters such as the domain searchlist or
// DNS servers have been manually configured, these parameters SHOULD NOT be
// overridden by DHCP." Otherwise it lists every lease's servers, DHCPv4
// clients before DHCPv6 clients, each group by interface then unit, each
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
var errUnnamedResolverSource = errors.New("resolver servers from an unnamed source: build the source with DHCPv4ResolverSource or DHCPv6ResolverSource")

var processResolvers Resolvers

// ProcessResolvers answers the daemon's resolv.conf writer: the hub
// configures it and every DHCP client sets its lease on it.
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
	if source.origin == resolverOriginUnspecified {
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
		cmp.Compare(a.origin, b.origin),
		strings.Compare(a.iface, b.iface),
		strings.Compare(a.unit, b.unit),
	)
}

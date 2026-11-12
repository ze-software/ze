// Design: docs/architecture/fib/fib-vpp-route-ownership.md -- the FIB source that marks Ze's IP routes in VPP
// Related: backend.go -- every IP route add and delete goes through sendRoute
// Related: mpls.go -- labeled pushes are IP routes and go through sendRoute too
// Related: mplsstale.go -- the same stale-mark-then-sweep for MPLS table 0
// Related: doctor_route.go -- the fib-vpp-stale-routes check reports what the sweep could not do
//
// VPP keeps its FIB when Ze stops, so the IP routes and labeled pushes a
// previous fib-vpp writer installed keep forwarding after a Ze restart. A
// kernel route carries rtm_protocol, which is how fib-kernel's startupSweep
// tells its own routes apart; a VPP route carries the FIB source that added
// it. ip_route_add_del (v1) always adds under FIB_SOURCE_API, which every
// binary-API client shares (static routes on VPP included), so it proves
// nothing about who installed a route. fib-vpp therefore registers a source of
// its own, named fibSourceName, with fib_source_add, and installs and removes
// every IP route under it with ip_route_add_del_v2. A writer that opens dumps
// each IP table with ip_route_v2_dump filtered on that source (VPP walks the
// table with fib_table_walk_w_src, so only entries this source contributes to
// are listed), and once the replay window has passed removes every one this
// run has not installed or removed since. A delete names the source, so it
// takes away only fib-vpp's contribution: an interface, DHCP, CLI or other
// API client's route for the same prefix stays.
package fibvpp

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sync"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/fib"
	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"
)

// fibSourceName is the VPP FIB source fib-vpp installs its IP routes under.
// VPP keeps a source for its own lifetime and allocates a new id on every
// fib_source_add, even for a name it holds (fib_source_allocate), so a writer
// looks the name up before it adds one: every Ze run against one VPP process
// shares the source.
const fibSourceName = "ze-fib-vpp"

// fibSourcePriority is FIB_SOURCE_API's priority (0x80, foreach_fib_source in
// VPP's fib_source.h), so fib-vpp's routes keep the preference against
// interface, DHCP and adjacency routes they had under FIB_SOURCE_API.
const fibSourcePriority = 0x80

// errFibSource marks a writer that could not find or register its FIB
// source: it could then mark no route as its own, so it installs none.
var errFibSource = errors.New("fib-vpp: VPP FIB source unavailable")

// ipRouteKey names one IP route: a prefix in a VPP IP table.
type ipRouteKey struct {
	table  uint32
	prefix netip.Prefix
}

// routeSource is the FIB source every fib-vpp IP route is installed under,
// and the record of the routes a previous writer left under it. Safe for
// concurrent use: mu guards the stale set and the failures.
type routeSource struct {
	ch api.Channel
	id uint8

	mu sync.Mutex
	// stale holds the routes the source carried when this writer opened, less
	// every one this run has since added or removed: what a previous writer
	// left behind. sweep removes them.
	stale map[ipRouteKey]struct{}
	// failures keeps each leftover VPP would not remove, and dumpErr why the
	// IP tables could not be read, for the doctor check.
	failures map[ipRouteKey]error
	dumpErr  error
}

// openRouteSource finds fibSourceName among VPP's FIB sources, or registers
// it, and returns the source bound to ch. An error wraps errFibSource.
func openRouteSource(ch api.Channel) (*routeSource, error) {
	id, found, err := findFibSource(ch)
	if err != nil {
		return nil, err
	}
	if !found {
		id, err = addFibSource(ch)
		if err != nil {
			return nil, err
		}
	}
	return &routeSource{ch: ch, id: id}, nil
}

// findFibSource looks fibSourceName up in fib_source_dump.
func findFibSource(ch api.Channel) (id uint8, found bool, err error) {
	dump := ch.SendMultiRequest(&fib.FibSourceDump{})
	for {
		details := &fib.FibSourceDetails{}
		last, err := dump.ReceiveReply(details)
		if err != nil {
			return 0, false, fmt.Errorf("%w: fib_source_dump: %w", errFibSource, err)
		}
		if last {
			return id, found, nil
		}
		if details.Src.Name == fibSourceName {
			id, found = details.Src.ID, true
		}
	}
}

// addFibSource registers fibSourceName with fib_source_add.
func addFibSource(ch api.Channel) (uint8, error) {
	req := &fib.FibSourceAdd{Src: fib.FibSource{Priority: fibSourcePriority, Name: fibSourceName}}
	reply := &fib.FibSourceAddReply{}
	if err := ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return 0, fmt.Errorf("%w: fib_source_add: %w", errFibSource, err)
	}
	if reply.Retval != 0 {
		return 0, fmt.Errorf("%w: fib_source_add retval=%d", errFibSource, reply.Retval)
	}
	return reply.ID, nil
}

// sendRoute adds or removes the route key names under this source, with
// paths on an add. A success means this run now decides the route, so it is
// no longer a leftover; a failure leaves the stale set as it was. op names
// the caller in the error.
func (s *routeSource) sendRoute(op string, isAdd bool, key ipRouteKey, paths []fib_types.FibPath) error {
	if len(paths) > 255 {
		return fmt.Errorf("%s: %d paths exceeds uint8 limit", op, len(paths))
	}
	req := &ip.IPRouteAddDelV2{
		IsAdd: isAdd,
		Route: ip.IPRouteV2{
			TableID: key.table,
			Prefix:  toVPPPrefix(key.prefix),
			NPaths:  uint8(len(paths)),
			Src:     s.id,
			Paths:   paths,
		},
	}
	reply := &ip.IPRouteAddDelV2Reply{}
	if err := s.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("%s: %w: %w", op, errVPPMutationUncertain, err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("%s retval=%d", op, reply.Retval)
	}
	s.mu.Lock()
	delete(s.stale, key)
	delete(s.failures, key)
	s.mu.Unlock()
	return nil
}

// markStale records every IP route this source holds in VPP. The writer
// calls it when it opens, before it asks sysrib and the label sources for a
// replay: at that point no route under the source is this run's. When a
// table cannot be read nothing is marked and the cause is kept for the
// doctor check: a partial mark would let the sweep act on part of what a
// previous run left and report the rest as gone.
func (s *routeSource) markStale() {
	keys, err := s.dumpRoutes()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.dumpErr = err
		s.stale = nil
		logger().Error("fib-vpp: VPP's IP tables could not be read, so IP routes a previous run left in VPP "+
			"are not removed; run 'ze show doctor' for the fix", "error", err, "code", codeStaleRoute)
		return
	}
	s.dumpErr = nil
	s.stale = make(map[ipRouteKey]struct{}, len(keys))
	for _, k := range keys {
		s.stale[k] = struct{}{}
	}
	logger().Info("fib-vpp: IP routes left in VPP by a previous run", "count", len(s.stale))
}

// dumpRoutes lists every route this source contributes to, in every IPv4
// and IPv6 table VPP holds.
func (s *routeSource) dumpRoutes() ([]ipRouteKey, error) {
	tables, err := s.dumpTables()
	if err != nil {
		return nil, err
	}
	var keys []ipRouteKey
	for _, table := range tables {
		dump := s.ch.SendMultiRequest(&ip.IPRouteV2Dump{Src: s.id, Table: table})
		for {
			details := &ip.IPRouteV2Details{}
			last, err := dump.ReceiveReply(details)
			if err != nil {
				return nil, fmt.Errorf("ip_route_v2_dump table %d: %w", table.TableID, err)
			}
			if last {
				break
			}
			prefix, err := fromVPPPrefix(details.Route.Prefix)
			if err != nil {
				return nil, fmt.Errorf("ip_route_v2_dump table %d: %w", table.TableID, err)
			}
			keys = append(keys, ipRouteKey{table: details.Route.TableID, prefix: prefix})
		}
	}
	return keys, nil
}

// dumpTables lists VPP's IP tables, both families.
func (s *routeSource) dumpTables() ([]ip.IPTable, error) {
	dump := s.ch.SendMultiRequest(&ip.IPTableDump{})
	var tables []ip.IPTable
	for {
		details := &ip.IPTableDetails{}
		last, err := dump.ReceiveReply(details)
		if err != nil {
			return nil, fmt.Errorf("ip_table_dump: %w", err)
		}
		if last {
			return tables, nil
		}
		tables = append(tables, details.Table)
	}
}

// sweep removes every leftover that no add or removal of this run claimed
// since markStale. A route VPP refuses to remove is kept for the doctor
// check. The caller holds the writer's lock and has checked that the writer
// is not retired, so the channel is still open.
func (s *routeSource) sweep() {
	s.mu.Lock()
	keys := make([]ipRouteKey, 0, len(s.stale))
	for k := range s.stale {
		keys = append(keys, k)
	}
	s.mu.Unlock()
	removed := 0
	for _, k := range keys {
		if err := s.sendRoute("ip_route_add_del_v2 stale del", false, k, nil); err != nil {
			s.recordFailure(k, err)
			continue
		}
		removed++
	}
	s.mu.Lock()
	s.stale = nil
	s.mu.Unlock()
	logger().Info("fib-vpp: IP routes left by a previous run removed", "count", removed)
}

// sweepStaleRoutes removes the IP routes and labeled pushes a previous
// writer left under the FIB source that this run has not re-installed. A
// retired writer's channel is closed, so it sweeps nothing. Safe for
// concurrent use: it takes f.mu, which orders the sweep against processEvent.
func (f *fibVPP) sweepStaleRoutes() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.retired {
		return
	}
	if f.routes == nil {
		return
	}
	f.routes.sweep()
}

// recordFailure keeps a leftover VPP would not remove for the doctor check.
func (s *routeSource) recordFailure(k ipRouteKey, err error) {
	logger().Error("fib-vpp: an IP route a previous run left in VPP could not be removed and still "+
		"forwards; run 'ze show doctor' for the fix", "prefix", k.prefix, "table", k.table,
		"error", err, "code", codeStaleRoute)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failures == nil {
		s.failures = make(map[ipRouteKey]error)
	}
	s.failures[k] = err
}

// staleFailures lists, sorted by table then prefix, the leftovers VPP would
// not remove with the error for each, and why the tables could not be read.
func (s *routeSource) staleFailures() (keys []ipRouteKey, errs map[ipRouteKey]error, dumpErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys = make([]ipRouteKey, 0, len(s.failures))
	errs = make(map[ipRouteKey]error, len(s.failures))
	for k, err := range s.failures {
		keys = append(keys, k)
		errs[k] = err
	}
	slices.SortFunc(keys, func(a, b ipRouteKey) int {
		if a.table != b.table {
			if a.table < b.table {
				return -1
			}
			return 1
		}
		return a.prefix.Compare(b.prefix)
	})
	return keys, errs, s.dumpErr
}

// fromVPPPrefix converts a prefix VPP reported into its masked netip form.
// VPP is an open API boundary, so an unknown family is refused, never read as
// IPv4.
func fromVPPPrefix(p ip_types.Prefix) (netip.Prefix, error) {
	var address netip.Addr
	switch p.Address.Af {
	case ip_types.ADDRESS_IP4:
		address = netip.AddrFrom4(p.Address.Un.GetIP4())
	case ip_types.ADDRESS_IP6:
		address = netip.AddrFrom16(p.Address.Un.GetIP6())
	default:
		// Address family is supplied by the VPP API, not a closed Ze enum.
		return netip.Prefix{}, fmt.Errorf("unknown VPP address family %d", p.Address.Af)
	}
	prefix := netip.PrefixFrom(address, int(p.Len))
	if !prefix.IsValid() {
		return netip.Prefix{}, fmt.Errorf("invalid VPP prefix %s/%d", address, p.Len)
	}
	return prefix.Masked(), nil
}

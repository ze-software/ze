// Design: docs/architecture/fib/fib-vpp-route-ownership.md -- the stale IP route sweep, unit tests
//
// The stale IP route sweep (routesource.go) driven through the testChannel
// fake: VPP's tables are scripted as a set of routes, each carrying the FIB
// source that added it, and the fake lists only the routes of the source a
// dump names, as VPP's fib_table_walk_w_src does. No real VPP daemon is used.
package fibvpp

import (
	"errors"
	"fmt"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/diagnostic"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/fib"
	"go.fd.io/govpp/binapi/ip"
)

// fibSourceAPI is FIB_SOURCE_API's id, the source ip_route_add_del (v1)
// adds under: another binary-API client's routes carry it.
const fibSourceAPI = 3

// testIPDumpCtx answers an IPTableDump, an IPRouteV2Dump or a FibSourceDump
// from the channel, one entry per reply, then the end of the dump.
type testIPDumpCtx struct {
	ch    *testChannel
	msg   api.Message
	index int
	rows  []ip.IPRouteV2 // the routes an IPRouteV2Dump lists, filtered once
	ready bool
}

func (r *testIPDumpCtx) ReceiveReply(reply api.Message) (bool, error) {
	if r.ch.sendErr != nil {
		return false, r.ch.sendErr
	}
	switch req := r.msg.(type) {
	case *ip.IPTableDump:
		details, ok := reply.(*ip.IPTableDetails)
		if !ok {
			return false, fmt.Errorf("IPTableDump reply decoded into %T", reply)
		}
		if r.index == len(r.ch.ipTables) {
			return true, nil
		}
		details.Table = r.ch.ipTables[r.index]
		r.index++
		return false, nil
	case *fib.FibSourceDump:
		details, ok := reply.(*fib.FibSourceDetails)
		if !ok {
			return false, fmt.Errorf("FibSourceDump reply decoded into %T", reply)
		}
		if r.index == len(r.ch.fibSources) {
			return true, nil
		}
		details.Src = r.ch.fibSources[r.index]
		r.index++
		return false, nil
	case *ip.IPRouteV2Dump:
		if r.ch.ipDumpErr != nil {
			return false, r.ch.ipDumpErr
		}
		details, ok := reply.(*ip.IPRouteV2Details)
		if !ok {
			return false, fmt.Errorf("IPRouteV2Dump reply decoded into %T", reply)
		}
		if !r.ready {
			r.rows = routesOfSource(r.ch.ipDump, req)
			r.ready = true
		}
		if r.index == len(r.rows) {
			return true, nil
		}
		details.Route = r.rows[r.index]
		r.index++
		return false, nil
	}
	return false, fmt.Errorf("testChannel serves no dump of %T", r.msg)
}

// routesOfSource is what VPP lists for req: the routes of req's table and
// family, only those of req.Src when it is set.
func routesOfSource(all []ip.IPRouteV2, req *ip.IPRouteV2Dump) []ip.IPRouteV2 {
	var rows []ip.IPRouteV2
	for _, route := range all {
		if route.TableID != req.Table.TableID {
			continue
		}
		prefix, err := fromVPPPrefix(route.Prefix)
		if err != nil {
			continue
		}
		if prefix.Addr().Is6() != req.Table.IsIP6 {
			continue
		}
		if req.Src != 0 && route.Src != req.Src {
			continue
		}
		rows = append(rows, route)
	}
	return rows
}

// vppRoute is one scripted VPP route of source src.
func vppRoute(table uint32, prefix string, src uint8) ip.IPRouteV2 {
	return ip.IPRouteV2{TableID: table, Prefix: toVPPPrefix(netip.MustParsePrefix(prefix)), Src: src}
}

// bothFamilies is VPP's default IP tables plus table 5, both families.
func bothFamilies() []ip.IPTable {
	return []ip.IPTable{
		{TableID: 0}, {TableID: 0, IsIP6: true}, {TableID: 5}, {TableID: 5, IsIP6: true},
	}
}

// routeDeletes lists the IPRouteAddDelV2 deletions sent from index from on,
// each with the FIB source it named.
func routeDeletes(ch *testChannel, from int) map[ipRouteKey]uint8 {
	out := make(map[ipRouteKey]uint8)
	for _, m := range ch.requests[from:] {
		req, ok := m.(*ip.IPRouteAddDelV2)
		if !ok || req.IsAdd {
			continue
		}
		prefix, err := fromVPPPrefix(req.Route.Prefix)
		if err != nil {
			continue
		}
		out[ipRouteKey{table: req.Route.TableID, prefix: prefix}] = req.Route.Src
	}
	return out
}

// newStaleRouteWriter is a writer over ch whose IP and MPLS backends share
// fib-vpp's FIB source, marked against what ch scripts, as openFibVPPWriter
// builds it.
func newStaleRouteWriter(ch *testChannel) *fibVPP {
	source := testRouteSource(ch)
	source.markStale()
	f := newFibVPPWithMPLS(newGovppBackend(ch, 0, source), newGovppMPLSBackend(ch, 0, source))
	f.routes = source
	return f
}

func staleKey(table uint32, prefix string) ipRouteKey {
	return ipRouteKey{table: table, prefix: netip.MustParsePrefix(prefix)}
}

// TestFibVPPSweepRemovesStaleIPRoutes proves the routes a previous run left
// under fib-vpp's FIB source, in every table and both families, are removed
// by the sweep, each under that source only, while a route this run
// re-announced (a plain route and a labeled push) and every route of another
// source stay.
//
// VALIDATES: journal guard-added-to-one-half-of-a-pair.md 2026-11-11, AC-15
// backend parity -- fib-vpp sweeps stale IP routes and labeled pushes as
// fib-kernel's startupSweep and sweepStale do.
// METHOD: script VPP with five routes of fib-vpp's source and two of
// FIB_SOURCE_API (one on a prefix fib-vpp also left), mark, re-announce two,
// sweep, read the deletions.
func TestFibVPPSweepRemovesStaleIPRoutes(t *testing.T) {
	ch := &testChannel{ipTables: bothFamilies(), ipDump: []ip.IPRouteV2{
		vppRoute(0, "10.1.0.0/24", testRouteSourceID),
		vppRoute(0, "10.2.0.0/24", testRouteSourceID),
		vppRoute(0, "10.3.0.0/24", testRouteSourceID),
		vppRoute(0, "2001:db8:1::/48", testRouteSourceID),
		vppRoute(5, "10.5.0.0/24", testRouteSourceID),
		vppRoute(0, "10.9.0.0/24", fibSourceAPI),
		vppRoute(0, "10.1.0.0/24", fibSourceAPI),
	}}
	f := newStaleRouteWriter(ch)

	for _, m := range ch.requests {
		if d, ok := m.(*ip.IPRouteV2Dump); ok {
			assert.Equal(t, uint8(testRouteSourceID), d.Src, "the writer lists only routes of its own source")
		}
	}

	f.processEvent(makeBatch(
		incomingChange{Action: routeaction.Update, Prefix: netip.MustParsePrefix("10.2.0.0/24"),
			NextHop: netip.MustParseAddr("192.0.2.1")},
		incomingChange{Action: routeaction.Update, Prefix: netip.MustParsePrefix("10.3.0.0/24"),
			NextHop: netip.MustParseAddr("192.0.2.1"), Labels: []uint32{100}},
	))

	before := len(ch.requests)
	f.sweepStaleRoutes()
	deletes := routeDeletes(ch, before)
	assert.Equal(t, map[ipRouteKey]uint8{
		staleKey(0, "10.1.0.0/24"):     testRouteSourceID,
		staleKey(0, "2001:db8:1::/48"): testRouteSourceID,
		staleKey(5, "10.5.0.0/24"):     testRouteSourceID,
	}, deletes, "only the unclaimed leftovers are removed, each under fib-vpp's source alone")
	assert.NotContains(t, deletes, staleKey(0, "10.9.0.0/24"), "another source's route is never touched")
	assert.Empty(t, staleRouteDoctorFindings(f, nil), "a sweep VPP accepted leaves nothing to report")

	before = len(ch.requests)
	f.sweepStaleRoutes()
	assert.Empty(t, routeDeletes(ch, before), "a second sweep finds nothing left to remove")
}

// TestFibVPPSweepSparesRouteWithdrawnAfterMark proves a route this run
// installed and withdrew again between the mark and the sweep is this run's:
// the sweep does not remove it a second time, so VPP is not asked to delete
// what is gone and no false finding follows.
//
// VALIDATES: the sweep never acts on a route this run decided.
// METHOD: mark with 10.1.0.0/24 left, install then withdraw it, sweep.
func TestFibVPPSweepSparesRouteWithdrawnAfterMark(t *testing.T) {
	ch := &testChannel{ipTables: bothFamilies(), ipDump: []ip.IPRouteV2{vppRoute(0, "10.1.0.0/24", testRouteSourceID)}}
	f := newStaleRouteWriter(ch)
	prefix := netip.MustParsePrefix("10.1.0.0/24")
	for _, action := range []routeaction.Action{routeaction.Update, routeaction.Withdraw} {
		f.processEvent(makeBatch(incomingChange{Action: action, Prefix: prefix, NextHop: netip.MustParseAddr("192.0.2.1")}))
	}
	before := len(ch.requests)
	f.sweepStaleRoutes()
	assert.Empty(t, routeDeletes(ch, before))
}

// TestFibVPPStaleRouteDumpFailureReachesDoctor proves IP tables that cannot
// be read when the writer opens are reported by fib-vpp-stale-routes with
// the code and the fix command, and that the sweep then removes nothing: a
// partial mark is never acted on.
//
// VALIDATES: a sweep that cannot run says so through doctor.
// METHOD: fail the route dump, mark, sweep, read the findings.
func TestFibVPPStaleRouteDumpFailureReachesDoctor(t *testing.T) {
	ch := &testChannel{ipTables: bothFamilies(), ipDump: []ip.IPRouteV2{vppRoute(0, "10.1.0.0/24", testRouteSourceID)},
		ipDumpErr: errors.New("VPP API timeout")}
	f := newStaleRouteWriter(ch)

	before := len(ch.requests)
	f.sweepStaleRoutes()
	assert.Empty(t, routeDeletes(ch, before), "nothing is removed from a table that was not read")

	findings, err := answerDoctorCheck(staleRoutesCheckName, f, nil)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, codeStaleRoute, findings[0].Code)
	assert.Equal(t, string(diagnostic.SeverityError), findings[0].Severity)
	assert.Contains(t, findings[0].Message, "VPP API timeout")
	assert.Equal(t, staleRouteFix, findings[0].Fix)
}

// TestFibVPPStaleRouteRefusalReachesDoctor proves a leftover VPP refuses to
// remove is named, with VPP's answer, by fib-vpp-stale-routes, while the
// others are removed; a later install of that prefix clears the finding.
//
// VALIDATES: a route the sweep cannot remove is reported through doctor.
// METHOD: refuse the delete of one of two leftovers, sweep, read the
// findings, re-install the prefix, read them again.
func TestFibVPPStaleRouteRefusalReachesDoctor(t *testing.T) {
	ch := &testChannel{ipTables: bothFamilies(), ipDump: []ip.IPRouteV2{
		vppRoute(0, "10.1.0.0/24", testRouteSourceID),
		vppRoute(0, "10.2.0.0/24", testRouteSourceID),
	}, delRetval: map[netip.Prefix]int32{netip.MustParsePrefix("10.1.0.0/24"): -6}}
	f := newStaleRouteWriter(ch)

	before := len(ch.requests)
	f.sweepStaleRoutes()
	assert.Len(t, routeDeletes(ch, before), 2, "both leftovers are asked for")

	findings := staleRouteDoctorFindings(f, nil)
	require.Len(t, findings, 1)
	assert.Equal(t, codeStaleRoute, findings[0].Code)
	assert.Contains(t, findings[0].Message, "10.1.0.0/24")
	assert.Contains(t, findings[0].Message, "retval=-6")

	f.processEvent(makeBatch(incomingChange{Action: routeaction.Update,
		Prefix: netip.MustParsePrefix("10.1.0.0/24"), NextHop: netip.MustParseAddr("192.0.2.1")}))
	assert.Empty(t, staleRouteDoctorFindings(f, nil), "the route is this run's once it is installed")
}

// TestOpenRouteSourceFindsOrAddsSource proves the writer reuses the FIB
// source a previous run registered with VPP, registers it with FIB_SOURCE_API's
// priority when VPP holds none, and that a refusal stops the writer and is
// reported by fib-vpp-stale-routes while no writer exists.
//
// VALIDATES: ownership is the FIB source, shared across Ze runs on one VPP.
// METHOD: openRouteSource over three scripted channels.
func TestOpenRouteSourceFindsOrAddsSource(t *testing.T) {
	held := &testChannel{fibSources: []fib.FibSource{
		{ID: 3, Name: "API"}, {ID: 31, Name: fibSourceName}, {ID: 32, Name: "lcp-rt"},
	}}
	source, err := openRouteSource(held)
	require.NoError(t, err)
	assert.Equal(t, uint8(31), source.id)
	for _, m := range held.requests {
		_, added := m.(*fib.FibSourceAdd)
		assert.False(t, added, "a source VPP holds is not registered again")
	}

	fresh := &testChannel{fibSourceAddID: 33}
	source, err = openRouteSource(fresh)
	require.NoError(t, err)
	assert.Equal(t, uint8(33), source.id)
	add, ok := fresh.lastRequest.(*fib.FibSourceAdd)
	require.True(t, ok, "got %T", fresh.lastRequest)
	assert.Equal(t, fibSourceName, add.Src.Name)
	assert.Equal(t, uint8(fibSourcePriority), add.Src.Priority)

	refused := &testChannel{retval: -1}
	_, err = openRouteSource(refused)
	require.ErrorIs(t, err, errFibSource)
	findings, err := answerDoctorCheck(staleRoutesCheckName, nil, err)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, codeStaleRoute, findings[0].Code)
	assert.Equal(t, staleRouteFix, findings[0].Fix)

	findings, err = answerDoctorCheck(staleRoutesCheckName, nil, errFibVPPNoConnector)
	require.NoError(t, err)
	assert.Empty(t, findings, "an unreachable VPP is the vpp component's finding")
}

package policy

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// TestRouteAttachmentRefusesFieldTheViewCannotHold proves a route attachment
// cannot declare a field the route view does not hold: carried with no
// RouteView.Read, or settable with no RouteView.Written. Such a declaration
// let `isis export` accept a filter reading origin-as, which then matched a
// zero and rejected every route. Method: a private vocabulary, one field with
// no view and one readable but not writable, and each misdeclaration panics
// at registration while a plain (BGP) attachment may still carry them.
func TestRouteAttachmentRefusesFieldTheViewCannotHold(t *testing.T) {
	v := newVocabulary()
	originAS := v.mustRegisterField("origin-as", RouteView{})
	level := v.mustRegisterField("level", RouteView{Read: func(r *ribpolicy.Route) any { return r.Level }})

	assert.PanicsWithValue(t,
		"BUG: policy: route attachment isis export carries origin-as, which a ribpolicy.Route does not hold",
		func() { v.mustRegisterAttachment("isis", "isis export", []Field{originAS}, nil, true) })
	assert.PanicsWithValue(t,
		"BUG: policy: route attachment isis level lets a filter set level, which ribpolicy.Sets cannot write",
		func() { v.mustRegisterAttachment("isis", "isis level", []Field{level}, []Field{level}, true) })
	assert.NotPanics(t, func() { v.mustRegisterAttachment("bgp", "bgp import", []Field{originAS}, nil, false) })
	assert.False(t, v.isRouteAttachment("bgp import"), "a plain attachment is not a route attachment")
}

// TestRouteViewReadsTheRoute proves every registered field's RouteView reads
// the route view and the Sets it claims, so a route attachment's check is
// derived from accessors that really reach the field. Method: for each field
// in the process vocabulary, a Read answers differently on an empty route and
// on one with every field set, and a Written answers false on empty Sets and
// true on Sets with every field set.
func TestRouteViewReadsTheRoute(t *testing.T) {
	empty := ribpolicy.Route{}
	full := ribpolicy.Route{
		Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("192.0.2.0/24"),
		Source: redistevents.ProtocolID(1), IsBGP: true, IsEBGP: true,
		NextHop: netip.MustParseAddr("192.0.2.1"), Interface: "eth0", Metric: 7,
		Tag: 0, HasTag: true, Level: ribpolicy.ISISLevel(1), OSPFRouteType: ribpolicy.OSPFRouteType(1),
	}
	fullSets := ribpolicy.Sets{
		Distance: 1, HasDistance: true, Tag: 1, HasTag: true, Metric: 1, HasMetric: true,
		MetricType: ribpolicy.ExternalMetricType1,
	}

	vocab.mu.RLock()
	defer vocab.mu.RUnlock()
	readable, writable := 0, 0
	for name, view := range vocab.views {
		if view.Read != nil {
			readable++
			assert.NotEqual(t, view.Read(&empty), view.Read(&full), "field %s: Read does not see the route's value", name)
		}
		if view.Written != nil {
			writable++
			assert.False(t, view.Written(&ribpolicy.Sets{}), "field %s: Written claims empty Sets write it", name)
			assert.True(t, view.Written(&fullSets), "field %s: Written does not see the Sets writing it", name)
		}
	}
	require.NotZero(t, readable, "the vocabulary declares readable fields")
	require.NotZero(t, writable, "the vocabulary declares settable fields")
}

// TestRouteChainRefusesPlainAttachment proves CompileRouteChain refuses an
// attachment registered with MustRegisterAttachment: its routes are not a
// ribpolicy.Route, so its fields were never checked against the view.
func TestRouteChainRefusesPlainAttachment(t *testing.T) {
	const plain = "route-view-test plain"
	MustRegisterAttachment("route-view-test", plain, []Field{FieldPrefix}, nil)
	_, err := CompileRouteChain(plain, nil, FinalAccept, map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MustRegisterRouteAttachment")
}

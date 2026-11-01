// Design: docs/guide/route-policy.md -- the match filter type and its conditions
// Related: match.go -- the condition registry these checks join
// Related: register_match.go -- registers each condition below
//
// The built-in match conditions over a ribpolicy.Route: the fields the route
// view of every engine-judged chain carries. Each is keyed by the field it
// reads, so its key is declared once, in the field vocabulary. A condition
// over a BGP attribute lives in its BGP package and adds its own checks.

package policy

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// RouteConditions is the condition set of a ribpolicy.Route, the view every
// engine-judged chain (Loc-RIB import, redistribute import, IGP export)
// judges. A condition package adds its route check here from init().
var RouteConditions = NewConditions[ribpolicy.Route]("a route an engine-judged chain judges")

// RouteMatch is a match compiled over a ribpolicy.Route.
type RouteMatch = Match[ribpolicy.Route]

// RouteCheck is a condition check over a ribpolicy.Route.
type RouteCheck = ConditionCheck[ribpolicy.Route]

// CompileRouteMatch compiles cfg's conditions over a ribpolicy.Route
// (CompileMatch over RouteConditions).
func CompileRouteMatch(cfg map[string]any, sourceOnly bool, resolve InstanceResolver) (*RouteMatch, error) {
	return CompileMatch(RouteConditions, cfg, sourceOnly, resolve)
}

// The BGP classes the protocol condition names beside the registered sources
// (spec "Route Field Vocabulary": protocol is `ebgp` or `ibgp` for a BGP best).
const (
	protocolClassEBGP = redistevents.SourceNameEBGP
	protocolClassIBGP = redistevents.SourceNameIBGP
)

// routeTypeNames maps the route-type enumeration of the YANG grouping onto the
// OSPF path types the producer states.
var routeTypeNames = map[string]ribpolicy.OSPFRouteType{
	"intra-area": ribpolicy.OSPFIntraArea,
	"inter-area": ribpolicy.OSPFInterArea,
	"external-1": ribpolicy.OSPFExternal1,
	"external-2": ribpolicy.OSPFExternal2,
	"nssa-1":     ribpolicy.OSPFNSSA1,
	"nssa-2":     ribpolicy.OSPFNSSA2,
}

// RouteTypeName returns the name the `route-type` match condition spells t
// with, derived from routeTypeNames so a display and the condition never
// disagree, and false for a type the map does not name (the unspecified one).
func RouteTypeName(t ribpolicy.OSPFRouteType) (string, bool) {
	for name, v := range routeTypeNames {
		if v == t {
			return name, true
		}
	}
	return "", false
}

// matchValues reads a leaf-list condition and refuses an empty one, which
// would match nothing and so hide every route in silence.
func matchValues(value any) ([]string, error) {
	values := configvalue.LeafList(value)
	if len(values) == 0 {
		return nil, errors.New("names no value; list at least one or remove the condition")
	}
	return values, nil
}

func compileRouteFamily(value any, _ InstanceResolver) (RouteCheck, error) {
	values, err := matchValues(value)
	if err != nil {
		return nil, err
	}
	families := make([]family.Family, 0, len(values))
	for _, v := range values {
		fam, ok := family.LookupFamily(v)
		if !ok {
			return nil, fmt.Errorf("%q is not a registered address family (for example ipv4/unicast)", v)
		}
		families = append(families, fam)
	}
	return func(r *ribpolicy.Route) (bool, error) { return slices.Contains(families, r.Family), nil }, nil
}

func compileRouteProtocol(value any, _ InstanceResolver) (RouteCheck, error) {
	values, err := matchValues(value)
	if err != nil {
		return nil, err
	}
	var protocols []redistevents.ProtocolID
	var ebgp, ibgp bool
	for _, v := range values {
		switch v {
		case protocolClassEBGP:
			ebgp = true
			continue
		case protocolClassIBGP:
			ibgp = true
			continue
		}
		id, ok := redistevents.ProtocolIDOf(v)
		if !ok {
			return nil, fmt.Errorf("%q is not a route source this build registers; use one of %s, %s, %s",
				v, strings.Join(redistevents.ProtocolNames(), ", "), protocolClassEBGP, protocolClassIBGP)
		}
		protocols = append(protocols, id)
	}
	return func(r *ribpolicy.Route) (bool, error) {
		if slices.Contains(protocols, r.Source) {
			return true, nil
		}
		if !r.IsBGP {
			return false, nil
		}
		if r.IsEBGP {
			return ebgp, nil
		}
		return ibgp, nil
	}, nil
}

// compileRouteNextHop matches a next hop inside a named prefix. A route with
// no next hop (connected) fails the condition.
func compileRouteNextHop(value any, _ InstanceResolver) (RouteCheck, error) {
	values, err := matchValues(value)
	if err != nil {
		return nil, err
	}
	nextHops := make([]netip.Prefix, 0, len(values))
	for _, v := range values {
		p, err := netip.ParsePrefix(v)
		if err != nil {
			return nil, fmt.Errorf("%q is not an address prefix (for example 192.0.2.0/24): %w", v, err)
		}
		nextHops = append(nextHops, p.Masked())
	}
	return func(r *ribpolicy.Route) (bool, error) {
		if !r.NextHop.IsValid() {
			return false, nil
		}
		for _, p := range nextHops {
			if p.Contains(r.NextHop) {
				return true, nil
			}
		}
		return false, nil
	}, nil
}

func compileRouteInterface(value any, _ InstanceResolver) (RouteCheck, error) {
	interfaces, err := matchValues(value)
	if err != nil {
		return nil, err
	}
	return func(r *ribpolicy.Route) (bool, error) { return slices.Contains(interfaces, r.Interface), nil }, nil
}

func compileRouteMetric(value any, _ InstanceResolver) (RouteCheck, error) {
	cfg, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("must be a container holding minimum and maximum")
	}
	metricMin, metricMax := uint32(0), ^uint32(0)
	if v, named := cfg["minimum"]; named {
		parsed, err := parseUint32(v)
		if err != nil {
			return nil, fmt.Errorf("minimum: %w", err)
		}
		metricMin = parsed
	}
	if v, named := cfg["maximum"]; named {
		parsed, err := parseUint32(v)
		if err != nil {
			return nil, fmt.Errorf("maximum: %w", err)
		}
		metricMax = parsed
	}
	if metricMin > metricMax {
		return nil, fmt.Errorf("minimum %d is above maximum %d, so no route could match", metricMin, metricMax)
	}
	return func(r *ribpolicy.Route) (bool, error) {
		if r.Metric < metricMin {
			return false, nil
		}
		return r.Metric <= metricMax, nil
	}, nil
}

// compileRouteTag matches a tag the route carries: 0 is a real tag, so an
// untagged route fails the condition.
func compileRouteTag(value any, _ InstanceResolver) (RouteCheck, error) {
	values, err := matchValues(value)
	if err != nil {
		return nil, err
	}
	tags := make([]uint32, 0, len(values))
	for _, v := range values {
		tag, err := parseUint32(v)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return func(r *ribpolicy.Route) (bool, error) {
		if !r.HasTag {
			return false, nil
		}
		return slices.Contains(tags, r.Tag), nil
	}, nil
}

func compileRouteLevel(value any, _ InstanceResolver) (RouteCheck, error) {
	values, err := matchValues(value)
	if err != nil {
		return nil, err
	}
	levels := make([]ribpolicy.ISISLevel, 0, len(values))
	for _, v := range values {
		switch v {
		case "1":
			levels = append(levels, ribpolicy.ISISLevel1)
		case "2":
			levels = append(levels, ribpolicy.ISISLevel2)
		default:
			return nil, fmt.Errorf("%q is not an IS-IS level; use 1 or 2", v)
		}
	}
	return func(r *ribpolicy.Route) (bool, error) { return slices.Contains(levels, r.Level), nil }, nil
}

func compileRouteRouteType(value any, _ InstanceResolver) (RouteCheck, error) {
	values, err := matchValues(value)
	if err != nil {
		return nil, err
	}
	routeTypes := make([]ribpolicy.OSPFRouteType, 0, len(values))
	for _, v := range values {
		routeType, ok := routeTypeNames[v]
		if !ok {
			return nil, fmt.Errorf("%q is not an OSPF route type; use intra-area, inter-area, external-1, external-2, nssa-1 or nssa-2", v)
		}
		routeTypes = append(routeTypes, routeType)
	}
	return func(r *ribpolicy.Route) (bool, error) { return slices.Contains(routeTypes, r.OSPFRouteType), nil }, nil
}

// parseUint32 reads a uint32 leaf value. The YANG range already bounds it;
// the check is paired here because a JSON delivery reaches this reader too.
func parseUint32(value any) (uint32, error) {
	text, ok := value.(string)
	if !ok {
		return 0, fmt.Errorf("%v is not a number", value)
	}
	parsed, err := strconv.ParseUint(text, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%q is not a number from 0 to 4294967295", text)
	}
	return uint32(parsed), nil
}

// compileMatchEvaluator compiles one match instance into a route evaluator.
// With action accept, a route meeting every named condition passes on and
// any other is rejected; with action reject, a route meeting them is rejected
// and any other passes on. A check that fails (a route condition has none
// today) rejects the route: the chain cannot judge it, so it does not pass.
func compileMatchEvaluator(_ string, instance map[string]any, resolve InstanceResolver) (RouteEvaluator, error) {
	m, err := CompileRouteMatch(instance, false, resolve)
	if err != nil {
		return nil, err
	}
	return func(r ribpolicy.Route) RouteResult {
		rejects, err := m.Rejects(&r)
		if err != nil {
			return RouteResult{Reject: true}
		}
		return RouteResult{Reject: rejects}
	}, nil
}

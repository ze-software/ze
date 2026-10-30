// Design: docs/guide/route-policy.md -- the match filter type and its conditions
// Related: rib.go -- the route evaluator a match instance compiles into
// Related: fields.go -- the per-instance field declaration the conditions feed
// Related: register.go -- the match filter type registration
//
// The route match conditions. One table names every condition, the config key
// that holds it and the route fields it reads, so the field check and the
// compiled matcher are derived from the same declaration: an instance is
// checked against exactly the fields its matcher reads. The `match` filter type
// is built from these conditions alone; `modify` reuses the source conditions
// (protocol, tag, level, route-type) beside its community matches.

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

// MatchFilterType is the ze:filter list of the generic match filter.
const MatchFilterType = "match"

// prefixListFilterType is the filter type a match's prefix-list condition
// references. It is the BGP prefix-list plugin's list; in a build without it
// no instance of that type exists, so the reference is refused by name.
const prefixListFilterType = "prefix-list"

// The config keys of the conditions.
const (
	matchKeyPrefixList = "prefix-list"
	matchKeyFamily     = "family"
	matchKeyProtocol   = "protocol"
	matchKeyNextHop    = "next-hop"
	matchKeyInterface  = "interface"
	matchKeyMetric     = "metric"
	matchKeyTag        = "tag"
	matchKeyLevel      = "level"
	matchKeyRouteType  = "route-type"
)

// The BGP classes the protocol condition names beside the registered sources
// (spec "Route Field Vocabulary": protocol is `ebgp` or `ibgp` for a BGP best).
const (
	protocolClassEBGP = "ebgp"
	protocolClassIBGP = "ibgp"
)

// matchCondition is one row of the condition table.
type matchCondition struct {
	key    string
	reads  []Field
	source bool // a source condition: modify carries it too
	parse  func(m *RouteMatch, value any, resolve InstanceResolver) error
}

// matchConditions is the condition table. Every reader of an instance's
// conditions walks it: routeMatchFields for the field check, CompileRouteMatch
// for the matcher.
var matchConditions = []matchCondition{
	{key: matchKeyPrefixList, reads: []Field{FieldPrefix, FieldFamily}, parse: parseMatchPrefixList},
	{key: matchKeyFamily, reads: []Field{FieldFamily}, parse: parseMatchFamily},
	{key: matchKeyProtocol, reads: []Field{FieldProtocol}, source: true, parse: parseMatchProtocol},
	{key: matchKeyNextHop, reads: []Field{FieldNextHop}, parse: parseMatchNextHop},
	{key: matchKeyInterface, reads: []Field{FieldInterface}, parse: parseMatchInterface},
	{key: matchKeyMetric, reads: []Field{FieldMetric}, parse: parseMatchMetric},
	{key: matchKeyTag, reads: []Field{FieldTag}, source: true, parse: parseMatchTag},
	{key: matchKeyLevel, reads: []Field{FieldLevel}, source: true, parse: parseMatchLevel},
	{key: matchKeyRouteType, reads: []Field{FieldRouteType}, source: true, parse: parseMatchRouteType},
}

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

// RouteMatch is a compiled set of conditions. A route matches when every
// condition the instance names holds; within one condition the values are
// alternatives. A route that does not carry a field a condition reads (no tag,
// no level) fails that condition: absence is never a match. Immutable after
// CompileRouteMatch, so safe for concurrent use.
type RouteMatch struct {
	prefixList RouteEvaluator // nil: no prefix-list condition
	families   []family.Family
	protocols  []redistevents.ProtocolID
	ebgp, ibgp bool
	nextHops   []netip.Prefix
	interfaces []string
	hasMetric  bool
	metricMin  uint32
	metricMax  uint32
	tags       []uint32
	levels     []ribpolicy.ISISLevel
	routeTypes []ribpolicy.OSPFRouteType
	named      int // the number of conditions the instance names
	// rejectMatching is the action leaf: true rejects a route meeting every
	// condition and passes the rest on, false (accept) does the opposite.
	rejectMatching bool
}

// MatchFields answers the route fields the conditions in cfg read, in table
// order. sourceOnly restricts it to the source conditions (modify's).
func MatchFields(cfg map[string]any, sourceOnly bool) []Field {
	var reads []Field
	for i := range matchConditions {
		c := &matchConditions[i]
		if sourceOnly && !c.source {
			continue
		}
		if _, named := cfg[c.key]; !named {
			continue
		}
		for _, f := range c.reads {
			if !slices.Contains(reads, f) {
				reads = append(reads, f)
			}
		}
	}
	return reads
}

// CompileRouteMatch compiles the conditions cfg names and, for a match
// instance, its action leaf. sourceOnly compiles only the source conditions,
// for a caller (modify) whose other keys are its own and whose match block
// has no action. A value that does not parse, or a reference resolve refuses,
// is an error naming the condition.
func CompileRouteMatch(cfg map[string]any, sourceOnly bool, resolve InstanceResolver) (*RouteMatch, error) {
	m := &RouteMatch{}
	for i := range matchConditions {
		c := &matchConditions[i]
		if sourceOnly && !c.source {
			continue
		}
		value, named := cfg[c.key]
		if !named {
			continue
		}
		if err := c.parse(m, value, resolve); err != nil {
			return nil, fmt.Errorf("match %s: %w", c.key, err)
		}
		m.named++
	}
	if sourceOnly {
		return m, nil
	}
	rejectMatching, err := parseMatchAction(cfg)
	if err != nil {
		return nil, err
	}
	m.rejectMatching = rejectMatching
	return m, nil
}

// IsSourceConditionKey reports whether key holds a source condition, the
// conditions modify carries beside its community matches.
func IsSourceConditionKey(key string) bool {
	for i := range matchConditions {
		if matchConditions[i].source && matchConditions[i].key == key {
			return true
		}
	}
	return false
}

// Rejects answers the match instance's verdict on r: with action accept, r is
// rejected unless it meets every condition; with action reject, r is
// rejected when it meets them. It allocates nothing.
func (m *RouteMatch) Rejects(r *ribpolicy.Route) bool {
	return m.Matches(r) == m.rejectMatching
}

// Named reports whether the match names any condition.
func (m *RouteMatch) Named() bool { return m.named > 0 }

// Matches reports whether r meets every condition. It allocates nothing.
func (m *RouteMatch) Matches(r *ribpolicy.Route) bool {
	if m.prefixList != nil {
		if m.prefixList(*r).Reject {
			return false
		}
	}
	if m.families != nil {
		if !slices.Contains(m.families, r.Family) {
			return false
		}
	}
	if !m.matchesProtocol(r) {
		return false
	}
	if !m.matchesNextHop(r.NextHop) {
		return false
	}
	if m.interfaces != nil {
		if !slices.Contains(m.interfaces, r.Interface) {
			return false
		}
	}
	if m.hasMetric {
		if r.Metric < m.metricMin {
			return false
		}
		if r.Metric > m.metricMax {
			return false
		}
	}
	if m.tags != nil {
		if !r.HasTag {
			return false
		}
		if !slices.Contains(m.tags, r.Tag) {
			return false
		}
	}
	if m.levels != nil {
		if !slices.Contains(m.levels, r.Level) {
			return false
		}
	}
	if m.routeTypes != nil {
		if !slices.Contains(m.routeTypes, r.OSPFRouteType) {
			return false
		}
	}
	return true
}

// matchesProtocol reports whether r's source is one the condition names. The
// condition is unnamed when it holds no source and no BGP class.
func (m *RouteMatch) matchesProtocol(r *ribpolicy.Route) bool {
	if m.protocols == nil && !m.ebgp && !m.ibgp {
		return true
	}
	if slices.Contains(m.protocols, r.Source) {
		return true
	}
	if !r.IsBGP {
		return false
	}
	if r.IsEBGP {
		return m.ebgp
	}
	return m.ibgp
}

// matchesNextHop reports whether r's next hop lies in a named prefix. A route
// with no next hop (connected) fails a named condition.
func (m *RouteMatch) matchesNextHop(nextHop netip.Addr) bool {
	if m.nextHops == nil {
		return true
	}
	if !nextHop.IsValid() {
		return false
	}
	for _, p := range m.nextHops {
		if p.Contains(nextHop) {
			return true
		}
	}
	return false
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

func parseMatchPrefixList(m *RouteMatch, value any, resolve InstanceResolver) error {
	name, ok := value.(string)
	if !ok {
		return errors.New("must name one prefix-list instance")
	}
	if name == "" {
		return errors.New("must name one prefix-list instance")
	}
	if resolve == nil {
		return errors.New("BUG: compiled with no instance resolver")
	}
	evaluator, err := resolve(name, prefixListFilterType)
	if err != nil {
		return err
	}
	m.prefixList = evaluator
	return nil
}

func parseMatchFamily(m *RouteMatch, value any, _ InstanceResolver) error {
	values, err := matchValues(value)
	if err != nil {
		return err
	}
	for _, v := range values {
		fam, ok := family.LookupFamily(v)
		if !ok {
			return fmt.Errorf("%q is not a registered address family (for example ipv4/unicast)", v)
		}
		m.families = append(m.families, fam)
	}
	return nil
}

func parseMatchProtocol(m *RouteMatch, value any, _ InstanceResolver) error {
	values, err := matchValues(value)
	if err != nil {
		return err
	}
	for _, v := range values {
		switch v {
		case protocolClassEBGP:
			m.ebgp = true
			continue
		case protocolClassIBGP:
			m.ibgp = true
			continue
		}
		id, ok := redistevents.ProtocolIDOf(v)
		if !ok {
			return fmt.Errorf("%q is not a route source this build registers; use one of %s, %s, %s",
				v, strings.Join(redistevents.ProtocolNames(), ", "), protocolClassEBGP, protocolClassIBGP)
		}
		m.protocols = append(m.protocols, id)
	}
	return nil
}

func parseMatchNextHop(m *RouteMatch, value any, _ InstanceResolver) error {
	values, err := matchValues(value)
	if err != nil {
		return err
	}
	for _, v := range values {
		p, err := netip.ParsePrefix(v)
		if err != nil {
			return fmt.Errorf("%q is not an address prefix (for example 192.0.2.0/24): %w", v, err)
		}
		m.nextHops = append(m.nextHops, p.Masked())
	}
	return nil
}

func parseMatchInterface(m *RouteMatch, value any, _ InstanceResolver) error {
	values, err := matchValues(value)
	if err != nil {
		return err
	}
	m.interfaces = values
	return nil
}

func parseMatchMetric(m *RouteMatch, value any, _ InstanceResolver) error {
	cfg, ok := value.(map[string]any)
	if !ok {
		return errors.New("must be a container holding minimum and maximum")
	}
	m.metricMin, m.metricMax = 0, ^uint32(0)
	if v, named := cfg["minimum"]; named {
		parsed, err := parseUint32(v)
		if err != nil {
			return fmt.Errorf("minimum: %w", err)
		}
		m.metricMin = parsed
	}
	if v, named := cfg["maximum"]; named {
		parsed, err := parseUint32(v)
		if err != nil {
			return fmt.Errorf("maximum: %w", err)
		}
		m.metricMax = parsed
	}
	if m.metricMin > m.metricMax {
		return fmt.Errorf("minimum %d is above maximum %d, so no route could match", m.metricMin, m.metricMax)
	}
	m.hasMetric = true
	return nil
}

func parseMatchTag(m *RouteMatch, value any, _ InstanceResolver) error {
	values, err := matchValues(value)
	if err != nil {
		return err
	}
	for _, v := range values {
		tag, err := parseUint32(v)
		if err != nil {
			return err
		}
		m.tags = append(m.tags, tag)
	}
	return nil
}

func parseMatchLevel(m *RouteMatch, value any, _ InstanceResolver) error {
	values, err := matchValues(value)
	if err != nil {
		return err
	}
	for _, v := range values {
		switch v {
		case "1":
			m.levels = append(m.levels, ribpolicy.ISISLevel1)
		case "2":
			m.levels = append(m.levels, ribpolicy.ISISLevel2)
		default:
			return fmt.Errorf("%q is not an IS-IS level; use 1 or 2", v)
		}
	}
	return nil
}

func parseMatchRouteType(m *RouteMatch, value any, _ InstanceResolver) error {
	values, err := matchValues(value)
	if err != nil {
		return err
	}
	for _, v := range values {
		routeType, ok := routeTypeNames[v]
		if !ok {
			return fmt.Errorf("%q is not an OSPF route type; use intra-area, inter-area, external-1, external-2, nssa-1 or nssa-2", v)
		}
		m.routeTypes = append(m.routeTypes, routeType)
	}
	return nil
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

// matchInstanceFields is the match filter type's per-instance declaration.
func matchInstanceFields(instance map[string]any) (reads, sets []Field) {
	return MatchFields(instance, false), nil
}

// matchKeyAction is the match instance's action leaf. It is not a condition:
// it reads no route field, so it is not in matchConditions.
const matchKeyAction = "action"

// The values of the action leaf, as the YANG enumeration spells them.
const (
	matchActionAccept = "accept"
	matchActionReject = "reject"
)

// parseMatchAction answers whether a route meeting the conditions is rejected.
// An instance that leaves the leaf out accepts, the YANG default; any value
// other than the two the enumeration names is refused, never read as accept.
func parseMatchAction(instance map[string]any) (rejectMatching bool, err error) {
	value, named := instance[matchKeyAction]
	if !named {
		return false, nil
	}
	switch value {
	case matchActionAccept:
		return false, nil
	case matchActionReject:
		return true, nil
	}
	return false, fmt.Errorf("match action %v: want %s or %s", value, matchActionAccept, matchActionReject)
}

// compileMatchEvaluator compiles one match instance. With action accept, a
// route meeting every named condition passes on and any other is rejected;
// with action reject, a route meeting them is rejected and any other passes on.
func compileMatchEvaluator(_ string, instance map[string]any, resolve InstanceResolver) (RouteEvaluator, error) {
	m, err := CompileRouteMatch(instance, false, resolve)
	if err != nil {
		return nil, err
	}
	return func(r ribpolicy.Route) RouteResult {
		return RouteResult{Reject: m.Rejects(&r)}
	}, nil
}

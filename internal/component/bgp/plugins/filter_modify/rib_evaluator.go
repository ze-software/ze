// Design: docs/guide/route-policy.md -- modify at a Loc-RIB attachment
// Related: config.go -- parseModifyDefs, the BGP path's parse of the same instance
// Related: register.go -- the route evaluator and per-instance field registrations
//
// The modify route evaluator for the Loc-RIB import gate. At a Loc-RIB
// attachment a modify instance matches on where the route came from (protocol,
// tag, level, route-type) and sets the administrative distance and the tag;
// at an IGP export it sets the advertised metric and the OSPF metric type. A
// route that does not match passes on unchanged. The BGP attribute operations
// of the same filter type run on BGP chains only; the per-instance field
// declaration refuses each kind at the attachment that cannot take it.

package filter_modify

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/component/policy"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// The route set keys of a modify set block: distance and tag at a Loc-RIB
// import, metric and metric-type at an IGP export. The attachment's settable
// fields decide which of them a chain there may use.
const (
	ribSetDistance   = "distance"
	ribSetTag        = "tag"
	ribSetMetric     = "metric"
	ribSetMetricType = "metric-type"
)

// The metric-type values, as the YANG enumeration spells them.
const (
	metricTypeName1 = "type-1"
	metricTypeName2 = "type-2"
)

// ribSetKeys are the set keys the route evaluator applies.
var ribSetKeys = map[string]bool{ribSetDistance: true, ribSetTag: true, ribSetMetric: true, ribSetMetricType: true}

// matchKeyFields maps modify's own match keys onto the fields they read.
var matchKeyFields = map[string]policy.Field{
	"community":          policy.FieldCommunity,
	"large-community":    policy.FieldLargeCommunity,
	"extended-community": policy.FieldExtendedCommunity,
}

// setKeyFields maps every set key onto the field it sets. A key missing here
// would set a field no attachment is checked for, so a test compares it with
// setBlockAllowedKeys and ribSetKeys.
var setKeyFields = map[string]policy.Field{
	localPreferenceAttr:              policy.FieldLocalPreference,
	medAttr:                          policy.FieldMED,
	originAttr:                       policy.FieldOrigin,
	nextHopAttr:                      policy.FieldNextHop,
	asPathPrependAttr:                policy.FieldASPath,
	communityAddDirective:            policy.FieldCommunity,
	communityRemoveDirective:         policy.FieldCommunity,
	largeCommunityAddDirective:       policy.FieldLargeCommunity,
	largeCommunityRemoveDirective:    policy.FieldLargeCommunity,
	extendedCommunityAddDirective:    policy.FieldExtendedCommunity,
	extendedCommunityRemoveDirective: policy.FieldExtendedCommunity,
	ribSetDistance:                   policy.FieldDistance,
	ribSetTag:                        policy.FieldTag,
	ribSetMetric:                     policy.FieldMetric,
	ribSetMetricType:                 policy.FieldMetricType,
}

// incDecKeyFields maps the increment, decrement and del keys onto the field
// each one sets.
var incDecKeyFields = map[string]policy.Field{
	localPreferenceAttr: policy.FieldLocalPreference,
	medAttr:             policy.FieldMED,
	aigpAttr:            policy.FieldAIGP,
}

// hasRIBSet reports whether a modify instance's set block holds a Loc-RIB set.
func hasRIBSet(defMap map[string]any) bool {
	setBlock, _ := defMap["set"].(map[string]any) // no set block holds no set
	for key := range setBlock {
		if ribSetKeys[key] {
			return true
		}
	}
	return false
}

// modifyInstanceFields is modify's per-instance field declaration: the
// fields its match reads and the fields its operations set, so a BGP-only
// instance (community sets) is refused at a Loc-RIB attachment and a Loc-RIB
// one (set distance) on a BGP chain. A key the parser does not know is left
// to parseModifyDefs, which refuses it.
func modifyInstanceFields(instance map[string]any) (reads, sets []policy.Field) {
	if match, ok := instance["match"].(map[string]any); ok {
		for key := range match {
			if f, known := matchKeyFields[key]; known {
				reads = appendField(reads, f)
			}
		}
		for _, f := range policy.MatchFields(match, true) {
			reads = appendField(reads, f)
		}
	}
	if set, ok := instance["set"].(map[string]any); ok {
		for key := range set {
			if f, known := setKeyFields[key]; known {
				sets = appendField(sets, f)
			}
		}
	}
	for _, block := range [...]string{"del", "increment", "decrement"} {
		ops, _ := instance[block].(map[string]any) // an absent block sets nothing
		for key := range ops {
			if f, known := incDecKeyFields[key]; known {
				sets = appendField(sets, f)
			}
		}
	}
	return reads, sets
}

func appendField(fields []policy.Field, f policy.Field) []policy.Field {
	if slices.Contains(fields, f) {
		return fields
	}
	return append(fields, f)
}

// compileRouteEvaluator compiles one modify instance into a Loc-RIB route
// evaluator. The field check has already refused BGP content at a Loc-RIB
// attachment; the refusals here pair that check, so a call that skipped it
// cannot install a modifier that silently does nothing.
func compileRouteEvaluator(name string, instance map[string]any, resolve policy.InstanceResolver) (policy.RouteEvaluator, error) {
	for key := range instance {
		switch key {
		case "match", "set":
		default:
			return nil, fmt.Errorf("modify %q: %s holds BGP attribute operations, which a Loc-RIB route does not carry; "+
				"use a separate modify instance on the BGP chain", name, key)
		}
	}
	match, _ := instance["match"].(map[string]any) // no match block: every route matches
	for key := range match {
		if !policy.IsSourceConditionKey(key) {
			return nil, fmt.Errorf("modify %q: match %s reads a BGP attribute, which a Loc-RIB route does not carry", name, key)
		}
	}
	cond, err := policy.CompileRouteMatch(match, true, resolve)
	if err != nil {
		return nil, fmt.Errorf("modify %q: %w", name, err)
	}
	sets, err := parseRIBSets(name, instance)
	if err != nil {
		return nil, err
	}
	return func(r ribpolicy.Route) policy.RouteResult {
		if !cond.Matches(&r) {
			return policy.RouteResult{}
		}
		return policy.RouteResult{Sets: sets}
	}, nil
}

// metricMax is the largest metric set metric takes: the 24-bit metric field
// of an OSPF AS-external LSA (RFC 2328 Appendix A.4.5). An IS-IS wide metric
// is wider, so every value fits there too.
const metricMax = 1<<24 - 1

// parseRIBSets reads the route sets of a modify instance and refuses one
// that sets nothing a Loc-RIB route takes.
func parseRIBSets(name string, instance map[string]any) (ribpolicy.Sets, error) {
	var sets ribpolicy.Sets
	set, _ := instance["set"].(map[string]any) // no set block is refused below
	for key, value := range set {
		text, ok := value.(string)
		if !ok {
			return ribpolicy.Sets{}, fmt.Errorf("modify %q: set %s: %v is not a number", name, key, value)
		}
		switch key {
		case ribSetDistance:
			distance, err := strconv.ParseUint(text, 10, 8)
			if err != nil {
				return ribpolicy.Sets{}, fmt.Errorf("modify %q: set distance %q is not a number from 0 to 255", name, text)
			}
			sets.Distance, sets.HasDistance = uint8(distance), true
		case ribSetTag:
			tag, err := strconv.ParseUint(text, 10, 32)
			if err != nil {
				return ribpolicy.Sets{}, fmt.Errorf("modify %q: set tag %q is not a number from 0 to 4294967295", name, text)
			}
			sets.Tag, sets.HasTag = uint32(tag), true
		case ribSetMetric:
			metric, err := strconv.ParseUint(text, 10, 32)
			if err != nil {
				return ribpolicy.Sets{}, fmt.Errorf("modify %q: set metric %q is not a number from 0 to 16777215", name, text)
			}
			if metric > metricMax {
				return ribpolicy.Sets{}, fmt.Errorf("modify %q: set metric %d is above 16777215, the largest metric an OSPF AS-external LSA carries", name, metric)
			}
			sets.Metric, sets.HasMetric = uint32(metric), true
		case ribSetMetricType:
			switch text {
			case metricTypeName1:
				sets.MetricType = ribpolicy.ExternalMetricType1
			case metricTypeName2:
				sets.MetricType = ribpolicy.ExternalMetricType2
			default:
				return ribpolicy.Sets{}, fmt.Errorf("modify %q: set metric-type %q is not type-1 or type-2", name, text)
			}
		default:
			return ribpolicy.Sets{}, fmt.Errorf("modify %q: set %s changes a BGP attribute, which a Loc-RIB route does not carry", name, key)
		}
	}
	if !sets.HasDistance && !sets.HasTag && !sets.HasMetric && sets.MetricType == ribpolicy.ExternalMetricTypeUnspecified {
		return ribpolicy.Sets{}, errors.New("modify " + strconv.Quote(name) + ": sets nothing a route takes; set distance, tag, metric or metric-type")
	}
	return sets, nil
}

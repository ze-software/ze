// Design: docs/architecture/api/architecture.md -- runtime non-direction event tag extraction and matching for compiled bus filters.
// Related: bus_filter.go -- CompiledBus deliveryIndex and BusTagMatcher compilation.
// Related: bus_tag_intern.go -- TagInterner read-only LookupValue and zero-alloc BusTagMatcher.
// Related: bus_op_gate.go -- filterReceiversByCompiledSender runtime enforcement.
// Related: dispatch.go -- deliverEventCaptured runtime delivery pipeline.

package server

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/events"
)

// busTagStackCap is the stack scratch capacity for extracted per-event tags so
// common tagged events avoid heap allocation on the delivery path.
const busTagStackCap = 8

// collectDeliveryFilterTags gathers the sorted, deduplicated non-direction tags
// tested by any receiver in receivers, resolving tag names once at compile time.
func collectDeliveryFilterTags(interner *TagInterner, receivers []compiledReceiver) []busFilterTag {
	if interner == nil {
		return nil
	}
	var out []busFilterTag
	for i := range receivers {
		out = appendMatcherFilterTags(interner, receivers[i].TagFilter, out)
	}
	return sortCompactFilterTags(out)
}

// compileInboundTagIndex indexes every endpoint's non-direction InboundTags
// matcher by (endpoint, ns, et) and records the distinct tags needed for each
// (ns, et). Leaves both maps nil when no participant carries a non-direction
// inbound tag filter so unfiltered deliveries pay only a nil-map check.
func compileInboundTagIndex(bus *CompiledBus) {
	if bus == nil {
		return
	}
	if bus.interner == nil {
		return
	}
	for _, f := range bus.filters {
		if f == nil {
			continue
		}
		if f.EndpointName == "" {
			continue
		}
		for msg, rule := range f.ReceiveRules {
			if rule == nil {
				continue
			}
			if rule.InboundTags == nil {
				continue
			}
			nsID, etID, ok := resolveMessageEventIDs(msg)
			if !ok {
				continue
			}
			if bus.inboundTagMatchers == nil {
				bus.inboundTagMatchers = make(map[busEndpointMsgKey]*BusTagMatcher)
				bus.inboundTagIndex = make(map[busMessageKey][]busFilterTag)
			}
			bus.inboundTagMatchers[busEndpointMsgKey{
				endpoint: f.EndpointName,
				ns:       nsID,
				et:       etID,
			}] = rule.InboundTags
			mk := busMessageKey{ns: nsID, et: etID}
			bus.inboundTagIndex[mk] = appendMatcherFilterTags(bus.interner, rule.InboundTags, bus.inboundTagIndex[mk])
		}
	}
	for mk, tags := range bus.inboundTagIndex {
		bus.inboundTagIndex[mk] = sortCompactFilterTags(tags)
	}
}

func appendMatcherFilterTags(interner *TagInterner, matcher *BusTagMatcher, dst []busFilterTag) []busFilterTag {
	if matcher == nil {
		return dst
	}
	for i := range matcher.rules {
		tagID := matcher.rules[i].tag
		name, ok := interner.TagName(tagID)
		if !ok {
			continue
		}
		dst = append(dst, busFilterTag{name: name, id: tagID})
	}
	return dst
}

func sortCompactFilterTags(tags []busFilterTag) []busFilterTag {
	if len(tags) <= 1 {
		return tags
	}
	slices.SortFunc(tags, func(a, b busFilterTag) int {
		if a.id < b.id {
			return -1
		}
		if a.id > b.id {
			return 1
		}
		return strings.Compare(a.name, b.name)
	})
	return slices.Compact(tags)
}

// hasEndpoint reports whether endpointName is represented as a participant in
// b without allocating a "/bus/endpoint/<name>" path string. Allocates nothing.
func (b *CompiledBus) hasEndpoint(endpointName string) bool {
	if b == nil {
		return false
	}
	_, ok := b.endpointToIdx[endpointName]
	return ok
}

// deliveryFilterTags returns the precomputed non-direction tags tested by any
// receiver on (senderPath, ns, et, dir), or nil when no receiver on that edge
// carries a runtime tag filter. Allocates nothing.
func (b *CompiledBus) deliveryFilterTags(
	senderPath string,
	ns events.NamespaceID,
	et events.EventTypeID,
	dir events.Direction,
) []busFilterTag {
	if b == nil {
		return nil
	}
	senderIdx, ok := b.pathToIdx[senderPath]
	// refuse-by-default: guards filterReceiversByCompiledSender -- unknown sender has no compiled delivery tag filters.
	if !ok {
		return nil
	}
	entry, ok := b.deliveryIndex[busDeliveryKey{
		senderIdx: senderIdx,
		ns:        ns,
		et:        et,
		dir:       dir,
	}]
	// refuse-by-default: guards filterReceiversByCompiledSender -- absent delivery entry carries no runtime tag filters.
	if !ok {
		return nil
	}
	return entry.filterTags
}

// inboundTagFilterFor returns the precomputed non-direction tags tested by any
// endpoint on (ns, et), or nil when no endpoint filters (ns, et) by tags.
// Allocates nothing.
func (b *CompiledBus) inboundTagFilterFor(ns events.NamespaceID, et events.EventTypeID) []busFilterTag {
	if b == nil {
		return nil
	}
	if len(b.inboundTagIndex) == 0 {
		return nil
	}
	return b.inboundTagIndex[busMessageKey{ns: ns, et: et}]
}

// inboundTagMatcher returns endpoint's compiled non-direction InboundTags
// matcher for (ns, et), or nil when endpoint has no tag filter on that message.
// Allocates nothing.
func (b *CompiledBus) inboundTagMatcher(endpoint string, ns events.NamespaceID, et events.EventTypeID) *BusTagMatcher {
	if b == nil {
		return nil
	}
	if len(b.inboundTagMatchers) == 0 {
		return nil
	}
	return b.inboundTagMatchers[busEndpointMsgKey{
		endpoint: endpoint,
		ns:       ns,
		et:       et,
	}]
}

// filterReceiversByInboundTags enforces compiled non-direction InboundTags
// matchers on candidate subscriber processes when an event is delivered without
// a compiled sender edge (engine-emitted BroadcastEvent / EmitEngineEvent or
// peer-scoped delivery). When no candidate process carries a non-direction tag
// filter, it returns procs immediately with zero allocations and zero tag
// comparisons.
func filterReceiversByInboundTags(
	compiled *CompiledBus,
	nsID events.NamespaceID,
	etID events.EventTypeID,
	payload any,
	procs []*process.Process,
) []*process.Process {
	if compiled == nil {
		return procs
	}
	if len(procs) == 0 {
		return procs
	}
	if len(compiled.inboundTagMatchers) == 0 {
		return procs
	}
	hasCandidateFilter := false
	for _, p := range procs {
		if compiled.inboundTagMatcher(p.Name(), nsID, etID) != nil {
			hasCandidateFilter = true
			break
		}
	}
	if !hasCandidateFilter {
		return procs
	}
	filterTags := compiled.inboundTagFilterFor(nsID, etID)
	var tagBuf [busTagStackCap]BusTagValue
	var msgTags []BusTagValue
	if len(filterTags) > 0 {
		msgTags = extractEventBusTags(compiled.Interner(), filterTags, payload, tagBuf[:0])
	}
	kept := procs[:0]
	for _, p := range procs {
		matcher := compiled.inboundTagMatcher(p.Name(), nsID, etID)
		if matcher != nil {
			if !matcher.Matches(msgTags) {
				continue
			}
		}
		kept = append(kept, p)
	}
	return kept
}

// extractEventBusTags extracts interned tag values for filterTags from an event
// payload without mutating interner (read-only LookupValue). Supports JSON
// strings, json.RawMessage, []byte, map[string]string, map[string]any, and
// JSON-marshalable Go structs; reads top-level "<tag>" fields and falls back to
// a nested "tags" object when a tag is absent at top level.
func extractEventBusTags(
	interner *TagInterner,
	filterTags []busFilterTag,
	payload any,
	dst []BusTagValue,
) []BusTagValue {
	if interner == nil {
		return dst
	}
	if len(filterTags) == 0 {
		return dst
	}
	if payload == nil {
		return dst
	}
	switch p := payload.(type) {
	case string:
		return extractEventBusTagsFromJSON(interner, filterTags, []byte(p), dst)
	case json.RawMessage:
		return extractEventBusTagsFromJSON(interner, filterTags, p, dst)
	case []byte:
		return extractEventBusTagsFromJSON(interner, filterTags, p, dst)
	case map[string]string:
		for i := range filterTags {
			ft := &filterTags[i]
			if val, ok := p[ft.name]; ok {
				dst = appendInternedTagValue(interner, ft.id, val, dst)
			}
		}
		return dst
	case map[string]any:
		return extractEventBusTagsFromMap(interner, filterTags, p, dst)
	default:
		raw, err := json.Marshal(payload)
		if err != nil {
			return dst
		}
		return extractEventBusTagsFromJSON(interner, filterTags, raw, dst)
	}
}

func extractEventBusTagsFromJSON(
	interner *TagInterner,
	filterTags []busFilterTag,
	raw []byte,
	dst []BusTagValue,
) []BusTagValue {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return dst
	}
	if trimmed[0] != '{' {
		return dst
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return dst
	}
	if len(obj) == 0 {
		return dst
	}
	var tagsObj map[string]json.RawMessage
	tagsParsed := false
	for i := range filterTags {
		ft := &filterTags[i]
		if fieldRaw, ok := obj[ft.name]; ok {
			dst = appendJSONTagValues(interner, ft.id, fieldRaw, dst)
			continue
		}
		if !tagsParsed {
			tagsParsed = true
			if rawTags, hasTags := obj["tags"]; hasTags {
				if err := json.Unmarshal(rawTags, &tagsObj); err != nil {
					tagsObj = nil
				}
			}
		}
		if fieldRaw, ok := tagsObj[ft.name]; ok {
			dst = appendJSONTagValues(interner, ft.id, fieldRaw, dst)
		}
	}
	return dst
}

func extractEventBusTagsFromMap(
	interner *TagInterner,
	filterTags []busFilterTag,
	obj map[string]any,
	dst []BusTagValue,
) []BusTagValue {
	for i := range filterTags {
		ft := &filterTags[i]
		if v, ok := obj[ft.name]; ok {
			dst = appendAnyTagValues(interner, ft.id, v, dst)
			continue
		}
		rawTags, hasTags := obj["tags"]
		if !hasTags {
			continue
		}
		switch tagsMap := rawTags.(type) {
		case map[string]string:
			if val, ok := tagsMap[ft.name]; ok {
				dst = appendInternedTagValue(interner, ft.id, val, dst)
			}
		case map[string]any:
			if v, ok := tagsMap[ft.name]; ok {
				dst = appendAnyTagValues(interner, ft.id, v, dst)
			}
		}
	}
	return dst
}

func appendJSONTagValues(
	interner *TagInterner,
	tagID TagID,
	raw json.RawMessage,
	dst []BusTagValue,
) []BusTagValue {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return dst
	}
	switch trimmed[0] {
	case '"':
		var val string
		if err := json.Unmarshal(trimmed, &val); err != nil {
			return dst
		}
		return appendInternedTagValue(interner, tagID, val, dst)
	case '[':
		var vals []string
		if err := json.Unmarshal(trimmed, &vals); err != nil {
			return dst
		}
		for _, val := range vals {
			dst = appendInternedTagValue(interner, tagID, val, dst)
		}
		return dst
	default:
		return dst
	}
}

func appendAnyTagValues(
	interner *TagInterner,
	tagID TagID,
	val any,
	dst []BusTagValue,
) []BusTagValue {
	switch v := val.(type) {
	case string:
		return appendInternedTagValue(interner, tagID, v, dst)
	case []string:
		for _, s := range v {
			dst = appendInternedTagValue(interner, tagID, s, dst)
		}
		return dst
	case []any:
		for _, elem := range v {
			if s, ok := elem.(string); ok {
				dst = appendInternedTagValue(interner, tagID, s, dst)
			}
		}
		return dst
	default:
		return dst
	}
}

func appendInternedTagValue(
	interner *TagInterner,
	tagID TagID,
	val string,
	dst []BusTagValue,
) []BusTagValue {
	if val == "" {
		return dst
	}
	valID, ok := interner.LookupValue(val)
	if !ok {
		return dst
	}
	return append(dst, BusTagValue{Tag: tagID, Value: valID})
}

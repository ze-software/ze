// Design: docs/architecture/api/architecture.md -- interned tag identifiers and zero-alloc tag matchers for bus delivery
// Related: bus_filter.go -- compiled bus filters and zero-alloc delivery index consuming BusTagMatcher

package server

import (
	"slices"
	"sync"
)

// Bounds on TagInterner tables so runtime lookups and registrations cannot
// grow unboundedly.
const (
	MaxInternedTags      = 4096
	MaxInternedTagValues = 65535
)

// Unknown sentinel IDs returned when a tag or tag value has not been interned
// or when an intern table reaches its declared capacity bound.
const (
	TagIDUnknown      TagID      = 0
	TagValueIDUnknown TagValueID = 0
)

// TagID is a compact interned identifier for a bus message tag name.
// Zero (TagIDUnknown) indicates an uninterned or unknown tag name.
type TagID uint32

// TagValueID is a compact interned identifier for a bus message tag value.
// Zero (TagValueIDUnknown) indicates an uninterned or unknown tag value.
type TagValueID uint32

// BusTagValue pairs one interned tag name ID with one interned tag value ID.
// It is a comparable value type carried in slices on the delivery hot path
// with zero heap allocations.
type BusTagValue struct {
	Tag   TagID
	Value TagValueID
}

// TagInterner maps bus tag names and tag values to compact uint32 identifiers
// for allocation-free matching on the message delivery hot path.
// Safe for concurrent use.
type TagInterner struct {
	mu      sync.RWMutex
	tagToID map[string]TagID
	idToTag []string
	valToID map[string]TagValueID
	idToVal []string
}

// NewTagInterner allocates a new TagInterner with index 0 reserved for the
// unknown sentinels TagIDUnknown and TagValueIDUnknown.
func NewTagInterner() *TagInterner {
	return &TagInterner{
		tagToID: make(map[string]TagID),
		idToTag: []string{""},
		valToID: make(map[string]TagValueID),
		idToVal: []string{""},
	}
}

// InternTag returns the TagID for name, assigning a new ID if name is not yet
// present and capacity remains within MaxInternedTags. Empty names and entries
// beyond MaxInternedTags return TagIDUnknown. Safe for concurrent use.
func (ti *TagInterner) InternTag(name string) TagID {
	if ti == nil {
		return TagIDUnknown
	}
	if name == "" {
		return TagIDUnknown
	}
	ti.mu.RLock()
	id, ok := ti.tagToID[name]
	ti.mu.RUnlock()
	if ok {
		return id
	}

	ti.mu.Lock()
	defer ti.mu.Unlock()
	if existing, found := ti.tagToID[name]; found {
		return existing
	}
	if len(ti.idToTag)-1 >= MaxInternedTags {
		return TagIDUnknown
	}
	assigned := TagID(len(ti.idToTag))
	ti.idToTag = append(ti.idToTag, name)
	ti.tagToID[name] = assigned
	return assigned
}

// LookupTag returns the interned TagID for name without mutating the table.
// Use LookupTag for untrusted or runtime-supplied names so lookup never grows
// the table. Safe for concurrent use.
func (ti *TagInterner) LookupTag(name string) (TagID, bool) {
	if ti == nil {
		return TagIDUnknown, false
	}
	if name == "" {
		return TagIDUnknown, false
	}
	ti.mu.RLock()
	id, ok := ti.tagToID[name]
	ti.mu.RUnlock()
	return id, ok
}

// TagName returns the string name for id, or ok=false if id is unknown.
// Safe for concurrent use.
func (ti *TagInterner) TagName(id TagID) (string, bool) {
	if ti == nil {
		return "", false
	}
	if id == TagIDUnknown {
		return "", false
	}
	ti.mu.RLock()
	defer ti.mu.RUnlock()
	idx := int(id)
	if idx >= len(ti.idToTag) {
		return "", false
	}
	return ti.idToTag[idx], true
}

// InternValue returns the TagValueID for val, assigning a new ID if val is not
// yet present and capacity remains within MaxInternedTagValues. Empty values
// and entries beyond MaxInternedTagValues return TagValueIDUnknown.
// Safe for concurrent use.
func (ti *TagInterner) InternValue(val string) TagValueID {
	if ti == nil {
		return TagValueIDUnknown
	}
	if val == "" {
		return TagValueIDUnknown
	}
	ti.mu.RLock()
	id, ok := ti.valToID[val]
	ti.mu.RUnlock()
	if ok {
		return id
	}

	ti.mu.Lock()
	defer ti.mu.Unlock()
	if existing, found := ti.valToID[val]; found {
		return existing
	}
	if len(ti.idToVal)-1 >= MaxInternedTagValues {
		return TagValueIDUnknown
	}
	assigned := TagValueID(len(ti.idToVal))
	ti.idToVal = append(ti.idToVal, val)
	ti.valToID[val] = assigned
	return assigned
}

// LookupValue returns the interned TagValueID for val without mutating the
// table. Safe for concurrent use.
func (ti *TagInterner) LookupValue(val string) (TagValueID, bool) {
	if ti == nil {
		return TagValueIDUnknown, false
	}
	if val == "" {
		return TagValueIDUnknown, false
	}
	ti.mu.RLock()
	id, ok := ti.valToID[val]
	ti.mu.RUnlock()
	return id, ok
}

// ValueName returns the string value for id, or ok=false if id is unknown.
// Safe for concurrent use.
func (ti *TagInterner) ValueName(id TagValueID) (string, bool) {
	if ti == nil {
		return "", false
	}
	if id == TagValueIDUnknown {
		return "", false
	}
	ti.mu.RLock()
	defer ti.mu.RUnlock()
	idx := int(id)
	if idx >= len(ti.idToVal) {
		return "", false
	}
	return ti.idToVal[idx], true
}

// InternTagValue interns both tag and value and returns the resulting
// BusTagValue pair. Safe for concurrent use.
func (ti *TagInterner) InternTagValue(tag, value string) BusTagValue {
	return BusTagValue{
		Tag:   ti.InternTag(tag),
		Value: ti.InternValue(value),
	}
}

// LookupTagValue resolves tag and value without mutating the table.
// Safe for concurrent use.
func (ti *TagInterner) LookupTagValue(tag, value string) (BusTagValue, bool) {
	tagID, ok := ti.LookupTag(tag)
	if !ok {
		return BusTagValue{}, false
	}
	valID, ok := ti.LookupValue(value)
	if !ok {
		return BusTagValue{}, false
	}
	return BusTagValue{Tag: tagID, Value: valID}, true
}

// busTagRule holds one interned tag name ID and the sorted, deduplicated slice
// of allowed interned value IDs for that tag.
type busTagRule struct {
	tag     TagID
	allowed []TagValueID
}

// BusTagMatcher is an immutable, precompiled tag predicate evaluated on the
// hot delivery path with zero heap allocations and no map lookups.
// Rules are sorted ascending by TagID; every rule must match for Matches to
// return true (logical AND across tags, logical OR across values of one tag).
// Safe for concurrent use.
type BusTagMatcher struct {
	matchNone bool
	rules     []busTagRule
}

// NewBusTagMatcher compiles a map of tag name -> allowed values into an
// immutable BusTagMatcher using interner. Returns nil when tagValues is empty
// so unfiltered receivers carry no matcher and perform zero tag comparisons.
// If any tag key is present with an empty allowed slice, the returned matcher
// rejects every message (unsatisfiable filter).
func NewBusTagMatcher(interner *TagInterner, tagValues map[string][]string) *BusTagMatcher {
	if len(tagValues) == 0 {
		return nil
	}
	if interner == nil {
		return &BusTagMatcher{matchNone: true}
	}

	rules := make([]busTagRule, 0, len(tagValues))
	for tagName, vals := range tagValues {
		if len(vals) == 0 {
			return &BusTagMatcher{matchNone: true}
		}
		tagID := interner.InternTag(tagName)
		if tagID == TagIDUnknown {
			return &BusTagMatcher{matchNone: true}
		}
		allowed := make([]TagValueID, 0, len(vals))
		for _, v := range vals {
			valID := interner.InternValue(v)
			if valID == TagValueIDUnknown {
				continue
			}
			allowed = append(allowed, valID)
		}
		if len(allowed) == 0 {
			return &BusTagMatcher{matchNone: true}
		}
		slices.Sort(allowed)
		allowed = slices.Compact(allowed)
		rules = append(rules, busTagRule{
			tag:     tagID,
			allowed: allowed,
		})
	}
	slices.SortFunc(rules, func(a, b busTagRule) int {
		if a.tag < b.tag {
			return -1
		}
		if a.tag > b.tag {
			return 1
		}
		return 0
	})
	return &BusTagMatcher{rules: rules}
}

// Matches reports whether tags satisfies every rule in m.
// When m is nil or has no rules (and is not unsatisfiable), Matches returns
// true immediately without reading tags. Allocates nothing.
func (m *BusTagMatcher) Matches(tags []BusTagValue) bool {
	if m == nil {
		return true
	}
	if m.matchNone {
		return false
	}
	if len(m.rules) == 0 {
		return true
	}
	for i := range m.rules {
		if !busTagRuleMatches(&m.rules[i], tags) {
			return false
		}
	}
	return true
}

// IsEmpty reports whether m imposes no tag restriction (nil or empty rule set).
func (m *BusTagMatcher) IsEmpty() bool {
	if m == nil {
		return true
	}
	if m.matchNone {
		return false
	}
	return len(m.rules) == 0
}

// IsUnsatisfiable reports whether m rejects every message.
func (m *BusTagMatcher) IsUnsatisfiable() bool {
	if m == nil {
		return false
	}
	return m.matchNone
}

// RuleCount returns the number of compiled tag rules held by m.
func (m *BusTagMatcher) RuleCount() int {
	if m == nil {
		return 0
	}
	return len(m.rules)
}

func busTagRuleMatches(rule *busTagRule, tags []BusTagValue) bool {
	for i := range tags {
		if tags[i].Tag != rule.tag {
			continue
		}
		if busTagValueAllowed(rule.allowed, tags[i].Value) {
			return true
		}
	}
	return false
}

func busTagValueAllowed(allowed []TagValueID, value TagValueID) bool {
	for i := range allowed {
		if allowed[i] == value {
			return true
		}
	}
	return false
}

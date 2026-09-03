// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn filter plugin
// Detail: match.go -- the AS_PATH scan that judges against the vocabulary below
// Related: filter_path_asn.go -- SDK entry point and handleFilterUpdate
//
// The position vocabulary and the config parse for the bgp-filter-path-asn
// plugin.
//
// It reads named reject-asn lists out of the BGP config subtree:
//
//	bgp { policy { reject-asn NAME { indirect [ N N ]; nth 2 [ N ]; } } }
//
// Each list resolves to one map from ASN to the union of the primitive
// positions the leaf-lists it appears under expand to, one set of (position,
// ASN) pairs for the `nth` entries, and the patterns of its regex leaf-list
// compiled at load. The union is what makes an ASN written under two keywords
// one entry rather than two (AC-17), and it is what the hot path reads.
//
// The schema makes most of the refusals an operator meets. Each ASN leaf-list is
// `type uint32`, `regex` is `type string` and an `nth` index is a uint8 bounded
// 1..255, so a word written where an ASN belongs and a keyword nobody declared
// are both refused by config.ParseTreeWithYANG before this runs.
//
// One refusal is owed HERE, because no schema can make it: a list that names
// nothing at all. An absent leaf-list has no node to walk, so nothing in the
// YANG can see the fault, and the list would accept every route while reading
// like a safety filter (AC-15).
//
// This parse also refuses a pattern that does not compile (AC-46) and one over
// the 512-character cap (AC-47), neither of which the schema can judge. Each
// message names the list and what is wrong.
package filter_path_asn

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/core/configvalue"
)

const (
	// maxPatternLen bounds one pattern under `regex`. It is the bound
	// bgp-filter-aspath applies to an entry regex, for the same reason: RE2 is
	// linear in the subject and does not backtrack, so the cap is defense in
	// depth rather than a guard against a pathological pattern.
	maxPatternLen = 512

	// maxNameLen bounds a reject-asn list name, matching the YANG length.
	maxNameLen = 256

	// positionKeyRegex names the one leaf-list of a reject-asn list whose values
	// are patterns rather than ASNs. No position applies to them, which is why it
	// sits beside the position leaves instead of modifying them.
	positionKeyRegex = "regex"

	// positionKeyNth names the one keyed list of a reject-asn list. Its key is a
	// number, which a leaf-list name cannot carry, so it is the one keyword that
	// is not a plain leaf-list.
	positionKeyNth = "nth"

	// positionKeyPasteable is the keyword `show bgp reject-asn known
	// transit-free` writes its ASNs under. It is a key of positionsByKey, checked
	// by TestPasteableLeafIsInTheVocabulary.
	positionKeyPasteable = "indirect"

	// nthIndexMax is the largest collapsed position an `nth` entry can name,
	// matching the uint8 range the YANG declares. A path longer than this cannot
	// be matched by any nth rule, which the scan reads as a non-match.
	nthIndexMax = 255
)

// position is one primitive place an ASN occupies in an AS_PATH, and the place
// a reject log line names.
//
// positionUnspecified is the Go zero value and is never a place, so a position
// nobody wrote cannot read as a valid one (ai/rules/principles.md).
type position uint8

const (
	positionUnspecified position = iota
	positionDirect
	positionTransit
	positionOrigin
	// positionNth is the property that cuts ACROSS the three above: a token at
	// collapsed position n holds it as well as whichever of direct, transit and
	// origin it occupies. It is never a member of a positionSet, because the set
	// expands the keywords that name a PARTITION of the path. It exists so a
	// reject caught by an nth rule says so in the log and in the counter.
	positionNth
)

// String names the position with the word the YANG leaf-list uses, so a log
// line and the config file say the same thing.
//
// The zero value and any value outside the enum both answer "unspecified",
// because neither is a place and a reader is owed the same word for both.
func (p position) String() string {
	switch p {
	case positionDirect:
		return "direct"
	case positionTransit:
		return "transit"
	case positionOrigin:
		return "origin"
	case positionNth:
		return positionKeyNth
	case positionUnspecified:
		return "unspecified"
	}
	return "unspecified"
}

// positionSet is the set of primitive positions one leaf-list name covers, and
// the value stored against each listed ASN. Two leaf-lists naming one ASN union
// into one set.
type positionSet uint8

const (
	setDirect  = positionSet(1) << positionDirect
	setTransit = positionSet(1) << positionTransit
	setOrigin  = positionSet(1) << positionOrigin
)

// holds reports whether the set rejects an ASN found at p.
func (s positionSet) holds(p position) bool {
	return s&(positionSet(1)<<p) != 0
}

// positionsByKey is the position vocabulary: the primitive set each position
// leaf-list of a reject-asn list expands to. It also DECIDES which leaf-lists
// parseOneList reads, so a leaf added to the YANG and not to this table is read
// by nobody.
//
// The table is the contract an operator writes against, which is why
// TestPositionKeyExpansion asserts the table itself rather than the matcher's
// behavior. An expansion that changed here would change every list in every
// config at once, and no test of the matcher would name it.
//
// `regex` and `nth` are deliberately absent, for two different reasons. A regex
// value is a pattern matched against the whole AS_PATH string, so no position
// applies to it. An nth index is a NUMBER, so it names a position this fixed
// bitmask cannot hold. parseOneList reads both on its own, after this table.
var positionsByKey = map[string]positionSet{
	"direct":   setDirect,
	"transit":  setTransit,
	"origin":   setOrigin,
	"indirect": setTransit | setOrigin,
	"anywhere": setDirect | setTransit | setOrigin,
}

// rejectList is one resolved reject-asn list, ready for the hot path.
//
// A list is an unordered reject SET. A route is rejected when positions matches
// or when any pattern matches, with no ordering between the two arms and no
// first-match-wins inside either. That is what keeps the type different from
// as-path-list, whose entries are ordered and carry their own action.
type rejectList struct {
	// name is the list name an operator wrote and a filter chain references.
	name string
	// positions maps each listed ASN to the union of the primitive positions the
	// list rejects it at. It is read for every UPDATE and never written after
	// the parse returns.
	positions map[uint32]positionSet
	// nth holds one entry per (collapsed position, ASN) pair the operator wrote
	// under an `nth` keyword. The pair is the map KEY rather than a map of maps,
	// so a token costs one lookup and no allocation. Empty for a list that names
	// no nth rule, and the scan skips the lookup entirely then.
	nth map[nthKey]struct{}
	// patterns are the compiled patterns of the regex leaf-list, in the order the
	// operator wrote them. Empty for a list that names no pattern, which is the
	// case the zero-allocation guarantee covers.
	patterns []*regexp.Regexp
}

// nthKey is one ASN rejected at one collapsed position of the AS_PATH.
type nthKey struct {
	// index is the collapsed position, counted from us and 1-based.
	index uint8
	// asn is the ASN the operator listed at that position.
	asn uint32
}

// parseRejectASNLists walks bgp { policy { reject-asn ... } } and returns each
// list resolved by name, ready for the hot path.
//
// A tree with no policy block and a policy block with no reject-asn list both
// answer an empty map: an operator who configured no list has configured no
// list, and a chain naming one that does not exist is refused elsewhere by
// ValidateFilterNames and fails closed here in handleFilterUpdate.
func parseRejectASNLists(bgpCfg map[string]any) (map[string]*rejectList, error) {
	lists := make(map[string]*rejectList)

	policyBlock, ok := bgpCfg["policy"].(map[string]any)
	if !ok {
		return lists, nil
	}
	rejectBlock, ok := policyBlock["reject-asn"].(map[string]any)
	if !ok {
		return lists, nil
	}

	for name, body := range rejectBlock {
		list, err := parseOneList(name, body)
		if err != nil {
			return nil, err
		}
		lists[name] = list
	}
	return lists, nil
}

// parseOneList resolves one named list.
//
// The position leaf-lists are read in sorted order, derived from the vocabulary
// table itself, so a list carrying two faults is refused by the same one on
// every run and a message never depends on Go's map iteration.
func parseOneList(name string, body any) (*rejectList, error) {
	if len(name) > maxNameLen {
		return nil, fmt.Errorf("reject-asn %q: name is longer than %d characters", name, maxNameLen)
	}
	listMap, ok := body.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("reject-asn %q: not a config block", name)
	}

	list := &rejectList{name: name, positions: make(map[uint32]positionSet)}

	for _, where := range slices.Sorted(maps.Keys(positionsByKey)) {
		values := configvalue.LeafList(listMap[where])
		if len(values) == 0 {
			continue
		}
		if err := list.addASNs(where, positionsByKey[where], values); err != nil {
			return nil, err
		}
	}
	for _, entry := range configvalue.ListEntries(listMap[positionKeyNth]) {
		if err := list.addNth(entry); err != nil {
			return nil, err
		}
	}
	if patterns := configvalue.LeafList(listMap[positionKeyRegex]); len(patterns) > 0 {
		if err := list.addPatterns(patterns); err != nil {
			return nil, err
		}
	}

	// AC-15. An empty list is the guard-shaped zero ai/rules/principles.md
	// forbids: it rejects nothing while reading like a safety filter, and the
	// schema cannot refuse it because an absent leaf-list has no node to walk.
	// A leaf-list written empty (`indirect [ ];`) reaches here as no members at
	// all, so it is the same refusal rather than a second one.
	if len(list.positions) == 0 && len(list.nth) == 0 && len(list.patterns) == 0 {
		return nil, fmt.Errorf(
			"reject-asn %q: no ASN and no pattern: a list that names nothing rejects nothing, "+
				"so it accepts every route while reading like a safety filter", name)
	}
	return list, nil
}

// addASNs unions one leaf-list's ASNs into the list under positions.
//
// The leaf-list is `type uint32`, so the config parser refuses a non-numeric
// value before this runs. The error stays because the alternative is to skip the
// value, which would drop an ASN the operator wrote and leave a filter that
// looks configured (ai/rules/principles.md).
func (l *rejectList) addASNs(where string, positions positionSet, values []string) error {
	for _, value := range values {
		asn, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("reject-asn %q: %s expects an ASN in 0-4294967295, got %q",
				l.name, where, value)
		}
		l.positions[uint32(asn)] |= positions
	}
	return nil
}

// addNth reads one `nth <index> [ asn ... ]` entry into the list.
//
// The index arrives as the entry KEY, because configvalue.ListEntries does not
// repeat a list key inside the entry. AC-16: an entry with no ASN is refused,
// for the same reason AC-15 refuses an empty list one level up.
func (l *rejectList) addNth(entry configvalue.ListEntry) error {
	index, err := strconv.ParseUint(entry.Key, 10, 8)
	if err != nil || index < 1 {
		// The YANG declares uint8 range 1..255 on the key, so the config parser
		// refuses anything else before this runs. The error stays rather than a
		// skip, because a skipped entry is a rule the operator wrote and Ze never
		// applied (ai/rules/principles.md).
		return fmt.Errorf("reject-asn %q: nth expects a position in 1-%d, got %q",
			l.name, nthIndexMax, entry.Key)
	}

	values := configvalue.LeafList(entry.Fields["asn"])
	if len(values) == 0 {
		return fmt.Errorf(
			"reject-asn %q: nth %s holds no asn: a position that names no ASN rejects nothing, "+
				"so it accepts every route while reading like a safety filter", l.name, entry.Key)
	}

	if l.nth == nil {
		l.nth = make(map[nthKey]struct{}, len(values))
	}
	for _, value := range values {
		asn, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("reject-asn %q: nth %s expects an ASN in 0-4294967295, got %q",
				l.name, entry.Key, value)
		}
		l.nth[nthKey{index: uint8(index), asn: uint32(asn)}] = struct{}{}
	}
	return nil
}

// addPatterns compiles the `regex` leaf-list's patterns.
//
// The over-long pattern is named by its length rather than quoted, because a
// message carrying 512 characters of pattern buries the sentence that says what
// to do about it.
func (l *rejectList) addPatterns(values []string) error {
	for _, value := range values {
		if len(value) > maxPatternLen {
			return fmt.Errorf("reject-asn %q: regex pattern is %d characters, longer than %d",
				l.name, len(value), maxPatternLen)
		}
		compiled, err := regexp.Compile(value)
		if err != nil {
			return fmt.Errorf("reject-asn %q: regex pattern %q does not compile: %w",
				l.name, value, err)
		}
		l.patterns = append(l.patterns, compiled)
	}
	return nil
}

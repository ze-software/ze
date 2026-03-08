// Design: docs/architecture/api/text-format.md — shared keyword definitions for text protocol
// Related: scanner.go — shared text tokenizer
// Related: ../format/text.go — message formatting uses keyword constants

package textparse

// Canonical keyword names (long forms used internally).
const (
	// Attribute keywords — parsed as flat key-value pairs in commands,
	// recognized as section boundaries in event parsing.
	KWOrigin            = "origin"
	KWASPath            = "as-path"
	KWMED               = "med"
	KWLocalPreference   = "local-preference"
	KWAtomicAggregate   = "atomic-aggregate"
	KWAggregator        = "aggregator"
	KWOriginatorID      = "originator-id"
	KWClusterList       = "cluster-list"
	KWCommunity         = "community"
	KWLargeCommunity    = "large-community"
	KWExtendedCommunity = "extended-community"
	KWNextHop           = "next-hop"

	// Structure keywords.
	KWNLRI            = "nlri"
	KWPathInformation = "path-information"
	KWRD              = "rd"
	KWLabel           = "label"
	KWWatchdog        = "watchdog"
	KWAttr            = "attr"

	// Action keywords (NLRI-only: MP_REACH vs MP_UNREACH).
	KWAdd = "add"
	KWDel = "del"
	KWEOR = "eor"

	// Value keywords.
	KWSelf = "self"
)

// Short forms for API output (compact wire format).
// These are the forms printed by the event formatter.
const (
	ShortNext = "next"
	ShortPref = "pref"
	ShortPath = "path"
	ShortSCom = "s-com"
	ShortLCom = "l-com"
	ShortXCom = "x-com"
	ShortInfo = "info"
	ShortRD   = "rd" // rd is already the short form
)

// aliasToCanonical maps all accepted keyword forms to their canonical (long) name.
// Every keyword that has a short form or alternate spelling is listed here.
// Keywords without aliases (origin, med, label, nlri, etc.) resolve to themselves.
var aliasToCanonical = map[string]string{
	// next-hop aliases.
	ShortNext: KWNextHop,
	KWNextHop: KWNextHop,
	"nhop":    KWNextHop, // legacy, still accepted

	// local-preference aliases.
	ShortPref:         KWLocalPreference,
	KWLocalPreference: KWLocalPreference,

	// as-path aliases.
	ShortPath: KWASPath,
	KWASPath:  KWASPath,

	// community aliases (x-com pattern).
	ShortSCom:         KWCommunity,
	KWCommunity:       KWCommunity,
	"short-community": KWCommunity,

	// large-community aliases.
	ShortLCom:        KWLargeCommunity,
	KWLargeCommunity: KWLargeCommunity,

	// extended-community aliases.
	ShortXCom:           KWExtendedCommunity,
	"e-com":             KWExtendedCommunity, // also accepted
	KWExtendedCommunity: KWExtendedCommunity,

	// path-information aliases.
	ShortInfo:         KWPathInformation,
	KWPathInformation: KWPathInformation,

	// route-distinguisher aliases.
	ShortRD:               KWRD,
	"route-distinguisher": KWRD,
}

// canonicalToShort maps canonical names to their short (API) form.
var canonicalToShort = map[string]string{
	KWNextHop:           ShortNext,
	KWLocalPreference:   ShortPref,
	KWASPath:            ShortPath,
	KWCommunity:         ShortSCom,
	KWLargeCommunity:    ShortLCom,
	KWExtendedCommunity: ShortXCom,
	KWPathInformation:   ShortInfo,
	KWRD:                ShortRD,
}

// ResolveAlias returns the canonical keyword name for any accepted form.
// If the token is not a known alias, returns the token unchanged.
func ResolveAlias(token string) string {
	if canonical, ok := aliasToCanonical[token]; ok {
		return canonical
	}
	return token
}

// ShortForm returns the short (API) form of a canonical keyword.
// If no short form exists, returns the canonical name unchanged.
func ShortForm(canonical string) string {
	if short, ok := canonicalToShort[canonical]; ok {
		return short
	}
	return canonical
}

// LongForm returns the canonical (long) form for display.
// This is identity for canonical names; for aliases it resolves first.
func LongForm(token string) string {
	return ResolveAlias(token)
}

// attributeKeywords are keywords that introduce an attribute value in commands
// and act as section boundaries in event parsing.
var attributeKeywords = map[string]bool{
	KWOrigin:            true,
	KWASPath:            true,
	KWMED:               true,
	KWLocalPreference:   true,
	KWAtomicAggregate:   true,
	KWAggregator:        true,
	KWOriginatorID:      true,
	KWClusterList:       true,
	KWCommunity:         true,
	KWLargeCommunity:    true,
	KWExtendedCommunity: true,
	KWNextHop:           true,
}

// topLevelKeywords are all keywords that stop NLRI token collection in event parsing.
// This is the union of attribute keywords plus structural keywords.
var topLevelKeywords = map[string]bool{
	KWOrigin:            true,
	KWASPath:            true,
	KWMED:               true,
	KWLocalPreference:   true,
	KWAtomicAggregate:   true,
	KWAggregator:        true,
	KWOriginatorID:      true,
	KWClusterList:       true,
	KWCommunity:         true,
	KWLargeCommunity:    true,
	KWExtendedCommunity: true,
	KWNextHop:           true,
	KWNLRI:              true,
}

// IsAttributeKeyword returns true if the token is an attribute keyword.
// Resolves aliases internally — accepts both short and long forms.
func IsAttributeKeyword(token string) bool {
	return attributeKeywords[ResolveAlias(token)]
}

// IsTopLevelKeyword returns true if the token stops NLRI collection.
// Resolves aliases internally — accepts both short and long forms.
func IsTopLevelKeyword(token string) bool {
	return topLevelKeywords[ResolveAlias(token)]
}

// NLRITypeKeywords are NLRI type keywords that start a new NLRI entry.
// Used for keyword-boundary format: "nlri add prefix <a> prefix <b>".
var NLRITypeKeywords = map[string]bool{
	"prefix":           true,
	"rd":               true,
	"reachability":     true,
	"node":             true,
	"link":             true,
	"srv6-sid":         true,
	"ethernet-ad":      true,
	"mac-ip":           true,
	"multicast":        true,
	"ethernet-segment": true,
	"ip-prefix":        true,
	"flow":             true,
	"flow-vpn":         true,
}

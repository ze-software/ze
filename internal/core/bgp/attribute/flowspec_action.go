// Design: docs/architecture/wire/attributes.md -- FlowSpec traffic-action extended communities
// RFC: rfc/short/rfc8955.md -- traffic filtering actions (Section 7)
// Related: community.go -- ExtendedCommunity, the 8-octet value these produce

package attribute

// FlowSpec traffic actions that are written as ONE word, with no colon.
//
// Every other extended community carries its value after a colon
// (`target:65000:1`, `rate-limit:1000`), so a parser can find the type by
// splitting on the first one. These three carry their whole meaning in the name,
// and a parser that splits first rejects them as malformed rather than as
// unknown -- which is exactly what happened: the config path understood
// `copy-to-nexthop` while the `update text` API path answered "invalid extended
// community format", so a route an operator could write in config could not be
// expressed through the API.
//
// One table, consumed by both parsers, so the two vocabularies cannot drift
// again (config/routeattr_community.go parseOneExtCommunity,
// route/route_community.go parseExtendedCommunity).
var flowSpecActionKeywords = map[string]ExtendedCommunity{
	// Pre-IETF draft: redirect to next-hop. Type 0x08, subtype 0x00, value 0.
	"redirect-to-nexthop-draft": {0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	// RFC 5575bis: copy and redirect to next-hop. Same type/subtype, value 1 --
	// the low bit is the copy semantic, which is why these two differ by one byte.
	"copy-to-nexthop": {0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
	// RFC 8955 Section 7.3: traffic-rate 0 means discard. Type 0x80, subtype 0x06.
	"discard": {0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
}

// FlowSpecActionKeyword returns the extended community for a colon-less FlowSpec
// action keyword, and whether the name is one.
//
// Callers must try this BEFORE splitting on ':', because these names contain no
// colon and a split-first parser reports them as a malformed format rather than
// as an unrecognized action.
func FlowSpecActionKeyword(name string) (ExtendedCommunity, bool) {
	ec, ok := flowSpecActionKeywords[name]
	return ec, ok
}

// FlowSpecActionKeywords returns the accepted keywords, sorted, for an error
// message that tells the operator what IS accepted rather than only that their
// input was not (ai/rules/cli.md). Derived from the table, so a new
// keyword cannot be added without the diagnostic naming it
// (ai/rules/evidence.md).
func FlowSpecActionKeywords() []string {
	names := make([]string, 0, len(flowSpecActionKeywords))
	for name := range flowSpecActionKeywords {
		names = append(names, name)
	}
	sortStrings(names)
	return names
}

// sortStrings is an insertion sort over a 3-element table: pulling in "sort"
// here would be the only import in this file, for a slice this size.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

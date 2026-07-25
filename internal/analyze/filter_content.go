// Design: docs/architecture/mrt.md -- AS-path and community regex filtering

package analyze

import (
	"regexp"

	"github.com/ze-software/ze/internal/mrt"
)

// matchMessageContent parses the BGP message in an MRT BGP4MP record and checks
// AS-path/community regex filters against its UPDATE attributes.
func matchMessageContent(m *mrt.MessageRecord, as4 bool, opts *filterOpts) bool {
	parsed, err := mrt.ParseBGPMessage(m.BGPMessage)
	if err != nil || parsed.Update == nil {
		return false
	}
	return matchParsedAttrs(parsed.Update.Attributes, as4, opts)
}

// matchRIBContent checks AS-path/community regex against any RIB entry's attributes.
// TABLE_DUMP_V2 always uses 4-byte ASNs.
func matchRIBContent(r *mrt.RIBRecord, opts *filterOpts) bool {
	for i := range r.Entries {
		if matchAttrsContent(r.Entries[i].Attributes, true, opts) {
			return true
		}
	}
	return false
}

// matchAttrsContent parses raw attribute bytes and checks regex filters.
func matchAttrsContent(rawAttrs []byte, as4 bool, opts *filterOpts) bool {
	attrs := mrt.ParseAttributes(rawAttrs)
	return matchParsedAttrs(attrs, as4, opts)
}

func matchParsedAttrs(attrs []mrt.PathAttribute, as4 bool, opts *filterOpts) bool {
	if opts.asPathRe != nil {
		if !matchASPath(attrs, as4, opts.asPathRe) {
			return false
		}
	}
	if opts.communityRe != nil {
		if !mrt.MatchCommunityRegex(attrs, opts.communityRe.MatchString) {
			return false
		}
	}
	return true
}

func matchASPath(attrs []mrt.PathAttribute, as4 bool, re *regexp.Regexp) bool {
	a := mrt.FindAttribute(attrs, 2)
	if a == nil {
		return false
	}
	segments, err := mrt.ParseASPath(a.Value, as4)
	if err != nil {
		return false
	}
	s := mrt.FormatASPath(segments)
	return re.MatchString(s)
}

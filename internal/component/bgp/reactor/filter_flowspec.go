// Design: docs/architecture/core-design.md -- prefix policy on FlowSpec NLRIs
// RFC: rfc/short/rfc8955.md -- Sections 4 and 4.2.1
// RFC: rfc/short/rfc8956.md -- Section 3.1
package reactor

import (
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// A destination-only projection lets the existing prefix-list policy apply its
// prefix/ge/le rules without discarding the rest of an accepted FlowSpec NLRI.
// An absent or nonzero-offset destination has no CIDR equivalent and is denied.
func appendFlowSpecFilterBlock(buf []byte, fam family.Family, op string, raw []byte, addPath, empty bool) []byte {
	buf = appendMPBlock(buf, fam, op, nil, addPath, empty)
	_, err := nlrisplit.SplitFlowSpec(raw, addPath, func(one []byte) bool {
		if addPath {
			one = one[4:]
		}
		destination := nlrisplit.FlowSpecDestination(fam, one)
		buf = append(buf, ' ')
		if !destination.IsValid() {
			buf = append(buf, "invalid"...)
			return true
		}
		buf = destination.AppendTo(buf)
		return true
	})
	if err != nil {
		buf = append(buf, " invalid"...)
	}
	return buf
}

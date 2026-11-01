// Design: docs/architecture/core-design.md -- the BGP route view
// Related: readers.go -- reconstructedPath, the AS path the text is built from
//
// The AS path as text, for a match condition that judges it as a string: an
// as-path-list's regular expressions are written against the flat list of
// decimal ASNs the filter text format carries.

package policyview

import (
	"github.com/ze-software/ze/internal/core/bgp/asn"
)

// sharedText caches the AS path text for one run, in a buffer the view keeps
// across runs, so a run allocates nothing once the buffer has grown to the
// longest path the view met. An AS_PATH attribute is at most 65535 octets, so
// the buffer is bounded by the message size.
type sharedText struct {
	run  uint64
	text []byte
	err  error
}

// ASPathText answers the reconstructed AS path as the filter text format's
// as-path value reads (filtertext.ASPath): every ASN in plain decimal, in
// path order, one space between two, with no segment marker, so an AS_SET's
// members read as sequence members do. An UPDATE with no AS path, or a path
// with no ASN, answers an empty text. It is built once per run however many
// conditions read it; the bytes are the view's own and MUST NOT be kept past
// the run. A malformed AS_PATH answers the error the as-path field answers.
func (v *View) ASPathText() ([]byte, error) {
	if v.update == nil {
		return nil, ErrNoUpdate
	}
	if v.asPathText.run == v.run {
		return v.asPathText.text, v.asPathText.err
	}
	path, _, err := v.reconstructedPath()
	text := v.asPathText.text[:0]
	if err == nil && path != nil {
		for _, segment := range path.Segments {
			for _, number := range segment.ASNs {
				if len(text) > 0 {
					text = append(text, ' ')
				}
				text = asn.Append(text, number, asn.NotationPlain)
			}
		}
	}
	v.asPathText = sharedText{run: v.run, text: text, err: err}
	return text, err
}

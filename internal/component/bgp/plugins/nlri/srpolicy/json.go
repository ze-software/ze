// Design: docs/architecture/wire/nlri.md -- SR-Policy in-process JSON writer
// RFC: rfc/short/rfc9830.md -- SR-Policy NLRI fields
//
// AppendJSON writes the SR-Policy NLRI's JSON representation directly into a
// caller-provided []byte, bypassing the wire-encode / hex / re-parse /
// map-marshal round-trip used by the RPC decoder path (DecodeNLRIHex).

package srpolicy

import (
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// AppendJSON satisfies nlri.JSONAppender.
// Keys alphabetical to match json.Marshal(map[string]any) output.
func (s *SRPolicy) AppendJSON(buf []byte) []byte {
	buf = append(buf, `{"color":`...)
	buf = strconv.AppendUint(buf, uint64(s.color), 10)
	buf = append(buf, `,"distinguisher":`...)
	buf = strconv.AppendUint(buf, uint64(s.distinguisher), 10)
	buf = append(buf, `,"endpoint":"`...)
	buf = textbuf.Addr(buf, s.endpoint)
	buf = append(buf, '"', '}')
	return buf
}

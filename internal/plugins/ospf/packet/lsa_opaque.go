// Design: plan/spec-ospf-2-wire.md -- opaque/unknown LSA passthrough
// RFC 5250 opaque types 9/10/11 are retained verbatim in v1.

package packet

import "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"

// OpaqueLSA is a raw body for OSPF opaque LSA types 9, 10, and 11.
type OpaqueLSA struct {
	Type types.LSType
	Data []byte
}

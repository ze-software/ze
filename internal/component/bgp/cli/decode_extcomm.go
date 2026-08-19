// Design: docs/architecture/core-design.md — BGP CLI commands
// Related: decode_update.go — path attribute parsing calls parseExtendedCommunities

package cli

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// parseExtendedCommunities renders each 8-octet extended community (RFC 4360
// Section 2) for `ze bgp decode` JSON: the raw 64-bit value, and the named form.
//
// The naming is attribute.ExtendedCommunity.AppendDecoded, the same renderer the
// receive path uses for the plugin event JSON, so the CLI and the event stream
// cannot answer differently about one community.
//
// A trailing run shorter than 8 octets is dropped rather than rendered: this
// decodes what a peer put on the wire, and half a community names nothing.
func parseExtendedCommunities(data []byte) []map[string]any {
	var comms []map[string]any
	var comm attribute.ExtendedCommunity
	var text [48]byte

	for len(data) >= 8 {
		copy(comm[:], data[:8])
		comms = append(comms, map[string]any{
			"value":  binary.BigEndian.Uint64(data[:8]),
			"string": string(comm.AppendDecoded(text[:0])),
		})
		data = data[8:]
	}

	return comms
}

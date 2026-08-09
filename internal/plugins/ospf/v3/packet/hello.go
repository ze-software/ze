// Design: docs/architecture/ospf/ospfv3-2-wire.md -- OSPFv3 Hello packet body codec.
// RFC: rfc/short/rfc5340.md (§A.3.2 Hello packet)

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/v3/types"

// helloFixedLen is the 20-octet fixed prefix before the neighbor list. RFC 5340
// §A.3.2 replaces the OSPFv2 network mask with a 32-bit Interface ID, widens
// Options to 24 bits, and narrows RouterDeadInterval to 16 bits.
const helloFixedLen = 20

// Hello body field offsets (RFC 5340 §A.3.2, body-relative).
const (
	helloInterfaceIDOff = 0
	helloPriorityOff    = 4
	helloOptionsOff     = 5
	helloHelloIntOff    = 8
	helloDeadIntOff     = 10
	helloDROff          = 12
	helloBDROff         = 16
)

// Hello is the OSPFv3 Hello packet body. There is no network mask: OSPFv3 runs
// per link and identifies the interface with a 32-bit Interface ID.
type Hello struct {
	InterfaceID        types.InterfaceID
	Priority           uint8
	Options            types.Options
	HelloInterval      uint16
	RouterDeadInterval uint16
	DR                 types.RouterID
	BDR                types.RouterID
	Neighbors          []types.RouterID
}

// DecodeHello parses a Hello body. The neighbor count is derived from the
// remaining body length, which must be a whole number of 4-octet Router IDs.
func DecodeHello(body []byte) (Hello, error) {
	if len(body) < helloFixedLen {
		return Hello{}, ErrTruncated
	}
	if (len(body)-helloFixedLen)%types.IDLen != 0 {
		return Hello{}, ErrLength
	}
	count := (len(body) - helloFixedLen) / types.IDLen
	iface, err := types.InterfaceIDFromBytes(body[helloInterfaceIDOff : helloInterfaceIDOff+types.IDLen])
	if err != nil {
		return Hello{}, err
	}
	opts, err := types.OptionsFromBytes(body, helloOptionsOff)
	if err != nil {
		return Hello{}, err
	}
	dr, err := types.RouterIDFromBytes(body[helloDROff : helloDROff+types.IDLen])
	if err != nil {
		return Hello{}, err
	}
	bdr, err := types.RouterIDFromBytes(body[helloBDROff : helloBDROff+types.IDLen])
	if err != nil {
		return Hello{}, err
	}
	out := Hello{
		InterfaceID:        iface,
		Priority:           body[helloPriorityOff],
		Options:            opts,
		HelloInterval:      readUint16(body, helloHelloIntOff),
		RouterDeadInterval: readUint16(body, helloDeadIntOff),
		DR:                 dr,
		BDR:                bdr,
		Neighbors:          make([]types.RouterID, 0, count),
	}
	off := helloFixedLen
	for range count {
		id, err := types.RouterIDFromBytes(body[off : off+types.IDLen])
		if err != nil {
			return Hello{}, err
		}
		out.Neighbors = append(out.Neighbors, id)
		off += types.IDLen
	}
	return out, nil
}

// EncodedLen returns the Hello body length.
func (h Hello) EncodedLen() int { return helloFixedLen + len(h.Neighbors)*types.IDLen }

// WriteTo serializes the Hello body into buf at off.
func (h Hello) WriteTo(buf []byte, off int) int {
	off += h.InterfaceID.WriteTo(buf, off)
	buf[off] = h.Priority
	off++
	off += h.Options.WriteTo(buf, off)
	off += writeUint16(buf, off, h.HelloInterval)
	off += writeUint16(buf, off, h.RouterDeadInterval)
	off += h.DR.WriteTo(buf, off)
	off += h.BDR.WriteTo(buf, off)
	for _, n := range h.Neighbors {
		off += n.WriteTo(buf, off)
	}
	return off
}

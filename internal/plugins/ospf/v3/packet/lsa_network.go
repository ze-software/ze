// Design: docs/architecture/ospf/ospfv3-2-wire.md -- OSPFv3 Network-LSA body codec.
// RFC: rfc/short/rfc5340.md (§A.4.4 Network-LSA)

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/v3/types"

const networkLSAFixedLen = 4

// NetworkLSA is the OSPFv3 Network-LSA body (RFC 5340 §A.4.4). Unlike OSPFv2 it
// carries NO network mask: the header's Link State ID is the Designated Router's
// Interface ID and is preserved verbatim by the common LSA header. The attached
// router count is derived from the LSA Length.
type NetworkLSA struct {
	Options         types.Options
	AttachedRouters []types.RouterID
}

// DecodeNetworkLSA parses a Network-LSA body. The attached-router count is
// derived from the remaining length, which must be a whole number of Router IDs.
func DecodeNetworkLSA(body []byte) (NetworkLSA, error) {
	if len(body) < networkLSAFixedLen {
		return NetworkLSA{}, ErrTruncated
	}
	if (len(body)-networkLSAFixedLen)%types.IDLen != 0 {
		return NetworkLSA{}, ErrLength
	}
	opts, err := types.OptionsFromBytes(body, 1)
	if err != nil {
		return NetworkLSA{}, err
	}
	count := (len(body) - networkLSAFixedLen) / types.IDLen
	out := NetworkLSA{Options: opts, AttachedRouters: make([]types.RouterID, 0, count)}
	off := networkLSAFixedLen
	for range count {
		router, err := types.RouterIDFromBytes(body[off : off+types.IDLen])
		if err != nil {
			return NetworkLSA{}, err
		}
		out.AttachedRouters = append(out.AttachedRouters, router)
		off += types.IDLen
	}
	return out, nil
}

// EncodedLen returns the Network-LSA body length.
func (l NetworkLSA) EncodedLen() int {
	return networkLSAFixedLen + len(l.AttachedRouters)*types.IDLen
}

// WriteTo serializes the Network-LSA body into buf at off. The leading reserved
// octet is written zero (RFC 5340 §A.4.4).
func (l NetworkLSA) WriteTo(buf []byte, off int) int {
	buf[off] = 0
	off++
	off += l.Options.WriteTo(buf, off)
	for _, router := range l.AttachedRouters {
		off += router.WriteTo(buf, off)
	}
	return off
}

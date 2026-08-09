// Design: docs/architecture/ospf/ospf-2-wire.md -- Network-LSA body codec
// RFC 2328 Appendix A.4.3: Network-LSA. The Link State ID is the DR interface
// address and is preserved by the common LSA header, not reinterpreted here.

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/types"

const networkLSAFixedLen = 4

// NetworkLSA is the Type 2 Network-LSA body.
type NetworkLSA struct {
	NetworkMask     [4]byte
	AttachedRouters []types.RouterID
}

// DecodeNetworkLSA parses a Network-LSA body.
func DecodeNetworkLSA(body []byte) (NetworkLSA, error) {
	if len(body) < networkLSAFixedLen {
		return NetworkLSA{}, ErrTruncated
	}
	if (len(body)-networkLSAFixedLen)%types.RouterIDLen != 0 {
		return NetworkLSA{}, ErrLength
	}
	count := (len(body) - networkLSAFixedLen) / types.RouterIDLen
	out := NetworkLSA{NetworkMask: readIPv4(body, 0), AttachedRouters: make([]types.RouterID, 0, count)}
	off := networkLSAFixedLen
	for range count {
		router, err := types.RouterIDFromBytes(body[off : off+types.RouterIDLen])
		if err != nil {
			return NetworkLSA{}, err
		}
		out.AttachedRouters = append(out.AttachedRouters, router)
		off += types.RouterIDLen
	}
	return out, nil
}

// EncodedLen returns the Network-LSA body length.
func (l NetworkLSA) EncodedLen() int {
	return networkLSAFixedLen + len(l.AttachedRouters)*types.RouterIDLen
}

// WriteTo serializes the Network-LSA body into buf at off.
func (l NetworkLSA) WriteTo(buf []byte, off int) int {
	off += writeIPv4(buf, off, l.NetworkMask)
	for _, router := range l.AttachedRouters {
		off += router.WriteTo(buf, off)
	}
	return off
}

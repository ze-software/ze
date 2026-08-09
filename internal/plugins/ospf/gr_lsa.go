// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- IPv4 (RFC 3623) Grace-LSA body glue.
// Related: packet/grace_lsa.go -- the ext-1 opaque TLV codec this wraps.
// RFC: rfc/short/rfc3623.md sec A -- the Grace-LSA is a Type-9 link-local Opaque LSA,
//
//	Opaque Type 3 / Opaque ID 0, body = Grace Period (type 1) + Restart Reason (type 2)
//	always, plus IP Interface Address (type 3) on broadcast/NBMA/P2MP segments.
package ospf

import (
	ospfiface "github.com/ze-software/ze/internal/plugins/ospf/iface"
	ospfpacket "github.com/ze-software/ze/internal/plugins/ospf/packet"
)

// grV4Body builds the RFC 3623 sec A IPv4 Grace-LSA body for one interface: the mandatory
// Grace Period (type 1) and Restart Reason (type 2) TLVs, plus the type-3 IP Interface
// Address TLV when the segment is shared media (broadcast/NBMA/P2MP) so the helper can bind
// the Grace-LSA to a neighbor by interface address (RFC 3623 sec 3.1). The bytes are the
// opaque body (after the 20-byte LSA header), which the ext-1 carrier installs and floods.
func grV4Body(gracePeriod uint32, reason uint8, ifaceAddr [4]byte, sharedMedia bool) []byte {
	g := ospfpacket.GraceLSA{
		GracePeriod:      gracePeriod,
		Reason:           reason,
		InterfaceAddr:    ifaceAddr,
		HasInterfaceAddr: sharedMedia,
	}
	return ospfpacket.EncodeGraceLSA(g)
}

// grV4Parse decodes a received IPv4 Grace-LSA opaque body. It fails when a mandatory TLV
// (Grace Period or Restart Reason) is missing, so a malformed Grace-LSA is rejected before
// the helper evaluates entry.
func grV4Parse(body []byte) (ospfpacket.GraceLSA, error) {
	return ospfpacket.DecodeGraceLSA(body)
}

// grSharedMedia reports whether an OSPF network type identifies a shared-media segment where
// RFC 3623 sec A requires the type-3 IP Interface Address TLV (broadcast, NBMA, or
// point-to-multipoint). Point-to-point and virtual links identify the neighbor by Router ID.
func grSharedMedia(networkType string) bool {
	switch networkType {
	case ospfiface.NetworkBroadcast, ospfiface.NetworkNBMA, ospfiface.NetworkPointToMultipoint:
		return true
	default:
		return false
	}
}

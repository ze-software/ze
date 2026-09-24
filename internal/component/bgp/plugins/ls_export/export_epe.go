// Design: docs/architecture/wire/nlri-bgpls.md -- EPE originator invariants
// RFC: rfc/short/rfc9086.md -- mandatory PeerNode SID and advertised SRGB

package ls_export

import (
	"encoding/binary"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	"github.com/ze-software/ze/internal/core/linkstateevents"
)

type epeNodeKey struct {
	asn      uint32
	routerID netip.Addr
}

func validateNativeEPE(snapshot *linkstateevents.Snapshot) error {
	if snapshot.Domain.Protocol != linkstateevents.BGP {
		return nil
	}
	blocks := make(map[epeNodeKey]uint64, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		if !validEPENode(node.ID) {
			return errors.New("native BGP node requires ASN and nonzero BGP Router-ID")
		}
		for _, attr := range node.Attributes {
			if attr.Type != ls.TLVSRCapabilities {
				continue
			}
			if len(attr.Value) < 12 {
				return errors.New("native EPE SRGB is empty")
			}
			var total uint64
			for value := attr.Value[2:]; len(value) != 0; {
				if len(value) < 10 || binary.BigEndian.Uint16(value[3:5]) != ls.TLVSIDLabel || binary.BigEndian.Uint16(value[5:7]) != 3 {
					return errors.New("native EPE SRGB requires complete label ranges")
				}
				size := uint32(value[0])<<16 | uint32(value[1])<<8 | uint32(value[2])
				base := uint32(value[7])<<16 | uint32(value[8])<<8 | uint32(value[9])
				if size == 0 || base < 16 || uint64(base)+uint64(size) > 1<<20 {
					return errors.New("native EPE SRGB label range invalid")
				}
				total += uint64(size)
				value = value[10:]
			}
			blocks[epeNodeKey{asn: node.ID.ASN, routerID: node.ID.BGPRouterID}] = total
		}
	}
	for i := range snapshot.Links {
		link := &snapshot.Links[i]
		if !validEPENode(link.Local) || !validEPENode(link.Remote) {
			return errors.New("native BGP link requires local and remote ASN and BGP Router-ID")
		}
		peerNode := false
		for _, attr := range link.Attributes {
			if attr.Type != ls.TLVPeerNodeSID && attr.Type != ls.TLVPeerAdjSID && attr.Type != ls.TLVPeerSetSID {
				continue
			}
			peerNode = peerNode || attr.Type == ls.TLVPeerNodeSID
			switch len(attr.Value) {
			case 7:
				if attr.Value[0]&0xc0 != 0xc0 || attr.Value[4]&0xf0 != 0 {
					return errors.New("native Peer SID label requires V/L flags and a 20-bit label")
				}
			case 8:
				if attr.Value[0]&0x80 != 0 {
					return errors.New("native Peer SID index has the value flag set")
				}
				index := binary.BigEndian.Uint32(attr.Value[4:])
				if uint64(index) >= blocks[epeNodeKey{asn: link.Local.ASN, routerID: link.Local.BGPRouterID}] {
					return errors.New("native Peer SID index requires an advertised SRGB containing the index")
				}
			default:
				return errors.New("native Peer SID has invalid length")
			}
		}
		if !peerNode {
			return errors.New("native EPE link requires a PeerNode SID")
		}
	}
	return nil
}

func validEPENode(node linkstateevents.NodeID) bool {
	return node.ASN != 0 && node.BGPRouterID.Is4() && !node.BGPRouterID.IsUnspecified()
}

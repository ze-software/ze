// Design: docs/architecture/testing/interop.md -- OSPF redistribution to eBGP.
// Related: register_ospf_redistribute_default.go -- native peer-preparation registration.
package bgp

import (
	"errors"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// prepareOSPFRedistributeDefaultPeers creates the ordinary producer interface
// before FRR starts. Its normal zebra configuration owns the interface address.
// The existing core start wrapper runs this Prestart under sh -e, so failed
// link creation prevents daemon startup and cannot become a missing route.
func prepareOSPFRedistributeDefaultPeers(peers []interoplab.PeerConfig, _ string) ([]interoplab.PeerConfig, error) {
	frr := -1
	for index := range peers {
		if peers[index].Name != peerFRR {
			continue
		}
		if frr >= 0 {
			return peers, errors.New("OSPF redistribution topology has two FRR peers")
		}
		frr = index
	}
	if frr < 0 {
		return peers, errors.New("OSPF redistribution topology has no FRR peer")
	}
	peers[frr].Prestart += "ip link add ze-redist0 type dummy\nip link set dev ze-redist0 up\n"
	return peers, nil
}

// Design: docs/architecture/testing/interop.md -- NSSA first-exchange observation.
package bgp

import (
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// prepareNSSAPeers MUST run before starting either prepared peer. Ze's
// original daemon command cannot execute until tcpdump has activated eth0, so
// even a startup-only Type-5 leak belongs to the observed first exchange.
func prepareNSSAPeers(peers []interoplab.PeerConfig, _ string) ([]interoplab.PeerConfig, error) {
	ze, foreign := -1, 0
	for index := range peers {
		switch peers[index].Name {
		case "ze":
			if ze >= 0 { return peers, errors.New("NSSA capture topology has two Ze peers") }
			ze = index
		case peerFRR:
			foreign++
		}
	}
	if ze < 0 { return peers, errors.New("NSSA capture has no Ze peer") }
	if foreign != 1 { return peers, errors.New("NSSA capture needs exactly one native FRR") }
	prestart, err := nssaPacketCapture.prestart()
	if err != nil { return peers, err }
	wrapStart(&peers[ze], linkLocalOwners[zeLinkLocalOwner], netip.Addr{}, prestart)
	return peers, nil
}

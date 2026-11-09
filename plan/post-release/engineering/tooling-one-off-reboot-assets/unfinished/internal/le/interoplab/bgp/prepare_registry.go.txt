// Design: docs/architecture/testing/interop.md -- registered peer preparation.
package bgp

import "github.com/ze-software/ze/internal/le/interoplab"

// peerPreparation transforms one selected scenario before any container starts.
// The second argument is its rendered directory, never a reconstructed identity.
type peerPreparation func([]interoplab.PeerConfig, string) ([]interoplab.PeerConfig, error)

var peerPreparations = map[string]peerPreparation{}

// registerPeerPreparation is called only by feature init functions. An invalid
// static registration is a Ze defect, never a value received from a peer.
func registerPeerPreparation(scenario string, prepare peerPreparation) {
	if scenario == "" {
		panic("BUG: peer preparation registered without a scenario")
	}
	if prepare == nil {
		panic("BUG: peer preparation registered without a callback")
	}
	if _, exists := peerPreparations[scenario]; exists {
		panic("BUG: peer preparation registered twice for " + scenario)
	}
	peerPreparations[scenario] = prepare
}

func prepareRegisteredPeers(peers []interoplab.PeerConfig, rendered, scenario string) ([]interoplab.PeerConfig, error) {
	prepare, registered := peerPreparations[scenario]
	if !registered {
		// Most scenarios have no peer transformation. Their ordinary prepared
		// peers are complete; absence here is not a substitute capture proof.
		return peers, nil
	}
	return prepare(peers, rendered)
}

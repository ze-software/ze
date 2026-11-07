package bgp

import (
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// wrapperExec is how every composed start wrapper ends: it execs the argv
// that follows its $0, the entrypoint and command Docker would have run.
const wrapperExec = `exec "$@"`

// zeImageStart and frrImageStart are the starts the core's wrapper reads from
// the images (`docker image inspect`): test/interop/Dockerfile.ze declares
// ENTRYPOINT ["tini", "--", "ze"], and the pinned FRR image Entrypoint
// ["/sbin/tini", "--"] with Cmd ["/usr/lib/frr/docker-start"].
var (
	zeImageStart  = interoplab.ImageStart{Entrypoint: []string{"tini", "--", "ze"}}
	frrImageStart = interoplab.ImageStart{Entrypoint: []string{"/sbin/tini", "--"}, Command: []string{"/usr/lib/frr/docker-start"}}
)

// composedPeers answers the prepared peers as the core starts them: every
// peer that joins one of links or carries a Prestart gets the start wrapper
// runScenario composes (interoplab.ComposeStart), against its image's start.
// A test then reads the start a container really gets.
func composedPeers(t *testing.T, peers []interoplab.PeerConfig, links []interoplab.Link) []interoplab.PeerConfig {
	t.Helper()
	composed := make([]interoplab.PeerConfig, len(peers))
	for index := range peers {
		peer := peers[index]
		if peer.Prestart == "" && len(interoplab.LinkMemberships(links, peer.Name, peer.Host)) == 0 {
			composed[index] = peer
			continue
		}
		start := zeImageStart
		if peer.Image != "ze" {
			start = frrImageStart
		}
		composed[index] = interoplab.ComposeStart(peer, links, start)
	}
	return composed
}

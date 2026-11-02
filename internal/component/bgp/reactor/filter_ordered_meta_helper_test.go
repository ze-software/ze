package reactor

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// runIngressPolicyChain runs the ingress policy step as the received path
// does, with a metadata map of the test's own that it discards: for tests
// that judge the step's verdict and no metadata set.
func (r *Reactor) runIngressPolicyChain(peer *Peer, peerAddr netip.Addr, src *filterapi.PeerFilterInfo, wireUpdate *wireu.WireUpdate, payload []byte) ingressStepResult {
	var meta map[string]any
	return r.runIngressPolicyChainMeta(peer, peerAddr, src, &meta, wireUpdate, payload)
}

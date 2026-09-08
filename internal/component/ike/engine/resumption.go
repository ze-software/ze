// Design: docs/architecture/ike/ipsec-14-responder.md -- the EAP-TLS authenticator's ticket key
// Detail: internal/core/eap/resumption.go -- the state this file hands out
// RFC: rfc/short/rfc9190.md -- Sections 2.1.2, 2.1.3 and 5.7

package engine

import (
	"log/slog"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/eap"
)

// resumptionStates holds one peering's EAP-TLS session-resumption state per
// configured peer name, and the authentication config that state was built for.
//
// IT OUTLIVES THE PeerSession ON PURPOSE. `clear vpn ipsec sa` destroys every
// session and rebuilds it (TerminateAllSAs, register.go, then reEstablishFn), so
// a store held on the session alone would be discarded by the operator command
// most likely to be followed by a second authentication, and no ticket ze issued
// could ever be redeemed. RFC 9190 Section 5.7 bounds how long a ticket lives at
// seven days; an operator typing `clear` is not that bound.
//
// The map is bounded by the config: reconcilePeers drops the entry when the peer
// leaves it.
var resumptionStates = struct {
	mu     sync.Mutex
	byPeer map[string]resumptionState
}{byPeer: map[string]resumptionState{}}

// resumptionState is one peering's store beside the authentication config that
// authorized it.
type resumptionState struct {
	auth  ipsec.AuthConfig
	store *eap.Resumption
}

// resumptionFor answers the store for one peer, building a fresh one whenever
// the peer's authentication config differs from the one the held store was built
// for.
//
// THE AUTH CONFIG IS THE INVALIDATION KEY, and it has to be. A resumed handshake
// presents no certificate at all: crypto/tls carries the chain in the ticket and
// re-checks it against the CURRENT trust anchors. An operator who changes the
// certificate, the CA or the revocation lists therefore expects the next
// authentication to be decided by the new material, and a ticket minted under
// the old configuration MUST NOT survive that edit.
func resumptionFor(peerName string, auth ipsec.AuthConfig) *eap.Resumption {
	resumptionStates.mu.Lock()
	defer resumptionStates.mu.Unlock()

	if held, ok := resumptionStates.byPeer[peerName]; ok && held.auth.Equal(auth) {
		return held.store
	}
	store := eap.NewResumption(time.Now, auth.SessionResumption)
	resumptionStates.byPeer[peerName] = resumptionState{auth: auth, store: store}
	return store
}

// recordEAPTLSAuthentication counts and logs how one completed EAP-TLS
// authentication was reached, on either role.
//
// It is the operator's only view of whether resumption is working: no CLI
// command reports DidResume, and an SA that resumed looks exactly like one that
// did not (`show vpn ipsec sa` carries no EAP field). The counters answer over
// time and the log line answers for one authentication.
//
// A peer running another EAP method is skipped rather than logged with
// resumed=false, which would read as a resumption that failed.
func recordEAPTLSAuthentication(sa *SA, resumed bool, log *slog.Logger) {
	if sa.PeerCfg.Auth.Mode != ipsec.AuthEAPTLS {
		return
	}
	countEAPTLSAuthentication(sa.PeerName, resumed)
	log.Info("ike: EAP-TLS authenticated", "peer", sa.PeerName, "resumed", resumed)
}

// forgetResumption drops a peer's store, for a peer the config no longer names.
func forgetResumption(peerName string) {
	resumptionStates.mu.Lock()
	defer resumptionStates.mu.Unlock()
	delete(resumptionStates.byPeer, peerName)
}

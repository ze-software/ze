// Design: docs/architecture/behavior/peer-lifecycle.md — one immutable OPEN proposal.
// Related: session_negotiate.go — prepareOpen constructs and publishes the proposal.
// Related: session_open_as.go — admission consumes the proposal's configuration.
package reactor

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// preparedOpen is the successful local OPEN construction result. Its message,
// configuration and plugin declarations MUST remain unchanged after publication.
// Its nil pointer and zero value are invalid: only successful prepareOpen
// constructs it. Session.settings remains shared for live policies; admission
// MUST use this proposal so an incumbent's dynamic-AS publication cannot change a candidate.
type preparedOpen struct {
	message            *message.Open
	settings           *PeerSettings
	pluginDeclarations map[uint8][]string
}

// openProposal returns the constructed proposal or refuses an OPEN received
// without one. Both TCP setup rails MUST prepare it before accepting a remote OPEN.
func (s *Session) openProposal() (*preparedOpen, error) {
	s.mu.RLock()
	proposal := s.proposal
	s.mu.RUnlock()
	if proposal == nil {
		return nil, fmt.Errorf("%w: remote OPEN has no prepared local proposal", ErrInvalidState)
	}
	return proposal, nil
}

// judgedPeerAS shares sessionPeerAS with every OPEN consumer. A configured AS
// wins; a dynamic proposal uses the advertised AS under its local ASN4 offer.
func (p *preparedOpen) judgedPeerAS(open *message.Open) uint32 {
	return sessionPeerAS(p.settings.PeerAS, !p.settings.DisableASN4, open)
}

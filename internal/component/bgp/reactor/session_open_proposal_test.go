package reactor

import "github.com/ze-software/ze/internal/component/bgp/message"

// preparedOpenForTest preserves deliberately hand-built local OPENs while
// pairing each with the same immutable configuration input as production.
// Callers MUST configure settings before constructing the proposal.
func preparedOpenForTest(s *Session, open *message.Open) *preparedOpen {
	return &preparedOpen{message: open, settings: s.openSettingsSnapshot()}
}

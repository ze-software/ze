// Design: docs/architecture/behavior/peer-lifecycle.md -- one plugin snapshot per OPEN proposal.
// Related: peer.go -- captures declarations before applying runtime GR bits.
// Related: session_negotiate.go -- constructs OPEN from the captured capabilities.
package reactor

import "github.com/ze-software/ze/internal/core/bgp/capability"

// pluginCapabilitySnapshot pairs one injector generation's raw declarations
// with the wire capabilities derived from them. The declaration index precedes
// runtime GR bit edits, so expiration of a restart window is not a config edit.
// The owner MUST NOT mutate either member after publication to a Session.
type pluginCapabilitySnapshot struct {
	declarations map[uint8][]string
	capabilities []capability.Capability
}

// setPluginCapabilitySnapshotGetter installs the owning Peer's single-snapshot
// producer. The caller MUST install it before starting the Session, and the
// producer MUST keep its returned snapshot immutable after publication.
func (s *Session) setPluginCapabilitySnapshotGetter(getter func() pluginCapabilitySnapshot) {
	s.pluginCapabilityGetter = getter
}

// pluginCapabilitySnapshot captures the offer once. A Session with no owning
// Peer has no plugin declarations, as for an ordinary standalone test session.
func (s *Session) pluginCapabilitySnapshot() pluginCapabilitySnapshot {
	if s.pluginCapabilityGetter == nil {
		return pluginCapabilitySnapshot{}
	}
	return s.pluginCapabilityGetter()
}

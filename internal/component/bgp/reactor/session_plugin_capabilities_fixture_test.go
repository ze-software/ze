// Design: docs/architecture/behavior/peer-lifecycle.md -- coherent plugin offers for session tests.
// Related: session_plugin_capabilities.go -- the runtime snapshot contract.
package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// buildOpen preserves the wire-only proposal fixture used by existing OPEN
// tests. Runtime callers capture one snapshot for both the proposal and its
// recorded declaration, then invoke the same production builder directly.
func (s *Session) buildOpen(settings *PeerSettings, configCaps []capability.Capability) (*message.Open, error) {
	// RFC 5492 Section 1
	return s.buildOpenFromSnapshot(settings, configCaps, s.pluginCapabilitySnapshot())
}

// setPluginCapabilityGetter preserves the wire-capability fixture used by
// existing OPEN encoder tests. Production installs the Peer's paired snapshot;
// this test-only adapter records the exact bytes its fixture supplied.
func (s *Session) setPluginCapabilityGetter(getter func() []capability.Capability) {
	s.setPluginCapabilitySnapshotGetter(func() pluginCapabilitySnapshot {
		caps := getter()
		injected := make([]plugin.InjectedCapability, len(caps))
		for index, offered := range caps {
			encoded := make([]byte, offered.Len())
			offered.WriteTo(encoded, 0)
			injected[index] = plugin.InjectedCapability{Code: uint8(offered.Code()), Value: encoded[2:]}
		}
		return pluginCapabilitySnapshot{declarations: pluginDeclarationIndex(injected), capabilities: caps}
	})
}

// getPluginCapabilities is the wire-only view existing GR tests ask for. The
// paired snapshot producer still performs their runtime restart-bit transform.
func (p *Peer) getPluginCapabilities() []capability.Capability {
	return p.pluginCapabilitySnapshot().capabilities
}

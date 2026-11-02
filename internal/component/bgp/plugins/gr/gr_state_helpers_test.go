package gr

import "github.com/ze-software/ze/internal/core/family"

// familyRetained answers whether one family of peerAddr is retained, from the
// authoritative GR/LLGR family set retainedFamilies also reads. Safe for
// concurrent use; callers MUST NOT hold m.mu.
func (m *grStateManager) familyRetained(peerAddr string, fam family.Family) bool { //nolint:unparam // Keep the queried peer explicit in retention assertions; current tests share one peer fixture.
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.peers[peerAddr]
	return state != nil && state.staleFamilies[fam]
}

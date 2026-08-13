package role

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// TestOTCIngressPublishesPeerRole pins the meta key OTCIngressFilter
// publishes for other filters: "src-peer-role", what the source peer IS to
// us.
//
// It is a separate key from "src-role" and the two carry OPPOSITE values
// for the same peer, which is the whole reason a consumer must not read the
// wrong one. "src-role" is OUR configured role toward the peer, taken from
// the `import` keyword. "src-peer-role" is resolvePeerRole's answer: the
// peer's own Role capability when it sent one, otherwise the complement of
// the configuration, from RFC 9234 Table 2.
//
// This is the cross-plugin seam the community filter's RFC 8195 relation
// tag reads (docs/architecture/meta/filter-community.md). Deleting the
// publish here silently stops every relation tag being written. No test in
// the other plugin can see it, because that plugin's tests inject the key
// directly.
//
// VALIDATES: AC-1..AC-4 of plan/spec-bcp194-1-communities.md, the producer half
// PREVENTS: a consumer reading "src-role" and tagging a provider as a customer.
func TestOTCIngressPublishesPeerRole(t *testing.T) {
	t.Cleanup(func() {
		setFilterState(nil, nil)
		filterMu.Lock()
		filterRemoteRoles = nil
		filterMu.Unlock()
	})

	ingress := func(addr string, peerAS uint32) map[string]any {
		meta := make(map[string]any)
		src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr(addr), PeerAS: peerAS}
		OTCIngressFilter(src, buildTestPayload(buildTestAttrs(0), nil), meta)
		return meta
	}

	t.Run("config only: the complement of our configured role", func(t *testing.T) {
		setFilterState(map[string]*peerRoleConfig{"10.0.0.1": {role: roleCustomer}}, nil)
		meta := ingress("10.0.0.1", 65001)

		assert.Equal(t, roleCustomer, meta["src-role"], "our role toward the peer")
		assert.Equal(t, roleProvider, meta["src-peer-role"],
			"the peer IS a provider when we are its customer (RFC 9234 Table 2)")
	})

	t.Run("an announced capability wins over the config complement", func(t *testing.T) {
		setFilterState(map[string]*peerRoleConfig{"10.0.0.2": {role: roleCustomer}}, nil)
		setFilterRemoteRole("10.0.0.2", "", rolePeer)
		meta := ingress("10.0.0.2", 65002)

		assert.Equal(t, roleCustomer, meta["src-role"], "unchanged: still our config")
		assert.Equal(t, rolePeer, meta["src-peer-role"],
			"the peer announced Peer, which resolvePeerRole prefers over the complement")
	})

	t.Run("published even when this plugin has no OTC work for the peer", func(t *testing.T) {
		// No local role config, but the peer announced one. OTCIngressFilter
		// returns early here; the key must still be set, because what the peer
		// IS is a different question from whether OTC applies to it.
		setFilterState(map[string]*peerRoleConfig{"10.0.0.3": nil}, nil)
		setFilterRemoteRole("10.0.0.3", "", roleProvider)
		meta := ingress("10.0.0.3", 65003)

		assert.Equal(t, roleProvider, meta["src-peer-role"])
		_, hasSrcRole := meta["src-role"]
		assert.False(t, hasSrcRole, "no configured role, so no src-role")
	})

	t.Run("unresolvable role leaves the key ABSENT, never present and empty", func(t *testing.T) {
		setFilterState(nil, nil)
		meta := ingress("10.0.0.99", 65099)

		_, has := meta["src-peer-role"]
		assert.False(t, has,
			"a reader must not be able to mistake 'we could not tell' for a role")
	})
}

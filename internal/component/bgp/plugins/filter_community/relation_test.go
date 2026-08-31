package filter_community

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRelationParameterFromRole covers the five RFC 9234 role values plus
// the unresolved case, and pins the two that must write nothing.
//
// VALIDATES: AC-1, AC-2, AC-3, AC-4 (spec-bcp194-1-communities)
// PREVENTS: a guessed relation on a route whose peer role could not be resolved,
// and an attribute written on a route-server session, which RFC 7947
// forbids.
func TestRelationParameterFromRole(t *testing.T) {
	tests := []struct {
		name     string
		peerRole string
		want     uint32
	}{
		{"provider writes parameter 4", "provider", 4},
		{"customer writes parameter 2", "customer", 2},
		{"peer writes parameter 3", "peer", 3},
		{"rs writes nothing", "rs", 0},
		{"rs-client writes nothing", "rs-client", 0},
		{"unresolved role writes nothing", "", 0},
		{"unknown token writes nothing", "unknown", 0},
		{"invented token writes nothing", "transit", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, relationParameterFor(tt.peerRole))
		})
	}
}

// TestRelationParameterInternalNeverWritten pins that parameter 1 (route
// originated internally) is reachable from no peer role. It applies to
// locally originated routes, so any peer role producing it would be a false
// statement about the route's origin.
//
// VALIDATES: AC-1..AC-4 boundary row -- parameter 1 is reserved and never written.
func TestRelationParameterInternalNeverWritten(t *testing.T) {
	for _, role := range []string{"provider", "customer", "peer", "rs", "rs-client", "", "unknown"} {
		assert.NotEqual(t, relationInternal, relationParameterFor(role),
			"role %q must not produce the internally-originated parameter", role)
	}
}

// TestRelationWireValue pins RFC 8092 Section 3 field order: Global
// Administrator, Local Data Part 1, Local Data Part 2, each four octets in
// network byte order.
//
// PREVENTS: a field-order swap, which would make every tag name a different AS.
func TestRelationWireValue(t *testing.T) {
	got := relationWireValue(65000, 3, 4)
	require.Len(t, got, 12)
	assert.Equal(t, uint32(65000), binary.BigEndian.Uint32(got[0:4]), "global administrator")
	assert.Equal(t, uint32(3), binary.BigEndian.Uint32(got[4:8]), "function")
	assert.Equal(t, uint32(4), binary.BigEndian.Uint32(got[8:12]), "parameter")
}

// TestRelationPeerRoleFromMeta covers the meta contract: a wrong type is
// treated as absent, and absent means no role, which is the closed branch.
//
// PREVENTS: a non-string meta value panicking the ingress path, and a wrong-type
// value being MORE permissive than a missing one.
func TestRelationPeerRoleFromMeta(t *testing.T) {
	assert.Equal(t, "provider", relationPeerRoleFromMeta(map[string]any{"src-peer-role": "provider"}))
	assert.Equal(t, "", relationPeerRoleFromMeta(map[string]any{}))
	assert.Equal(t, "", relationPeerRoleFromMeta(nil))
	assert.Equal(t, "", relationPeerRoleFromMeta(map[string]any{"src-peer-role": 4}),
		"a non-string value is treated as absent, per docs/architecture/meta/README.md")
	assert.Equal(t, "", relationPeerRoleFromMeta(map[string]any{"src-role": "customer"}),
		"the OTC key carries our role toward the peer, not the peer's role, and must not be read here")
}

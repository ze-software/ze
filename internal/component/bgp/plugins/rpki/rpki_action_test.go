package rpki

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
)

// TestBuildDecisions_NotFoundEnforced verifies origin NotFound is now enforced: reject drops the
// route, log-only and accept keep it (AC-14). Before this spec, not-found was parsed but inert.
func TestBuildDecisions_NotFoundEnforced(t *testing.T) {
	cases := []struct {
		name       string
		action     uint8
		wantAccept bool
	}{
		{"reject", ASPAPolicyReject, false},
		{"log-only", ASPAPolicyLogOnly, true},
		{"accept", ASPAPolicyAccept, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rp := &rPKIPlugin{}
			rp.originNotFoundAction.Store(uint32(tc.action))
			d := rp.buildDecisions([]validationRequest{{state: ValidationNotFound}})
			require.Len(t, d, 1)
			assert.Equal(t, tc.wantAccept, d[0].Accept)
		})
	}
}

// TestBuildDecisions_PerPeerAction verifies a per-peer override changes the outcome for that peer
// only; a peer with no override uses the global action (AC-2).
func TestBuildDecisions_PerPeerAction(t *testing.T) {
	rp := &rPKIPlugin{}
	rp.originInvalidAction.Store(uint32(ASPAPolicyReject)) // global: reject Invalid

	m := map[configjson.PeerConfigKey]peerActionSet{
		{ID: "192.0.2.1"}: {
			OriginInvalid:  resolvedAction{Action: ASPAPolicyAccept, Source: sourcePeer},
			OriginNotFound: resolvedAction{Action: ASPAPolicyAccept, Source: sourceGlobal},
			ASPAInvalid:    resolvedAction{Action: ASPAPolicyLogOnly, Source: sourceGlobal},
			ASPAUnknown:    resolvedAction{Action: ASPAPolicyAccept, Source: sourceGlobal},
		},
	}
	rp.perPeerActions.Store(&m)

	d := rp.buildDecisions([]validationRequest{
		{peerAddr: "192.0.2.1", state: ValidationInvalid}, // override -> accept
		{peerAddr: "192.0.2.2", state: ValidationInvalid}, // no override -> global reject
	})
	require.Len(t, d, 2)
	assert.True(t, d[0].Accept, "peer with override accepts Invalid")
	assert.False(t, d[1].Accept, "peer without override rejects Invalid (global)")
}

// TestBuildDecisions_DynamicGroupMemberUsesGroupAction verifies AC-6 at the
// decision: a session created from a listen-range group carries an address the
// config document never mentions, so its actions resolve under the group's name.
//
// A peer entry still beats the group entry (AC-9), and a session in no group is
// untouched by the group's statement.
func TestBuildDecisions_DynamicGroupMemberUsesGroupAction(t *testing.T) {
	rp := &rPKIPlugin{}
	rp.originInvalidAction.Store(uint32(ASPAPolicyReject)) // global: reject Invalid

	accept := peerActionSet{
		OriginInvalid:  resolvedAction{Action: ASPAPolicyAccept, Source: sourceGroup},
		OriginNotFound: resolvedAction{Action: ASPAPolicyAccept, Source: sourceGlobal},
		ASPAInvalid:    resolvedAction{Action: ASPAPolicyLogOnly, Source: sourceGlobal},
		ASPAUnknown:    resolvedAction{Action: ASPAPolicyAccept, Source: sourceGlobal},
	}
	reject := accept
	reject.OriginInvalid = resolvedAction{Action: ASPAPolicyReject, Source: sourcePeer}

	m := map[configjson.PeerConfigKey]peerActionSet{
		configjson.GroupKey("ix"): accept,
		{ID: "192.0.2.7"}:         reject,
		// A PEER named "ix". config.ResolveBGPTree refuses a duplicate peer name and
		// never compares a group's against it, so this config loads. The typed key
		// keeps the two apart: a bare group key would answer the member's lookup
		// from this entry.
		{ID: "ix"}: reject,
	}
	rp.perPeerActions.Store(&m)

	d := rp.buildDecisions([]validationRequest{
		{peerAddr: "192.0.2.50", peerGroup: "ix", state: ValidationInvalid}, // group -> accept
		{peerAddr: "192.0.2.7", peerGroup: "ix", state: ValidationInvalid},  // own entry beats it
		{peerAddr: "203.0.113.9", state: ValidationInvalid},                 // no group -> global reject
	})
	require.Len(t, d, 3)
	assert.True(t, d[0].Accept, "a member of the dynamic group did not inherit its action")
	assert.False(t, d[1].Accept, "the group's action beat the peer's own entry")
	assert.False(t, d[2].Accept, "a session outside the group picked up the group's action")
}

// TestBuildDecisions_PerPeerNotFound verifies a per-peer not-found override is enforced.
func TestBuildDecisions_PerPeerNotFound(t *testing.T) {
	rp := &rPKIPlugin{}
	rp.originNotFoundAction.Store(uint32(ASPAPolicyAccept)) // global: accept NotFound

	m := map[configjson.PeerConfigKey]peerActionSet{
		{ID: "192.0.2.1"}: {
			OriginInvalid:  resolvedAction{Action: ASPAPolicyReject, Source: sourceGlobal},
			OriginNotFound: resolvedAction{Action: ASPAPolicyReject, Source: sourcePeer}, // reject NotFound for this peer
			ASPAInvalid:    resolvedAction{Action: ASPAPolicyLogOnly, Source: sourceGlobal},
			ASPAUnknown:    resolvedAction{Action: ASPAPolicyAccept, Source: sourceGlobal},
		},
	}
	rp.perPeerActions.Store(&m)

	d := rp.buildDecisions([]validationRequest{
		{peerAddr: "192.0.2.1", state: ValidationNotFound}, // override -> reject
		{peerAddr: "192.0.2.2", state: ValidationNotFound}, // global -> accept
	})
	require.Len(t, d, 2)
	assert.False(t, d[0].Accept, "peer overrides NotFound to reject")
	assert.True(t, d[1].Accept, "other peer keeps NotFound (global accept)")
}

// Related: role.go -- getFilterConfig, remoteRoleLocked, setFilterState, applyValidateOpen
// Related: otc.go -- OTCIngressFilter, the RFC 9234 Section 5 ingress gate
// RFC: rfc/short/rfc9234.md
//
// Spec: plan/spec-fixit-dynamic-group-peer-config.md (AC-5, AC-9).
//
// A peer created from a dynamic group's template exists in no config document.
// The reactor builds it when a connection arrives and names it "dyn-<addr>".
// Its GROUP is the one identity it shares with the operator's config. Every
// decision the role plugin makes for such a peer therefore resolves through the
// group key or resolves nothing at all. These tests hold that resolution at the
// two places it decides an RFC 9234 outcome: the OPEN check and the ingress
// gate.

package role

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// ixGroupConfigs is the canonical IXP route-server shape this spec exists for.
// A listen-range group states one role, one static peer states its own, and no
// entry names the members the group will build.
func ixGroupConfigs(strict bool) map[string]*peerRoleConfig {
	return map[string]*peerRoleConfig{
		configjson.CapabilityGroupKey("ix"): {role: roleRS, strict: strict},
		"10.0.0.1":                          {role: roleProvider},
	}
}

// TestGetFilterConfigFallsBackToGroup verifies the lookup AC-5 rests on.
//
// VALIDATES: a peer built from a dynamic group's template resolves its group's
// role config, a configured peer keeps resolving its own (AC-9), and a peer
// whose group states nothing resolves no config at all.
// PREVENTS: the silent under-enforcement this spec fixes. A nil cfg sends every
// RFC 9234 Section 5 gate down its permissive branch, and a miss is then
// indistinguishable from "this operator configured no role".
func TestGetFilterConfigFallsBackToGroup(t *testing.T) {
	clearFilterState(t)
	setFilterState(ixGroupConfigs(false), nil)

	cfg, _ := getFilterConfig("192.0.2.7", "dyn-192.0.2.7", "ix")
	require.NotNil(t, cfg, "a dynamic group's member must resolve its group's role config")
	assert.Equal(t, roleRS, cfg.role, "the role must be the one the GROUP states")

	// AC-9: what a peer states beats what its group states.
	cfg, _ = getFilterConfig("10.0.0.1", "upstream", "ix")
	require.NotNil(t, cfg)
	assert.Equal(t, roleProvider, cfg.role,
		"a configured peer keeps its own role even when its group states another")

	// A group that states nothing answers nothing: the miss stays a miss.
	cfg, _ = getFilterConfig("192.0.2.8", "dyn-192.0.2.8", "other")
	assert.Nil(t, cfg, "no config anywhere must not resolve to another group's")

	// The group is consulted by NAME, never by a peer's address, so a group
	// selector can never answer for a peer of the same name (A-7).
	cfg, _ = getFilterConfig("ix", "ix", "")
	assert.Nil(t, cfg, "a peer named like the group must not draw the group's template")
}

// TestOTCIngressGateRunsForADynamicGroupMember is AC-5 at the decision.
//
// VALIDATES: an UPDATE from a member of a listen-range group is judged by the
// group's role -- the member is an RS-Client because the group states RS, so a
// route already carrying OTC is a route leak.
// PREVENTS: the defect this spec fixes: OTCIngressFilter returned early with
// cfg == nil for every such member, so no ingress rule ran for any IXP route
// server member.
//
// RFC requirement: RFC9234-5-1 positive -- a route carrying OTC received from an RS-Client is a route leak and marked ineligible, including when that peer's role comes from the dynamic group it was built from.
func TestOTCIngressGateRunsForADynamicGroupMember(t *testing.T) {
	clearFilterState(t)
	setFilterState(ixGroupConfigs(false), nil)

	member := filterapi.PeerFilterInfo{
		Address:   netip.MustParseAddr("192.0.2.7"),
		Name:      "dyn-192.0.2.7",
		GroupName: "ix",
		PeerAS:    65007,
	}
	leaked := buildTestPayload(buildTestAttrs(65099), []byte{24, 10, 2, 2})

	meta := make(map[string]any)
	accept, _ := OTCIngressFilter(member, leaked, meta)
	assert.False(t, accept,
		"RFC 9234 Section 5: a route with OTC from an RS-Client is a route leak")
	assert.Equal(t, roleRS, meta["src-role"], "src-role is OUR role toward the member")
	assert.Equal(t, roleRSClient, meta["src-peer-role"],
		"src-peer-role is what the member IS to us, the RFC 9234 Table 2 complement of the group's role")

	// Discrimination: the identical UPDATE from a peer that belongs to no group
	// reaches no gate, because nothing resolves a role for it. The group name is
	// the whole difference between the two calls.
	orphan := member
	orphan.GroupName = ""
	accept, _ = OTCIngressFilter(orphan, leaked, make(map[string]any))
	assert.True(t, accept, "without the group there is no role config and so no gate")
}

// TestValidateOpenRolePairRunsForADynamicGroupMember is the RFC 9234 Section 4.2
// half: the OPEN check.
//
// VALIDATES: a member's OPEN is validated against the role its GROUP states, so
// a non-corresponding pair is refused with the Role Mismatch NOTIFICATION and a
// corresponding one is accepted.
// PREVENTS: the state this spec found -- rpc.ValidateOpenInput carried no group,
// applyValidateOpen resolved cfg == nil for every dynamic member, and
// validateOpenRolePair returned Accept unconditionally while ze was advertising
// the group's Role capability to that same peer.
//
// RFC requirement: RFC9234-4.2-1 negative -- a pair absent from RFC 9234 Table 2 does not correspond and is not accepted, for a peer whose role comes from its dynamic group.
// RFC requirement: RFC9234-4.2-2 positive -- a non-corresponding pair is rejected with the Role Mismatch NOTIFICATION code 2 subcode 11, for a peer built from a dynamic group's template.
func TestValidateOpenRolePairRunsForADynamicGroupMember(t *testing.T) {
	clearFilterState(t)
	configs := ixGroupConfigs(false)

	// The group states RS, so RFC 9234 Table 2 admits one remote role: RS-Client.
	member := &sdk.ValidateOpenInput{
		Peer:  "dyn-192.0.2.7",
		Group: "ix",
		Remote: rpc.ValidateOpenMessage{
			Capabilities: []sdk.ValidateOpenCapability{roleCap(hexByte(1))}, // RS
		},
	}
	out := applyValidateOpen(configs, nil, member)
	require.NotNil(t, out)
	assert.False(t, out.Accept, "RS/RS is not a pair RFC 9234 Table 2 holds")
	assert.Equal(t, uint8(2), out.NotifyCode)
	assert.Equal(t, uint8(11), out.NotifySubcode)

	member.Remote.Capabilities = []sdk.ValidateOpenCapability{roleCap(hexByte(2))} // RS-Client
	out = applyValidateOpen(configs, nil, member)
	require.NotNil(t, out)
	assert.True(t, out.Accept, "RS/RS-Client corresponds and must establish")

	// The learned role reaches the filters under the member's own name, which is
	// the only key its OPEN can be recorded against.
	_, learned := getFilterConfig("192.0.2.7", "dyn-192.0.2.7", "ix")
	assert.Equal(t, roleRSClient, learned,
		"a member's announced role must be readable at the filter decision")

	// Discrimination: the same OPEN with no group resolves no config, which is
	// what every dynamic member did before rpc.ValidateOpenInput carried one.
	orphan := &sdk.ValidateOpenInput{
		Peer: "dyn-192.0.2.9",
		Remote: rpc.ValidateOpenMessage{
			Capabilities: []sdk.ValidateOpenCapability{roleCap(hexByte(1))},
		},
	}
	out = applyValidateOpen(configs, nil, orphan)
	require.NotNil(t, out)
	assert.True(t, out.Accept, "no group and no peer entry means no role config to check against")
}

// TestValidateOpenStrictModeRefusesADynamicGroupMemberWithNoRole covers the
// operator's strict-mode choice on the same path.
//
// VALIDATES: `strict` stated on a listen-range group refuses a member whose OPEN
// carries no Role capability, and leaves a member that sends the corresponding
// role alone.
// PREVENTS: strict mode silently applying to configured peers only, which is the
// opposite of what an operator asks for by stating it on a route-server group.
//
// RFC requirement: RFC9234-4.2-5 positive -- strict mode rejects a peer that sends no Role capability, including a peer built from a dynamic group's template.
func TestValidateOpenStrictModeRefusesADynamicGroupMemberWithNoRole(t *testing.T) {
	clearFilterState(t)
	configs := ixGroupConfigs(true)

	silent := &sdk.ValidateOpenInput{Peer: "dyn-192.0.2.7", Group: "ix", Remote: rpc.ValidateOpenMessage{}}
	out := applyValidateOpen(configs, nil, silent)
	require.NotNil(t, out)
	assert.False(t, out.Accept, "strict mode must refuse a member that announces no role")
	assert.Equal(t, uint8(2), out.NotifyCode)
	assert.Equal(t, uint8(11), out.NotifySubcode)
	assert.Contains(t, out.Reason, "strict")

	compliant := &sdk.ValidateOpenInput{
		Peer:  "dyn-192.0.2.8",
		Group: "ix",
		Remote: rpc.ValidateOpenMessage{
			Capabilities: []sdk.ValidateOpenCapability{roleCap(hexByte(2))}, // RS-Client
		},
	}
	out = applyValidateOpen(configs, nil, compliant)
	require.NotNil(t, out)
	assert.True(t, out.Accept, "strict mode must not refuse a member that announces the corresponding role")
}

// TestReconfigureKeepsADynamicGroupMembersLearnedRole guards the retention half.
//
// A learned role is a property of the SESSION, and an established member sends
// no second OPEN. setFilterState asked configs[key] alone, and a member's key is
// in no config, so every reconfigure dropped what every member had announced.
//
// VALIDATES: a member's learned role survives a reconfigure that keeps its
// group, and is dropped by one that removes it.
// PREVENTS: the reload-time regression already fixed for configured peers
// (TestReconfigureKeepsLearnedRolesForStillConfiguredPeers) reappearing for
// dynamic members, where it silently retargets OTCEgressFilter's export set.
func TestReconfigureKeepsADynamicGroupMembersLearnedRole(t *testing.T) {
	clearFilterState(t)
	setFilterState(ixGroupConfigs(false), nil)
	setFilterRemoteRole("dyn-192.0.2.7", "ix", roleRSClient)

	// A reload that keeps the group keeps what the member announced.
	setFilterState(ixGroupConfigs(false), nil)
	_, learned := getFilterConfig("192.0.2.7", "dyn-192.0.2.7", "ix")
	assert.Equal(t, roleRSClient, learned, "a live member's announced role must survive a reload")
	assert.True(t, remoteRoleRecorded("192.0.2.7", "dyn-192.0.2.7"),
		"the member's OPEN stays recorded, which is a separate fact from what it recorded")

	// A reload that removes the group drops it: no reader can reach it again.
	setFilterState(map[string]*peerRoleConfig{"10.0.0.1": {role: roleProvider}}, nil)
	assert.False(t, remoteRoleRecorded("192.0.2.7", "dyn-192.0.2.7"),
		"a member of a group the config no longer holds must not keep an unreachable entry")
}

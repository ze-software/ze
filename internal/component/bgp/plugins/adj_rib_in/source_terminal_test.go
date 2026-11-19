// Design: docs/architecture/plugin/rib-storage-design.md -- original owner removal.
// Related: source_state.go, source_policy.go, retained_validation.go.
package adj_rib_in

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The real terminal event has an original owner and no physical connection.
// JSON is decoded by the same full event parser as ordinary state delivery.
func terminalAdjState(t *testing.T, p *adjOwnerProducer, owner uint64) {
	t.Helper()
	if !p.json {
		p.r.handleStructuredState(&rpc.StructuredEvent{EventType: rpc.EventKindState,
			State: rpc.SessionStateDown, PeerAddress: p.address.String(), PeerOwner: owner, Reason: rpc.ReasonPeerRemoved})
		return
	}
	raw := fmt.Sprintf(`{"type":"state","peer":{"remote":{"address":%q,"as":65000}},"peer-owner":%d,"state":"down","reason":%q}`,
		p.address.String(), owner, rpc.ReasonPeerRemoved)
	event, err := bgp.ParseEvent([]byte(raw))
	require.NoError(t, err)
	require.Zero(t, event.ConnectionID)
	p.r.handleState(event)
}

// Terminal removal consumes installed, pending, early, retained and policy
// state even after physical DOWN. A same-address sibling and raw observation
// remain useful, with and without a successor connection of the removed owner.
// MUTATION: ignore ReasonPeerRemoved on either state rail to redden this.
func TestAdjTerminalOwnerClearsAllStateBothRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		for _, reconnect := range []bool{false, true} {
			t.Run(fmt.Sprintf("json-%t/reconnect-%t", jsonRail, reconnect), func(t *testing.T) {
				p := newAdjOwnerProducer(t, jsonRail)
				p.state(1701, 41, true)
				p.state(1702, 42, true)
				old := p.receive(1701, 41, 71, 0)
				sibling := p.receive(1702, 42, 72, 0)
				p.receive(0, 0, 73, 4)
				observedSource := receivedPeer{address: p.address}
				observedRoutes := p.r.ribIn[observedSource]
				require.NotNil(t, observedRoutes)
				require.Equal(t, 1, observedRoutes.Len())
				p.r.validationEnabled = true
				pending := p.receive(1701, 41, 74, 1)
				decision := rpc.ValidationDecision{PeerAddr: p.address.String(), PeerOwner: 1701, ConnectionID: 41,
					Family: "ipv4/unicast", Prefix: "10.77.2.0/24", MsgID: 75, Accept: true, ValState: ValidationValid}
				result, err := p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
				require.NoError(t, err)
				require.Equal(t, 1, result.Early)
				siblingPending := p.receive(1702, 42, 78, 6)
				siblingDecision := decision
				siblingDecision.PeerOwner = 1702
				siblingDecision.ConnectionID = 42
				siblingDecision.Prefix = "10.77.7.0/24"
				siblingDecision.MsgID = 79
				result, err = p.r.handleBatchValidateTyped([]rpc.ValidationDecision{siblingDecision})
				require.NoError(t, err)
				require.Equal(t, 1, result.Early)
				siblingEarly := pendingKey(receivedPeer{address: p.address, owner: 1702}, routeKeyFromStrings(family.IPv4Unicast, "10.77.7.0/24", 0))
				require.NoError(t, p.policy(1701, rpc.SourcePolicyPrepare, "terminal", 0))
				require.NoError(t, p.policy(1701, rpc.SourcePolicyApply, "terminal", 6))
				require.NoError(t, p.policy(1702, rpc.SourcePolicyPrepare, "sibling", 0))
				payload, err := retention.Encode(1701, 41, p.address.String(), []family.Family{family.IPv4Unicast})
				require.NoError(t, err)
				matched, err := p.r.handleRetainedValidationEvent(retainedAdjEnvelope(t, retention.EventType, payload))
				require.True(t, matched)
				require.NoError(t, err)
				p.state(1701, 41, false)
				require.True(t, p.r.routePresent(old))
				require.True(t, p.r.routePresent(pending))
				require.Contains(t, p.r.sourcePolicy, uint64(1701))
				require.Contains(t, p.r.policyDenied, uint64(1701))
				if reconnect {
					p.state(1701, 43, true)
					fresh := p.receive(1701, 43, 76, 3)
					require.True(t, p.r.routePresent(fresh))
					decision.ConnectionID = 43
					decision.Prefix = "10.77.5.0/24"
					decision.MsgID = 77
					result, err = p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
					require.NoError(t, err)
					require.Equal(t, 1, result.Early)
				}
				terminalAdjState(t, p, 1701)
				require.False(t, p.r.routePresent(old))
				require.False(t, p.r.routePresent(pending))
				for source := range p.r.ribIn {
					require.NotEqual(t, uint64(1701), source.owner)
				}
				for key := range p.r.pending {
					require.NotEqual(t, uint64(1701), key.Peer.owner)
				}
				for key := range p.r.earlyDecisions {
					require.NotEqual(t, uint64(1701), key.Peer.owner)
				}
				for key := range p.r.validationPeers {
					require.NotEqual(t, uint64(1701), key.source.owner)
				}
				require.NotContains(t, p.r.sourceLive, uint64(1701))
				require.NotContains(t, p.r.sourcePolicy, uint64(1701))
				require.NotContains(t, p.r.policyDenied, uint64(1701))
				require.False(t, p.r.retentionVerdicts.Active(1701, 41))
				require.False(t, p.r.retentionVerdicts.Active(1701, 43))
				require.True(t, p.r.routeEligible(sibling, 72))
				require.Same(t, observedRoutes, p.r.ribIn[observedSource])
				require.Equal(t, 1, observedRoutes.Len())
				require.True(t, p.r.peerUp[p.address])
				require.Contains(t, p.r.sourcePolicy, uint64(1702))
				require.True(t, p.r.routePresent(siblingPending))
				require.Contains(t, p.r.earlyDecisions, siblingEarly)
				require.Error(t, p.policy(1701, rpc.SourcePolicyFinalize, "terminal", 0))
				terminalAdjState(t, p, 0)
				require.Same(t, observedRoutes, p.r.ribIn[observedSource], "an unowned terminal event cannot clear manual observations")
				require.Equal(t, 1, observedRoutes.Len())
				require.True(t, p.r.routeEligible(sibling, 72))
				terminalAdjState(t, p, 1701)
				require.True(t, p.r.routeEligible(sibling, 72), "duplicate terminal leaves no completed-owner history")
			})
		}
	}
}

// A decision may precede a live source's UPDATE, but cannot recreate early
// state after physical or terminal cleanup. Retained original bytes continue
// to accept cache revalidation while an independent same-address source works.
// MUTATION: omit sourceDecisionAdmittedLocked to recreate early dead-source debt.
func TestAdjLateDecisionCannotReopenRetiredSourceBothRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(fmt.Sprintf("json-%t", jsonRail), func(t *testing.T) {
			p := newAdjOwnerProducer(t, jsonRail)
			p.state(1701, 41, true)
			p.state(1702, 42, true)
			sibling := p.receive(1702, 42, 72, 0)
			p.r.validationEnabled = true
			decision := rpc.ValidationDecision{PeerAddr: p.address.String(), PeerOwner: 1701, ConnectionID: 41,
				Family: "ipv4/unicast", Prefix: "10.77.0.0/24", MsgID: 71, Accept: true, ValState: ValidationValid}
			result, err := p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
			require.NoError(t, err)
			require.Equal(t, 1, result.Early)
			old := p.receive(1701, 41, 71, 0)
			require.True(t, p.r.routeEligible(old, 71))
			payload, err := retention.Encode(1701, 41, p.address.String(), []family.Family{family.IPv4Unicast})
			require.NoError(t, err)
			matched, err := p.r.handleRetainedValidationEvent(retainedAdjEnvelope(t, retention.EventType, payload))
			require.True(t, matched)
			require.NoError(t, err)
			p.state(1701, 41, false)
			decision.Accept = false
			decision.Ineligible = true
			decision.ValState = ValidationInvalid
			_, err = p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
			require.NoError(t, err)
			require.True(t, p.r.routePresent(old))
			require.False(t, p.r.routeEligible(old, 71))
			decision.Accept = true
			decision.Ineligible = false
			decision.ValState = ValidationValid
			_, err = p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
			require.NoError(t, err)
			require.True(t, p.r.routeEligible(old, 71))
			decision.Prefix = "10.77.9.0/24"
			decision.MsgID = 79
			result, err = p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
			require.NoError(t, err)
			require.Zero(t, result.Early)
			terminalAdjState(t, p, 1701)
			for _, message := range []uint64{71, 80, 0} {
				decision.Prefix = "10.77.0.0/24"
				decision.MsgID = message
				result, err = p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
				require.NoError(t, err)
				require.Zero(t, result.Early)
			}
			for key := range p.r.earlyDecisions {
				require.NotEqual(t, uint64(1701), key.Peer.owner)
			}
			require.False(t, p.r.routePresent(old))
			require.True(t, p.r.routeEligible(sibling, 72))
			p.state(1703, 43, true)
			decision.PeerOwner = 1703
			decision.ConnectionID = 43
			decision.MsgID = 81
			result, err = p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
			require.NoError(t, err)
			require.Equal(t, 1, result.Early)
			fresh := p.receive(1703, 43, 81, 0)
			require.True(t, p.r.routeEligible(fresh, 81))
			p.state(1703, 43, false)
			result, err = p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
			require.NoError(t, err)
			require.Zero(t, result.Early)
			require.False(t, p.r.routePresent(fresh))
			require.True(t, p.r.routeEligible(sibling, 72))
		})
	}
}

// The JSON snapshot and the operator's same-address aggregate remain valid
// after cleanup; every emitted original identity uses the existing uint helper.
func TestAdjTerminalSnapshotKeepsUsefulOriginalIDs(t *testing.T) {
	p := newAdjOwnerProducer(t, false)
	p.state(1701, 41, true)
	p.state(1702, 42, true)
	p.receive(1701, 41, 71, 0)
	p.receive(1702, 42, 72, 0)
	terminalAdjState(t, p, 1701)
	_, snapshot, err := p.r.handleCommand("request bgp adj-rib-in enable-validation", []string{"refresh"}, "")
	require.NoError(t, err)
	encoded, err := json.Marshal(snapshot)
	require.NoError(t, err)
	var value struct {
		Routes []struct {
			Owner      string `json:"peer-owner"`
			Connection string `json:"connection-id"`
		} `json:"routes"`
	}
	require.NoError(t, json.Unmarshal(encoded, &value))
	require.Len(t, value.Routes, 1)
	require.Equal(t, "1702", value.Routes[0].Owner)
	require.Equal(t, "42", value.Routes[0].Connection)
}

// Design: docs/architecture/plugin/rib-storage-design.md -- RPKI gate configuration
package adj_rib_in

import (
	"encoding/json"
	"net/netip"
	"time"

	bgp "github.com/ze-software/ze/internal/component/bgp"
)

// validationPeer is captured once per peer, not once per prefix. A config
// revalidation must use the same peer identity as the retained wire attributes.
// In particular, a dynamic group's name and the iBGP/external distinction are
// not recoverable from a prefix or from a flattened AS_PATH.
type validationPeer struct {
	Name    string
	Group   string
	ASN     uint32
	LocalAS uint32
}

func (r *AdjRIBInManager) rememberValidationPeer(address netip.Addr, peer validationPeer) {
	if r.validationPeers == nil {
		r.validationPeers = make(map[netip.Addr]validationPeer)
	}
	r.validationPeers[address] = peer
}

func (r *AdjRIBInManager) validationRouteData(peer netip.Addr, key compactRouteKey, route *RawRoute) map[string]any {
	identity := r.validationPeers[peer]
	return map[string]any{
		"peer": peer.String(), "peer-name": identity.Name, "peer-group": identity.Group,
		"peer-as": identity.ASN, "local-as": identity.LocalAS,
		routeKeyFamily: route.Family.String(), "prefix": key.Prefix.String(),
		routeKeyAttrHex: route.AttrHex, routeKeyNHopHex: route.NHopHex, routeKeyNLRIHex: route.NLRIHex,
		routeKeyValidationState: route.ValidationState, routeKeyIneligible: route.Ineligible,
		"path-id": route.PathID, "msg-id": route.MsgID,
	}
}

// suspendForRevalidation moves only RPKI-owned families to pending and returns
// their authoritative received data in the same locked operation. Import,
// FlowSpec and export eligibility remain owned by their independent producers.
func (r *AdjRIBInManager) suspendForRevalidation() []map[string]any {
	var snapshots []map[string]any
	now := time.Now()
	for peer, routes := range r.ribIn {
		var pending []*pendingRoute
		routes.Range(func(key compactRouteKey, _ uint64, route *RawRoute) bool {
			if !isSimplePrefixFamily(route.Family) {
				return true
			}
			pending = append(pending, &pendingRoute{
				peerAddr: peer, family: route.Family, prefix: key.Prefix.String(), routeKey: key,
				route: route, receivedAt: now, state: ValidationPending,
			})
			return true
		})
		for _, route := range pending {
			r.removeInstalled(peer, route.routeKey)
			r.pending[pendingKey(peer, route.routeKey)] = route
		}
	}
	for _, route := range r.pending {
		if isSimplePrefixFamily(route.family) {
			snapshots = append(snapshots, r.validationRouteData(route.peerAddr, route.routeKey, route.route))
		}
	}
	return snapshots
}

// disableValidationCommand removes the RPKI contribution, not the independent
// authorization checks enforced by selection and forwarding. A decision for a
// newer UPDATE remains a generation fence until that UPDATE arrives: disabling
// a validator must not resurrect its already superseded predecessor.
func (r *AdjRIBInManager) disableValidationCommand() (string, any, error) {
	r.mu.Lock()
	defer r.unlockValidation()
	r.validationEnabled = false
	for key, pending := range r.pending {
		if !isSimplePrefixFamily(pending.family) {
			continue
		}
		pending.route.Ineligible = false
		r.promoteToInstalled(pending, ValidationNotValidated)
		delete(r.pending, key)
	}
	for peer, routes := range r.ribIn {
		var recovered []compactRouteKey
		routes.Range(func(key compactRouteKey, _ uint64, route *RawRoute) bool {
			if !isSimplePrefixFamily(route.Family) {
				return true
			}
			newer := r.earlyDecisions[pendingKey(peer, key)]
			blocked := newer != nil && newer.msgID > route.MsgID
			if route.Ineligible && !blocked {
				recovered = append(recovered, key)
			}
			if route.Ineligible != blocked || route.ValidationState != ValidationNotValidated {
				route.Ineligible = blocked
				route.ValidationState = ValidationNotValidated
				r.noteValidationChange(peer, key)
			}
			return true
		})
		for _, key := range recovered {
			route, _ := routes.Get(key)
			r.seqCounter++
			routes.Put(key, r.seqCounter, route)
		}
	}
	for key, decision := range r.earlyDecisions {
		if routes := r.ribIn[key.PeerAddr]; routes != nil {
			if route, ok := routes.Get(key.Route); ok && decision.msgID > route.MsgID {
				decision.action = earlyAccept
				decision.state = ValidationNotValidated
				continue
			}
		}
		delete(r.earlyDecisions, key)
	}
	return statusDone, map[string]any{"validation-enabled": false}, nil
}

func validationPeerFromEvent(event *bgp.Event) validationPeer {
	peer := validationPeer{Name: event.GetPeerName(), Group: event.GetPeerGroup(), ASN: event.GetPeerASN()}
	// The received JSON representation carries local and remote identity in
	// one object; parsing it here also preserves dynamic-group membership.
	var identity bgp.PeerInfoJSON
	if err := json.Unmarshal(event.Peer, &identity); err == nil && identity.Local != nil {
		peer.LocalAS = identity.Local.AS
	}
	return peer
}

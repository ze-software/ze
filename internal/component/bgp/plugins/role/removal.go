// Design: docs/architecture/meta/role.md -- learned role lifecycle
// Related: role.go -- forgetRemovedPeer, the learned-role store it clears

package role

import (
	"encoding/json"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// handleRoleStructuredEvents takes the state events an in-process plugin is
// delivered: a down whose reason is rpc.ReasonPeerRemoved frees the removed
// peer's learned role. The reactor emits that down for every removal,
// established or not (reactor removePeer).
//
// The plugin subscribes to state (RunRolePlugin). Its registered OPEN observer
// receives removals independently of filter membership, so observations survive
// inactive source-policy intervals without losing their cleanup owner. The
// subscription and its peer selector still intersect that removal-only grant.
//
// The down is delivered on the event loop and validate-open on the callback
// loop, so a down can still be queued when a peer re-created under the same
// name validates its OPEN. The down's session generation orders the two: the
// reactor issues it as it deletes the peer (doRemovePeer), and the new peer's
// OPEN is stamped with a generation at or above it
// (rpc.ValidateOpenInput.Generation), so forgetRemovedPeer keeps the role that
// OPEN stored (docs/architecture/meta/role.md, "Learned role lifecycle").
func handleRoleStructuredEvents(events []any) error {
	for _, event := range events {
		se, ok := event.(*rpc.StructuredEvent)
		if !ok {
			continue
		}
		if se.State != rpc.SessionStateDown || se.Reason != rpc.ReasonPeerRemoved {
			continue
		}
		forgetRemovedPeer(removedPeerInfo(se.PeerAddress, se.PeerName), se.SessionGeneration)
	}
	return nil
}

// roleStateEvent is the part of a JSON state event a forked plugin reads.
type roleStateEvent struct {
	BGP struct {
		Message struct {
			Type string `json:"type"`
		} `json:"message"`
		Peer struct {
			Name   string `json:"name"`
			Remote struct {
				Address string `json:"address"`
			} `json:"remote"`
		} `json:"peer"`
		State             string `json:"state"`
		Reason            string `json:"reason"`
		SessionGeneration uint64 `json:"session-generation,string"`
	} `json:"bgp"`
}

// handleRoleEvent is handleRoleStructuredEvents for a forked plugin, which
// is delivered the event as JSON.
func handleRoleEvent(event string) error {
	var parsed roleStateEvent
	if err := json.Unmarshal([]byte(event), &parsed); err != nil {
		logger().Warn("bgp-role: an event cannot be read, a removed peer's learned role may stay", "error", err)
		return nil
	}
	bgp := parsed.BGP
	if bgp.Message.Type != "state" {
		return nil
	}
	if bgp.State != rpc.SessionStateDown.String() || bgp.Reason != rpc.ReasonPeerRemoved {
		return nil
	}
	forgetRemovedPeer(removedPeerInfo(bgp.Peer.Remote.Address, bgp.Peer.Name), bgp.SessionGeneration)
	return nil
}

// removedPeerInfo builds the identity forgetRemovedPeer clears. An address
// that does not parse leaves only the name, which keys every role an OPEN of
// a named peer stored.
func removedPeerInfo(address, name string) filterapi.PeerFilterInfo {
	addr, err := netip.ParseAddr(address)
	if err != nil {
		logger().Warn("bgp-role: a removed peer's address cannot be read, only its name is freed",
			"peer", name, "address", address)
	}
	return filterapi.PeerFilterInfo{Address: addr, Name: name}
}

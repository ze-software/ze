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
// The plugin subscribes to state (RunRolePlugin), and the implicit state grant
// of a filter plugin delivers it for exactly the peers whose filter lists carry
// a bgp-role member: the peers whose config sets role import. Those are the
// only peers applyValidateOpen stores a learned role for, so every stored role
// has a removal that reaches it, on either transport.
//
// The down is delivered on the event loop and validate-open on the callback
// loop, and the down carries no session identity, so a down that is still
// queued when a peer re-created under the same name validates its OPEN
// deletes that new peer's role. A stale role never judges a route: a removed
// peer sends none, and a re-created one stores its own role at its OPEN
// before it can carry one. The early delete changes no RFC 9234 Section 5
// verdict either: the peer is role-configured, and its learned role, once
// its session passed validateOpenRolePair, is the configured complement
// resolvePeerRole falls back to. It changes the export set, which reads the
// capability alone and sees the peer as unknown until its next OPEN
// (docs/architecture/meta/role.md, "Learned role lifecycle").
func handleRoleStructuredEvents(events []any) error {
	for _, event := range events {
		se, ok := event.(*rpc.StructuredEvent)
		if !ok {
			continue
		}
		if se.State != rpc.SessionStateDown || se.Reason != rpc.ReasonPeerRemoved {
			continue
		}
		forgetRemovedPeer(removedPeerInfo(se.PeerAddress, se.PeerName))
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
		State  string `json:"state"`
		Reason string `json:"reason"`
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
	forgetRemovedPeer(removedPeerInfo(bgp.Peer.Remote.Address, bgp.Peer.Name))
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

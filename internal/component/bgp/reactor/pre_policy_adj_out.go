// Design: docs/guide/bmp.md -- RFC 8671 Section 5.2 pre-policy Adj-RIB-Out emission
// RFC: rfc/short/rfc8671.md -- Section 5.2 (Pre-policy Adj-RIB-Out candidates before export policy)
// Related: reactor_api_forward.go -- forwardUpdateSection
// Related: forward_rs.go -- reactorForwardRSSection
// Related: egress_inject_filter.go -- exportFilterFromFacts

package reactor

import (
	"time"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// prePolicyNextHopUnknownForFacts reports whether the destination's outbound
// next hop is rewritten at transmission time (post-policy), so at pre-policy
// phase completion the final next hop is unknown (RFC 8671 Section 5.2).
func prePolicyNextHopUnknownForFacts(facts *peerForwardFacts) bool {
	if facts == nil {
		return false
	}
	if facts.nhSelfWithheld {
		return true
	}
	return facts.nhMode != nhModeNone
}

// prePolicyNextHopUnknownInBody reports whether an originated/injected UPDATE
// body carries an unknown (missing or all-zero) NEXT_HOP or MP_REACH_NLRI next
// hop at pre-policy phase completion (RFC 8671 Section 5.2).
func prePolicyNextHopUnknownInBody(body []byte) bool {
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return false
	}
	attrs := sections.Attrs(body)
	unknown := false
	if sections.NLRILen(body) > 0 {
		_, _, nh, found := attribute.AttrFind(attrs, attribute.AttrNextHop)
		if !found || len(nh) != 4 || (nh[0] == 0 && nh[1] == 0 && nh[2] == 0 && nh[3] == 0) {
			unknown = true
		}
	}
	if !unknown {
		if _, _, mpRaw, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI); found {
			if len(mpRaw) < 4 {
				unknown = true
			} else {
				nhLen := int(mpRaw[3])
				if nhLen == 0 || 4+nhLen > len(mpRaw) {
					unknown = true
				} else {
					allZero := true
					for _, b := range mpRaw[4 : 4+nhLen] {
						if b != 0 {
							allZero = false
							break
						}
					}
					if allZero {
						unknown = true
					}
				}
			}
		}
	}
	return unknown
}

// hasPrePolicyAdjRIBOutSubscriber checks with zero allocations whether any
// plugin (such as bgp-bmp) subscribes to update-pre-policy-out for this peer.
func (r *Reactor) hasPrePolicyAdjRIBOutSubscriber(peerAddr, peerName string) bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	receiver := r.messageReceiver
	r.mu.RUnlock()
	if receiver == nil {
		return false
	}
	if r.api == nil {
		return true
	}
	ns := events.LookupNamespaceID(bgpevents.Namespace)
	et := events.LookupEventTypeID(bgpevents.EventUpdatePrePolicyOut)
	return len(r.api.PeerScopedProcs(ns, et, events.DirSent, peerAddr, peerName)) > 0
}

// emitPrePolicyAdjRIBOut delivers a destination peer's pre-policy Adj-RIB-Out
// candidate UPDATE (before export policy filtering/modification and before
// post-policy transmission next-hop rewrite) on EventKindUpdatePrePolicyOut.
//
// RFC 8671 Section 5.2: "Pre-policy Adj-RIB-Out conveys only routes that are
// available based on the peering type. Post-policy represents the
// filtered/changed routes from the available routes."
func (r *Reactor) emitPrePolicyAdjRIBOut(peer *Peer, facts *peerForwardFacts, body []byte, ctxID bgpctx.ContextID, nextHopUnknown bool) {
	if peer == nil {
		return
	}
	if len(body) == 0 {
		return
	}
	s := peer.Settings()
	peerAddr := peer.addrString
	if peerAddr == "" && s != nil && s.Address.IsValid() {
		peerAddr = s.Address.String()
	}
	peerName := ""
	if s != nil {
		peerName = s.Name
	}
	if !r.hasPrePolicyAdjRIBOutSubscriber(peerAddr, peerName) {
		return
	}
	r.mu.RLock()
	receiver := r.messageReceiver
	r.mu.RUnlock()
	if receiver == nil {
		return
	}
	peerAS := peer.PeerAS()
	if peerAS == 0 && facts != nil {
		peerAS = facts.peerAS
	}
	localAS := uint32(0)
	if facts != nil {
		localAS = facts.localAS
	} else if s != nil {
		localAS = s.LocalAS
	}
	peerInfo := plugin.PeerInfo{
		PeerOwner:       peer.sourceOwner,
		AddressStr:      peerAddr,
		LocalAddressStr: peer.localAddrString,
		Name:            peerName,
		LocalAS:         localAS,
		PeerAS:          rpc.RemoteASOf(peerAS),
		RemoteRouterID:  peer.RemoteRouterID(),
		State:           peer.State().PluginState(),
	}
	if s != nil {
		peerInfo.Address = s.Address
		peerInfo.LocalAddress = s.LocalAddress
		peerInfo.GroupName = s.GroupName
		peerInfo.RouterID = s.RouterID
		peerInfo.IsIBGP = s.isIBGPWith(peerAS)
		peerInfo.RSClient = s.RSClient
		peerInfo.RouteReflectorClient = s.RouteReflectorClient
	} else if facts != nil {
		peerInfo.IsIBGP = !facts.isEBGP
		peerInfo.RSClient = facts.rsClient
		peerInfo.RouteReflectorClient = facts.rrClient
	}
	bodyCopy := append([]byte(nil), body...)
	rawMsg := bgptypes.RawMessage{
		Type:                    msgtype.TypeUPDATE,
		RawBytes:                bodyCopy,
		WireUpdate:              wireu.NewWireUpdate(bodyCopy, ctxID),
		Direction:               rpc.DirectionSent,
		Timestamp:               time.Now(),
		PrePolicyOut:            true,
		PrePolicyNextHopUnknown: nextHopUnknown,
	}
	receiver.OnMessageSent(&peerInfo, rawMsg)
}

// Design: docs/architecture/core-design.md — peer forwarding facts precomputation
// RFC: rfc/short/rfc2545.md
// Related: peer.go — Peer struct, lifecycle methods
// Related: reactor_api_forward.go — ForwardUpdate egress pipeline
// Related: forward_rs.go — reactorForwardRS egress pipeline
package reactor

import (
	"encoding/binary"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/network"
)

const (
	nhModeNone   uint8 = iota // No next-hop ops (Auto or Unchanged)
	nhModeSelf4               // Self IPv4: legacy NEXT_HOP + mapped MP_REACH
	nhModeSelfV6              // Self IPv6: MP_REACH global only
	// nhModeSelfV6LL is the RFC 2545 Section 3 two-address form: MP_REACH carries
	// the global address then the link-local one. applyLinkLocalNextHop
	// (link_scope.go) raises BOTH nhModeSelfV6 and nhModeExplicitV6 to it, so the
	// "Self" in the name is historical: what it names is the wire form, and the
	// global address it carries is already in nhGlobal either way.
	nhModeSelfV6LL
	nhModeExplicit4  // Explicit IPv4: legacy NEXT_HOP + mapped MP_REACH
	nhModeExplicitV6 // Explicit IPv6: MP_REACH global only
)

type sendCommunityMask uint8

const (
	scSuppressStandard sendCommunityMask = 1 << iota
	scSuppressExtended
	scSuppressLarge
)

// peerForwardFacts holds precomputed per-peer forwarding decisions.
// Built at session lifecycle boundaries, read on every UPDATE iteration.
// Stored via atomic.Pointer on Peer; nil means not established.
type peerForwardFacts struct {
	addr netip.Addr
	// localAddr is Ze's own address on this session, held for ONE question:
	// sessionEndsShareOneAddress (forward_next_hop.go).
	localAddr netip.Addr
	peerKey   netip.AddrPort
	addrStr   string

	localAS       uint32
	globalLocalAS uint32
	peerAS        uint32
	isEBGP        bool

	rsClient         bool
	rrClient         bool
	asOverride       bool
	localASNoPrepend bool
	localASReplaceAS bool
	name             string
	groupName        string
	exportFilters    []filterapi.FilterRef

	clusterID      uint32
	clusterIDBytes [4]byte

	sendCtxID   bgpctx.ContextID
	sendASN4    bool
	extendedMsg bool
	maxMsgSize  int

	filterInfo filterapi.PeerFilterInfo

	secondaryAS uint32

	nhMode     uint8
	nhLegacy   [4]byte
	nhMapped   [16]byte
	nhGlobal   [16]byte
	nhGlobalLL [32]byte

	scMask sendCommunityMask
}

func (p *Peer) forwardFacts() *peerForwardFacts {
	return p.fwdFacts.Load()
}

// refreshForwardFacts builds and stores a new forwarding facts snapshot.
// Called at: setEncodingContexts (after unlock), resolveDynamicPeerSettings.
//
// The link scope is re-read first, because buildForwardFacts precomputes the
// next-hop wire form and RFC 2545 Section 3 decides that form against the host
// interface table (link_scope.go).
func (p *Peer) refreshForwardFacts() {
	p.refreshLinkScope()
	p.fwdFacts.Store(p.buildForwardFacts())
}

// refreshForwardFactsIfLive rebuilds the snapshot only for a peer that already has
// one, and only if nobody replaced it meanwhile.
//
// A nil snapshot is the "this peer has no session" gate on the forwarding rails
// (reactorForwardRS skips a peer whose forwardFacts() is nil, forward_rs.go), so
// storing one unconditionally from the config-reload goroutine would make a down
// peer look established. The compare-and-swap covers the other direction:
// clearEncodingContexts stores nil on teardown (peer.go), and a plain store racing
// it would resurrect the snapshot for a session that has gone.
func (p *Peer) refreshForwardFactsIfLive() {
	p.refreshForwardFactsIfLiveFrom(network.ConnectedPrefixes())
}

// refreshForwardFactsIfLiveFrom is refreshForwardFactsIfLive against an interface
// table the caller has already read, for a fan-out over many peers
// (refreshPeerLinkScopes, reactor_iface.go).
func (p *Peer) refreshForwardFactsIfLiveFrom(connected []netip.Prefix) {
	for {
		previous := p.fwdFacts.Load()
		if previous == nil {
			return
		}
		p.refreshLinkScopeFrom(connected)
		if p.fwdFacts.CompareAndSwap(previous, p.buildForwardFacts()) {
			return
		}
	}
}

// buildForwardFacts computes the snapshot. It reads the peer's settings but stores
// nothing, so the caller decides whether publishing it is correct.
func (p *Peer) buildForwardFacts() *peerForwardFacts {
	s := p.settings

	// ExportFilters is read under p.mu because it is one of the mutable fields:
	// resolveDynamicPeerSettings (reactor_dynamic.go) and applyHotSwappableSettings
	// (peer_settings_apply.go) both write it on the pointed-to struct under this
	// lock. Every other field read below is set at construction and never mutated
	// (the contract on Peer.Settings, peer.go).
	p.mu.RLock()
	sendCtxID := p.sendCtxID
	exportFilters := s.ExportFilters
	p.mu.RUnlock()

	ctx := p.sendCtx.Load()
	nc := p.negotiated.Load()

	sendASN4 := true
	if ctx != nil {
		sendASN4 = ctx.ASN4()
	}

	extendedMsg := nc != nil && nc.ExtendedMessage

	facts := &peerForwardFacts{
		addr:      s.Address,
		localAddr: s.LocalAddress,
		peerKey:   s.PeerKey(),
		addrStr:   p.addrString,

		localAS:       s.LocalAS,
		globalLocalAS: s.GlobalLocalAS,
		peerAS:        s.PeerAS,
		isEBGP:        s.IsEBGP(),

		rsClient:         s.RSClient,
		rrClient:         s.RouteReflectorClient,
		asOverride:       s.ASOverride,
		localASNoPrepend: s.LocalASNoPrepend,
		localASReplaceAS: s.LocalASReplaceAS,
		name:             s.Name,
		groupName:        s.GroupName,
		exportFilters:    exportFilters,

		sendCtxID:   sendCtxID,
		sendASN4:    sendASN4,
		extendedMsg: extendedMsg,
		maxMsgSize:  int(message.MaxMessageLength(msgtype.TypeUPDATE, extendedMsg)),

		filterInfo: filterapi.PeerFilterInfo{
			Address: s.Address,
			PeerAS:  s.PeerAS,
			// LocalAS is the effective per-peer local AS (per-peer local-as
			// override when set, otherwise the global local-as). Filling it here
			// lets egress filters read dest.LocalAS instead of re-parsing the raw
			// config JSON: role/OTC stamps it (RFC 9234 R008) and gr/LLGR uses it
			// for iBGP detection (RFC 9494 Section 4.5.3). The readvertise rail
			// already fills the same field from peer.Settings().LocalAS
			// (reactor_api_batch.go), so both egress rails agree.
			LocalAS:   s.LocalAS,
			Name:      s.Name,
			GroupName: s.GroupName,
		},
	}

	facts.clusterID = s.effectiveClusterID()
	binary.BigEndian.PutUint32(facts.clusterIDBytes[:], facts.clusterID)

	if s.GlobalLocalAS != 0 && s.GlobalLocalAS != s.LocalAS &&
		!s.LocalASNoPrepend && !s.LocalASReplaceAS {
		facts.secondaryAS = s.GlobalLocalAS
	}

	precomputeNextHop(s, facts)
	applyLinkLocalNextHop(s, facts, p.llScope.Load())
	precomputeSendCommunity(s, facts)

	return facts
}

// precomputeNextHop fixes the next-hop wire form this peer will send, from
// config alone. An IPv6 next hop lands on the single-address form here;
// applyLinkLocalNextHop then decides whether RFC 2545 Section 3 puts a second
// address beside it.
func precomputeNextHop(s *PeerSettings, f *peerForwardFacts) {
	switch s.NextHopMode {
	case NextHopAuto, NextHopUnchanged:
		f.nhMode = nhModeNone
	case NextHopSelf:
		if !s.LocalAddress.IsValid() {
			f.nhMode = nhModeNone
			return
		}
		local := s.LocalAddress.Unmap()
		switch {
		case local.Is4():
			f.nhMode = nhModeSelf4
			f.nhLegacy = local.As4()
			f.nhMapped = local.As16()
		default:
			f.nhMode = nhModeSelfV6
			f.nhGlobal = local.As16()
		}
	case NextHopExplicit:
		if !s.NextHopAddress.IsValid() {
			f.nhMode = nhModeNone
			return
		}
		explicit := s.NextHopAddress.Unmap()
		if explicit.Is4() {
			f.nhMode = nhModeExplicit4
			f.nhLegacy = explicit.As4()
			f.nhMapped = explicit.As16()
		} else {
			f.nhMode = nhModeExplicitV6
			f.nhGlobal = explicit.As16()
		}
	}
}

func precomputeSendCommunity(s *PeerSettings, f *peerForwardFacts) {
	if len(s.SendCommunity) == 0 {
		return
	}
	sendStandard, sendLarge, sendExtended := false, false, false
	for _, v := range s.SendCommunity {
		switch v {
		case "all":
			return
		case "none":
			f.scMask = scSuppressStandard | scSuppressExtended | scSuppressLarge
			return
		case "standard":
			sendStandard = true
		case "large":
			sendLarge = true
		case "extended":
			sendExtended = true
		}
	}
	if !sendStandard {
		f.scMask |= scSuppressStandard
	}
	if !sendExtended {
		f.scMask |= scSuppressExtended
	}
	if !sendLarge {
		f.scMask |= scSuppressLarge
	}
}

func applyFactsNextHop(f *peerForwardFacts, mods *filterapi.ModAccumulator) {
	switch f.nhMode {
	case nhModeNone:
		return
	case nhModeSelf4, nhModeExplicit4:
		mods.Op(3, filterapi.AttrModSet, f.nhLegacy[:])
		mods.Op(14, filterapi.AttrModSet, f.nhMapped[:])
	case nhModeSelfV6, nhModeExplicitV6:
		mods.Op(14, filterapi.AttrModSet, f.nhGlobal[:])
	case nhModeSelfV6LL:
		mods.Op(14, filterapi.AttrModSet, f.nhGlobalLL[:])
	}
	// RFC 9252 Section 3.3: when next-hop is changed, strip PrefixSID.
	// Ze does not originate local SRv6 SIDs, so the correct behavior is
	// to remove the attribute rather than rebuild with a local SID.
	mods.Op(40, filterapi.AttrModSuppress, nil)
}

func applyFactsSendCommunity(f *peerForwardFacts, mods *filterapi.ModAccumulator) {
	if f.scMask == 0 {
		return
	}
	if f.scMask&scSuppressStandard != 0 {
		mods.Op(8, filterapi.AttrModSuppress, nil)
	}
	if f.scMask&scSuppressExtended != 0 {
		mods.Op(16, filterapi.AttrModSuppress, nil)
	}
	if f.scMask&scSuppressLarge != 0 {
		mods.Op(32, filterapi.AttrModSuppress, nil)
	}
}

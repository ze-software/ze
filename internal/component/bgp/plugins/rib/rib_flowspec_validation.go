// Design: docs/architecture/plugin/rib-storage-design.md -- live FlowSpec validation
// RFC: rfc/short/rfc8955.md -- Section 6, revised by RFC 9117 Section 4
package rib

import (
	"bytes"
	"encoding/binary"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
)

var flowSpecFamilies = [...]family.Family{
	{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec},
	{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec},
	{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpecVPN},
	{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpecVPN},
}

type flowSpecDomain struct {
	family family.Family
	rd     [8]byte
}

type flowSpecRule struct {
	family family.Family
	nlri   string
}

type flowSpecVerdict struct {
	msgID    uint64
	eligible bool
}

type flowSpecWinner struct {
	key             ribevents.ValidationRoute
	communities     []byte
	ipv6Communities []byte
	change          bestChangeEntry
}

// Published snapshots contain only values and owned bytes, never pool handles.
// Their size is bounded by the routes currently retained in bgpPeers.
type flowSpecState struct {
	paths map[ribevents.ValidationRoute]flowSpecVerdict
	best  map[flowSpecRule]flowSpecWinner
}

type flowSpecRoute struct {
	key         ribevents.ValidationRoute
	domain      flowSpecDomain
	destination netip.Prefix
	originator  netip.Addr
	firstAS     uint32
	localDomain bool
	validPath   bool
	candidate   *Candidate
	entry       storage.RouteEntry
}

type flowSpecUnicast struct {
	domain     flowSpecDomain
	prefix     netip.Prefix
	originator netip.Addr
	firstAS    uint32
	candidate  *Candidate
}

func flowSpecKey(peer netip.Addr, fam family.Family, raw []byte, addPath bool) (ribevents.ValidationRoute, bool) {
	key := ribevents.ValidationRoute{Peer: peer, Family: fam}
	if addPath {
		if len(raw) < 4 {
			return key, false
		}
		key.PathID = binary.BigEndian.Uint32(raw[:4])
		raw = raw[4:]
	}
	key.NLRI = ribevents.FlowSpecKey(raw)
	return key, key.NLRI != ""
}

// flowSpecDestination uses the registered component codec. An IPv6 destination
// with a nonzero offset cannot authorize a contiguous destination prefix.
func flowSpecDestination(fam family.Family, raw []byte) (flowSpecDomain, netip.Prefix) {
	domain := flowSpecDomain{family: family.Family{AFI: fam.AFI, SAFI: family.SAFIUnicast}}
	var components []flowspec.FlowComponent
	if fam.SAFI == family.SAFIFlowSpecVPN {
		domain.family.SAFI = family.SAFIVPN
		flow, err := flowspec.ParseFlowSpecVPN(fam, raw)
		if err != nil {
			return domain, netip.Prefix{}
		}
		rd := flow.RD()
		rd.WriteTo(domain.rd[:], 0)
		components = flow.Components()
	} else {
		flow, err := flowspec.ParseFlowSpec(fam, raw)
		if err != nil {
			return domain, netip.Prefix{}
		}
		components = flow.Components()
	}
	for _, component := range components {
		if component.Type() != flowspec.FlowDestPrefix {
			continue
		}
		prefix, ok := component.(interface {
			Prefix() netip.Prefix
			Offset() uint8
		})
		if ok && prefix.Offset() == 0 {
			return domain, prefix.Prefix().Masked()
		}
	}
	return domain, netip.Prefix{}
}

func flowSpecUnicastPrefix(fam family.Family, raw []byte, addPath bool) (flowSpecDomain, netip.Prefix) {
	domain := flowSpecDomain{family: fam}
	if addPath {
		if len(raw) < 4 {
			return domain, netip.Prefix{}
		}
		raw = raw[4:]
	}
	if fam.SAFI == family.SAFIUnicast {
		_, prefix, ok := parsePrevKey(fam, raw, false)
		if ok {
			return domain, prefix
		}
		return domain, netip.Prefix{}
	}
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	key, err := nlrisplit.GetPrefixKey(fam)(raw, scratch[:], false)
	if err != nil || len(key) < 9 || key[0] < 64 {
		return domain, netip.Prefix{}
	}
	copy(domain.rd[:], key[1:9])
	var cidr [17]byte
	cidr[0] = key[0] - 64
	if len(key)-9 > len(cidr)-1 {
		return domain, netip.Prefix{}
	}
	n := copy(cidr[1:], key[9:])
	_, prefix, ok := parsePrevKey(family.Family{AFI: fam.AFI, SAFI: family.SAFIUnicast}, cidr[:1+n], false)
	if !ok {
		return domain, netip.Prefix{}
	}
	return domain, prefix
}

// flowSpecOriginator deliberately uses the transport address as fallback.
// Candidate.OriginatorIP serves BGP tie-breaking and falls back to Router ID.
func flowSpecOriginator(peer netip.Addr, entry storage.RouteEntry) netip.Addr {
	bundle := entry.GetBundle()
	if !bundle.HasOriginatorID() {
		return peer
	}
	raw, err := pool.OriginatorID.Get(bundle.OriginatorID)
	if err != nil || len(raw) != 4 {
		return netip.Addr{}
	}
	return netip.AddrFrom4([4]byte(raw))
}

// flowSpecASPath reads the canonical four-octet AS_PATH. Empty/confederation-only
// paths originate in the Local Domain (RFC 9117 Sections 2 and 4.1). AS_SET has
// no leftmost AS_SEQUENCE ASN and cannot satisfy the external-path comparison.
func flowSpecASPath(entry storage.RouteEntry) (first uint32, local, valid bool) {
	if !entry.HasASPath() {
		return 0, false, false
	}
	raw, err := pool.ASPath.Get(entry.ASPath)
	if err != nil {
		return 0, false, false
	}
	local = true
	firstSegment := true
	for off := 0; off < len(raw); {
		if len(raw)-off < 2 {
			return 0, false, false
		}
		kind, count := raw[off], int(raw[off+1])
		off += 2
		if count == 0 || count > (len(raw)-off)/4 {
			return 0, false, false
		}
		switch kind {
		case 1, 2:
			local = false
			if firstSegment {
				if kind == 2 {
					first = binary.BigEndian.Uint32(raw[off : off+4])
				}
				firstSegment = false
			}
		case 3, 4:
			// RFC 9117 Section 1 extends the local-domain treatment of
			// AS_CONFED_SEQUENCE to AS_CONFED_SET as well.
		default:
			return 0, false, false
		}
		off += count * 4
	}
	return first, local, true
}

// flowSpecAuthorized applies rules a-c and RFC 9117's external AS check to the
// current same-AFI unicast domain. Every received eligible more-specific path
// participates in rule c, including a path that lost unicast best selection.
func flowSpecAuthorized(flow *flowSpecRoute, unicast []flowSpecUnicast) bool {
	if !flow.destination.IsValid() || !flow.validPath {
		return false
	}
	var best *flowSpecUnicast
	for i := range unicast {
		u := &unicast[i]
		if u.domain != flow.domain || u.prefix.Bits() > flow.destination.Bits() || !u.prefix.Contains(flow.destination.Addr()) {
			continue
		}
		if best == nil || u.prefix.Bits() > best.prefix.Bits() || (u.prefix == best.prefix && ComparePair(u.candidate, best.candidate) < 0) {
			best = u
		}
	}
	if best == nil {
		return false
	}
	if !flow.localDomain && (!flow.originator.IsValid() || flow.originator != best.originator) {
		return false
	}
	// Confederation-only paths belong to RFC 9117's Local Domain even when
	// the transport session connects different member ASNs.
	external := flow.candidate.LocalASN == 0 || flow.candidate.PeerASN != flow.candidate.LocalASN
	if external && !flow.localDomain {
		if flow.firstAS == 0 || flow.firstAS != best.firstAS {
			return false
		}
	}
	// An empty external path does not establish membership in a confederation.
	if external && flow.localDomain && flow.candidate.ASPathLen == 0 {
		data, _ := pool.ASPath.Get(flow.entry.ASPath)
		if len(data) == 0 {
			return false
		}
	}
	for i := range unicast {
		u := &unicast[i]
		if u.domain == flow.domain && u.prefix.Bits() > flow.destination.Bits() && flow.destination.Contains(u.prefix.Addr()) && u.firstAS != best.firstAS {
			return false
		}
	}
	return true
}

func (r *RIBManager) flowSpecEligible(key ribevents.ValidationRoute, msgID uint64) bool {
	state := r.flowSpec.Load()
	if state == nil {
		return false
	}
	verdict, ok := state.paths[key]
	return ok && verdict.eligible && (msgID == 0 || verdict.msgID == msgID)
}

func (r *RIBManager) flowSpecPresent(key ribevents.ValidationRoute) bool {
	state := r.flowSpec.Load()
	if state == nil {
		return false
	}
	_, ok := state.paths[key]
	return ok
}

func (r *RIBManager) flowSpecPath(key ribevents.ValidationRoute) (ribevents.FlowSpecPath, bool) {
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()
	peer := r.bgpPeers[key.Peer]
	if peer == nil {
		return ribevents.FlowSpecPath{}, false
	}
	raw := []byte(key.NLRI)
	if peer.IsAddPath(key.Family) {
		wire := make([]byte, 4+len(raw))
		binary.BigEndian.PutUint32(wire[:4], key.PathID)
		copy(wire[4:], raw)
		raw = wire
	}
	entry, ok := peer.LookupRetained(key.Family, raw)
	if !ok {
		return ribevents.FlowSpecPath{}, false
	}
	defer entry.Release()
	attrs, err := entry.ToWireBytes()
	if err != nil {
		return ribevents.FlowSpecPath{}, false
	}
	return ribevents.FlowSpecPath{Attributes: attrs, MsgID: entry.MsgID}, true
}

// flowSpecIPv6Communities copies the selected twenty-octet action values out
// of pooled storage. Malformed or unavailable attributes cannot become a
// no-action rule at the firewall consumer.
func flowSpecIPv6Communities(entry storage.RouteEntry) ([]byte, bool) {
	bundle := entry.GetBundle()
	if !bundle.HasOtherAttrs() {
		return nil, true
	}
	data, err := pool.OtherAttrs.Get(bundle.OtherAttrs)
	if err != nil {
		return nil, false
	}
	for len(data) > 0 {
		if len(data) < 4 {
			return nil, false
		}
		code, length := data[0], int(binary.BigEndian.Uint16(data[2:4]))
		data = data[4:]
		if length > len(data) {
			return nil, false
		}
		if code == byte(attribute.AttrIPv6ExtCommunity) {
			if length%20 != 0 {
				return nil, false
			}
			return bytes.Clone(data[:length]), true
		}
		data = data[length:]
	}
	return nil, true
}

// reconcileFlowSpecs MUST be called after receive-store locks are released for
// a FlowSpec or unicast mutation. It snapshots retained paths under their owning
// locks, publishes eligibility atomically, then emits after releasing all locks.
// Work is bounded by stored routes and no unicast scan occurs without FlowSpec.
func (r *RIBManager) reconcileFlowSpecs() {
	r.flowSpecMu.Lock()
	r.peerMu.RLock()
	previous := r.flowSpec.Load()
	haveFlows := false
	for _, routes := range r.bgpPeers {
		for _, fam := range flowSpecFamilies {
			if routes.FamilyLen(fam) != 0 {
				haveFlows = true
				break
			}
		}
		if haveFlows {
			break
		}
	}
	if !haveFlows && (previous == nil || len(previous.paths) == 0) {
		r.peerMu.RUnlock()
		r.flowSpecMu.Unlock()
		return
	}
	var flows []flowSpecRoute
	for peer, routes := range r.bgpPeers {
		for _, fam := range flowSpecFamilies {
			if routes.FamilyLen(fam) == 0 {
				continue
			}
			addPath := routes.IsAddPath(fam)
			routes.IterateFamily(fam, func(raw []byte, entry storage.RouteEntry) bool {
				key, ok := flowSpecKey(peer, fam, raw, addPath)
				if !ok || entry.AddRef() != nil {
					return true
				}
				domain, destination := flowSpecDestination(fam, []byte(key.NLRI))
				first, local, valid := flowSpecASPath(entry)
				flows = append(flows, flowSpecRoute{key: key, domain: domain, destination: destination,
					originator: flowSpecOriginator(peer, entry), firstAS: first, localDomain: local, validPath: valid,
					candidate: r.extractCandidate(fam, peer, routes.PeerAddr(), entry), entry: entry})
				return true
			})
		}
	}
	var unicast []flowSpecUnicast
	if len(flows) > 0 {
		for peer, routes := range r.bgpPeers {
			flags := routes.AddPathFamilies()
			for _, fam := range routes.Families() {
				if fam.SAFI != family.SAFIUnicast && fam.SAFI != family.SAFIVPN {
					continue
				}
				routes.IterateFamily(fam, func(raw []byte, entry storage.RouteEntry) bool {
					domain, prefix := flowSpecUnicastPrefix(fam, raw, flags[fam])
					if !prefix.IsValid() || isSRv6Ineligible(entry) || isSelfNextHop(r.selfNextHops.Load(), entryNextHopAddr(fam, entry)) {
						return true
					}
					relevant := false
					for i := range flows {
						flow := &flows[i]
						if flow.domain == domain && flow.destination.IsValid() && flow.destination.Overlaps(prefix) {
							relevant = true
							break
						}
					}
					if !relevant {
						return true
					}
					if storage.IsCIDRFamily(fam) {
						pathID, _, _ := parsePrevKey(fam, raw, flags[fam])
						if !ribevents.RouteEligible(ribevents.ValidationRoute{Peer: peer, Family: fam, Prefix: prefix, PathID: pathID}, entry.MsgID) {
							return true
						}
					}
					first, _, valid := flowSpecASPath(entry)
					if !valid {
						return true
					}
					candidate := r.extractCandidate(fam, peer, routes.PeerAddr(), entry)
					unicast = append(unicast, flowSpecUnicast{domain: domain, prefix: prefix,
						originator: flowSpecOriginator(peer, entry), firstAS: first, candidate: candidate})
					return true
				})
			}
		}
	}
	r.peerMu.RUnlock()
	next := &flowSpecState{paths: make(map[ribevents.ValidationRoute]flowSpecVerdict, len(flows)), best: make(map[flowSpecRule]flowSpecWinner)}
	winners := make(map[flowSpecRule]*flowSpecRoute)
	for i := range flows {
		flow := &flows[i]
		eligible := flowSpecAuthorized(flow, unicast)
		next.paths[flow.key] = flowSpecVerdict{msgID: flow.entry.MsgID, eligible: eligible}
		if !eligible {
			continue
		}
		rule := flowSpecRule{family: flow.key.Family, nlri: flow.key.NLRI}
		current := winners[rule]
		if current == nil || ComparePair(flow.candidate, current.candidate) < 0 || (ComparePair(flow.candidate, current.candidate) == 0 && flow.key.PathID < current.key.PathID) {
			winners[rule] = flow
		}
	}
	for rule, flow := range winners {
		var communities []byte
		bundle := flow.entry.GetBundle()
		if bundle.ExtCommunities.IsValid() {
			data, err := pool.ExtCommunities.Get(bundle.ExtCommunities)
			if err != nil {
				continue
			}
			communities = bytes.Clone(data)
		}
		ipv6Communities, ok := flowSpecIPv6Communities(flow.entry)
		if !ok {
			continue
		}
		protoType, priority := r.protocolType(flow.candidate), 200
		if protoType == routeaction.ProtocolEBGP {
			priority = 20
		}
		change := bestChangeEntry{Action: routeaction.Add, NLRI: []byte(rule.nlri), Metric: flow.candidate.MED,
			Priority: priority, ProtocolType: protoType, AIGP: flow.candidate.AIGP, AIGPPresent: flow.candidate.HasAIGP}
		next.best[rule] = flowSpecWinner{key: flow.key, communities: communities, ipv6Communities: ipv6Communities, change: change}
	}
	for i := range flows {
		flows[i].entry.Release()
	}
	r.flowSpec.Store(next)
	r.flowSpecPending = next
	if r.flowSpecEmitting {
		r.flowSpecMu.Unlock()
		return
	}
	r.drainFlowSpecEvents()
}

// drainFlowSpecEvents takes ownership of flowSpecMu and releases it before
// delivery. Reentrant mutations and replay requests coalesce behind the current
// delivery so an older replay cannot overtake a replacement or withdrawal.
func (r *RIBManager) drainFlowSpecEvents() {
	r.flowSpecEmitting = true
	for {
		next, replay := r.flowSpecPending, r.flowSpecReplayPending
		r.flowSpecPending, r.flowSpecReplayPending = nil, false
		previous := r.flowSpecPublished
		if next == nil {
			next = r.flowSpec.Load()
		}
		r.flowSpecMu.Unlock()
		if next != nil {
			if next != previous {
				r.emitFlowSpecChanges(previous, next)
			}
			if replay {
				for rule, winner := range next.best {
					r.emitFlowSpec(&ribevents.FlowSpecChange{Family: rule.family, NLRI: []byte(rule.nlri),
						ExtendedCommunities: bytes.Clone(winner.communities), IPv6ExtendedCommunities: bytes.Clone(winner.ipv6Communities)})
				}
			}
		}
		r.flowSpecMu.Lock()
		r.flowSpecPublished = next
		if r.flowSpecPending == nil && !r.flowSpecReplayPending {
			r.flowSpecEmitting = false
			r.flowSpecMu.Unlock()
			return
		}
	}
}

func (r *RIBManager) emitFlowSpecChanges(previous, next *flowSpecState) {
	bus := getEventBus()
	if bus == nil {
		return
	}
	var validation []ribevents.ValidationRoute
	for key, verdict := range next.paths {
		if previous == nil || previous.paths[key] != verdict {
			validation = append(validation, key)
		}
	}
	if previous != nil {
		for key := range previous.paths {
			if _, exists := next.paths[key]; !exists {
				validation = append(validation, key)
			}
		}
		for rule, old := range previous.best {
			if _, exists := next.best[rule]; exists {
				continue
			}
			change := old.change
			change.Action = routeaction.Withdraw
			publishBestChanges([]bestChangeEntry{change}, rule.family)
			r.emitFlowSpec(&ribevents.FlowSpecChange{Family: rule.family, NLRI: []byte(rule.nlri), Withdraw: true})
		}
	}
	for rule, winner := range next.best {
		change := winner.change
		if previous != nil {
			if old, exists := previous.best[rule]; exists {
				if old.key == winner.key && old.change.Metric == change.Metric &&
					old.change.AIGP == change.AIGP && old.change.AIGPPresent == change.AIGPPresent &&
					bytes.Equal(old.communities, winner.communities) && bytes.Equal(old.ipv6Communities, winner.ipv6Communities) {
					continue
				}
				change.Action = routeaction.Update
			}
		}
		publishBestChanges([]bestChangeEntry{change}, rule.family)
		r.emitFlowSpec(&ribevents.FlowSpecChange{Family: rule.family, NLRI: []byte(rule.nlri),
			ExtendedCommunities: bytes.Clone(winner.communities), IPv6ExtendedCommunities: bytes.Clone(winner.ipv6Communities)})
	}
	if len(validation) > 0 {
		if _, err := ribevents.ValidationChange.Emit(bus, validation); err != nil {
			logger().Warn("FlowSpec export reconciliation failed", "error", err)
		}
	}
}

func (r *RIBManager) emitFlowSpec(change *ribevents.FlowSpecChange) {
	if bus := getEventBus(); bus != nil {
		if _, err := ribevents.FlowSpecChanged.Emit(bus, change); err != nil {
			logger().Warn("FlowSpec selection event failed", "error", err)
		}
	}
}

func (r *RIBManager) replayFlowSpecs() {
	r.flowSpecMu.Lock()
	r.flowSpecReplayPending = true
	if r.flowSpecEmitting {
		r.flowSpecMu.Unlock()
		return
	}
	r.drainFlowSpecEvents()
}

func (r *RIBManager) flowSpecRoutes() []ribevents.ValidationRoute {
	state := r.flowSpec.Load()
	if state == nil {
		return nil
	}
	var routes []ribevents.ValidationRoute
	for key, verdict := range state.paths {
		if verdict.eligible {
			routes = append(routes, key)
		}
	}
	return routes
}

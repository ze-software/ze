// Design: docs/architecture/route-selection.md -- source-bound metric re-advertisement
// RFC: rfc/short/rfc7311.md -- Section 3.4.3
package reactor

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type aigpNLRI struct {
	family family.Family
	key    string
	pathID uint32
}

type aigpSourceKey struct {
	peer netip.Addr
	nlri aigpNLRI
}

type aigpRoute struct {
	key     aigpSourceKey
	wire    *wireu.WireUpdate
	raw     []byte
	nextHop []byte
	order   uint32
	meta    map[string]any
}

type aigpRecipientKey struct {
	peer *Peer
	nlri aigpNLRI
}

type aigpAdvertisement struct {
	key       aigpRecipientKey
	route     *aigpRoute
	session   *Session
	sender    plugin.Sender
	revision  uint64
	metric    uint64
	hasMetric bool
}

// dispatch serializes receive invalidation and replay enqueueing. mu protects
// only retained generations and write receipts; it never enters the reactor,
// RIB or session write locks.
// State is bounded by live received paths and actual advertised destinations.
type aigpState struct {
	dispatch   sync.Mutex
	mu         sync.Mutex
	routes     map[aigpSourceKey]*aigpRoute
	messages   map[uint64]map[aigpSourceKey]*aigpRoute
	advertised map[aigpRecipientKey]*aigpAdvertisement
	changed    chan struct{}
}

// walkAIGPNLRI keeps native family identity and both ADD-PATH directions. raw
// excludes the identifier; retaining slices requires preserving the wire payload.
func walkAIGPNLRI(wu *wireu.WireUpdate, fn func(aigpNLRI, []byte, []byte, bool)) {
	ctx := bgpctx.Registry.Get(wu.SourceCtxID())
	walk := func(fam family.Family, data, nh []byte, withdraw bool) {
		split := nlrisplit.Get(fam)
		if withdraw {
			split = nlrisplit.GetWithdraw(fam)
		}
		if split == nil {
			return
		}
		addPath := ctx != nil && ctx.AddPath(fam)
		keyFor := nlrisplit.GetPrefixKey(fam)
		_, _ = split(data, addPath, func(raw []byte) {
			var id uint32
			if addPath {
				id = binary.BigEndian.Uint32(raw[:4])
				raw = raw[4:]
			}
			var scratch [nlrisplit.PrefixKeyScratchSize]byte
			key, err := keyFor(raw, scratch[:], withdraw)
			if err == nil {
				fn(aigpNLRI{family: fam, key: string(key), pathID: id}, raw, nh, withdraw)
			}
		})
	}
	if wd, err := wu.Withdrawn(); err == nil {
		walk(family.IPv4Unicast, wd, nil, true)
	}
	if mp, err := wu.MPUnreach(); err == nil && mp != nil && len(mp) >= 3 {
		walk(mp.Family(), mp[3:], nil, true)
	}
	if announced, err := wu.NLRI(); err == nil {
		nh := payloadNextHop(wu.Payload()).legacy
		walk(family.IPv4Unicast, announced, nh.AsSlice(), false)
	}
	if mp, err := wu.MPReach(); err == nil && mp != nil {
		nh := mp.NextHopBytes()
		if off := 5 + len(nh); off <= len(mp) {
			walk(mp.Family(), mp[off:], nh, false)
		}
	}
}

func (s *aigpState) signalLocked() {
	if s.changed != nil {
		select {
		case s.changed <- struct{}{}:
		default:
		}
	}
}

func (s *aigpState) removeLocked(key aigpSourceKey) {
	old := s.routes[key]
	if old == nil {
		return
	}
	delete(s.routes, key)
	id := old.wire.MessageID()
	delete(s.messages[id], key)
	if len(s.messages[id]) == 0 {
		delete(s.messages, id)
	}
}

// invalidateAIGPReceived runs before import policy, so a rejected replacement
// cannot leave its previous generation available to metric replay.
func (r *Reactor) invalidateAIGPReceived(peer netip.Addr, wu *wireu.WireUpdate) {
	s := &r.aigp
	s.dispatch.Lock()
	defer s.dispatch.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.routes) == 0 {
		return
	}
	walkAIGPNLRI(wu, func(key aigpNLRI, _, _ []byte, _ bool) { s.removeLocked(aigpSourceKey{peer: peer, nlri: key}) })
	s.signalLocked()
}

func (r *Reactor) retainAIGPReceived(peer netip.Addr, wu *wireu.WireUpdate, meta map[string]any) {
	if len(payloadAIGP(wu.Payload())) == 0 {
		return
	}
	body := bytes.Clone(wu.Payload())
	retained := wireu.NewWireUpdate(body, wu.SourceCtxID())
	retained.SetMessageID(wu.MessageID())
	retained.SetSourceID(wu.SourceID())
	s := &r.aigp
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.routes == nil {
		s.routes = make(map[aigpSourceKey]*aigpRoute)
		s.messages = make(map[uint64]map[aigpSourceKey]*aigpRoute)
		s.advertised = make(map[aigpRecipientKey]*aigpAdvertisement)
	}
	var order uint32
	walkAIGPNLRI(retained, func(nlri aigpNLRI, raw, nh []byte, withdrawn bool) {
		if withdrawn {
			return
		}
		key := aigpSourceKey{peer: peer, nlri: nlri}
		s.removeLocked(key)
		order++
		route := &aigpRoute{key: key, wire: retained, raw: raw, nextHop: nh, meta: meta, order: order}
		s.routes[key] = route
		id := wu.MessageID()
		if s.messages[id] == nil {
			s.messages[id] = make(map[aigpSourceKey]*aigpRoute)
		}
		s.messages[id][key] = route
	})
}

func aigpRevision() uint64 {
	if loc := locrib.Default(); loc != nil {
		return loc.Revision()
	}
	return 0
}

// noteAIGPWrite records only the final, policy-filtered body. The pending entries
// MUST be committed only after flush succeeds; failed flushes MUST discard them.
func (s *Session) noteAIGPWrite(body []byte) {
	if s.aigpReactor == nil || s.aigpPeer == nil {
		return
	}
	state := &s.aigpReactor.aigp
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.routes) == 0 && len(state.advertised) == 0 {
		return
	}
	metric, hasMetric := aigpMetricValue(payloadAIGP(body))
	localNextHop := isLocalAIGPNextHop(payloadNextHop(body), s.settings.LocalAddress, s.nextHopScope.Load())

	sourcePeer, _ := netip.ParseAddr(s.sentSourcePeerStr)
	sources := state.messages[s.sentSourceMessageID]
	sendCtx := bgpctx.Registry.Get(s.sendCtxID)
	var mapped map[aigpNLRI]*aigpRoute
	if s.sentAIGPOrigin.sender.IsSet() && s.settings.AIGPEnabled() {
		var keyScratch [nlrisplit.PrefixKeyScratchSize]byte
		for _, route := range sources {
			sourceCtx := bgpctx.Registry.Get(route.wire.SourceCtxID())
			sourceAdd := sourceCtx != nil && sourceCtx.AddPath(route.key.nlri.family)
			destAdd := sendCtx != nil && sendCtx.AddPath(route.key.nlri.family)
			if !sourceAdd && !destAdd {
				continue
			}
			key := route.key.nlri
			key.pathID = 0
			if destAdd {
				if sourceAdd {
					var pathKey fwdPathKey
					if fwdPathKeyFor(&pathKey, key.family, route.key.nlri.pathID, route.raw, false, keyScratch[:]) != nil {
						continue
					}
					key.pathID = fwdPathIDs.generatePath(route.wire.SourceID(), &pathKey)
				} else {
					key.pathID = fwdPathIDs.generate(route.wire.SourceID(), 0)
				}
			}
			if mapped == nil {
				mapped = make(map[aigpNLRI]*aigpRoute)
			}
			if previous := mapped[key]; previous == nil || previous.order < route.order {
				mapped[key] = route
			}
		}
	}

	out := wireu.NewWireUpdate(body, s.sendCtxID)
	walkAIGPNLRI(out, func(key aigpNLRI, _, nextHop []byte, withdrawn bool) {
		entry := aigpAdvertisement{key: aigpRecipientKey{peer: s.aigpPeer, nlri: key}, session: s, sender: s.sentAIGPOrigin.sender, revision: s.sentAIGPRevision, metric: metric, hasMetric: hasMetric}
		if !withdrawn && localNextHop && s.sentAIGPOrigin.sender.IsSet() && s.settings.AIGPEnabled() {
			route := sources[aigpSourceKey{peer: sourcePeer, nlri: key}]
			if remapped := mapped[key]; remapped != nil {
				route = remapped
			}
			if route != nil && !bytes.Equal(route.nextHop, nextHop) {
				entry.route = route
			}
		}
		s.aigpPending = append(s.aigpPending, entry)
	})
}

func (s *Session) commitAIGPWrites(ok bool) {
	if len(s.aigpPending) == 0 {
		return
	}
	state := &s.aigpReactor.aigp
	state.mu.Lock()
	if ok && s.State() == fsm.StateEstablished {
		for _, sent := range s.aigpPending {
			delete(state.advertised, sent.key)
			if sent.route != nil && state.routes[sent.route.key] == sent.route {
				entry := sent
				state.advertised[sent.key] = &entry
			}
		}
		state.signalLocked()
	}
	state.mu.Unlock()
	clear(s.aigpPending)
	s.aigpPending = s.aigpPending[:0]
}

func (r *Reactor) forgetAIGPPeer(peer *Peer) {
	s := &r.aigp
	s.dispatch.Lock()
	defer s.dispatch.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.routes {
		if key.peer == peer.Settings().Address {
			s.removeLocked(key)
		}
	}
	for key, ad := range s.advertised {
		if key.peer == peer || s.routes[ad.route.key] != ad.route {
			delete(s.advertised, key)
		}
	}
}

func (r *Reactor) currentAIGPAdvertisement(ad *aigpAdvertisement) bool {
	r.aigp.mu.Lock()
	defer r.aigp.mu.Unlock()
	return r.aigp.advertised[ad.key] == ad && r.aigp.routes[ad.route.key] == ad.route
}

func (r *Reactor) runAIGPAdvertisements(ctx context.Context) {
	loc := locrib.Default()
	if loc == nil {
		return
	}
	changed := make(chan struct{}, 1)
	r.aigp.mu.Lock()
	r.aigp.changed = changed
	r.aigp.mu.Unlock()
	unsubscribe := loc.OnChange(func(_ locrib.Change) {
		select {
		case changed <- struct{}{}:
		default:
		}
	})
	defer unsubscribe()
	r.readvertiseAIGP()
	for {
		select {
		case <-ctx.Done():
			return
		case <-changed:
			r.readvertiseAIGP()
		}
	}
}

func (r *Reactor) readvertiseAIGP() {
	revision := aigpRevision()
	r.aigp.mu.Lock()
	var pending []*aigpAdvertisement
	for key, ad := range r.aigp.advertised {
		if r.aigp.routes[ad.route.key] != ad.route {
			delete(r.aigp.advertised, key)
			continue
		}
		if ad.revision != revision {
			pending = append(pending, ad)
		}
	}
	r.aigp.mu.Unlock()
	for _, ad := range pending {
		r.replayAIGP(ad, revision)
	}
}

func (r *Reactor) replayAIGP(ad *aigpAdvertisement, revision uint64) {
	adapter := &reactorAPIAdapter{r: r}
	src := adapter.resolveRelaySource(ad.route.key.peer)
	if !src.ok || ad.key.peer.currentSession() != ad.session {
		return
	}
	r.aigp.dispatch.Lock()
	defer r.aigp.dispatch.Unlock()
	if !r.currentAIGPAdvertisement(ad) {
		return
	}
	r.mu.RLock()
	peers, _ := filterPermittedPeers([]*Peer{ad.key.peer}, announceOrigin(ad.sender))
	r.mu.RUnlock()
	if len(peers) == 0 {
		return
	}
	metric, present := aigpMetricValue(payloadAIGP(ad.route.wire.Payload()))
	// A family can arrive in either legacy or MP framing. The retained route
	// records the next hop from the field that carried this particular NLRI.
	nextHop, _ := nextHopAddr(ad.route.nextHop)
	increment, usable := aigpIncrement(nextHop, src.addr, r.sourceAIGPLinkMetric(src.addr))
	present = present && usable
	if present {
		metric = igpcost.Add(metric, increment)
	}
	if present == ad.hasMetric && (!present || metric == ad.metric) {
		r.aigp.mu.Lock()
		if r.aigp.advertised[ad.key] == ad {
			ad.revision = revision
		}
		r.aigp.mu.Unlock()
		return
	}
	wu := ad.route.wire
	withdrawn, err := wu.Withdrawn()
	if err != nil {
		return
	}
	start := 4 + len(withdrawn)
	end := start + int(binary.BigEndian.Uint16(wu.Payload()[start-2:start]))
	route := rpc.StoredRoute{SourcePeer: ad.route.key.peer.String(), Family: ad.route.key.nlri.family.String(),
		MsgID: wu.MessageID(), PathID: ad.route.key.nlri.pathID, NLRIFraming: rpc.NLRIFramingPrefixOnly,
		NLRIHex: hex.EncodeToString(ad.route.raw), AttrHex: hex.EncodeToString(wu.Payload()[start:end]), NextHopHex: hex.EncodeToString(ad.route.nextHop)}
	var spans []relayAttrSpan
	update, id, _, err := adapter.buildRelayUpdate([]rpc.StoredRoute{route}, src, &spans)
	if err != nil {
		return
	}
	defer r.recentUpdates.Release(id)
	update.Meta = ad.route.meta
	update.aigpReplay = ad
	src.info.sender = ad.sender
	if err := adapter.forwardUpdateValidated(update, id, peers, src.info); err != nil {
		fwdLogger().Debug("AIGP metric re-advertisement withheld", "source", src.addr, "error", err)
	}
}

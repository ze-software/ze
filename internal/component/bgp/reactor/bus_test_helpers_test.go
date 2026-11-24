// Design: docs/architecture/api/architecture.md -- test helpers for compiled bus filters and delivery graphs
// Related: delivery_graph.go -- DeliveryGraph publication and lookup on Reactor
// Related: peer_derived_bindings.go -- bus feed helpers for hot-swappable reloads
package reactor

import (
	"maps"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// testPeerEndpointGrant describes one endpoint's bus feed/send relationship
// with a test BGP peer.
type testPeerEndpointGrant struct {
	Name       string
	ReceiveAll bool
	Receive    []string // BGP receive tokens ("update", "update-received", "update-sent", "state", "negotiated", "open", "refresh", "connected", "fsm", ...)
	SendAll    bool
	Send       []string // BGP send tokens ("update", "raw", "refresh", ...)
}

func testAllBGPEventNames() []string {
	types := slices.Clone(events.AllEventTypes()[bgpevents.Namespace])
	for _, st := range bgpevents.BaseSendTypes() {
		if !slices.Contains(types, st) {
			types = append(types, st)
		}
	}
	slices.Sort(types)
	return types
}

func testAllBGPMessages() []string {
	if reg := registry.Lookup(bgpevents.Namespace); reg != nil && len(reg.Sends) > 0 {
		out := make([]string, 0, len(reg.Sends))
		for _, s := range reg.Sends {
			out = append(out, s.Message)
		}
		slices.Sort(out)
		return out
	}
	ets := testAllBGPEventNames()
	out := make([]string, 0, len(ets))
	for _, et := range ets {
		out = append(out, bgpevents.Namespace+":"+et)
	}
	return out
}

func testRegisteredDirTags(msg string) []rpc.MessageTagDecl {
	if desc, ok := events.LookupMessage(msg); ok {
		for _, tag := range desc.Tags {
			if tag.Name == events.TagDirection && len(tag.Values) > 0 {
				return []rpc.MessageTagDecl{{Name: tag.Name, Values: slices.Clone(tag.Values)}}
			}
		}
	}
	return nil
}

func testAllBGPSendDecls() []rpc.MessageSendDecl {
	ets := testAllBGPEventNames()
	sends := make([]rpc.MessageSendDecl, 0, len(ets))
	for _, et := range ets {
		msg := bgpevents.Namespace + ":" + et
		sends = append(sends, rpc.MessageSendDecl{Message: msg, Tags: testRegisteredDirTags(msg)})
	}
	return sends
}

func testAllBGPReceiveDecls() []rpc.MessageReceiveDecl {
	ets := testAllBGPEventNames()
	receives := make([]rpc.MessageReceiveDecl, 0, len(ets))
	for _, et := range ets {
		msg := bgpevents.Namespace + ":" + et
		receives = append(receives, rpc.MessageReceiveDecl{Message: msg, Tags: testRegisteredDirTags(msg)})
	}
	return receives
}

func testBusEndpointAll(name string) pluginserver.BusParticipantInput {
	return pluginserver.BusParticipantInput{
		Path:         plugin.EndpointPath(name),
		EndpointName: name,
		Sends:        testAllBGPSendDecls(),
		Receives:     testAllBGPReceiveDecls(),
	}
}

func qualifyBGPMessage(token string) string {
	if strings.Contains(token, ":") {
		return token
	}
	return bgpevents.Namespace + ":" + token
}

func testBusNone() plugin.BusBlock {
	return plugin.BusBlock{
		Receive: plugin.BusDirectionBlock{AllowPresent: true, AllowNone: true},
		Send:    plugin.BusDirectionBlock{AllowPresent: true, AllowNone: true},
	}
}

// testBusBlockForGrants builds a peer BusBlock expressing one or more endpoint
// grants. Receive grants populate Bus.Send (what the peer feeds to endpoints);
// Send grants populate Bus.Receive (what endpoints may send to this peer).
func testBusBlockForGrants(grants ...testPeerEndpointGrant) plugin.BusBlock {
	if len(grants) == 0 {
		return testBusNone()
	}

	allMsgs := testAllBGPMessages()
	allSendMsgs := make([]string, 0, len(bgpevents.BaseSendTypes()))
	for _, st := range bgpevents.BaseSendTypes() {
		allSendMsgs = append(allSendMsgs, qualifyBGPMessage(st))
	}
	slices.Sort(allSendMsgs)

	// Peer Send = endpoint Receive (what the peer feeds to endpoints).
	sendTargetsByMsg := make(map[string][]string)
	sendDirsByMsg := make(map[string]events.Direction)
	// Peer Receive = endpoint Send (what endpoints may send to this peer).
	recvTargetsByMsg := make(map[string][]string)

	addUnique := func(m map[string][]string, key, val string) {
		if !slices.Contains(m[key], val) {
			m[key] = append(m[key], val)
		}
	}

	for _, g := range grants {
		if g.Name == "" {
			continue
		}
		epPath := plugin.EndpointPath(g.Name)

		if g.ReceiveAll {
			for _, msg := range allMsgs {
				addUnique(sendTargetsByMsg, msg, epPath)
				sendDirsByMsg[msg] |= events.DirBoth
			}
		} else {
			for _, tok := range g.Receive {
				if tok == events.TokenWildcard {
					for _, msg := range allMsgs {
						addUnique(sendTargetsByMsg, msg, epPath)
						sendDirsByMsg[msg] |= events.DirBoth
					}
					continue
				}
				et, dir, ok := events.SplitTypeToken(bgpevents.Namespace, tok)
				if !ok {
					et = tok
					dir = events.DirBoth
				}
				msg := qualifyBGPMessage(et)
				addUnique(sendTargetsByMsg, msg, epPath)
				sendDirsByMsg[msg] |= dir
			}
		}

		if g.SendAll {
			for _, msg := range allSendMsgs {
				addUnique(recvTargetsByMsg, msg, epPath)
			}
		} else {
			for _, tok := range g.Send {
				if tok == events.TokenWildcard {
					for _, msg := range allSendMsgs {
						addUnique(recvTargetsByMsg, msg, epPath)
					}
					continue
				}
				msg := qualifyBGPMessage(tok)
				addUnique(recvTargetsByMsg, msg, epPath)
			}
		}
	}

	var blk plugin.BusBlock

	if len(sendTargetsByMsg) == 0 {
		blk.Send = plugin.BusDirectionBlock{AllowPresent: true, AllowNone: true}
	} else {
		allow := slices.Sorted(maps.Keys(sendTargetsByMsg))
		blk.Send = plugin.BusDirectionBlock{
			AllowPresent:   true,
			Allow:          allow,
			TargetsPresent: true,
			TargetsNone:    true,
			Messages:       make(map[string]*plugin.BusMessageRule, len(allow)),
		}
		for _, msg := range allow {
			targets := slices.Clone(sendTargetsByMsg[msg])
			slices.Sort(targets)
			rule := &plugin.BusMessageRule{
				TargetsPresent: true,
				Targets:        targets,
			}
			dir := sendDirsByMsg[msg]
			switch dir {
			case events.DirReceived:
				rule.Tags = map[string][]string{events.TagDirection: {events.DirectionReceived}}
			case events.DirSent:
				rule.Tags = map[string][]string{events.TagDirection: {events.DirectionSent}}
			}
			blk.Send.Messages[msg] = rule
		}
	}

	if len(recvTargetsByMsg) == 0 {
		blk.Receive = plugin.BusDirectionBlock{AllowPresent: true, AllowNone: true}
	} else {
		allow := slices.Sorted(maps.Keys(recvTargetsByMsg))
		blk.Receive = plugin.BusDirectionBlock{
			AllowPresent:   true,
			Allow:          allow,
			TargetsPresent: true,
			TargetsNone:    true,
			Messages:       make(map[string]*plugin.BusMessageRule, len(allow)),
		}
		for _, msg := range allow {
			targets := slices.Clone(recvTargetsByMsg[msg])
			slices.Sort(targets)
			blk.Receive.Messages[msg] = &plugin.BusMessageRule{
				TargetsPresent: true,
				Targets:        targets,
			}
		}
	}

	return blk
}

func testBusSendOnly(name string, sendTokens ...string) plugin.BusBlock {
	return testBusBlockForGrants(testPeerEndpointGrant{Name: name, Send: sendTokens})
}

func testBusSendAll(name string) plugin.BusBlock {
	return testBusBlockForGrants(testPeerEndpointGrant{Name: name, SendAll: true})
}

func testBusReceiveOnly(name string, recvTokens ...string) plugin.BusBlock {
	return testBusBlockForGrants(testPeerEndpointGrant{Name: name, Receive: recvTokens})
}

func testBusReceiveAll(name string) plugin.BusBlock {
	return testBusBlockForGrants(testPeerEndpointGrant{Name: name, ReceiveAll: true})
}

func testBusBoth(name string, recvTokens, sendTokens []string) plugin.BusBlock {
	return testBusBlockForGrants(testPeerEndpointGrant{Name: name, Receive: recvTokens, Send: sendTokens})
}

func testBusRSRelaySend() plugin.BusBlock {
	return testBusSendOnly(rsFixtureRelay, bgpevents.SendUpdate)
}

func testBusRSRelayReceive() plugin.BusBlock {
	return testBusReceiveOnly(rsFixtureRelay, "update-received")
}

func testBusSourceLease(name string) plugin.BusBlock {
	return testBusReceiveOnly(name, "update-received", bgpevents.EventState)
}

func testBusSelectedDest(name string) plugin.BusBlock {
	return testBusBoth(name, []string{bgpevents.EventState, bgpevents.EventRefresh, "update-sent"}, []string{bgpevents.SendUpdate})
}

func testBusHasEndpoint(blk plugin.BusBlock, name string) bool {
	seen := make(map[string]bool)
	var found []string
	collectTestEndpointsFromBus(blk, seen, &found)
	return seen[name]
}

func mergeTestBusDirections(a, b plugin.BusDirectionBlock) plugin.BusDirectionBlock {
	if a.IsEmpty() || a.AllowNone {
		return b.Clone()
	}
	if b.IsEmpty() || b.AllowNone {
		return a.Clone()
	}
	out := a.Clone()
	for _, msg := range b.Allow {
		if !slices.Contains(out.Allow, msg) {
			out.Allow = append(out.Allow, msg)
		}
	}
	slices.Sort(out.Allow)
	out.AllowPresent = len(out.Allow) > 0 || out.AllowPresent
	if len(b.Messages) > 0 && out.Messages == nil {
		out.Messages = make(map[string]*plugin.BusMessageRule, len(b.Messages))
	}
	for msg, br := range b.Messages {
		if br == nil {
			continue
		}
		ar := out.Messages[msg]
		if ar == nil {
			out.Messages[msg] = br.Clone()
			continue
		}
		for _, target := range br.Targets {
			if !slices.Contains(ar.Targets, target) {
				ar.Targets = append(ar.Targets, target)
			}
		}
		slices.Sort(ar.Targets)
		ar.TargetsPresent = ar.TargetsPresent || br.TargetsPresent
		if len(ar.Tags) > 0 {
			if len(br.Tags) == 0 {
				ar.Tags = nil
			} else {
				aDirs := ar.Tags[events.TagDirection]
				bDirs := br.Tags[events.TagDirection]
				if len(aDirs) == 0 || len(bDirs) == 0 {
					delete(ar.Tags, events.TagDirection)
				} else {
					for _, d := range bDirs {
						if !slices.Contains(aDirs, d) {
							aDirs = append(aDirs, d)
						}
					}
					if len(aDirs) >= 2 {
						delete(ar.Tags, events.TagDirection)
					} else {
						ar.Tags[events.TagDirection] = aDirs
					}
				}
				if len(ar.Tags) == 0 {
					ar.Tags = nil
				}
			}
		}
	}
	return out
}

func mergeTestBusBlocks(a, b plugin.BusBlock) plugin.BusBlock {
	if a.IsEmpty() {
		return b.Clone()
	}
	if b.IsEmpty() {
		return a.Clone()
	}
	return plugin.BusBlock{
		Receive: mergeTestBusDirections(a.Receive, b.Receive),
		Send:    mergeTestBusDirections(a.Send, b.Send),
	}
}

func addTestPeerBusGrant(settings *PeerSettings, grant testPeerEndpointGrant) {
	settings.Bus = mergeTestBusBlocks(settings.Bus, testBusBlockForGrants(grant))
	settings.BusOrigin = pluginserver.BusOriginWritten
}

// collectTestEndpointsFromBus extracts every /bus/endpoint/<name> referenced in
// blk and appends its endpoint name to seen/out.
func collectTestEndpointsFromBus(blk plugin.BusBlock, seen map[string]bool, out *[]string) {
	collectDir := func(d plugin.BusDirectionBlock) {
		for _, tp := range d.Targets {
			if ep, ok := endpointNameFromBusPath(tp); ok && !seen[ep] {
				seen[ep] = true
				*out = append(*out, ep)
			}
		}
		for _, rule := range d.Messages {
			if rule == nil {
				continue
			}
			for _, tp := range rule.Targets {
				if ep, ok := endpointNameFromBusPath(tp); ok && !seen[ep] {
					seen[ep] = true
					*out = append(*out, ep)
				}
			}
		}
	}
	collectDir(blk.Send)
	collectDir(blk.Receive)
}

func buildTestEndpointInputsForPeers(
	peers []pluginserver.DeliveryPeer,
	srv *pluginserver.Server,
	extra []pluginserver.BusParticipantInput,
) []pluginserver.BusParticipantInput {
	seen := make(map[string]bool)
	out := make([]pluginserver.BusParticipantInput, 0, len(extra)+4)

	for _, ep := range extra {
		epName := ep.EndpointName
		if epName == "" {
			if parsed, ok := endpointNameFromBusPath(ep.Path); ok {
				epName = parsed
			}
		}
		if epName != "" {
			seen[epName] = true
		}
		out = append(out, ep)
	}

	var discovered []string
	for i := range peers {
		collectTestEndpointsFromBus(peers[i].Bus, seen, &discovered)
	}
	if srv != nil {
		if pm := srv.ProcessManager(); pm != nil {
			for _, proc := range pm.AllProcesses() {
				if !seen[proc.Name()] {
					seen[proc.Name()] = true
					discovered = append(discovered, proc.Name())
				}
			}
		}
	}
	slices.Sort(discovered)
	for _, name := range discovered {
		input := testBusEndpointAll(name)
		// Preserve explicit compiled-in declarations if registered with non-empty sends/receives.
		if reg := registry.Lookup(name); reg != nil && (len(reg.Sends) > 0 || len(reg.Receives) > 0) {
			// Still keep all BGP sends/receives for test fixtures if needed; merge.
			input = mergeTestEndpointDecls(input, reg.Sends, reg.Receives)
		}
		out = append(out, input)
	}
	return out
}

func mergeTestEndpointDecls(
	base pluginserver.BusParticipantInput,
	sends []rpc.MessageSendDecl,
	receives []rpc.MessageReceiveDecl,
) pluginserver.BusParticipantInput {
	seenSend := make(map[string]bool, len(base.Sends))
	for _, s := range base.Sends {
		seenSend[s.Message] = true
	}
	for _, s := range sends {
		if !seenSend[s.Message] {
			seenSend[s.Message] = true
			base.Sends = append(base.Sends, s)
		}
	}
	seenRecv := make(map[string]bool, len(base.Receives))
	for _, r := range base.Receives {
		seenRecv[r.Message] = true
	}
	for _, r := range receives {
		if !seenRecv[r.Message] {
			seenRecv[r.Message] = true
			base.Receives = append(base.Receives, r)
		}
	}
	return base
}

// publishTestBusGraph compiles and publishes a DeliveryGraph for all peers in r
// (plus any extraEndpoints). Works both with and without r.api.
func publishTestBusGraph(t testing.TB, r *Reactor, extraEndpoints ...pluginserver.BusParticipantInput) *pluginserver.DeliveryGraph {
	t.Helper()
	r.mu.RLock()
	settings := make([]*PeerSettings, 0, len(r.peers))
	for _, p := range r.peers {
		settings = append(settings, p.settings)
	}
	r.mu.RUnlock()
	peers := deliveryPeersFromSettings(settings)
	endpoints := buildTestEndpointInputsForPeers(peers, r.api, extraEndpoints)
	if r.api != nil {
		require.NoError(t, r.api.UpdateDeliveryGraphWithEndpoints(bgpevents.Namespace, peers, endpoints))
		r.mu.Lock()
		r.deliveryPublished = true
		r.mu.Unlock()
		return r.api.DeliveryGraph()
	}
	g, err := pluginserver.BuildDeliveryGraph(bgpevents.Namespace, peers, r.busConfig, endpoints)
	require.NoError(t, err)
	r.mu.Lock()
	r.testBusEndpoints = endpoints
	r.deliveryGraph.Store(g)
	r.deliveryPublished = true
	r.mu.Unlock()
	return g
}

// setTestPeerBus sets peer.settings.Bus and publishes the compiled
// DeliveryGraph on peer.reactor (creating a minimal standalone Reactor if
// peer.reactor is nil).
func setTestPeerBus(t testing.TB, peer *Peer, block plugin.BusBlock, extraEndpoints ...pluginserver.BusParticipantInput) *pluginserver.DeliveryGraph {
	t.Helper()
	peer.mu.Lock()
	peer.settings.Bus = block
	if !block.IsEmpty() {
		peer.settings.BusOrigin = pluginserver.BusOriginWritten
	}
	r := peer.reactor
	if r == nil {
		r = &Reactor{
			peers: map[netip.AddrPort]*Peer{
				peer.settings.PeerKey(): peer,
			},
		}
		peer.reactor = r
	}
	peer.mu.Unlock()

	r.mu.Lock()
	if r.peers == nil {
		r.peers = make(map[netip.AddrPort]*Peer)
	}
	r.peers[peer.settings.PeerKey()] = peer
	r.mu.Unlock()

	return publishTestBusGraph(t, r, extraEndpoints...)
}

func setTestPeerBusPanic(peer *Peer, block plugin.BusBlock) *pluginserver.DeliveryGraph {
	peer.mu.Lock()
	peer.settings.Bus = block
	if !block.IsEmpty() {
		peer.settings.BusOrigin = pluginserver.BusOriginWritten
	}
	r := peer.reactor
	if r == nil {
		r = &Reactor{peers: map[netip.AddrPort]*Peer{peer.settings.PeerKey(): peer}}
		peer.reactor = r
	}
	peer.mu.Unlock()
	peers := deliveryPeersFromSettings([]*PeerSettings{peer.settings})
	g, err := pluginserver.BuildDeliveryGraph(bgpevents.Namespace, peers, r.busConfig, buildTestEndpointInputsForPeers(peers, nil, nil))
	if err != nil {
		panic("BUG: build delivery graph for test peer: " + err.Error())
	}
	r.deliveryGraph.Store(g)
	return g
}

// buildTestDisseminationGraph compiles a standalone DeliveryGraph for
// dissemination unit tests from peers and auto-discovered endpoint inputs.
func buildTestDisseminationGraph(t testing.TB, peers []pluginserver.DeliveryPeer, extraEndpoints ...pluginserver.BusParticipantInput) *pluginserver.DeliveryGraph {
	t.Helper()
	endpoints := buildTestEndpointInputsForPeers(peers, nil, extraEndpoints)
	g, err := pluginserver.NewDisseminationGraph(bgpevents.Namespace, peers, plugin.BusConfig{}, endpoints)
	require.NoError(t, err)
	return g
}

// testDeliveryPeer builds a DeliveryPeer for unit tests from addr, name, and BusBlock.
func testDeliveryPeer(addr, name string, block plugin.BusBlock) pluginserver.DeliveryPeer {
	path := bgpSecondaryIPPrefix + addr
	var secondary []string
	if name != "" {
		path = "/bgp/peer/" + name
		if addr != "" {
			secondary = []string{bgpSecondaryIPPrefix + addr}
		}
	}
	return pluginserver.DeliveryPeer{
		Addr:           addr,
		Name:           name,
		Path:           path,
		SecondaryPaths: secondary,
		Bus:            block,
		BusOrigin:      pluginserver.BusOriginWritten,
		BusSource:      path + " > bus",
	}
}

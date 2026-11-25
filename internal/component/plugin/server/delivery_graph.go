// Design: docs/architecture/api/architecture.md -- compiled bus delivery index
// Related: bus_filter.go -- CompiledBus, CompileBus, and CompiledParticipantFilter
// Related: subscribe.go -- SubscriptionManager, the runtime event subscription table

package server

import (
	"context"
	"errors"
	"io"
	"maps"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// DeliveryPeer describes one participant address entry published into the
// runtime DeliveryGraph. Callers supply the participant's canonical bus path,
// optional parent container path, optional secondary lookup paths, and resolved
// BusBlock.
type DeliveryPeer struct {
	// Addr is the lookup key (e.g. remote address string) used on the delivery
	// hot path.
	Addr string
	// Name is the optional configured entry name.
	Name string
	// Path is the canonical short bus path, e.g. "/proto/entry/c1".
	Path string
	// ParentPath is the optional parent container path, e.g. "/proto/container/ixp".
	ParentPath string
	// SecondaryPaths holds optional alias paths resolving to this participant.
	SecondaryPaths []string
	// Bus is the participant's resolved own BusBlock.
	Bus plugin.BusBlock
	// BusOrigin records whether Bus was written directly (BusOriginWritten) or
	// inherited from ParentPath (BusOriginInherited).
	BusOrigin BusEdgeOrigin
	// BusSource is the human-readable configuration location of Bus.
	BusSource string
	// OwnBus optionally holds the participant's unmerged own BusBlock when
	// ParentBus carries the parent container's own BusBlock separately.
	OwnBus plugin.BusBlock
	// ParentBus optionally holds the parent container's own BusBlock so
	// CompileBus attributes inherited items to ParentPath.
	ParentBus plugin.BusBlock
	// ParentBusSource is the human-readable configuration location of ParentBus.
	ParentBusSource string
	// Dynamic reports whether this participant was admitted dynamically under
	// ParentPath rather than statically configured.
	Dynamic bool
	// SessionFacts optionally carries key/value session properties evaluated
	// against DisseminationProcess.ClaimSessionScope.
	SessionFacts map[string]string
}

// ProcessEdges is the operator view of one endpoint's compiled delivery
// relationship with a participant address entry.
type ProcessEdges struct {
	Process string            `json:"process"`
	Path    string            `json:"path,omitempty"`
	Receive []string          `json:"receive,omitempty"`
	Send    []string          `json:"send,omitempty"`
	Origin  string            `json:"origin,omitempty"`
	Origins map[string]string `json:"origins,omitempty"`
}

// PeerEdges is the operator view of all compiled delivery relationships for
// one participant address entry.
type PeerEdges struct {
	Peer       string         `json:"peer"`
	Name       string         `json:"name,omitempty"`
	Path       string         `json:"path,omitempty"`
	ParentPath string         `json:"parent-path,omitempty"`
	Dynamic    bool           `json:"dynamic,omitempty"`
	Processes  []ProcessEdges `json:"processes"`
}

// DeliveryEdge describes one compiled bus delivery relationship across any
// namespace, deduplicated per registered rule ("one registration, one line").
type DeliveryEdge struct {
	SenderPath       string              `json:"sender-path,omitempty"`
	SenderEndpoint   string              `json:"sender-endpoint,omitempty"`
	SenderGroup      string              `json:"sender-group,omitempty"`
	ReceiverPath     string              `json:"receiver-path,omitempty"`
	ReceiverEndpoint string              `json:"receiver-endpoint,omitempty"`
	ReceiverGroup    string              `json:"receiver-group,omitempty"`
	Message          string              `json:"message"`
	Origin           string              `json:"origin"`
	Source           string              `json:"source,omitempty"`
	Destinations     []string            `json:"destinations,omitempty"`
	Tags             map[string][]string `json:"tags,omitempty"`
}

// deliveryKey is one hot-path delivery lookup key. Every field is comparable so
// lookup is a single map read with zero allocations.
type deliveryKey struct {
	ns  events.NamespaceID
	et  events.EventTypeID
	dir events.Direction
}

type peerSendKey struct {
	process  string
	sendType string
}

// DeliveryGraph is an immutable runtime delivery index backed by one
// CompiledBus snapshot. Readers take it atomically via Server.DeliveryGraph so
// reloads swap whole snapshots with zero partial state.
type DeliveryGraph struct {
	compiled           *CompiledBus
	disseminationPeers []DeliveryPeer
	peers              map[string]*peerDelivery
	pathToAddr         map[string]string
	dynamicGroupByPath map[string]string
	order              []string
	namespace          string
	ns                 events.NamespaceID
}

type peerDelivery struct {
	name          string
	path          string
	parentPath    string
	dynamic       bool
	sessionFacts  map[string]string
	procs         []processDelivery
	receive       map[deliveryKey][]string
	sendersByType map[string][]string
	maySend       map[peerSendKey]bool
	recvState     map[string]bool
	allProcs      []string
}

type processDelivery struct {
	name        string
	path        string
	send        []string
	origin      BusEdgeOrigin
	recvOrigins map[events.EventTypeID]BusEdgeOrigin
	sendOrigins map[string]BusEdgeOrigin
}

// emptyDeliveryGraph is returned before the first graph publication so lookups
// fail closed and never answer "everything".
var emptyDeliveryGraph = NewDeliveryGraphFromCompiled("", nil, nil)

// BuildDeliveryGraph compiles peers, central busCfg, and extraEndpoints into
// one CompiledBus and builds the runtime DeliveryGraph over it.
func BuildDeliveryGraph(
	namespace string,
	peers []DeliveryPeer,
	busCfg plugin.BusConfig,
	extraEndpoints []BusParticipantInput,
) (*DeliveryGraph, error) {
	defaultSends, defaultReceives := defaultNamespaceDeclarations(namespace)
	inputs := make([]BusParticipantInput, 0, len(peers)+len(extraEndpoints))
	seenPaths := make(map[string]bool, len(peers)+len(extraEndpoints))
	type parentBusSpec struct {
		block  plugin.BusBlock
		source string
	}
	parentContainers := make(map[string]bool)
	parentOwnBus := make(map[string]parentBusSpec)

	for i := range peers {
		p := &peers[i]
		if p.Addr == "" && p.Path == "" {
			continue
		}
		pPath := deliveryPeerPath(namespace, p)
		if pPath == "" {
			continue
		}
		if seenPaths[pPath] {
			continue
		}
		seenPaths[pPath] = true

		hasParentBus := p.ParentPath != "" && !p.ParentBus.IsEmpty()
		var ownBlock plugin.BusBlock
		var ownOrigin BusEdgeOrigin
		var ownSource string
		if hasParentBus {
			ownBlock = p.OwnBus.Clone()
			if !ownBlock.IsEmpty() {
				ownOrigin = BusOriginWritten
				ownSource = pPath + " > bus"
			}
			if _, exists := parentOwnBus[p.ParentPath]; !exists {
				src := p.ParentBusSource
				if src == "" {
					src = p.ParentPath + " > bus"
				}
				parentOwnBus[p.ParentPath] = parentBusSpec{
					block:  p.ParentBus.Clone(),
					source: src,
				}
			}
		} else {
			ownBlock = p.Bus.Clone()
			ownOrigin = p.BusOrigin
			ownSource = p.BusSource
		}

		input := BusParticipantInput{
			Path:           pPath,
			SecondaryPaths: slices.Clone(p.SecondaryPaths),
			ParentPath:     p.ParentPath,
			Sends:          defaultSends,
			Receives:       defaultReceives,
			OwnBlock:       ownBlock,
			OwnOrigin:      ownOrigin,
			OwnSource:      ownSource,
		}
		applyCentralOverrideToInput(&input, busCfg, !hasParentBus)
		inputs = append(inputs, input)

		if p.ParentPath != "" {
			parentContainers[p.ParentPath] = true
		}
	}

	for i := range extraEndpoints {
		ep := extraEndpoints[i]
		if ep.Path == "" && ep.EndpointName != "" {
			ep.Path = plugin.EndpointPath(ep.EndpointName)
		}
		if ep.Path == "" {
			continue
		}
		if seenPaths[ep.Path] {
			continue
		}
		seenPaths[ep.Path] = true
		applyEndpointBusConfig(&ep, busCfg)
		inputs = append(inputs, ep)
	}

	// Ensure referenced parent container paths exist as container participants
	// so parent own bus blocks and bus > participant overrides inherit and
	// prefix paths resolve.
	for _, parentPath := range slices.Sorted(maps.Keys(parentContainers)) {
		if seenPaths[parentPath] {
			continue
		}
		seenPaths[parentPath] = true
		cInput := BusParticipantInput{Path: parentPath, PureContainer: true}
		if spec, ok := parentOwnBus[parentPath]; ok {
			if !spec.block.IsEmpty() {
				cInput.OwnBlock = spec.block.Clone()
				cInput.OwnOrigin = BusOriginWritten
				cInput.OwnSource = spec.source
			}
		}
		if ov, ok := busCfg.Participants[parentPath]; ok {
			cInput.BusSideBlock = ov.Block.Clone()
			cInput.BusSideSource = ov.Source
		}
		inputs = append(inputs, cInput)
	}

	compiled, err := CompileBus(NewTagInterner(), inputs)
	if err != nil {
		return nil, err
	}
	return NewDeliveryGraphFromCompiled(namespace, peers, compiled), nil
}

// NewDeliveryGraphFromCompiled builds a zero-alloc runtime DeliveryGraph index
// over an already-compiled CompiledBus snapshot.
func NewDeliveryGraphFromCompiled(namespace string, peers []DeliveryPeer, compiled *CompiledBus) *DeliveryGraph {
	nsID := events.LookupNamespaceID(namespace)
	stateETID := events.LookupEventTypeID("state")
	g := &DeliveryGraph{
		compiled:           compiled,
		disseminationPeers: cloneDeliveryPeers(peers),
		peers:              make(map[string]*peerDelivery, len(peers)),
		pathToAddr:         make(map[string]string, len(peers)*2),
		dynamicGroupByPath: make(map[string]string),
		order:              make([]string, 0, len(peers)),
		namespace:          namespace,
		ns:                 nsID,
	}

	for i := range peers {
		p := &peers[i]
		if p.Addr == "" {
			continue
		}
		pPath := deliveryPeerPath(namespace, p)
		g.pathToAddr[pPath] = p.Addr
		for _, sp := range p.SecondaryPaths {
			if sp != "" {
				g.pathToAddr[sp] = p.Addr
			}
		}

		isDynamic := p.Dynamic
		if !isDynamic && p.ParentPath != "" && !plugin.PathCovers(p.ParentPath, pPath) {
			isDynamic = true
		}
		if isDynamic && p.ParentPath != "" {
			g.dynamicGroupByPath[pPath] = p.ParentPath
			for _, sp := range p.SecondaryPaths {
				if sp != "" {
					g.dynamicGroupByPath[sp] = p.ParentPath
				}
			}
		}

		pd := &peerDelivery{
			name:          p.Name,
			path:          pPath,
			parentPath:    p.ParentPath,
			dynamic:       isDynamic,
			sessionFacts:  maps.Clone(p.SessionFacts),
			receive:       make(map[deliveryKey][]string),
			sendersByType: make(map[string][]string),
			maySend:       make(map[peerSendKey]bool),
			recvState:     make(map[string]bool),
		}

		if compiled != nil {
			populatePeerDeliveryFromCompiled(pd, namespace, nsID, stateETID, pPath, compiled)
		}

		if _, exists := g.peers[p.Addr]; !exists {
			g.order = append(g.order, p.Addr)
		}
		g.peers[p.Addr] = pd
	}

	slices.Sort(g.order)
	return g
}

func populatePeerDeliveryFromCompiled(
	pd *peerDelivery,
	namespace string,
	nsID events.NamespaceID,
	stateETID events.EventTypeID,
	pPath string,
	compiled *CompiledBus,
) {
	procMeta := make(map[string]*processDelivery)
	ensureProc := func(epName, epPath string, origin BusEdgeOrigin) *processDelivery {
		pm, ok := procMeta[epName]
		if !ok {
			pm = &processDelivery{
				name:        epName,
				path:        epPath,
				origin:      origin,
				recvOrigins: make(map[events.EventTypeID]BusEdgeOrigin),
				sendOrigins: make(map[string]BusEdgeOrigin),
			}
			procMeta[epName] = pm
		} else {
			pm.origin = strongerEdgeOrigin(pm.origin, origin)
		}
		return pm
	}

	// 1. Index outbound events emitted by this participant (deliveryIndex).
	if senderIdx, ok := compiled.pathToIdx[pPath]; ok {
		for k, entry := range compiled.deliveryIndex {
			if k.senderIdx != senderIdx {
				continue
			}
			dk := deliveryKey{ns: k.ns, et: k.et, dir: k.dir}
			names := make([]string, 0, len(entry.receivers))
			for i := range entry.receivers {
				r := &entry.receivers[i]
				if r.EndpointName == "" {
					continue
				}
				epName := r.EndpointName
				if !slices.Contains(names, epName) {
					names = append(names, epName)
				}
				pm := ensureProc(epName, r.Path, r.Origin)
				if prev, exists := pm.recvOrigins[k.et]; exists {
					pm.recvOrigins[k.et] = strongerEdgeOrigin(prev, r.Origin)
				} else {
					pm.recvOrigins[k.et] = r.Origin
				}
				if k.ns == nsID && stateETID != events.EventTypeUnknown && k.et == stateETID && k.dir == events.DirUnspecified {
					pd.recvState[epName] = true
				}
			}
			slices.Sort(names)
			pd.receive[dk] = names
		}
	}

	// 2. Index inbound send permissions from every other participant to pPath.
	for _, sFilter := range compiled.filters {
		if sFilter == nil {
			continue
		}
		if sFilter.Path == pPath {
			continue
		}
		epName := sFilter.EndpointName
		if epName == "" {
			continue
		}
		for _, msg := range slices.Sorted(maps.Keys(sFilter.SendRules)) {
			if strings.HasPrefix(msg, busCLIPrefix) {
				continue
			}
			msgNS, bareName, hasNS := events.SplitMessageName(msg)
			if namespace != "" && (!hasNS || msgNS != namespace) {
				continue
			}
			allowed, _ := compiled.MaySend(sFilter.Path, msg, pPath)
			if !allowed {
				continue
			}
			rule := sFilter.SendRules[msg]
			origin := BusOriginRegistered
			if rule != nil {
				if rule.ToOrigin.Origin != BusOriginRegistered {
					origin = rule.ToOrigin.Origin
				} else if rule.AllowOrigin.Origin != BusOriginRegistered {
					origin = rule.AllowOrigin.Origin
				}
			}
			pm := ensureProc(epName, sFilter.Path, origin)

			pd.maySend[peerSendKey{process: epName, sendType: msg}] = true
			pd.sendersByType[msg] = appendUniqueSorted(pd.sendersByType[msg], epName)

			displayToken := msg
			if hasNS && msgNS == namespace {
				pd.maySend[peerSendKey{process: epName, sendType: bareName}] = true
				pd.sendersByType[bareName] = appendUniqueSorted(pd.sendersByType[bareName], epName)
				displayToken = bareName
			}
			if !slices.Contains(pm.send, displayToken) {
				pm.send = append(pm.send, displayToken)
			}
			if prev, exists := pm.sendOrigins[displayToken]; exists {
				pm.sendOrigins[displayToken] = strongerEdgeOrigin(prev, origin)
			} else {
				pm.sendOrigins[displayToken] = origin
			}
		}
	}

	procNames := slices.Sorted(maps.Keys(procMeta))
	pd.allProcs = procNames
	pd.procs = make([]processDelivery, 0, len(procNames))
	for _, name := range procNames {
		pm := procMeta[name]
		slices.Sort(pm.send)
		pd.procs = append(pd.procs, *pm)
	}
}

func strongerEdgeOrigin(cur, next BusEdgeOrigin) BusEdgeOrigin {
	if edgeOriginPriority(next) > edgeOriginPriority(cur) {
		return next
	}
	return cur
}

func edgeOriginPriority(o BusEdgeOrigin) int {
	switch o {
	case BusOriginRequired:
		return 5
	case BusOriginWritten:
		return 4
	case BusOriginInherited:
		return 3
	case BusOriginRegistered:
		return 2
	case BusOriginDormant:
		return 1
	default:
		panic("BUG: server: invalid BusEdgeOrigin value")
	}
}

func appendUniqueSorted(list []string, val string) []string {
	if slices.Contains(list, val) {
		return list
	}
	list = append(list, val)
	slices.Sort(list)
	return list
}

func cloneDeliveryPeers(peers []DeliveryPeer) []DeliveryPeer {
	if len(peers) == 0 {
		return nil
	}
	out := make([]DeliveryPeer, len(peers))
	for i := range peers {
		out[i] = DeliveryPeer{
			Addr:            peers[i].Addr,
			Name:            peers[i].Name,
			Path:            peers[i].Path,
			ParentPath:      peers[i].ParentPath,
			SecondaryPaths:  slices.Clone(peers[i].SecondaryPaths),
			Bus:             peers[i].Bus.Clone(),
			BusOrigin:       peers[i].BusOrigin,
			BusSource:       peers[i].BusSource,
			OwnBus:          peers[i].OwnBus.Clone(),
			ParentBus:       peers[i].ParentBus.Clone(),
			ParentBusSource: peers[i].ParentBusSource,
			Dynamic:         peers[i].Dynamic,
			SessionFacts:    maps.Clone(peers[i].SessionFacts),
		}
	}
	return out
}

func deliveryPeerPath(namespace string, p *DeliveryPeer) string {
	if p.Path != "" {
		return p.Path
	}
	var tb textbuf.Buffer
	prefix := namespace
	if prefix == "" {
		prefix = "participant"
	}
	if p.Name != "" {
		return tb.Byte('/').Str(prefix).Str("/entry/").Str(p.Name).String()
	}
	if p.Addr == "" {
		return ""
	}
	return tb.Byte('/').Str(prefix).Byte('/').Str(p.Addr).String()
}

func defaultNamespaceDeclarations(namespace string) ([]rpc.MessageSendDecl, []rpc.MessageReceiveDecl) {
	if namespace == "" {
		return nil, nil
	}
	if reg := registry.Lookup(namespace); reg != nil {
		if len(reg.Sends) > 0 {
			return reg.Sends, reg.Receives
		}
		if len(reg.Receives) > 0 {
			return reg.Sends, reg.Receives
		}
	}

	// Fallback for standalone or test namespaces without a compiled-in plugin
	// registration: derive sends and receives from the events registry.
	var sends []rpc.MessageSendDecl
	var receives []rpc.MessageReceiveDecl
	seen := make(map[string]bool)

	for _, def := range events.AllMessages() {
		if def.Namespace != namespace {
			continue
		}
		full := def.FullName()
		seen[full] = true
		sends = append(sends, rpc.MessageSendDecl{
			Message:             full,
			Tags:                eventTagsToRPCTags(def.Tags),
			DefaultDestinations: slices.Clone(def.DefaultDestinations),
		})
		receives = append(receives, rpc.MessageReceiveDecl{Message: full})
	}

	dirTags := []rpc.MessageTagDecl{{
		Name:   events.TagDirection,
		Values: []string{events.DirectionReceived, events.DirectionSent},
	}}
	types := events.AllEventTypes()[namespace]
	slices.Sort(types)
	for _, et := range types {
		var tb textbuf.Buffer
		full := tb.Str(namespace).Byte(':').Str(et).String()
		if seen[full] {
			continue
		}
		seen[full] = true
		sends = append(sends, rpc.MessageSendDecl{
			Message: full,
			Tags:    dirTags,
		})
		receives = append(receives, rpc.MessageReceiveDecl{Message: full})
	}
	return sends, receives
}

func applyCentralOverrideToInput(input *BusParticipantInput, busCfg plugin.BusConfig, includeParentOverride bool) {
	if len(busCfg.Participants) == 0 {
		return
	}
	if ov, ok := busCfg.Participants[input.Path]; ok {
		input.BusSideBlock = ov.Block.Clone()
		input.BusSideSource = ov.Source
	}
	if input.BusSideBlock.IsEmpty() {
		for _, sp := range input.SecondaryPaths {
			if ov, ok := busCfg.Participants[sp]; ok {
				input.BusSideBlock = ov.Block.Clone()
				input.BusSideSource = ov.Source
				break
			}
		}
	}
	if !includeParentOverride {
		return
	}
	if input.ParentPath != "" {
		if ov, ok := busCfg.Participants[input.ParentPath]; ok {
			input.ParentBusSideBlock = ov.Block.Clone()
			input.ParentBusSideSource = ov.Source
			return
		}
	}
	// Walk ancestor prefixes of input.Path (e.g. "/bgp/peer/p1" -> "/bgp") so
	// central prefix overrides like `bus { participant /bgp { ... } }` inherit
	// into covered participants at runtime.
	cur := input.Path
	for {
		slash := strings.LastIndexByte(cur, '/')
		if slash <= 0 {
			break
		}
		cur = cur[:slash]
		if ov, ok := busCfg.Participants[cur]; ok {
			input.ParentBusSideBlock = ov.Block.Clone()
			input.ParentBusSideSource = ov.Source
			break
		}
	}
}

func applyEndpointBusConfig(ep *BusParticipantInput, busCfg plugin.BusConfig) {
	if ep.BusSideBlock.IsEmpty() {
		if blk, ok := busCfg.Endpoints[ep.EndpointName]; ok && !blk.IsEmpty() {
			ep.BusSideBlock = blk.Clone()
			ep.BusSideSource = "bus > endpoint " + ep.EndpointName
		}
	}
	if ov, ok := busCfg.Participants[ep.Path]; ok && !ov.Block.IsEmpty() {
		if ep.BusSideBlock.IsEmpty() {
			ep.BusSideBlock = ov.Block.Clone()
			ep.BusSideSource = ov.Source
		} else if ep.OwnBlock.IsEmpty() {
			ep.OwnBlock = ov.Block.Clone()
			ep.OwnSource = ov.Source
		}
	}
}

// Compiled returns the underlying CompiledBus snapshot, or nil for the empty
// graph.
func (g *DeliveryGraph) Compiled() *CompiledBus {
	if g == nil {
		return nil
	}
	return g.compiled
}

// Receivers returns the endpoint names that receive (ns, et, dir) from
// peerAddr. It performs a single map lookup after resolving peerAddr and
// allocates nothing; callers MUST treat the returned slice as read-only.
func (g *DeliveryGraph) Receivers(ns events.NamespaceID, et events.EventTypeID, dir events.Direction, peerAddr string) []string {
	if g == nil {
		return nil
	}
	pd := g.peers[peerAddr]
	if pd == nil {
		return nil
	}
	return pd.receive[deliveryKey{ns: ns, et: et, dir: dir}]
}

// ReceiversForPeer returns the sorted endpoint names receiving (ns, et, dir)
// from peerAddr with zero heap allocations.
func (g *DeliveryGraph) ReceiversForPeer(peerAddr string, ns events.NamespaceID, et events.EventTypeID, dir events.Direction) []string {
	return g.Receivers(ns, et, dir, peerAddr)
}

// MaySend reports whether processName is permitted to send sendType (either a
// bare token such as "update" or a qualified message name such as
// "bgp:update") to peerAddr. Allocates nothing on the hot path.
func (g *DeliveryGraph) MaySend(peerAddr, processName, sendType string) bool {
	if g == nil {
		return false
	}
	pd := g.peers[peerAddr]
	if pd == nil {
		return false
	}
	return pd.maySend[peerSendKey{process: processName, sendType: sendType}]
}

// SendersForPeer returns the sorted endpoint names permitted to send sendType
// (bare or qualified) to peerAddr. Allocates nothing; callers MUST treat the
// returned slice as read-only.
func (g *DeliveryGraph) SendersForPeer(peerAddr, sendType string) []string {
	if g == nil {
		return nil
	}
	pd := g.peers[peerAddr]
	// refuse-by-default: guards sendersByType -- unknown participant has no senders.
	if pd == nil {
		return nil
	}
	return pd.sendersByType[sendType]
}

// ParticipantPathForAddr returns the canonical short bus path for peerAddr, or
// empty when peerAddr is unknown.
func (g *DeliveryGraph) ParticipantPathForAddr(peerAddr string) string {
	if g == nil {
		return ""
	}
	pd := g.peers[peerAddr]
	// refuse-by-default: guards DeliveryGraph.Inspect -- unknown address has no registered path.
	if pd == nil {
		return ""
	}
	return pd.path
}

// ProcessReceivesState reports whether processName receives state events from
// peerAddr. Allocates nothing.
func (g *DeliveryGraph) ProcessReceivesState(peerAddr, processName string) bool {
	if g == nil {
		return false
	}
	pd := g.peers[peerAddr]
	if pd == nil {
		return false
	}
	return pd.recvState[processName]
}

// ProcessListForPeer returns the sorted list of all endpoint names interacting
// (receiving or sending) with peerAddr. Callers MUST treat the returned slice
// as read-only.
func (g *DeliveryGraph) ProcessListForPeer(peerAddr string) []string {
	if g == nil {
		return nil
	}
	pd := g.peers[peerAddr]
	// refuse-by-default: guards allProcs -- unknown participant has no endpoint edges.
	if pd == nil {
		return nil
	}
	return pd.allProcs
}

// EdgeDiff compares g (previous graph) against next for peerAddr and returns
// the sorted endpoint names added in next and removed from g.
func (g *DeliveryGraph) EdgeDiff(next *DeliveryGraph, peerAddr string) (added, removed []string) {
	oldList := g.ProcessListForPeer(peerAddr)
	newList := next.ProcessListForPeer(peerAddr)
	for _, name := range newList {
		if !slices.Contains(oldList, name) {
			added = append(added, name)
		}
	}
	for _, name := range oldList {
		if !slices.Contains(newList, name) {
			removed = append(removed, name)
		}
	}
	return added, removed
}

// Stage2FilterFor projects the compiled bus filter of endpointName into the
// wire rpc.BusParticipantFilter shape, attaching resolved remote addresses to
// every target participant. Returns nil when endpointName is not present in
// the compiled bus.
func (g *DeliveryGraph) Stage2FilterFor(endpointName string) *rpc.BusParticipantFilter {
	if g == nil {
		return nil
	}
	if g.compiled == nil {
		return nil
	}
	if endpointName == "" {
		return nil
	}
	f := g.compiled.Filter(plugin.EndpointPath(endpointName))
	if f == nil {
		for _, cand := range g.compiled.filters {
			if cand != nil && cand.EndpointName == endpointName {
				f = cand
				break
			}
		}
	}
	// refuse-by-default: guards BusParticipantFilter -- unknown endpoint receives no Stage 2 filter.
	if f == nil {
		return nil
	}

	buildTarget := func(targetPath string) rpc.BusFilterTarget {
		t := rpc.BusFilterTarget{
			Path:    targetPath,
			Address: g.pathToAddr[targetPath],
		}
		if tf := g.compiled.Filter(targetPath); tf != nil {
			t.EndpointName = tf.EndpointName
			t.SecondaryPaths = slices.Clone(tf.SecondaryPaths)
			if t.Address == "" {
				for _, sp := range tf.SecondaryPaths {
					if addr := g.pathToAddr[sp]; addr != "" {
						t.Address = addr
						break
					}
				}
			}
		}
		return t
	}

	out := &rpc.BusParticipantFilter{
		ParticipantPath: f.Path,
		EndpointName:    f.EndpointName,
		SendAll:         f.SendAllowAllDeclared,
		SendNone:        f.SendAllowNone,
		SendToAll:       f.SendToAll,
		SendToNone:      f.SendToNone,
		ReceiveAll:      f.ReceiveAllowAllDeclared,
		ReceiveNone:     f.ReceiveAllowNone,
		ReceiveFromAll:  f.ReceiveFromAll,
		ReceiveFromNone: f.ReceiveFromNone,
	}

	for _, msg := range slices.Sorted(maps.Keys(f.SendRules)) {
		r := f.SendRules[msg]
		if r == nil {
			continue
		}
		targets := make([]rpc.BusFilterTarget, 0, len(r.ResolvedPaths))
		for _, tp := range r.ResolvedPaths {
			targets = append(targets, buildTarget(tp))
		}
		out.SendRules = append(out.SendRules, rpc.BusFilterRule{
			Message:    r.Message,
			Allowed:    r.Allowed,
			TargetAll:  r.ToAll,
			TargetNone: r.ToNone,
			Targets:    targets,
			Tags:       cloneTagValueMap(r.OutboundTagMap),
		})
	}

	for _, msg := range slices.Sorted(maps.Keys(f.ReceiveRules)) {
		r := f.ReceiveRules[msg]
		if r == nil {
			continue
		}
		targets := make([]rpc.BusFilterTarget, 0, len(r.ResolvedPaths))
		for _, tp := range r.ResolvedPaths {
			targets = append(targets, buildTarget(tp))
		}
		out.ReceiveRules = append(out.ReceiveRules, rpc.BusFilterRule{
			Message:    r.Message,
			Allowed:    r.Allowed,
			TargetAll:  r.FromAll,
			TargetNone: r.FromNone,
			Targets:    targets,
			Tags:       cloneTagValueMap(r.InboundTagMap),
		})
	}

	return out
}

// Inspect returns the graph as plain inspection data ordered by address.
func (g *DeliveryGraph) Inspect() []PeerEdges {
	if g == nil {
		return nil
	}
	out := make([]PeerEdges, 0, len(g.order))
	for _, addr := range g.order {
		pd := g.peers[addr]
		entry := PeerEdges{
			Peer:       addr,
			Name:       pd.name,
			Path:       pd.path,
			ParentPath: pd.parentPath,
			Dynamic:    pd.dynamic,
			Processes:  make([]ProcessEdges, 0, len(pd.procs)),
		}
		for _, proc := range pd.procs {
			recvTokens := g.receiveTokens(addr, proc.name)
			origins := g.tokenOriginsForProcess(proc, recvTokens)
			entry.Processes = append(entry.Processes, ProcessEdges{
				Process: proc.name,
				Path:    proc.path,
				Receive: recvTokens,
				Send:    slices.Clone(proc.send),
				Origin:  proc.origin.String(),
				Origins: origins,
			})
		}
		out = append(out, entry)
	}
	return out
}

func (g *DeliveryGraph) tokenOriginsForProcess(proc processDelivery, recvTokens []string) map[string]string {
	total := len(recvTokens) + len(proc.send)
	if total == 0 {
		return nil
	}
	origins := make(map[string]string, total)
	for _, tok := range recvTokens {
		origin := proc.origin
		if evType, _, ok := events.SplitTypeToken(g.namespace, tok); ok {
			if o, found := proc.recvOrigins[events.LookupEventTypeID(evType)]; found {
				origin = o
			}
		}
		origins[tok] = origin.String()
	}
	for _, tok := range proc.send {
		origin := proc.origin
		if o, found := proc.sendOrigins[tok]; found {
			origin = o
		}
		origins[tok] = origin.String()
	}
	return origins
}

type regEdgeGroupKey struct {
	senderPath string
	message    string
}

type regEdgeGroup struct {
	senderPath     string
	senderEndpoint string
	senderGroup    string
	message        string
	source         string
	destinations   []string
	tags           map[string][]string
	single         *DeliveryEdge
}

// InspectEdges returns the multi-namespace, origin-annotated delivery edges
// compiled in g, deduplicated per registered rule ("one registration, one
// line"). Dynamic participants carry their admitting container path in
// SenderGroup or ReceiverGroup; static participants leave those fields empty.
func (g *DeliveryGraph) InspectEdges() []DeliveryEdge {
	if g == nil {
		return []DeliveryEdge{}
	}
	if g.compiled == nil {
		return []DeliveryEdge{}
	}

	compiledEdges := g.compiled.Edges()
	if len(compiledEdges) == 0 {
		return []DeliveryEdge{}
	}

	out := make([]DeliveryEdge, 0, len(compiledEdges))
	regGroups := make(map[regEdgeGroupKey]*regEdgeGroup)
	var regGroupOrder []regEdgeGroupKey

	for i := range compiledEdges {
		ce := &compiledEdges[i]
		sFilter := g.compiled.Filter(ce.SenderPath)
		rFilter := g.compiled.Filter(ce.ReceiverPath)

		senderEP := ""
		if sFilter != nil {
			senderEP = sFilter.EndpointName
		}
		recvEP := ce.ReceiverEndpoint
		if recvEP == "" && rFilter != nil {
			recvEP = rFilter.EndpointName
		}
		senderGroup := g.dynamicGroupByPath[ce.SenderPath]
		recvGroup := g.dynamicGroupByPath[ce.ReceiverPath]

		var sendRule *BusCompiledSendRule
		if sFilter != nil {
			sendRule = sFilter.SendRules[ce.Message]
		}
		var recvRule *BusCompiledReceiveRule
		if rFilter != nil {
			recvRule = rFilter.ReceiveRules[ce.Message]
		}

		source := compiledEdgeSource(ce, sFilter, sendRule, rFilter, recvRule)
		tags := compiledEdgeTags(sendRule, recvRule)

		// Collapse registered multi-destination sends from endpoints (or to
		// non-endpoint participants) into one row ("one registration, one line").
		if ce.Origin == BusOriginRegistered && ce.SenderPath != "" && (recvEP == "" || senderEP != "") && sendRule != nil && sendRule.ToOrigin.Origin == BusOriginRegistered {
			key := regEdgeGroupKey{senderPath: ce.SenderPath, message: ce.Message}
			grp, exists := regGroups[key]
			if !exists {
				edgeCopy := DeliveryEdge{
					SenderPath:       ce.SenderPath,
					SenderEndpoint:   senderEP,
					SenderGroup:      senderGroup,
					ReceiverPath:     ce.ReceiverPath,
					ReceiverEndpoint: recvEP,
					ReceiverGroup:    recvGroup,
					Message:          ce.Message,
					Origin:           ce.Origin.String(),
					Source:           source,
					Destinations:     []string{ce.ReceiverPath},
					Tags:             tags,
				}
				grp = &regEdgeGroup{
					senderPath:     ce.SenderPath,
					senderEndpoint: senderEP,
					senderGroup:    senderGroup,
					message:        ce.Message,
					source:         registrationSourceLabel(sFilter, ce.SenderPath),
					destinations:   []string{ce.ReceiverPath},
					tags:           tags,
					single:         &edgeCopy,
				}
				regGroups[key] = grp
				regGroupOrder = append(regGroupOrder, key)
			} else {
				grp.single = nil
				if !slices.Contains(grp.destinations, ce.ReceiverPath) {
					grp.destinations = append(grp.destinations, ce.ReceiverPath)
				}
			}
			continue
		}

		var dests []string
		if ce.ReceiverPath != "" && ce.Origin != BusOriginDormant {
			dests = []string{ce.ReceiverPath}
		}
		out = append(out, DeliveryEdge{
			SenderPath:       ce.SenderPath,
			SenderEndpoint:   senderEP,
			SenderGroup:      senderGroup,
			ReceiverPath:     ce.ReceiverPath,
			ReceiverEndpoint: recvEP,
			ReceiverGroup:    recvGroup,
			Message:          ce.Message,
			Origin:           ce.Origin.String(),
			Source:           source,
			Destinations:     dests,
			Tags:             tags,
		})
	}

	for _, key := range regGroupOrder {
		grp := regGroups[key]
		if grp.single != nil {
			out = append(out, *grp.single)
			continue
		}
		slices.Sort(grp.destinations)
		out = append(out, DeliveryEdge{
			SenderPath:     grp.senderPath,
			SenderEndpoint: grp.senderEndpoint,
			SenderGroup:    grp.senderGroup,
			Message:        grp.message,
			Origin:         BusOriginRegistered.String(),
			Source:         grp.source,
			Destinations:   grp.destinations,
			Tags:           grp.tags,
		})
	}

	slices.SortFunc(out, compareDeliveryEdges)
	return out
}

func compareDeliveryEdges(a, b DeliveryEdge) int {
	if a.SenderPath != b.SenderPath {
		return strings.Compare(a.SenderPath, b.SenderPath)
	}
	if a.ReceiverPath != b.ReceiverPath {
		return strings.Compare(a.ReceiverPath, b.ReceiverPath)
	}
	if a.Message != b.Message {
		return strings.Compare(a.Message, b.Message)
	}
	if a.Origin != b.Origin {
		return strings.Compare(a.Origin, b.Origin)
	}
	return strings.Compare(a.Source, b.Source)
}

func registrationSourceLabel(f *CompiledParticipantFilter, fallbackPath string) string {
	label := fallbackPath
	if f != nil {
		if f.EndpointName != "" {
			label = f.EndpointName
		} else if f.Path != "" {
			label = f.Path
		}
	}
	if label == "" {
		return "registration"
	}
	return "registration of " + label
}

func compiledEdgeSource(
	ce *BusCompiledEdge,
	sFilter *CompiledParticipantFilter,
	sendRule *BusCompiledSendRule,
	rFilter *CompiledParticipantFilter,
	recvRule *BusCompiledReceiveRule,
) string {
	switch ce.Origin {
	case BusOriginDormant:
		if ce.TagSummary != "" {
			if ce.SenderPath == "" {
				return enrichDormantReason(ce.Message, ce.TagSummary)
			}
			return ce.TagSummary
		}
		return enrichDormantReason(ce.Message, "no active sender")
	case BusOriginRequired:
		if src := firstSourceWithOrigin(BusOriginRequired, sendRule, recvRule); src != "" {
			return src
		}
		if rFilter != nil && rFilter.EndpointName != "" {
			return "feature requirement for " + rFilter.EndpointName
		}
		if sFilter != nil && sFilter.EndpointName != "" {
			return "feature requirement for " + sFilter.EndpointName
		}
		return "feature requirement"
	case BusOriginWritten:
		if src := firstSourceWithOrigin(BusOriginWritten, sendRule, recvRule); src != "" {
			return src
		}
		return ce.SenderPath
	case BusOriginInherited:
		if src := firstSourceWithOrigin(BusOriginInherited, sendRule, recvRule); src != "" {
			return "inherited from " + src
		}
		if ce.ParentPath != "" {
			return "inherited from " + ce.ParentPath
		}
		return "inherited"
	case BusOriginRegistered:
		if rFilter != nil && rFilter.EndpointName != "" {
			return registrationSourceLabel(rFilter, ce.ReceiverPath)
		}
		return registrationSourceLabel(sFilter, ce.SenderPath)
	default:
		panic("BUG: server: invalid BusEdgeOrigin value")
	}
}

func firstSourceWithOrigin(want BusEdgeOrigin, sendRule *BusCompiledSendRule, recvRule *BusCompiledReceiveRule) string {
	if sendRule != nil {
		if sendRule.ToOrigin.Origin == want && len(sendRule.ToOrigin.Sources) > 0 {
			return textbuf.Join(sendRule.ToOrigin.Sources, ", ")
		}
		if sendRule.AllowOrigin.Origin == want && len(sendRule.AllowOrigin.Sources) > 0 {
			return textbuf.Join(sendRule.AllowOrigin.Sources, ", ")
		}
		for _, tagName := range slices.Sorted(maps.Keys(sendRule.TagOrigins)) {
			orig := sendRule.TagOrigins[tagName]
			if orig.Origin == want && len(orig.Sources) > 0 {
				return textbuf.Join(orig.Sources, ", ")
			}
		}
	}
	if recvRule != nil {
		if recvRule.FromOrigin.Origin == want && len(recvRule.FromOrigin.Sources) > 0 {
			return textbuf.Join(recvRule.FromOrigin.Sources, ", ")
		}
		if recvRule.AllowOrigin.Origin == want && len(recvRule.AllowOrigin.Sources) > 0 {
			return textbuf.Join(recvRule.AllowOrigin.Sources, ", ")
		}
		for _, tagName := range slices.Sorted(maps.Keys(recvRule.TagOrigins)) {
			orig := recvRule.TagOrigins[tagName]
			if orig.Origin == want && len(orig.Sources) > 0 {
				return textbuf.Join(orig.Sources, ", ")
			}
		}
	}
	return ""
}

func compiledEdgeTags(sendRule *BusCompiledSendRule, recvRule *BusCompiledReceiveRule) map[string][]string {
	if recvRule != nil && len(recvRule.InboundTagMap) > 0 {
		return cloneTagValueMap(recvRule.InboundTagMap)
	}
	if sendRule != nil && len(sendRule.OutboundTagMap) > 0 {
		return cloneTagValueMap(sendRule.OutboundTagMap)
	}
	return nil
}

// receiveTokens reads the event types indexed for one endpoint on one address
// back out of the index, formatted as operator tokens.
func (g *DeliveryGraph) receiveTokens(peerAddr, processName string) []string {
	pd := g.peers[peerAddr]
	type nsET struct {
		ns events.NamespaceID
		et events.EventTypeID
	}
	seen := make(map[nsET]struct{}, len(pd.receive))
	var out []string
	for k := range pd.receive {
		pair := nsET{ns: k.ns, et: k.et}
		if _, done := seen[pair]; done {
			continue
		}
		seen[pair] = struct{}{}
		hasRec := slices.Contains(g.Receivers(k.ns, k.et, events.DirReceived, peerAddr), processName)
		hasSent := slices.Contains(g.Receivers(k.ns, k.et, events.DirSent, peerAddr), processName)
		hasAny := slices.Contains(g.Receivers(k.ns, k.et, events.DirUnspecified, peerAddr), processName)
		if !hasRec && !hasSent && !hasAny {
			continue
		}
		dir := events.DirUnspecified
		switch {
		case hasRec && hasSent:
			dir = events.DirBoth
		case hasRec:
			dir = events.DirReceived
		case hasSent:
			dir = events.DirSent
		}
		out = append(out, events.DirectionToken(k.et.String(), dir))
	}
	slices.Sort(out)
	return out
}

// Len returns the number of addresses indexed in g.
func (g *DeliveryGraph) Len() int {
	if g == nil {
		return 0
	}
	return len(g.order)
}

// SetBusConfig stores the parsed central bus configuration on s for subsequent
// delivery-graph compilations. Safe for concurrent use.
func (s *Server) SetBusConfig(cfg plugin.BusConfig) {
	cloned := cfg.Clone()
	s.busConfig.Store(&cloned)
}

// SetBusConfigTree publishes the active (or in-flight reload) config tree for
// delivery-graph singleton-participant bus block extraction. Keeping a lock-free
// atomic snapshot on Server avoids calling back into s.reactor.GetConfigTree()
// while the reactor holds r.mu during publishDeliveryGraphLocked.
func (s *Server) SetBusConfigTree(tree map[string]any) {
	if tree == nil {
		s.busConfigTree.Store(nil)
		return
	}
	t := tree
	s.busConfigTree.Store(&t)
}

// currentBusConfigTree returns the atomic config-tree snapshot governing
// delivery-graph singleton-participant bus block extraction, or nil when none
// has been published via SetBusConfigTree. It never calls back into
// s.reactor.GetConfigTree() so delivery-graph rebuilds during startup, reload,
// and reactor callbacks stay lock-order independent.
func (s *Server) currentBusConfigTree() map[string]any {
	if p := s.busConfigTree.Load(); p != nil {
		return *p
	}
	return nil
}

// BusConfig returns the stored central bus configuration (empty if unset).
// Safe for concurrent use.
func (s *Server) BusConfig() plugin.BusConfig {
	if p := s.busConfig.Load(); p != nil {
		return p.Clone()
	}
	return plugin.BusConfig{}
}

// DeliveryGraph returns the current compiled delivery index. Never nil: before
// the first publication it returns the empty graph, which feeds nobody.
func (s *Server) DeliveryGraph() *DeliveryGraph {
	if g := s.deliveryGraph.Load(); g != nil {
		return g
	}
	return emptyDeliveryGraph
}

// PeerProcessEdges returns the resolved ProcessEdges for one peer address,
// matching the entries Inspect produces. Returns nil if g is nil or peerAddr
// is absent.
func (g *DeliveryGraph) PeerProcessEdges(peerAddr string) []ProcessEdges {
	if g == nil {
		return nil
	}
	if peerAddr == "" {
		return nil
	}
	pd, ok := g.peers[peerAddr]
	// refuse-by-default: guards DeliveryGraph.Inspect -- a peer absent from the graph snapshot has no resolved process edges.
	if !ok {
		return nil
	}
	out := make([]ProcessEdges, 0, len(pd.procs))
	for _, proc := range pd.procs {
		recvTokens := g.receiveTokens(peerAddr, proc.name)
		origins := g.tokenOriginsForProcess(proc, recvTokens)
		out = append(out, ProcessEdges{
			Process: proc.name,
			Path:    proc.path,
			Receive: recvTokens,
			Send:    slices.Clone(proc.send),
			Origin:  proc.origin.String(),
			Origins: origins,
		})
	}
	return out
}

// PreviewDeliveryGraph compiles a candidate DeliveryGraph from s's current
// BusConfig, active/declared endpoints, and peers without publishing it.
func (s *Server) PreviewDeliveryGraph(namespace string, peers []DeliveryPeer) (*DeliveryGraph, error) {
	busCfg := s.BusConfig()
	endpoints := s.collectBusEndpointInputs(namespace, busCfg)
	return BuildDeliveryGraph(namespace, peers, busCfg, endpoints)
}

// SetDeliveryGraph compiles the bus from s's stored BusConfig, active and
// declared endpoints, and peers, then atomically swaps the resulting
// DeliveryGraph into s.
func (s *Server) SetDeliveryGraph(namespace string, peers []DeliveryPeer) error {
	busCfg := s.BusConfig()
	endpoints := s.collectBusEndpointInputs(namespace, busCfg)
	return s.UpdateDeliveryGraphWithEndpoints(namespace, peers, endpoints)
}

// UpdateDeliveryGraph compiles and atomically publishes a new DeliveryGraph
// from s's stored BusConfig, active endpoints, and peers. Compilation errors
// are logged at WARN rather than swapping in a broken graph.
func (s *Server) UpdateDeliveryGraph(namespace string, peers []DeliveryPeer) {
	if err := s.SetDeliveryGraph(namespace, peers); err != nil {
		logger().Warn("delivery graph compilation failed; retaining previous index",
			"namespace", namespace, "err", err, "code", CodeDoctorPluginDeliveryDisagreement)
	}
}

// RefreshDeliveryGraph recompiles the bus delivery index from s's current
// BusConfig, active/configured endpoints, and retained participant set (even
// when no protocol participants have been published yet), swaps the new
// snapshot in atomically, and publishes the updated plugin command profiles.
func (s *Server) RefreshDeliveryGraph() {
	cur := s.DeliveryGraph()
	s.UpdateDeliveryGraph(cur.namespace, cur.disseminationPeers)
}

// participantFilterPushTimeout bounds each asynchronous participant-filter
// rebuild delivery so one slow plugin cannot stall updates to other plugins.
const participantFilterPushTimeout = 5 * time.Second

// UpdateDeliveryGraphWithEndpoints compiles peers with explicit endpoint
// participant inputs and atomically swaps the resulting DeliveryGraph into s.
func (s *Server) UpdateDeliveryGraphWithEndpoints(
	namespace string,
	peers []DeliveryPeer,
	extraEndpoints []BusParticipantInput,
) error {
	g, err := BuildDeliveryGraph(namespace, peers, s.BusConfig(), extraEndpoints)
	if err != nil {
		return err
	}
	if prev := s.deliveryGraph.Swap(g); prev != nil {
		s.prevDeliveryGraph.Store(prev)
	}
	publishPluginProfileServer(s)
	logger().Debug("delivery graph published", "namespace", namespace, "entries", g.Len())
	s.reconcileDelivery(nil)
	s.enqueueParticipantFilterPush(g)
	return nil
}

// enqueueParticipantFilterPush schedules an asynchronous push of updated
// participant filters from g to all running plugins. Never blocks callers:
// rapid rebuilds coalesce onto the latest snapshot via pendingFilterGraph and
// one long-lived worker per Server (stopped in Server.Stop).
func (s *Server) enqueueParticipantFilterPush(g *DeliveryGraph) {
	if g == nil {
		return
	}
	if s.stopping.Load() {
		return
	}
	pm := s.procManager.Load()
	if pm == nil {
		return
	}
	hasTarget := false
	for _, proc := range pm.AllProcesses() {
		if proc == nil {
			continue
		}
		if !proc.Running() {
			continue
		}
		if proc.Stage() != plugin.StageRunning {
			continue
		}
		if proc.Conn() == nil {
			continue
		}
		if g.Stage2FilterFor(proc.Name()) != nil {
			hasTarget = true
			break
		}
	}
	if !hasTarget {
		return
	}

	s.pendingFilterGraph.Store(g)

	s.filterPushMu.Lock()
	if s.filterPushStopped {
		s.filterPushMu.Unlock()
		return
	}
	if s.filterPushNotify == nil {
		workerCtx, workerCancel := context.WithCancel(context.Background())
		notify := make(chan struct{}, 1)
		done := make(chan struct{})
		s.filterPushNotify = notify
		s.filterPushCancel = workerCancel
		s.filterPushDone = done
		go s.runParticipantFilterPushWorker(workerCtx, notify, done)
	}
	notify := s.filterPushNotify
	s.filterPushMu.Unlock()

	select {
	case notify <- struct{}{}:
	default:
	}
}

func (s *Server) runParticipantFilterPushWorker(ctx context.Context, notify <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	for {
		select {
		case <-ctx.Done():
			return
		case <-notify:
			g := s.pendingFilterGraph.Swap(nil)
			if g == nil {
				continue
			}
			s.pushParticipantFilters(ctx, g)
		}
	}
}

func (s *Server) stopParticipantFilterPushWorker() {
	s.filterPushMu.Lock()
	if s.filterPushStopped {
		s.filterPushMu.Unlock()
		return
	}
	s.filterPushStopped = true
	cancel := s.filterPushCancel
	done := s.filterPushDone
	s.filterPushMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (s *Server) pushParticipantFilters(ctx context.Context, g *DeliveryGraph) {
	pm := s.procManager.Load()
	if pm == nil {
		return
	}
	for _, proc := range pm.AllProcesses() {
		if ctx.Err() != nil {
			return
		}
		if s.stopping.Load() {
			return
		}
		if proc == nil {
			continue
		}
		if !proc.Running() {
			continue
		}
		if proc.Stage() != plugin.StageRunning {
			continue
		}
		conn := proc.Conn()
		if conn == nil {
			continue
		}
		filter := g.Stage2FilterFor(proc.Name())
		if filter == nil {
			continue
		}
		s.deliverParticipantFilter(ctx, conn, proc, filter)
	}
}

func (s *Server) deliverParticipantFilter(
	baseCtx context.Context,
	conn *plugipc.PluginConn,
	proc *process.Process,
	filter *rpc.BusParticipantFilter,
) {
	ctx, cancel := context.WithTimeout(baseCtx, participantFilterPushTimeout)
	defer cancel()
	err := conn.SendParticipantFilter(ctx, filter)
	if err == nil {
		return
	}
	if s.stopping.Load() {
		return
	}
	if baseCtx.Err() != nil {
		return
	}
	if isIgnorableParticipantFilterError(err) {
		return
	}
	logger().Warn("participant filter update delivery failed",
		"plugin", proc.Name(), "error", err, "code", CodeDoctorPluginDeliveryDisagreement)
}

func isIgnorableParticipantFilterError(err error) bool {
	if errors.Is(err, rpc.ErrMuxConnClosed) {
		return true
	}
	if errors.Is(err, rpc.ErrBridgeClosed) {
		return true
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	if errors.Is(err, io.ErrClosedPipe) {
		return true
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	msg := err.Error()
	if strings.Contains(msg, "not-understood") {
		return true
	}
	if strings.Contains(msg, "unknown method") {
		return true
	}
	if strings.Contains(msg, "use of closed network connection") {
		return true
	}
	return strings.Contains(msg, "read/write on closed pipe")
}

// collectBusEndpointInputs collects BusParticipantInput entries for all active
// processes, configured endpoints, and bus > endpoint entries known to s.
func (s *Server) collectBusEndpointInputs(namespace string, busCfg plugin.BusConfig) []BusParticipantInput {
	seen := make(map[string]bool)
	var out []BusParticipantInput
	cfgTree := s.currentBusConfigTree()

	addEndpoint := func(epName, regName string, liveReg *plugin.PluginRegistration) {
		if epName == "" {
			return
		}
		if seen[epName] {
			return
		}
		seen[epName] = true
		if regName == namespace && epName == namespace {
			return
		}
		if regName == "" && (liveReg == nil || (!liveReg.Done && len(liveReg.Sends) == 0 && len(liveReg.Receives) == 0 && !liveReg.WantsValidateOpen)) {
			// External endpoint whose Stage 1 registration has not arrived yet:
			// NewProcess initializes an empty PluginRegistration (Done=false),
			// so wait until Stage 1 completes or declarations are populated.
			return
		}

		var sends []rpc.MessageSendDecl
		var receives []rpc.MessageReceiveDecl
		var singletonPath string

		if liveReg != nil && (len(liveReg.Sends) > 0 || len(liveReg.Receives) > 0) {
			sends = liveReg.Sends
			receives = liveReg.Receives
		}
		if regName != "" {
			if compiled := registry.Lookup(regName); compiled != nil {
				if len(sends) == 0 && len(receives) == 0 {
					sends = compiled.Sends
					receives = compiled.Receives
				}
				if compiled.Name != namespace {
					singletonPath = singletonParticipantPath(compiled.ParticipantNodes)
				}
			}
		}
		if liveReg != nil && liveReg.WantsValidateOpen && namespace != "" {
			stateMsg := namespace + ":state"
			hasState := slices.ContainsFunc(receives, func(d rpc.MessageReceiveDecl) bool {
				return d.Message == stateMsg
			})
			if !hasState {
				receives = append(slices.Clone(receives), rpc.MessageReceiveDecl{Message: stateMsg})
			}
		}

		epPath := plugin.EndpointPath(epName)
		canonicalPath := epPath
		var secondary []string
		var ownBlock plugin.BusBlock
		var ownOrigin BusEdgeOrigin
		var ownSource string
		if singletonPath != "" {
			if singletonPath != epPath {
				canonicalPath = singletonPath
				secondary = []string{epPath}
			}
			ownBlock, ownSource = extractSingletonBusBlock(cfgTree, singletonPath)
			if !ownBlock.IsEmpty() {
				ownOrigin = BusOriginWritten
			}
		}

		out = append(out, BusParticipantInput{
			Path:           canonicalPath,
			EndpointName:   epName,
			SecondaryPaths: secondary,
			Sends:          sends,
			Receives:       receives,
			OwnBlock:       ownBlock,
			OwnOrigin:      ownOrigin,
			OwnSource:      ownSource,
		})
	}

	if pm := s.procManager.Load(); pm != nil {
		for _, proc := range pm.AllProcesses() {
			addEndpoint(proc.Name(), proc.RegistrationName(), proc.Registration())
		}
	}

	if s.config != nil {
		for _, pc := range s.config.Plugins {
			addEndpoint(pc.Name, pc.CompiledIdentity(), nil)
		}
	}

	for _, epName := range slices.Sorted(maps.Keys(busCfg.Endpoints)) {
		regName := ""
		// Only endpoints absent from pm and s.config reach here. Never infer a
		// compiled registration when cfgTree declares the endpoint as external
		// (AC-13, R-4).
		if !isExternalBusEndpointInTree(cfgTree, epName) && registry.Lookup(epName) != nil {
			regName = epName
		}
		addEndpoint(epName, regName, nil)
	}

	return out
}

func isExternalBusEndpointInTree(cfgTree map[string]any, epName string) bool {
	if len(cfgTree) == 0 {
		return false
	}
	busMap, ok := cfgTree[configSectionBus].(map[string]any)
	if !ok {
		return false
	}
	endpoints, ok := busMap["endpoint"].(map[string]any)
	if !ok {
		return false
	}
	epMap, ok := endpoints[epName].(map[string]any)
	if !ok {
		return false
	}
	_, hasExternal := epMap["external"]
	return hasExternal
}

func singletonParticipantPath(nodes []rpc.ParticipantNodeDecl) string {
	if len(nodes) != 1 {
		return ""
	}
	if nodes[0].SchemaPath == "" {
		return ""
	}
	trimmed := strings.TrimPrefix(nodes[0].SchemaPath, "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	segs := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		if _, local, ok := strings.Cut(part, ":"); ok {
			segs = append(segs, local)
		} else {
			segs = append(segs, part)
		}
	}
	if len(segs) == 0 {
		return ""
	}
	return "/" + textbuf.Join(segs, "/")
}

// extractSingletonBusBlock reads the optional `bus` container nested under a
// singleton participant's canonical config path (e.g. "/bgp/route-server" ->
// bgp > route-server > bus) from cfgTree.
func extractSingletonBusBlock(cfgTree map[string]any, singletonPath string) (plugin.BusBlock, string) {
	if len(cfgTree) == 0 {
		return plugin.BusBlock{}, ""
	}
	if singletonPath == "" {
		return plugin.BusBlock{}, ""
	}
	trimmed := strings.TrimPrefix(singletonPath, "/")
	if trimmed == "" {
		return plugin.BusBlock{}, ""
	}
	cur := cfgTree
	for _, seg := range strings.Split(trimmed, "/") {
		if seg == "" {
			continue
		}
		next, ok := cur[seg].(map[string]any)
		if !ok {
			return plugin.BusBlock{}, ""
		}
		cur = next
	}
	busMap, ok := cur[configSectionBus].(map[string]any)
	if !ok {
		return plugin.BusBlock{}, ""
	}
	source := singletonPath + " > bus"
	parsed, err := plugin.ParseBusBlockFromMap(busMap, source)
	if err != nil {
		logger().Warn("failed to parse singleton participant bus block",
			"path", singletonPath, "err", err, "code", CodeDoctorPluginDeliveryDisagreement)
		return plugin.BusBlock{}, ""
	}
	return parsed, source
}

// DiscardRuntimeSubscriptions drops every live `subscribe` override.
func (s *Server) DiscardRuntimeSubscriptions() {
	if s.subscriptions == nil {
		return
	}
	s.subscriptions.clearRuntimeOverrides()
}

// PeerScopedProcs returns the processes that take delivery of one scoped event:
// the intersection of the process's live subscription and the compiled
// DeliveryGraph receivers for peerAddr. Allocates nothing beyond getMatching.
func (s *Server) PeerScopedProcs(ns events.NamespaceID, et events.EventTypeID, dir events.Direction, peerAddr, peerName string) []*process.Process {
	if s.subscriptions == nil {
		return nil
	}
	procs := s.subscriptions.getMatching(ns, et, dir, peerAddr, peerName)
	if len(procs) == 0 {
		return procs
	}
	granted := s.DeliveryGraph().Receivers(ns, et, dir, peerAddr)
	if len(granted) == 0 {
		return nil
	}
	kept := procs[:0]
	for _, proc := range procs {
		if slices.Contains(granted, proc.Name()) {
			kept = append(kept, proc)
		}
	}
	return kept
}

// PeerScopedProcsNamed returns the subset of PeerScopedProcs whose endpoint
// name appears in names. Every scoped lifecycle delivery passes through the
// compiled bus filter with no bypass.
func (s *Server) PeerScopedProcsNamed(ns events.NamespaceID, et events.EventTypeID, dir events.Direction, peerAddr, peerName string, names []string) []*process.Process {
	if len(names) == 0 {
		return nil
	}
	procs := s.PeerScopedProcs(ns, et, dir, peerAddr, peerName)
	if len(procs) == 0 {
		return nil
	}
	kept := procs[:0]
	for _, proc := range procs {
		if slices.Contains(names, proc.Name()) {
			kept = append(kept, proc)
		}
	}
	return kept
}

// PeerDeclaresSend reports whether the peer participant at peerAddr declares
// sending (ns, et) in its compiled bus filter. When the peer is unknown or the
// graph carries no compiled bus, it returns true so callers stay on the
// fail-closed PeerScopedProcs path.
func (g *DeliveryGraph) PeerDeclaresSend(peerAddr string, ns events.NamespaceID, et events.EventTypeID) bool {
	if g == nil {
		return true
	}
	if g.compiled == nil {
		return true
	}
	pd, ok := g.peers[peerAddr]
	// refuse-by-default: guards Server.BroadcastEvent -- an unknown peer stays on the fail-closed PeerScopedProcs path.
	if !ok {
		return true
	}
	f := g.compiled.Filter(pd.path)
	if f == nil {
		return true
	}
	_, declared := f.SendRules[ns.String()+":"+et.String()]
	return declared
}

// EndpointAcceptsFrom reports whether endpointName's compiled inbound receive
// filter in g permits receiving (ns, et) from peerAddr, independent of
// peerAddr's outbound send rules. Used during pre-publish peer-attach delivery
// (PeerScopedProcsBind) where the caller stages the peer's new send block
// before publishing the next DeliveryGraph snapshot.
func (g *DeliveryGraph) EndpointAcceptsFrom(peerAddr, endpointName string, ns events.NamespaceID, et events.EventTypeID) bool {
	if g == nil {
		return false
	}
	if g.compiled == nil {
		return false
	}
	if g.namespace == "" {
		return false
	}
	if ns != g.ns {
		return false
	}
	if endpointName == "" {
		return false
	}
	pd := g.peers[peerAddr]
	if pd == nil {
		return false
	}
	epFilter := g.compiled.Filter(plugin.EndpointPath(endpointName))
	if epFilter == nil {
		for _, cand := range g.compiled.filters {
			if cand == nil {
				continue
			}
			if cand.EndpointName == endpointName {
				epFilter = cand
				break
			}
		}
	}
	if epFilter == nil {
		return false
	}
	var tb textbuf.Buffer
	msg := tb.Str(g.namespace).Byte(':').Str(et.String()).String()
	rule := epFilter.ReceiveRules[msg]
	if rule == nil {
		return false
	}
	if !rule.Allowed {
		return false
	}
	if rule.FromNone {
		return false
	}
	if rule.FromAll {
		return true
	}
	peerIdx, hasPeerIdx := g.compiled.pathToIdx[pd.path]
	peerFilter := g.compiled.Filter(pd.path)
	for _, pat := range rule.FromPaths {
		if hasPeerIdx {
			if resolvedIdx, ok := g.compiled.pathToIdx[pat]; ok && resolvedIdx == peerIdx {
				return true
			}
		}
		if plugin.PathCovers(pat, pd.path) {
			return true
		}
		if peerFilter != nil {
			for _, sp := range peerFilter.SecondaryPaths {
				if plugin.PathCovers(pat, sp) {
					return true
				}
			}
		}
		if pd.parentPath != "" && plugin.PathCovers(pat, pd.parentPath) {
			return true
		}
	}
	return false
}

// PeerScopedProcsBind returns the subset of subscribed processes in names
// admitted either by the current compiled DeliveryGraph or (for pre-publish
// peer-attach delivery while the peer's new send block is staged) by the
// endpoint's own compiled receive filter for peerAddr. An endpoint whose
// compiled receive filter rejects the message or peerAddr receives nothing.
func (s *Server) PeerScopedProcsBind(ns events.NamespaceID, et events.EventTypeID, dir events.Direction, peerAddr, peerName string, names []string) []*process.Process {
	if s.subscriptions == nil {
		return nil
	}
	if len(names) == 0 {
		return nil
	}
	procs := s.subscriptions.getMatching(ns, et, dir, peerAddr, peerName)
	if len(procs) == 0 {
		return nil
	}
	g := s.DeliveryGraph()
	granted := g.Receivers(ns, et, dir, peerAddr)
	var preview *DeliveryGraph
	previewLoaded := false
	kept := procs[:0]
	for _, proc := range procs {
		name := proc.Name()
		if !slices.Contains(names, name) {
			continue
		}
		if slices.Contains(granted, name) {
			kept = append(kept, proc)
			continue
		}
		if g.EndpointAcceptsFrom(peerAddr, name, ns, et) {
			kept = append(kept, proc)
			continue
		}
		if !previewLoaded {
			previewLoaded = true
			if cand, err := s.PreviewDeliveryGraph(g.namespace, g.disseminationPeers); err == nil {
				preview = cand
			}
		}
		if preview != nil && (slices.Contains(preview.Receivers(ns, et, dir, peerAddr), name) || preview.EndpointAcceptsFrom(peerAddr, name, ns, et)) {
			kept = append(kept, proc)
		}
	}
	return kept
}

// PeerScopedProcsUnbind returns the subset of subscribed processes in names
// admitted by either the current compiled DeliveryGraph or the immediately
// preceding DeliveryGraph (for terminal detach delivery right after a graph
// swap removes an edge). A process admitted by neither receives nothing.
func (s *Server) PeerScopedProcsUnbind(ns events.NamespaceID, et events.EventTypeID, dir events.Direction, peerAddr, peerName string, names []string) []*process.Process {
	if s.subscriptions == nil {
		return nil
	}
	if len(names) == 0 {
		return nil
	}
	procs := s.subscriptions.getMatching(ns, et, dir, peerAddr, peerName)
	if len(procs) == 0 {
		return nil
	}
	cur := s.DeliveryGraph().Receivers(ns, et, dir, peerAddr)
	var prev []string
	if pg := s.prevDeliveryGraph.Load(); pg != nil {
		prev = pg.Receivers(ns, et, dir, peerAddr)
	}
	kept := procs[:0]
	for _, proc := range procs {
		name := proc.Name()
		if !slices.Contains(names, name) {
			continue
		}
		if slices.Contains(cur, name) || slices.Contains(prev, name) {
			kept = append(kept, proc)
		}
	}
	return kept
}

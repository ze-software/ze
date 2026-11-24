// Design: docs/architecture/api/architecture.md -- compiled bus delivery index
// Related: bus_filter.go -- CompiledBus, CompileBus, and CompiledParticipantFilter
// Related: subscribe.go -- SubscriptionManager, the runtime event subscription table

package server

import (
	"maps"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
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
	// SessionFacts optionally carries key/value session properties evaluated
	// against DisseminationProcess.ClaimSessionScope.
	SessionFacts map[string]string
}

// ProcessEdges is the operator view of one endpoint's compiled delivery
// relationship with a participant address entry.
type ProcessEdges struct {
	Process string   `json:"process"`
	Path    string   `json:"path,omitempty"`
	Receive []string `json:"receive,omitempty"`
	Send    []string `json:"send,omitempty"`
	Origin  string   `json:"origin,omitempty"`
}

// PeerEdges is the operator view of all compiled delivery relationships for
// one participant address entry.
type PeerEdges struct {
	Peer       string         `json:"peer"`
	Name       string         `json:"name,omitempty"`
	Path       string         `json:"path,omitempty"`
	ParentPath string         `json:"parent-path,omitempty"`
	Processes  []ProcessEdges `json:"processes"`
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
	order              []string
	namespace          string
	ns                 events.NamespaceID
}

type peerDelivery struct {
	name          string
	path          string
	parentPath    string
	sessionFacts  map[string]string
	procs         []processDelivery
	receive       map[deliveryKey][]string
	sendersByType map[string][]string
	maySend       map[peerSendKey]bool
	recvState     map[string]bool
	allProcs      []string
}

type processDelivery struct {
	name   string
	path   string
	send   []string
	origin BusEdgeOrigin
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
	parentContainers := make(map[string]bool)

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

		input := BusParticipantInput{
			Path:           pPath,
			SecondaryPaths: slices.Clone(p.SecondaryPaths),
			ParentPath:     p.ParentPath,
			Sends:          defaultSends,
			Receives:       defaultReceives,
			OwnBlock:       p.Bus.Clone(),
			OwnOrigin:      p.BusOrigin,
			OwnSource:      p.BusSource,
		}
		applyCentralOverrideToInput(&input, busCfg)
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
	// so parent bus > participant overrides inherit and prefix paths resolve.
	for _, parentPath := range slices.Sorted(maps.Keys(parentContainers)) {
		if seenPaths[parentPath] {
			continue
		}
		seenPaths[parentPath] = true
		cInput := BusParticipantInput{Path: parentPath, PureContainer: true}
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

		pd := &peerDelivery{
			name:          p.Name,
			path:          pPath,
			parentPath:    p.ParentPath,
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
				name:   epName,
				path:   epPath,
				origin: origin,
			}
			procMeta[epName] = pm
		} else if origin == BusOriginWritten || (origin == BusOriginInherited && pm.origin == BusOriginRegistered) {
			pm.origin = origin
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
				ensureProc(epName, r.Path, r.Origin)
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
			Addr:           peers[i].Addr,
			Name:           peers[i].Name,
			Path:           peers[i].Path,
			ParentPath:     peers[i].ParentPath,
			SecondaryPaths: slices.Clone(peers[i].SecondaryPaths),
			Bus:            peers[i].Bus.Clone(),
			BusOrigin:      peers[i].BusOrigin,
			BusSource:      peers[i].BusSource,
			SessionFacts:   maps.Clone(peers[i].SessionFacts),
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

func applyCentralOverrideToInput(input *BusParticipantInput, busCfg plugin.BusConfig) {
	if len(busCfg.Participants) == 0 {
		return
	}
	if ov, ok := busCfg.Participants[input.Path]; ok {
		input.BusSideBlock = ov.Block.Clone()
		input.BusSideSource = ov.Source
	} else {
		for _, sp := range input.SecondaryPaths {
			if ov, ok := busCfg.Participants[sp]; ok {
				input.BusSideBlock = ov.Block.Clone()
				input.BusSideSource = ov.Source
				break
			}
		}
	}
	if input.ParentPath != "" {
		if ov, ok := busCfg.Participants[input.ParentPath]; ok {
			input.ParentBusSideBlock = ov.Block.Clone()
			input.ParentBusSideSource = ov.Source
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
		}
		return t
	}

	out := &rpc.BusParticipantFilter{
		ParticipantPath: f.Path,
		EndpointName:    f.EndpointName,
		SendAll:         f.SendAllowAllDeclared,
		SendNone:        f.SendAllowNone,
		ReceiveAll:      f.ReceiveAllowAllDeclared,
		ReceiveNone:     f.ReceiveAllowNone,
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
			Message: r.Message,
			Allowed: r.Allowed,
			Targets: targets,
			Tags:    cloneTagValueMap(r.OutboundTagMap),
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
			Message: r.Message,
			Allowed: r.Allowed,
			Targets: targets,
			Tags:    cloneTagValueMap(r.InboundTagMap),
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
			Processes:  make([]ProcessEdges, 0, len(pd.procs)),
		}
		for _, proc := range pd.procs {
			entry.Processes = append(entry.Processes, ProcessEdges{
				Process: proc.name,
				Path:    proc.path,
				Receive: g.receiveTokens(addr, proc.name),
				Send:    slices.Clone(proc.send),
				Origin:  proc.origin.String(),
			})
		}
		out = append(out, entry)
	}
	return out
}

// receiveTokens reads the event types indexed for one endpoint on one address
// back out of the index, formatted as operator tokens.
func (g *DeliveryGraph) receiveTokens(peerAddr, processName string) []string {
	pd := g.peers[peerAddr]
	var out []string
	for k := range pd.receive {
		if k.dir != events.DirUnspecified {
			continue
		}
		if !slices.Contains(g.Receivers(k.ns, k.et, k.dir, peerAddr), processName) {
			continue
		}
		dir := events.DirUnspecified
		if slices.Contains(g.Receivers(k.ns, k.et, events.DirReceived, peerAddr), processName) {
			dir = events.DirReceived
		}
		if slices.Contains(g.Receivers(k.ns, k.et, events.DirSent, peerAddr), processName) {
			if dir == events.DirReceived {
				dir = events.DirBoth
			} else {
				dir = events.DirSent
			}
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
	return nil
}

// collectBusEndpointInputs collects BusParticipantInput entries for all active
// processes, configured endpoints, and bus > endpoint entries known to s.
func (s *Server) collectBusEndpointInputs(namespace string, busCfg plugin.BusConfig) []BusParticipantInput {
	seen := make(map[string]bool)
	var out []BusParticipantInput

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
		if regName == "" && (liveReg == nil || (!liveReg.Done && len(liveReg.Sends) == 0 && len(liveReg.Receives) == 0)) {
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

		epPath := plugin.EndpointPath(epName)
		canonicalPath := epPath
		var secondary []string
		if singletonPath != "" && singletonPath != epPath {
			canonicalPath = singletonPath
			secondary = []string{epPath}
		}

		out = append(out, BusParticipantInput{
			Path:           canonicalPath,
			EndpointName:   epName,
			SecondaryPaths: secondary,
			Sends:          sends,
			Receives:       receives,
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
		addEndpoint(epName, "", nil)
	}

	return out
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

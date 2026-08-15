// Design: docs/architecture/api/architecture.md -- peer-to-process delivery edges
// Related: subscribe.go -- SubscriptionManager, the runtime producer of the same edges

package server

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/events"
)

// DeliveryPeer is one peer's attached processes, as the peer's RESOLVED
// settings state them. The reactor produces it after every config apply.
//
// Resolved, never a config tree: a binding a peer inherits from its group is
// already merged in, and a member that restates the same process has already
// replaced the group's list (`receive` is a leaf-list outside cumulativePaths,
// bgp/config/resolve.go).
type DeliveryPeer struct {
	// Addr is the peer address, spelled exactly as every delivery site spells
	// it (plugin.PeerInfo.AddressStr, which is PeerSettings.Address.String()).
	// It is the graph's key.
	Addr string
	// Name is the configured peer name, empty when the config names none.
	Name string
	// Bindings is one entry per `attach process <name>` block on the peer.
	Bindings []plugin.PeerProcessBinding
}

// ProcessEdges is what one peer grants one process, after every token has been
// resolved against the event registry. It is the operator's view of one edge.
type ProcessEdges struct {
	Process string `json:"process"`
	// Receive names the granted event types, spelled as an operator writes
	// them (`update-received`, `state`).
	Receive []string `json:"receive,omitempty"`
	// Send names the message types this process may originate toward the peer.
	Send []string `json:"send,omitempty"`
	// Unresolved names granted tokens the event registry does not know, so the
	// index carries no edge for them. A guard that drops an edge in silence is
	// a guard an operator cannot debug (ai/rules/evidence.md).
	Unresolved []string `json:"unresolved,omitempty"`
}

// PeerEdges is one peer's whole delivery relationship, for inspection.
type PeerEdges struct {
	Peer      string         `json:"peer"`
	Name      string         `json:"name,omitempty"`
	Processes []ProcessEdges `json:"processes"`
}

// deliveryKey is one question the delivery path asks: which processes does this
// peer feed for this event, in this direction. Every field is comparable, so a
// lookup is one map read and allocates nothing.
type deliveryKey struct {
	ns  events.NamespaceID
	et  events.EventTypeID
	dir events.Direction
}

// DeliveryGraph is the peer-to-process edge index, built once per config apply
// and never mutated afterwards. Readers take it whole (Server.DeliveryGraph),
// so a rebuild cannot show them a half-built index.
//
// It is an INDEX over the edges a subscription registers, not a second registry
// beside them: the config is one producer, the runtime `subscribe` command is
// the other, and the index is the one lookup the delivery path reads.
type DeliveryGraph struct {
	peers map[string]*peerDelivery
	order []string // peer addresses, sorted, for inspection
	// namespace is the event namespace every token in this index was resolved
	// against, kept so a reader of the index (the reconciliation report) needs
	// no second copy of it.
	namespace string
	ns        events.NamespaceID
}

type peerDelivery struct {
	name    string
	procs   []processDelivery // attached processes, sorted by name
	receive map[deliveryKey][]string
}

// processDelivery is the half of one edge the receive index cannot answer: the
// send permission, which names no event type, and the tokens that resolved to
// no type at all. The receive half is read back OUT of the index (receiveTokens)
// rather than stored beside it, so an operator is shown what delivery reads.
type processDelivery struct {
	name       string
	send       []string
	unresolved []string
}

// emptyDeliveryGraph is what a server answers with before its first config
// apply. It holds no peer, so it feeds no process: a graph that cannot find a
// peer must never answer "everything" (ai/rules/evidence.md).
var emptyDeliveryGraph = newDeliveryGraph("", nil)

// newDeliveryGraph builds the index from resolved peer settings.
//
// namespace is the event namespace the config's tokens belong to; a peer's
// receive leaf-list names bgp event types today (config/validators.go).
func newDeliveryGraph(namespace string, peers []DeliveryPeer) *DeliveryGraph {
	nsID := events.LookupNamespaceID(namespace)
	g := &DeliveryGraph{
		peers:     make(map[string]*peerDelivery, len(peers)),
		order:     make([]string, 0, len(peers)),
		namespace: namespace,
		ns:        nsID,
	}
	// The wildcard names every type the registry holds NOW, which is the same
	// expansion the ready RPC makes for a plugin that declares "*"
	// (dispatch.go, registerSubscriptions).
	wildcard := events.AllEventTypes()[namespace]
	slices.Sort(wildcard)

	for _, p := range peers {
		if p.Addr == "" {
			continue // an edge with no peer to key on is not an edge
		}
		pd := &peerDelivery{
			name:    p.Name,
			receive: make(map[deliveryKey][]string),
		}
		// The parser builds bindings by ranging a map (reactor/config.go), so
		// their order is random. Sorting here makes the index, the delivery
		// order and every surface that prints them deterministic.
		bindings := slices.Clone(p.Bindings)
		slices.SortFunc(bindings, func(a, b plugin.PeerProcessBinding) int {
			return strings.Compare(a.PluginName, b.PluginName)
		})
		for _, b := range bindings {
			pd.procs = append(pd.procs, pd.addBinding(namespace, nsID, wildcard, b))
		}
		g.peers[p.Addr] = pd
		g.order = append(g.order, p.Addr)
	}
	slices.Sort(g.order)
	return g
}

// addBinding indexes one `attach process` block and returns the half of it the
// receive index does not carry.
func (pd *peerDelivery) addBinding(namespace string, nsID events.NamespaceID, wildcard []string, b plugin.PeerProcessBinding) processDelivery {
	edges := processDelivery{name: b.PluginName}

	for _, et := range grantedTypes(wildcard, b) {
		dir := grantDirection(wildcard, b, et)
		if !events.IsValidEvent(namespace, et) {
			edges.unresolved = append(edges.unresolved, events.DirectionToken(et, dir))
			continue
		}
		etID := events.LookupEventTypeID(et)
		// An event that carries no direction of its own matches any grant of
		// its type (Subscription.Matches), so every grant is indexed under it.
		pd.grant(deliveryKey{ns: nsID, et: etID, dir: events.DirUnspecified}, b.PluginName)
		if dir == events.DirReceived || dir == events.DirBoth {
			pd.grant(deliveryKey{ns: nsID, et: etID, dir: events.DirReceived}, b.PluginName)
		}
		if dir == events.DirSent || dir == events.DirBoth {
			pd.grant(deliveryKey{ns: nsID, et: etID, dir: events.DirSent}, b.PluginName)
		}
	}

	if b.SendAll {
		edges.send = append(edges.send, events.TokenWildcard)
	}
	sendTypes := make([]string, 0, len(b.Send))
	for st := range b.Send {
		sendTypes = append(sendTypes, st)
	}
	slices.Sort(sendTypes)
	edges.send = append(edges.send, sendTypes...)

	return edges
}

// grantedTypes returns every event type one binding grants, sorted. The
// wildcard adds the whole registry; a named type adds itself.
func grantedTypes(wildcard []string, b plugin.PeerProcessBinding) []string {
	out := make([]string, 0, len(wildcard)+len(b.Receive))
	if b.ReceiveAll {
		out = append(out, wildcard...)
	}
	for et := range b.Receive {
		if !b.ReceiveAll || !slices.Contains(wildcard, et) {
			out = append(out, et)
		}
	}
	slices.Sort(out)
	return out
}

// grantDirection returns the direction one binding grants one event type. A
// grant only ever adds, so a type named beside "*" keeps both directions:
// naming it takes nothing away from the wildcard.
func grantDirection(wildcard []string, b plugin.PeerProcessBinding, eventType string) events.Direction {
	if b.ReceiveAll && slices.Contains(wildcard, eventType) {
		return events.DirBoth
	}
	return b.Receive[eventType]
}

func (pd *peerDelivery) grant(k deliveryKey, process string) {
	pd.receive[k] = append(pd.receive[k], process)
}

// Receivers returns the processes the peer feeds for one event. The delivery
// path calls it once per event, so it allocates nothing: the returned slice is
// STORED, and a caller MUST treat it as read-only -- never append to it, never
// keep it past the delivery it was asked for.
//
// A peer the config never named has no edges and feeds nobody. That is the
// whole guard: an unknown peer returns nothing, never everything.
func (g *DeliveryGraph) Receivers(ns events.NamespaceID, et events.EventTypeID, dir events.Direction, peerAddr string) []string {
	pd := g.peers[peerAddr]
	if pd == nil {
		return nil
	}
	return pd.receive[deliveryKey{ns: ns, et: et, dir: dir}]
}

// Inspect returns the graph as plain data, peers in address order. It
// ALLOCATES and is the operator surface (`show event delivery`); the delivery
// path uses Receivers.
//
// The receive tokens are read back OUT of the index through Receivers rather
// than stored beside it. An operator debugging "why is my program not fed" is
// then shown the edges delivery reads, not a second structure that could
// disagree with them.
func (g *DeliveryGraph) Inspect() []PeerEdges {
	out := make([]PeerEdges, 0, len(g.order))
	for _, addr := range g.order {
		pd := g.peers[addr]
		peer := PeerEdges{Peer: addr, Name: pd.name, Processes: make([]ProcessEdges, 0, len(pd.procs))}
		for _, proc := range pd.procs {
			peer.Processes = append(peer.Processes, ProcessEdges{
				Process:    proc.name,
				Receive:    g.receiveTokens(addr, proc.name),
				Send:       proc.send,
				Unresolved: proc.unresolved,
			})
		}
		out = append(out, peer)
	}
	return out
}

// receiveTokens reads the event types the index grants one process on one peer
// back out of the index, spelled as the tokens an operator writes.
func (g *DeliveryGraph) receiveTokens(peerAddr, process string) []string {
	pd := g.peers[peerAddr]
	var out []string
	for k := range pd.receive {
		// Every grant is indexed under the directionless key, so walking those
		// keys walks each granted type exactly once.
		if k.dir != events.DirUnspecified {
			continue
		}
		if !slices.Contains(g.Receivers(k.ns, k.et, k.dir, peerAddr), process) {
			continue
		}
		dir := events.DirUnspecified
		if slices.Contains(g.Receivers(k.ns, k.et, events.DirReceived, peerAddr), process) {
			dir = events.DirReceived
		}
		if slices.Contains(g.Receivers(k.ns, k.et, events.DirSent, peerAddr), process) {
			if dir == events.DirReceived {
				dir = events.DirBoth
			} else {
				dir = events.DirSent
			}
		}
		out = append(out, events.DirectionToken(k.et.String(), dir))
	}
	slices.Sort(out) // map iteration is random; an operator surface is not
	return out
}

// Len returns the number of peers the index holds.
func (g *DeliveryGraph) Len() int { return len(g.order) }

// DeliveryGraph returns the current peer-to-process index. Never nil: before
// the first config apply it is the empty index, which feeds nobody.
func (s *Server) DeliveryGraph() *DeliveryGraph {
	if g := s.deliveryGraph.Load(); g != nil {
		return g
	}
	return emptyDeliveryGraph
}

// UpdateDeliveryGraph rebuilds the index from resolved peer settings and swaps
// it in under one pointer write.
//
// Build then swap is what makes a reload safe (R-7): a reader takes the whole
// graph in one atomic load, so it sees the old index or the new one and never a
// half-built one, and an edge that survives the reload misses no event.
//
// It does NOT discard an operator's runtime subscriptions. A publish is not a
// config apply: a peer joining or leaving republishes too, and a route server
// accepting an inbound dynamic peer would otherwise delete the subscription an
// operator typed to watch a live session, with no message. The discard belongs
// to the apply and is DiscardRuntimeSubscriptions, which the reactor calls at
// the end of every reconcile (bgp/reactor/reactor_api.go).
func (s *Server) UpdateDeliveryGraph(namespace string, peers []DeliveryPeer) {
	g := newDeliveryGraph(namespace, peers)
	s.deliveryGraph.Store(g)
	logger().Debug("delivery graph published", "namespace", namespace, "peers", g.Len())
	s.reconcileDelivery(nil)
}

// DiscardRuntimeSubscriptions drops every live `subscribe` override.
//
// A config APPLY calls this, and nothing else does. The config is durable truth
// and a reload's job is to make the daemon match the document, so an override
// that survived one would make the document a lie about what the daemon
// delivers. The rule is stated to the operator in the same words
// (docs/architecture/api/commands.md) and is the spec's R-10.
//
// A dynamic peer connecting, a peer being removed, or the index being rebuilt
// for any other reason is NOT an apply and leaves the override standing.
func (s *Server) DiscardRuntimeSubscriptions() {
	if s.subscriptions == nil {
		return
	}
	s.subscriptions.clearRuntimeOverrides()
}

// PeerScopedProcs returns the processes that take delivery of one peer-scoped
// event. Every peer-scoped delivery site asks this one question, so the filter
// lives here rather than at each site.
//
// A process takes delivery when BOTH halves agree: it subscribed to the event,
// which is what the program can handle, and the peer's config attaches it and
// grants the type, which is what the operator says it gets. The effective set
// is the overlap. Delivering an ungranted type is the defect this index exists
// to remove; delivering an undeclared type spends IPC on an event the program
// has no handler for.
//
// A peer the config never named grants nothing, so it feeds nobody. That guard
// fails closed on purpose: an index that cannot find a peer must never answer
// "everything" (ai/rules/evidence.md).
//
// The one addition to the overlap is a RUNTIME override, which an operator
// makes against a running daemon and which the next config apply discards.
//
// It allocates nothing: the graph hands back a stored slice, and the survivors
// are compacted into the slice GetMatching already built for this call.
func (s *Server) PeerScopedProcs(ns events.NamespaceID, et events.EventTypeID, dir events.Direction, peerAddr, peerName string) []*process.Process {
	if s.subscriptions == nil {
		return nil
	}
	procs := s.subscriptions.GetMatching(ns, et, dir, peerAddr, peerName)
	if len(procs) == 0 {
		return procs
	}
	granted := s.DeliveryGraph().Receivers(ns, et, dir, peerAddr)
	overrides := s.subscriptions.hasRuntimeOverride()
	if len(granted) == 0 && !overrides {
		return nil
	}
	kept := procs[:0]
	for _, proc := range procs {
		takes := slices.Contains(granted, proc.Name())
		if !takes && overrides {
			takes = s.subscriptions.matchesRuntimeOverride(proc, ns, et, dir, peerAddr, peerName)
		}
		if takes {
			kept = append(kept, proc)
		}
	}
	return kept
}

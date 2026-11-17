// Design: docs/architecture/api/architecture.md -- peer dissemination ownership.
// Related: delivery_graph.go -- the immutable actual-grant index.
// Related: startup_claims.go -- aliases and authenticated Stage 1 declarations.
package server

import (
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// DisseminationProcess is one configured process's declaration snapshot. It is
// raw configuration input, not permission or process-lifetime authority. The
// caller MUST keep it unchanged while resolving the graph. Separate configured
// names remain separate instances even when they use one implementation.
type DisseminationProcess struct {
	Name         string
	Claims       []string
	Declarations []rpc.DisseminationDecl
}

// DisseminationOwner identifies one configuration's effective peer-output
// owner. Its zero value means no eligible configured owner; IsSet names that
// intentional absence. It does not authenticate a running process lifetime.
// Immutable and safe for concurrent use.
type DisseminationOwner struct {
	name     string
	explicit bool
}

// IsSet reports whether the configuration supplies one eligible owner.
func (owner DisseminationOwner) IsSet() bool { return owner.name != "" }

// Name answers the configured instance, never its implementation alias.
func (owner DisseminationOwner) Name() string { return owner.name }

// Explicit reports whether the owner declares a Claim of this role.
func (owner DisseminationOwner) Explicit() bool { return owner.explicit }

// ResolveDisseminationOwner resolves a destination from one immutable graph.
// Complete explicit blocks are required on the same process's source and
// destination; derived bindings and implicit receive additions grant no role.
// One eligible claimant supersedes ordinary owners. Ambiguity in the winning
// class is refused, never ordered by declarations. Safe for concurrent use.
func (g *DeliveryGraph) ResolveDisseminationOwner(peer, role string, processes []DisseminationProcess) (DisseminationOwner, error) {
	var ordinary, explicit []string
	for i := range processes {
		process := &processes[i]
		if process.Name == "" {
			return DisseminationOwner{}, fmt.Errorf("dissemination role %q has an unnamed configured process", role)
		}
		if err := registry.CheckDisseminationDeclarations(process.Name, process.Declarations); err != nil {
			return DisseminationOwner{}, err
		}
		if err := registry.CheckDisseminationEvents(process.Name, process.Declarations); err != nil {
			return DisseminationOwner{}, err
		}
		for j := range process.Declarations {
			decl := &process.Declarations[j]
			if decl.Namespace != g.namespace {
				continue
			}
			if decl.Role != role {
				continue
			}
			if !g.disseminationDestination(peer, process.Name, decl) {
				continue
			}
			if !g.disseminationSource(process.Name, decl) {
				continue
			}
			if slices.Contains(process.Claims, role) {
				explicit = appendDisseminationOwner(explicit, process.Name)
				continue
			}
			ordinary = appendDisseminationOwner(ordinary, process.Name)
		}
	}
	if len(explicit) > 0 {
		return g.disseminationWinner(peer, role, explicit, true)
	}
	return g.disseminationWinner(peer, role, ordinary, false)
}

func appendDisseminationOwner(owners []string, name string) []string {
	if slices.Contains(owners, name) {
		return owners
	}
	return append(owners, name)
}

func (g *DeliveryGraph) disseminationWinner(peer, role string, names []string, explicit bool) (DisseminationOwner, error) {
	if len(names) == 0 {
		return DisseminationOwner{}, nil
	}
	if len(names) == 1 {
		return DisseminationOwner{name: names[0], explicit: explicit}, nil
	}
	slices.Sort(names)
	name := peer
	if destination := g.peers[peer]; destination != nil {
		if destination.name != "" {
			name = destination.name
		}
	}
	return DisseminationOwner{}, fmt.Errorf("peer %q has conflicting processes %q for dissemination role %q", name, names, role)
}

func (g *DeliveryGraph) disseminationSource(process string, decl *rpc.DisseminationDecl) bool {
	for _, peer := range g.order {
		if g.disseminationReceive(peer, process, decl.SourceReceive) {
			return true
		}
	}
	return false
}

func (g *DeliveryGraph) disseminationDestination(peer, process string, decl *rpc.DisseminationDecl) bool {
	if !g.disseminationReceive(peer, process, decl.DestinationReceive) {
		return false
	}
	for _, required := range decl.DestinationSend {
		if !g.MaySend(peer, process, required) {
			return false
		}
	}
	return true
}

func (g *DeliveryGraph) disseminationReceive(peer, process string, required []string) bool {
	attachment := g.disseminationAttachment(peer, process)
	if attachment == nil {
		return false
	}
	if !attachment.explicit {
		return false
	}
	for _, token := range required {
		name, direction, known := events.SplitTypeToken(g.namespace, token)
		if !known {
			return false
		}
		if slices.Contains(attachment.implicit, name) {
			return false
		}
		if !g.disseminationReceives(peer, process, events.LookupEventTypeID(name), direction) {
			return false
		}
	}
	return len(required) > 0
}

func (g *DeliveryGraph) disseminationAttachment(peer, process string) *processDelivery {
	destination := g.peers[peer]
	if destination == nil {
		return nil
	}
	for i := range destination.procs {
		if destination.procs[i].name == process {
			return &destination.procs[i]
		}
	}
	return nil
}

func (g *DeliveryGraph) disseminationReceives(peer, process string, event events.EventTypeID, direction events.Direction) bool {
	// This direction is parsed from an external declaration; unknown values
	// grant nothing. Every named event direction is handled explicitly.
	switch direction {
	case events.DirReceived, events.DirSent:
		return slices.Contains(g.Receivers(g.ns, event, direction, peer), process)
	case events.DirBoth:
		if !slices.Contains(g.Receivers(g.ns, event, events.DirReceived, peer), process) {
			return false
		}
		return slices.Contains(g.Receivers(g.ns, event, events.DirSent, peer), process)
	case events.DirUnspecified:
		return false
	default:
		return false
	}
}

// ResolveDisseminationOwner reads prospective declarations under configured
// names, using a runtime declaration when one exists. Callers that authorize a
// write MUST separately authenticate and recheck that process's live lifetime.
// It is a configuration/lifecycle lookup, not a per-route hot-path lookup.
func (s *Server) ResolveDisseminationOwner(peer, role string) (DisseminationOwner, error) {
	return s.DeliveryGraph().ResolveDisseminationOwner(peer, role, s.disseminationProcesses())
}

func (s *Server) disseminationProcesses() []DisseminationProcess {
	var processes []DisseminationProcess
	for _, prospective := range s.prospectivePlugins() {
		declaration := s.disseminationForPlugin(prospective)
		if len(declaration.Declarations) == 0 {
			continue
		}
		processes = append(processes, declaration)
	}
	return processes
}

// ResolveDisseminationOwnerForPeer resolves one exact destination's grants,
// preserving all original source nodes even when peers share an IP address.
// This is a cold lifecycle lookup: it acquires process-manager locks and MUST
// NOT run under a socket writer lock, reactor lock or peer lock.
func (s *Server) ResolveDisseminationOwnerForPeer(peer DeliveryPeer, role string) (DisseminationOwner, error) {
	graph := s.DeliveryGraph()
	peers := make([]DeliveryPeer, 0, len(graph.disseminationPeers)+1)
	for i, source := range graph.disseminationPeers {
		source.Addr = fmt.Sprintf("source-%d", i)
		peers = append(peers, source)
	}
	peer.Addr = "destination"
	peers = append(peers, peer)
	return newDeliveryGraph(graph.namespace, peers).ResolveDisseminationOwner("destination", role, s.disseminationProcesses())
}

// ValidateDisseminationBindings applies the same complete-grant gate to
// authenticated runtime declarations, including third-party processes.
func (s *Server) ValidateDisseminationBindings() error {
	graph := s.DeliveryGraph()
	peers := slices.Clone(graph.disseminationPeers)
	for i := range peers {
		peers[i].Addr = fmt.Sprintf("peer-%d", i)
	}
	return newDeliveryGraph(graph.namespace, peers).ValidateDisseminationBindings(s.disseminationProcesses())
}

// ResolveDisseminationOwners verifies a complete candidate before publication.
// Peer keys MUST distinguish exact configured peers. Runtime declarations,
// including an empty claim or declaration list, supersede compiled metadata.
// Callers MUST hold no reactor or peer lock while this reads declarations.
func (s *Server) ResolveDisseminationOwners(namespace, role string, peers []DeliveryPeer) (map[string]DisseminationOwner, error) {
	graph := newDeliveryGraph(namespace, peers)
	processes := s.disseminationProcesses()
	if err := graph.ValidateDisseminationBindings(processes); err != nil {
		return nil, err
	}
	owners := make(map[string]DisseminationOwner, len(peers))
	for _, peer := range peers {
		owner, err := graph.ResolveDisseminationOwner(peer.Addr, role, processes)
		if err != nil {
			return nil, err
		}
		owners[peer.Addr] = owner
	}
	return owners, nil
}

func (s *Server) disseminationForPlugin(prospective prospectivePlugin) DisseminationProcess {
	if manager := s.procManager.Load(); manager != nil {
		if process := manager.GetProcess(prospective.process); process != nil {
			if declaration := process.Registration(); declaration != nil {
				return DisseminationProcess{Name: prospective.process, Claims: declaration.Claims, Declarations: declaration.Dissemination}
			}
		}
	}
	for _, name := range prospective.registry {
		if declaration := registry.Lookup(name); declaration != nil {
			if len(declaration.Dissemination) > 0 {
				return DisseminationProcess{Name: prospective.process, Claims: declaration.Claims, Declarations: declaration.Dissemination}
			}
		}
	}
	return DisseminationProcess{Name: prospective.process}
}

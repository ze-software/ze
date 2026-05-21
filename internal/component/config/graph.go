// Design: docs/architecture/config/syntax.md — config dependency graph for agent impact analysis
// Related: plugin_verify.go — plugin config root verification uses same registry
// Related: listener.go — listener endpoint collection for port conflict detection

package config

import (
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// GraphNodeKind identifies what a graph node represents.
type GraphNodeKind string

const (
	NodeSection  GraphNodeKind = "section"
	NodePeer     GraphNodeKind = "peer"
	NodeGroup    GraphNodeKind = "group"
	NodePlugin   GraphNodeKind = "plugin"
	NodeProfile  GraphNodeKind = "profile"
	NodeUser     GraphNodeKind = "user"
	NodeListener GraphNodeKind = "listener"
)

// GraphEdgeKind identifies the dependency relationship.
type GraphEdgeKind string

const (
	EdgeContains     GraphEdgeKind = "contains"
	EdgeInherits     GraphEdgeKind = "inherits"
	EdgeReferences   GraphEdgeKind = "references"
	EdgeConfigRoot   GraphEdgeKind = "config-root"
	EdgeListensOn    GraphEdgeKind = "listens-on"
	EdgeDependsOn    GraphEdgeKind = "depends-on"
	EdgeProcessBinds GraphEdgeKind = "process-binds"
)

// GraphNode is a node in the config dependency graph.
type GraphNode struct {
	ID   string        `json:"id"`
	Kind GraphNodeKind `json:"kind"`
	Name string        `json:"name"`
}

// GraphEdge is a directed edge in the config dependency graph.
type GraphEdge struct {
	From string        `json:"from"`
	To   string        `json:"to"`
	Kind GraphEdgeKind `json:"kind"`
}

// Graph is the full config dependency graph.
type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

func (g *Graph) addNode(id string, kind GraphNodeKind, name string) {
	g.Nodes = append(g.Nodes, GraphNode{ID: id, Kind: kind, Name: name})
}

func (g *Graph) addEdge(from, to string, kind GraphEdgeKind) {
	g.Edges = append(g.Edges, GraphEdge{From: from, To: to, Kind: kind})
}

// BuildGraph builds a dependency graph from a parsed config tree and schema.
// It derives all relationships from the same data sources that validation uses.
func BuildGraph(tree *Tree, schema *Schema) *Graph {
	g := &Graph{}

	addSectionNodes(g, tree)
	addBGPEdges(g, tree)
	addAuthzEdges(g, tree)
	addListenerNodes(g, tree, schema)
	addPluginRegistryEdges(g)

	sortGraph(g)
	return g
}

// addSectionNodes adds a node for each top-level config section.
func addSectionNodes(g *Graph, tree *Tree) {
	for _, name := range tree.ContainerNames() {
		g.addNode("section/"+name, NodeSection, name)
	}
}

// addBGPEdges adds peer, group, and plugin-binding edges from the bgp section.
func addBGPEdges(g *Graph, tree *Tree) {
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return
	}

	pluginContainer := tree.GetContainer("plugin")

	for _, groupEntry := range bgp.GetListOrdered("group") {
		groupName := groupEntry.Key
		groupID := "group/" + groupName
		g.addNode(groupID, NodeGroup, groupName)
		g.addEdge("section/bgp", groupID, EdgeContains)

		for _, peerEntry := range groupEntry.Value.GetListOrdered("peer") {
			peerName := peerEntry.Key
			peerID := "peer/" + peerName
			g.addNode(peerID, NodePeer, peerName)
			g.addEdge(peerID, groupID, EdgeInherits)

			addProcessBindings(g, peerEntry.Value, peerID, pluginContainer)
		}
	}

	for _, peerEntry := range bgp.GetListOrdered("peer") {
		peerName := peerEntry.Key
		peerID := "peer/" + peerName
		g.addNode(peerID, NodePeer, peerName)
		g.addEdge("section/bgp", peerID, EdgeContains)

		addProcessBindings(g, peerEntry.Value, peerID, pluginContainer)
	}
}

// addProcessBindings adds edges from a peer to the plugins it references via process bindings.
func addProcessBindings(g *Graph, peerTree *Tree, peerID string, pluginContainer *Tree) {
	processList := peerTree.GetList("process")
	for name, processTree := range processList {
		if name == KeyDefault {
			continue
		}

		run, hasRun := processTree.Get("run")
		use, hasUse := processTree.Get("use")

		if hasRun && run != "" {
			pluginID := "plugin/" + name
			g.addNode(pluginID, NodePlugin, name)
			g.addEdge(peerID, pluginID, EdgeProcessBinds)
			continue
		}
		if hasUse && use != "" {
			pluginID := "plugin/" + name
			g.addNode(pluginID, NodePlugin, name)
			g.addEdge(peerID, pluginID, EdgeProcessBinds)
			continue
		}

		if pluginContainer != nil {
			if externals := pluginContainer.GetList("external"); externals != nil {
				if _, ok := externals[name]; ok {
					pluginID := "plugin/" + name
					g.addNode(pluginID, NodePlugin, name)
					g.addEdge(peerID, pluginID, EdgeProcessBinds)
				}
			}
		}
	}
}

// addAuthzEdges adds user->profile reference edges from system.authentication/authorization.
func addAuthzEdges(g *Graph, tree *Tree) {
	sys := tree.GetContainer("system")
	if sys == nil {
		return
	}

	authzContainer := sys.GetContainer("authorization")
	if authzContainer != nil {
		for name := range authzContainer.GetList("profile") {
			profileID := "profile/" + name
			g.addNode(profileID, NodeProfile, name)
			g.addEdge("section/system", profileID, EdgeContains)
		}
	}

	authContainer := sys.GetContainer("authentication")
	if authContainer == nil {
		return
	}

	for username, userTree := range authContainer.GetList("user") {
		userID := "user/" + username
		g.addNode(userID, NodeUser, username)
		g.addEdge("section/system", userID, EdgeContains)

		for _, profileName := range userTree.GetSlice("profile") {
			g.addEdge(userID, "profile/"+profileName, EdgeReferences)
		}
	}
}

// addListenerNodes adds listener endpoint nodes and service ownership edges.
func addListenerNodes(g *Graph, tree *Tree, schema *Schema) {
	endpoints := CollectListeners(tree, schema)
	for i := range endpoints {
		ep := &endpoints[i]
		listenerID := "listener/" + ep.Service
		g.addNode(listenerID, NodeListener, ep.Service)

		sectionName := listenerSectionName(ep.Service)
		if sectionName != "" {
			g.addEdge("section/"+sectionName, listenerID, EdgeListensOn)
		}
	}
}

// listenerSectionName maps a listener service name back to the top-level config
// section that owns it. Listener service names use the conventions from
// DiscoverListenerServices: "web", "ssh", "plugin-hub", "looking-glass", etc.
func listenerSectionName(service string) string {
	switch {
	case service == "web" || strings.HasPrefix(service, "web "):
		return "web"
	case service == "ssh" || strings.HasPrefix(service, "ssh "):
		return "ssh"
	case service == "looking-glass" || strings.HasPrefix(service, "looking-glass "):
		return "looking-glass"
	case service == "mcp" || strings.HasPrefix(service, "mcp "):
		return sectionEnvironment
	case strings.HasPrefix(service, "plugin-hub"):
		return sectionPlugin
	case strings.HasPrefix(service, "dns"):
		return "dns"
	case strings.HasPrefix(service, "telemetry"):
		return "telemetry"
	case strings.HasPrefix(service, "api-server"):
		return sectionEnvironment
	case strings.HasPrefix(service, "wireguard"):
		return sectionInterface
	case strings.HasPrefix(service, "l2tp"):
		return sectionEnvironment
	case strings.HasPrefix(service, "bgp"):
		return "bgp"
	}
	return ""
}

const (
	sectionEnvironment = "environment"
	sectionInterface   = "interface"
	sectionPlugin      = "plugin"
)

// addPluginRegistryEdges adds edges from registered plugins to their declared config roots.
func addPluginRegistryEdges(g *Graph) {
	for pluginName, roots := range registry.ConfigRootsMap() {
		pluginID := "plugin/" + pluginName
		g.addNode(pluginID, NodePlugin, pluginName)
		for _, root := range roots {
			g.addEdge(pluginID, "section/"+root, EdgeConfigRoot)
		}
	}

	for _, reg := range registry.All() {
		if len(reg.Dependencies) == 0 {
			continue
		}
		for _, dep := range reg.Dependencies {
			g.addEdge("plugin/"+reg.Name, "plugin/"+dep, EdgeDependsOn)
		}
	}
}

// sortGraph deduplicates nodes and edges, creates implicit nodes for dangling
// edge endpoints, and sorts both for deterministic output.
func sortGraph(g *Graph) {
	seen := make(map[string]bool, len(g.Nodes))
	unique := make([]GraphNode, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		if !seen[n.ID] {
			seen[n.ID] = true
			unique = append(unique, n)
		}
	}

	// Create implicit nodes for edge endpoints that have no explicit node.
	for _, e := range g.Edges {
		for _, id := range []string{e.From, e.To} {
			if !seen[id] {
				seen[id] = true
				unique = append(unique, GraphNode{
					ID:   id,
					Kind: kindFromID(id),
					Name: nameFromID(id),
				})
			}
		}
	}
	g.Nodes = unique

	// Deduplicate edges.
	type edgeKey struct {
		from, to string
		kind     GraphEdgeKind
	}
	seenEdges := make(map[edgeKey]bool, len(g.Edges))
	dedupEdges := make([]GraphEdge, 0, len(g.Edges))
	for _, e := range g.Edges {
		k := edgeKey{e.From, e.To, e.Kind}
		if !seenEdges[k] {
			seenEdges[k] = true
			dedupEdges = append(dedupEdges, e)
		}
	}
	g.Edges = dedupEdges

	sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].ID < g.Nodes[j].ID })
	sort.Slice(g.Edges, func(i, j int) bool {
		if g.Edges[i].From != g.Edges[j].From {
			return g.Edges[i].From < g.Edges[j].From
		}
		if g.Edges[i].To != g.Edges[j].To {
			return g.Edges[i].To < g.Edges[j].To
		}
		return g.Edges[i].Kind < g.Edges[j].Kind
	})
}

// kindFromID infers a GraphNodeKind from a node ID prefix.
func kindFromID(id string) GraphNodeKind {
	prefix, _, ok := strings.Cut(id, "/")
	if !ok {
		return NodeSection
	}
	switch GraphNodeKind(prefix) {
	case NodeSection:
		return NodeSection
	case NodePeer:
		return NodePeer
	case NodeGroup:
		return NodeGroup
	case NodePlugin:
		return NodePlugin
	case NodeProfile:
		return NodeProfile
	case NodeUser:
		return NodeUser
	case NodeListener:
		return NodeListener
	default:
		return NodeSection
	}
}

// nameFromID extracts the name portion after the first slash.
func nameFromID(id string) string {
	_, name, ok := strings.Cut(id, "/")
	if !ok {
		return id
	}
	return name
}

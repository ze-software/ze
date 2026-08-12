// Design: docs/architecture/config/syntax.md — config dependency graph for agent impact analysis
// Related: plugin_verify.go — plugin config root verification uses same registry
// Related: listener.go — listener endpoint collection for port conflict detection

package config

import (
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
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
	NodeAddress  GraphNodeKind = "address"
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
	EdgeUsesAddress  GraphEdgeKind = "uses-address"
)

const listenerServiceSSH = "ssh"

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
	addAddressNodes(g, tree)
	addAddressUsageEdges(g, tree)

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
	bgp := tree.GetContainer(sectionBGP)
	if bgp == nil {
		return
	}

	pluginContainer := tree.GetContainer("plugin")

	var tb textbuf.Buffer
	for _, groupEntry := range bgp.GetListOrdered("group") {
		groupName := groupEntry.Key
		groupID := tb.Reset().Str("group/").Str(groupName).String()
		g.addNode(groupID, NodeGroup, groupName)
		g.addEdge("section/bgp", groupID, EdgeContains)

		for _, peerEntry := range groupEntry.Value.GetListOrdered("peer") {
			peerName := peerEntry.Key
			peerID := tb.Reset().Str("peer/").Str(peerName).String()
			g.addNode(peerID, NodePeer, peerName)
			g.addEdge(peerID, groupID, EdgeInherits)

			addProcessBindings(g, peerEntry.Value, peerID, pluginContainer)
		}
	}

	for _, peerEntry := range bgp.GetListOrdered("peer") {
		peerName := peerEntry.Key
		peerID := tb.Reset().Str("peer/").Str(peerName).String()
		g.addNode(peerID, NodePeer, peerName)
		g.addEdge("section/bgp", peerID, EdgeContains)

		addProcessBindings(g, peerEntry.Value, peerID, pluginContainer)
	}
}

// addProcessBindings adds edges from a peer to the plugins it references via process bindings.
func addProcessBindings(g *Graph, peerTree *Tree, peerID string, pluginContainer *Tree) {
	var tb textbuf.Buffer
	processList := peerTree.GetList("process")
	for name, processTree := range processList {
		if name == KeyDefault {
			continue
		}

		run, hasRun := processTree.Get("run")
		use, hasUse := processTree.Get("use")

		if hasRun && run != "" {
			pluginID := tb.Reset().Str("plugin/").Str(name).String()
			g.addNode(pluginID, NodePlugin, name)
			g.addEdge(peerID, pluginID, EdgeProcessBinds)
			continue
		}
		if hasUse && use != "" {
			pluginID := tb.Reset().Str("plugin/").Str(name).String()
			g.addNode(pluginID, NodePlugin, name)
			g.addEdge(peerID, pluginID, EdgeProcessBinds)
			continue
		}

		if pluginContainer != nil {
			if internals := pluginContainer.GetList("internal"); internals != nil {
				if _, ok := internals[name]; ok {
					pluginID := tb.Reset().Str("plugin/").Str(name).String()
					g.addNode(pluginID, NodePlugin, name)
					g.addEdge(peerID, pluginID, EdgeProcessBinds)
					continue
				}
			}
			if externals := pluginContainer.GetList("external"); externals != nil {
				if _, ok := externals[name]; ok {
					pluginID := tb.Reset().Str("plugin/").Str(name).String()
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

	var tb textbuf.Buffer
	authzContainer := sys.GetContainer("authorization")
	if authzContainer != nil {
		for name := range authzContainer.GetList("profile") {
			profileID := tb.Reset().Str("profile/").Str(name).String()
			g.addNode(profileID, NodeProfile, name)
			g.addEdge("section/system", profileID, EdgeContains)
		}
	}

	authContainer := sys.GetContainer("authentication")
	if authContainer == nil {
		return
	}

	for username, userTree := range authContainer.GetList("user") {
		userID := tb.Reset().Str("user/").Str(username).String()
		g.addNode(userID, NodeUser, username)
		g.addEdge("section/system", userID, EdgeContains)

		for _, profileName := range userTree.GetSlice("profile") {
			g.addEdge(userID, tb.Reset().Str("profile/").Str(profileName).String(), EdgeReferences)
		}
	}
}

// addListenerNodes adds listener endpoint nodes and service ownership edges.
func addListenerNodes(g *Graph, tree *Tree, schema *Schema) {
	var tb textbuf.Buffer
	endpoints := CollectListeners(tree, schema)
	for i := range endpoints {
		ep := &endpoints[i]
		listenerID := tb.Reset().Str("listener/").Str(ep.Service).String()
		g.addNode(listenerID, NodeListener, ep.Service)

		sectionName := listenerSectionName(ep.Service)
		if sectionName != "" {
			g.addEdge(tb.Reset().Str("section/").Str(sectionName).String(), listenerID, EdgeListensOn)
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
	case service == listenerServiceSSH || strings.HasPrefix(service, listenerServiceSSH+" "):
		return listenerServiceSSH
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
	case strings.HasPrefix(service, sectionBGP):
		return sectionBGP
	}
	return ""
}

const (
	sectionBGP         = "bgp"
	sectionEnvironment = "environment"
	sectionInterface   = "interface"
	sectionPlugin      = "plugin"
)

// addPluginRegistryEdges adds edges from registered plugins to their declared config roots.
func addPluginRegistryEdges(g *Graph) {
	var tb textbuf.Buffer
	for pluginName, roots := range registry.ConfigRootsMap() {
		pluginID := tb.Reset().Str("plugin/").Str(pluginName).String()
		g.addNode(pluginID, NodePlugin, pluginName)
		for _, root := range roots {
			g.addEdge(pluginID, tb.Reset().Str("section/").Str(root).String(), EdgeConfigRoot)
		}
	}

	for _, reg := range registry.All() {
		if len(reg.Dependencies) == 0 {
			continue
		}
		src := tb.Reset().Str("plugin/").Str(reg.Name).String()
		for _, dep := range reg.Dependencies {
			g.addEdge(src, tb.Reset().Str("plugin/").Str(dep).String(), EdgeDependsOn)
		}
	}
}

// ifaceListKinds enumerates the interface list names that carry addresses.
// Each corresponds to a keyed list under the "interface" container whose
// entries have unit > ipv4/ipv6 > address leaf-lists.
var ifaceListKinds = []string{
	"dummy", "ethernet", "veth", "bridge", "tunnel", "wireguard", "xfrm",
}

// addAddressNodes walks the interface config section and creates an address
// node for every configured CIDR. Each address node gets an EdgeContains
// edge from the section/interface node.
func addAddressNodes(g *Graph, tree *Tree) {
	iface := tree.GetContainer("interface")
	if iface == nil {
		return
	}

	var tb textbuf.Buffer
	for _, kind := range ifaceListKinds {
		for _, entry := range iface.GetListOrdered(kind) {
			ifName := entry.Key
			unitList := entry.Value.GetListOrdered("unit")
			for _, unitEntry := range unitList {
				for _, cidr := range collectUnitAddresses(unitEntry.Value) {
					addrID := tb.Reset().Str("address/").Str(ifName).Byte('/').Str(cidr).String()
					g.addNode(addrID, NodeAddress, tb.Reset().Str(ifName).Byte('/').Str(cidr).String())
					g.addEdge("section/interface", addrID, EdgeContains)
				}
			}
		}
	}

	// Loopback is a container, not a list entry.
	lo := iface.GetContainer("loopback")
	if lo != nil {
		for _, unitEntry := range lo.GetListOrdered("unit") {
			for _, cidr := range collectUnitAddresses(unitEntry.Value) {
				addrID := tb.Reset().Str("address/lo/").Str(cidr).String()
				g.addNode(addrID, NodeAddress, tb.Reset().Str("lo/").Str(cidr).String())
				g.addEdge("section/interface", addrID, EdgeContains)
			}
		}
	}
}

// collectUnitAddresses returns all CIDR strings from a unit tree's ipv4 and
// ipv6 family containers.
func collectUnitAddresses(unit *Tree) []string {
	var addrs []string
	for _, family := range []string{"ipv4", "ipv6"} {
		fam := unit.GetContainer(family)
		if fam == nil {
			continue
		}
		addrs = append(addrs, fam.GetSlice("address")...)
	}
	return addrs
}

// addAddressUsageEdges links each BGP peer whose connection > local > ip
// matches an existing address node with an EdgeUsesAddress edge.
func addAddressUsageEdges(g *Graph, tree *Tree) {
	// Build a set of address-node IPs (without prefix length) -> node IDs.
	addrIndex := make(map[string]string) // bare IP -> address node ID
	for _, n := range g.Nodes {
		if n.Kind != NodeAddress {
			continue
		}
		// Node ID is "address/<iface>/<cidr>". Extract the IP part of the CIDR.
		cidr := n.ID[strings.LastIndex(n.ID[:strings.LastIndex(n.ID, "/")], "/")+1:]
		ip, _, ok := strings.Cut(cidr, "/")
		if !ok {
			ip = cidr
		}
		addrIndex[ip] = n.ID
	}
	if len(addrIndex) == 0 {
		return
	}

	bgp := tree.GetContainer(sectionBGP)
	if bgp == nil {
		return
	}

	linkPeerAddress := func(peerID string, peerTree *Tree) {
		conn := peerTree.GetContainer("connection")
		if conn == nil {
			return
		}
		local := conn.GetContainer("local")
		if local == nil {
			return
		}
		ip, ok := local.Get("ip")
		if !ok || ip == "" || ip == "auto" {
			return
		}
		if addrNodeID, found := addrIndex[ip]; found {
			g.addEdge(peerID, addrNodeID, EdgeUsesAddress)
		}
	}

	for _, groupEntry := range bgp.GetListOrdered("group") {
		for _, peerEntry := range groupEntry.Value.GetListOrdered("peer") {
			linkPeerAddress("peer/"+peerEntry.Key, peerEntry.Value)
		}
	}
	for _, peerEntry := range bgp.GetListOrdered("peer") {
		linkPeerAddress("peer/"+peerEntry.Key, peerEntry.Value)
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
	case NodeAddress:
		return NodeAddress
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

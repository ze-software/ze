// Design: docs/architecture/chaos-web-dashboard.md -- the AS path and next-hop graphs of a simulated network, compared with the model's announcements
// Related: lg_simulated_network.go -- the run these questions are asked of, and the list they are entries of
// Related: lg_simulated_trace.go -- the same comparison for the trace graph

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"

	chaosscenario "github.com/ze-software/ze/internal/chaos/scenario"
)

// The words of the two graphs' text forms this file reads
// (internal/component/lg/graph.go, renderGraphText, and graph_nexthop.go,
// renderNextHopGraphText), and the answer both modes give for a prefix with
// nothing to draw (handler_graph.go, handleASPathGraph).
const (
	lgSimulatedGraphASPath   = "aspath"
	lgSimulatedGraphNextHop  = "nexthop"
	lgSimulatedGraphNode     = "node"
	lgSimulatedGraphEdge     = "edge"
	lgSimulatedGraphEgress   = "egress"
	lgSimulatedGraphOutside  = "outside"
	lgSimulatedGraphArrow    = "->"
	lgSimulatedGraphNoRoutes = "No routes found\n"
)

// lgSimulatedGraphText is what the fixture reads of one graph in its text
// form: the nodes, the nodes marked egress, the nodes marked outside, and the
// edges, each sorted. The layer of a node is not kept: it is the drawing's own
// arrangement, and the model states none.
type lgSimulatedGraphText struct {
	nodes   []string
	egress  []string
	outside []string // The next-hop graph's ASes outside the local one.
	edges   []string // "<from> -> <to>".
}

// lgSimulatedReadGraph reads the text form of one graph of the given mode. A
// text that does not start with the mode's header is refused, so an error page
// or an empty answer is never read as a graph with no node.
func lgSimulatedReadGraph(text, mode string) (lgSimulatedGraphText, error) {
	var read lgSimulatedGraphText
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if header := "mode " + mode; lines[0] != header {
		return read, fmt.Errorf("the graph does not start with %q:\n%s", header, text)
	}

	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return read, fmt.Errorf("a line of the %s graph names nothing: %q", mode, line)
		}
		switch fields[0] {
		case lgSimulatedGraphNode:
			read.nodes = append(read.nodes, fields[1])
			if slices.Contains(fields[2:], lgSimulatedGraphEgress) {
				read.egress = append(read.egress, fields[1])
			}
			if slices.Contains(fields[2:], lgSimulatedGraphOutside) {
				read.outside = append(read.outside, fields[1])
			}
		case lgSimulatedGraphEdge:
			if len(fields) != 4 {
				return read, fmt.Errorf("an edge of the %s graph is not `edge <from> -> <to>`: %q", mode, line)
			}
			if fields[2] != lgSimulatedGraphArrow {
				return read, fmt.Errorf("an edge of the %s graph is not `edge <from> -> <to>`: %q", mode, line)
			}
			read.edges = append(read.edges, fields[1]+" "+lgSimulatedGraphArrow+" "+fields[3])
		default:
			// The text form is this repository's own, so a line this reader
			// does not know is a change it must learn.
			return read, fmt.Errorf("the %s graph holds a line that is no node and no edge: %q", mode, line)
		}
	}
	read.sort()
	return read, nil
}

// sort orders the four lists, so two graphs compare whatever order they were
// read or built in.
func (g *lgSimulatedGraphText) sort() {
	slices.Sort(g.nodes)
	slices.Sort(g.egress)
	slices.Sort(g.outside)
	slices.Sort(g.edges)
}

// lgSimulatedGraphProblem answers how a graph differs from the model's, ""
// when nothing does.
func lgSimulatedGraphProblem(got, want lgSimulatedGraphText) string {
	if !slices.Equal(got.nodes, want.nodes) {
		return fmt.Sprintf("holds the nodes %v, and the model computed %v", got.nodes, want.nodes)
	}
	if !slices.Equal(got.egress, want.egress) {
		return fmt.Sprintf("marks %v as egress, and the model computed %v", got.egress, want.egress)
	}
	if !slices.Equal(got.outside, want.outside) {
		return fmt.Sprintf("marks %v as outside, and the model computed %v", got.outside, want.outside)
	}
	if !slices.Equal(got.edges, want.edges) {
		return fmt.Sprintf("holds the edges %v, and the model computed %v", got.edges, want.edges)
	}
	return ""
}

// lgSimulatedAnnouncementsOf answers what every speaker of the model sends the
// collector for prefix, each with the speaker that sends it: the rows the one
// RIB query of a graph answers.
func lgSimulatedAnnouncementsOf(topology *chaosscenario.Topology, prefix netip.Prefix) map[netip.Addr]chaosscenario.Announcement {
	sent := map[netip.Addr]chaosscenario.Announcement{}
	for _, speaker := range topology.Speakers() {
		for _, announcement := range speaker.Announcements {
			if announcement.Prefix == prefix {
				sent[speaker.Address] = announcement
			}
		}
	}
	return sent
}

// lgSimulatedASPathWant answers the AS path graph the model expects for
// prefix: one node per AS on any speaker's path, and one edge per step between
// two different ASes, toward the origin. A repeated AS is one node and no
// step, as the graph collapses a prepended AS (internal/graph,
// BuildGraphFromPaths).
func lgSimulatedASPathWant(topology *chaosscenario.Topology, prefix netip.Prefix) lgSimulatedGraphText {
	nodes := map[string]bool{}
	edges := map[string]bool{}
	for _, announcement := range lgSimulatedAnnouncementsOf(topology, prefix) {
		previous := ""
		for _, number := range announcement.ASPath {
			node := "AS" + strconv.FormatUint(uint64(number), 10)
			nodes[node] = true
			if previous != "" && previous != node {
				edges[previous+" "+lgSimulatedGraphArrow+" "+node] = true
			}
			previous = node
		}
	}
	return lgSimulatedGraphOf(nodes, nil, nil, edges)
}

// lgSimulatedNextHopWant answers the next-hop graph the model expects for
// prefix. A speaker of the AS (an internal session) and an address that is no
// speaker's are each a node by address. A speaker outside the AS (an external
// session) is the node of its AS, marked outside, and never its address, so
// two such speakers of one AS are one node. A router that announces its own
// address is marked egress, and every other speaker has an edge to the node of
// its next hop.
//
// The session kind and the AS are the model's own (Speaker.Session,
// Speaker.AS), never read back from the looking glass.
func lgSimulatedNextHopWant(topology *chaosscenario.Topology, prefix netip.Prefix) lgSimulatedGraphText {
	outsideAS := map[netip.Addr]string{}
	for _, speaker := range topology.Speakers() {
		if speaker.Session == chaosscenario.SessionKindExternal {
			outsideAS[speaker.Address] = "AS" + strconv.FormatUint(uint64(speaker.AS), 10)
		}
	}
	nodes := map[string]bool{}
	egress := map[string]bool{}
	outside := map[string]bool{}
	edges := map[string]bool{}
	nodeOf := func(address netip.Addr) string {
		node, isOutside := outsideAS[address]
		if !isOutside {
			return address.String()
		}
		outside[node] = true
		return node
	}
	for speaker, announcement := range lgSimulatedAnnouncementsOf(topology, prefix) {
		from, to := nodeOf(speaker), nodeOf(announcement.NextHop)
		nodes[from] = true
		nodes[to] = true
		if from != to {
			edges[from+" "+lgSimulatedGraphArrow+" "+to] = true
			continue
		}
		if !outside[from] {
			egress[from] = true
		}
	}
	return lgSimulatedGraphOf(nodes, egress, outside, edges)
}

// lgSimulatedGraphOf answers the sorted graph of four sets.
func lgSimulatedGraphOf(nodes, egress, outside, edges map[string]bool) lgSimulatedGraphText {
	var graph lgSimulatedGraphText
	for node := range nodes {
		graph.nodes = append(graph.nodes, node)
	}
	for node := range egress {
		graph.egress = append(graph.egress, node)
	}
	for node := range outside {
		graph.outside = append(graph.outside, node)
	}
	for edge := range edges {
		graph.edges = append(graph.edges, edge)
	}
	graph.sort()
	return graph
}

// lgSimulatedGraphAnswer asks one graph of prefix in its text form.
func lgSimulatedGraphAnswer(ctx context.Context, base, mode string, prefix netip.Prefix) (string, error) {
	target := base + "/lg/graph?mode=" + mode + "&format=text&prefix=" + url.QueryEscape(prefix.String())
	return lgCoveringRequest(ctx, http.MethodGet, target, "")
}

// lgSimulatedASPathGraph asks the AS path graph of every prefix of the model
// and compares its ASes and its steps with the paths the model's speakers
// announce. A prefix no speaker announces with an AS (it originates inside the
// AS) has nothing to draw, and the answer must be the empty one.
//
// It is the one place a real bgp-rib is asked `mode=aspath`: the AS path graph
// is otherwise proven on rows written by hand.
func lgSimulatedASPathGraph(ctx context.Context, base string, topology *chaosscenario.Topology) error {
	drawn, steps, empty := 0, 0, 0
	for _, prefix := range topology.Prefixes() {
		text, err := lgSimulatedGraphAnswer(ctx, base, lgSimulatedGraphASPath, prefix)
		if err != nil {
			return err
		}
		want := lgSimulatedASPathWant(topology, prefix)
		if len(want.nodes) == 0 {
			if text != lgSimulatedGraphNoRoutes {
				return fmt.Errorf("the AS path graph of %s, whose paths hold no AS, answered %q, and the empty answer is %q",
					prefix, text, lgSimulatedGraphNoRoutes)
			}
			empty++
			continue
		}
		got, err := lgSimulatedReadGraph(text, lgSimulatedGraphASPath)
		if err != nil {
			return fmt.Errorf("the AS path graph of %s: %w", prefix, err)
		}
		if problem := lgSimulatedGraphProblem(got, want); problem != "" {
			return fmt.Errorf("the AS path graph of %s %s", prefix, problem)
		}
		drawn++
		steps += len(want.edges)
	}

	if drawn == 0 {
		return errors.New("no prefix of the network has an AS path, so the AS path graph is unproven")
	}
	if steps == 0 {
		return errors.New("no path of the network is longer than one AS, so the steps of the AS path graph are unproven")
	}
	fmt.Fprintf(os.Stderr,
		"OK: every AS path graph held the ASes and the steps of the paths the model computed: %d prefixes, %d steps, %d with no AS to draw\n",
		drawn, steps, empty)
	return nil
}

// lgSimulatedNextHopGraph asks the next-hop graph of every prefix of the model
// and compares each speaker's next hop, the egress marks and the ASes drawn
// for the speakers outside the AS with what the model's speakers announce and
// the kind of session each holds.
//
// It is the one place a real bgp-rib is asked `mode=nexthop` of routes that
// arrived over sessions.
func lgSimulatedNextHopGraph(ctx context.Context, base string, topology *chaosscenario.Topology) error {
	prefixes, egress, outside, edges := 0, 0, 0, 0
	for _, prefix := range topology.Prefixes() {
		text, err := lgSimulatedGraphAnswer(ctx, base, lgSimulatedGraphNextHop, prefix)
		if err != nil {
			return err
		}
		got, err := lgSimulatedReadGraph(text, lgSimulatedGraphNextHop)
		if err != nil {
			return fmt.Errorf("the next-hop graph of %s: %w", prefix, err)
		}
		want := lgSimulatedNextHopWant(topology, prefix)
		if problem := lgSimulatedGraphProblem(got, want); problem != "" {
			return fmt.Errorf("the next-hop graph of %s %s", prefix, problem)
		}
		prefixes++
		egress += len(want.egress)
		outside += len(want.outside)
		edges += len(want.edges)
	}

	if egress == 0 {
		return errors.New("no speaker of the network announces its own address, so the egress mark is unproven")
	}
	if outside == 0 {
		return errors.New("no speaker outside the AS announces a prefix of the network, so the outside AS node is unproven")
	}
	if edges == 0 {
		return errors.New("no speaker of the network announces another address, so the next-hop edge is unproven")
	}
	fmt.Fprintf(os.Stderr,
		"OK: every next-hop graph held each speaker's next hop and the egress marks the model computed: %d prefixes, %d egress marks, %d outside ASes, %d edges\n",
		prefixes, egress, outside, edges)
	return nil
}

// Design: docs/architecture/testing/interop.md -- the topology suite.
// Related: register_catalog.go -- where the suite lives and how it runs.
// Related: register_ze_static_link_chain.go -- three Ze nodes on two links.
// Related: register_two_ze_pppoe_link.go -- PPPoE across a layer-2 link.
package topology

import (
	"context"
	"maps"
	"net/netip"
	"path/filepath"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// node is one planned peer of a topology scenario. Its image is a name some
// suite registered (interoplab.RegisterImage), so a node runs any suite's
// image. A node with a config file mounts it from the scenario directory.
type node struct {
	name  string
	image string
	host  uint8
	// config is the file in the scenario directory the node mounts at
	// mountedAt, empty for a node that reads none.
	config    string
	mountedAt string
	// command replaces the image's command; empty runs the image's own.
	command      []string
	capabilities []string
	arguments    []string
	prestart     string
	// shared are files the node mounts from a lab directory several
	// scenarios share, each named relative to the checkout, so one file
	// serves every scenario that runs the same node.
	shared []sharedFile
	// ready is the command the core waits on before it starts the next node,
	// empty for a node the next one does not depend on: the checker then
	// waits on the state it reads.
	ready []string
}

// sharedFile is one read-only mount of a node: source is a slash path
// relative to the checkout, target the path inside the container.
type sharedFile struct {
	source string
	target string
}

// scenario is one topology scenario's declaration: its nodes in start order,
// its links, and its checker. A register file adds it to scenarios.
type scenario struct {
	nodes   []node
	links   []interoplab.Link
	checker interoplab.Checker
}

// scenarios holds every topology scenario by its directory name. Register
// files fill it from init(), so the runner names no scenario.
var scenarios = map[string]scenario{}

// Options carries the native scenario selector and image-build controls.
type Options struct {
	Scenario string
	NoBuild  bool
	Suffix   string
}

// Run resolves the checkout and runs the topology suite.
func Run(ctx context.Context, options Options) interoplab.SuiteReport {
	root, err := lepath.Root()
	if err != nil {
		return setupFailure(err)
	}
	return RunAt(ctx, root, options)
}

// RunScenario runs exactly the scenario named name, for the catalog.
func RunScenario(ctx context.Context, root, name string) interoplab.SuiteReport {
	return RunAt(ctx, root, Options{Scenario: name})
}

// RunAt runs the topology suite against root. The kernel check, the orphan
// sweep and every refusal are the core's (interoplab.Suite.Run); the suite
// adds only the plans and the images they name.
func RunAt(ctx context.Context, root string, options Options) interoplab.SuiteReport {
	environment := interoplab.ReadEnvironment(interoplab.EnvironmentOptions{
		SelectorVariable: selectorVariable,
		SuffixVariable:   suffixVariable,
	})
	if options.Scenario == "" {
		options.Scenario = environment.Selector
	}
	if options.Suffix == "" {
		options.Suffix = environment.Suffix
	}
	options.NoBuild = options.NoBuild || environment.NoBuild
	sources, err := interoplab.Discover(filepath.Join(root, filepath.FromSlash(scenarioDirectory)), options.Scenario, checkers())
	if err != nil {
		return setupFailure(err)
	}
	plans := make([]interoplab.ScenarioPlan, 0, len(sources))
	var images []string
	for _, source := range sources {
		plan := scenarioPlan(root, source, scenarios[source.Name], options.Suffix)
		for index := range plan.Peers {
			images = append(images, plan.Peers[index].Image)
		}
		plans = append(plans, plan)
	}
	builds, binaries, err := interoplab.ResolveImages(interoplab.ImageContext{Root: root}, images)
	if err != nil {
		return setupFailure(err)
	}
	suite := interoplab.Suite{
		SuffixVariable: environment.SuffixVariable,
		Docker:         interoplab.NewDocker(),
		StagedZe:       interoplab.StagedZePath(root, binaries),
		Preflight:      interoplab.StageBinaries(root, options.NoBuild, binaries...),
		Images:         builds,
		Scenarios:      plans,
		NoBuild:        options.NoBuild,
		Checkout:       root,
	}
	return suite.Run(ctx)
}

func setupFailure(err error) interoplab.SuiteReport {
	return interoplab.SuiteReport{SetupError: err.Error(), Code: 1}
}

// checkers answers every declared scenario's checker, for Discover to join
// against the scenario directories.
func checkers() map[string]interoplab.Checker {
	joined := make(map[string]interoplab.Checker, len(scenarios))
	for name, declared := range scenarios {
		joined[name] = declared.checker
	}
	return joined
}

// ScenarioNames answers every declared scenario in lexical order.
func ScenarioNames() []string {
	return slices.Sorted(maps.Keys(scenarios))
}

// scenarioPlan turns a declaration into the plan the core runs: one network,
// one peer per node, and the declared links. root is the checkout a node's
// shared files are named relative to.
func scenarioPlan(root string, source interoplab.ScenarioSource, declared scenario, suffix string) interoplab.ScenarioPlan {
	peers := make([]interoplab.PeerConfig, 0, len(declared.nodes))
	for index := range declared.nodes {
		peers = append(peers, nodePeer(root, source.Directory, &declared.nodes[index], suffix))
	}
	var network textbuf.Buffer
	return interoplab.ScenarioPlan{
		Source:  source,
		Network: interoplab.NetworkSpec{Name: network.Str("ze-topo-").Str(suffix).String(), Candidates: subnets},
		Peers:   peers,
		Links:   declared.links,
	}
}

// nodePeer is one node's container. Readiness waits on the node's ready
// command when it declares one, and on nothing otherwise: each checker waits
// on the state it reads.
func nodePeer(root, directory string, declared *node, suffix string) interoplab.PeerConfig {
	var container textbuf.Buffer
	peer := interoplab.PeerConfig{Name: declared.name,
		Container: container.Str("ze-topo-").Str(declared.name).Byte('-').Str(suffix).String(),
		Image:     declared.image, Host: declared.host, Command: declared.command,
		Capabilities: declared.capabilities, Arguments: declared.arguments, Prestart: declared.prestart,
		Ready: &interoplab.ReadyProbe{Command: []string{"true"}, Timeout: 30 * time.Second, Interval: time.Second}}
	if declared.config != "" {
		peer.Mounts = append(peer.Mounts, interoplab.Mount{Source: filepath.Join(directory, declared.config), Target: declared.mountedAt, ReadOnly: true})
	}
	for _, file := range declared.shared {
		peer.Mounts = append(peer.Mounts, interoplab.Mount{Source: filepath.Join(root, filepath.FromSlash(file.source)), Target: file.target, ReadOnly: true})
	}
	if len(declared.ready) != 0 {
		peer.Ready = &interoplab.ReadyProbe{Command: declared.ready, Timeout: 90 * time.Second, Interval: time.Second}
	}
	return peer
}

// zeNode is a Ze node of a topology scenario: the ze image the BGP suite
// registers, its own configuration file and NET_ADMIN for the routes and
// devices it programs. A checker that asks the node's CLI uses
// interoplab.ZeCLICommand, whose account the node's configuration then
// carries; one that reads its REST surface needs the api-server block.
func zeNode(name string, host uint8, config string, arguments ...string) node {
	return node{name: name, image: zeImage, host: host, config: config, mountedAt: zeMountedConfig,
		command: []string{"start", zeMountedConfig}, capabilities: []string{capabilityNetAdmin}, arguments: arguments}
}

const (
	// zeImage is the ze image the BGP suite registers (bgp/register_images.go).
	zeImage            = "ze"
	zeMountedConfig    = "/etc/ze/ze.conf"
	capabilityNetAdmin = "NET_ADMIN"
	// suffixVariable sets this lab's container and network name suffix.
	suffixVariable = "ZE_TOPOLOGY_INTEROP_SUFFIX"
)

// subnets are the scenario network's candidates, distinct from every other
// suite's so two suites can run at once.
var subnets = []interoplab.Subnet{
	{IPv4: netip.MustParsePrefix("172.31.181.0/24")},
	{IPv4: netip.MustParsePrefix("172.31.182.0/24")},
}

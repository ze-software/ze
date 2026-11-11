// Design: docs/architecture/testing/interop.md -- Ze's readiness probe.
// Related: prepare.go -- zeDaemonPeer, scenarioPeers.
// Related: extended_relay.go -- prepareExtendedRelayPeers keeps the listener barrier.
package bgp

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	bgpconfig "github.com/ze-software/ze/internal/component/bgp/config"
	"github.com/ze-software/ze/internal/component/config"

	// The schema a scenario's Ze config is parsed against: every root the
	// daemon in the ze image accepts, so the parse refuses only what Ze would.
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// zeDefaultListenPort is the port Ze listens on for a peer whose
// `connection local port` is absent (ze-bgp-conf.yang, "The default is 179").
const zeDefaultListenPort = "179"

// zeListenerCommand lists the TCP listeners in a Ze container. iproute2
// carries ss in the ze image (test/interop/Dockerfile.ze).
var zeListenerCommand = []string{"ss", "-ltn"}

// readZeConfig parses the Ze config at path against the full schema, the one
// parse of a scenario's ze.conf (or ze-peer.conf) that every question the lab
// asks of it reads: Ze's readiness probe and whether Ze sends BMP. A file
// that cannot be read or parsed fails the scenario's preparation, naming the
// file, because no answer read from a config Ze would refuse is an answer.
func readZeConfig(path string) (*config.Tree, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path is a rendered scenario config under the run's own directory
	if err != nil {
		return nil, fmt.Errorf("read %s for the interop lab: %w", filepath.Base(path), err)
	}
	tree, err := config.ParseTreeForValidation(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse %s for the interop lab: %w", filepath.Base(path), err)
	}
	return tree, nil
}

// zeBMPSenderPath is the container whose presence starts Ze's BMP sender
// (ze-bmp-conf.yang, `container sender { presence "BMP sender enabled"; }`
// under the bgp augment). The receiver lives under `environment bmp` and
// sends nothing a collector reads.
const zeBMPSenderPath = "bgp" + config.PathSep + "bmp" + config.PathSep + "sender"

// zeSendsBMP answers whether a Ze started from tree runs its BMP sender, so
// the lab owes it a collector. The answer is read from the parsed config:
// a text match on `bmp {` fired on a comment, missed another spacing, and
// also matched the receiver's `environment bmp`, which dials no collector.
func zeSendsBMP(tree *config.Tree) bool {
	return tree.GetContainerPath(zeBMPSenderPath) != nil
}

// zePassiveListenPorts answers the ports a Ze started from tree, the parsed
// config file name, listens on for a peer it never dials: a peer whose
// `connection remote connect` is false, and a dynamic group (`remote ip
// dynamic`), which has no address to dial and which Ze refuses without
// `connect false`. Ze refuses connect and accept both false, so each such
// peer accepts, and the reactor binds its configured local address at its
// listen port (listenerSpecs, listenPortFor). The config is resolved by
// ResolveBGPTree, the resolver the daemon runs, so a group's leaves reach its
// peers here exactly as they do in Ze. The answer is sorted, holds each port
// once, and is empty when Ze dials every peer or the config holds no bgp
// block. The question is answered from the parsed config, never from its
// text: a text match fires on a comment.
func zePassiveListenPorts(tree *config.Tree, name string) ([]string, error) {
	if tree.GetContainer("bgp") == nil {
		return nil, nil
	}
	resolved, err := bgpconfig.ResolveBGPTree(tree)
	if err != nil {
		return nil, fmt.Errorf("resolve %s for the Ze readiness probe: %w", name, err)
	}
	var ports []string
	if peers, ok := resolved["peer"].(map[string]any); ok {
		for peerName, entry := range peers {
			peer, isBlock := entry.(map[string]any)
			if !isBlock {
				return nil, fmt.Errorf("resolve %s for the Ze readiness probe: peer %s is not a block", name, peerName)
			}
			remote, hasRemote := config.BGPRemote(peer)
			if !hasRemote {
				continue
			}
			if remote["connect"] == "false" {
				ports = append(ports, zeListenPort(peer))
			}
		}
	}
	if groups, ok := resolved["dynamic-groups"].([]bgpconfig.DynamicGroupTemplate); ok {
		for index := range groups {
			ports = append(ports, zeListenPort(groups[index].Template))
		}
	}
	slices.Sort(ports)
	return slices.Compact(ports), nil
}

// zeListenPort is the port Ze listens on for a resolved peer or dynamic group
// template: its `connection local port`, or the YANG default where the
// resolved block sets none.
func zeListenPort(peer map[string]any) string {
	port := zeDefaultListenPort
	if connection, ok := peer["connection"].(map[string]any); ok {
		if local, hasLocal := connection["local"].(map[string]any); hasLocal {
			if value, hasPort := local["port"].(string); hasPort {
				port = value
			}
		}
	}
	return port
}

// zeReadyProbe is the readiness of a Ze daemon. A Ze that listens for a peer
// it never dials is ready once ss lists a listener on each of listenPorts:
// the lab starts the next peer as soon as this probe passes, and a
// third-party daemon that dials before Ze listens is refused, then waits its
// connect-retry (120 s by default in FRR and BIRD) before it dials again,
// which outlasts the session budget. A Ze that dials every peer needs no
// barrier: it dials the daemons started after it and redials on its own
// backoff (reactor DefaultReconnectMin), so its probe only runs a command.
func zeReadyProbe(listenPorts []string) *interoplab.ReadyProbe {
	probe := &interoplab.ReadyProbe{Command: []string{"true"}, Timeout: 30 * time.Second, Interval: time.Second}
	if len(listenPorts) == 0 {
		return probe
	}
	probe.Command = slices.Clone(zeListenerCommand)
	probe.Contains = make([]string, len(listenPorts))
	for index, port := range listenPorts {
		// ss pads the local address column, so ":179 " cannot match ":1790".
		probe.Contains[index] = ":" + port + " "
	}
	return probe
}

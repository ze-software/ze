// Design: docs/architecture/testing/interop.md -- pre-daemon native GoBGP observation.
// Related: prepare.go -- the existing GoBGP role calls this leaf preparation.
package bgp

import (
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

var gobgpReloadCapture = packetCapture{base: "/tmp/gobgp-reload", filter: "ip and tcp port 179", seconds: 600, packets: 4096}

// prepareGoBGPReloadPeers changes only the two successor topologies. The caller
// MUST retain tini as the signal-forwarding parent; the checker MUST complete
// and join the native capture before interpreting absence on the wire.
func prepareGoBGPReloadPeers(peers []interoplab.PeerConfig, scenario string) ([]interoplab.PeerConfig, error) {
	index := slices.IndexFunc(peers, func(peer interoplab.PeerConfig) bool { return peer.Name == peerGoBGP })
	if index < 0 {
		return nil, fmt.Errorf("GoBGP successor has no prepared GoBGP role")
	}
	peer := &peers[index]
	for _, phase := range gobgpReloadPhases[1:] {
		name := "gobgp-reload-" + phase + ".toml"
		path := filepath.Join(scenario, name)
		if !regularFile(path) {
			return nil, fmt.Errorf("GoBGP reload successor is missing %s", name)
		}
		peer.Mounts = append(peer.Mounts, interoplab.Mount{Source: path, Target: gobgpReloadMounted + name, ReadOnly: true})
	}
	prestart, err := gobgpReloadCapture.prestart()
	if err != nil {
		return nil, err
	}
	command := prestart + "cp /etc/gobgp/gobgp.toml " + gobgpReloadRunning +
		" && exec gobgpd -f " + gobgpReloadRunning
	peer.Arguments = append(peer.Arguments, dockerEntrypointFlag, "tini")
	peer.Command = []string{"--", "/bin/sh", "-c", command}
	peer.Ready = &interoplab.ReadyProbe{
		Command: []string{"gobgp", "global", "-j"},
		// GetBgp also succeeds before StartBgp; require its configured AS field.
		Contains: []string{`"asn":65005,`},
		Timeout:  30 * time.Second,
		Interval: time.Second,
	}
	// The native runner starts peers sequentially. Put the ready listener first
	// so no source can dial it before capture activation and daemon readiness.
	first := *peer
	copy(peers[1:index+1], peers[:index])
	peers[0] = first
	return peers, nil
}

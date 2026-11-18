// Design: docs/architecture/testing/interop.md -- independent native transit peers.
// Related: prepare_registry.go -- exact-scenario registration, no common switch.
package bgp

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/interoplab"
)

func prepareBestRouteTransitFRR(peers []interoplab.PeerConfig, directory string) ([]interoplab.PeerConfig, error) {
	return bestRouteTransitCopyPeer(peers, directory, peerFRR, bestRouteTransitFRRBackup,
		"frr-backup.conf", frrConfigPath, 14)
}

func prepareBestRouteTransitBIRD(peers []interoplab.PeerConfig, directory string) ([]interoplab.PeerConfig, error) {
	peers, err := bestRouteTransitCopyPeer(peers, directory, peerBIRD, bestRouteTransitBIRDBackup,
		"bird-backup.conf", "/etc/bird/bird.conf", 14)
	if err != nil {
		return nil, err
	}
	return bestRouteTransitCopyPeer(peers, directory, peerBIRD, bestRouteTransitBIRDSink,
		"bird-sink.conf", "/etc/bird/bird.conf", 15)
}

// bestRouteTransitCopyPeer preserves the lab's pinned image, daemon start,
// capabilities and readiness contract. Only the native role's own name,
// address and mounted configuration differ. All shared config slices remain
// immutable; the one changed slice (Mounts) is owned by the new role.
func bestRouteTransitCopyPeer(peers []interoplab.PeerConfig, directory, source, name, file, target string, host uint8) ([]interoplab.PeerConfig, error) {
	index := slices.IndexFunc(peers, func(peer interoplab.PeerConfig) bool { return peer.Name == source })
	if index < 0 {
		return nil, fmt.Errorf("ordinary transit has no prepared %s source", source)
	}
	if slices.ContainsFunc(peers, func(peer interoplab.PeerConfig) bool { return peer.Name == name }) {
		return nil, fmt.Errorf("ordinary transit already has peer %s", name)
	}
	path := filepath.Join(directory, file)
	if !regularFile(path) {
		return nil, fmt.Errorf("ordinary transit is missing native config %s", file)
	}
	suffix, valid := strings.CutPrefix(peers[index].Container, containerName(source, ""))
	if !valid {
		return nil, errors.New("ordinary transit source has no lab container identity")
	}
	peer := peers[index]
	peer.Name, peer.Container, peer.Host = name, containerName(name, suffix), host
	peer.Mounts = slices.Clone(peer.Mounts)
	mountIndex := slices.IndexFunc(peer.Mounts, func(mount interoplab.Mount) bool { return mount.Target == target })
	if mountIndex < 0 {
		return nil, fmt.Errorf("ordinary transit %s has no native config mount", source)
	}
	peer.Mounts[mountIndex] = interoplab.Mount{Source: path, Target: target, ReadOnly: true}
	return append(peers, peer), nil
}

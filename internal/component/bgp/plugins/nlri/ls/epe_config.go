// Design: docs/architecture/wire/nlri-bgpls.md -- native BGP EPE state
// RFC: rfc/short/rfc9086.md -- PeerNode SID and SRGB origination

package ls

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const epeName = "bgp-epe"

type epePeerConfig struct {
	index  uint32
	weight uint8
}

type epeConfig struct {
	base  uint32
	size  uint32
	peers map[netip.Addr]epePeerConfig
}

func parseEPEConfig(sections []rpc.ConfigSection) (epeConfig, error) {
	var cfg epeConfig
	for _, section := range sections {
		if section.Root != epeName {
			continue
		}
		var wrapper map[string]any
		if err := json.Unmarshal([]byte(section.Data), &wrapper); err != nil {
			return cfg, err
		}
		tree := configvalue.Section(epeName, wrapper)
		if tree == nil {
			return cfg, nil
		}
		rangeMap, _ := tree["srgb"].(map[string]any)
		lower, lowerOK := configvalue.Int(rangeMap["lower-bound"])
		upper, upperOK := configvalue.Int(rangeMap["upper-bound"])
		if !lowerOK || !upperOK || lower < 16 || upper < lower || upper > 1048575 {
			return cfg, errors.New("BGP EPE requires an SRGB within labels 16..1048575")
		}
		cfg.base, cfg.size = uint32(lower), uint32(upper-lower+1)
		cfg.peers = make(map[netip.Addr]epePeerConfig)
		indices := make(map[uint32]struct{})
		for _, entry := range configvalue.ListEntries(tree["peer"]) {
			peer, err := netip.ParseAddr(entry.Key)
			if err != nil || peer.IsUnspecified() || peer.IsMulticast() {
				return epeConfig{}, fmt.Errorf("invalid EPE peer address %q", entry.Key)
			}
			index, ok := configvalue.Int(entry.Fields["sid-index"])
			if !ok || index < 0 || index >= int64(cfg.size) {
				return epeConfig{}, fmt.Errorf("EPE peer %s SID index is outside SRGB", peer)
			}
			if _, duplicate := indices[uint32(index)]; duplicate {
				return epeConfig{}, fmt.Errorf("EPE SID index %d assigned to multiple peers", index)
			}
			weight := int64(0)
			if raw, present := entry.Fields["weight"]; present {
				var valid bool
				weight, valid = configvalue.Int(raw)
				if !valid || weight < 0 || weight > 255 {
					return epeConfig{}, errors.New("EPE weight must be 0..255")
				}
			}
			indices[uint32(index)] = struct{}{}
			cfg.peers[peer] = epePeerConfig{index: uint32(index), weight: uint8(weight)}
		}
		if len(cfg.peers) == 0 {
			return epeConfig{}, errors.New("BGP EPE requires at least one peer SID assignment")
		}
		if len(cfg.peers) > exportRouteLimit/2 {
			return epeConfig{}, errors.New("BGP EPE peer assignments exceed topology export bound")
		}
		return cfg, nil
	}
	return cfg, nil
}

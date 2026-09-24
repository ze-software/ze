// Design: docs/architecture/wire/nlri-bgpls.md -- native BGP EPE state
// RFC: rfc/short/rfc9086.md -- PeerNode SID and SRGB origination

package epe

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Name is the plugin name, the configuration root and the link-state source name.
const Name = "bgp-epe"

// PeerConfig is the PeerNode SID assignment for one BGP peer.
type PeerConfig struct {
	Index  uint32
	Weight uint8
}

// Config is the parsed bgp-epe configuration: the local SRGB and the peer assignments.
// The zero value means the plugin is not configured.
type Config struct {
	Base  uint32
	Size  uint32
	Peers map[netip.Addr]PeerConfig
}

// ParseConfig reads the bgp-epe configuration sections the engine delivers.
func ParseConfig(sections []rpc.ConfigSection) (Config, error) {
	var cfg Config
	for _, section := range sections {
		if section.Root != Name {
			continue
		}
		var wrapper map[string]any
		if err := json.Unmarshal([]byte(section.Data), &wrapper); err != nil {
			return cfg, err
		}
		tree := configvalue.Section(Name, wrapper)
		if tree == nil {
			return cfg, nil
		}
		rangeMap, _ := tree["srgb"].(map[string]any)
		lower, lowerOK := configvalue.Int(rangeMap["lower-bound"])
		upper, upperOK := configvalue.Int(rangeMap["upper-bound"])
		if !lowerOK || !upperOK || lower < 16 || upper < lower || upper > 1048575 {
			return cfg, errors.New("BGP EPE requires an SRGB within labels 16..1048575")
		}
		cfg.Base, cfg.Size = uint32(lower), uint32(upper-lower+1)
		cfg.Peers = make(map[netip.Addr]PeerConfig)
		indices := make(map[uint32]struct{})
		for _, entry := range configvalue.ListEntries(tree["peer"]) {
			peer, err := netip.ParseAddr(entry.Key)
			if err != nil || peer.IsUnspecified() || peer.IsMulticast() {
				return Config{}, fmt.Errorf("invalid EPE peer address %q", entry.Key)
			}
			index, ok := configvalue.Int(entry.Fields["sid-index"])
			if !ok || index < 0 || index >= int64(cfg.Size) {
				return Config{}, fmt.Errorf("EPE peer %s SID index is outside SRGB", peer)
			}
			if _, duplicate := indices[uint32(index)]; duplicate {
				return Config{}, fmt.Errorf("EPE SID index %d assigned to multiple peers", index)
			}
			weight := int64(0)
			if raw, present := entry.Fields["weight"]; present {
				var valid bool
				weight, valid = configvalue.Int(raw)
				if !valid || weight < 0 || weight > 255 {
					return Config{}, errors.New("EPE weight must be 0..255")
				}
			}
			indices[uint32(index)] = struct{}{}
			cfg.Peers[peer] = PeerConfig{Index: uint32(index), Weight: uint8(weight)}
		}
		if len(cfg.Peers) == 0 {
			return Config{}, errors.New("BGP EPE requires at least one peer SID assignment")
		}
		if len(cfg.Peers) > linkstateevents.RouteMax/2 {
			return Config{}, errors.New("BGP EPE peer assignments exceed topology export bound")
		}
		return cfg, nil
	}
	return cfg, nil
}

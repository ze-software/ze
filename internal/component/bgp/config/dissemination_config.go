// Design: docs/architecture/api/process-protocol.md -- candidate peer grants.
// Related: peers.go -- startup and reload use this same binding parser.
package bgpconfig

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/bgp/redistribute"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
)

func init() {
	if err := pluginserver.RegisterDisseminationConfigResolver(bgpevents.Namespace, disseminationPeersFromTree); err != nil {
		panic("BUG: BGP candidate attachment resolver registration failed: " + err.Error())
	}
}

// parsedPeerSettings is the resolved input for the remaining peer layers. Only
// peerSettingsFromTree constructs it; nil and the zero value are invalid.
type parsedPeerSettings struct {
	peers    []*reactor.PeerSettings
	skipped  []reactor.SkippedPeer // configured peers the parser skipped as incomplete
	groups   []*reactor.DynamicGroupConfig
	resolved map[string]any
	schema   *config.Schema
}

// peerSettingsFromTree is the shared, non-applying prefix of the peer loader.
// It resolves inheritance and defaults before parsing either static peers or
// dynamic templates. It deliberately judges no process declaration: the static
// caller and the authenticated Stage 1 caller supply their own declaration set.
// The input is owned by the caller; inactive pruning modifies it in place.
func peerSettingsFromTree(tree *config.Tree) (*parsedPeerSettings, error) {
	redistribute.RegisterBGPSources()
	schema, err := config.YANGSchema()
	if err != nil {
		return nil, fmt.Errorf("load schema for inactive pruning: %w", err)
	}
	config.PruneInactive(tree, schema)
	if err := validateFilterPolicies(tree); err != nil {
		return nil, err
	}
	bgpTree, err := ResolveBGPTree(tree)
	if err != nil {
		return nil, err
	}
	if err := CheckRequiredFields(schema, bgpTree); err != nil {
		return nil, err
	}
	applyPeerSchemaDefaults(bgpTree)
	peers, skipped, err := reactor.PeersAndSkippedFromTree(bgpTree)
	if err != nil {
		return nil, err
	}
	groups, err := dynamicGroupsFromTree(bgpTree)
	if err != nil {
		return nil, err
	}
	return &parsedPeerSettings{peers: peers, skipped: skipped, groups: groups, resolved: bgpTree, schema: schema}, nil
}

// disseminationPeersFromTree owns its rebuilt parsed tree and returns resolved
// DeliveryPeers carrying canonical bus paths, parent group paths, local bus
// blocks, and session facts.
func disseminationPeersFromTree(tree map[string]any) ([]pluginserver.DeliveryPeer, error) {
	parsed, err := config.TreeFromPluginMap(tree)
	if err != nil {
		return nil, err
	}
	if parsed.GetContainer("bgp") == nil {
		return nil, nil
	}
	snapshot := parsed.Clone()
	bindings, err := peerSettingsFromTree(parsed)
	if err != nil {
		return nil, err
	}
	delivery, _, _, groupInputs, err := buildBGPDeliveryPeersFromTree(snapshot, bindings.peers, bindings.groups)
	if err != nil {
		return nil, err
	}
	if len(groupInputs) > 0 {
		groupBlocks := make(map[string]pluginserver.BusParticipantInput, len(groupInputs))
		for _, g := range groupInputs {
			groupBlocks[g.Path] = g
		}
		for i := range delivery {
			if delivery[i].ParentPath == "" {
				continue
			}
			g, ok := groupBlocks[delivery[i].ParentPath]
			if !ok {
				continue
			}
			if g.OwnBlock.IsEmpty() {
				continue
			}
			hadOwn := !delivery[i].Bus.IsEmpty()
			delivery[i].Bus = mergeInheritedBusBlock(delivery[i].Bus, g.OwnBlock)
			if !hadOwn {
				delivery[i].BusOrigin = pluginserver.BusOriginInherited
				delivery[i].BusSource = g.OwnSource
			}
		}
	}
	return delivery, nil
}

// mergeInheritedBusBlock overlays parent onto child item-by-item: any field
// written on child wins, while omitted fields inherit from parent.
func mergeInheritedBusBlock(child, parent plugin.BusBlock) plugin.BusBlock {
	return plugin.BusBlock{
		Receive: mergeInheritedDirBlock(child.Receive, parent.Receive),
		Send:    mergeInheritedDirBlock(child.Send, parent.Send),
	}
}

func mergeInheritedDirBlock(child, parent plugin.BusDirectionBlock) plugin.BusDirectionBlock {
	out := child.Clone()
	if !out.AllowPresent && parent.AllowPresent {
		out.AllowPresent = true
		out.AllowNone = parent.AllowNone
		out.Allow = append([]string(nil), parent.Allow...)
	}
	if !out.TargetsPresent && parent.TargetsPresent {
		out.TargetsPresent = true
		out.TargetsNone = parent.TargetsNone
		out.Targets = append([]string(nil), parent.Targets...)
	}
	for msg, pRule := range parent.Messages {
		if pRule == nil {
			continue
		}
		if out.Messages == nil {
			out.Messages = make(map[string]*plugin.BusMessageRule)
		}
		cRule := out.Messages[msg]
		if cRule == nil {
			out.Messages[msg] = pRule.Clone()
			continue
		}
		if !cRule.TargetsPresent && pRule.TargetsPresent {
			cRule.TargetsPresent = true
			cRule.TargetsNone = pRule.TargetsNone
			cRule.Targets = append([]string(nil), pRule.Targets...)
		}
		for k, vals := range pRule.Tags {
			if cRule.Tags == nil {
				cRule.Tags = make(map[string][]string)
			}
			if _, exists := cRule.Tags[k]; !exists {
				cRule.Tags[k] = append([]string(nil), vals...)
			}
		}
	}
	return out
}

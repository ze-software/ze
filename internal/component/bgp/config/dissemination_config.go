// Design: docs/architecture/api/process-protocol.md -- candidate peer grants.
// Related: peers.go -- startup and reload use this same binding parser.
package bgpconfig

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/bgp/redistribute"
	"github.com/ze-software/ze/internal/component/config"
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
	peers, err := reactor.PeersFromTree(bgpTree)
	if err != nil {
		return nil, err
	}
	groups, err := dynamicGroupsFromTree(bgpTree)
	if err != nil {
		return nil, err
	}
	return &parsedPeerSettings{peers: peers, groups: groups, resolved: bgpTree, schema: schema}, nil
}

// disseminationPeersFromTree owns its rebuilt parsed tree and returns only
// resolved grants. Runtime declarations, including genuine external programs,
// are judged by the infrastructure caller rather than compiled-in metadata.
func disseminationPeersFromTree(tree map[string]any) ([]pluginserver.DeliveryPeer, error) {
	parsed, err := config.TreeFromPluginMap(tree)
	if err != nil {
		return nil, err
	}
	if parsed.GetContainer("bgp") == nil {
		return nil, nil
	}
	bindings, err := peerSettingsFromTree(parsed)
	if err != nil {
		return nil, err
	}
	peers := bindings.peers
	for _, group := range bindings.groups {
		peers = append(peers, group.Settings)
	}
	return reactor.DisseminationPeersFromSettings(peers), nil
}

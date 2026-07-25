// Design: docs/architecture/plugin/rib-storage-design.md -- RPKI config parsing
// Overview: rpki.go -- plugin entry point using parsed config
package rpki

import (
	"errors"
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
)

var errRpkiInvalidBgpConfigJson = errors.New("rpki: invalid BGP config JSON")

// cacheServerConfig holds parsed config for a single RTR cache server.
type cacheServerConfig struct {
	Address       string
	Port          uint16
	Preference    uint8
	SourceAddress string
}

// ASPA policy actions.
const (
	ASPAPolicyReject  uint8 = 0
	ASPAPolicyLogOnly uint8 = 1
	ASPAPolicyAccept  uint8 = 2
)

// aspaActionFromString converts a config string to a policy action constant.
func aspaActionFromString(s string) (uint8, bool) {
	switch s {
	case "reject":
		return ASPAPolicyReject, true
	case "log-only":
		return ASPAPolicyLogOnly, true
	case "accept":
		return ASPAPolicyAccept, true
	default:
		return 0, false
	}
}

// actionSource identifies the config level a resolved per-peer action came from.
type actionSource uint8

const (
	sourceGlobal actionSource = iota
	sourceGroup
	sourcePeer
)

// String renders the source for the `show bgp rpki status` peer-actions display.
func (s actionSource) String() string {
	switch s {
	case sourcePeer:
		return "peer"
	case sourceGroup:
		return "group"
	default:
		return "global"
	}
}

// resolvedAction is an action value plus the config level it was resolved from.
type resolvedAction struct {
	Action uint8
	Source actionSource
}

// peerActionSet holds the fully-resolved RPKI actions for one peer, merged
// peer > group > global per leaf, with each leaf's source retained for display.
type peerActionSet struct {
	OriginInvalid  resolvedAction
	OriginNotFound resolvedAction
	ASPAInvalid    resolvedAction
	ASPAUnknown    resolvedAction
}

// rpkiConfig holds the parsed RPKI plugin configuration.
type rpkiConfig struct {
	CacheServers      []cacheServerConfig
	ValidationTimeout uint16 // seconds, 0 = use default (30s)
	ASPAValidation    bool   // enable/disable ASPA path verification (default false)
	ASPAInvalidAction uint8  // action for ASPA Invalid routes (default: log-only)
	ASPAUnknownAction uint8  // action for ASPA Unknown routes (default: accept)
	// OriginInvalidAction is the RFC 6811 Section 3 operator-configurable action for the Invalid
	// origin-validation state (default: reject). It is what makes the exclusion of Invalid routes
	// an explicitly configured policy choice (RFC 6811 Section 2) rather than an unconditional
	// side effect: only OriginInvalidAction == ASPAPolicyReject excludes the route.
	OriginInvalidAction uint8
	// OriginNotFoundAction is the action for the NotFound origin-validation state (default: accept).
	OriginNotFoundAction uint8
	// PeerActions holds per-peer resolved action overrides, keyed by the peer's remote IP
	// (matching the route's peerAddr at decision time). A peer is present only when peer- or
	// group-level config overrode at least one leaf; absent peers use the global actions above.
	PeerActions map[string]peerActionSet
}

// parseRPKIConfig extracts RPKI configuration from a BGP config JSON string.
// The JSON is delivered by the engine via OnConfigure with root="bgp".
// Returns empty config (no cache servers) when no rpki section is present.
func parseRPKIConfig(jsonStr string) (*rpkiConfig, error) {
	bgpTree, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		return nil, errRpkiInvalidBgpConfigJson
	}

	cfg := &rpkiConfig{
		ASPAValidation:       false,
		ASPAInvalidAction:    ASPAPolicyLogOnly,
		ASPAUnknownAction:    ASPAPolicyAccept,
		OriginInvalidAction:  ASPAPolicyReject, // RFC 6811: default reject Invalid (matches YANG default)
		OriginNotFoundAction: ASPAPolicyAccept, // default accept NotFound (matches YANG default)
	}

	rpkiMap, ok := bgpTree["rpki"].(map[string]any)
	if !ok {
		return cfg, nil // No RPKI config section -- empty config
	}

	// Parse validation-timeout
	if vtStr, ok := rpkiMap["validation-timeout"].(string); ok {
		vt, err := strconv.ParseUint(vtStr, 10, 16)
		if err == nil {
			cfg.ValidationTimeout = uint16(vt) //nolint:gosec // range checked by ParseUint
		}
	}

	// Parse origin-validation actions from rpki/action container. RFC 6811 Section 3: the action
	// for each validation state is operator-configurable. The YANG defaults are
	// action/invalid=reject and action/not-found=accept.
	if actionMap, ok := rpkiMap["action"].(map[string]any); ok {
		if action, set := parseActionLeaf(actionMap, "invalid"); set {
			cfg.OriginInvalidAction = action
		}
		if action, set := parseActionLeaf(actionMap, "not-found"); set {
			cfg.OriginNotFoundAction = action
		}
	}

	// Parse ASPA settings from rpki/aspa container.
	if aspaMap, ok := rpkiMap["aspa"].(map[string]any); ok {
		if valStr, ok := aspaMap["validation"].(string); ok {
			cfg.ASPAValidation = valStr == "true" || valStr == "1"
		}
		if actionMap, ok := aspaMap["action"].(map[string]any); ok {
			if action, set := parseActionLeaf(actionMap, "invalid"); set {
				cfg.ASPAInvalidAction = action
			}
			if action, set := parseActionLeaf(actionMap, "unknown"); set {
				cfg.ASPAUnknownAction = action
			}
		}
	}

	// Parse per-peer / per-group action overrides. Uses the final global actions above as the
	// fallback for unset leaves, so the resolved sets are what enforcement applies.
	cfg.PeerActions = parsePeerActions(bgpTree, cfg)

	// Parse cache-server list (YANG list keyed by address)
	csMap, ok := rpkiMap["cache-server"].(map[string]any)
	if !ok {
		return cfg, nil // RPKI section exists but no cache servers
	}

	for addr, serverRaw := range csMap {
		serverMap, ok := serverRaw.(map[string]any)
		if !ok {
			continue
		}

		cs := cacheServerConfig{
			Address:    addr,
			Port:       323, // RTR default port
			Preference: 100, // YANG default
		}

		if portStr, ok := serverMap["port"].(string); ok {
			p, err := strconv.ParseUint(portStr, 10, 16)
			if err == nil {
				cs.Port = uint16(p) //nolint:gosec // range checked by ParseUint
			}
		}

		if prefStr, ok := serverMap["preference"].(string); ok {
			p, err := strconv.ParseUint(prefStr, 10, 8)
			if err == nil {
				cs.Preference = uint8(p) //nolint:gosec // range checked by ParseUint
			}
		}

		if sa, ok := serverMap["source-address"].(string); ok && sa != "" {
			cs.SourceAddress = sa
		}

		cfg.CacheServers = append(cfg.CacheServers, cs)
	}

	return cfg, nil
}

// parseActionLeaf reads a single action enum leaf ("reject"/"log-only"/"accept") from a
// container map. Returns (action, true) when the leaf is present and valid, (0, false) otherwise.
func parseActionLeaf(m map[string]any, key string) (uint8, bool) {
	s, ok := m[key].(string)
	if !ok {
		return 0, false
	}
	return aspaActionFromString(s)
}

// actionOverride holds the optionally-set action leaves from one rpki container
// (peer- or group-level). A nil pointer means the leaf was not set at that level.
type actionOverride struct {
	originInvalid  *uint8
	originNotFound *uint8
	aspaInvalid    *uint8
	aspaUnknown    *uint8
}

// parseActionOverride extracts the four action leaves from a peer/group `rpki` container map.
// rpkiMap is the value of the peer's or group's "rpki" key (nil when absent).
func parseActionOverride(rpkiMap map[string]any) actionOverride {
	var o actionOverride
	if rpkiMap == nil {
		return o
	}
	if actionMap, ok := rpkiMap["action"].(map[string]any); ok {
		if action, set := parseActionLeaf(actionMap, "invalid"); set {
			o.originInvalid = &action
		}
		if action, set := parseActionLeaf(actionMap, "not-found"); set {
			o.originNotFound = &action
		}
	}
	if aspaMap, ok := rpkiMap["aspa"].(map[string]any); ok {
		if actionMap, ok := aspaMap["action"].(map[string]any); ok {
			if action, set := parseActionLeaf(actionMap, "invalid"); set {
				o.aspaInvalid = &action
			}
			if action, set := parseActionLeaf(actionMap, "unknown"); set {
				o.aspaUnknown = &action
			}
		}
	}
	return o
}

// parsePeerActions walks every peer in the config and builds the per-peer resolved action map,
// keyed by remote IP. Each leaf resolves peer > group > global. A peer is recorded only when at
// least one leaf came from peer or group config (an all-global peer uses the global path).
func parsePeerActions(bgpTree map[string]any, global *rpkiConfig) map[string]peerActionSet {
	result := make(map[string]peerActionSet)

	configjson.ForEachPeer(bgpTree, func(_ string, peerMap, groupMap map[string]any) {
		var peerRPKI, groupRPKI map[string]any
		if peerMap != nil {
			peerRPKI, _ = peerMap["rpki"].(map[string]any)
		}
		if groupMap != nil {
			groupRPKI, _ = groupMap["rpki"].(map[string]any)
		}

		peerOv := parseActionOverride(peerRPKI)
		groupOv := parseActionOverride(groupRPKI)

		set := peerActionSet{
			OriginInvalid:  resolveLeaf(global.OriginInvalidAction, groupOv.originInvalid, peerOv.originInvalid),
			OriginNotFound: resolveLeaf(global.OriginNotFoundAction, groupOv.originNotFound, peerOv.originNotFound),
			ASPAInvalid:    resolveLeaf(global.ASPAInvalidAction, groupOv.aspaInvalid, peerOv.aspaInvalid),
			ASPAUnknown:    resolveLeaf(global.ASPAUnknownAction, groupOv.aspaUnknown, peerOv.aspaUnknown),
		}

		// All-global: no override, so the global path already covers this peer.
		if set.OriginInvalid.Source == sourceGlobal && set.OriginNotFound.Source == sourceGlobal &&
			set.ASPAInvalid.Source == sourceGlobal && set.ASPAUnknown.Source == sourceGlobal {
			return
		}

		// Key on the remote IP so buildDecisions' req.peerAddr matches. Dynamic/range peers have
		// no static remote IP (connection>remote>ip == "dynamic" or absent) and cannot be keyed;
		// they fall back to the global actions (documented Known Limitation).
		ip := configjson.PeerRemoteIP(peerMap, groupMap)
		if ip == "" || ip == "dynamic" {
			logger().Warn("rpki: per-peer action override ignored: peer has no static remote IP")
			return
		}
		result[ip] = set
	})

	if len(result) == 0 {
		return nil
	}
	return result
}

// resolveLeaf merges one action leaf with peer > group > global precedence,
// recording the source for display.
func resolveLeaf(global uint8, group, peer *uint8) resolvedAction {
	if peer != nil {
		return resolvedAction{Action: *peer, Source: sourcePeer}
	}
	if group != nil {
		return resolvedAction{Action: *group, Source: sourceGroup}
	}
	return resolvedAction{Action: global, Source: sourceGlobal}
}

// Design: docs/architecture/plugin/rib-storage-design.md -- RPKI config parsing
// RFC: rfc/short/rfc6811.md -- Section 3, the per-state actions parsed here
// RFC: rfc/short/rfc7999.md -- Section 3.3, the per-session blackhole agreement
// Overview: rpki.go -- plugin entry point using parsed config
package rpki

import (
	"errors"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/blackholecfg"
	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
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
	// BlackholeExempt keeps a BLACKHOLE-tagged route whose ONLY origin-validation
	// fault is that its prefix is longer than a covering VRP allows. RFC 7999
	// Section 3.3 puts that obligation on the operator and names no mechanism;
	// this is it. Peer then group, with no global level: Section 3.3 binds the
	// blackhole agreement to one BGP session, and a daemon-wide exemption would
	// reach sessions that agreed to nothing.
	BlackholeExempt bool
	// BlackholeCommunities are the values this session agreed to blackhole on,
	// read from the same `blackhole communities` leaf-list the honoring path uses.
	// The exemption protects a LEGITIMATE announcement, and RFC 7999 Section 3.3
	// makes the session's agreement part of what legitimate means, so an empty
	// list exempts nothing however the exempt flag is set.
	BlackholeCommunities []attribute.Community
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
	// PeerActions holds per-peer resolved action overrides, keyed by configjson.KeyFor:
	// the peer's remote IP (matching the route's peerAddr at decision time), and a dynamic
	// group's NAME for the template its members inherit. A peer is present only when peer-
	// or group-level config overrode at least one leaf; absent peers use the global actions
	// above. Readers resolve both with configjson.LookupPeerConfig.
	PeerActions map[configjson.PeerConfigKey]peerActionSet
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
	peerActions, err := parsePeerActions(bgpTree, cfg)
	if err != nil {
		return nil, err
	}
	cfg.PeerActions = peerActions

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
	originInvalid   *uint8
	originNotFound  *uint8
	aspaInvalid     *uint8
	aspaUnknown     *uint8
	blackholeExempt *bool
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
	o.blackholeExempt = parseBoolLeaf(rpkiMap["blackhole-exempt"])
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

// subjectKind names what a resolved-action key identifies, for a message an
// operator reads. A template's key holds a GROUP's name, and "peer ix" sends the
// operator looking for a peer that does not exist.
func subjectKind(key configjson.PeerConfigKey) string {
	if key.Template {
		return "group"
	}
	return "peer"
}

// parsePeerActions walks every peer in the config and builds the per-peer resolved action map,
// keyed by configjson.KeyFor. Each leaf resolves peer > group > global. A peer is recorded only
// when at least one leaf came from peer or group config (an all-global peer uses the global path).
//
// A dynamic group's template is recorded under the group's name. Its members are
// created from that template when a connection arrives, so they appear nowhere in
// the config document and the group's name is the only identity they share with it
// (configjson.PeerOrigin). The decision path resolves address then group.
//
// The `blackhole` container is read through blackholecfg rather than here, because
// the honoring path and the origination gate read the same leaves and a second
// walk is what would let the three answers drift. Its error is returned rather
// than logged: the container decides whether a peer can make Ze discard traffic,
// so a value nobody could parse must not resolve to a silently empty agreement.
func parsePeerActions(bgpTree map[string]any, global *rpkiConfig) (map[configjson.PeerConfigKey]peerActionSet, error) {
	agreements, err := blackholecfg.Parse(bgpTree)
	if err != nil {
		return nil, err
	}

	result := make(map[configjson.PeerConfigKey]peerActionSet)

	configjson.ForEachPeer(bgpTree, func(peerName string, peerMap, groupMap map[string]any, origin configjson.PeerOrigin) {
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
			OriginInvalid:   resolveLeaf(global.OriginInvalidAction, groupOv.originInvalid, peerOv.originInvalid),
			OriginNotFound:  resolveLeaf(global.OriginNotFoundAction, groupOv.originNotFound, peerOv.originNotFound),
			ASPAInvalid:     resolveLeaf(global.ASPAInvalidAction, groupOv.aspaInvalid, peerOv.aspaInvalid),
			ASPAUnknown:     resolveLeaf(global.ASPAUnknownAction, groupOv.aspaUnknown, peerOv.aspaUnknown),
			BlackholeExempt: resolveBoolLeaf(groupOv.blackholeExempt, peerOv.blackholeExempt),
		}

		// All-global: no override, so the global path already covers this peer.
		// BlackholeExempt has no global level, so a peer that sets only that leaf
		// must still be recorded.
		if set.OriginInvalid.Source == sourceGlobal && set.OriginNotFound.Source == sourceGlobal &&
			set.ASPAInvalid.Source == sourceGlobal && set.ASPAUnknown.Source == sourceGlobal &&
			!set.BlackholeExempt {
			return
		}

		// Key on what a runtime reader can produce: the remote IP for a configured
		// peer, and the group's name for a dynamic group's template. A NAMED peer
		// with no static remote IP (its own, or the "dynamic" placeholder inherited
		// from its group) is neither -- the reactor never builds it from the
		// template, so no consumer ever holds an address for it -- and an entry
		// under a key nothing queries reads as in force and does nothing.
		key, ok := configjson.KeyFor(peerName, peerMap, groupMap, origin)
		if !ok {
			logger().Warn("rpki: per-peer action override ignored: no static remote ip",
				"peer", peerName,
				"effect", "origin validation and ASPA use the global actions for this session",
				"fix", "set connection > remote > ip to a literal address on the peer")
			return
		}
		// Canonicalize the address the operator wrote. Every reader produces its key
		// with netip.Addr.String(), and blackholecfg.Parse stores its agreements the
		// same way, so one spelling serves the runtime lookup and the read below.
		if !key.Template {
			if addr, addrErr := netip.ParseAddr(key.ID); addrErr == nil {
				key.ID = addr.String()
			}
		}
		// The two maps come from one walk over one document, so this key is the key
		// blackholecfg stored the same visit's agreement under. Branching on the
		// comma-ok keeps a miss visible: the zero Rule names no community, which is
		// the answer for a session that agreed to nothing (ai/rules/evidence.md).
		if rule, agreed := agreements[key]; agreed {
			set.BlackholeCommunities = rule.Communities
		}
		// A guard that fails closed must say so (ai/rules/evidence.md). This one
		// keeps a route origin validation would drop, so an operator who asked for
		// it and gets nothing has to be told which leaf is missing rather than
		// reading a running config that exempts nothing.
		if set.BlackholeExempt && len(set.BlackholeCommunities) == 0 {
			logger().Warn("rpki: blackhole-exempt has no effect on this session: it names no blackhole community",
				subjectKind(key), key.ID,
				"fix", "add `blackhole { communities <value>; }` or `blackhole { prefixes <block>; }` to the same peer or its group")
		}
		result[key] = set
	})

	if len(result) == 0 {
		// A nil map, not an empty one: the decision path branches on nil to mean
		// "every route uses the global actions", and that branch is the common
		// deployment.
		result = nil
	}
	return result, nil
}

// parseBoolLeaf reads a YANG boolean that may be absent. The config framework
// delivers leaf values as strings and a JSON round-trip delivers real booleans,
// so both forms are read. An unparseable value reads as ABSENT, which leaves
// the group value standing rather than silently canceling it.
func parseBoolLeaf(v any) *bool {
	switch b := v.(type) {
	case bool:
		return &b
	case string:
		switch b {
		case "true":
			t := true
			return &t
		case "false":
			f := false
			return &f
		}
	}
	return nil
}

// resolveBoolLeaf merges one boolean leaf with peer > group precedence. There is
// no global level for it, so an unset leaf at both levels is false.
func resolveBoolLeaf(group, peer *bool) bool {
	if peer != nil {
		return *peer
	}
	if group != nil {
		return *group
	}
	return false
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

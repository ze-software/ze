// Design: docs/architecture/bgp/filter-irr.md -- IRR filter config parsing from OnConfigure JSON

package filter_irr

import (
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/events"
)

// pluginName is the registered plugin name (register.go). It is also the
// left-hand side of a filter chain reference: the engine routes a chain entry to
// a plugin by cutting the ref at the first ':' and dispatching on that prefix
// (internal/component/bgp/reactor/filter_chain.go PolicyFilterChain), so
// "bgp-filter-irr:65001" is the only spelling that can reach this plugin.
const pluginName = "bgp-filter-irr"

const (
	defaultServer          = "whois.radb.net"
	defaultPeeringDBURL    = "https://www.peeringdb.com"
	defaultRefreshInterval = uint32(3600)
	minRefreshInterval     = uint32(60)
	maxRefreshInterval     = uint32(86400)
)

type irrConfig struct {
	Server          string
	PeeringDBURL    string
	SourceAddress   string
	RefreshInterval uint32
	Peers           []peerIRRConfig
}

type peerIRRConfig struct {
	PeerAddr  string
	RemoteASN uint32
	ASSet     string
	Disabled  bool

	// RemoteIP is the peer's address (connection > remote > ip), the key the
	// engine names the peer by in a filter UPDATE. PeerAddr is the config
	// list key, which is the peer's NAME. Invalid when the config states none.
	RemoteIP netip.Addr

	// RefreshGrant reports whether the peer, or its group, attaches this
	// plugin with `send [ refresh ]`, without which the engine refuses the
	// ROUTE-REFRESH the re-admission worker asks for (readmit.go).
	RefreshGrant bool

	// RouteRefresh reports whether ze advertises the Route Refresh capability
	// to the peer: its `capability > route-refresh`, or its group's, is enable
	// or require (reactor config_capabilities.go). Without it the capability
	// is never negotiated and the re-admission worker cannot ask the peer.
	RouteRefresh bool

	// UsesIRR reports whether this peer actually opted into IRR filtering,
	// either by naming a bgp-filter-irr entry in a filter chain that applies to
	// it (global, group, or peer level) or by setting an explicit
	// session.irr.as-set. Only such a peer is enrolled for resolution; see
	// handleConfigure. A peer that merely has a remote ASN has NOT asked for
	// IRR: its filter chain never reaches this plugin, so resolving it would be
	// an unsolicited whois/PeeringDB request on the operator's behalf.
	UsesIRR bool
}

// parseIRRConfig reads the IRR settings from policy { irr { } } and the peers
// that opted into IRR filtering from the bgp root.
func parseIRRConfig(policyCfg, bgpCfg map[string]any) *irrConfig {
	cfg := &irrConfig{
		Server:          defaultServer,
		PeeringDBURL:    defaultPeeringDBURL,
		RefreshInterval: defaultRefreshInterval,
	}

	if irrBlock, ok := policyCfg["irr"].(map[string]any); ok {
		if srv, ok := irrBlock["server"].(string); ok && srv != "" {
			cfg.Server = srv
		}
		if pdbURL, ok := irrBlock["peeringdb-url"].(string); ok && pdbURL != "" {
			cfg.PeeringDBURL = pdbURL
		}
		if sa, ok := irrBlock["source-address"].(string); ok && sa != "" {
			cfg.SourceAddress = sa
		}
		if v, ok := readUint(irrBlock["refresh-interval"]); ok {
			cfg.RefreshInterval = clampRefreshInterval(v)
		}
	}

	globalChained := chainReferencesIRR(bgpCfg)

	// A dynamic group's template visit is discarded here, and the origin is not
	// read for that reason. IRR resolution is keyed by the peer's remote ASN,
	// which parsePeerIRR reads from session.asn.remote. A listen-range group
	// states no remote ASN: its members declare theirs in the OPEN, after the
	// config is parsed. So the template carries nothing this plugin could key,
	// and the RemoteASN == 0 test below already drops it.
	configjson.ForEachPeer(bgpCfg, func(peerAddr string, peerMap, groupMap map[string]any, _ configjson.PeerOrigin) {
		p := parsePeerIRR(peerAddr, peerMap)
		if p.RemoteASN == 0 {
			return
		}
		// A chain at any level that applies to this peer opts it in; so does an
		// explicit as-set, which is a direct request to resolve this peer's
		// prefixes (and keeps `show bgp irr check` usable as a dry run before
		// the chain is wired).
		p.UsesIRR = globalChained ||
			chainReferencesIRR(groupMap) ||
			chainReferencesIRR(peerMap) ||
			p.ASSet != ""
		if ip, err := netip.ParseAddr(configjson.PeerRemoteIP(peerMap, groupMap)); err == nil {
			p.RemoteIP = ip
		}
		p.RefreshGrant = grantsRefresh(peerMap, groupMap)
		p.RouteRefresh = advertisesRouteRefresh(peerMap, groupMap)
		cfg.Peers = append(cfg.Peers, p)
	})

	return cfg
}

// enrolled reports whether configure enrolls the peer for IRR filtering.
func (p *peerIRRConfig) enrolled() bool {
	if p.Disabled {
		return false
	}
	return p.UsesIRR
}

// sendRefresh is the `send` list word that lets a process ask the engine for a
// ROUTE-REFRESH (reactor send_permission.go).
const sendRefresh = "refresh"

// grantsRefresh reports whether the peer's binding of this plugin carries
// `send [ refresh ]` or `send [ * ]`, the wildcard that grants every type
// (reactor parseOneSendFlag). ResolveBGPTree merges the peer over its group
// key by key, and `send` is not cumulative (bgp/config/resolve.go
// cumulativePaths), so the peer's own `send` list replaces the group's: the
// group decides only when the peer's binding states none.
func grantsRefresh(peerMap, groupMap map[string]any) bool {
	if send, stated := bindingSend(peerMap); stated {
		return grantsRefreshWord(send)
	}
	if send, stated := bindingSend(groupMap); stated {
		return grantsRefreshWord(send)
	}
	return false
}

// bindingSend returns the `send` list of this plugin's binding at one config
// level, and whether that level states one.
func bindingSend(m map[string]any) ([]string, bool) {
	attach, ok := m["attach"].(map[string]any)
	if !ok {
		return nil, false
	}
	processes, ok := attach["process"].(map[string]any)
	if !ok {
		return nil, false
	}
	binding, ok := processes[pluginName].(map[string]any)
	if !ok {
		return nil, false
	}
	raw, stated := binding["send"]
	if !stated {
		return nil, false
	}
	return configvalue.LeafList(raw), true
}

func grantsRefreshWord(send []string) bool {
	if slices.Contains(send, events.TokenWildcard) {
		return true
	}
	return slices.Contains(send, sendRefresh)
}

// advertisesRouteRefresh reports whether ze advertises Route Refresh to the
// peer. The peer's `capability > route-refresh` decides when it is set, and
// its group's otherwise, as ResolveBGPTree merges the capability container key
// by key. Absent means disabled (reactor config_capabilities.go).
func advertisesRouteRefresh(peerMap, groupMap map[string]any) bool {
	for _, level := range []map[string]any{peerMap, groupMap} {
		capabilities, ok := level["capability"].(map[string]any)
		if !ok {
			continue
		}
		mode, stated := capabilities["route-refresh"].(string)
		if !stated {
			continue
		}
		// The two modes that advertise; disable and refuse do not.
		return mode == "enable" || mode == "require"
	}
	return false
}

// peersMissingRouteRefresh names every enrolled peer to which ze does not
// advertise Route Refresh, so the filter can never ask it for its routes again.
func peersMissingRouteRefresh(cfg *irrConfig) []string {
	var names []string
	for i := range cfg.Peers {
		peer := &cfg.Peers[i]
		if !peer.enrolled() {
			continue
		}
		if peer.RouteRefresh {
			continue
		}
		names = append(names, peer.PeerAddr)
	}
	return names
}

// peersMissingRefreshGrant names every enrolled peer that does not let this
// plugin ask it for its routes again.
func peersMissingRefreshGrant(cfg *irrConfig) []string {
	var names []string
	for i := range cfg.Peers {
		peer := &cfg.Peers[i]
		if !peer.enrolled() {
			continue
		}
		if peer.RefreshGrant {
			continue
		}
		names = append(names, peer.PeerAddr)
	}
	return names
}

// chainReferencesIRR reports whether the import or export filter chain at one
// config level (bgp / group / peer) names this plugin. A chain entry is
// "<plugin>:<filter>"; the engine cuts at the first ':' and dispatches on the
// prefix (reactor/filter_chain.go PolicyFilterChain), so only that spelling can
// invoke us -- a bare name would dispatch to a plugin of that name instead.
// Refs arrive clean via ToMap; per-member deactivation is out-of-band, so there
// is no "inactive:" prefix to strip (same as filter_family's chain reader).
func chainReferencesIRR(m map[string]any) bool {
	if m == nil {
		return false
	}
	filterBlock, ok := m["filter"].(map[string]any)
	if !ok {
		return false
	}
	for _, key := range [...]string{"import", "export"} {
		policy, ok := filterBlock[key].(map[string]any)
		if !ok {
			continue
		}
		for _, ref := range configvalue.LeafList(policy["filters"]) {
			if plugin, _, found := strings.Cut(ref, ":"); found && plugin == pluginName {
				return true
			}
		}
	}
	return false
}

func parsePeerIRR(peerAddr string, peerMap map[string]any) peerIRRConfig {
	p := peerIRRConfig{PeerAddr: peerAddr}
	if peerMap == nil {
		return p
	}

	session, ok := peerMap["session"].(map[string]any)
	if !ok {
		return p
	}

	if asnBlock, ok := session["asn"].(map[string]any); ok {
		if v, ok := readUint(asnBlock["remote"]); ok {
			p.RemoteASN = uint32(v) //nolint:gosec // clamped by readUint uint64
		}
	}

	if irrBlock, ok := session["irr"].(map[string]any); ok {
		if asSet, ok := irrBlock["as-set"].(string); ok && asSet != "" {
			if irr.ValidateASSetName(asSet) == nil {
				p.ASSet = asSet
			} else {
				logger().Warn("irr: ignoring invalid as-set in config", "peer", peerAddr, "as-set", asSet)
			}
		}
		if enable, ok := irrBlock["enable"].(string); ok && enable == "disable" {
			p.Disabled = true
		}
	}

	return p
}

func clampRefreshInterval(v uint64) uint32 {
	if v < uint64(minRefreshInterval) {
		return minRefreshInterval
	}
	if v > uint64(maxRefreshInterval) {
		return maxRefreshInterval
	}
	return uint32(v) //nolint:gosec // range checked above
}

func readUint(v any) (uint64, bool) {
	switch n := v.(type) {
	case float64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case int64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true //nolint:gosec // range checked above
	case uint64:
		return n, true
	case string:
		x, err := strconv.ParseUint(n, 10, 64)
		if err != nil {
			return 0, false
		}
		return x, true
	default:
		return 0, false
	}
}
